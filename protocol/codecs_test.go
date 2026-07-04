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
