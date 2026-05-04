SHELL := /bin/bash
.DEFAULT_GOAL := help

export VERSION ?= dev
export PGO_PROFILE ?=

COMPOSE := docker compose
PROFILES_INFRA := --profile infra
PROFILES_PRODUCER := $(PROFILES_INFRA) --profile producer
PROFILES_CONSUMER := $(PROFILES_PRODUCER) --profile consumer
PROFILES_ALL := --profile all
PROFILES_TOOLING := --profile tooling

.PHONY: help up-infra up-producer up-consumer up-all down build build-stripped build-pgo compare-builds \
	sqlc sync-openapi migrate-up migrate-down migrate-down-all migrate-status coverage test-integration profile-cpu profile-heap profile-trace flamegraph flamegraph-heap compose-config

help: ## Show this help
	@echo "Marbl — common targets"
	@grep -E '^[a-zA-Z0-9_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-22s %s\n", $$1, $$2}'

up-infra: ## Postgres + Prometheus + Grafana
	$(COMPOSE) $(PROFILES_INFRA) up -d

up-producer: ## infra + producer (Service 1 alone)
	$(COMPOSE) $(PROFILES_PRODUCER) up -d

up-consumer: ## infra + producer + consumer (staged demo phase 3)
	$(COMPOSE) $(PROFILES_CONSUMER) up -d

up-all: ## Full stack (quick smoke)
	$(COMPOSE) $(PROFILES_ALL) up -d

down: ## Stop containers for this compose project
	$(COMPOSE) --profile infra --profile producer --profile consumer --profile all --profile tooling down --remove-orphans

build: ## Local dev build (no strip / PGO)
	@mkdir -p bin
	go build -o bin/producer ./cmd/producer
	go build -o bin/consumer ./cmd/consumer
	go build -o bin/migrate ./cmd/migrate

build-stripped: ## -ldflags -s -w -X main.version=$(VERSION)
	@mkdir -p bin
	go build -trimpath -ldflags="-s -w -X main.version=$(VERSION)" -o bin/producer ./cmd/producer
	go build -trimpath -ldflags="-s -w -X main.version=$(VERSION)" -o bin/consumer ./cmd/consumer
	go build -trimpath -ldflags="-s -w -X main.version=$(VERSION)" -o bin/migrate ./cmd/migrate

build-pgo: ## -pgo=auto (requires ./default.pgo; copied into cmd/*/ for each main)
	@mkdir -p bin
	@if [[ ! -f default.pgo ]]; then echo "error: default.pgo missing — collect a CPU profile first (README)"; exit 1; fi
	@for d in cmd/producer cmd/consumer cmd/migrate; do cp -f default.pgo "$$d/default.pgo"; done
	go build -trimpath -ldflags="-s -w -X main.version=$(VERSION)" -pgo=auto -o bin/producer ./cmd/producer
	go build -trimpath -ldflags="-s -w -X main.version=$(VERSION)" -pgo=auto -o bin/consumer ./cmd/consumer
	go build -trimpath -ldflags="-s -w -X main.version=$(VERSION)" -pgo=auto -o bin/migrate ./cmd/migrate

compare-builds: ## Show binary sizes under bin/
	@ls -lh bin/* 2>/dev/null || echo "(run make build / build-stripped / build-pgo first)"

sqlc: ## sqlc generate (Docker); see README for how this relates to persistence SQL
	docker run --rm -v "$$(pwd)":/src -w /src docker.io/sqlc/sqlc:latest generate

sync-openapi: ## Copy internal/openapi/spec.yaml → api/openapi.yaml
	cp internal/openapi/spec.yaml api/openapi.yaml

migrate-up: ## Apply migrations (Postgres must be reachable)
	$(COMPOSE) $(PROFILES_INFRA) $(PROFILES_TOOLING) run --rm --build migrate up

migrate-down: ## Roll back one migration step (e.g. v2 -> v1, removes comment only)
	$(COMPOSE) $(PROFILES_INFRA) $(PROFILES_TOOLING) run --rm --build migrate down

migrate-down-all: ## Roll back all migrations to nil (reset DB schema — not for the live demo)
	$(COMPOSE) $(PROFILES_INFRA) $(PROFILES_TOOLING) run --rm --build migrate down-all

migrate-status: ## Show current migration version
	$(COMPOSE) $(PROFILES_INFRA) $(PROFILES_TOOLING) run --rm --build migrate version

coverage: ## Race + HTML coverage report
	go test ./... -race -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "wrote coverage.html"

test-integration: ## Integration tests (Docker + MARBL_INTEGRATION=1)
	MARBL_INTEGRATION=1 go test ./internal/integration/... -count=1

profile-cpu: ## 15s CPU profile from producer pprof (host ports)
	curl -sS "http://127.0.0.1:6060/debug/pprof/profile?seconds=15" -o cpu.pprof || true
	@echo "wrote cpu.pprof (ensure producer is up on :6060)"

profile-heap: ## Heap snapshot (producer pprof)
	curl -sS "http://127.0.0.1:6060/debug/pprof/heap" -o heap.pprof || true
	@echo "wrote heap.pprof"

profile-trace: ## 5s execution trace
	curl -sS "http://127.0.0.1:6060/debug/pprof/trace?seconds=5" -o trace.out || true
	@echo "wrote trace.out"

flamegraph: ## CPU profile -> flamegraph.svg (requires graphviz for -svg)
	curl -sS "http://127.0.0.1:6060/debug/pprof/profile?seconds=15" -o cpu.pprof
	go tool pprof -svg cpu.pprof > flamegraph.svg
	@echo "wrote flamegraph.svg (install graphviz if dot is missing)"

flamegraph-heap: ## Heap snapshot -> heap-flame.svg (graphviz)
	curl -sS "http://127.0.0.1:6060/debug/pprof/heap" -o heap.pprof
	go tool pprof -svg heap.pprof > heap-flame.svg
	@echo "wrote heap-flame.svg"

compose-config: ## Validate compose (profiles all + tooling)
	$(COMPOSE) $(PROFILES_ALL) $(PROFILES_TOOLING) config >/dev/null && echo OK
