package javamatch.engine;

import org.junit.jupiter.api.Test;

import java.util.List;

import static javamatch.engine.EngineAsserts.assertEvents;
import static org.junit.jupiter.api.Assertions.assertEquals;

class OrderBookTest {
    @Test
    void rejectsInvalidOrders() {
        OrderBook b = new OrderBook();
        assertEvents(b.newLimitOrder(new NewOrderCmd(7, 1, Side.BUY, 10, 0)),
            List.of(new Event(EventType.REJECTED).clientOrderId(7).owner(1).reason(RejectReason.BAD_QTY)));
        assertEvents(b.newLimitOrder(new NewOrderCmd(8, 1, Side.BUY, -5, 10)),
            List.of(new Event(EventType.REJECTED).clientOrderId(8).owner(1).reason(RejectReason.BAD_PRICE)));
    }

    @Test
    void restingOrderAcceptedWithBookUpdate() {
        OrderBook b = new OrderBook();
        assertEvents(b.newLimitOrder(new NewOrderCmd(7, 1, Side.BUY, 100, 30)), List.of(
            new Event(EventType.ACCEPTED).orderId(1).clientOrderId(7).owner(1).side(Side.BUY).price(100).qty(30),
            new Event(EventType.BOOK_UPDATE).side(Side.BUY).price(100).aggregateQty(30)));
    }

    @Test
    void orderIdsIncrease() {
        OrderBook b = new OrderBook();
        List<Event> first = b.newLimitOrder(new NewOrderCmd(1, 1, Side.BUY, 100, 1));
        List<Event> second = b.newLimitOrder(new NewOrderCmd(2, 1, Side.SELL, 200, 1));
        assertEquals(1, first.get(0).orderId);
        assertEquals(2, second.get(0).orderId);
    }

    @Test
    void sameLevelAggregates() {
        OrderBook b = new OrderBook();
        b.newLimitOrder(new NewOrderCmd(1, 1, Side.BUY, 100, 30));
        assertEvents(b.newLimitOrder(new NewOrderCmd(2, 2, Side.BUY, 100, 20)), List.of(
            new Event(EventType.ACCEPTED).orderId(2).clientOrderId(2).owner(2).side(Side.BUY).price(100).qty(20),
            new Event(EventType.BOOK_UPDATE).side(Side.BUY).price(100).aggregateQty(50)));
    }
}
