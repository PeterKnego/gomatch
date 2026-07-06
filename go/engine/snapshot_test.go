package engine

import (
	"bytes"
	"strings"
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
	// A full-size header (magic + version + instrument + nextOrderId + count
	// = 28 bytes) with the wrong magic must be rejected by the magic check.
	bad := make([]byte, 28)
	if _, err := RestoreOrderBook(bytes.NewReader(bad)); err == nil ||
		!strings.Contains(err.Error(), "magic") {
		t.Fatalf("expected bad-magic error, got %v", err)
	}
	// Truncated input must also fail (EOF path).
	if _, err := RestoreOrderBook(bytes.NewReader(bad[:8])); err == nil {
		t.Fatal("expected error for truncated header")
	}
}
