package javamatch.engine;

import java.util.Objects;

/**
 * Flat tagged union of everything the engine can emit; which fields are
 * meaningful depends on type (see the Go engine's Event doc):
 *
 *  ACCEPTED:    orderId, clientOrderId, owner, side, price, qty
 *  REJECTED:    orderId (cancels), clientOrderId (orders), owner, reason
 *  TRADE:       price, qty, makerOrderId, takerOrderId, makerOwner, takerOwner
 *  FILLED:      orderId, clientOrderId, owner, side, price, qty (fill), remainingQty
 *  CANCELED:    orderId, clientOrderId, owner, side, price, remainingQty (qty canceled)
 *  BOOK_UPDATE: side, price, aggregateQty (0 = level gone)
 */
public final class Event {
    public EventType type;
    public long orderId;
    public long clientOrderId;
    public long owner;
    public Side side = Side.BUY;
    public long price;
    public long qty;
    public long remainingQty;
    public RejectReason reason = RejectReason.NONE;
    public long makerOrderId;
    public long takerOrderId;
    public long makerOwner;
    public long takerOwner;
    public long aggregateQty;

    public Event(EventType type) {
        this.type = type;
    }

    public Event orderId(long v) { orderId = v; return this; }
    public Event clientOrderId(long v) { clientOrderId = v; return this; }
    public Event owner(long v) { owner = v; return this; }
    public Event side(Side v) { side = v; return this; }
    public Event price(long v) { price = v; return this; }
    public Event qty(long v) { qty = v; return this; }
    public Event remainingQty(long v) { remainingQty = v; return this; }
    public Event reason(RejectReason v) { reason = v; return this; }
    public Event makerOrderId(long v) { makerOrderId = v; return this; }
    public Event takerOrderId(long v) { takerOrderId = v; return this; }
    public Event makerOwner(long v) { makerOwner = v; return this; }
    public Event takerOwner(long v) { takerOwner = v; return this; }
    public Event aggregateQty(long v) { aggregateQty = v; return this; }

    @Override
    public boolean equals(Object o) {
        if (this == o) {
            return true;
        }
        if (!(o instanceof Event e)) {
            return false;
        }
        return type == e.type && orderId == e.orderId && clientOrderId == e.clientOrderId
            && owner == e.owner && side == e.side && price == e.price && qty == e.qty
            && remainingQty == e.remainingQty && reason == e.reason
            && makerOrderId == e.makerOrderId && takerOrderId == e.takerOrderId
            && makerOwner == e.makerOwner && takerOwner == e.takerOwner
            && aggregateQty == e.aggregateQty;
    }

    @Override
    public int hashCode() {
        return Objects.hash(type, orderId, clientOrderId, owner, side, price, qty,
            remainingQty, reason, makerOrderId, takerOrderId, makerOwner, takerOwner, aggregateQty);
    }

    @Override
    public String toString() {
        return "Event{" + type + " orderId=" + orderId + " clientOrderId=" + clientOrderId
            + " owner=" + owner + " side=" + side + " price=" + price + " qty=" + qty
            + " remainingQty=" + remainingQty + " reason=" + reason
            + " makerOrderId=" + makerOrderId + " takerOrderId=" + takerOrderId
            + " makerOwner=" + makerOwner + " takerOwner=" + takerOwner
            + " aggregateQty=" + aggregateQty + '}';
    }
}
