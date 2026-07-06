# gomatch — Clustered Matching Engine (v1 design)

A single-instrument limit order book matching engine running as a Go
`ClusteredService` on Aeron Cluster, built on the `PeterKnego/aeron-go`
fork (v0.2.0, cluster protocol schema v16, Aeron 1.52 media driver).

**Ambition:** serious prototype — production-shaped architecture and honest
benchmarks, minimal feature set.

## Scope

In v1:

- One instrument, one order book.
- Order operations: **new limit order** (rests or matches, price-time
  priority) and **cancel by engine order id**.
- Outputs: execution reports to the sessions that own the affected orders,
  plus market data (trades and per-level book updates) broadcast to **all**
  connected sessions over cluster egress.
- Snapshot/restore of complete engine state; deterministic recovery.
- Load generator client reporting throughput and end-to-end latency
  percentiles.

Explicitly excluded from v1 (future work, not designed here): market orders,
IOC/FOK, cancel-replace, multi-instrument, auto-cancel-on-disconnect,
separate market-data stream, authentication, self-trade prevention.

## Repository

New Go module (working name `gomatch`), independent of the aeron-go fork:

```
go.mod                 module gomatch; replace github.com/lirm/aeron-go => github.com/PeterKnego/aeron-go v0.2.0
engine/                pure deterministic matching core (no aeron imports)
protocol/              SBE schema + generated codecs + generate.sh
service/               ClusteredService glue
client/                typed client wrapping cluster/client
cmd/engine/            cluster node binary
cmd/loadgen/           benchmark / load generator
systest/               integration tests against a real ClusteredMediaDriver
docs/superpowers/specs/  design docs (this file)
```

## engine/ — matching core

Data structure (approach A): price levels held in two ordered structures
(bids descending, asks ascending); each level is a FIFO queue of resting
orders; `map[int64]*order` for O(1) cancel lookup. Integer arithmetic only:
`price` and `qty` are `int64` ticks/lots; no floats anywhere in the module.

API (exact signatures refined in the implementation plan):

```go
type OrderBook struct{ ... }

func NewOrderBook() *OrderBook

// Apply a new limit order. Matches against the opposite side from the top,
// price-time priority; any remainder rests. Returns the events produced.
func (b *OrderBook) NewLimitOrder(in NewOrderCmd) []Event

// Cancel a resting order. The engine owns the order map, so it performs the
// ownership check: a cancel from a non-owner is rejected.
func (b *OrderBook) Cancel(orderId int64, requestingOwner int64) []Event

func (b *OrderBook) Snapshot(w io.Writer) error
func RestoreOrderBook(r io.Reader) (*OrderBook, error)
```

Events (engine-level, aeron-free):

- `OrderAccepted{orderId, clientOrderId, owner, side, price, qty}`
- `OrderRejected{clientOrderId, owner, reason}` — reasons: bad qty (<= 0),
  bad price (<= 0), unknown order id (cancel), not owner (cancel).
- `Trade{makerOrderId, takerOrderId, makerOwner, takerOwner, price, qty}`
- `OrderCanceled{orderId, clientOrderId, owner, remainingQty}`
- `OrderFilled{orderId, clientOrderId, owner, price, qty, remainingQty}` —
  one per side per fill.
- `BookUpdate{side, price, aggregateQty}` — new total resting quantity at a
  level (0 = level removed). Emitted for every level whose aggregate
  changed.

Engine order ids are assigned from a monotonically increasing sequence owned
by the book (part of snapshot state).

Determinism rules: no wall-clock time, no randomness, no map iteration in
any code path that affects outputs or state. Given the same command
sequence, two engines produce identical event sequences and identical
snapshots (byte-for-byte).

Snapshot format: versioned header (magic, format version int32), reserved
instrument id field (always 1 in v1), next-order-id, resting order count, then orders serialized in
deterministic book order (bids best-to-worst then asks best-to-worst, FIFO
order within level): orderId, clientOrderId, owner, side, price, qty.
Restore rebuilds the book by replaying orders as rests (no matching).

