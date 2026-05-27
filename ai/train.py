import argparse
import os
import signal
import subprocess
import time

from stable_baselines3 import PPO
from stable_baselines3.common.callbacks import CheckpointCallback, BaseCallback
from stable_baselines3.common.env_util import make_vec_env

from config import PPO_PARAMS, CHECKPOINT_INTERVAL, BASE_PORT
from env import PixelGameEnv


class GracefulShutdown(BaseCallback):
    def __init__(self, save_path: str):
        super().__init__()
        self.save_path = save_path
        self._shutdown = False
        signal.signal(signal.SIGINT, self._handler)

    def _handler(self, signum, frame):
        print("\nGraceful shutdown — saving checkpoint...")
        self._shutdown = True

    def _on_step(self) -> bool:
        if self._shutdown:
            path = os.path.join(self.save_path, "ppo_interrupted")
            self.model.save(path)
            print(f"Saved to {path}.zip")
            return False
        return True


def main():
    parser = argparse.ArgumentParser(description="Train PPO agent")
    parser.add_argument("--timesteps", type=int, default=1_000_000)
    parser.add_argument("--envs", type=int, default=4)
    parser.add_argument("--resume", type=str, default=None, help="Path to checkpoint .zip to resume from")
    parser.add_argument("--start-server", action="store_true", help="Auto-start Go training server")
    args = parser.parse_args()

    os.makedirs("checkpoints", exist_ok=True)
    os.makedirs("logs", exist_ok=True)

    server_proc = None
    if args.start_server:
        print(f"Starting Go training server with {args.envs} envs...")
        server_proc = subprocess.Popen(
            ["go", "run", "../cmd/train", f"-envs={args.envs}", f"-port={BASE_PORT}"],
            cwd=os.path.dirname(os.path.abspath(__file__)),
        )
        time.sleep(3)

    try:
        vec_env = make_vec_env(PixelGameEnv, n_envs=args.envs, env_kwargs=[
            {"port": BASE_PORT + i} for i in range(args.envs)
        ])

        if args.resume:
            print(f"Resuming from {args.resume}")
            model = PPO.load(args.resume, env=vec_env, tensorboard_log="logs/")
        else:
            model = PPO(
                "MlpPolicy",
                vec_env,
                verbose=1,
                tensorboard_log="logs/",
                **PPO_PARAMS,
            )

        checkpoint_cb = CheckpointCallback(
            save_freq=max(CHECKPOINT_INTERVAL // args.envs, 1),
            save_path="checkpoints/",
            name_prefix="ppo",
        )
        shutdown_cb = GracefulShutdown("checkpoints/")

        print(f"Training for {args.timesteps} timesteps with {args.envs} envs...")
        model.learn(
            total_timesteps=args.timesteps,
            callback=[checkpoint_cb, shutdown_cb],
            reset_num_timesteps=args.resume is None,
        )

        model.save("checkpoints/ppo_final")
        print("Training complete. Saved to checkpoints/ppo_final.zip")

    finally:
        vec_env.close()
        if server_proc:
            server_proc.terminate()
            server_proc.wait()


if __name__ == "__main__":
    main()
