.PHONY: help build run debug logincheck test check proto clean

GO := go
BIN_DIR := bin
APP := $(BIN_DIR)/rom-agent
PROTO_OUT := internal/proto/pb

help:
	@printf "Targets: build run debug logincheck test check proto clean\n"

build:
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(APP) ./cmd/rom-agent

run:
	$(GO) run ./cmd/rom-agent --config config/config.yaml

debug:
	$(GO) run ./cmd/rom-agent --config config/config.yaml --debug

logincheck:
	$(GO) run ./cmd/logincheck

test:
	$(GO) test ./...

check:
	$(GO) test ./internal/client
	$(GO) build ./...

run-sea:
	$(GO) run ./cmd/rom-agent --config config/config.yaml

proto:
	PATH="$$(go env GOPATH)/bin:$$PATH" protoc --go_out=$(PROTO_OUT) --go_opt=paths=source_relative proto/Pb.proto

clean:
	$(GO) clean
	rm -rf $(BIN_DIR) coverage.out coverage.html
	rm -f bot_state_*.json
