# gomatch

Single-instrument limit order book matching engine running as a Go
ClusteredService on Aeron Cluster (fork `PeterKnego/aeron-go` v0.2.0).

## Layout

- `engine/` — pure deterministic matching core
- `protocol/` — SBE schema 901 + generated codecs (`protocol/generate.sh`)
- `service/` — ClusteredService glue
- `client/` — typed client; `cmd/loadgen/` — benchmark tool
- `systest/` — integration tests against a real Java 1.52 ClusteredMediaDriver

## Test

    go test ./engine/ ./protocol/ ./service/ ./client/     # unit, no jar
    curl -fLO --output-dir systest https://repo1.maven.org/maven2/io/aeron/aeron-all/1.52.0/aeron-all-1.52.0.jar
    go test ./systest/ -count=1 -timeout 300s               # integration

## Benchmark

Start a single-node cluster + engine (see systest harness for config), then:

    go run ./cmd/loadgen -orders 100000 -ingress 0=localhost:20000

Design: `docs/superpowers/specs/2026-07-04-matching-engine-design.md`.