## protocol/ — wire protocol

SBE schema (own XML, own schema id, generated with SBE 1.38.1 golang target,
same toolchain as aeron-go's codecs; `generate.sh` documents the command).

Ingress messages (client → cluster):

- `NewOrder{clientOrderId int64, side enum{BUY,SELL}, price int64, qty int64}`
- `CancelOrder{orderId int64}`

Egress messages (cluster → clients):

- `ExecutionReport{orderId, clientOrderId, status enum{ACCEPTED, REJECTED,
  FILLED, PARTIALLY_FILLED, CANCELED}, reason enum, price, qty,
  remainingQty, timestamp}` — sent to the owning session only.
- `TradeEvent{price, qty, makerOrderId, takerOrderId, timestamp}` —
  broadcast to all sessions.
- `BookUpdate{side, price, aggregateQty, timestamp}` — broadcast to all
  sessions.

Timestamps are `Cluster.Time()` (cluster time units), never local time.

## service/ — ClusteredService glue

- `OnSessionMessage`: dispatch on SBE template id; decode; call the engine
  (passing the session id as owner/requesting owner); route resulting
  events. Ownership checks live in the engine.
- Event routing: execution reports → owning session (`ClientSession.Offer`);
  trades and book updates → every open session. All offers use the
  back-pressure retry loop with `Cluster.IdleStrategy()` (as the echo
  service does); non-recoverable offer errors are logged and skipped.
- Sessions tracked from `OnSessionOpen`/`OnSessionClose`. Resting orders of
  a closed session remain in the book (v1 decision); their exec reports are
  dropped if the session is gone.
- `OnTakeSnapshot`: engine snapshot plus the service's order-owner records
  are all inside the engine snapshot (owner is part of order state), so the
  service writes exactly the engine snapshot stream.
- `OnStart` with image: restore engine from snapshot stream; post-snapshot
  log entries then replay through `OnSessionMessage` as usual.
- `OnTimerEvent` unused in v1.

## client/ and cmd/loadgen/

- `client/`: thin typed wrapper over `cluster/client.AeronCluster`:
  `SubmitOrder`, `CancelOrder`, callback interface for exec reports, trades,
  book updates. Owns SBE encode/decode so applications never touch codecs.
- `cmd/loadgen/`: N synthetic traders over one client connection; submits a
  configurable mix of crossing/resting limit orders and cancels; measures
  orders/sec sustained and end-to-end latency (submit → own exec report)
  with p50/p99/p99.9, reported at the end. Uses cluster time only for
  business timestamps; latency measured with local monotonic clock at the
  client (legitimate: client-side measurement).

## cmd/engine/

Bootstrap identical in shape to aeron-go's echo example: aeron dir from
`AERON_DIR` or `/dev/shm`, `cluster.NewOptions()`, cluster dir from env,
`NewClusteredServiceAgent(...)`, `StartAndRun()`.

## Testing

- **engine unit tests**: matching semantics table tests (price-time
  priority, partial fills, multi-level sweeps, cancel of resting/unknown/
  filled orders, reject paths); snapshot round-trip; determinism test (same
  pseudo-random command stream applied to two books → identical events and
  identical snapshot bytes).
- **systest/** (jar-gated like aeron-go's, skips without the jar): reuse the
  harness pattern from `aeron-go/systests/cluster` — single-node
  ClusteredMediaDriver; two clients trade and each receives correct exec
  reports and market data; snapshot → restart → book state identical
  (verified by submitting a probe order and comparing book updates);
  determinism via restart replay.
- **benchmark**: loadgen against a single-node cluster in CI-friendly short
  mode; manual longer runs for real numbers.

## Error handling summary

- Invalid commands → reject exec report with reason; engine never panics on
  input.
- Unknown template ids on ingress → logged, skipped.
- Egress back-pressure → idle-strategy retry; session gone → drop event for
  that session, log at debug.
- Snapshot restore version mismatch → fatal error at startup (fail fast).
