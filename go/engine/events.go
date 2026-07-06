// Package engine is the pure deterministic matching core: no aeron imports,
// no I/O, integer ticks only.
package engine

type Side int8

const (
	Buy  Side = 0
	Sell Side = 1
)

type EventType int8

const (
	EvAccepted EventType = iota
	EvRejected
	EvTrade
	EvFilled
	EvCanceled
	EvBookUpdate
)

type RejectReason int8

const (
	ReasonNone RejectReason = iota
	ReasonBadQty
	ReasonBadPrice
	ReasonUnknownOrder
	ReasonNotOwner
)

// Event is a flat tagged union of everything the engine can emit. Which
// fields are meaningful depends on Type:
//
//	EvAccepted:   OrderId, ClientOrderId, Owner, Side, Price, Qty
//	EvRejected:   OrderId (cancels), ClientOrderId (orders), Owner, Reason
//	EvTrade:      Price, Qty, MakerOrderId, TakerOrderId, MakerOwner, TakerOwner
//	EvFilled:     OrderId, ClientOrderId, Owner, Side, Price, Qty (fill), RemainingQty
//	EvCanceled:   OrderId, ClientOrderId, Owner, Side, Price, RemainingQty (qty canceled)
//	EvBookUpdate: Side, Price, AggregateQty (0 = level gone)
type Event struct {
	Type          EventType
	OrderId       int64
	ClientOrderId int64
	Owner         int64
	Side          Side
	Price         int64
	Qty           int64
	RemainingQty  int64
	Reason        RejectReason
	MakerOrderId  int64
	TakerOrderId  int64
	MakerOwner    int64
	TakerOwner    int64
	AggregateQty  int64
}
