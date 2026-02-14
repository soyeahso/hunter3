# Hunter3 Go Makefile
# The vendor/ directory contains non-Go assets (a2ui), so we use -mod=mod
# to tell Go to ignore it and use the module cache instead.

GOFLAGS := -mod=mod
export GOFLAGS

BINARY := hunter3
BUILD_DIR := dist
VERSION ?= dev
DOCKER_IMAGE := soyeahso/hunter3
DOCKER_TAG ?= $(VERSION)
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X github.com/soyeahso/hunter3/internal/version.Version=$(VERSION) \
           -X github.com/soyeahso/hunter3/internal/version.Commit=$(COMMIT) \
           -X github.com/soyeahso/hunter3/internal/version.Date=$(DATE)

.PHONY: all build test vet lint clean mcp-all mcp-register \
       mcp-fetch-website mcp-make mcp-git mcp-gh mcp-weather mcp-filesystem mcp-brave mcp-docker mcp-gdrive mcp-gmail mcp-imail mcp-digitalocean mcp-curl mcp-ssh \
       docker-login docker-build docker-push

# Build all binaries from cmd/ into dist/
all:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/mcp-brave ./cmd/mcp-brave
	go build -o $(BUILD_DIR)/mcp-fetch-website ./cmd/mcp-fetch-website
	go build -o $(BUILD_DIR)/mcp-make ./cmd/mcp-make
	go build -o $(BUILD_DIR)/mcp-git ./cmd/mcp-git
	go build -o $(BUILD_DIR)/mcp-gh ./cmd/mcp-gh
	go build -o $(BUILD_DIR)/mcp-weather ./cmd/mcp-weather
	go build -o $(BUILD_DIR)/mcp-filesystem ./cmd/mcp-filesystem
	go build -o $(BUILD_DIR)/mcp-docker ./cmd/mcp-docker
	go build -o $(BUILD_DIR)/mcp-digitalocean ./cmd/mcp-digitalocean
	go build -o $(BUILD_DIR)/mcp-gdrive ./cmd/mcp-gdrive
	go build -o $(BUILD_DIR)/mcp-gmail ./cmd/mcp-gmail
	go build -o $(BUILD_DIR)/mcp-imail ./cmd/mcp-imail
	go build -o $(BUILD_DIR)/mcp-ssh ./cmd/mcp-ssh
	go build -o $(BUILD_DIR)/mcp-curl ./cmd/mcp-curl
	@echo "All binaries built in $(BUILD_DIR):"
	@ls -lh $(BUILD_DIR)/

rebuild:
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/hunter3 ./cmd/hunter3

build:
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) ./cmd/$(BINARY)

test:
	go test -race ./internal/... ./cmd/...

vet:
	go vet ./internal/... ./cmd/...

lint: vet
	@which golangci-lint > /dev/null 2>&1 && golangci-lint run ./internal/... ./cmd/... || echo "golangci-lint not installed, skipping (install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)"

clean:
	rm -rf $(BUILD_DIR)

# Quick build for development (no ldflags)
dev:
	go build -o $(BUILD_DIR)/$(BINARY) ./cmd/hunter3

# Run all checks
check: vet test
	@echo "All checks passed"

claude:
	claude --dangerously-skip-permissions

# Build individual MCP plugins (build only, no registration)
mcp-fetch-website:
	go build -o $(BUILD_DIR)/mcp-fetch-website ./cmd/mcp-fetch-website

mcp-make:
	go build -o $(BUILD_DIR)/mcp-make ./cmd/mcp-make

mcp-git:
	go build -o $(BUILD_DIR)/mcp-git ./cmd/mcp-git

mcp-gh:
	go build -o $(BUILD_DIR)/mcp-gh ./cmd/mcp-gh

mcp-weather:
	go build -o $(BUILD_DIR)/mcp-weather ./cmd/mcp-weather

mcp-filesystem:
	go build -o $(BUILD_DIR)/mcp-filesystem ./cmd/mcp-filesystem

mcp-brave:
	go build -o $(BUILD_DIR)/mcp-brave ./cmd/mcp-brave

mcp-docker:
	go build -o $(BUILD_DIR)/mcp-docker ./cmd/mcp-docker

mcp-digitalocean:
	go build -o $(BUILD_DIR)/mcp-digitalocean ./cmd/mcp-digitalocean

mcp-gdrive:
	go build -o $(BUILD_DIR)/mcp-gdrive ./cmd/mcp-gdrive

mcp-gmail:
	go build -o $(BUILD_DIR)/mcp-gmail ./cmd/mcp-gmail

