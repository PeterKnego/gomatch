# Benchmarks: Java vs Go engine

Comparison of the Java (`javamatch`) and Go (`gomatch`) matching-engine
implementations, running the identical single-node Aeron cluster infra
(`scripts/start-cluster.sh`, fixed 20000 port block) and the same
loadgen protocol on both sides. This is a wire-compat + performance sanity
check, not a rigorous benchmark — see "Machine and methodology" below.

## Results

Each loadgen invocation was run twice per engine/mode; the table reports the
**second** run only (the first run pays JIT warmup on the Java side, plus
OS/page-cache/JIT-adjacent warmup on both sides — see Methodology).
`-orders 100000` for both open-loop and paced modes; paced mode targets
20,000 orders/sec.

| Engine | Mode              | Throughput      | p50        | p99        | p99.9      |
|--------|-------------------|-----------------|------------|------------|------------|
| Java   | open-loop         | 170,885 ord/s   | 243.871 ms | 413.500 ms | 415.674 ms |
| Java   | paced (20,000/s)  | 20,000 ord/s    | 75.196 µs  | 10.242 ms  | 24.046 ms  |
| Go     | open-loop         | 174,473 ord/s   | 212.544 ms | 336.025 ms | 337.240 ms |
| Go     | paced (20,000/s)  | 20,000 ord/s    | 792.889 µs | 10.137 ms  | 22.328 ms  |

Notes on reading this table:

- **Open-loop** mode submits as fast as the client can go with no rate limit;
  throughput is the headline number, and the reported latency mostly reflects
  queueing depth built up during the burst (100k orders arrive far faster
  than the engine ack's them, so p50 latency is dominated by how far back in
  the queue the median order sits) rather than steady-state per-order cost.
  Both engines land within ~2% of each other on throughput at this order
  count.
- **Paced** mode (target 20,000 orders/sec, well under either engine's
  open-loop ceiling) gives an honest per-order latency picture with
  coordinated-omission correction (latency measured from each order's
  *scheduled* send time, so generator stalls count against the reported
  numbers). Both engines hit the 20,000/s target exactly (or within
  rounding) and post similar p99/p99.9. Java's p50 (75µs) is measurably lower
  than Go's (793µs) under pacing; both p99/p99.9 are comparable and dominated
  by scheduling jitter on this shared VM rather than either engine's
  matching-logic cost.

## Wire-compat cross-check

The Go loadgen (`cmd/loadgen`) was pointed at a running **Java** engine
(`EngineMain`, same cluster) for a short run. This cross-check reused the
still-running Java-engine cluster from the Java benchmark run above rather
than starting a fresh cluster:

```
orders=5000 acked=5000 elapsed=25.095328ms rate=199240 orders/sec
ack latency p50=11.981522ms p99=13.903918ms p99.9=14.020538ms
```

`acked=5000` confirms the Go client's SBE-encoded ingress messages and
egress decoding are wire-compatible with the Java engine's protocol codecs —
cross-language interoperability at the byte level, not just same-language
round-trips.

## Machine and methodology

- **Hardware**: 4 cores, 15 GB RAM, Linux (shared/noisy VM — not a dedicated
  benchmark box; other unrelated processes may be scheduled concurrently,
  and results should be read as "same ballpark" rather than to the second
  significant digit).
- **JVM**: Temurin 21.0.11+10 (`openjdk version "21.0.11" 2026-04-21 LTS`).
- **Go**: `go version go1.26.0 linux/amd64`.
- **Cluster**: single-node Aeron cluster (media driver + archive + consensus
  module) via `scripts/start-cluster.sh`, port block 20000-20010,
  `AERON_DIR=/dev/shm/aeron-$USER-bench`,
  `CLUSTER_DIR=/tmp/javamatch-bench-cluster/cluster`. A fresh cluster (and
  fresh aeron/cluster dirs) was started for each engine so neither run
  carried over any state or warm OS caches from the other.
- **Warmup**: each loadgen invocation (open-loop and paced) was run twice
  back-to-back against the same live engine; only the **second** run is
  reported. The first run absorbs JIT compilation on the Java side (the
  engine and loadgen JVMs start cold) and general warmup (connection
  handshake, GC housekeeping, page-cache population) on both sides. The
  first-run numbers were visibly worse for Java open-loop
  (109,935 ord/s vs. 170,885 ord/s on the second run) confirming this
  matters; Go's first/second-run delta was smaller (145,508 vs. 174,473
  ord/s) since Go has no JIT warmup, only OS/scheduling warmup.
- **Order mix**: both loadgens use the same deterministic PRNG seed (1) and
  generate the same crossing/resting limit-order mix (alternating
  buy/sell, price in [98, 102] straddling a mid of 100, quantity 1-10) —
  so both engines process an equivalent workload shape.
- **One stack at a time**: cluster + one engine + one loadgen were run
  serially (Java stack fully torn down, `/tmp/javamatch-bench-cluster` and
  `/dev/shm/aeron-$USER-bench` removed, before starting the Go stack) to
  avoid port/resource contention on this 4-core machine.

Full raw output (all four loadgen invocations per engine, PIDs, and cleanup
evidence) is recorded in `.superpowers/sdd/task-13-report.md`.
