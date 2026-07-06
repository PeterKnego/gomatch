package engine

import (
	"encoding/binary"
	"fmt"
	"io"
)

const snapshotMagic uint32 = 0x474D5331 // "GMS1"
const snapshotVersion int32 = 1

// Snapshot writes the complete book state: header, then resting orders in
// deterministic book order (bids best-to-worst FIFO, then asks).
func (b *OrderBook) Snapshot(w io.Writer) error {
	for _, v := range []any{snapshotMagic, snapshotVersion, int64(1) /* reserved instrument id */, b.nextOrderId, int32(len(b.orders))} {
		if err := binary.Write(w, binary.LittleEndian, v); err != nil {
			return err
		}
	}
	writeSide := func(levels []*priceLevel) error {
		for _, lvl := range levels {
			for o := lvl.head; o != nil; o = o.next {
				for _, v := range []any{o.id, o.clientOrderId, o.owner, int8(o.side), o.price, o.qty} {
					if err := binary.Write(w, binary.LittleEndian, v); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}
	if err := writeSide(b.bids); err != nil {
		return err
	}
	return writeSide(b.asks)
}

// RestoreOrderBook rebuilds a book from a snapshot stream. Orders are
// re-rested without matching; the id sequence continues where it left off.
func RestoreOrderBook(r io.Reader) (*OrderBook, error) {
	var magic uint32
	var version int32
	var instrument, nextOrderId int64
	var count int32
	for _, v := range []any{&magic, &version, &instrument, &nextOrderId, &count} {
		if err := binary.Read(r, binary.LittleEndian, v); err != nil {
			return nil, err
		}
	}
	if magic != snapshotMagic {
		return nil, fmt.Errorf("bad snapshot magic 0x%x", magic)
	}
	if version != snapshotVersion {
		return nil, fmt.Errorf("unsupported snapshot version %d", version)
	}
	b := NewOrderBook()
	for i := int32(0); i < count; i++ {
		o := &order{}
		var side int8
		for _, v := range []any{&o.id, &o.clientOrderId, &o.owner, &side, &o.price, &o.qty} {
			if err := binary.Read(r, binary.LittleEndian, v); err != nil {
				return nil, err
			}
		}
		o.side = Side(side)
		b.restOrder(o)
	}
	b.nextOrderId = nextOrderId
	return b, nil
}
