import argparse

import numpy as np
from stable_baselines3 import PPO

from config import BASE_PORT
from env import PixelGameEnv


def main():
    parser = argparse.ArgumentParser(description="Evaluate trained model")
    parser.add_argument("--model", type=str, required=True, help="Path to model .zip")
    parser.add_argument("--episodes", type=int, default=100)
    parser.add_argument("--port", type=int, default=BASE_PORT)
    args = parser.parse_args()

    model = PPO.load(args.model)
    env = PixelGameEnv(port=args.port)

    scores = []
    survived = 0

    for ep in range(args.episodes):
        obs, _ = env.reset()
        total_reward = 0
        done = False
        while not done:
            action, _ = model.predict(obs, deterministic=True)
            obs, reward, done, _, info = env.step(action)
            total_reward += reward

        score = info.get("score", 0)
        scores.append(score)
        lives = info.get("lives", 0)
        if lives > 0:
            survived += 1

        if (ep + 1) % 10 == 0:
            print(f"Episode {ep+1}/{args.episodes}: score={score}, lives={lives}")

    env.close()

    scores = np.array(scores)
    print(f"\n{'='*40}")
    print(f"Episodes: {args.episodes}")
    print(f"Mean score: {scores.mean():.1f} ± {scores.std():.1f}")
    print(f"Max score: {scores.max()}")
    print(f"Min score: {scores.min()}")
    print(f"Survival rate: {survived}/{args.episodes} ({100*survived/args.episodes:.0f}%)")


if __name__ == "__main__":
    main()
