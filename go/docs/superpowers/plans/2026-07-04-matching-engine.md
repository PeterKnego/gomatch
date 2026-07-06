# gomatch Matching Engine Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A single-instrument limit order book matching engine running as a Go ClusteredService on Aeron Cluster, with SBE wire protocol, snapshot/restore, typed client, load generator, and jar-gated integration tests.

**Architecture:** Pure deterministic matching core (`engine/`, zero aeron imports) driven by a thin `ClusteredService` glue layer (`service/`) that decodes SBE ingress and routes engine events to cluster egress (exec reports to owners, trades/book updates broadcast). Spec: `docs/superpowers/specs/2026-07-04-matching-engine-design.md`.

**Tech Stack:** Go ≥1.22, `github.com/lirm/aeron-go` replaced by fork `PeterKnego/aeron-go v0.2.0`, SBE 1.38.1 golang codegen, Aeron 1.52 `ClusteredMediaDriver` (Java 21) for integration tests.

## Global Constraints

- Module `gomatch`; dependency `github.com/lirm/aeron-go` via `replace github.com/lirm/aeron-go => github.com/PeterKnego/aeron-go v0.2.0`. If Go rejects the version replace because the fork's go.mod declares the `lirm` path, fall back to a directory replace `=> ../aeron-go` and note it in the commit message.
- All prices/quantities are `int64` ticks/lots. No floats anywhere in `engine/` or `service/`.
- Determinism: no wall-clock time, no randomness, no map iteration in any code path affecting engine outputs or state. Timestamps only from `Cluster.Time()`.
- SBE schema id **901**, generated with sbe-all-**1.38.1** (`-Dsbe.target.language=golang -Dsbe.target.namespace=codecs`). Generated code is committed.
- Every task ends with `gofmt -l .` empty, `go vet ./...` clean, tests passing, one commit.
- Integration tests skip when `aeron-all-1.52.0.jar` is absent (env `AERON_ALL_JAR` overrides path), and in `-short` mode.

---

### Task 1: Repo scaffold + engine validation, resting, ids

**Files:**
- Create: `go.mod`, `.gitignore`
- Create: `engine/events.go`, `engine/order_book.go`
- Test: `engine/order_book_test.go`

**Interfaces:**
- Produces: `engine.Side` (`Buy`=0/`Sell`=1), `engine.EventType` (`EvAccepted`, `EvRejected`, `EvTrade`, `EvFilled`, `EvCanceled`, `EvBookUpdate`), `engine.RejectReason` (`ReasonNone`, `ReasonBadQty`, `ReasonBadPrice`, `ReasonUnknownOrder`, `ReasonNotOwner`), flat struct `engine.Event`, `engine.NewOrderCmd{ClientOrderId, Owner int64; Side Side; Price, Qty int64}`, `engine.NewOrderBook() *OrderBook`, `(*OrderBook).NewLimitOrder(NewOrderCmd) []Event`.

- [ ] **Step 1: Scaffold module**

```bash
cd /home/claude/ultima/gomatch
cat > go.mod <<'EOF'
module gomatch

go 1.22

require github.com/lirm/aeron-go v0.2.0

replace github.com/lirm/aeron-go => github.com/PeterKnego/aeron-go v0.2.0
EOF
cat > .gitignore <<'EOF'
*.jar
driver-*.log
EOF
```

- [ ] **Step 2: Write the failing test**

`engine/order_book_test.go`:

```go
package engine

import (
	"reflect"
	"testing"
)

func assertEvents(t *testing.T, got, want []Event) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("events mismatch\ngot:  %+v\nwant: %+v", got, want)
	}
}

func TestRejectsInvalidOrders(t *testing.T) {
	b := NewOrderBook()
	assertEvents(t, b.NewLimitOrder(NewOrderCmd{ClientOrderId: 7, Owner: 1, Side: Buy, Price: 10, Qty: 0}),
		[]Event{{Type: EvRejected, ClientOrderId: 7, Owner: 1, Reason: ReasonBadQty}})
	assertEvents(t, b.NewLimitOrder(NewOrderCmd{ClientOrderId: 8, Owner: 1, Side: Buy, Price: -5, Qty: 10}),
		[]Event{{Type: EvRejected, ClientOrderId: 8, Owner: 1, Reason: ReasonBadPrice}})
}

func TestRestingOrderAcceptedWithBookUpdate(t *testing.T) {
	b := NewOrderBook()
	got := b.NewLimitOrder(NewOrderCmd{ClientOrderId: 7, Owner: 1, Side: Buy, Price: 100, Qty: 30})
	assertEvents(t, got, []Event{
		{Type: EvAccepted, OrderId: 1, ClientOrderId: 7, Owner: 1, Side: Buy, Price: 100, Qty: 30},
		{Type: EvBookUpdate, Side: Buy, Price: 100, AggregateQty: 30},
	})
}

func TestOrderIdsIncrease(t *testing.T) {
	b := NewOrderBook()
	first := b.NewLimitOrder(NewOrderCmd{ClientOrderId: 1, Owner: 1, Side: Buy, Price: 100, Qty: 1})
	second := b.NewLimitOrder(NewOrderCmd{ClientOrderId: 2, Owner: 1, Side: Sell, Price: 200, Qty: 1})
	if first[0].OrderId != 1 || second[0].OrderId != 2 {
		t.Fatalf("expected ids 1,2 got %d,%d", first[0].OrderId, second[0].OrderId)
	}
}

func TestSameLevelAggregates(t *testing.T) {
	b := NewOrderBook()
	b.NewLimitOrder(NewOrderCmd{ClientOrderId: 1, Owner: 1, Side: Buy, Price: 100, Qty: 30})
	got := b.NewLimitOrder(NewOrderCmd{ClientOrderId: 2, Owner: 2, Side: Buy, Price: 100, Qty: 20})
	assertEvents(t, got, []Event{
		{Type: EvAccepted, OrderId: 2, ClientOrderId: 2, Owner: 2, Side: Buy, Price: 100, Qty: 20},
		{Type: EvBookUpdate, Side: Buy, Price: 100, AggregateQty: 50},
	})
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `cd /home/claude/ultima/gomatch && go test ./engine/ -v`
Expected: compile failure — types not defined.

- [ ] **Step 4: Write minimal implementation**

`engine/events.go`:

```go
// Package engine is the pure deterministic matching core: no aeron imports,
// no I/O, integer ticks only.
package engine

type Side int8

const (
	Buy  Side = 0
	Sell Side = 1
)

type EventType int8

const (
	EvAccepted EventType = iota
	EvRejected
	EvTrade
	EvFilled
	EvCanceled
	EvBookUpdate
)

type RejectReason int8

const (
	ReasonNone RejectReason = iota
	ReasonBadQty
	ReasonBadPrice
	ReasonUnknownOrder
	ReasonNotOwner
)

// Event is a flat tagged union of everything the engine can emit. Which
// fields are meaningful depends on Type:
//   EvAccepted:   OrderId, ClientOrderId, Owner, Side, Price, Qty
//   EvRejected:   OrderId (cancels), ClientOrderId (orders), Owner, Reason
//   EvTrade:      Price, Qty, MakerOrderId, TakerOrderId, MakerOwner, TakerOwner
//   EvFilled:     OrderId, ClientOrderId, Owner, Side, Price, Qty (fill), RemainingQty
//   EvCanceled:   OrderId, ClientOrderId, Owner, Side, Price, RemainingQty (qty canceled)
//   EvBookUpdate: Side, Price, AggregateQty (0 = level gone)
type Event struct {
	Type          EventType
	OrderId       int64
	ClientOrderId int64
	Owner         int64
	Side          Side
	Price         int64
	Qty           int64
	RemainingQty  int64
	Reason        RejectReason
	MakerOrderId  int64
	TakerOrderId  int64
	MakerOwner    int64
	TakerOwner    int64
	AggregateQty  int64
}
```

`engine/order_book.go`:

```go
package engine

type order struct {
	id            int64
	clientOrderId int64
	owner         int64
	side          Side
	price         int64
	qty           int64 // remaining quantity
	level         *priceLevel
	prev, next    *order // FIFO within the level
}

type priceLevel struct {
	price      int64
	totalQty   int64
	head, tail *order
}

type NewOrderCmd struct {
	ClientOrderId int64
	Owner         int64
	Side          Side
	Price         int64
	Qty           int64
}

type OrderBook struct {
	bids        []*priceLevel // sorted best-first: highest price first
	asks        []*priceLevel // sorted best-first: lowest price first
	orders      map[int64]*order
	nextOrderId int64
}

func NewOrderBook() *OrderBook {
	return &OrderBook{orders: make(map[int64]*order), nextOrderId: 1}
}

