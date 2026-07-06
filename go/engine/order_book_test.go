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
