import argparse
import os
import signal

from stable_baselines3 import PPO
from stable_baselines3.common.callbacks import CheckpointCallback, BaseCallback
from stable_baselines3.common.vec_env import DummyVecEnv

from config import PPO_PARAMS, CHECKPOINT_INTERVAL, BASE_PORT
from player_vs_orc_env import PlayerVsOrcEnv


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
            path = os.path.join(self.save_path, "ppo_vs_orc_interrupted")
            self.model.save(path)
            print(f"Saved to {path}.zip")
            return False
        return True


def main():
    parser = argparse.ArgumentParser(description="Train player RL agent vs frozen orc model")
    parser.add_argument("--timesteps", type=int, default=500_000)
    parser.add_argument("--orc-model", type=str, default="checkpoints/orc_final.zip",
                        help="Path to trained orc model (frozen opponent)")
    parser.add_argument("--resume", type=str, default=None)
    args = parser.parse_args()

    os.makedirs("checkpoints", exist_ok=True)
    os.makedirs("logs", exist_ok=True)

    orc_model = None
    if os.path.exists(args.orc_model):
        print(f"Loading orc model from {args.orc_model}")
        orc_model = PPO.load(args.orc_model)
    else:
        print(f"WARNING: Orc model not found at {args.orc_model}, using idle orc")

    vec_env = None
    try:
        vec_env = DummyVecEnv([lambda: PlayerVsOrcEnv(orc_model=orc_model, port=BASE_PORT)])

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
            save_freq=CHECKPOINT_INTERVAL,
            save_path="checkpoints/",
            name_prefix="ppo_vs_orc",
        )
        shutdown_cb = GracefulShutdown("checkpoints/")

        print(f"Training player vs orc for {args.timesteps} timesteps...")
        model.learn(
            total_timesteps=args.timesteps,
            callback=[checkpoint_cb, shutdown_cb],
            reset_num_timesteps=args.resume is None,
        )

        model.save("checkpoints/ppo_vs_orc_final")
        print("Player-vs-orc training complete. Saved to checkpoints/ppo_vs_orc_final.zip")

    finally:
        if vec_env is not None:
            vec_env.close()


if __name__ == "__main__":
    main()
