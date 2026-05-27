import argparse
import os
import signal

from stable_baselines3 import PPO
from stable_baselines3.common.callbacks import CheckpointCallback, BaseCallback
from stable_baselines3.common.vec_env import DummyVecEnv

from orc_config import ORC_PPO_PARAMS, ORC_CHECKPOINT_INTERVAL, ORC_BASE_PORT
from orc_env import OrcGymEnv


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
            path = os.path.join(self.save_path, "orc_interrupted")
            self.model.save(path)
            print(f"Saved to {path}.zip")
            return False
        return True


def main():
    parser = argparse.ArgumentParser(description="Train orc RL agent")
    parser.add_argument("--timesteps", type=int, default=500_000)
    parser.add_argument("--player-model", type=str, default="checkpoints/ppo_final.zip",
                        help="Path to trained player model")
    parser.add_argument("--resume", type=str, default=None)
    args = parser.parse_args()

    os.makedirs("checkpoints", exist_ok=True)
    os.makedirs("logs", exist_ok=True)

    player_model = None
    if os.path.exists(args.player_model):
        print(f"Loading player model from {args.player_model}")
        player_model = PPO.load(args.player_model)
    else:
        print(f"WARNING: Player model not found at {args.player_model}, using idle player")

    vec_env = None
    try:
        vec_env = DummyVecEnv([lambda: OrcGymEnv(player_model=player_model, port=ORC_BASE_PORT)])

        if args.resume:
            print(f"Resuming from {args.resume}")
            model = PPO.load(args.resume, env=vec_env, tensorboard_log="logs/")
        else:
            model = PPO(
                "MlpPolicy",
                vec_env,
                verbose=1,
                tensorboard_log="logs/",
                **ORC_PPO_PARAMS,
            )

        checkpoint_cb = CheckpointCallback(
            save_freq=ORC_CHECKPOINT_INTERVAL,
            save_path="checkpoints/",
            name_prefix="orc",
        )
        shutdown_cb = GracefulShutdown("checkpoints/")

        print(f"Training orc for {args.timesteps} timesteps...")
        model.learn(
            total_timesteps=args.timesteps,
            callback=[checkpoint_cb, shutdown_cb],
            reset_num_timesteps=args.resume is None,
        )

        model.save("checkpoints/orc_final")
        print("Orc training complete. Saved to checkpoints/orc_final.zip")

    finally:
        if vec_env is not None:
            vec_env.close()


if __name__ == "__main__":
    main()
