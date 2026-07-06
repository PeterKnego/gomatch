package javamatch.engine;

import org.agrona.collections.Long2ObjectHashMap;

import java.util.ArrayList;
import java.util.List;

/**
 * Single-instrument limit order book: price levels sorted best-first per
 * side, FIFO within a level. Pure and deterministic — no I/O, integer ticks.
 */
public final class OrderBook {
    static final class Order {
        long id;
        long clientOrderId;
        long owner;
        Side side;
        long price;
        long qty; // remaining quantity
        PriceLevel level;
        Order prev; // FIFO within the level
        Order next;
    }

    static final class PriceLevel {
        final long price;
        long totalQty;
        Order head;
        Order tail;

        PriceLevel(long price) {
            this.price = price;
        }
    }

    private final ArrayList<PriceLevel> bids = new ArrayList<>(); // sorted best-first: highest price first
    private final ArrayList<PriceLevel> asks = new ArrayList<>(); // sorted best-first: lowest price first
    private final Long2ObjectHashMap<Order> orders = new Long2ObjectHashMap<>();
    private long nextOrderId = 1;

    /**
     * Binary-searches levels for price; desc is true for bids. Returns the
     * index when found, else {@code -(insertionIndex) - 1} (the
     * Arrays.binarySearch convention).
     */
    private static int levelIndex(ArrayList<PriceLevel> levels, long price, boolean desc) {
        int lo = 0;
        int hi = levels.size();
        while (lo < hi) {
            int mid = (lo + hi) >>> 1;
            long p = levels.get(mid).price;
            if (p == price) {
                return mid;
            }
            if (desc == (p > price)) {
                lo = mid + 1;
            } else {
                hi = mid;
            }
        }
        return -lo - 1;
    }

    public List<Event> newLimitOrder(NewOrderCmd cmd) {
        List<Event> events = new ArrayList<>();
        if (cmd.qty() <= 0) {
            events.add(new Event(EventType.REJECTED)
                .clientOrderId(cmd.clientOrderId()).owner(cmd.owner()).reason(RejectReason.BAD_QTY));
            return events;
        }
        if (cmd.price() <= 0) {
            events.add(new Event(EventType.REJECTED)
                .clientOrderId(cmd.clientOrderId()).owner(cmd.owner()).reason(RejectReason.BAD_PRICE));
            return events;
        }
        Order taker = new Order();
        taker.id = nextOrderId++;
        taker.clientOrderId = cmd.clientOrderId();
        taker.owner = cmd.owner();
        taker.side = cmd.side();
        taker.price = cmd.price();
        taker.qty = cmd.qty();
        events.add(new Event(EventType.ACCEPTED)
            .orderId(taker.id).clientOrderId(taker.clientOrderId).owner(taker.owner)
            .side(taker.side).price(taker.price).qty(taker.qty));

        // Track levels whose aggregate changed, in order of first change, so
        // the final BookUpdate batch is deterministic without map iteration.
        ArrayList<PriceLevel> changed = new ArrayList<>();
        ArrayList<Side> changedSides = new ArrayList<>();

        match(taker, events, changed, changedSides);
        if (taker.qty > 0) {
            PriceLevel lvl = restOrder(taker);
            touch(changed, changedSides, taker.side, lvl);
        }
        for (int i = 0; i < changed.size(); i++) {
            PriceLevel lvl = changed.get(i);
            events.add(new Event(EventType.BOOK_UPDATE)
                .side(changedSides.get(i)).price(lvl.price).aggregateQty(lvl.totalQty));
        }
        return events;
    }

    /**
     * Removes a resting order. The engine owns the order map, so it performs
     * the ownership check: a cancel from a non-owner is rejected.
     */
    public List<Event> cancel(long orderId, long requestingOwner) {
        List<Event> events = new ArrayList<>();
        Order o = orders.get(orderId);
        if (o == null) {
            events.add(new Event(EventType.REJECTED)
                .orderId(orderId).owner(requestingOwner).reason(RejectReason.UNKNOWN_ORDER));
            return events;
        }
        if (o.owner != requestingOwner) {
            events.add(new Event(EventType.REJECTED)
                .orderId(orderId).owner(requestingOwner).reason(RejectReason.NOT_OWNER));
            return events;
        }
        PriceLevel lvl = o.level;
        lvl.totalQty -= o.qty;
        unlink(o);
        events.add(new Event(EventType.CANCELED)
            .orderId(o.id).clientOrderId(o.clientOrderId).owner(o.owner)
            .side(o.side).price(o.price).remainingQty(o.qty));
        events.add(new Event(EventType.BOOK_UPDATE)
            .side(o.side).price(lvl.price).aggregateQty(lvl.totalQty));
        return events;
    }

