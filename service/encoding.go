// Package service glues the matching engine to Aeron Cluster.
package service

import (
	"bytes"
	"io"

	"gomatch/engine"
	"gomatch/protocol/codecs"
)

type sbeMessage interface {
	Encode(*codecs.SbeGoMarshaller, io.Writer, bool) error
	SbeBlockLength() uint16
	SbeTemplateId() uint16
	SbeSchemaId() uint16
	SbeSchemaVersion() uint16
}

func encodeFrame(m *codecs.SbeGoMarshaller, msg sbeMessage) ([]byte, error) {
	var buf bytes.Buffer
	hdr := codecs.MessageHeader{
		BlockLength: msg.SbeBlockLength(),
		TemplateId:  msg.SbeTemplateId(),
		SchemaId:    msg.SbeSchemaId(),
		Version:     msg.SbeSchemaVersion(),
	}
	if err := hdr.Encode(m, &buf); err != nil {
		return nil, err
	}
	if err := msg.Encode(m, &buf, true); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func statusOf(ev engine.Event) codecs.OrderStatusEnum {
	switch ev.Type {
	case engine.EvAccepted:
		return codecs.OrderStatus.ACCEPTED
	case engine.EvRejected:
		return codecs.OrderStatus.REJECTED
	case engine.EvCanceled:
		return codecs.OrderStatus.CANCELED
	case engine.EvFilled:
		if ev.RemainingQty == 0 {
			return codecs.OrderStatus.FILLED
		}
		return codecs.OrderStatus.PARTIALLY_FILLED
	}
	return codecs.OrderStatus.REJECTED
}

func encodeExecutionReport(m *codecs.SbeGoMarshaller, ev engine.Event, timestamp int64) ([]byte, error) {
	return encodeFrame(m, &codecs.ExecutionReport{
		OrderId:       ev.OrderId,
		ClientOrderId: ev.ClientOrderId,
		Status:        statusOf(ev),
		Reason:        codecs.RejectReasonEnum(ev.Reason),
		Side:          codecs.SideEnum(ev.Side),
		Price:         ev.Price,
		Qty:           ev.Qty,
		RemainingQty:  ev.RemainingQty,
		Timestamp:     timestamp,
	})
}

func encodeTrade(m *codecs.SbeGoMarshaller, ev engine.Event, timestamp int64) ([]byte, error) {
	return encodeFrame(m, &codecs.TradeEvent{
		Price:        ev.Price,
		Qty:          ev.Qty,
		MakerOrderId: ev.MakerOrderId,
		TakerOrderId: ev.TakerOrderId,
		Timestamp:    timestamp,
	})
}

func encodeBookUpdate(m *codecs.SbeGoMarshaller, ev engine.Event, timestamp int64) ([]byte, error) {
	return encodeFrame(m, &codecs.BookUpdate{
		Side:         codecs.SideEnum(ev.Side),
		Price:        ev.Price,
		AggregateQty: ev.AggregateQty,
		Timestamp:    timestamp,
	})
}
