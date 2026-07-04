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
