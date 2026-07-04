// Package client is a typed gomatch client over the Aeron Cluster client.
package client

import (
	"bytes"

	"github.com/lirm/aeron-go/aeron/atomic"
	"github.com/lirm/aeron-go/aeron/logbuffer"
	"github.com/lirm/aeron-go/aeron/logging"
	aeroncluster "github.com/lirm/aeron-go/cluster/client"

	"gomatch/protocol/codecs"
)

var logger = logging.MustGetLogger("gomatch-client")

type ExecReport struct {
	OrderId, ClientOrderId              int64
	Status                              codecs.OrderStatusEnum
	Reason                              codecs.RejectReasonEnum
	Side                                codecs.SideEnum
	Price, Qty, RemainingQty, Timestamp int64
}

type Trade struct{ Price, Qty, MakerOrderId, TakerOrderId, Timestamp int64 }

type Book struct {
	Side                           codecs.SideEnum
	Price, AggregateQty, Timestamp int64
}

type Listener interface {
	OnExecutionReport(ExecReport)
	OnTrade(Trade)
	OnBookUpdate(Book)
}

// egressAdapter decodes gomatch egress frames into typed callbacks. It also
// satisfies the cluster client's EgressListener for session-level events.
type egressAdapter struct {
	listener   Listener
	marshaller *codecs.SbeGoMarshaller
}

func newEgressAdapter(l Listener) *egressAdapter {
	return &egressAdapter{listener: l, marshaller: codecs.NewSbeGoMarshaller()}
}

func (a *egressAdapter) onMessage(
	_ *aeroncluster.AeronCluster,
	_ int64,
	buffer *atomic.Buffer,
	offset int32,
	length int32,
	_ *logbuffer.Header,
) {
	if length < 8 {
		return
	}
	blockLength := buffer.GetUInt16(offset)
	templateId := buffer.GetUInt16(offset + 2)
	version := buffer.GetUInt16(offset + 6)
	body := &bytes.Buffer{}
	buffer.WriteBytes(body, offset+8, length-8)
	switch templateId {
	case (&codecs.ExecutionReport{}).SbeTemplateId():
		var m codecs.ExecutionReport
		if err := m.Decode(a.marshaller, body, version, blockLength, true); err != nil {
			logger.Errorf("ExecutionReport decode: %v", err)
			return
		}
		a.listener.OnExecutionReport(ExecReport{OrderId: m.OrderId, ClientOrderId: m.ClientOrderId,
			Status: m.Status, Reason: m.Reason, Side: m.Side,
			Price: m.Price, Qty: m.Qty, RemainingQty: m.RemainingQty, Timestamp: m.Timestamp})
	case (&codecs.TradeEvent{}).SbeTemplateId():
		var m codecs.TradeEvent
		if err := m.Decode(a.marshaller, body, version, blockLength, true); err != nil {
			logger.Errorf("TradeEvent decode: %v", err)
			return
		}
		a.listener.OnTrade(Trade{Price: m.Price, Qty: m.Qty,
			MakerOrderId: m.MakerOrderId, TakerOrderId: m.TakerOrderId, Timestamp: m.Timestamp})
	case (&codecs.BookUpdate{}).SbeTemplateId():
		var m codecs.BookUpdate
		if err := m.Decode(a.marshaller, body, version, blockLength, true); err != nil {
			logger.Errorf("BookUpdate decode: %v", err)
			return
		}
		a.listener.OnBookUpdate(Book{Side: m.Side, Price: m.Price,
			AggregateQty: m.AggregateQty, Timestamp: m.Timestamp})
	default:
		logger.Debugf("ignoring egress templateId=%d", templateId)
	}
}
