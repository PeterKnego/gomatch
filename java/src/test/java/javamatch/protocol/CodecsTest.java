package javamatch.protocol;

import javamatch.protocol.codecs.ExecutionReportDecoder;
import javamatch.protocol.codecs.ExecutionReportEncoder;
import javamatch.protocol.codecs.MessageHeaderDecoder;
import javamatch.protocol.codecs.MessageHeaderEncoder;
import javamatch.protocol.codecs.NewOrderDecoder;
import javamatch.protocol.codecs.NewOrderEncoder;
import javamatch.protocol.codecs.OrderStatus;
import javamatch.protocol.codecs.RejectReason;
import javamatch.protocol.codecs.Side;
import org.agrona.concurrent.UnsafeBuffer;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertEquals;

class CodecsTest {
    @Test
    void newOrderRoundTrip() {
        UnsafeBuffer buf = new UnsafeBuffer(new byte[128]);
        new NewOrderEncoder().wrapAndApplyHeader(buf, 0, new MessageHeaderEncoder())
            .clientOrderId(42).side(Side.SELL).price(101).qty(7);

        MessageHeaderDecoder hdr = new MessageHeaderDecoder().wrap(buf, 0);
        assertEquals(NewOrderDecoder.TEMPLATE_ID, hdr.templateId());
        NewOrderDecoder dec = new NewOrderDecoder()
            .wrap(buf, hdr.encodedLength(), hdr.blockLength(), hdr.version());
        assertEquals(42, dec.clientOrderId());
        assertEquals(Side.SELL, dec.side());
        assertEquals(101, dec.price());
        assertEquals(7, dec.qty());
    }

    @Test
    void executionReportRoundTrip() {
        UnsafeBuffer buf = new UnsafeBuffer(new byte[128]);
        new ExecutionReportEncoder().wrapAndApplyHeader(buf, 0, new MessageHeaderEncoder())
            .orderId(1).clientOrderId(42).status(OrderStatus.PARTIALLY_FILLED)
            .reason(RejectReason.NONE).side(Side.BUY)
            .price(100).qty(30).remainingQty(20).timestamp(999);

        MessageHeaderDecoder hdr = new MessageHeaderDecoder().wrap(buf, 0);
        ExecutionReportDecoder dec = new ExecutionReportDecoder()
            .wrap(buf, hdr.encodedLength(), hdr.blockLength(), hdr.version());
        assertEquals(1, dec.orderId());
        assertEquals(42, dec.clientOrderId());
        assertEquals(OrderStatus.PARTIALLY_FILLED, dec.status());
        assertEquals(RejectReason.NONE, dec.reason());
        assertEquals(Side.BUY, dec.side());
        assertEquals(100, dec.price());
        assertEquals(30, dec.qty());
        assertEquals(20, dec.remainingQty());
        assertEquals(999, dec.timestamp());
    }

    @Test
    void schemaIdentity() {
        assertEquals(901, NewOrderEncoder.SCHEMA_ID);
    }
}
