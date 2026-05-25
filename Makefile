GO_VERSION := 1.22.5
GO_INSTALL_DIR := /usr/local

# Devuan daedalus uses older docker so add fall back to docker-compose V1
COMPOSE := $(shell docker compose version >/dev/null 2>&1 && echo "docker compose" || echo "docker-compose")
COMPOSE := $(COMPOSE) -f docker/docker-compose.yml

.PHONY: install-go test build clean docker-build run run-clean stop logs

# These targets are mostly for development.
#
# NOTE: if you haven't exported/customized any of the variables from docker-compose, you'll end up running with defaults
#       so look there for options and export the variables before running.
#       See authbox-launch.sh for a more generalized launcher template.

install-go:
	curl -fsSL https://go.dev/dl/go$(GO_VERSION).linux-amd64.tar.gz -o /tmp/go.tar.gz
	sudo rm -rf $(GO_INSTALL_DIR)/go
	sudo tar -C $(GO_INSTALL_DIR) -xzf /tmp/go.tar.gz
	rm /tmp/go.tar.gz
	@echo "Add to your shell profile: export PATH=\$$PATH:$(GO_INSTALL_DIR)/go/bin"

test:
	go test ./tests/unit/... -v -short

test-all:
	go test ./tests/unit/... -v

build:
	CGO_ENABLED=0 go build -o bin/authbox ./cmd/server

clean:
	rm -rf bin/

docker-build:
	$(COMPOSE) build primary

run: docker-build
	$(COMPOSE) up primary

run-clean:
	$(COMPOSE) down -v
	$(COMPOSE) build primary
	$(COMPOSE) up primary

stop:
	$(COMPOSE) down

logs:
	$(COMPOSE) logs -f primary
