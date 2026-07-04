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
