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
