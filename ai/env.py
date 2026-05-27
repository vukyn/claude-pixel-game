import json
import socket

import gymnasium as gym
import numpy as np
from gymnasium import spaces

from config import OBS_SIZE, NUM_ACTIONS


class PixelGameEnv(gym.Env):
    metadata = {"render_modes": []}

    def __init__(self, host: str = "localhost", port: int = 9876):
        super().__init__()
        self.host = host
        self.port = port
        self.action_space = spaces.Discrete(NUM_ACTIONS)
        self.observation_space = spaces.Box(
            low=0.0, high=1.0, shape=(OBS_SIZE,), dtype=np.float32
        )
        self._sock = None
        self._rfile = None

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

    def reset(self, seed=None, options=None):
        super().reset(seed=seed)
        self._connect()
        self._send({"type": "reset"})
        resp = self._recv()
        obs = np.array(resp["obs"], dtype=np.float32)
        return obs, resp.get("info", {})

    def step(self, action: int):
        self._send({"type": "action", "action": int(action)})
        resp = self._recv()
        obs = np.array(resp["obs"], dtype=np.float32)
        reward = float(resp["reward"])
        terminated = bool(resp["done"])
        truncated = False
        info = resp.get("info", {})
        return obs, reward, terminated, truncated, info

    def close(self):
        if self._sock is not None:
            self._sock.close()
            self._sock = None
            self._rfile = None
