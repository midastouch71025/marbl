# Acceptance Test Framework

This directory contains black-box acceptance tests for implementations built from `Plan.md` and `Takehome-Golang-Task.pdf`.

The tests are intentionally independent from the project source layout except for the public files and commands promised by the plan. They are meant to be run after another agent has scaffolded or changed the Go project.

## Run Static Acceptance Checks

These checks do not start Docker or call service endpoints:

```sh
python3 -m unittest tests/acceptance/test_acceptance.py
```

They verify the expected scaffold, Dockerization files, Makefile targets, migrations, OpenAPI docs, and README/demo coverage.

## Run Command Checks

These checks run Go/Make commands and can take longer:

```sh
RUN_COMMAND_ACCEPTANCE=1 python3 -m unittest tests/acceptance/test_acceptance.py
```

They run `go test ./...`, `make build`, and the version binaries if they exist after the build.

## Run HTTP Checks

Start the stack first, then run:

```sh
RUN_HTTP_ACCEPTANCE=1 python3 -m unittest tests/acceptance/test_acceptance.py
```

Default endpoint assumptions can be overridden:

- `PRODUCER_METRICS_URL`, default `http://localhost:9101/metrics`
- `CONSUMER_METRICS_URL`, default `http://localhost:9102/metrics`
- `CONSUMER_TASKS_URL`, default `http://localhost:8080/tasks`
- `SWAGGER_URL`, default `http://localhost:8080/swagger.yaml`
- `PROMETHEUS_READY_URL`, default `http://localhost:9090/-/ready`
- `GRAFANA_HEALTH_URL`, default `http://localhost:3000/api/health`

## Run Full Suite

```sh
RUN_COMMAND_ACCEPTANCE=1 RUN_HTTP_ACCEPTANCE=1 python3 -m unittest tests/acceptance/test_acceptance.py
```

The framework does not call destructive cleanup commands by itself. Bring the stack up/down using the implementation's documented `make up-*` and `make down` targets.
