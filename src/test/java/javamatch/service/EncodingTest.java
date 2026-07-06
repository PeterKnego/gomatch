package javamatch.service;

import javamatch.engine.Event;
import javamatch.engine.EventType;
import javamatch.engine.Side;
import javamatch.protocol.codecs.BookUpdateDecoder;
import javamatch.protocol.codecs.ExecutionReportDecoder;
import javamatch.protocol.codecs.MessageHeaderDecoder;
import javamatch.protocol.codecs.OrderStatus;
import javamatch.protocol.codecs.TradeEventDecoder;
import org.agrona.ExpandableArrayBuffer;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertEquals;

class EncodingTest {
    private final Encoding encoding = new Encoding();
    private final ExpandableArrayBuffer buf = new ExpandableArrayBuffer();
    private final MessageHeaderDecoder hdr = new MessageHeaderDecoder();

    @Test
    void encodePartialFillExecutionReport() {
        Event ev = new Event(EventType.FILLED).orderId(3).clientOrderId(20).owner(2)
            .side(Side.BUY).price(100).qty(30).remainingQty(20);
        int length = encoding.encodeExecutionReport(buf, ev, 12345);
        assertEquals(MessageHeaderDecoder.ENCODED_LENGTH + ExecutionReportDecoder.BLOCK_LENGTH, length);

        hdr.wrap(buf, 0);
        assertEquals(ExecutionReportDecoder.TEMPLATE_ID, hdr.templateId());
        ExecutionReportDecoder er = new ExecutionReportDecoder()
            .wrap(buf, hdr.encodedLength(), hdr.blockLength(), hdr.version());
        assertEquals(OrderStatus.PARTIALLY_FILLED, er.status());
        assertEquals(30, er.qty());
        assertEquals(20, er.remainingQty());
        assertEquals(12345, er.timestamp());
        assertEquals(3, er.orderId());
        assertEquals(20, er.clientOrderId());
    }

    @Test
    void encodeFullFillStatus() {
        Event ev = new Event(EventType.FILLED).orderId(1).remainingQty(0).qty(30).side(Side.SELL).price(100);
        encoding.encodeExecutionReport(buf, ev, 1);
        hdr.wrap(buf, 0);
        ExecutionReportDecoder er = new ExecutionReportDecoder()
            .wrap(buf, hdr.encodedLength(), hdr.blockLength(), hdr.version());
        assertEquals(OrderStatus.FILLED, er.status());
    }

    @Test
    void encodeTradeAndBookUpdate() {
        Event trade = new Event(EventType.TRADE).price(100).qty(30).makerOrderId(1).takerOrderId(3);
        encoding.encodeTrade(buf, trade, 7);
        hdr.wrap(buf, 0);
        assertEquals(TradeEventDecoder.TEMPLATE_ID, hdr.templateId());
        TradeEventDecoder te = new TradeEventDecoder()
            .wrap(buf, hdr.encodedLength(), hdr.blockLength(), hdr.version());
        assertEquals(100, te.price());
        assertEquals(30, te.qty());
        assertEquals(1, te.makerOrderId());
        assertEquals(3, te.takerOrderId());
        assertEquals(7, te.timestamp());

        Event bu = new Event(EventType.BOOK_UPDATE).side(Side.SELL).price(100).aggregateQty(0);
        encoding.encodeBookUpdate(buf, bu, 8);
        hdr.wrap(buf, 0);
        assertEquals(BookUpdateDecoder.TEMPLATE_ID, hdr.templateId());
        BookUpdateDecoder b = new BookUpdateDecoder()
            .wrap(buf, hdr.encodedLength(), hdr.blockLength(), hdr.version());
        assertEquals(javamatch.protocol.codecs.Side.SELL, b.side());
        assertEquals(0, b.aggregateQty());
        assertEquals(8, b.timestamp());
    }
}
