# syntax=docker/dockerfile:1.3-labs

FROM debian:bullseye-slim as base
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
RUN useradd -r -u 999 -d /home/amihan amihan

FROM ghcr.io/acorn-io/images-mirror/golang:1.20 AS build
COPY / /src
WORKDIR /src
RUN \
  --mount=type=cache,target=/go/pkg \
  --mount=type=cache,target=/root/.cache/go-build \
  go build -o bin/amihan main.go

FROM base AS goreleaser
COPY amihan /usr/local/bin/amihan
USER amihan

FROM base
COPY --from=build /src/bin/amihan /usr/local/bin/amihan
USER amihan