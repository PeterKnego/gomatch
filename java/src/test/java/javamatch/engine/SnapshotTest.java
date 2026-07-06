package javamatch.engine;

import org.agrona.ExpandableArrayBuffer;
import org.agrona.concurrent.UnsafeBuffer;
import org.junit.jupiter.api.Test;

import java.util.Arrays;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

class SnapshotTest {
    static OrderBook populatedBook() {
        OrderBook b = new OrderBook();
        b.newLimitOrder(new NewOrderCmd(1, 1, Side.BUY, 99, 10));
        b.newLimitOrder(new NewOrderCmd(2, 2, Side.BUY, 100, 20));
        b.newLimitOrder(new NewOrderCmd(3, 1, Side.BUY, 100, 5));
        b.newLimitOrder(new NewOrderCmd(4, 3, Side.SELL, 101, 7));
        b.newLimitOrder(new NewOrderCmd(5, 3, Side.SELL, 103, 9));
        return b;
    }

    static byte[] snapshotBytes(OrderBook b) {
        ExpandableArrayBuffer buf = new ExpandableArrayBuffer();
        int len = Snapshots.write(b, buf);
        return Arrays.copyOf(buf.byteArray(), len);
    }

    @Test
    void snapshotRoundTrip() {
        OrderBook b = populatedBook();
        byte[] first = snapshotBytes(b);
        OrderBook restored = Snapshots.restore(new UnsafeBuffer(first), 0, first.length);
        byte[] again = snapshotBytes(restored);
        assertTrue(Arrays.equals(first, again), "snapshot of restored book differs from original");
    }

    @Test
    void restorePreservesIdsAndMatching() {
        OrderBook b = populatedBook();
        byte[] snap = snapshotBytes(b);
        OrderBook restored = Snapshots.restore(new UnsafeBuffer(snap), 0, snap.length);
        // Next id continues the sequence (populatedBook used ids 1-5).
        List<Event> got = restored.newLimitOrder(new NewOrderCmd(9, 9, Side.SELL, 100, 25));
        assertEquals(6, got.get(0).orderId);
        // It must cross the restored best bid level (100: id 2 qty 20 then id 3 qty 5).
        assertEquals(EventType.TRADE, got.get(1).type);
        assertEquals(2, got.get(1).makerOrderId);
        assertEquals(20, got.get(1).qty);
        // Cancels of restored orders work (map rebuilt).
        assertEquals(EventType.CANCELED, restored.cancel(1, 1).get(0).type);
    }

    @Test
    void restoreRejectsBadHeader() {
        // A full-size header (magic + version + instrument + nextOrderId +
        // count = 28 bytes) with the wrong magic must be rejected.
        byte[] bad = new byte[28];
        IllegalArgumentException ex = assertThrows(IllegalArgumentException.class,
            () -> Snapshots.restore(new UnsafeBuffer(bad), 0, bad.length));
        assertTrue(ex.getMessage().contains("magic"), () -> "expected bad-magic error, got " + ex.getMessage());
        // Truncated input must also fail.
        assertThrows(IllegalArgumentException.class,
            () -> Snapshots.restore(new UnsafeBuffer(bad), 0, 8));
    }
}