    private static void touch(ArrayList<PriceLevel> changed, ArrayList<Side> changedSides, Side side, PriceLevel lvl) {
        for (PriceLevel l : changed) {
            if (l == lvl) {
                return;
            }
        }
        changed.add(lvl);
        changedSides.add(side);
    }

    private void match(Order taker, List<Event> events, ArrayList<PriceLevel> changed, ArrayList<Side> changedSides) {
        while (taker.qty > 0) {
            ArrayList<PriceLevel> opp = taker.side == Side.SELL ? bids : asks;
            Side oppSide = taker.side == Side.SELL ? Side.BUY : Side.SELL;
            if (opp.isEmpty()) {
                break;
            }
            PriceLevel best = opp.get(0);
            if (taker.side == Side.BUY && best.price > taker.price) {
                break;
            }
            if (taker.side == Side.SELL && best.price < taker.price) {
                break;
            }
            Order maker = best.head;
            long fill = Math.min(taker.qty, maker.qty);
            maker.qty -= fill;
            taker.qty -= fill;
            best.totalQty -= fill;
            touch(changed, changedSides, oppSide, best);
            events.add(new Event(EventType.TRADE)
                .price(best.price).qty(fill)
                .makerOrderId(maker.id).takerOrderId(taker.id)
                .makerOwner(maker.owner).takerOwner(taker.owner));
            events.add(new Event(EventType.FILLED)
                .orderId(maker.id).clientOrderId(maker.clientOrderId).owner(maker.owner)
                .side(maker.side).price(best.price).qty(fill).remainingQty(maker.qty));
            events.add(new Event(EventType.FILLED)
                .orderId(taker.id).clientOrderId(taker.clientOrderId).owner(taker.owner)
                .side(taker.side).price(best.price).qty(fill).remainingQty(taker.qty));
            if (maker.qty == 0) {
                unlink(maker);
            }
        }
    }

    /**
     * Removes an order from its level FIFO and the order map, and removes the
     * level from its side when it becomes empty. Does not touch totalQty:
     * callers account for quantity themselves.
     */
    private void unlink(Order o) {
        PriceLevel lvl = o.level;
        if (o.prev != null) {
            o.prev.next = o.next;
        } else {
            lvl.head = o.next;
        }
        if (o.next != null) {
            o.next.prev = o.prev;
        } else {
            lvl.tail = o.prev;
        }
        o.prev = null;
        o.next = null;
        o.level = null;
        orders.remove(o.id);
        if (lvl.head == null) {
            ArrayList<PriceLevel> levels = o.side == Side.BUY ? bids : asks;
            boolean desc = o.side == Side.BUY;
            int idx = levelIndex(levels, lvl.price, desc);
            if (idx >= 0) {
                levels.remove(idx);
            }
        }
    }

    ArrayList<PriceLevel> bids() { return bids; }
    ArrayList<PriceLevel> asks() { return asks; }
    int orderCount() { return orders.size(); }
    long nextOrderId() { return nextOrderId; }
    void nextOrderId(long v) { nextOrderId = v; }

    PriceLevel restOrder(Order o) {
        ArrayList<PriceLevel> levels = o.side == Side.BUY ? bids : asks;
        boolean desc = o.side == Side.BUY;
        int idx = levelIndex(levels, o.price, desc);
        PriceLevel lvl;
        if (idx >= 0) {
            lvl = levels.get(idx);
        } else {
            lvl = new PriceLevel(o.price);
            levels.add(-idx - 1, lvl);
        }
        o.level = lvl;
        if (lvl.tail == null) {
            lvl.head = o;
            lvl.tail = o;
        } else {
            o.prev = lvl.tail;
            lvl.tail.next = o;
            lvl.tail = o;
        }
        lvl.totalQty += o.qty;
        orders.put(o.id, o);
        return lvl;
    }
}
