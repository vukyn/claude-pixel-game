import json
import socket

import gymnasium as gym
import numpy as np
from gymnasium import spaces

from orc_config import ORC_OBS_SIZE, ORC_NUM_ACTIONS


class OrcGymEnv(gym.Env):
    metadata = {"render_modes": []}

    def __init__(self, player_model=None, host="localhost", port=9876):
        super().__init__()
        self.host = host
        self.port = port
        self.player_model = player_model
        self.action_space = spaces.Discrete(ORC_NUM_ACTIONS)
        self.observation_space = spaces.Box(
            low=0.0, high=1.0, shape=(ORC_OBS_SIZE,), dtype=np.float32
        )
        self._sock = None
        self._rfile = None
        self._player_obs = None
        self._last_orc_count = 1

    def _connect(self):
        if self._sock is not None:
            return
        self._sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self._sock.connect((self.host, self.port))
        self._rfile = self._sock.makefile("r")

    def _send(self, msg: dict):
        data = json.dumps(msg) + "\n"
        self._sock.sendall(data.encode())

    def _recv(self) -> dict:
        line = self._rfile.readline()
        if not line:
            raise ConnectionError("Server closed connection")
        return json.loads(line)

    def _get_player_action(self, player_obs):
        if self.player_model is None:
            return 0
        obs = np.array(player_obs, dtype=np.float32)
        action, _ = self.player_model.predict(obs, deterministic=False)
        return int(action)

    def reset(self, seed=None, options=None):
        super().reset(seed=seed)
        self._connect()
        self._send({"type": "reset"})
        resp = self._recv()
        self._player_obs = resp["player_obs"]
        orc_obs = resp["orc_obs"]
        self._last_orc_count = len(orc_obs)
        if len(orc_obs) == 0:
            return np.zeros(ORC_OBS_SIZE, dtype=np.float32), {}
        return np.array(orc_obs[0], dtype=np.float32), {}

    def step(self, action: int):
        player_action = self._get_player_action(self._player_obs)
        orc_actions = [int(action)] * self._last_orc_count

        self._send({
            "type": "action",
            "player_action": player_action,
            "orc_actions": orc_actions,
        })
        resp = self._recv()
        self._player_obs = resp["player_obs"]
        orc_obs_list = resp["orc_obs"]
        orc_rewards = resp["orc_rewards"]
        orc_dones = resp["orc_dones"]
        done = resp["done"]

        self._last_orc_count = max(len(orc_obs_list), 1)

        if len(orc_obs_list) == 0:
            obs = np.zeros(ORC_OBS_SIZE, dtype=np.float32)
            reward = 0.0
            terminated = done
        else:
            obs = np.array(orc_obs_list[0], dtype=np.float32)
            reward = float(orc_rewards[0]) if orc_rewards else 0.0
            terminated = done or (orc_dones[0] if orc_dones else False)

        if terminated:
            obs, _ = self.reset()
            return obs, reward, False, False, resp.get("info", {})

        return obs, reward, terminated, False, resp.get("info", {})

    def close(self):
        if self._sock is not None:
            self._sock.close()
            self._sock = None
            self._rfile = None
