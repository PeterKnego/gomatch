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

	events = b.match(taker, events, touch)
	if taker.qty > 0 {
		lvl := b.restOrder(taker)
		touch(taker.side, lvl)
	}
	for i, lvl := range changed {
		events = append(events, Event{Type: EvBookUpdate, Side: changedSides[i], Price: lvl.price, AggregateQty: lvl.totalQty})
	}
	return events
}

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
