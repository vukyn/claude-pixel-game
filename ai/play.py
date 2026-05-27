import argparse
import time

from stable_baselines3 import PPO

from config import BASE_PORT
from env import PixelGameEnv


def main():
    parser = argparse.ArgumentParser(description="Watch AI play")
    parser.add_argument("--model", type=str, required=True, help="Path to model .zip")
    parser.add_argument("--port", type=int, default=BASE_PORT + 1)
    parser.add_argument("--episodes", type=int, default=0, help="0 = infinite")
    args = parser.parse_args()

    model = PPO.load(args.model)
    env = PixelGameEnv(port=args.port)

    ep = 0
    try:
        while args.episodes == 0 or ep < args.episodes:
            obs, _ = env.reset()
            done = False
            step = 0
            while not done:
                action, _ = model.predict(obs, deterministic=True)
                obs, reward, done, _, info = env.step(action)
                step += 1
                time.sleep(1 / 60)

            ep += 1
            print(f"Episode {ep}: score={info.get('score', 0)}, "
                  f"lives={info.get('lives', 0)}, steps={step}")
    except KeyboardInterrupt:
        pass
    finally:
        env.close()


if __name__ == "__main__":
    main()
