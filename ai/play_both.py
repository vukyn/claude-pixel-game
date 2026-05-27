import argparse
import json
import socket
import time

import numpy as np
from stable_baselines3 import PPO

from orc_config import ORC_BASE_PORT


def main():
    parser = argparse.ArgumentParser(description="Watch player vs orc AI battle")
    parser.add_argument("--player-model", type=str, default="checkpoints/ppo_final.zip")
    parser.add_argument("--orc-model", type=str, default="checkpoints/orc_final.zip")
    parser.add_argument("--port", type=int, default=ORC_BASE_PORT)
    parser.add_argument("--episodes", type=int, default=0, help="0 = infinite")
    parser.add_argument("--speed", type=float, default=1.0, help="Playback speed multiplier")
    args = parser.parse_args()

    print(f"Loading player model: {args.player_model}")
    player_model = PPO.load(args.player_model)
    print(f"Loading orc model: {args.orc_model}")
    orc_model = PPO.load(args.orc_model)

    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.connect(("localhost", args.port))
    rfile = sock.makefile("r")

    def send(msg):
        sock.sendall((json.dumps(msg) + "\n").encode())

    def recv():
        line = rfile.readline()
        if not line:
            raise ConnectionError("Server closed")
        return json.loads(line)

    ep = 0
    try:
        while args.episodes == 0 or ep < args.episodes:
            send({"type": "reset"})
            resp = recv()
            step = 0
            done = False

            while not done:
                player_obs = np.array(resp["player_obs"], dtype=np.float32)
                orc_obs_list = resp["orc_obs"]

                player_action, _ = player_model.predict(player_obs, deterministic=True)
                orc_actions = []
                for orc_obs in orc_obs_list:
                    obs = np.array(orc_obs, dtype=np.float32)
                    action, _ = orc_model.predict(obs, deterministic=True)
                    orc_actions.append(int(action))

                send({
                    "type": "action",
                    "player_action": int(player_action),
                    "orc_actions": orc_actions,
                })
                resp = recv()
                done = resp["done"]
                step += 1
                time.sleep((1 / 60) / args.speed)

            ep += 1
            info = resp.get("info", {})
            print(f"Episode {ep}: steps={step}, "
                  f"player_lives={info.get('player_lives', '?')}, "
                  f"orcs={info.get('orc_count', '?')}")

    except KeyboardInterrupt:
        pass
    finally:
        sock.close()


if __name__ == "__main__":
    main()
