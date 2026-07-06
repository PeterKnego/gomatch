package javamatch.service;

import javamatch.protocol.codecs.CancelOrderEncoder;
import javamatch.protocol.codecs.ExecutionReportDecoder;
import javamatch.protocol.codecs.MessageHeaderDecoder;
import javamatch.protocol.codecs.MessageHeaderEncoder;
import javamatch.protocol.codecs.NewOrderEncoder;
import javamatch.protocol.codecs.OrderStatus;
import javamatch.protocol.codecs.Side;
import io.aeron.cluster.codecs.CloseReason;
import org.agrona.DirectBuffer;
import org.agrona.ExpandableArrayBuffer;
import org.agrona.concurrent.UnsafeBuffer;
import org.junit.jupiter.api.Test;

import java.io.ByteArrayOutputStream;
import java.nio.ByteOrder;
import java.util.ArrayList;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;

class MatchingServiceTest {
    private static UnsafeBuffer newOrderFrame(long clientOrderId, Side side, long price, long qty) {
        ExpandableArrayBuffer buf = new ExpandableArrayBuffer();
        NewOrderEncoder enc = new NewOrderEncoder().wrapAndApplyHeader(buf, 0, new MessageHeaderEncoder());
        enc.clientOrderId(clientOrderId).side(side).price(price).qty(qty);
        int length = MessageHeaderEncoder.ENCODED_LENGTH + enc.encodedLength();
        byte[] bytes = new byte[length];
        buf.getBytes(0, bytes);
        return new UnsafeBuffer(bytes);
    }

    private static UnsafeBuffer cancelFrame(long orderId) {
        ExpandableArrayBuffer buf = new ExpandableArrayBuffer();
        CancelOrderEncoder enc = new CancelOrderEncoder().wrapAndApplyHeader(buf, 0, new MessageHeaderEncoder());
        enc.orderId(orderId);
        int length = MessageHeaderEncoder.ENCODED_LENGTH + enc.encodedLength();
        byte[] bytes = new byte[length];
        buf.getBytes(0, bytes);
        return new UnsafeBuffer(bytes);
    }

    private static List<Integer> templateIdsOf(List<byte[]> frames) {
        List<Integer> ids = new ArrayList<>();
        for (byte[] frame : frames) {
            ids.add((int) new UnsafeBuffer(frame).getShort(2, ByteOrder.LITTLE_ENDIAN));
        }
        return ids;
    }

    private static ExecutionReportDecoder decodeExecutionReport(byte[] frame) {
        DirectBuffer buf = new UnsafeBuffer(frame);
        MessageHeaderDecoder hdr = new MessageHeaderDecoder().wrap(buf, 0);
        assertEquals(ExecutionReportDecoder.TEMPLATE_ID, hdr.templateId());
        return new ExecutionReportDecoder().wrap(buf, hdr.encodedLength(), hdr.blockLength(), hdr.version());
    }

    @Test
    void matchRoutesReportsAndMarketData() {
        MatchingService s = new MatchingService();
        s.onStart(new FakeCluster(1000), null);
        FakeClientSession seller = new FakeClientSession(1);
        FakeClientSession buyer = new FakeClientSession(2);
        FakeClientSession watcher = new FakeClientSession(3);
        for (FakeClientSession sess : List.of(seller, buyer, watcher)) {
            s.onSessionOpen(sess, 1);
        }

        UnsafeBuffer sell = newOrderFrame(10, Side.SELL, 100, 50);
        s.onSessionMessage(seller, 1, sell, 0, sell.capacity(), null);
        UnsafeBuffer buy = newOrderFrame(20, Side.BUY, 100, 50);
        s.onSessionMessage(buyer, 2, buy, 0, buy.capacity(), null);

        int erId = ExecutionReportDecoder.TEMPLATE_ID;
        int teId = javamatch.protocol.codecs.TradeEventDecoder.TEMPLATE_ID;
        int buId = javamatch.protocol.codecs.BookUpdateDecoder.TEMPLATE_ID;

        // Watcher: only broadcast market data (BookUpdate after rest, then
        // TradeEvent + BookUpdate after the match).
        assertEquals(List.of(buId, teId, buId), templateIdsOf(watcher.frames));
        // Engine event order per command: Accepted, Trade, Filled(maker),
        // Filled(taker), BookUpdate. Routing preserves that order per session.
        assertEquals(List.of(erId, buId, teId, erId, buId), templateIdsOf(seller.frames));
        assertEquals(List.of(buId, erId, teId, erId, buId), templateIdsOf(buyer.frames));

        // Decode the buyer's FILLED report (frame index 3) and check payload.
        ExecutionReportDecoder er = decodeExecutionReport(buyer.frames.get(3));
        assertEquals(OrderStatus.FILLED, er.status());
        assertEquals(20, er.clientOrderId());
        assertEquals(50, er.qty());
        assertEquals(2, er.timestamp());
    }

