package service

import (
	"bytes"
	"io"
	"testing"

	"gomatch/engine"
	"gomatch/protocol/codecs"
)

// decodeFrame reads the SBE header then decodes the body with the header's
// blockLength/version, exactly as a client would.
func decodeFrame(t *testing.T, frame []byte, out interface {
	Decode(*codecs.SbeGoMarshaller, io.Reader, uint16, uint16, bool) error
}) codecs.MessageHeader {
	t.Helper()
	m := codecs.NewSbeGoMarshaller()
	buf := bytes.NewBuffer(frame)
	var hdr codecs.MessageHeader
	if err := hdr.Decode(m, buf, 0); err != nil {
		t.Fatal(err)
	}
	if err := out.Decode(m, buf, hdr.Version, hdr.BlockLength, true); err != nil {
		t.Fatal(err)
	}
	return hdr
}

func TestEncodePartialFillExecutionReport(t *testing.T) {
	m := codecs.NewSbeGoMarshaller()
	ev := engine.Event{Type: engine.EvFilled, OrderId: 3, ClientOrderId: 20, Owner: 2,
		Side: engine.Buy, Price: 100, Qty: 30, RemainingQty: 20}
	frame, err := encodeExecutionReport(m, ev, 12345)
	if err != nil {
		t.Fatal(err)
	}
	var er codecs.ExecutionReport
	hdr := decodeFrame(t, frame, &er)
	if hdr.TemplateId != er.SbeTemplateId() {
		t.Fatalf("bad template id %d", hdr.TemplateId)
	}
	if er.Status != codecs.OrderStatus.PARTIALLY_FILLED || er.Qty != 30 || er.RemainingQty != 20 ||
		er.Timestamp != 12345 || er.OrderId != 3 || er.ClientOrderId != 20 {
		t.Fatalf("bad exec report %+v", er)
	}
}

func TestEncodeFullFillStatus(t *testing.T) {
	m := codecs.NewSbeGoMarshaller()
	ev := engine.Event{Type: engine.EvFilled, OrderId: 1, RemainingQty: 0, Qty: 30, Side: engine.Sell, Price: 100}
	frame, err := encodeExecutionReport(m, ev, 1)
	if err != nil {
		t.Fatal(err)
	}
	var er codecs.ExecutionReport
	decodeFrame(t, frame, &er)
	if er.Status != codecs.OrderStatus.FILLED {
		t.Fatalf("expected FILLED, got %v", er.Status)
	}
}

func TestEncodeTradeAndBookUpdate(t *testing.T) {
	m := codecs.NewSbeGoMarshaller()
	trade := engine.Event{Type: engine.EvTrade, Price: 100, Qty: 30, MakerOrderId: 1, TakerOrderId: 3}
	frame, err := encodeTrade(m, trade, 7)
	if err != nil {
		t.Fatal(err)
	}
	var te codecs.TradeEvent
	decodeFrame(t, frame, &te)
	if te.Price != 100 || te.Qty != 30 || te.MakerOrderId != 1 || te.TakerOrderId != 3 || te.Timestamp != 7 {
		t.Fatalf("bad trade %+v", te)
	}

	bu := engine.Event{Type: engine.EvBookUpdate, Side: engine.Sell, Price: 100, AggregateQty: 0}
	frame, err = encodeBookUpdate(m, bu, 8)
	if err != nil {
		t.Fatal(err)
	}
	var b codecs.BookUpdate
	decodeFrame(t, frame, &b)
	if b.Side != codecs.Side.SELL || b.AggregateQty != 0 || b.Timestamp != 8 {
		t.Fatalf("bad book update %+v", b)
	}
}