mcp-imail:
	go build -o $(BUILD_DIR)/mcp-imail ./cmd/mcp-imail

mcp-auditor:


mcp-curl:
	go build -o $(BUILD_DIR)/mcp-curl ./cmd/mcp-curl

mcp-ssh:
	go build -o $(BUILD_DIR)/mcp-ssh ./cmd/mcp-ssh

# Build all MCP plugins
mcp-all: mcp-fetch-website mcp-make mcp-git mcp-gh mcp-weather mcp-filesystem mcp-brave mcp-docker mcp-digitalocean mcp-gdrive mcp-gmail mcp-imail mcp-curl mcp-ssh
	@echo "All MCP plugins built in $(BUILD_DIR)/"

# Register all MCP plugins with claude CLI (run once, or when adding new plugins)
mcp-register: mcp-all
	@echo "Registering MCP plugins with claude CLI..."
	@claude mcp add --transport stdio mcp-fetch-website -- $(shell readlink -f $(BUILD_DIR)/mcp-fetch-website) || true
	@claude mcp add --transport stdio mcp-make -- $(shell readlink -f $(BUILD_DIR)/mcp-make) || true
	@claude mcp add --transport stdio mcp-git -- $(shell readlink -f $(BUILD_DIR)/mcp-git) || true
	@claude mcp add --transport stdio mcp-gh -- $(shell readlink -f $(BUILD_DIR)/mcp-gh) || true
	@claude mcp add --transport stdio mcp-weather -- $(shell readlink -f $(BUILD_DIR)/mcp-weather) || true
	@claude mcp add --transport stdio mcp-filesystem -- $(shell readlink -f $(BUILD_DIR)/mcp-filesystem) /home/genoeg/sandbox || true
	@claude mcp add --transport stdio mcp-brave -- $(shell readlink -f $(BUILD_DIR)/mcp-brave) /home/genoeg/sandbox || true
	@claude mcp add --transport stdio mcp-docker -- $(shell readlink -f $(BUILD_DIR)/mcp-docker) || true
	@claude mcp add --transport stdio mcp-digitalocean -- $(shell readlink -f $(BUILD_DIR)/mcp-digitalocean) || true
	@claude mcp add --transport stdio mcp-gdrive -- $(shell readlink -f $(BUILD_DIR)/mcp-gdrive) || true
	@claude mcp add --transport stdio mcp-gmail -- $(shell readlink -f $(BUILD_DIR)/mcp-gmail) || true
	@claude mcp add --transport stdio mcp-imail -- $(shell readlink -f $(BUILD_DIR)/mcp-imail) || true
	@claude mcp add --transport stdio mcp-ssh -- $(shell readlink -f $(BUILD_DIR)/mcp-ssh) || true
	@claude mcp add --transport stdio mcp-curl -- $(shell readlink -f $(BUILD_DIR)/mcp-curl) || true
	@echo "All MCP plugins registered."

run:
	./dist/hunter3 gateway run

# Tail all MCP server logs
tail_logs:
	@echo "Tailing all MCP server logs (Ctrl+C to stop)..."
	@tail -f ~/.hunter3/logs/mcp-*.log

# Git add, commit, and push
git-push:
	@if [ -z "$(MSG)" ]; then \
		echo "Error: Commit message required. Usage: make gacp MSG='your commit message'"; \
		exit 1; \
	fi
	git add -A
	git commit -m "$(MSG)"
	git push

# Git revert the last commit
git-revert:
	@echo "Reverting last commit..."
	git revert HEAD --no-edit
	@echo "Last commit reverted successfully"

vendor:
	go mod vendor
	go mod tidy

# Docker targets
docker-login:
	docker login

docker-build:
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	@if [ "$(DOCKER_TAG)" != "latest" ]; then docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_IMAGE):latest; fi
	@echo "Built $(DOCKER_IMAGE):$(DOCKER_TAG)"

# make docker-push VERSION=1.0.0.
docker-push: docker-build
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)
	@if [ "$(DOCKER_TAG)" != "latest" ]; then docker push $(DOCKER_IMAGE):latest; fi
	@echo "Pushed $(DOCKER_IMAGE):$(DOCKER_TAG)"

terraform:
	cd deploy/terraform && cp terraform.tfvars.example terraform.tfvars && terraform init && terraform apply

ansible:
	cd deploy/ansible && ansible-playbook -i inventory.ini playbook.yml -e github_token=$(GITHUB_TOKEN) -e irc_server=irc.h4ks.com
