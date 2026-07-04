package service

import (
	"bytes"
	"testing"

	"github.com/lirm/aeron-go/aeron/atomic"
	"github.com/lirm/aeron-go/aeron/idlestrategy"
	"github.com/lirm/aeron-go/aeron/logbuffer/term"
	"github.com/lirm/aeron-go/cluster"
	ccodecs "github.com/lirm/aeron-go/cluster/codecs"

	"gomatch/protocol/codecs"
)

type fakeSession struct {
	id     int64
	frames [][]byte
}

func (f *fakeSession) Id() int64                { return f.id }
func (f *fakeSession) ResponseStreamId() int32  { return 0 }
func (f *fakeSession) ResponseChannel() string  { return "" }
func (f *fakeSession) EncodedPrincipal() []byte { return nil }
func (f *fakeSession) Close()                   {}
func (f *fakeSession) Offer(b *atomic.Buffer, offset, length int32, _ term.ReservedValueSupplier) int64 {
	f.frames = append(f.frames, b.GetBytesArray(offset, length))
	return int64(length)
}

type fakeCluster struct{ now int64 }

func (f *fakeCluster) LogPosition() int64                       { return 0 }
func (f *fakeCluster) MemberId() int32                          { return 0 }
func (f *fakeCluster) Role() cluster.Role                       { return cluster.Leader }
func (f *fakeCluster) Time() int64                              { return f.now }
func (f *fakeCluster) TimeUnit() ccodecs.ClusterTimeUnitEnum    { return ccodecs.ClusterTimeUnit.MILLIS }
func (f *fakeCluster) IdleStrategy() idlestrategy.Idler         { return &idlestrategy.Busy{} }
func (f *fakeCluster) ScheduleTimer(int64, int64) bool          { return true }
func (f *fakeCluster) CancelTimer(int64) bool                   { return true }
func (f *fakeCluster) Offer(*atomic.Buffer, int32, int32) int64 { return 0 }

func ingressFrame(t *testing.T, msg sbeMessage) *atomic.Buffer {
	t.Helper()
	frame, err := encodeFrame(codecs.NewSbeGoMarshaller(), msg)
	if err != nil {
		t.Fatal(err)
	}
	return atomic.MakeBuffer(frame)
}

func templateIdsOf(t *testing.T, frames [][]byte) []uint16 {
	t.Helper()
	ids := make([]uint16, 0, len(frames))
	for _, f := range frames {
		buf := atomic.MakeBuffer(f)
		ids = append(ids, buf.GetUInt16(2))
	}
	return ids
}

func TestMatchRoutesReportsAndMarketData(t *testing.T) {
	s := NewMatchingService()
	s.OnStart(&fakeCluster{now: 1000}, nil)
	seller := &fakeSession{id: 1}
	buyer := &fakeSession{id: 2}
	watcher := &fakeSession{id: 3}
	for _, sess := range []*fakeSession{seller, buyer, watcher} {
		s.OnSessionOpen(sess, 1)
	}

	sell := ingressFrame(t, &codecs.NewOrder{ClientOrderId: 10, Side: codecs.Side.SELL, Price: 100, Qty: 50})
	s.OnSessionMessage(seller, 1, sell, 0, sell.Capacity(), nil)
	buy := ingressFrame(t, &codecs.NewOrder{ClientOrderId: 20, Side: codecs.Side.BUY, Price: 100, Qty: 50})
	s.OnSessionMessage(buyer, 2, buy, 0, buy.Capacity(), nil)

	// Seller: ACCEPTED + FILLED exec reports, plus broadcast market data.
	// Buyer: ACCEPTED + FILLED exec reports, plus broadcast market data.
	// Watcher: only broadcast market data (BookUpdate after rest, then
	// TradeEvent + BookUpdate after the match).
	erId := (&codecs.ExecutionReport{}).SbeTemplateId()
	teId := (&codecs.TradeEvent{}).SbeTemplateId()
	buId := (&codecs.BookUpdate{}).SbeTemplateId()

	wantWatcher := []uint16{buId, teId, buId}
	if got := templateIdsOf(t, watcher.frames); !equalU16(got, wantWatcher) {
		t.Fatalf("watcher frames %v want %v", got, wantWatcher)
	}
	// Engine event order per command: Accepted, Trade, Filled(maker),
	// Filled(taker), BookUpdate. Routing preserves that order per session.
	wantSeller := []uint16{erId, buId, teId, erId, buId}
	if got := templateIdsOf(t, seller.frames); !equalU16(got, wantSeller) {
		t.Fatalf("seller frames %v want %v", got, wantSeller)
	}
	wantBuyer := []uint16{buId, erId, teId, erId, buId}
	if got := templateIdsOf(t, buyer.frames); !equalU16(got, wantBuyer) {
		t.Fatalf("buyer frames %v want %v", got, wantBuyer)
	}

	// Decode the buyer's FILLED report (frame index 3) and check the payload.
	m := codecs.NewSbeGoMarshaller()
	var hdr codecs.MessageHeader
	buf := bytes.NewBuffer(buyer.frames[3])
	if err := hdr.Decode(m, buf, 0); err != nil {
		t.Fatal(err)
	}
	var er codecs.ExecutionReport
	if err := er.Decode(m, buf, hdr.Version, hdr.BlockLength, true); err != nil {
		t.Fatal(err)
	}
	if er.Status != codecs.OrderStatus.FILLED || er.ClientOrderId != 20 || er.Qty != 50 || er.Timestamp != 2 {
		t.Fatalf("bad buyer fill %+v", er)
	}
}

