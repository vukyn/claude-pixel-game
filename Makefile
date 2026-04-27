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
