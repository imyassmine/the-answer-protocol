# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ LAYOUT ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

DEFAULT			= \033[0m
CYAN			= \033[1;36m

define ANIMATION
				@for frame in ⠋ ⠙ ⠹ ⠸ ⠼ ⠴ ⠦ ⠧ ⠇ ⠏; do \
				printf "$$frame $(1)...\r"; \
				sleep 0.02; \
				done; \
				printf "\n"
endef


# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ VARIABLES ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

SERVER_PKG		= ./cmd/server/
CLIENT_PKG		= ./cmd/client/
GUI_PKG			= ./cmd/gui/

BIN_DIR			= bin
SERVER_BIN		= $(BIN_DIR)/tap-server
CLIENT_BIN		= $(BIN_DIR)/tap-client
GUI_BIN			= $(BIN_DIR)/tap-gui


# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ RULES ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

install:
				$(call ANIMATION,Installing dependencies)
				@go mod download

build:
				$(call ANIMATION,Building binaries)
				@mkdir -p $(BIN_DIR)
				@go build -o $(SERVER_BIN) $(SERVER_PKG) \
					&& go build -o $(CLIENT_BIN) $(CLIENT_PKG) \
					&& go build -o $(GUI_BIN) $(GUI_PKG) \
					|| { rm -rf $(BIN_DIR); echo "build failed"; exit 1; }

run-server:
				@go run -race $(SERVER_PKG)

run-client:
				@go run -race $(CLIENT_PKG)

run-client-gui:
				@go run -race $(GUI_PKG)

clean:
				$(call ANIMATION,Removing binaries and caches)
				@rm -rf $(BIN_DIR)
				@go clean -testcache

fmt:
				@gofmt -w .

lint:
				@test -z "$$(gofmt -l .)" || { echo "gofmt: files need formatting"; exit 1; }
				@go vet ./...

lint-strict:	lint
				@golangci-lint run ./...

test:
				@go test -race -v ./...


help:
				@printf "Usage: make [target]\n"
				@printf "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n"
				@printf "$(CYAN)install$(DEFAULT)		- Download dependencies\n"
				@printf "$(CYAN)build$(DEFAULT)		- Build server, CLI and web client binaries into bin/\n"
				@printf "$(CYAN)run-server$(DEFAULT)		- Run the server\n"
				@printf "$(CYAN)run-client$(DEFAULT)		- Run the CLI client\n"
				@printf "$(CYAN)run-client-gui$(DEFAULT)	- Run the GUI client\n"
				@printf "$(CYAN)clean$(DEFAULT)		- Remove built binaries and test cache\n"
				@printf "$(CYAN)fmt$(DEFAULT)		- Format all Go files with gofmt\n"
				@printf "$(CYAN)lint$(DEFAULT)		- Fail on unformatted files, then run go vet\n"
				@printf "$(CYAN)lint-strict$(DEFAULT)	- Run lint, then golangci-lint\n"
				@printf "$(CYAN)test$(DEFAULT)		- Run tests with the race detector\n"
				@printf "$(CYAN)help$(DEFAULT)		- Show this help message\n"

.PHONY:			install build run-server run-client run-client-gui clean fmt lint lint-strict test help