func TestCancelUnknownOrderRejected(t *testing.T) {
	s := NewMatchingService()
	s.OnStart(&fakeCluster{}, nil)
	sess := &fakeSession{id: 1}
	s.OnSessionOpen(sess, 1)
	cancel := ingressFrame(t, &codecs.CancelOrder{OrderId: 99})
	s.OnSessionMessage(sess, 1, cancel, 0, cancel.Capacity(), nil)
	if len(sess.frames) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(sess.frames))
	}
}

func TestClosedSessionSkipped(t *testing.T) {
	s := NewMatchingService()
	s.OnStart(&fakeCluster{}, nil)
	a := &fakeSession{id: 1}
	s.OnSessionOpen(a, 1)
	b := &fakeSession{id: 2}
	s.OnSessionOpen(b, 1)
	s.OnSessionClose(a, 2, ccodecs.CloseReason.CLIENT_ACTION)
	order := ingressFrame(t, &codecs.NewOrder{ClientOrderId: 1, Side: codecs.Side.BUY, Price: 10, Qty: 1})
	// Session already closed: engine still applies the (replayed) command,
	// but nothing is offered anywhere and nothing panics.
	s.OnSessionMessage(a, 3, order, 0, order.Capacity(), nil)
	if len(a.frames) != 0 {
		t.Fatalf("expected no frames to closed session, got %d", len(a.frames))
	}
	if len(b.frames) != 1 {
		t.Fatalf("expected surviving session to receive 1 broadcast frame, got %d", len(b.frames))
	}
}

func TestSnapshotChunksRoundTrip(t *testing.T) {
	s := NewMatchingService()
	s.OnStart(&fakeCluster{}, nil)
	sess := &fakeSession{id: 1}
	s.OnSessionOpen(sess, 1)
	order := ingressFrame(t, &codecs.NewOrder{ClientOrderId: 1, Side: codecs.Side.BUY, Price: 10, Qty: 5})
	s.OnSessionMessage(sess, 1, order, 0, order.Capacity(), nil)

	var chunks [][]byte
	if err := s.writeSnapshot(func(chunk []byte) error {
		chunks = append(chunks, append([]byte(nil), chunk...))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var stream bytes.Buffer
	for _, c := range chunks {
		stream.Write(c)
	}
	restored := NewMatchingService()
	if err := restored.restoreSnapshot(&stream); err != nil {
		t.Fatal(err)
	}
	// The restored book must still hold order id 1 (cancel works).
	restored.OnStart(&fakeCluster{}, nil)
	sess2 := &fakeSession{id: 1}
	restored.OnSessionOpen(sess2, 1)
	cancel := ingressFrame(t, &codecs.CancelOrder{OrderId: 1})
	restored.OnSessionMessage(sess2, 2, cancel, 0, cancel.Capacity(), nil)
	m := codecs.NewSbeGoMarshaller()
	var hdr codecs.MessageHeader
	buf := bytes.NewBuffer(sess2.frames[0])
	if err := hdr.Decode(m, buf, 0); err != nil {
		t.Fatal(err)
	}
	var er codecs.ExecutionReport
	if err := er.Decode(m, buf, hdr.Version, hdr.BlockLength, true); err != nil {
		t.Fatal(err)
	}
	if er.Status != codecs.OrderStatus.CANCELED {
		t.Fatalf("expected CANCELED after restore, got %+v", er)
	}
}

func equalU16(a, b []uint16) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
