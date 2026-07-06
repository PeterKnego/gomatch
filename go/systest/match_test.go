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
