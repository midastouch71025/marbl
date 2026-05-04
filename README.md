# Marbl

Greenfield Go producer/consumer exercise: HTTP task delivery, Postgres persistence, observability, and demo-oriented operational assets. Core task loops are described in [Plan.md](./Plan.md); this repository ships runnable **wiring** (metrics, pprof, migrations, Swagger, compose) so you can iterate on business logic without redoing ops scaffolding.

## Quickstart

Prerequisites: Docker with Compose v2, Go 1.22+ (for local `make build` / tests), optional `sqlc` (or use `make sqlc` via Docker).

```bash
git clone https://github.com/your-org/marbl.git marbl && cd marbl

# Full stack (Postgres, Prometheus, Grafana, producer, consumer)
# Thin wrapper: `make up-all` → `docker compose --profile all up -d`
make up-all
make migrate-up        # apply embedded migrations (run after Postgres is healthy)

cp .env.example .env   # optional for local go run / tooling; compose wires env inline

# URLs (defaults below). Host publish ports are configurable via `.env` — see `.env.example`
# (`MARBL_*_HOST_PORT`). Compose loads `.env` for variable substitution on `docker compose up`.
#
# Postgres (host): localhost:${MARBL_POSTGRES_HOST_PORT:-15432}
# Grafana:         http://localhost:${MARBL_GRAFANA_HOST_PORT:-3000}  (admin / admin)
# Prometheus:      http://localhost:${MARBL_PROMETHEUS_HOST_PORT:-9090}
# Consumer API:    http://localhost:${MARBL_CONSUMER_HTTP_HOST_PORT:-8080}
# Swagger UI:      http://localhost:${MARBL_CONSUMER_HTTP_HOST_PORT:-8080}/swagger/
# OpenAPI:         http://localhost:${MARBL_CONSUMER_HTTP_HOST_PORT:-8080}/swagger.yaml
# Producer /metrics: http://localhost:${MARBL_PRODUCER_METRICS_HOST_PORT:-9091}/metrics
# Consumer /metrics: http://localhost:${MARBL_CONSUMER_METRICS_HOST_PORT:-9092}/metrics
# Producer pprof:    http://localhost:${MARBL_PRODUCER_PPROF_HOST_PORT:-6060}/debug/pprof/
# Consumer pprof:    http://localhost:${MARBL_CONSUMER_PPROF_HOST_PORT:-6061}/debug/pprof/
```

Print injected version (after local `make build-stripped VERSION=v0.1.0`):

```bash
./bin/producer -version
./bin/consumer -version
./bin/migrate -version
```

Producer OpenAPI dump (embedded spec):

```bash
./bin/producer -openapi | head
```

## Staged demo flow (PDF-style)

1. **Infra only** — database + observability, no app tasks yet.

   ```bash
   make up-infra
   make migrate-up
   ```

2. **Producer alone (Service 1)** — start backlog generation/delivery wiring against a down or absent consumer (full behavior per Plan.md once core loops land).

   ```bash
   make up-producer
   ```

3. **Add consumer (Service 2)** — drain backlog and live traffic under rate limits (when implemented).

   ```bash
   make up-consumer
   ```

4. **Logs**

   ```bash
   docker compose logs -f producer consumer
   ```

5. **Live migration demo** (Postgres must be up):

   ```bash
   make migrate-up      # adds comment column (000002)
   make migrate-status
   make migrate-down    # one step: v2 -> v1 (removes `comment` only; keeps `tasks` table)
   # Full reset to no migrations (drops schema) — use only if you mean it:
   # make migrate-down-all
   ```

Runtime SQL never references the optional `comment` column (only migrations add/remove it), so **`migrate-down` is safe while producer and consumer stay up** — verify the column with `psql`/`\d tasks`, not via application endpoints.

## Architecture (high level)

