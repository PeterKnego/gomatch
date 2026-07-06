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
