.PHONY: run tune tidy test editor web web-install web-build

run:
	go run ./cmd/game

tune:
	go run ./cmd/tune $(ARGS)

tidy:
	go mod tidy

test:
	go test ./...

editor:
	go run ./cmd/editor

web:
	cd tools/editor-web && npm run dev

web-install:
	cd tools/editor-web && npm install

web-build:
	cd tools/editor-web && npm run build

# === AI Training ===

.PHONY: ai-setup train-server train train-resume train-visual train-eval train-play train-tensorboard train-clean

ai-setup:              ## Install Python dependencies for AI training
	cd ai && pip install -r requirements.txt

train-server:          ## Start headless Go game server for RL training (ENVS=4 default)
	go run ./cmd/train -envs=$(or $(ENVS),4) -port=9876

train:                 ## Train PPO agent (STEPS=1000000 default, ENVS=4 default)
	cd ai && python3 train.py --timesteps=$(or $(STEPS),1000000) --envs=$(or $(ENVS),4)

train-resume:          ## Resume training from latest checkpoint (STEPS=500000 default)
	cd ai && python3 train.py --resume checkpoints/ppo_latest.zip --timesteps=$(or $(STEPS),500000)

train-visual:          ## Train with game window visible (slower, 60fps, STEPS=50000 default)
	@echo "Training with visual — game window will open..."
	@echo "Close game window or Ctrl+C to stop."
	go run ./cmd/game -ai 9876 &
	@sleep 2
	cd ai && python3 train.py --timesteps=$(or $(STEPS),50000) --envs=1

train-eval:            ## Evaluate trained model over 100 episodes
	cd ai && python3 eval.py --model checkpoints/ppo_final.zip --episodes=$(or $(EPISODES),100)

train-play:            ## Watch AI play game with full rendering (starts game + AI agent)
	@echo "Starting game with AI mode on port 9877..."
	@echo "Close the game window to stop."
	go run ./cmd/game -ai 9877 &
	@sleep 2
	cd ai && python3 play.py --model $(or $(MODEL),checkpoints/ppo_final.zip) --port 9877

train-tensorboard:     ## Open TensorBoard dashboard for training metrics
	tensorboard --logdir ai/logs/

train-clean:           ## Remove all checkpoints and training logs
	rm -rf ai/checkpoints/ ai/logs/
