.PHONY: run tune tidy test editor

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
	