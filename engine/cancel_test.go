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
