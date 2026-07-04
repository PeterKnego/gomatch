# Loadgen paced-submission latency mode — design

Date: 2026-07-04
Status: approved

## Problem

`cmd/loadgen` submits orders open-loop: it pushes all orders as fast as the
ingress accepts them. The reported latency percentiles (~360ms p50 at 100k
orders) therefore measure burst queueing depth, not per-order processing
latency. To get honest latency numbers the generator must submit at a fixed
target rate below saturation.

## Interface

New flag on `cmd/loadgen`:

- `-rate N` — target submission rate in orders/sec (int).
  Default `0` keeps the existing open-loop behavior unchanged.

Existing flags (`-orders`, `-aeron-dir`, `-ingress`) are unaffected.

## Behavior (rate > 0)

- Order *i* (0-based) has a scheduled send time `start + i * (1s / rate)`.
- Before each submit, the generator spins on `c.Poll()` until the wall clock
  reaches the scheduled time. Polling while waiting keeps draining egress
  acks; burning a core is acceptable for a load generator.
- If the generator falls behind schedule it submits immediately — no orders
  are skipped and no schedule reset occurs; it catches up naturally.

## Latency measurement

- Paced mode records `submitted[id] = scheduled send time`, not the actual
  `time.Now()` at submit. Ack latency is measured from the scheduled time, so
  generator stalls are charged to the reported latency (coordinated-omission
  correction, wrk2/HdrHistogram style).
- Open-loop mode (`-rate 0`) continues to record the actual send time.

## Output

Existing two output lines are kept. When paced, the first line additionally
shows the target rate so achieved-vs-target is visible at a glance:

```
orders=100000 acked=100000 target=10000/s elapsed=10.0s rate=9998 orders/sec
ack latency p50=... p99=... p99.9=...
```

## Scope and testing

- All changes are inside `cmd/loadgen/main.go`; no client, protocol, or
  engine changes.
- No unit tests for the pacing loop (a ~10-line wall-clock loop in a
  benchmark tool). Verification: run against the local single-node cluster
  and confirm (a) elapsed ≈ orders/rate, and (b) p50 drops from ~360ms
  open-loop to sub-millisecond territory at a below-saturation rate.
