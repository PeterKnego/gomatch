# javamatch

Single-instrument limit order book matching engine running as a Java
ClusteredService on Aeron Cluster 1.52. Java port of
[gomatch](../go) — same SBE schema (901), same snapshot format, same
event semantics; either project's loadgen can benchmark either engine.

## Layout

- `javamatch.engine` — pure deterministic matching core
- `javamatch.protocol.codecs` — SBE codecs generated at build from
  `src/main/resources/gomatch-schema.xml`
- `javamatch.service` — ClusteredService glue
- `javamatch.client` — typed client; `javamatch.app.LoadGen` — benchmark tool
- `src/systest/java` — integration tests against an in-process single-node
  cluster

## Test

    ./gradlew test      # unit
    ./gradlew systest   # integration (in-process ClusteredMediaDriver)

## Benchmark

    ./scripts/start-cluster.sh                     # terminal 1: cluster node
    AERON_DIR=/dev/shm/aeron-$USER-bench \
      CLUSTER_DIR=/tmp/javamatch-bench-cluster/cluster \
      ./gradlew runEngine                          # terminal 2: engine
    ./gradlew runLoadGen -PappArgs="-orders 100000 -aeron-dir /dev/shm/aeron-$USER-bench"

Open-loop by default (throughput numbers; latency reflects burst queueing).
Pass `-rate N` in `appArgs` to pace submission for honest per-order latency
percentiles (coordinated-omission corrected, measured from scheduled send
time).

To benchmark the Go engine on the same cluster, run gomatch's
`cmd/engine` with the same `AERON_DIR`/`CLUSTER_DIR` instead of `runEngine`.

## Known limitations (v1)

Inherited from gomatch: if only the service container restarts while the
consensus module keeps running, sessions restored from a cluster snapshot
are not replayed through `onSessionOpen`, so those clients stop receiving
egress until they reconnect. A full node restart does not have this problem.

Design: `docs/superpowers/specs/2026-07-06-javamatch-port-design.md`.
