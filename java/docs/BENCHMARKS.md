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

## Paced vs open-loop: which models real traffic

The two modes answer different questions, and only one of them looks like
production order flow.

- **Paced mode models real user requests.** Real submitters act on their own
  clocks — an order is sent when a strategy fires, independent of whether
  the previous ack has arrived. Paced mode reproduces that: arrivals follow
  a fixed schedule regardless of system state, and latency is measured from
  each order's *scheduled* send time (coordinated-omission corrected), which
  is the submitter's experience of the system. Read the ladder rungs as
  "the latency a user sees at N orders/sec."
- **Open-loop mode is a saturation test.** Despite the name (kept from
  benchmarking convention; queueing-theory literature would call the *paced*
  mode "open-loop arrival"), submission here is throttled only by ingress
  back-pressure, so demand permanently exceeds drain rate and a queue grows
  for the whole burst. Its throughput is the capacity ceiling; its latency
  columns are **queue-depth indicators, not request latencies** — a p50 of
  hundreds of ms means the median order sat behind ~p50 × drain-rate queued
  orders, and a *microsecond* open-loop p50 (e.g. the Java engine on the
  3-node fleet) means the engine drained as fast as the client could submit
  and no queue ever formed. Never compare an open-loop latency cell to a
  paced one.
- **Caveat in the other direction:** pacing here is perfectly uniform (one
  order every 1/rate), which is kinder than real traffic. Real order flow is
  bursty — Poisson at best, event-driven microbursts in practice — so
  real-world tails at a given *average* rate sit somewhere between the paced
  rung and short stretches of open-loop behavior.

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

---

## 3-node AWS cluster sweep (2026-07-06)

Paced rate-ladder sweep on a real 3-node Aeron Cluster fleet, run with the
`bench-infra` rig at the repo root (`bench-infra/`): 3 ×
**c7i.4xlarge** (16 vCPU, sustained networking), us-east-1, single AZ,
cluster placement group, Ubuntu 24.04, Temurin/OpenJDK 21 headless.
Every node runs the Java `ClusteredMediaDriver` (media driver + archive +
consensus module, aeron-all 1.52.0) plus one engine service container.
**The Go loadgen drives both engines from node0** (egress bound to node0's
private IP), so the client and measurement side is identical and the engine
is the only variable. Each rung submits `rate × 10` orders (~10 s); latency
is ack latency from the *scheduled* send time (coordinated-omission
corrected). One open-loop run (500k orders) closes each sweep. Fresh cluster
state per engine (log/archive/shm wiped, cluster re-formed).

| rate (ord/s) | Go p50 | Go p99 | Go p99.9 | Java p50 | Java p99 | Java p99.9 |
|---|---|---|---|---|---|---|
| 1,000 | 1.234ms | 3.224ms | 3.627ms | 1.106ms | 3.138ms | 3.650ms |
| 5,000 | 750.8µs | 1.325ms | 1.382ms | 288.9µs | 454.2µs | 510.2µs |
| 10,000 | 720.2µs | 1.291ms | 1.394ms | 254.6µs | 381.8µs | 960.5µs |
| 20,000 | 692.2µs | 1.263ms | 1.914ms | 221.4µs | 354.4µs | 1.852ms |
| 40,000 | 674.7µs | 1.278ms | 3.289ms | 204.2µs | 344.8µs | 2.686ms |
| 60,000 | 696.8µs | 2.587ms | 4.699ms | 222.7µs | 379.6µs | 4.030ms |
| 80,000 | 699.9µs | 3.432ms | 5.166ms | 212.6µs | 555.6µs | 5.040ms |
| 100,000 | 695.6µs | 4.714ms | 13.798ms | 223.3µs | 1.058ms | 6.372ms |
| 125,000 | 756.4µs | 6.905ms | 8.815ms | 222.4µs | 1.245ms | 6.241ms |
| 150,000 | 808.6µs | 11.671ms | 16.314ms | 248.7µs | 4.071ms | 12.270ms |
| open-loop | 277,200 ord/s | p50 335.7ms | p99 537.7ms | **354,285 ord/s** | p50 411.8µs | p99 17.03ms |

Both engines sustained every rung with 100% acks (no drops anywhere on the
ladder).

### Where they diverge

- **Median: everywhere, by a constant ≈ 470–560µs.** From 5k/s up, Go's p50
  sits at ~675–810µs while Java's sits at ~205–250µs (~3×). The driver
  fleet and the client are identical, so this fixed gap is the service
  container itself: the aeron-go cluster service agent's poll/idle loop adds
  per-message turnaround that the Java `ClusteredServiceContainer` does not.
  (The 1k rung is warmup-dominated for both.)
- **Tail: knee at ~60k/s (Go) vs ~100k/s (Java).** Go's p99 leaves the
  ~1.3ms floor at 60k/s (2.6ms) and reaches 11.7ms at 150k/s. Java's p99
  stays under 600µs through 80k/s, crosses 1ms around 100k/s, and reaches
  4.1ms at 150k/s — consistently ~3× lower in the upper half of the ladder.
- **Ceiling: Java +28%.** Open-loop, same 500k-order burst: 354k ord/s
  (Java) vs 277k ord/s (Go). The open-loop p50 difference is dramatic
  (412µs vs 336ms): the Java engine drains the burst nearly as fast as the
  Go client can submit it, while the Go engine builds a deep queue.

Contrast with the single-node localhost result above (engines within ~2%):
on localhost with a shared 4-core VM the bottleneck was the shared
media-driver/consensus stack, masking engine differences. On a real 3-node
fleet with dedicated cores and cross-host replication, the service-container
implementation dominates the visible latency budget, and javamatch's zero-copy
flyweight decode + reused egress buffers pull clearly ahead.

Caveats: one fleet, one 10s run per rung (no repeat/variance analysis);
single-instrument workload; both engines behind the same Java
consensus/driver infra (this benchmarks the service container + engine, not
aeron-go's driver). Raw results: `bench-infra/bench-out/20260706T103037/`
(`results-go.txt`) and `.../20260706T103347/` (`results-java.txt`) on the
control machine; reproduce with `make up && make bench-both && make destroy`
in `bench-infra/`.
