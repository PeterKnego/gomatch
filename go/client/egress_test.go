package client

import (
	"bytes"
	"io"
	"testing"

	"github.com/lirm/aeron-go/aeron/atomic"

	"gomatch/protocol/codecs"
)

type recording struct {
	reports []ExecReport
	trades  []Trade
	books   []Book
}

func (r *recording) OnExecutionReport(e ExecReport) { r.reports = append(r.reports, e) }
func (r *recording) OnTrade(t Trade)                { r.trades = append(r.trades, t) }
func (r *recording) OnBookUpdate(b Book)            { r.books = append(r.books, b) }

func frame(t *testing.T, msg interface {
	Encode(*codecs.SbeGoMarshaller, io.Writer, bool) error
	SbeBlockLength() uint16
	SbeTemplateId() uint16
	SbeSchemaId() uint16
	SbeSchemaVersion() uint16
}) *atomic.Buffer {
	t.Helper()
	m := codecs.NewSbeGoMarshaller()
	var buf bytes.Buffer
	hdr := codecs.MessageHeader{BlockLength: msg.SbeBlockLength(), TemplateId: msg.SbeTemplateId(),
		SchemaId: msg.SbeSchemaId(), Version: msg.SbeSchemaVersion()}
	if err := hdr.Encode(m, &buf); err != nil {
		t.Fatal(err)
	}
	if err := msg.Encode(m, &buf, true); err != nil {
		t.Fatal(err)
	}
	return atomic.MakeBuffer(buf.Bytes())
}

func TestEgressDispatch(t *testing.T) {
	rec := &recording{}
	ad := newEgressAdapter(rec)

	er := frame(t, &codecs.ExecutionReport{OrderId: 5, ClientOrderId: 42,
		Status: codecs.OrderStatus.ACCEPTED, Side: codecs.Side.BUY, Price: 100, Qty: 10, RemainingQty: 10, Timestamp: 7})
	ad.onMessage(nil, 7, er, 0, er.Capacity(), nil)
	te := frame(t, &codecs.TradeEvent{Price: 100, Qty: 10, MakerOrderId: 1, TakerOrderId: 5, Timestamp: 8})
	ad.onMessage(nil, 8, te, 0, te.Capacity(), nil)
	bu := frame(t, &codecs.BookUpdate{Side: codecs.Side.SELL, Price: 100, AggregateQty: 0, Timestamp: 9})
	ad.onMessage(nil, 9, bu, 0, bu.Capacity(), nil)

	if len(rec.reports) != 1 || rec.reports[0].ClientOrderId != 42 || rec.reports[0].Status != codecs.OrderStatus.ACCEPTED {
		t.Fatalf("bad reports %+v", rec.reports)
	}
	if len(rec.trades) != 1 || rec.trades[0].TakerOrderId != 5 {
		t.Fatalf("bad trades %+v", rec.trades)
	}
	if len(rec.books) != 1 || rec.books[0].AggregateQty != 0 {
		t.Fatalf("bad books %+v", rec.books)
	}
}