- **Producer** (`cmd/producer`): exposes `/metrics` (Prometheus registry with DB-backed `tasks_in_state` gauges refreshed from Postgres) and pprof on `PPROF_PORT`. CLI flags `-version`, `-openapi`. Hosts backlog-aware generation/delivery loops (see Plan.md).
- **Consumer** (`cmd/consumer`): HTTP API on `LISTEN_ADDR`, `/metrics` with DB-backed gauges `task_value_sum_by_type` and `tasks_done_count_by_type`, embedded Swagger UI at `/swagger/`, raw spec at `/swagger.yaml`, pprof on `PPROF_PORT`.
- **Migrate** (`cmd/migrate`): `golang-migrate` over **embedded** SQL from `db/migrations` (`//go:embed`). `migrate down` rolls back **one** version; `migrate down-all` resets the schema (library `Down()` behavior — avoid for the staged demo).
- **Persistence**: versioned SQL migrations; `sqlc` config targets `db/schema.sql` + `db/queries/tasks.sql` → `internal/persistence/db` (`make sqlc`).
- **Images**: single multi-stage [Dockerfile](./Dockerfile) with `--target producer|consumer|migrate`, `VERSION` and optional `PGO_PROFILE` (copied to `cmd/*/default.pgo`, then `-pgo=auto`), distroless `nonroot`, `-ldflags '-s -w -X main.version=...'`.

## Required demo topics

### communication

- Service-to-service communication is HTTP/1 REST: producer `POST /tasks` to consumer.
- Infra communication is Prometheus scraping producer/consumer `/metrics` endpoints.
- Operational communication is stdout/stderr structured logs (`log/slog`) collected by Docker.

### file structure

- Entrypoints: `cmd/producer`, `cmd/consumer`, `cmd/migrate`.
- Shared code: `internal/config`, `internal/producer`, `internal/consumer`, `internal/metrics`, `internal/persistence`, `internal/openapi`, `internal/serverutil`.
- Database assets: `db/migrations`, `db/schema.sql`, `db/queries/tasks.sql`.
- Ops/docs assets: `docker-compose.yml`, `prometheus.yml`, `deploy/grafana`, `Makefile`, `README.md`.

### goroutine

- Producer runs generation, delivery, and gauge refresh loops in separate goroutines.
- Consumer runs HTTP server, reaper loop, and aggregate refresh loop concurrently.
- Metrics and pprof servers are each started in goroutines with context-driven shutdown.

### channel

- Shutdown and lifecycle coordination uses context cancellation and done channels.
- `signal.NotifyContext` propagates SIGINT/SIGTERM cancellation to long-running loops.

### mutex

- The producer in-flight task set uses a mutex for safe concurrent access.
- The in-memory consumer window rate limiter uses a mutex to protect shared counters/window state.

## Compose profiles

| Profile    | Services                                      |
|-----------|------------------------------------------------|
| `infra`   | Postgres, Prometheus, Grafana                |
| `producer`| infra + producer                             |
| `consumer`| infra + producer + consumer                    |
| `all`     | infra + producer + consumer (single command) |
| `tooling` | migrate CLI (used via `make migrate-*`)      |

Wrappers: `make up-infra`, `make up-producer`, `make up-consumer`, `make up-all`.

## OpenAPI / Swagger

- Canonical embedded copy: `internal/openapi/spec.yaml` (`openapi.Spec()` in binaries; `producer -openapi` prints it).
- Review copy: [api/openapi.yaml](./api/openapi.yaml) — run `make sync-openapi` after editing the canonical spec.
- Swagger UI HTML loads JS/CSS from jsDelivr; the page itself is embedded under `internal/assets/swagger/`. For fully air-gapped demos, vendor Swagger UI assets locally and adjust `internal/assets/swagger/index.html`.

## Makefile reference

| Target            | Purpose |
|-------------------|---------|
| `up-infra`        | Postgres + Prometheus + Grafana |
| `up-producer`     | infra + producer |
| `up-consumer`     | infra + producer + consumer |
| `up-all`          | `--profile all` |
| `down`            | Tear down compose project |
| `build`           | Plain `go build` → `bin/` |
| `build-stripped`  | `-s -w` + `-X main.version=$(VERSION)` |
| `build-pgo`       | Copies `default.pgo` into `cmd/*/` + `-pgo=auto` |
| `compare-builds`  | `ls -lh bin/*` after builds |
| `sqlc`            | `docker run sqlc/sqlc generate` → `internal/persistence/db` from `db/queries/tasks.sql` |
| `migrate-up` / `migrate-down` / `migrate-down-all` / `migrate-status` | Compose `migrate` service |
| `coverage`        | `go test -race -coverprofile` + HTML |
| `test-integration` | `MARBL_INTEGRATION=1 go test ./internal/integration/...` (Docker) |
| `profile-cpu` / `profile-heap` / `profile-trace` | `curl` pprof endpoints (producer `:6060`) |
| `flamegraph`      | CPU profile → `flamegraph.svg` via `go tool pprof -svg` |
| `flamegraph-heap` | Heap snapshot → `heap-flame.svg` |
| `compose-config`  | `docker compose --profile all --profile tooling config` |

