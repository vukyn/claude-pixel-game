# ai/config.py

OBS_SIZE = 25
NUM_ACTIONS = 10

PPO_PARAMS = dict(
    learning_rate=3e-4,
    n_steps=2048,
    batch_size=64,
    n_epochs=10,
    gamma=0.99,
    gae_lambda=0.95,
    clip_range=0.2,
    ent_coef=0.01,
    vf_coef=0.5,
    max_grad_norm=0.5,
    policy_kwargs=dict(
        net_arch=[128, 128],
    ),
)

CHECKPOINT_INTERVAL = 50_000
BASE_PORT = 9876
