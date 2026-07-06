package javamatch.engine;

import org.junit.jupiter.api.Test;

import java.util.List;

import static javamatch.engine.EngineAsserts.assertEvents;
import static org.junit.jupiter.api.Assertions.assertEquals;

class MatchingTest {
    @Test
    void fullFillSingleLevel() {
        OrderBook b = new OrderBook();
        b.newLimitOrder(new NewOrderCmd(10, 1, Side.SELL, 100, 50)); // id 1 rests
        List<Event> got = b.newLimitOrder(new NewOrderCmd(20, 2, Side.BUY, 100, 50));
        assertEvents(got, List.of(
            new Event(EventType.ACCEPTED).orderId(2).clientOrderId(20).owner(2).side(Side.BUY).price(100).qty(50),
            new Event(EventType.TRADE).price(100).qty(50).makerOrderId(1).takerOrderId(2).makerOwner(1).takerOwner(2),
            new Event(EventType.FILLED).orderId(1).clientOrderId(10).owner(1).side(Side.SELL).price(100).qty(50).remainingQty(0),
            new Event(EventType.FILLED).orderId(2).clientOrderId(20).owner(2).side(Side.BUY).price(100).qty(50).remainingQty(0),
            new Event(EventType.BOOK_UPDATE).side(Side.SELL).price(100).aggregateQty(0)));
    }

    @Test
    void partialFillRemainderRests() {
        OrderBook b = new OrderBook();
        b.newLimitOrder(new NewOrderCmd(10, 1, Side.SELL, 100, 30)); // id 1
        List<Event> got = b.newLimitOrder(new NewOrderCmd(20, 2, Side.BUY, 100, 50));
        assertEvents(got, List.of(
            new Event(EventType.ACCEPTED).orderId(2).clientOrderId(20).owner(2).side(Side.BUY).price(100).qty(50),
            new Event(EventType.TRADE).price(100).qty(30).makerOrderId(1).takerOrderId(2).makerOwner(1).takerOwner(2),
            new Event(EventType.FILLED).orderId(1).clientOrderId(10).owner(1).side(Side.SELL).price(100).qty(30).remainingQty(0),
            new Event(EventType.FILLED).orderId(2).clientOrderId(20).owner(2).side(Side.BUY).price(100).qty(30).remainingQty(20),
            new Event(EventType.BOOK_UPDATE).side(Side.SELL).price(100).aggregateQty(0),
            new Event(EventType.BOOK_UPDATE).side(Side.BUY).price(100).aggregateQty(20)));
    }

    @Test
    void tradesAtMakerPrice() {
        OrderBook b = new OrderBook();
        b.newLimitOrder(new NewOrderCmd(10, 1, Side.SELL, 100, 10)); // id 1
        List<Event> got = b.newLimitOrder(new NewOrderCmd(20, 2, Side.BUY, 105, 10));
        assertEquals(EventType.TRADE, got.get(1).type);
        assertEquals(100, got.get(1).price);
    }

    @Test
    void nonCrossingOrderRests() {
        OrderBook b = new OrderBook();
        b.newLimitOrder(new NewOrderCmd(10, 1, Side.SELL, 101, 10));
        List<Event> got = b.newLimitOrder(new NewOrderCmd(20, 2, Side.BUY, 100, 10));
        assertEvents(got, List.of(
            new Event(EventType.ACCEPTED).orderId(2).clientOrderId(20).owner(2).side(Side.BUY).price(100).qty(10),
            new Event(EventType.BOOK_UPDATE).side(Side.BUY).price(100).aggregateQty(10)));
    }

    @Test
    void sweepsMultipleLevels() {
        OrderBook b = new OrderBook();
        b.newLimitOrder(new NewOrderCmd(10, 1, Side.SELL, 100, 30)); // id 1
        b.newLimitOrder(new NewOrderCmd(11, 1, Side.SELL, 101, 40)); // id 2
        List<Event> got = b.newLimitOrder(new NewOrderCmd(20, 2, Side.BUY, 101, 100));
        assertEvents(got, List.of(
            new Event(EventType.ACCEPTED).orderId(3).clientOrderId(20).owner(2).side(Side.BUY).price(101).qty(100),
            new Event(EventType.TRADE).price(100).qty(30).makerOrderId(1).takerOrderId(3).makerOwner(1).takerOwner(2),
            new Event(EventType.FILLED).orderId(1).clientOrderId(10).owner(1).side(Side.SELL).price(100).qty(30).remainingQty(0),
            new Event(EventType.FILLED).orderId(3).clientOrderId(20).owner(2).side(Side.BUY).price(100).qty(30).remainingQty(70),
            new Event(EventType.TRADE).price(101).qty(40).makerOrderId(2).takerOrderId(3).makerOwner(1).takerOwner(2),
            new Event(EventType.FILLED).orderId(2).clientOrderId(11).owner(1).side(Side.SELL).price(101).qty(40).remainingQty(0),
            new Event(EventType.FILLED).orderId(3).clientOrderId(20).owner(2).side(Side.BUY).price(101).qty(40).remainingQty(30),
            new Event(EventType.BOOK_UPDATE).side(Side.SELL).price(100).aggregateQty(0),
            new Event(EventType.BOOK_UPDATE).side(Side.SELL).price(101).aggregateQty(0),
            new Event(EventType.BOOK_UPDATE).side(Side.BUY).price(101).aggregateQty(30)));
    }

    @Test
    void fifoPriorityWithinLevel() {
        OrderBook b = new OrderBook();
        b.newLimitOrder(new NewOrderCmd(10, 1, Side.SELL, 100, 30)); // id 1, first
        b.newLimitOrder(new NewOrderCmd(11, 3, Side.SELL, 100, 40)); // id 2, second
        List<Event> got = b.newLimitOrder(new NewOrderCmd(20, 2, Side.BUY, 100, 50));
        // id 1 must fill completely before id 2 trades at all.
        assertEvents(got, List.of(
            new Event(EventType.ACCEPTED).orderId(3).clientOrderId(20).owner(2).side(Side.BUY).price(100).qty(50),
            new Event(EventType.TRADE).price(100).qty(30).makerOrderId(1).takerOrderId(3).makerOwner(1).takerOwner(2),
            new Event(EventType.FILLED).orderId(1).clientOrderId(10).owner(1).side(Side.SELL).price(100).qty(30).remainingQty(0),
            new Event(EventType.FILLED).orderId(3).clientOrderId(20).owner(2).side(Side.BUY).price(100).qty(30).remainingQty(20),
            new Event(EventType.TRADE).price(100).qty(20).makerOrderId(2).takerOrderId(3).makerOwner(3).takerOwner(2),
            new Event(EventType.FILLED).orderId(2).clientOrderId(11).owner(3).side(Side.SELL).price(100).qty(20).remainingQty(20),
            new Event(EventType.FILLED).orderId(3).clientOrderId(20).owner(2).side(Side.BUY).price(100).qty(20).remainingQty(0),
            new Event(EventType.BOOK_UPDATE).side(Side.SELL).price(100).aggregateQty(20)));
    }

    @Test
    void bestPriceOrdering() {
        OrderBook b = new OrderBook();
        b.newLimitOrder(new NewOrderCmd(1, 1, Side.SELL, 102, 10)); // id 1
        b.newLimitOrder(new NewOrderCmd(2, 1, Side.SELL, 100, 10)); // id 2 - better
        List<Event> got = b.newLimitOrder(new NewOrderCmd(3, 2, Side.BUY, 102, 10));
        assertEquals(2, got.get(1).makerOrderId, "expected best ask (id 2 @100) to fill first");
    }
}