## Profiling and tracing

CPU profile (interactive):

```bash
go tool pprof http://127.0.0.1:6060/debug/pprof/profile?seconds=30
```

Heap snapshot as PNG (demo artifact):

```bash
go tool pprof -png http://127.0.0.1:6060/debug/pprof/heap > heap.png
```

Execution trace:

```bash
curl -o trace.out "http://127.0.0.1:6060/debug/pprof/trace?seconds=5"
go tool trace trace.out
```

## PGO walkthrough

1. Run a representative load (full app or benchmarks) against a binary built **without** PGO.
2. Capture a CPU profile (30s is a good default):

   ```bash
   curl -sS "http://127.0.0.1:6060/debug/pprof/profile?seconds=30" -o default.pgo
   ```

3. Rebuild with profile-guided optimization (`-pgo=auto` after copying the profile next to each `main` package — `make build-pgo` does this):

   ```bash
   export VERSION=v0.1.0
   make build-pgo
   make compare-builds
   ```

4. In Docker builds, pass the profile path **relative to the build context** (the file is copied to `cmd/*/default.pgo` and linked with `-pgo=auto`):

   ```bash
   docker compose --profile all build --build-arg VERSION=v0.1.0 --build-arg PGO_PROFILE=default.pgo
   ```

`default.pgo` must be a **valid** CPU profile; an empty file will make the Go toolchain error — collect a real profile first.

## GOGC / GOMEMLIMIT

- Lower **`GOGC`** (e.g. `50`) trades more CPU for lower RSS; higher values do the opposite.
- **`GOMEMLIMIT`** sets a soft memory cap (for example `512MiB`); pairs well with container limits. Tune while watching RSS in Grafana/cAdvisor and heap profiles from pprof.

## Trade-offs

- **HTTP/1 REST vs gRPC/queues**: simpler ops and debugging; at high throughput, connection overhead and JSON CPU cost dominate — switch transports or add batching if needed.
- **Postgres vs SQLite**: multi-service access, ENUM/CHECK constraints, and live migration story; requires connection pooling and careful indexing under load.
- **stdout logging (`slog`) vs files**: twelve-factor friendly for Docker/Kubernetes; log volume becomes a cost/retention concern at scale.
- **Docker Compose vs k8s**: fastest local demo; not a production deployment model.
- **DB-backed Prometheus gauges**: correct cross-restart totals for “current state” panels; higher DB load than counters — scrape interval vs query cost must be balanced.
- **In-memory rate limiting**: single replica only; scale-out needs Redis/etcd-backed limiter or edge rate limits.
- **PGO**: benefits depend on workload; profiles collected under synthetic load may not match production hot paths.

## Scaling and bottlenecks

- **Postgres** is the central choke point: single-writer contention on `tasks`, index design on `(state, id)`, and connection counts from producer + consumer + metrics refreshers.
- **Producer delivery** in-memory “in-flight” set does not survive multi-replica delivery — use `FOR UPDATE SKIP LOCKED` or lease columns for horizontal scale.
- **Consumer HTTP** path is single-process in this layout; add replicas behind a load balancer and sticky-less idempotent handlers (already directionally required by Plan.md).
- **Prometheus** default 5s scrape is fine for demos; high cardinality labels (per-task IDs) must be avoided.

## Repository layout (ops-focused)

```
Dockerfile                 # --target producer|consumer|migrate
docker-compose.yml         # profiles: infra, producer, consumer, all, tooling
Makefile
api/openapi.yaml           # review copy of HTTP contract
deploy/prometheus/prometheus.yml
deploy/grafana/...
db/migrations/*.sql        # golang-migrate files + embed
db/schema.sql              # sqlc schema snapshot (matches migrated DB)
db/queries/tasks.sql       # sqlc query definitions (regenerate `queries.sql.go` via `make sqlc`)
internal/openapi/spec.yaml # embedded OpenAPI (consumer + producer -openapi)
internal/assets/swagger/   # embedded Swagger UI shell
internal/persistence/db/   # sqlc output (`queries.sql.go`, models, querier) — run `make sqlc` to refresh
cmd/producer|consumer|migrate/
```

## License

[MIT](./LICENSE)
