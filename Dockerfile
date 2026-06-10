# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.23
ARG DEBIAN_VERSION=bookworm
ARG NSJAIL_VERSION=3.4

# ---- Build nsjail from source ----
FROM debian:${DEBIAN_VERSION}-slim AS nsjail-builder
ARG NSJAIL_VERSION
RUN apt-get update && apt-get install -y --no-install-recommends \
        autoconf bison ca-certificates flex g++ gcc git libnl-route-3-dev \
        libprotobuf-dev libtool make pkg-config protobuf-compiler \
    && rm -rf /var/lib/apt/lists/*
RUN git clone --depth 1 --branch ${NSJAIL_VERSION} https://github.com/google/nsjail.git /src/nsjail \
    && make -C /src/nsjail \
    && install -m 0755 /src/nsjail/nsjail /usr/local/bin/nsjail

# ---- Builder / dev image (Go + linters + nsjail) ----
FROM golang:${GO_VERSION}-${DEBIAN_VERSION} AS builder
RUN apt-get update && apt-get install -y --no-install-recommends \
        libnl-route-3-200 libprotobuf32 \
    && rm -rf /var/lib/apt/lists/*
COPY --from=nsjail-builder /usr/local/bin/nsjail /usr/local/bin/nsjail
RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags="-s -w \
    -X goboxd/internal/api.Version=$(git describe --tags --always --dirty 2>/dev/null || echo dev) \
    -X goboxd/internal/api.Commit=$(git rev-parse --short HEAD 2>/dev/null || echo unknown) \
    -X goboxd/internal/api.GoVersion=$(go env GOVERSION)" \
    -o /out/goboxd ./cmd/goboxd

# ---- Runtime image ----
FROM debian:${DEBIAN_VERSION}-slim AS runtime
RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates libnl-route-3-200 libprotobuf32 \
    && rm -rf /var/lib/apt/lists/*
COPY --from=nsjail-builder /usr/local/bin/nsjail /usr/local/bin/nsjail
COPY --from=builder        /out/goboxd          /usr/local/bin/goboxd
COPY configs               ./configs
COPY scripts/lang_install  ./scripts/lang_install

# Install all language toolchains. Each script must exit 1 on failure.
# apt-get update runs once before the loop.
RUN apt-get update && \
    for f in ./scripts/lang_install/*.sh; do \
        echo "=== Installing: $f ===" && sh "$f" || exit 1; \
    done && \
    rm -rf /var/lib/apt/lists/*

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/goboxd"]
