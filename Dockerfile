# syntax=docker/dockerfile:1
ARG GO_VERSION=1.22
FROM golang:${GO_VERSION}-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG PGO_PROFILE=
ENV CGO_ENABLED=0

# Optional PGO: copy CPU profile into each main package so `go build -pgo=auto` picks it up.
RUN set -eu; \
  if [ -n "${PGO_PROFILE}" ]; then \
    if [ ! -f "/src/${PGO_PROFILE}" ]; then \
      echo "PGO_PROFILE=${PGO_PROFILE} not found under /src" >&2; exit 1; \
    fi; \
    for d in cmd/producer cmd/consumer cmd/migrate; do \
      cp "/src/${PGO_PROFILE}" "${d}/default.pgo"; \
    done; \
    echo "PGO: using -pgo=auto with ${PGO_PROFILE} copied to cmd/*/default.pgo"; \
  fi

RUN set -eu; \
  LDFLAGS="-s -w -X main.version=${VERSION}"; \
  if [ -f cmd/producer/default.pgo ]; then \
    PGO_FLAG="-pgo=auto"; \
  else \
    PGO_FLAG=""; \
  fi; \
  go build -trimpath -ldflags="${LDFLAGS}" ${PGO_FLAG} -o /out/producer ./cmd/producer; \
  go build -trimpath -ldflags="${LDFLAGS}" ${PGO_FLAG} -o /out/consumer ./cmd/consumer; \
  go build -trimpath -ldflags="${LDFLAGS}" ${PGO_FLAG} -o /out/migrate ./cmd/migrate

FROM gcr.io/distroless/static-debian12:nonroot AS producer
COPY --from=builder /out/producer /producer
USER nonroot:nonroot
ENTRYPOINT ["/producer"]

FROM gcr.io/distroless/static-debian12:nonroot AS consumer
COPY --from=builder /out/consumer /consumer
USER nonroot:nonroot
ENTRYPOINT ["/consumer"]

FROM gcr.io/distroless/static-debian12:nonroot AS migrate
COPY --from=builder /out/migrate /migrate
USER nonroot:nonroot
ENTRYPOINT ["/migrate"]