// levelIndex binary-searches levels for price. desc is true for bids.
// Returns (index, true) when found, else (insertion index, false).
func levelIndex(levels []*priceLevel, price int64, desc bool) (int, bool) {
	lo, hi := 0, len(levels)
	for lo < hi {
		mid := (lo + hi) / 2
		p := levels[mid].price
		if p == price {
			return mid, true
		}
		if desc == (p > price) {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo, false
}

func (b *OrderBook) NewLimitOrder(cmd NewOrderCmd) []Event {
	if cmd.Qty <= 0 {
		return []Event{{Type: EvRejected, ClientOrderId: cmd.ClientOrderId, Owner: cmd.Owner, Reason: ReasonBadQty}}
	}
	if cmd.Price <= 0 {
		return []Event{{Type: EvRejected, ClientOrderId: cmd.ClientOrderId, Owner: cmd.Owner, Reason: ReasonBadPrice}}
	}
	taker := &order{
		id:            b.nextOrderId,
		clientOrderId: cmd.ClientOrderId,
		owner:         cmd.Owner,
		side:          cmd.Side,
		price:         cmd.Price,
		qty:           cmd.Qty,
	}
	b.nextOrderId++
	events := []Event{{Type: EvAccepted, OrderId: taker.id, ClientOrderId: taker.clientOrderId,
		Owner: taker.owner, Side: taker.side, Price: taker.price, Qty: taker.qty}}

	// Track levels whose aggregate changed, in order of first change, so the
	// final BookUpdate batch is deterministic without map iteration.
	var changed []*priceLevel
	var changedSides []Side
	touch := func(side Side, lvl *priceLevel) {
		for _, l := range changed {
			if l == lvl {
				return
			}
		}
		changed = append(changed, lvl)
		changedSides = append(changedSides, side)
	}

	events = b.match(taker, events, touch) // added in Task 2; no-op until then
	if taker.qty > 0 {
		lvl := b.restOrder(taker)
		touch(taker.side, lvl)
	}
	for i, lvl := range changed {
		events = append(events, Event{Type: EvBookUpdate, Side: changedSides[i], Price: lvl.price, AggregateQty: lvl.totalQty})
	}
	return events
}

// match is implemented in Task 2.
func (b *OrderBook) match(taker *order, events []Event, touch func(Side, *priceLevel)) []Event {
	return events
}

func (b *OrderBook) restOrder(o *order) *priceLevel {
	levels, desc := &b.asks, false
	if o.side == Buy {
		levels, desc = &b.bids, true
	}
	idx, found := levelIndex(*levels, o.price, desc)
	var lvl *priceLevel
	if found {
		lvl = (*levels)[idx]
	} else {
		lvl = &priceLevel{price: o.price}
		*levels = append(*levels, nil)
		copy((*levels)[idx+1:], (*levels)[idx:])
		(*levels)[idx] = lvl
	}
	o.level = lvl
	if lvl.tail == nil {
		lvl.head, lvl.tail = o, o
	} else {
		o.prev = lvl.tail
		lvl.tail.next = o
		lvl.tail = o
	}
	lvl.totalQty += o.qty
	b.orders[o.id] = o
	return lvl
}
```

- [ ] **Step 5: Run tests, verify pass, commit**

Run: `go test ./engine/ -v` — expected: all PASS. `go vet ./...`, `gofmt -l .` empty.

```bash
git add go.mod .gitignore engine/
git commit -m "engine: order book scaffold - validation, resting, ids, book updates"
```

---

### Task 2: Matching — single level, full and partial fills

**Files:**
- Modify: `engine/order_book.go` (replace the `match` stub)
- Test: `engine/matching_test.go`

**Interfaces:**
- Consumes: Task 1 types.
- Produces: working `match` — crossing orders trade at the **maker (resting) price**; per fill the event order is `EvTrade`, `EvFilled` (maker), `EvFilled` (taker); a buy crosses when `bestAsk.price <= taker.price`, a sell when `bestBid.price >= taker.price`.

- [ ] **Step 1: Write the failing test**

`engine/matching_test.go`:

```go
package engine

import "testing"

func TestFullFillSingleLevel(t *testing.T) {
	b := NewOrderBook()
	b.NewLimitOrder(NewOrderCmd{ClientOrderId: 10, Owner: 1, Side: Sell, Price: 100, Qty: 50}) // id 1 rests
	got := b.NewLimitOrder(NewOrderCmd{ClientOrderId: 20, Owner: 2, Side: Buy, Price: 100, Qty: 50})
	assertEvents(t, got, []Event{
		{Type: EvAccepted, OrderId: 2, ClientOrderId: 20, Owner: 2, Side: Buy, Price: 100, Qty: 50},
		{Type: EvTrade, Price: 100, Qty: 50, MakerOrderId: 1, TakerOrderId: 2, MakerOwner: 1, TakerOwner: 2},
		{Type: EvFilled, OrderId: 1, ClientOrderId: 10, Owner: 1, Side: Sell, Price: 100, Qty: 50, RemainingQty: 0},
		{Type: EvFilled, OrderId: 2, ClientOrderId: 20, Owner: 2, Side: Buy, Price: 100, Qty: 50, RemainingQty: 0},
		{Type: EvBookUpdate, Side: Sell, Price: 100, AggregateQty: 0},
	})
}

func TestPartialFillRemainderRests(t *testing.T) {
	b := NewOrderBook()
	b.NewLimitOrder(NewOrderCmd{ClientOrderId: 10, Owner: 1, Side: Sell, Price: 100, Qty: 30}) // id 1
	got := b.NewLimitOrder(NewOrderCmd{ClientOrderId: 20, Owner: 2, Side: Buy, Price: 100, Qty: 50})
	assertEvents(t, got, []Event{
		{Type: EvAccepted, OrderId: 2, ClientOrderId: 20, Owner: 2, Side: Buy, Price: 100, Qty: 50},
		{Type: EvTrade, Price: 100, Qty: 30, MakerOrderId: 1, TakerOrderId: 2, MakerOwner: 1, TakerOwner: 2},
		{Type: EvFilled, OrderId: 1, ClientOrderId: 10, Owner: 1, Side: Sell, Price: 100, Qty: 30, RemainingQty: 0},
		{Type: EvFilled, OrderId: 2, ClientOrderId: 20, Owner: 2, Side: Buy, Price: 100, Qty: 30, RemainingQty: 20},
		{Type: EvBookUpdate, Side: Sell, Price: 100, AggregateQty: 0},
		{Type: EvBookUpdate, Side: Buy, Price: 100, AggregateQty: 20},
	})
}

func TestTradesAtMakerPrice(t *testing.T) {
	b := NewOrderBook()
	b.NewLimitOrder(NewOrderCmd{ClientOrderId: 10, Owner: 1, Side: Sell, Price: 100, Qty: 10}) // id 1
	got := b.NewLimitOrder(NewOrderCmd{ClientOrderId: 20, Owner: 2, Side: Buy, Price: 105, Qty: 10})
	if got[1].Type != EvTrade || got[1].Price != 100 {
		t.Fatalf("expected trade at maker price 100, got %+v", got[1])
	}
}

func TestNonCrossingOrderRests(t *testing.T) {
	b := NewOrderBook()
	b.NewLimitOrder(NewOrderCmd{ClientOrderId: 10, Owner: 1, Side: Sell, Price: 101, Qty: 10})
	got := b.NewLimitOrder(NewOrderCmd{ClientOrderId: 20, Owner: 2, Side: Buy, Price: 100, Qty: 10})
	assertEvents(t, got, []Event{
		{Type: EvAccepted, OrderId: 2, ClientOrderId: 20, Owner: 2, Side: Buy, Price: 100, Qty: 10},
		{Type: EvBookUpdate, Side: Buy, Price: 100, AggregateQty: 10},
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./engine/ -run 'Fill|Maker|NonCrossing' -v`
Expected: FAIL — the `match` stub never trades.

- [ ] **Step 3: Implement match and unlink**

Replace the `match` stub in `engine/order_book.go` and add `unlink`:

```go
func (b *OrderBook) match(taker *order, events []Event, touch func(Side, *priceLevel)) []Event {
	for taker.qty > 0 {
		opp, oppSide := &b.asks, Sell
		if taker.side == Sell {
			opp, oppSide = &b.bids, Buy
		}
		if len(*opp) == 0 {
			break
		}
		best := (*opp)[0]
		if taker.side == Buy && best.price > taker.price {
			break
		}
		if taker.side == Sell && best.price < taker.price {
			break
		}
		maker := best.head
		fill := taker.qty
		if maker.qty < fill {
			fill = maker.qty
		}
		maker.qty -= fill
		taker.qty -= fill
		best.totalQty -= fill
		touch(oppSide, best)
		events = append(events,
			Event{Type: EvTrade, Price: best.price, Qty: fill,
				MakerOrderId: maker.id, TakerOrderId: taker.id,
				MakerOwner: maker.owner, TakerOwner: taker.owner},
			Event{Type: EvFilled, OrderId: maker.id, ClientOrderId: maker.clientOrderId, Owner: maker.owner,
				Side: maker.side, Price: best.price, Qty: fill, RemainingQty: maker.qty},
			Event{Type: EvFilled, OrderId: taker.id, ClientOrderId: taker.clientOrderId, Owner: taker.owner,
				Side: taker.side, Price: best.price, Qty: fill, RemainingQty: taker.qty},
		)
		if maker.qty == 0 {
			b.unlink(maker)
		}
	}
	return events
}

// unlink removes an order from its level FIFO and the order map, and removes
// the level from its side when it becomes empty. It does not touch totalQty:
// callers account for quantity themselves.
func (b *OrderBook) unlink(o *order) {
	lvl := o.level
	if o.prev != nil {
		o.prev.next = o.next
	} else {
		lvl.head = o.next
	}
	if o.next != nil {
		o.next.prev = o.prev
	} else {
		lvl.tail = o.prev
	}
	o.prev, o.next, o.level = nil, nil, nil
	delete(b.orders, o.id)
	if lvl.head == nil {
		levels, desc := &b.asks, false
		if o.side == Buy {
			levels, desc = &b.bids, true
		}
		if idx, found := levelIndex(*levels, lvl.price, desc); found {
			*levels = append((*levels)[:idx], (*levels)[idx+1:]...)
		}
	}
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./engine/ -v` — expected: all PASS (including Task 1 tests).

- [ ] **Step 5: Commit**

```bash
git add engine/
git commit -m "engine: price-time matching with trades at maker price"
```

---

### Task 3: Matching semantics tests — sweeps and FIFO priority

**Files:**
- Test: `engine/matching_test.go` (append)

**Interfaces:** consumes Tasks 1–2 only; verification-only task (no production code expected to change — if these tests fail, fix `match` and say so in the commit).

- [ ] **Step 1: Add the tests**

Append to `engine/matching_test.go`:

```go
func TestSweepsMultipleLevels(t *testing.T) {
	b := NewOrderBook()
	b.NewLimitOrder(NewOrderCmd{ClientOrderId: 10, Owner: 1, Side: Sell, Price: 100, Qty: 30}) // id 1
	b.NewLimitOrder(NewOrderCmd{ClientOrderId: 11, Owner: 1, Side: Sell, Price: 101, Qty: 40}) // id 2
	got := b.NewLimitOrder(NewOrderCmd{ClientOrderId: 20, Owner: 2, Side: Buy, Price: 101, Qty: 100})
	assertEvents(t, got, []Event{
		{Type: EvAccepted, OrderId: 3, ClientOrderId: 20, Owner: 2, Side: Buy, Price: 101, Qty: 100},
		{Type: EvTrade, Price: 100, Qty: 30, MakerOrderId: 1, TakerOrderId: 3, MakerOwner: 1, TakerOwner: 2},
		{Type: EvFilled, OrderId: 1, ClientOrderId: 10, Owner: 1, Side: Sell, Price: 100, Qty: 30, RemainingQty: 0},
		{Type: EvFilled, OrderId: 3, ClientOrderId: 20, Owner: 2, Side: Buy, Price: 100, Qty: 30, RemainingQty: 70},
		{Type: EvTrade, Price: 101, Qty: 40, MakerOrderId: 2, TakerOrderId: 3, MakerOwner: 1, TakerOwner: 2},
		{Type: EvFilled, OrderId: 2, ClientOrderId: 11, Owner: 1, Side: Sell, Price: 101, Qty: 40, RemainingQty: 0},
		{Type: EvFilled, OrderId: 3, ClientOrderId: 20, Owner: 2, Side: Buy, Price: 101, Qty: 40, RemainingQty: 30},
		{Type: EvBookUpdate, Side: Sell, Price: 100, AggregateQty: 0},
		{Type: EvBookUpdate, Side: Sell, Price: 101, AggregateQty: 0},
		{Type: EvBookUpdate, Side: Buy, Price: 101, AggregateQty: 30},
	})
}

func TestFifoPriorityWithinLevel(t *testing.T) {
	b := NewOrderBook()
	b.NewLimitOrder(NewOrderCmd{ClientOrderId: 10, Owner: 1, Side: Sell, Price: 100, Qty: 30}) // id 1, first
	b.NewLimitOrder(NewOrderCmd{ClientOrderId: 11, Owner: 3, Side: Sell, Price: 100, Qty: 40}) // id 2, second
	got := b.NewLimitOrder(NewOrderCmd{ClientOrderId: 20, Owner: 2, Side: Buy, Price: 100, Qty: 50})
	// id 1 must fill completely before id 2 trades at all.
	assertEvents(t, got, []Event{
		{Type: EvAccepted, OrderId: 3, ClientOrderId: 20, Owner: 2, Side: Buy, Price: 100, Qty: 50},
		{Type: EvTrade, Price: 100, Qty: 30, MakerOrderId: 1, TakerOrderId: 3, MakerOwner: 1, TakerOwner: 2},
		{Type: EvFilled, OrderId: 1, ClientOrderId: 10, Owner: 1, Side: Sell, Price: 100, Qty: 30, RemainingQty: 0},
		{Type: EvFilled, OrderId: 3, ClientOrderId: 20, Owner: 2, Side: Buy, Price: 100, Qty: 30, RemainingQty: 20},
		{Type: EvTrade, Price: 100, Qty: 20, MakerOrderId: 2, TakerOrderId: 3, MakerOwner: 3, TakerOwner: 2},
		{Type: EvFilled, OrderId: 2, ClientOrderId: 11, Owner: 3, Side: Sell, Price: 100, Qty: 20, RemainingQty: 20},
		{Type: EvFilled, OrderId: 3, ClientOrderId: 20, Owner: 2, Side: Buy, Price: 100, Qty: 20, RemainingQty: 0},
		{Type: EvBookUpdate, Side: Sell, Price: 100, AggregateQty: 20},
	})
}

func TestBestPriceOrdering(t *testing.T) {
	b := NewOrderBook()
	b.NewLimitOrder(NewOrderCmd{ClientOrderId: 1, Owner: 1, Side: Sell, Price: 102, Qty: 10}) // id 1
	b.NewLimitOrder(NewOrderCmd{ClientOrderId: 2, Owner: 1, Side: Sell, Price: 100, Qty: 10}) // id 2 - better
	got := b.NewLimitOrder(NewOrderCmd{ClientOrderId: 3, Owner: 2, Side: Buy, Price: 102, Qty: 10})
	if got[1].MakerOrderId != 2 {
		t.Fatalf("expected best ask (id 2 @100) to fill first, got %+v", got[1])
	}
}
```

- [ ] **Step 2: Run, expect pass; commit**

Run: `go test ./engine/ -v` — expected: PASS with no production changes.

```bash
git add engine/matching_test.go
git commit -m "engine: sweep and FIFO priority semantics tests"
```

---

### Task 4: Cancel

**Files:**
- Modify: `engine/order_book.go` (add `Cancel`)
- Test: `engine/cancel_test.go`

**Interfaces:**
- Produces: `(*OrderBook).Cancel(orderId, requestingOwner int64) []Event` — the engine performs the ownership check.

- [ ] **Step 1: Write the failing test**

`engine/cancel_test.go`:

```go
package engine

import "testing"

func TestCancelRestingOrder(t *testing.T) {
	b := NewOrderBook()
	b.NewLimitOrder(NewOrderCmd{ClientOrderId: 10, Owner: 1, Side: Sell, Price: 100, Qty: 30}) // id 1
	got := b.Cancel(1, 1)
	assertEvents(t, got, []Event{
		{Type: EvCanceled, OrderId: 1, ClientOrderId: 10, Owner: 1, Side: Sell, Price: 100, RemainingQty: 30},
		{Type: EvBookUpdate, Side: Sell, Price: 100, AggregateQty: 0},
	})
	// Canceled order must be gone: cancel again is unknown.
	assertEvents(t, b.Cancel(1, 1),
		[]Event{{Type: EvRejected, OrderId: 1, Owner: 1, Reason: ReasonUnknownOrder}})
}

func TestCancelNotOwner(t *testing.T) {
	b := NewOrderBook()
	b.NewLimitOrder(NewOrderCmd{ClientOrderId: 10, Owner: 1, Side: Sell, Price: 100, Qty: 30}) // id 1
	assertEvents(t, b.Cancel(1, 99),
		[]Event{{Type: EvRejected, OrderId: 1, Owner: 99, Reason: ReasonNotOwner}})
}

func TestCancelLeavesRestOfLevel(t *testing.T) {
	b := NewOrderBook()
	b.NewLimitOrder(NewOrderCmd{ClientOrderId: 10, Owner: 1, Side: Sell, Price: 100, Qty: 30}) // id 1
	b.NewLimitOrder(NewOrderCmd{ClientOrderId: 11, Owner: 1, Side: Sell, Price: 100, Qty: 20}) // id 2
	got := b.Cancel(1, 1)
	assertEvents(t, got, []Event{
		{Type: EvCanceled, OrderId: 1, ClientOrderId: 10, Owner: 1, Side: Sell, Price: 100, RemainingQty: 30},
		{Type: EvBookUpdate, Side: Sell, Price: 100, AggregateQty: 20},
	})
	// id 2 must still match.
	fills := b.NewLimitOrder(NewOrderCmd{ClientOrderId: 20, Owner: 2, Side: Buy, Price: 100, Qty: 20})
	if fills[1].Type != EvTrade || fills[1].MakerOrderId != 2 {
		t.Fatalf("expected id 2 to trade, got %+v", fills[1])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./engine/ -run Cancel -v`
Expected: compile failure — `Cancel` not defined.

- [ ] **Step 3: Implement Cancel**

Append to `engine/order_book.go`:

```go
// Cancel removes a resting order. The engine owns the order map, so it
// performs the ownership check: a cancel from a non-owner is rejected.
func (b *OrderBook) Cancel(orderId, requestingOwner int64) []Event {
	o, ok := b.orders[orderId]
	if !ok {
		return []Event{{Type: EvRejected, OrderId: orderId, Owner: requestingOwner, Reason: ReasonUnknownOrder}}
	}
	if o.owner != requestingOwner {
		return []Event{{Type: EvRejected, OrderId: orderId, Owner: requestingOwner, Reason: ReasonNotOwner}}
	}
	lvl := o.level
	lvl.totalQty -= o.qty
	b.unlink(o)
	return []Event{
		{Type: EvCanceled, OrderId: o.id, ClientOrderId: o.clientOrderId, Owner: o.owner,
			Side: o.side, Price: o.price, RemainingQty: o.qty},
		{Type: EvBookUpdate, Side: o.side, Price: lvl.price, AggregateQty: lvl.totalQty},
	}
}
```

- [ ] **Step 4: Run tests, verify pass**

Run: `go test ./engine/ -v` — expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add engine/
git commit -m "engine: cancel with ownership check"
```

---

### Task 5: Snapshot, restore, determinism

**Files:**
- Create: `engine/snapshot.go`
- Test: `engine/snapshot_test.go`, `engine/determinism_test.go`

**Interfaces:**
- Produces: `(*OrderBook).Snapshot(w io.Writer) error`, `engine.RestoreOrderBook(r io.Reader) (*OrderBook, error)`. Format: LE binary — magic `uint32(0x474D5331)`, version `int32(1)`, reserved instrument id `int64(1)`, nextOrderId `int64`, order count `int32`, then per order (bids best-to-worst FIFO, then asks): id, clientOrderId, owner `int64`; side `int8`; price, qty `int64`.

- [ ] **Step 1: Write the failing tests**

`engine/snapshot_test.go`:

```go
package engine

import (
	"bytes"
	"testing"
)

func populatedBook() *OrderBook {
	b := NewOrderBook()
	b.NewLimitOrder(NewOrderCmd{ClientOrderId: 1, Owner: 1, Side: Buy, Price: 99, Qty: 10})
	b.NewLimitOrder(NewOrderCmd{ClientOrderId: 2, Owner: 2, Side: Buy, Price: 100, Qty: 20})
	b.NewLimitOrder(NewOrderCmd{ClientOrderId: 3, Owner: 1, Side: Buy, Price: 100, Qty: 5})
	b.NewLimitOrder(NewOrderCmd{ClientOrderId: 4, Owner: 3, Side: Sell, Price: 101, Qty: 7})
	b.NewLimitOrder(NewOrderCmd{ClientOrderId: 5, Owner: 3, Side: Sell, Price: 103, Qty: 9})
	return b
}

func TestSnapshotRoundTrip(t *testing.T) {
	b := populatedBook()
	var buf bytes.Buffer
	if err := b.Snapshot(&buf); err != nil {
		t.Fatal(err)
	}
	restored, err := RestoreOrderBook(&buf)
	if err != nil {
		t.Fatal(err)
	}
	var again bytes.Buffer
	if err := restored.Snapshot(&again); err != nil {
		t.Fatal(err)
	}
	var first bytes.Buffer
	if err := b.Snapshot(&first); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Bytes(), again.Bytes()) {
		t.Fatal("snapshot of restored book differs from original")
	}
}

func TestRestorePreservesIdsAndMatching(t *testing.T) {
	b := populatedBook()
	var buf bytes.Buffer
	if err := b.Snapshot(&buf); err != nil {
		t.Fatal(err)
	}
	restored, err := RestoreOrderBook(&buf)
	if err != nil {
		t.Fatal(err)
	}
	// Next id continues the sequence (populatedBook used ids 1-5).
	got := restored.NewLimitOrder(NewOrderCmd{ClientOrderId: 9, Owner: 9, Side: Sell, Price: 100, Qty: 25})
	if got[0].OrderId != 6 {
		t.Fatalf("expected next order id 6, got %d", got[0].OrderId)
	}
	// It must cross the restored best bid level (100: id 2 qty 20 then id 3 qty 5).
	if got[1].Type != EvTrade || got[1].MakerOrderId != 2 || got[1].Qty != 20 {
		t.Fatalf("expected trade with restored maker id 2 qty 20, got %+v", got[1])
	}
	// Cancels of restored orders work (map rebuilt).
	if ev := restored.Cancel(1, 1); ev[0].Type != EvCanceled {
		t.Fatalf("expected cancel of restored order to work, got %+v", ev[0])
	}
}

func TestRestoreRejectsBadHeader(t *testing.T) {
	if _, err := RestoreOrderBook(bytes.NewReader([]byte{1, 2, 3, 4, 5, 6, 7, 8})); err == nil {
		t.Fatal("expected error for bad magic")
	}
}
```

`engine/determinism_test.go`:

```go
package engine

import (
	"bytes"
	"math/rand"
	"reflect"
	"testing"
)

// The engine must be a pure function of its command sequence: the same
// stream applied to two books yields identical events and identical
// snapshot bytes. Seeded rand is fine here - it generates the *inputs*.
func TestDeterministicReplay(t *testing.T) {
	commands := make([]NewOrderCmd, 0, 2000)
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 2000; i++ {
		commands = append(commands, NewOrderCmd{
			ClientOrderId: int64(i),
			Owner:         int64(rng.Intn(5) + 1),
			Side:          Side(rng.Intn(2)),
			Price:         int64(rng.Intn(20) + 90),
			Qty:           int64(rng.Intn(50) + 1),
		})
	}
	run := func() ([]Event, []byte) {
		b := NewOrderBook()
		var events []Event
		for i, cmd := range commands {
			events = append(events, b.NewLimitOrder(cmd)...)
			if i%7 == 3 { // deterministic sprinkle of cancels
				events = append(events, b.Cancel(int64(i/2+1), cmd.Owner)...)
			}
		}
		var snap bytes.Buffer
		if err := b.Snapshot(&snap); err != nil {
			t.Fatal(err)
		}
		return events, snap.Bytes()
	}
	ev1, snap1 := run()
	ev2, snap2 := run()
	if !reflect.DeepEqual(ev1, ev2) {
		t.Fatal("event streams differ between identical runs")
	}
	if !bytes.Equal(snap1, snap2) {
		t.Fatal("snapshots differ between identical runs")
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./engine/ -run 'Snapshot|Restore|Deterministic' -v`
Expected: compile failure — `Snapshot`/`RestoreOrderBook` not defined.

- [ ] **Step 3: Implement snapshot.go**

`engine/snapshot.go`:

```go
package engine

import (
	"encoding/binary"
	"fmt"
	"io"
)

const snapshotMagic uint32 = 0x474D5331 // "GMS1"
const snapshotVersion int32 = 1

// Snapshot writes the complete book state: header, then resting orders in
// deterministic book order (bids best-to-worst FIFO, then asks).
func (b *OrderBook) Snapshot(w io.Writer) error {
	for _, v := range []any{snapshotMagic, snapshotVersion, int64(1) /* reserved instrument id */, b.nextOrderId, int32(len(b.orders))} {
		if err := binary.Write(w, binary.LittleEndian, v); err != nil {
			return err
		}
	}
	writeSide := func(levels []*priceLevel) error {
		for _, lvl := range levels {
			for o := lvl.head; o != nil; o = o.next {
				for _, v := range []any{o.id, o.clientOrderId, o.owner, int8(o.side), o.price, o.qty} {
					if err := binary.Write(w, binary.LittleEndian, v); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}
	if err := writeSide(b.bids); err != nil {
		return err
	}
	return writeSide(b.asks)
}

// RestoreOrderBook rebuilds a book from a snapshot stream. Orders are
// re-rested without matching; the id sequence continues where it left off.
func RestoreOrderBook(r io.Reader) (*OrderBook, error) {
	var magic uint32
	var version int32
	var instrument, nextOrderId int64
	var count int32
	for _, v := range []any{&magic, &version, &instrument, &nextOrderId, &count} {
		if err := binary.Read(r, binary.LittleEndian, v); err != nil {
			return nil, err
		}
	}
	if magic != snapshotMagic {
		return nil, fmt.Errorf("bad snapshot magic 0x%x", magic)
	}
	if version != snapshotVersion {
		return nil, fmt.Errorf("unsupported snapshot version %d", version)
	}
	b := NewOrderBook()
	for i := int32(0); i < count; i++ {
		o := &order{}
		var side int8
		for _, v := range []any{&o.id, &o.clientOrderId, &o.owner, &side, &o.price, &o.qty} {
			if err := binary.Read(r, binary.LittleEndian, v); err != nil {
				return nil, err
			}
		}
		o.side = Side(side)
		b.restOrder(o)
	}
	b.nextOrderId = nextOrderId
	return b, nil
}
```

- [ ] **Step 4: Run tests, verify pass**

Run: `go test ./engine/ -v` — expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add engine/
git commit -m "engine: versioned snapshot/restore and determinism test"
```

---

### Task 6: SBE wire protocol

**Files:**
- Create: `protocol/gomatch-schema.xml`, `protocol/generate.sh`
- Create (generated): `protocol/codecs/*.go`
- Test: `protocol/codecs_test.go`

**Interfaces:**
- Produces: package `gomatch/protocol/codecs` with `NewOrder{ClientOrderId int64, Side SideEnum, Price, Qty int64}` (template 1), `CancelOrder{OrderId int64}` (template 2), `ExecutionReport{OrderId, ClientOrderId int64, Status OrderStatusEnum, Reason RejectReasonEnum, Side SideEnum, Price, Qty, RemainingQty, Timestamp int64}` (template 10), `TradeEvent{Price, Qty, MakerOrderId, TakerOrderId, Timestamp int64}` (template 11), `BookUpdate{Side SideEnum, Price, AggregateQty, Timestamp int64}` (template 12), `MessageHeader`, `NewSbeGoMarshaller()`. Schema id 901. Enums: `codecs.Side.BUY/SELL`, `codecs.OrderStatus.ACCEPTED/REJECTED/PARTIALLY_FILLED/FILLED/CANCELED`, `codecs.RejectReason.NONE/BAD_QTY/BAD_PRICE/UNKNOWN_ORDER/NOT_OWNER`.

- [ ] **Step 1: Write the schema**

`protocol/gomatch-schema.xml`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<sbe:messageSchema xmlns:sbe="http://fixprotocol.io/2016/sbe"
                   package="codecs" id="901" version="1"
                   semanticVersion="1.0"
                   description="gomatch order entry and market data"
                   byteOrder="littleEndian">
    <types>
        <composite name="messageHeader" description="Template identifiers and length of message root">
            <type name="blockLength" primitiveType="uint16"/>
            <type name="templateId" primitiveType="uint16"/>
            <type name="schemaId" primitiveType="uint16"/>
            <type name="version" primitiveType="uint16"/>
        </composite>
        <enum name="Side" encodingType="int8">
            <validValue name="BUY">0</validValue>
            <validValue name="SELL">1</validValue>
        </enum>
        <enum name="OrderStatus" encodingType="int8">
            <validValue name="ACCEPTED">0</validValue>
            <validValue name="REJECTED">1</validValue>
            <validValue name="PARTIALLY_FILLED">2</validValue>
            <validValue name="FILLED">3</validValue>
            <validValue name="CANCELED">4</validValue>
        </enum>
        <enum name="RejectReason" encodingType="int8">
            <validValue name="NONE">0</validValue>
            <validValue name="BAD_QTY">1</validValue>
            <validValue name="BAD_PRICE">2</validValue>
            <validValue name="UNKNOWN_ORDER">3</validValue>
            <validValue name="NOT_OWNER">4</validValue>
        </enum>
    </types>

    <sbe:message name="NewOrder" id="1" description="Client submits a limit order">
        <field name="clientOrderId" id="1" type="int64"/>
        <field name="side" id="2" type="Side"/>
        <field name="price" id="3" type="int64"/>
        <field name="qty" id="4" type="int64"/>
    </sbe:message>

    <sbe:message name="CancelOrder" id="2" description="Client cancels a resting order by engine order id">
        <field name="orderId" id="1" type="int64"/>
    </sbe:message>

    <sbe:message name="ExecutionReport" id="10" description="Order lifecycle event to the owning session">
        <field name="orderId" id="1" type="int64"/>
        <field name="clientOrderId" id="2" type="int64"/>
        <field name="status" id="3" type="OrderStatus"/>
        <field name="reason" id="4" type="RejectReason"/>
        <field name="side" id="5" type="Side"/>
        <field name="price" id="6" type="int64"/>
        <field name="qty" id="7" type="int64"/>
        <field name="remainingQty" id="8" type="int64"/>
        <field name="timestamp" id="9" type="int64"/>
    </sbe:message>

    <sbe:message name="TradeEvent" id="11" description="Trade broadcast to all sessions">
        <field name="price" id="1" type="int64"/>
        <field name="qty" id="2" type="int64"/>
        <field name="makerOrderId" id="3" type="int64"/>
        <field name="takerOrderId" id="4" type="int64"/>
        <field name="timestamp" id="5" type="int64"/>
    </sbe:message>

    <sbe:message name="BookUpdate" id="12" description="Aggregate quantity change at a price level, broadcast">
        <field name="side" id="1" type="Side"/>
        <field name="price" id="2" type="int64"/>
        <field name="aggregateQty" id="3" type="int64"/>
        <field name="timestamp" id="4" type="int64"/>
    </sbe:message>
</sbe:messageSchema>
```

`protocol/generate.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
JAR="${SBE_JAR:-$DIR/sbe-all-1.38.1.jar}"
if [ ! -f "$JAR" ]; then
  curl -fL -o "$JAR" https://repo1.maven.org/maven2/uk/co/real-logic/sbe-all/1.38.1/sbe-all-1.38.1.jar
fi
java -Dsbe.output.dir="$DIR" -Dsbe.target.language=golang \
     -Dsbe.target.namespace=codecs -Dfile.encoding=UTF-8 \
     -jar "$JAR" "$DIR/gomatch-schema.xml"
```

Run: `chmod +x protocol/generate.sh && ./protocol/generate.sh`
Expected: `protocol/codecs/` appears with one .go file per message plus `SbeMarshalling.go`, `MessageHeader.go`, enum files. (A copy of the jar exists in the session scratchpad; `SBE_JAR` env avoids re-downloading.)

- [ ] **Step 2: Write the round-trip test**

`protocol/codecs_test.go`:

```go
package protocol

import (
	"bytes"
	"testing"

	"gomatch/protocol/codecs"
)

func TestNewOrderRoundTrip(t *testing.T) {
	m := codecs.NewSbeGoMarshaller()
	in := codecs.NewOrder{ClientOrderId: 42, Side: codecs.Side.SELL, Price: 101, Qty: 7}
	var buf bytes.Buffer
	if err := in.Encode(m, &buf, true); err != nil {
		t.Fatal(err)
	}
	var out codecs.NewOrder
	if err := out.Decode(m, &buf, in.SbeSchemaVersion(), in.SbeBlockLength(), true); err != nil {
		t.Fatal(err)
	}
	if out != in {
		t.Fatalf("round trip mismatch: %+v != %+v", out, in)
	}
}

func TestExecutionReportRoundTrip(t *testing.T) {
	m := codecs.NewSbeGoMarshaller()
	in := codecs.ExecutionReport{OrderId: 1, ClientOrderId: 42, Status: codecs.OrderStatus.PARTIALLY_FILLED,
		Reason: codecs.RejectReason.NONE, Side: codecs.Side.BUY, Price: 100, Qty: 30, RemainingQty: 20, Timestamp: 999}
	var buf bytes.Buffer
	if err := in.Encode(m, &buf, true); err != nil {
		t.Fatal(err)
	}
	var out codecs.ExecutionReport
	if err := out.Decode(m, &buf, in.SbeSchemaVersion(), in.SbeBlockLength(), true); err != nil {
		t.Fatal(err)
	}
	if out != in {
		t.Fatalf("round trip mismatch: %+v != %+v", out, in)
	}
}

func TestSchemaIdentity(t *testing.T) {
	if id := (&codecs.NewOrder{}).SbeSchemaId(); id != 901 {
		t.Fatalf("expected schema id 901, got %d", id)
	}
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./protocol/ -v`
Expected: PASS. If the generated struct/enum names differ from the test's expectations (SBE golang naming), fix the *test* to the generated names and update this plan's Interfaces block in the commit message.

- [ ] **Step 4: Commit (generated code included)**

```bash
git add protocol/
git commit -m "protocol: SBE schema 901 with generated codecs and round-trip tests"
```

---

### Task 7: Service egress encoding

**Files:**
- Create: `service/encoding.go`
- Test: `service/encoding_test.go`

**Interfaces:**
- Consumes: `engine.Event`, `gomatch/protocol/codecs`.
- Produces: package `gomatch/service` —
  `encodeExecutionReport(m *codecs.SbeGoMarshaller, ev engine.Event, timestamp int64) ([]byte, error)`,
  `encodeTrade(m *codecs.SbeGoMarshaller, ev engine.Event, timestamp int64) ([]byte, error)`,
  `encodeBookUpdate(m *codecs.SbeGoMarshaller, ev engine.Event, timestamp int64) ([]byte, error)`.
  Each returns a full SBE frame (messageHeader + body). Status mapping: EvAccepted→ACCEPTED, EvRejected→REJECTED, EvFilled→FILLED when RemainingQty==0 else PARTIALLY_FILLED, EvCanceled→CANCELED.

- [ ] **Step 1: Write the failing test**

`service/encoding_test.go`:

```go
package service

import (
	"bytes"
	"testing"

	"gomatch/engine"
	"gomatch/protocol/codecs"
)

// decodeFrame reads the SBE header then decodes the body with the header's
// blockLength/version, exactly as a client would.
func decodeFrame(t *testing.T, frame []byte, out interface {
	Decode(*codecs.SbeGoMarshaller, *bytes.Buffer, uint16, uint16, bool) error
}) codecs.MessageHeader {
	t.Helper()
	m := codecs.NewSbeGoMarshaller()
	buf := bytes.NewBuffer(frame)
	var hdr codecs.MessageHeader
	if err := hdr.Decode(m, buf); err != nil {
		t.Fatal(err)
	}
	if err := out.Decode(m, buf, hdr.Version, hdr.BlockLength, true); err != nil {
		t.Fatal(err)
	}
	return hdr
}

func TestEncodePartialFillExecutionReport(t *testing.T) {
	m := codecs.NewSbeGoMarshaller()
	ev := engine.Event{Type: engine.EvFilled, OrderId: 3, ClientOrderId: 20, Owner: 2,
		Side: engine.Buy, Price: 100, Qty: 30, RemainingQty: 20}
	frame, err := encodeExecutionReport(m, ev, 12345)
	if err != nil {
		t.Fatal(err)
	}
	var er codecs.ExecutionReport
	hdr := decodeFrame(t, frame, &er)
	if hdr.TemplateId != er.SbeTemplateId() {
		t.Fatalf("bad template id %d", hdr.TemplateId)
	}
	if er.Status != codecs.OrderStatus.PARTIALLY_FILLED || er.Qty != 30 || er.RemainingQty != 20 ||
		er.Timestamp != 12345 || er.OrderId != 3 || er.ClientOrderId != 20 {
		t.Fatalf("bad exec report %+v", er)
	}
}

func TestEncodeFullFillStatus(t *testing.T) {
	m := codecs.NewSbeGoMarshaller()
	ev := engine.Event{Type: engine.EvFilled, OrderId: 1, RemainingQty: 0, Qty: 30, Side: engine.Sell, Price: 100}
	frame, err := encodeExecutionReport(m, ev, 1)
	if err != nil {
		t.Fatal(err)
	}
	var er codecs.ExecutionReport
	decodeFrame(t, frame, &er)
	if er.Status != codecs.OrderStatus.FILLED {
		t.Fatalf("expected FILLED, got %v", er.Status)
	}
}

func TestEncodeTradeAndBookUpdate(t *testing.T) {
	m := codecs.NewSbeGoMarshaller()
	trade := engine.Event{Type: engine.EvTrade, Price: 100, Qty: 30, MakerOrderId: 1, TakerOrderId: 3}
	frame, err := encodeTrade(m, trade, 7)
	if err != nil {
		t.Fatal(err)
	}
	var te codecs.TradeEvent
	decodeFrame(t, frame, &te)
	if te.Price != 100 || te.Qty != 30 || te.MakerOrderId != 1 || te.TakerOrderId != 3 || te.Timestamp != 7 {
		t.Fatalf("bad trade %+v", te)
	}

	bu := engine.Event{Type: engine.EvBookUpdate, Side: engine.Sell, Price: 100, AggregateQty: 0}
	frame, err = encodeBookUpdate(m, bu, 8)
	if err != nil {
		t.Fatal(err)
	}
	var b codecs.BookUpdate
	decodeFrame(t, frame, &b)
	if b.Side != codecs.Side.SELL || b.AggregateQty != 0 || b.Timestamp != 8 {
		t.Fatalf("bad book update %+v", b)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./service/ -v`
Expected: compile failure — encode functions not defined. (If `codecs.MessageHeader.Decode`'s signature differs from the test's assumption, adapt the test helper to the generated signature.)

- [ ] **Step 3: Implement encoding.go**

`service/encoding.go`:

```go
// Package service glues the matching engine to Aeron Cluster.
package service

import (
	"bytes"

	"gomatch/engine"
	"gomatch/protocol/codecs"
)

type sbeMessage interface {
	Encode(*codecs.SbeGoMarshaller, *bytes.Buffer, bool) error
	SbeBlockLength() uint16
	SbeTemplateId() uint16
	SbeSchemaId() uint16
	SbeSchemaVersion() uint16
}

func encodeFrame(m *codecs.SbeGoMarshaller, msg sbeMessage) ([]byte, error) {
	var buf bytes.Buffer
	hdr := codecs.MessageHeader{
		BlockLength: msg.SbeBlockLength(),
		TemplateId:  msg.SbeTemplateId(),
		SchemaId:    msg.SbeSchemaId(),
		Version:     msg.SbeSchemaVersion(),
	}
	if err := hdr.Encode(m, &buf); err != nil {
		return nil, err
	}
	if err := msg.Encode(m, &buf, true); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func statusOf(ev engine.Event) codecs.OrderStatusEnum {
	switch ev.Type {
	case engine.EvAccepted:
		return codecs.OrderStatus.ACCEPTED
	case engine.EvRejected:
		return codecs.OrderStatus.REJECTED
	case engine.EvCanceled:
		return codecs.OrderStatus.CANCELED
	case engine.EvFilled:
		if ev.RemainingQty == 0 {
			return codecs.OrderStatus.FILLED
		}
		return codecs.OrderStatus.PARTIALLY_FILLED
	}
	return codecs.OrderStatus.REJECTED
}

func encodeExecutionReport(m *codecs.SbeGoMarshaller, ev engine.Event, timestamp int64) ([]byte, error) {
	return encodeFrame(m, &codecs.ExecutionReport{
		OrderId:       ev.OrderId,
		ClientOrderId: ev.ClientOrderId,
		Status:        statusOf(ev),
		Reason:        codecs.RejectReasonEnum(ev.Reason),
		Side:          codecs.SideEnum(ev.Side),
		Price:         ev.Price,
		Qty:           ev.Qty,
		RemainingQty:  ev.RemainingQty,
		Timestamp:     timestamp,
	})
}

func encodeTrade(m *codecs.SbeGoMarshaller, ev engine.Event, timestamp int64) ([]byte, error) {
	return encodeFrame(m, &codecs.TradeEvent{
		Price:        ev.Price,
		Qty:          ev.Qty,
		MakerOrderId: ev.MakerOrderId,
		TakerOrderId: ev.TakerOrderId,
		Timestamp:    timestamp,
	})
}

func encodeBookUpdate(m *codecs.SbeGoMarshaller, ev engine.Event, timestamp int64) ([]byte, error) {
	return encodeFrame(m, &codecs.BookUpdate{
		Side:         codecs.SideEnum(ev.Side),
		Price:        ev.Price,
		AggregateQty: ev.AggregateQty,
		Timestamp:    timestamp,
	})
}
```

Note: `engine.RejectReason` and `engine.Side` values were deliberately defined with the same numeric values as the SBE enums, so the direct casts are correct; the round-trip tests verify it.

- [ ] **Step 4: Run tests, verify pass**

Run: `go test ./service/ -v` — expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add service/
git commit -m "service: engine event to SBE egress frame encoding"
```

---

### Task 8: MatchingService (ClusteredService implementation)

**Files:**
- Create: `service/service.go`
- Test: `service/service_test.go`

**Interfaces:**
- Consumes: `cluster.ClusteredService`, `cluster.Cluster`, `cluster.ClientSession` interfaces from `github.com/lirm/aeron-go/cluster`; `engine`; encode functions from Task 7.
- Produces: `service.NewMatchingService() *MatchingService` implementing `cluster.ClusteredService`. Ingress dispatch on schema 901 template ids 1 (NewOrder) and 2 (CancelOrder). Routing: exec reports → owner session only; trades/book updates → all open sessions. Snapshot: engine snapshot stream in ≤1024-byte chunks; restore: concatenate all image fragments.

- [ ] **Step 1: Write the failing test**

`service/service_test.go`:

```go
package service

import (
	"bytes"
	"testing"

	"github.com/lirm/aeron-go/aeron/atomic"
	"github.com/lirm/aeron-go/aeron/idlestrategy"
	"github.com/lirm/aeron-go/aeron/logbuffer/term"
	"github.com/lirm/aeron-go/cluster"
	ccodecs "github.com/lirm/aeron-go/cluster/codecs"

	"gomatch/protocol/codecs"
)

type fakeSession struct {
	id     int64
	frames [][]byte
}

func (f *fakeSession) Id() int64                { return f.id }
func (f *fakeSession) ResponseStreamId() int32  { return 0 }
func (f *fakeSession) ResponseChannel() string  { return "" }
func (f *fakeSession) EncodedPrincipal() []byte { return nil }
func (f *fakeSession) Close()                   {}
func (f *fakeSession) Offer(b *atomic.Buffer, offset, length int32, _ term.ReservedValueSupplier) int64 {
	f.frames = append(f.frames, b.GetBytesArray(offset, length))
	return int64(length)
}

type fakeCluster struct{ now int64 }

func (f *fakeCluster) LogPosition() int64                        { return 0 }
func (f *fakeCluster) MemberId() int32                           { return 0 }
func (f *fakeCluster) Role() cluster.Role                        { return cluster.Leader }
func (f *fakeCluster) Time() int64                               { return f.now }
func (f *fakeCluster) TimeUnit() ccodecs.ClusterTimeUnitEnum     { return ccodecs.ClusterTimeUnit.MILLIS }
func (f *fakeCluster) IdleStrategy() idlestrategy.Idler          { return &idlestrategy.Busy{} }
func (f *fakeCluster) ScheduleTimer(int64, int64) bool           { return true }
func (f *fakeCluster) CancelTimer(int64) bool                    { return true }
func (f *fakeCluster) Offer(*atomic.Buffer, int32, int32) int64  { return 0 }

func ingressFrame(t *testing.T, msg sbeMessage) *atomic.Buffer {
	t.Helper()
	frame, err := encodeFrame(codecs.NewSbeGoMarshaller(), msg)
	if err != nil {
		t.Fatal(err)
	}
	return atomic.MakeBuffer(frame)
}

func templateIdsOf(t *testing.T, frames [][]byte) []uint16 {
	t.Helper()
	ids := make([]uint16, 0, len(frames))
	for _, f := range frames {
		buf := atomic.MakeBuffer(f)
		ids = append(ids, buf.GetUInt16(2))
	}
	return ids
}

func TestMatchRoutesReportsAndMarketData(t *testing.T) {
	s := NewMatchingService()
	s.OnStart(&fakeCluster{now: 1000}, nil)
	seller := &fakeSession{id: 1}
	buyer := &fakeSession{id: 2}
	watcher := &fakeSession{id: 3}
	for _, sess := range []*fakeSession{seller, buyer, watcher} {
		s.OnSessionOpen(sess, 1)
	}

	sell := ingressFrame(t, &codecs.NewOrder{ClientOrderId: 10, Side: codecs.Side.SELL, Price: 100, Qty: 50})
	s.OnSessionMessage(seller, 1, sell, 0, sell.Capacity(), nil)
	buy := ingressFrame(t, &codecs.NewOrder{ClientOrderId: 20, Side: codecs.Side.BUY, Price: 100, Qty: 50})
	s.OnSessionMessage(buyer, 2, buy, 0, buy.Capacity(), nil)

	// Seller: ACCEPTED + FILLED exec reports, plus broadcast market data.
	// Buyer: ACCEPTED + FILLED exec reports, plus broadcast market data.
	// Watcher: only broadcast market data (BookUpdate after rest, then
	// TradeEvent + BookUpdate after the match).
	erId := (&codecs.ExecutionReport{}).SbeTemplateId()
	teId := (&codecs.TradeEvent{}).SbeTemplateId()
	buId := (&codecs.BookUpdate{}).SbeTemplateId()

	wantWatcher := []uint16{buId, teId, buId}
	if got := templateIdsOf(t, watcher.frames); !equalU16(got, wantWatcher) {
		t.Fatalf("watcher frames %v want %v", got, wantWatcher)
	}
	// Engine event order per command: Accepted, Trade, Filled(maker),
	// Filled(taker), BookUpdate. Routing preserves that order per session.
	wantSeller := []uint16{erId, buId, teId, erId, buId}
	if got := templateIdsOf(t, seller.frames); !equalU16(got, wantSeller) {
		t.Fatalf("seller frames %v want %v", got, wantSeller)
	}
	wantBuyer := []uint16{buId, erId, teId, erId, buId}
	if got := templateIdsOf(t, buyer.frames); !equalU16(got, wantBuyer) {
		t.Fatalf("buyer frames %v want %v", got, wantBuyer)
	}

	// Decode the buyer's FILLED report (frame index 3) and check the payload.
	m := codecs.NewSbeGoMarshaller()
	var hdr codecs.MessageHeader
	buf := bytes.NewBuffer(buyer.frames[3])
	if err := hdr.Decode(m, buf); err != nil {
		t.Fatal(err)
	}
	var er codecs.ExecutionReport
	if err := er.Decode(m, buf, hdr.Version, hdr.BlockLength, true); err != nil {
		t.Fatal(err)
	}
	if er.Status != codecs.OrderStatus.FILLED || er.ClientOrderId != 20 || er.Qty != 50 || er.Timestamp != 2 {
		t.Fatalf("bad buyer fill %+v", er)
	}
}

func TestCancelUnknownOrderRejected(t *testing.T) {
	s := NewMatchingService()
	s.OnStart(&fakeCluster{}, nil)
	sess := &fakeSession{id: 1}
	s.OnSessionOpen(sess, 1)
	cancel := ingressFrame(t, &codecs.CancelOrder{OrderId: 99})
	s.OnSessionMessage(sess, 1, cancel, 0, cancel.Capacity(), nil)
	if len(sess.frames) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(sess.frames))
	}
}

func TestClosedSessionSkipped(t *testing.T) {
	s := NewMatchingService()
	s.OnStart(&fakeCluster{}, nil)
	a := &fakeSession{id: 1}
	s.OnSessionOpen(a, 1)
	s.OnSessionClose(a, 2, ccodecs.CloseReason.CLIENT_ACTION)
	order := ingressFrame(t, &codecs.NewOrder{ClientOrderId: 1, Side: codecs.Side.BUY, Price: 10, Qty: 1})
	// Session already closed: engine still applies the (replayed) command,
	// but nothing is offered anywhere and nothing panics.
	s.OnSessionMessage(a, 3, order, 0, order.Capacity(), nil)
	if len(a.frames) != 0 {
		t.Fatalf("expected no frames to closed session, got %d", len(a.frames))
	}
}

func TestSnapshotChunksRoundTrip(t *testing.T) {
	s := NewMatchingService()
	s.OnStart(&fakeCluster{}, nil)
	sess := &fakeSession{id: 1}
	s.OnSessionOpen(sess, 1)
	order := ingressFrame(t, &codecs.NewOrder{ClientOrderId: 1, Side: codecs.Side.BUY, Price: 10, Qty: 5})
	s.OnSessionMessage(sess, 1, order, 0, order.Capacity(), nil)

	var chunks [][]byte
	if err := s.writeSnapshot(func(chunk []byte) error {
		chunks = append(chunks, append([]byte(nil), chunk...))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var stream bytes.Buffer
	for _, c := range chunks {
		stream.Write(c)
	}
	restored := NewMatchingService()
	if err := restored.restoreSnapshot(&stream); err != nil {
		t.Fatal(err)
	}
	// The restored book must still hold order id 1 (cancel works).
	restored.OnStart(&fakeCluster{}, nil)
	sess2 := &fakeSession{id: 1}
	restored.OnSessionOpen(sess2, 1)
	cancel := ingressFrame(t, &codecs.CancelOrder{OrderId: 1})
	restored.OnSessionMessage(sess2, 2, cancel, 0, cancel.Capacity(), nil)
	m := codecs.NewSbeGoMarshaller()
	var hdr codecs.MessageHeader
	buf := bytes.NewBuffer(sess2.frames[0])
	if err := hdr.Decode(m, buf); err != nil {
		t.Fatal(err)
	}
	var er codecs.ExecutionReport
	if err := er.Decode(m, buf, hdr.Version, hdr.BlockLength, true); err != nil {
		t.Fatal(err)
	}
	if er.Status != codecs.OrderStatus.CANCELED {
		t.Fatalf("expected CANCELED after restore, got %+v", er)
	}
}

func equalU16(a, b []uint16) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./service/ -v`
Expected: compile failure — `NewMatchingService` not defined. (If `cluster.ClientSession`/`cluster.Cluster` method sets differ, adjust the fakes to the interfaces — the compiler tells you.)

- [ ] **Step 3: Implement service.go**

`service/service.go`:

```go
package service

import (
	"bytes"
	"fmt"
	"io"

	"github.com/lirm/aeron-go/aeron"
	"github.com/lirm/aeron-go/aeron/atomic"
	"github.com/lirm/aeron-go/aeron/logbuffer"
	"github.com/lirm/aeron-go/aeron/logging"
	"github.com/lirm/aeron-go/cluster"
	ccodecs "github.com/lirm/aeron-go/cluster/codecs"

	"gomatch/engine"
	"gomatch/protocol/codecs"
)

var logger = logging.MustGetLogger("gomatch")

const (
	sbeHeaderLength       = 8
	schemaId              = 901
	newOrderTemplateId    = 1
	cancelOrderTemplateId = 2
	snapshotChunkSize     = 1024
)

// MatchingService is the ClusteredService: it decodes ingress, drives the
// matching engine, and routes engine events to cluster egress.
type MatchingService struct {
	cluster    cluster.Cluster
	book       *engine.OrderBook
	sessions   map[int64]cluster.ClientSession
	sessionIds []int64 // deterministic broadcast order (insertion order)
	marshaller *codecs.SbeGoMarshaller
}

func NewMatchingService() *MatchingService {
	return &MatchingService{
		book:       engine.NewOrderBook(),
		sessions:   map[int64]cluster.ClientSession{},
		marshaller: codecs.NewSbeGoMarshaller(),
	}
}

func (s *MatchingService) OnStart(c cluster.Cluster, image aeron.Image) {
	s.cluster = c
	if image == nil {
		return
	}
	var stream bytes.Buffer
	for {
		polled := image.Poll(func(b *atomic.Buffer, offset, length int32, _ *logbuffer.Header) {
			stream.Write(b.GetBytesArray(offset, length))
		}, 64)
		if image.IsEndOfStream() || image.IsClosed() {
			break
		}
		if polled == 0 {
			c.IdleStrategy().Idle(0)
		}
	}
	if err := s.restoreSnapshot(&stream); err != nil {
		panic(fmt.Sprintf("gomatch: snapshot restore failed: %v", err))
	}
}

func (s *MatchingService) restoreSnapshot(r io.Reader) error {
	book, err := engine.RestoreOrderBook(r)
	if err != nil {
		return err
	}
	s.book = book
	return nil
}

func (s *MatchingService) OnSessionOpen(session cluster.ClientSession, timestamp int64) {
	s.sessions[session.Id()] = session
	s.sessionIds = append(s.sessionIds, session.Id())
}

func (s *MatchingService) OnSessionClose(session cluster.ClientSession, timestamp int64, _ ccodecs.CloseReasonEnum) {
	delete(s.sessions, session.Id())
	for i, id := range s.sessionIds {
		if id == session.Id() {
			s.sessionIds = append(s.sessionIds[:i], s.sessionIds[i+1:]...)
			break
		}
	}
}

func (s *MatchingService) OnSessionMessage(
	session cluster.ClientSession,
	timestamp int64,
	buffer *atomic.Buffer,
	offset int32,
	length int32,
	header *logbuffer.Header,
) {
	if length < sbeHeaderLength {
		return
	}
	blockLength := buffer.GetUInt16(offset)
	templateId := buffer.GetUInt16(offset + 2)
	msgSchemaId := buffer.GetUInt16(offset + 4)
	version := buffer.GetUInt16(offset + 6)
	if msgSchemaId != schemaId {
		logger.Errorf("unexpected schemaId=%d templateId=%d", msgSchemaId, templateId)
		return
	}
	body := &bytes.Buffer{}
	buffer.WriteBytes(body, offset+sbeHeaderLength, length-sbeHeaderLength)

	var events []engine.Event
	switch templateId {
	case newOrderTemplateId:
		msg := codecs.NewOrder{}
		if err := msg.Decode(s.marshaller, body, version, blockLength, true); err != nil {
			logger.Errorf("NewOrder decode error: %v", err)
			return
		}
		events = s.book.NewLimitOrder(engine.NewOrderCmd{
			ClientOrderId: msg.ClientOrderId,
			Owner:         session.Id(),
			Side:          engine.Side(msg.Side),
			Price:         msg.Price,
			Qty:           msg.Qty,
		})
	case cancelOrderTemplateId:
		msg := codecs.CancelOrder{}
		if err := msg.Decode(s.marshaller, body, version, blockLength, true); err != nil {
			logger.Errorf("CancelOrder decode error: %v", err)
			return
		}
		events = s.book.Cancel(msg.OrderId, session.Id())
	default:
		logger.Debugf("ignoring unknown templateId=%d", templateId)
		return
	}
	s.route(events, timestamp)
}

func (s *MatchingService) route(events []engine.Event, timestamp int64) {
	for _, ev := range events {
		switch ev.Type {
		case engine.EvAccepted, engine.EvRejected, engine.EvCanceled:
			s.sendTo(ev.Owner, mustEncode(encodeExecutionReport(s.marshaller, ev, timestamp)))
		case engine.EvFilled:
			s.sendTo(ev.Owner, mustEncode(encodeExecutionReport(s.marshaller, ev, timestamp)))
		case engine.EvTrade:
			s.broadcast(mustEncode(encodeTrade(s.marshaller, ev, timestamp)))
		case engine.EvBookUpdate:
			s.broadcast(mustEncode(encodeBookUpdate(s.marshaller, ev, timestamp)))
		}
	}
}

// mustEncode: encoding into a bytes.Buffer cannot fail for valid messages;
// treat failure as a programming error.
func mustEncode(frame []byte, err error) []byte {
	if err != nil {
		panic(err)
	}
	return frame
}

func (s *MatchingService) sendTo(sessionId int64, frame []byte) {
	if sess, ok := s.sessions[sessionId]; ok {
		s.offer(sess, frame)
	}
}

func (s *MatchingService) broadcast(frame []byte) {
	for _, id := range s.sessionIds {
		s.offer(s.sessions[id], frame)
	}
}

func (s *MatchingService) offer(sess cluster.ClientSession, frame []byte) {
	buf := atomic.MakeBuffer(frame)
	for {
		result := sess.Offer(buf, 0, buf.Capacity(), nil)
		if result >= 0 { // includes cluster.ClientSessionMockedOffer on non-leaders
			return
		}
		if result != aeron.BackPressured && result != aeron.AdminAction {
			logger.Errorf("egress offer failed - sessionId=%d result=%d", sess.Id(), result)
			return
		}
		s.cluster.IdleStrategy().Idle(0)
	}
}

func (s *MatchingService) OnTimerEvent(correlationId, timestamp int64) {}

func (s *MatchingService) writeSnapshot(emit func([]byte) error) error {
	var stream bytes.Buffer
	if err := s.book.Snapshot(&stream); err != nil {
		return err
	}
	data := stream.Bytes()
	for len(data) > 0 {
		n := snapshotChunkSize
		if len(data) < n {
			n = len(data)
		}
		if err := emit(data[:n]); err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}

func (s *MatchingService) OnTakeSnapshot(publication *aeron.Publication) {
	err := s.writeSnapshot(func(chunk []byte) error {
		buf := atomic.MakeBuffer(chunk)
		for {
			result := publication.Offer(buf, 0, buf.Capacity(), nil)
			if result >= 0 {
				return nil
			}
			if result != aeron.BackPressured && result != aeron.AdminAction {
				return fmt.Errorf("snapshot offer failed: %d", result)
			}
			s.cluster.IdleStrategy().Idle(0)
		}
	})
	if err != nil {
		logger.Errorf("snapshot failed: %v", err)
	}
}

func (s *MatchingService) OnRoleChange(role cluster.Role) {
	logger.Infof("role change: %v", role)
}

func (s *MatchingService) OnTerminate(c cluster.Cluster) {}

func (s *MatchingService) OnNewLeadershipTermEvent(
	leadershipTermId, logPosition, timestamp, termBaseLogPosition int64,
	leaderMemberId, logSessionId int32,
	timeUnit ccodecs.ClusterTimeUnitEnum, appVersion int32,
) {
}
```

- [ ] **Step 4: Run tests, verify pass**

Run: `go test ./service/ ./engine/ -v` — expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add service/
git commit -m "service: MatchingService with ingress dispatch, routing, snapshot chunks"
```

---

### Task 9: Engine node binary

**Files:**
- Create: `cmd/engine/main.go`

**Interfaces:**
- Consumes: `service.NewMatchingService`, `cluster.NewClusteredServiceAgent`.
- Produces: `gomatch-engine` binary; env: `AERON_DIR` (default `/dev/shm/aeron-<user>`), `CLUSTER_DIR` (default `/tmp/aeron-cluster`).

- [ ] **Step 1: Implement main.go**

`cmd/engine/main.go`:

```go
package main

import (
	"fmt"
	"os"

	"github.com/lirm/aeron-go/aeron"
	"github.com/lirm/aeron-go/cluster"

	"gomatch/service"
)

func main() {
	ctx := aeron.NewContext()
	if aeronDir := os.Getenv("AERON_DIR"); aeronDir != "" {
		ctx.AeronDir(aeronDir)
	} else if _, err := os.Stat("/dev/shm"); err == nil {
		ctx.AeronDir(fmt.Sprintf("/dev/shm/aeron-%s", aeron.UserName))
	}
	opts := cluster.NewOptions()
	if clusterDir := os.Getenv("CLUSTER_DIR"); clusterDir != "" {
		opts.ClusterDir = clusterDir
	}
	agent, err := cluster.NewClusteredServiceAgent(ctx, opts, service.NewMatchingService())
	if err != nil {
		panic(err)
	}
	if err := agent.StartAndRun(); err != nil {
		panic(err)
	}
}
```

- [ ] **Step 2: Build and commit**

Run: `go build ./... && go vet ./...` — expected: clean.

```bash
git add cmd/
git commit -m "cmd/engine: cluster node binary"
```

---

### Task 10: Typed client

**Files:**
- Create: `client/client.go`, `client/egress.go`
- Test: `client/egress_test.go`

**Interfaces:**
- Consumes: `github.com/lirm/aeron-go/cluster/client` (`AeronCluster`, `EgressListener`, `NewAeronCluster`, `Options`), `gomatch/protocol/codecs`.
- Produces: package `gomatch/client` —

```go
type ExecReport struct {
	OrderId, ClientOrderId int64
	Status                 codecs.OrderStatusEnum
	Reason                 codecs.RejectReasonEnum
	Side                   codecs.SideEnum
	Price, Qty, RemainingQty, Timestamp int64
}
type Trade struct{ Price, Qty, MakerOrderId, TakerOrderId, Timestamp int64 }
type Book struct {
	Side         codecs.SideEnum
	Price, AggregateQty, Timestamp int64
}
type Listener interface {
	OnExecutionReport(ExecReport)
	OnTrade(Trade)
	OnBookUpdate(Book)
}
func Connect(aeronDir, ingressEndpoints string, l Listener) (*Client, error)
func (c *Client) SubmitOrder(clientOrderId int64, side codecs.SideEnum, price, qty int64) error
func (c *Client) CancelOrder(orderId int64) error
func (c *Client) Poll() int
func (c *Client) Close()
```

- [ ] **Step 1: Write the failing test (egress decode dispatch)**

`client/egress_test.go`:

```go
package client

import (
	"bytes"
	"testing"

	"github.com/lirm/aeron-go/aeron/atomic"

	"gomatch/protocol/codecs"
)

type recording struct {
	reports []ExecReport
	trades  []Trade
	books   []Book
}

func (r *recording) OnExecutionReport(e ExecReport) { r.reports = append(r.reports, e) }
func (r *recording) OnTrade(t Trade)                { r.trades = append(r.trades, t) }
func (r *recording) OnBookUpdate(b Book)            { r.books = append(r.books, b) }

func frame(t *testing.T, msg interface {
	Encode(*codecs.SbeGoMarshaller, *bytes.Buffer, bool) error
	SbeBlockLength() uint16
	SbeTemplateId() uint16
	SbeSchemaId() uint16
	SbeSchemaVersion() uint16
}) *atomic.Buffer {
	t.Helper()
	m := codecs.NewSbeGoMarshaller()
	var buf bytes.Buffer
	hdr := codecs.MessageHeader{BlockLength: msg.SbeBlockLength(), TemplateId: msg.SbeTemplateId(),
		SchemaId: msg.SbeSchemaId(), Version: msg.SbeSchemaVersion()}
	if err := hdr.Encode(m, &buf); err != nil {
		t.Fatal(err)
	}
	if err := msg.Encode(m, &buf, true); err != nil {
		t.Fatal(err)
	}
	return atomic.MakeBuffer(buf.Bytes())
}

func TestEgressDispatch(t *testing.T) {
	rec := &recording{}
	ad := newEgressAdapter(rec)

	er := frame(t, &codecs.ExecutionReport{OrderId: 5, ClientOrderId: 42,
		Status: codecs.OrderStatus.ACCEPTED, Side: codecs.Side.BUY, Price: 100, Qty: 10, RemainingQty: 10, Timestamp: 7})
	ad.onMessage(nil, 7, er, 0, er.Capacity(), nil)
	te := frame(t, &codecs.TradeEvent{Price: 100, Qty: 10, MakerOrderId: 1, TakerOrderId: 5, Timestamp: 8})
	ad.onMessage(nil, 8, te, 0, te.Capacity(), nil)
	bu := frame(t, &codecs.BookUpdate{Side: codecs.Side.SELL, Price: 100, AggregateQty: 0, Timestamp: 9})
	ad.onMessage(nil, 9, bu, 0, bu.Capacity(), nil)

	if len(rec.reports) != 1 || rec.reports[0].ClientOrderId != 42 || rec.reports[0].Status != codecs.OrderStatus.ACCEPTED {
		t.Fatalf("bad reports %+v", rec.reports)
	}
	if len(rec.trades) != 1 || rec.trades[0].TakerOrderId != 5 {
		t.Fatalf("bad trades %+v", rec.trades)
	}
	if len(rec.books) != 1 || rec.books[0].AggregateQty != 0 {
		t.Fatalf("bad books %+v", rec.books)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./client/ -v`
Expected: compile failure — `newEgressAdapter` not defined.

- [ ] **Step 3: Implement egress.go and client.go**

`client/egress.go`:

```go
// Package client is a typed gomatch client over the Aeron Cluster client.
package client

import (
	"bytes"

	"github.com/lirm/aeron-go/aeron/atomic"
	"github.com/lirm/aeron-go/aeron/logbuffer"
	"github.com/lirm/aeron-go/aeron/logging"
	aeroncluster "github.com/lirm/aeron-go/cluster/client"

	"gomatch/protocol/codecs"
)

var logger = logging.MustGetLogger("gomatch-client")

type ExecReport struct {
	OrderId, ClientOrderId              int64
	Status                              codecs.OrderStatusEnum
	Reason                              codecs.RejectReasonEnum
	Side                                codecs.SideEnum
	Price, Qty, RemainingQty, Timestamp int64
}

type Trade struct{ Price, Qty, MakerOrderId, TakerOrderId, Timestamp int64 }

type Book struct {
	Side                           codecs.SideEnum
	Price, AggregateQty, Timestamp int64
}

type Listener interface {
	OnExecutionReport(ExecReport)
	OnTrade(Trade)
	OnBookUpdate(Book)
}

// egressAdapter decodes gomatch egress frames into typed callbacks. It also
// satisfies the cluster client's EgressListener for session-level events.
type egressAdapter struct {
	listener   Listener
	marshaller *codecs.SbeGoMarshaller
}

func newEgressAdapter(l Listener) *egressAdapter {
	return &egressAdapter{listener: l, marshaller: codecs.NewSbeGoMarshaller()}
}

func (a *egressAdapter) onMessage(
	_ *aeroncluster.AeronCluster,
	_ int64,
	buffer *atomic.Buffer,
	offset int32,
	length int32,
	_ *logbuffer.Header,
) {
	if length < 8 {
		return
	}
	blockLength := buffer.GetUInt16(offset)
	templateId := buffer.GetUInt16(offset + 2)
	version := buffer.GetUInt16(offset + 6)
	body := &bytes.Buffer{}
	buffer.WriteBytes(body, offset+8, length-8)
	switch templateId {
	case (&codecs.ExecutionReport{}).SbeTemplateId():
		var m codecs.ExecutionReport
		if err := m.Decode(a.marshaller, body, version, blockLength, true); err != nil {
			logger.Errorf("ExecutionReport decode: %v", err)
			return
		}
		a.listener.OnExecutionReport(ExecReport{OrderId: m.OrderId, ClientOrderId: m.ClientOrderId,
			Status: m.Status, Reason: m.Reason, Side: m.Side,
			Price: m.Price, Qty: m.Qty, RemainingQty: m.RemainingQty, Timestamp: m.Timestamp})
	case (&codecs.TradeEvent{}).SbeTemplateId():
		var m codecs.TradeEvent
		if err := m.Decode(a.marshaller, body, version, blockLength, true); err != nil {
			logger.Errorf("TradeEvent decode: %v", err)
			return
		}
		a.listener.OnTrade(Trade{Price: m.Price, Qty: m.Qty,
			MakerOrderId: m.MakerOrderId, TakerOrderId: m.TakerOrderId, Timestamp: m.Timestamp})
	case (&codecs.BookUpdate{}).SbeTemplateId():
		var m codecs.BookUpdate
		if err := m.Decode(a.marshaller, body, version, blockLength, true); err != nil {
			logger.Errorf("BookUpdate decode: %v", err)
			return
		}
		a.listener.OnBookUpdate(Book{Side: m.Side, Price: m.Price,
			AggregateQty: m.AggregateQty, Timestamp: m.Timestamp})
	default:
		logger.Debugf("ignoring egress templateId=%d", templateId)
	}
}
```

`client/client.go`:

```go
package client

import (
	"bytes"
	"fmt"
	"time"

	"github.com/lirm/aeron-go/aeron"
	"github.com/lirm/aeron-go/aeron/atomic"
	"github.com/lirm/aeron-go/aeron/logbuffer"
	aeroncluster "github.com/lirm/aeron-go/cluster/client"

	"gomatch/protocol/codecs"
)

type Client struct {
	ac      *aeroncluster.AeronCluster
	adapter *egressAdapter
	opts    *aeroncluster.Options
}

// clusterEgress adapts egressAdapter to the cluster client's EgressListener.
type clusterEgress struct{ a *egressAdapter }

func (c *clusterEgress) OnConnect(*aeroncluster.AeronCluster)                       {}
func (c *clusterEgress) OnDisconnect(*aeroncluster.AeronCluster, string)            {}
func (c *clusterEgress) OnNewLeader(*aeroncluster.AeronCluster, int64, int32)       {}
func (c *clusterEgress) OnError(_ *aeroncluster.AeronCluster, detail string)        { logger.Errorf("cluster error: %s", detail) }
func (c *clusterEgress) OnMessage(ac *aeroncluster.AeronCluster, timestamp int64,
	buffer *atomic.Buffer, offset int32, length int32, header *logbuffer.Header) {
	c.a.onMessage(ac, timestamp, buffer, offset, length, header)
}

// Connect connects to the cluster. ingressEndpoints example: "0=localhost:20000".
func Connect(aeronDir, ingressEndpoints string, l Listener) (*Client, error) {
	adapter := newEgressAdapter(l)
	opts := aeroncluster.NewOptions()
	opts.IngressChannel = "aeron:udp?alias=gomatch-ingress"
	opts.IngressEndpoints = ingressEndpoints
	ac, err := aeroncluster.NewAeronCluster(
		aeron.NewContext().AeronDir(aeronDir), opts, &clusterEgress{a: adapter})
	if err != nil {
		return nil, err
	}
	c := &Client{ac: ac, adapter: adapter, opts: opts}
	deadline := time.Now().Add(30 * time.Second)
	for !ac.IsConnected() {
		if time.Now().After(deadline) {
			ac.Close()
			return nil, fmt.Errorf("timed out connecting to cluster")
		}
		opts.IdleStrategy.Idle(ac.Poll())
	}
	return c, nil
}

func (c *Client) offer(msg interface {
	Encode(*codecs.SbeGoMarshaller, *bytes.Buffer, bool) error
	SbeBlockLength() uint16
	SbeTemplateId() uint16
	SbeSchemaId() uint16
	SbeSchemaVersion() uint16
}) error {
	m := codecs.NewSbeGoMarshaller()
	var buf bytes.Buffer
	hdr := codecs.MessageHeader{BlockLength: msg.SbeBlockLength(), TemplateId: msg.SbeTemplateId(),
		SchemaId: msg.SbeSchemaId(), Version: msg.SbeSchemaVersion()}
	if err := hdr.Encode(m, &buf); err != nil {
		return err
	}
	if err := msg.Encode(m, &buf, true); err != nil {
		return err
	}
	payload := atomic.MakeBuffer(buf.Bytes())
	deadline := time.Now().Add(10 * time.Second)
	for c.ac.Offer(payload, 0, payload.Capacity()) < 0 {
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out offering to cluster")
		}
		c.opts.IdleStrategy.Idle(c.ac.Poll())
	}
	return nil
}

func (c *Client) SubmitOrder(clientOrderId int64, side codecs.SideEnum, price, qty int64) error {
	return c.offer(&codecs.NewOrder{ClientOrderId: clientOrderId, Side: side, Price: price, Qty: qty})
}

func (c *Client) CancelOrder(orderId int64) error {
	return c.offer(&codecs.CancelOrder{OrderId: orderId})
}

func (c *Client) Poll() int { return c.ac.Poll() }

func (c *Client) Close() { c.ac.Close() }
```

- [ ] **Step 4: Run tests, verify pass**

Run: `go test ./client/ -v && go build ./...` — expected: PASS, clean build.

- [ ] **Step 5: Commit**

```bash
git add client/
git commit -m "client: typed gomatch client with egress dispatch"
```

---

### Task 11: Load generator

**Files:**
- Create: `cmd/loadgen/main.go`

**Interfaces:**
- Consumes: `gomatch/client`.
- Produces: `gomatch-loadgen` binary. Flags: `-orders` (default 100000), `-aeron-dir`, `-ingress` (default `0=localhost:20000`). Prints sustained orders/sec and ack latency p50/p99/p99.9.

- [ ] **Step 1: Implement main.go**

`cmd/loadgen/main.go`:

```go
// loadgen submits a deterministic mix of crossing and resting limit orders
// and reports throughput and submit-to-ack latency percentiles.
package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"sync"
	"time"

	"gomatch/client"
	"gomatch/protocol/codecs"
)

type collector struct {
	mu        sync.Mutex
	submitted map[int64]time.Time
	latencies []time.Duration
	acked     int
}

func (c *collector) OnExecutionReport(e client.ExecReport) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if t0, ok := c.submitted[e.ClientOrderId]; ok {
		c.latencies = append(c.latencies, time.Since(t0))
		delete(c.submitted, e.ClientOrderId)
		c.acked++
	}
}
func (c *collector) OnTrade(client.Trade)      {}
func (c *collector) OnBookUpdate(client.Book)  {}

func main() {
	orders := flag.Int("orders", 100000, "number of orders to submit")
	aeronDir := flag.String("aeron-dir", fmt.Sprintf("/dev/shm/aeron-%s", os.Getenv("USER")), "aeron media driver directory")
	ingress := flag.String("ingress", "0=localhost:20000", "cluster ingress endpoints")
	flag.Parse()

	col := &collector{submitted: make(map[int64]time.Time)}
	c, err := client.Connect(*aeronDir, *ingress, col)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	rng := rand.New(rand.NewSource(1))
	start := time.Now()
	for i := 0; i < *orders; i++ {
		side := codecs.Side.BUY
		if i%2 == 1 {
			side = codecs.Side.SELL
		}
		price := int64(100 + rng.Intn(5) - 2) // 98..102 straddling mid: ~half cross
		id := int64(i + 1)
		col.mu.Lock()
		col.submitted[id] = time.Now()
		col.mu.Unlock()
		if err := c.SubmitOrder(id, side, price, int64(rng.Intn(10)+1)); err != nil {
			panic(err)
		}
		c.Poll()
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		col.mu.Lock()
		done := col.acked >= *orders
		col.mu.Unlock()
		if done || time.Now().After(deadline) {
			break
		}
		c.Poll()
	}
	elapsed := time.Since(start)

	col.mu.Lock()
	defer col.mu.Unlock()
	sort.Slice(col.latencies, func(i, j int) bool { return col.latencies[i] < col.latencies[j] })
	pct := func(p float64) time.Duration {
		if len(col.latencies) == 0 {
			return 0
		}
		idx := int(p * float64(len(col.latencies)-1))
		return col.latencies[idx]
	}
	fmt.Printf("orders=%d acked=%d elapsed=%v rate=%.0f orders/sec\n",
		*orders, col.acked, elapsed, float64(col.acked)/elapsed.Seconds())
	fmt.Printf("ack latency p50=%v p99=%v p99.9=%v\n", pct(0.50), pct(0.99), pct(0.999))
}
```

- [ ] **Step 2: Build, vet, commit**

Run: `go build ./... && go vet ./...` — expected: clean.

```bash
git add cmd/loadgen/
git commit -m "cmd/loadgen: throughput and ack-latency benchmark client"
```

---

### Task 12: Integration harness + match/market-data test

**Files:**
- Create: `systest/harness.go` (adapted copy of `/home/claude/ultima/aeron-go/systests/cluster/harness.go`)
- Test: `systest/match_test.go`

**Interfaces:**
- Consumes: everything above; `github.com/google/uuid` (new dependency, `go get github.com/google/uuid`).
- Produces: `systest` package with `StartClusteredMediaDriver()`, `Shutdown()`, `Restart()`, `Stop()`, `JarAvailable()` — same API as the aeron-go harness — plus `startEngine(t, driver)` running `service.NewMatchingService` on a `ClusteredServiceAgent` goroutine (stop flag + `agent.Close()`), and `connectGomatchClient(t, driver, listener)`.

- [ ] **Step 1: Copy and adapt the harness**

```bash
cp /home/claude/ultima/aeron-go/systests/cluster/harness.go systest/harness.go
```

Edit `systest/harness.go`: change `package clustertests` → `package systest`; keep everything else (jar name `aeron-all-1.52.0.jar`, `AERON_ALL_JAR` env, pid-derived ports, per-driver log files, pinned fork thread, `ClusterTool`). Run `go get github.com/google/uuid && go mod tidy`.

Add to `systest/harness.go` (bottom):

```go
// engineRunner drives a MatchingService agent until stopped.
type engineRunner struct {
	agent *cluster.ClusteredServiceAgent
	svc   *service.MatchingService
	stop  atomic2.Bool
	done  chan struct{}
}

func startEngine(t *testing.T, driver *ClusteredMediaDriver) *engineRunner {
	t.Helper()
	opts := cluster.NewOptions()
	opts.ClusterDir = driver.ClusterDir
	svc := service.NewMatchingService()
	agent, err := cluster.NewClusteredServiceAgent(aeron.NewContext().AeronDir(driver.AeronDir), opts, svc)
	if err != nil {
		t.Fatalf("failed to create service agent: %v", err)
	}
	r := &engineRunner{agent: agent, svc: svc, done: make(chan struct{})}
	started := make(chan error, 1)
	go func() {
		defer close(r.done)
		defer func() {
			if rec := recover(); rec != nil && !r.stop.Load() {
				t.Errorf("engine agent panicked: %v", rec)
			}
		}()
		if err := agent.OnStart(); err != nil {
			started <- err
			return
		}
		started <- nil
		for !r.stop.Load() {
			agent.Idle(agent.DoWork())
		}
	}()
	select {
	case err := <-started:
		if err != nil {
			t.Fatalf("engine agent failed to start: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("timed out waiting for engine agent to join the cluster")
	}
	return r
}

func (r *engineRunner) shutdown() {
	r.stop.Store(true)
	select {
	case <-r.done:
	case <-time.After(10 * time.Second):
	}
	r.agent.Close()
}
```

(Imports to add: `sync/atomic` as `atomic2`, `testing`, `time`, `github.com/lirm/aeron-go/cluster`, `github.com/lirm/aeron-go/aeron`, `gomatch/service`.)

- [ ] **Step 2: Write the integration test**

`systest/match_test.go`:

```go
package systest

import (
	"sync"
	"testing"
	"time"

	"gomatch/client"
	"gomatch/protocol/codecs"
)

type recorder struct {
	mu      sync.Mutex
	reports []client.ExecReport
	trades  []client.Trade
	books   []client.Book
}

func (r *recorder) OnExecutionReport(e client.ExecReport) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reports = append(r.reports, e)
}
func (r *recorder) OnTrade(t client.Trade) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.trades = append(r.trades, t)
}
func (r *recorder) OnBookUpdate(b client.Book) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.books = append(r.books, b)
}

func (r *recorder) await(t *testing.T, c *client.Client, cond func() bool, what string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		c.Poll()
		r.mu.Lock()
		ok := cond()
		r.mu.Unlock()
		if ok {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

func requireCluster(t *testing.T) *ClusteredMediaDriver {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping system test in short mode")
	}
	if jar, ok := JarAvailable(); !ok {
		t.Skipf("aeron-all jar not found at %s", jar)
	}
	driver, err := StartClusteredMediaDriver()
	if err != nil {
		t.Fatalf("failed to start driver: %v", err)
	}
	return driver
}

func TestMatchAndMarketData(t *testing.T) {
	driver := requireCluster(t)
	defer driver.Stop()
	engineNode := startEngine(t, driver)
	defer engineNode.shutdown()

	sellerRec, buyerRec := &recorder{}, &recorder{}
	seller, err := client.Connect(driver.AeronDir, "0="+driver.IngressEndpoint, sellerRec)
	if err != nil {
		t.Fatal(err)
	}
	defer seller.Close()
	buyer, err := client.Connect(driver.AeronDir, "0="+driver.IngressEndpoint, buyerRec)
	if err != nil {
		t.Fatal(err)
	}
	defer buyer.Close()

	if err := seller.SubmitOrder(1, codecs.Side.SELL, 100, 50); err != nil {
		t.Fatal(err)
	}
	sellerRec.await(t, seller, func() bool { return len(sellerRec.reports) >= 1 }, "sell ack")

	if err := buyer.SubmitOrder(2, codecs.Side.BUY, 100, 50); err != nil {
		t.Fatal(err)
	}
	buyerRec.await(t, buyer, func() bool { return len(buyerRec.reports) >= 2 }, "buy ack+fill")
	sellerRec.await(t, seller, func() bool { return len(sellerRec.reports) >= 2 }, "sell fill")
	// Both parties see the trade broadcast; the seller also polls.
	buyerRec.await(t, buyer, func() bool { return len(buyerRec.trades) >= 1 }, "buyer trade")
	sellerRec.await(t, seller, func() bool { return len(sellerRec.trades) >= 1 }, "seller trade")

	buyerRec.mu.Lock()
	defer buyerRec.mu.Unlock()
	fill := buyerRec.reports[1]
	if fill.Status != codecs.OrderStatus.FILLED || fill.Qty != 50 || fill.Price != 100 {
		t.Fatalf("bad buyer fill %+v", fill)
	}
	if buyerRec.trades[0].Price != 100 || buyerRec.trades[0].Qty != 50 {
		t.Fatalf("bad trade %+v", buyerRec.trades[0])
	}
	if len(buyerRec.books) == 0 {
		t.Fatal("expected book updates broadcast to buyer")
	}
}
```

- [ ] **Step 3: Fetch the jar and run**

```bash
cp /home/claude/ultima/aeron-go/systests/cluster/aeron-all-1.52.0.jar systest/ \
  || (cd systest && curl -fLO https://repo1.maven.org/maven2/io/aeron/aeron-all/1.52.0/aeron-all-1.52.0.jar)
cd /home/claude/ultima/gomatch && go test ./systest/ -run TestMatchAndMarketData -count=1 -v -timeout 180s
```

Expected: PASS in <60s. Debug with the harness's driver log files and `ClusterTool` output on failure.

- [ ] **Step 4: Commit**

```bash
git add systest/ go.mod go.sum
git commit -m "systest: harness + end-to-end match and market data test"
```

---

### Task 13: Restart integration test + README

**Files:**
- Test: `systest/restart_test.go`
- Create: `README.md`

**Interfaces:** consumes Task 12's harness (`Shutdown`, `Restart`, `ClusterTool("snapshot")`).

- [ ] **Step 1: Write the restart test**

`systest/restart_test.go`:

```go
package systest

import (
	"testing"

	"gomatch/client"
	"gomatch/protocol/codecs"
)

// A resting order must survive snapshot + full node restart and still match.
func TestRestingOrderSurvivesRestart(t *testing.T) {
	driver := requireCluster(t)
	currentDriver := driver
	defer func() { currentDriver.Stop() }()
	engineNode := startEngine(t, driver)

	rec := &recorder{}
	seller, err := client.Connect(driver.AeronDir, "0="+driver.IngressEndpoint, rec)
	if err != nil {
		t.Fatal(err)
	}
	if err := seller.SubmitOrder(1, codecs.Side.SELL, 105, 40); err != nil {
		t.Fatal(err)
	}
	rec.await(t, seller, func() bool { return len(rec.reports) >= 1 }, "sell ack")
	orderId := rec.reports[0].OrderId

	if out, err := driver.ClusterTool("snapshot"); err != nil {
		t.Fatalf("snapshot failed: %v - %s", err, out)
	}

	seller.Close()
	engineNode.shutdown()
	driver.Shutdown()
	restarted, err := driver.Restart()
	if err != nil {
		t.Fatal(err)
	}
	currentDriver = restarted
	recoveredEngine := startEngine(t, restarted)
	defer recoveredEngine.shutdown()

	buyerRec := &recorder{}
	buyer, err := client.Connect(restarted.AeronDir, "0="+restarted.IngressEndpoint, buyerRec)
	if err != nil {
		t.Fatal(err)
	}
	defer buyer.Close()
	if err := buyer.SubmitOrder(2, codecs.Side.BUY, 105, 40); err != nil {
		t.Fatal(err)
	}
	buyerRec.await(t, buyer, func() bool { return len(buyerRec.trades) >= 1 }, "trade after restart")

	buyerRec.mu.Lock()
	defer buyerRec.mu.Unlock()
	if buyerRec.trades[0].MakerOrderId != orderId || buyerRec.trades[0].Price != 105 || buyerRec.trades[0].Qty != 40 {
		t.Fatalf("expected fill against restored order %d, got %+v", orderId, buyerRec.trades[0])
	}
}
```

- [ ] **Step 2: Run it**

Run: `go test ./systest/ -count=1 -v -timeout 300s`
Expected: both systests PASS.

- [ ] **Step 3: Write README.md**

```markdown
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
```

- [ ] **Step 4: Full suite + commit**

Run: `gofmt -l . && go vet ./... && go test ./... -count=1 -timeout 300s`
Expected: everything PASS.

```bash
git add systest/ README.md
git commit -m "systest: restart-from-snapshot test; README"
```
