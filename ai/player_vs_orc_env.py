import json
import socket

import gymnasium as gym
import numpy as np
from gymnasium import spaces

from config import OBS_SIZE, NUM_ACTIONS
from orc_config import ORC_OBS_SIZE


class PlayerVsOrcEnv(gym.Env):
    """Player learns vs frozen orc model. Mirrors OrcGymEnv but reversed roles."""
    metadata = {"render_modes": []}

    def __init__(self, orc_model=None, host="localhost", port=9876):
        super().__init__()
        self.host = host
        self.port = port
        self.orc_model = orc_model
        self.action_space = spaces.Discrete(NUM_ACTIONS)
        self.observation_space = spaces.Box(
            low=0.0, high=1.0, shape=(OBS_SIZE,), dtype=np.float32
        )
        self._sock = None
        self._rfile = None
        self._orc_obs = []

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

    def _get_orc_actions(self):
        if self.orc_model is None or not self._orc_obs:
            return [0] * max(len(self._orc_obs), 1)
        actions = []
        for obs in self._orc_obs:
            arr = np.array(obs, dtype=np.float32)
            action, _ = self.orc_model.predict(arr, deterministic=False)
            actions.append(int(action))
        return actions

    def reset(self, seed=None, options=None):
        super().reset(seed=seed)
        self._connect()
        self._send({"type": "reset"})
        resp = self._recv()
        self._orc_obs = resp["orc_obs"]
        return np.array(resp["player_obs"], dtype=np.float32), {}

    def step(self, action: int):
        orc_actions = self._get_orc_actions()

        self._send({
            "type": "action",
            "player_action": int(action),
            "orc_actions": orc_actions,
        })
        resp = self._recv()
        self._orc_obs = resp["orc_obs"]
        obs = np.array(resp["player_obs"], dtype=np.float32)
        reward = float(resp.get("player_reward", 0.0))
        terminated = bool(resp["done"])

        if terminated:
            new_obs, _ = self.reset()
            return new_obs, reward, False, False, resp.get("info", {})

        return obs, reward, terminated, False, resp.get("info", {})

    def close(self):
        if self._sock is not None:
            self._sock.close()
            self._sock = None
            self._rfile = None
