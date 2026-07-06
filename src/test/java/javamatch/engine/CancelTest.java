package javamatch.engine;

import org.junit.jupiter.api.Test;

import java.util.List;

import static javamatch.engine.EngineAsserts.assertEvents;
import static org.junit.jupiter.api.Assertions.assertEquals;

class CancelTest {
    @Test
    void cancelRestingOrder() {
        OrderBook b = new OrderBook();
        b.newLimitOrder(new NewOrderCmd(10, 1, Side.SELL, 100, 30)); // id 1
        assertEvents(b.cancel(1, 1), List.of(
            new Event(EventType.CANCELED).orderId(1).clientOrderId(10).owner(1).side(Side.SELL).price(100).remainingQty(30),
            new Event(EventType.BOOK_UPDATE).side(Side.SELL).price(100).aggregateQty(0)));
        // Canceled order must be gone: cancel again is unknown.
        assertEvents(b.cancel(1, 1),
            List.of(new Event(EventType.REJECTED).orderId(1).owner(1).reason(RejectReason.UNKNOWN_ORDER)));
    }

    @Test
    void cancelNotOwner() {
        OrderBook b = new OrderBook();
        b.newLimitOrder(new NewOrderCmd(10, 1, Side.SELL, 100, 30)); // id 1
        assertEvents(b.cancel(1, 99),
            List.of(new Event(EventType.REJECTED).orderId(1).owner(99).reason(RejectReason.NOT_OWNER)));
    }

    @Test
    void cancelLeavesRestOfLevel() {
        OrderBook b = new OrderBook();
        b.newLimitOrder(new NewOrderCmd(10, 1, Side.SELL, 100, 30)); // id 1
        b.newLimitOrder(new NewOrderCmd(11, 1, Side.SELL, 100, 20)); // id 2
        assertEvents(b.cancel(1, 1), List.of(
            new Event(EventType.CANCELED).orderId(1).clientOrderId(10).owner(1).side(Side.SELL).price(100).remainingQty(30),
            new Event(EventType.BOOK_UPDATE).side(Side.SELL).price(100).aggregateQty(20)));
        // id 2 must still match.
        List<Event> fills = b.newLimitOrder(new NewOrderCmd(20, 2, Side.BUY, 100, 20));
        assertEquals(EventType.TRADE, fills.get(1).type);
        assertEquals(2, fills.get(1).makerOrderId);
    }
}
