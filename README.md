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

Open-loop by default (throughput numbers; latency reflects burst queueing).
Pass `-rate N` to pace submission at N orders/sec for honest per-order
latency percentiles — measured from each order's *scheduled* send time, so
generator stalls count (coordinated-omission correction).

## Known limitations (v1)

- If only the service container restarts while the consensus module keeps
  running, sessions restored from a cluster snapshot are not visible to the
  service (the cluster library does not replay them through OnSessionOpen),
  so those clients stop receiving egress until they reconnect. A full node
  restart — the deployment shape v1 targets — does not have this problem.

Design: `docs/superpowers/specs/2026-07-04-matching-engine-design.md`.