    @Test
    void cancelUnknownOrderRejected() {
        MatchingService s = new MatchingService();
        s.onStart(new FakeCluster(0), null);
        FakeClientSession sess = new FakeClientSession(1);
        s.onSessionOpen(sess, 1);
        UnsafeBuffer cancel = cancelFrame(99);
        s.onSessionMessage(sess, 1, cancel, 0, cancel.capacity(), null);
        assertEquals(1, sess.frames.size());
    }

    @Test
    void closedSessionSkipped() {
        MatchingService s = new MatchingService();
        s.onStart(new FakeCluster(0), null);
        FakeClientSession a = new FakeClientSession(1);
        s.onSessionOpen(a, 1);
        FakeClientSession b = new FakeClientSession(2);
        s.onSessionOpen(b, 1);
        s.onSessionClose(a, 2, CloseReason.CLIENT_ACTION);
        UnsafeBuffer order = newOrderFrame(1, Side.BUY, 10, 1);
        // Session already closed: engine still applies the (replayed)
        // command, but nothing is offered to the closed session.
        s.onSessionMessage(a, 3, order, 0, order.capacity(), null);
        assertEquals(0, a.frames.size());
        assertEquals(1, b.frames.size());
    }

    @Test
    void truncatedFrameLoggedAndDropped() {
        MatchingService s = new MatchingService();
        s.onStart(new FakeCluster(0), null);
        FakeClientSession sess = new FakeClientSession(1);
        s.onSessionOpen(sess, 1);
        FakeClientSession other = new FakeClientSession(2);
        s.onSessionOpen(other, 1);
        UnsafeBuffer order = newOrderFrame(1, Side.BUY, 10, 5);
        // Valid SBE header but body shorter than blockLength: must be
        // logged and dropped, not thrown (Go OnSessionMessage parity).
        s.onSessionMessage(sess, 1, order, 0, order.capacity() - 8, null);
        assertEquals(0, sess.frames.size());
        assertEquals(0, other.frames.size());
    }

    @Test
    void invalidSideByteLoggedAndDropped() {
        MatchingService s = new MatchingService();
        s.onStart(new FakeCluster(0), null);
        FakeClientSession sess = new FakeClientSession(1);
        s.onSessionOpen(sess, 1);
        FakeClientSession other = new FakeClientSession(2);
        s.onSessionOpen(other, 1);
        UnsafeBuffer order = newOrderFrame(1, Side.BUY, 10, 5);
        // side field offset in the frame: header (8 bytes) + clientOrderId
        // (8 bytes) = 16. Overwrite with a value outside {0, 1}: the
        // generated NewOrderDecoder.side() would throw IllegalArgumentException
        // for this byte (SBE Side.get() only accepts 0, 1 or the null value
        // -128), which would otherwise wedge replay on a poison frame.
        order.putByte(16, (byte) 7);
        s.onSessionMessage(sess, 1, order, 0, order.capacity(), null);
        assertEquals(0, sess.frames.size());
        assertEquals(0, other.frames.size());
    }

    @Test
    void snapshotChunksRoundTrip() {
        MatchingService s = new MatchingService();
        s.onStart(new FakeCluster(0), null);
        FakeClientSession sess = new FakeClientSession(1);
        s.onSessionOpen(sess, 1);
        UnsafeBuffer order = newOrderFrame(1, Side.BUY, 10, 5);
        s.onSessionMessage(sess, 1, order, 0, order.capacity(), null);

        ByteArrayOutputStream stream = new ByteArrayOutputStream();
        s.writeSnapshot((buffer, offset, length) -> {
            byte[] chunk = new byte[length];
            buffer.getBytes(offset, chunk);
            stream.writeBytes(chunk);
        });

        MatchingService restored = new MatchingService();
        byte[] snapshot = stream.toByteArray();
        restored.restoreSnapshot(new UnsafeBuffer(snapshot), 0, snapshot.length);
        restored.onStart(new FakeCluster(0), null);
        FakeClientSession sess2 = new FakeClientSession(1);
        restored.onSessionOpen(sess2, 1);
        UnsafeBuffer cancel = cancelFrame(1);
        restored.onSessionMessage(sess2, 2, cancel, 0, cancel.capacity(), null);
        ExecutionReportDecoder er = decodeExecutionReport(sess2.frames.get(0));
        assertEquals(OrderStatus.CANCELED, er.status());
    }
}
