.PHONY: run tune tidy test

run:
	go run ./cmd/game

tune:
	go run ./cmd/tune $(ARGS)

tidy:
	go mod tidy

test:
	go test ./...
	