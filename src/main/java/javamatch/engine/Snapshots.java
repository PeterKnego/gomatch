package javamatch.engine;

import org.agrona.DirectBuffer;
import org.agrona.ExpandableArrayBuffer;

import java.nio.ByteOrder;
import java.util.ArrayList;

/**
 * Snapshot I/O in the exact gomatch byte format (magic "GMS1", little-endian):
 * header of magic uint32, version int32, reserved instrument int64,
 * nextOrderId int64, count int32; then per resting order id, clientOrderId,
 * owner int64s, side int8, price, qty int64s — bids best-to-worst FIFO, then
 * asks. A Java engine can restore a Go snapshot and vice versa.
 */
public final class Snapshots {
    static final int MAGIC = 0x474D5331; // "GMS1"
    static final int VERSION = 1;
    static final int HEADER_LENGTH = 4 + 4 + 8 + 8 + 4;
    static final int ORDER_LENGTH = 8 + 8 + 8 + 1 + 8 + 8;
    private static final ByteOrder LE = ByteOrder.LITTLE_ENDIAN;

    private Snapshots() {}

    /** Writes the complete book state at offset 0; returns the length. */
    public static int write(OrderBook book, ExpandableArrayBuffer out) {
        int off = 0;
        out.putInt(off, MAGIC, LE);
        off += 4;
        out.putInt(off, VERSION, LE);
        off += 4;
        out.putLong(off, 1L, LE); // reserved instrument id
        off += 8;
        out.putLong(off, book.nextOrderId(), LE);
        off += 8;
        out.putInt(off, book.orderCount(), LE);
        off += 4;
        off = writeSide(book.bids(), out, off);
        off = writeSide(book.asks(), out, off);
        return off;
    }

    private static int writeSide(ArrayList<OrderBook.PriceLevel> levels, ExpandableArrayBuffer out, int off) {
        for (OrderBook.PriceLevel lvl : levels) {
            for (OrderBook.Order o = lvl.head; o != null; o = o.next) {
                out.putLong(off, o.id, LE);
                off += 8;
                out.putLong(off, o.clientOrderId, LE);
                off += 8;
                out.putLong(off, o.owner, LE);
                off += 8;
                out.putByte(off, o.side.code());
                off += 1;
                out.putLong(off, o.price, LE);
                off += 8;
                out.putLong(off, o.qty, LE);
                off += 8;
            }
        }
        return off;
    }

    /**
     * Rebuilds a book from snapshot bytes. Orders are re-rested without
     * matching; the id sequence continues where it left off.
     */
    public static OrderBook restore(DirectBuffer in, int offset, int length) {
        if (length < HEADER_LENGTH) {
            throw new IllegalArgumentException("truncated snapshot header: " + length + " bytes");
        }
        int off = offset;
        int magic = in.getInt(off, LE);
        off += 4;
        if (magic != MAGIC) {
            throw new IllegalArgumentException(String.format("bad snapshot magic 0x%x", magic));
        }
        int version = in.getInt(off, LE);
        off += 4;
        if (version != VERSION) {
            throw new IllegalArgumentException("unsupported snapshot version " + version);
        }
        off += 8; // reserved instrument id
        long nextOrderId = in.getLong(off, LE);
        off += 8;
        int count = in.getInt(off, LE);
        off += 4;
        if (length < HEADER_LENGTH + (long) count * ORDER_LENGTH) {
            throw new IllegalArgumentException("truncated snapshot body: " + length + " bytes for " + count + " orders");
        }
        OrderBook b = new OrderBook();
        for (int i = 0; i < count; i++) {
            OrderBook.Order o = new OrderBook.Order();
            o.id = in.getLong(off, LE);
            off += 8;
            o.clientOrderId = in.getLong(off, LE);
            off += 8;
            o.owner = in.getLong(off, LE);
            off += 8;
            o.side = Side.fromCode(in.getByte(off));
            off += 1;
            o.price = in.getLong(off, LE);
            off += 8;
            o.qty = in.getLong(off, LE);
            off += 8;
            b.restOrder(o);
        }
        b.nextOrderId(nextOrderId);
        return b;
    }
}
