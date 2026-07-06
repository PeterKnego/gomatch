package javamatch.service;

import io.aeron.ExclusivePublication;
import io.aeron.Image;
import io.aeron.Publication;
import io.aeron.cluster.codecs.CloseReason;
import io.aeron.cluster.service.ClientSession;
import io.aeron.cluster.service.Cluster;
import io.aeron.cluster.service.ClusteredService;
import io.aeron.logbuffer.Header;
import javamatch.engine.Event;
import javamatch.engine.NewOrderCmd;
import javamatch.engine.OrderBook;
import javamatch.engine.Snapshots;
import javamatch.protocol.codecs.CancelOrderDecoder;
import javamatch.protocol.codecs.MessageHeaderDecoder;
import javamatch.protocol.codecs.NewOrderDecoder;
import javamatch.engine.Side;
import org.agrona.DirectBuffer;
import org.agrona.ExpandableArrayBuffer;
import org.agrona.ExpandableDirectByteBuffer;
import org.agrona.collections.Long2ObjectHashMap;
import org.agrona.collections.LongArrayList;

import java.util.List;

/**
 * The ClusteredService: decodes ingress, drives the matching engine, and
 * routes engine events to cluster egress. Mirror of gomatch's
 * service.MatchingService.
 */
public final class MatchingService implements ClusteredService {
    static final int SBE_HEADER_LENGTH = MessageHeaderDecoder.ENCODED_LENGTH;
    static final int SCHEMA_ID = 901;
    static final int NEW_ORDER_TEMPLATE_ID = NewOrderDecoder.TEMPLATE_ID;
    static final int CANCEL_ORDER_TEMPLATE_ID = CancelOrderDecoder.TEMPLATE_ID;
    static final int SNAPSHOT_CHUNK_SIZE = 1024;

    /** Receives snapshot bytes in chunks; mirrors Go's writeSnapshot(emit). */
    interface SnapshotChunkConsumer {
        void accept(DirectBuffer buffer, int offset, int length);
    }

    private Cluster cluster;
    private OrderBook book = new OrderBook();
    private final Long2ObjectHashMap<ClientSession> sessions = new Long2ObjectHashMap<>();
    private final LongArrayList sessionIds = new LongArrayList(); // deterministic broadcast order (insertion order)
    private final MessageHeaderDecoder headerDecoder = new MessageHeaderDecoder();
    private final NewOrderDecoder newOrderDecoder = new NewOrderDecoder();
    private final CancelOrderDecoder cancelOrderDecoder = new CancelOrderDecoder();
    private final Encoding encoding = new Encoding();
    private final ExpandableDirectByteBuffer egressBuffer = new ExpandableDirectByteBuffer(256);
    private final ExpandableArrayBuffer snapshotBuffer = new ExpandableArrayBuffer();

    public void onStart(Cluster cluster, Image snapshotImage) {
        this.cluster = cluster;
        if (snapshotImage == null) {
            return;
        }
        ExpandableArrayBuffer stream = new ExpandableArrayBuffer();
        int[] streamLength = {0};
        while (true) {
            int polled = snapshotImage.poll((buffer, offset, length, header) -> {
                stream.putBytes(streamLength[0], buffer, offset, length);
                streamLength[0] += length;
            }, 64);
            if (snapshotImage.isEndOfStream() || snapshotImage.isClosed()) {
                break;
            }
            if (polled == 0) {
                cluster.idleStrategy().idle(0);
            }
        }
        restoreSnapshot(stream, 0, streamLength[0]);
    }

    void restoreSnapshot(DirectBuffer buffer, int offset, int length) {
        book = Snapshots.restore(buffer, offset, length);
    }

    public void onSessionOpen(ClientSession session, long timestamp) {
        sessions.put(session.id(), session);
        sessionIds.addLong(session.id());
    }

    public void onSessionClose(ClientSession session, long timestamp, CloseReason closeReason) {
        sessions.remove(session.id());
        for (int i = 0; i < sessionIds.size(); i++) {
            if (sessionIds.getLong(i) == session.id()) {
                sessionIds.remove(i);
                break;
            }
        }
    }

    public void onSessionMessage(
        ClientSession session,
        long timestamp,
        DirectBuffer buffer,
        int offset,
        int length,
        Header header) {
        if (length < SBE_HEADER_LENGTH) {
            return;
        }
        headerDecoder.wrap(buffer, offset);
        int templateId = headerDecoder.templateId();
        if (headerDecoder.schemaId() != SCHEMA_ID) {
            System.err.println("unexpected schemaId=" + headerDecoder.schemaId() + " templateId=" + templateId);
            return;
        }
        List<Event> events;
        switch (templateId) {
            case NEW_ORDER_TEMPLATE_ID -> {
                newOrderDecoder.wrap(buffer, offset + SBE_HEADER_LENGTH,
                    headerDecoder.blockLength(), headerDecoder.version());
                events = book.newLimitOrder(new NewOrderCmd(
                    newOrderDecoder.clientOrderId(),
                    session.id(),
                    Side.fromCode((byte) newOrderDecoder.side().value()),
                    newOrderDecoder.price(),
                    newOrderDecoder.qty()));
            }
            case CANCEL_ORDER_TEMPLATE_ID -> {
                cancelOrderDecoder.wrap(buffer, offset + SBE_HEADER_LENGTH,
                    headerDecoder.blockLength(), headerDecoder.version());
                events = book.cancel(cancelOrderDecoder.orderId(), session.id());
            }
            default -> {
                return;
            }
        }
        route(events, timestamp);
    }

    private void route(List<Event> events, long timestamp) {
        for (int i = 0; i < events.size(); i++) {
            Event ev = events.get(i);
            switch (ev.type) {
                case ACCEPTED, REJECTED, CANCELED, FILLED ->
                    sendTo(ev.owner, encoding.encodeExecutionReport(egressBuffer, ev, timestamp));
                case TRADE ->
                    broadcast(encoding.encodeTrade(egressBuffer, ev, timestamp));
                case BOOK_UPDATE ->
                    broadcast(encoding.encodeBookUpdate(egressBuffer, ev, timestamp));
            }
        }
    }

    private void sendTo(long sessionId, int frameLength) {
        ClientSession session = sessions.get(sessionId);
        if (session != null) {
            offer(session, frameLength);
        }
    }

    private void broadcast(int frameLength) {
        for (int i = 0; i < sessionIds.size(); i++) {
            ClientSession session = sessions.get(sessionIds.getLong(i));
            if (session != null) {
                offer(session, frameLength);
            }
        }
    }

    private void offer(ClientSession session, int frameLength) {
        while (true) {
            long result = session.offer(egressBuffer, 0, frameLength);
            if (result >= 0 || result == ClientSession.MOCKED_OFFER) { // mocked on non-leaders
                return;
            }
            if (result != Publication.BACK_PRESSURED && result != Publication.ADMIN_ACTION) {
                System.err.println("egress offer failed - sessionId=" + session.id() + " result=" + result);
                return;
            }
            cluster.idleStrategy().idle(0);
        }
    }

    public void onTimerEvent(long correlationId, long timestamp) {
    }

    void writeSnapshot(SnapshotChunkConsumer consumer) {
        int length = Snapshots.write(book, snapshotBuffer);
        int offset = 0;
        while (offset < length) {
            int n = Math.min(SNAPSHOT_CHUNK_SIZE, length - offset);
            consumer.accept(snapshotBuffer, offset, n);
            offset += n;
        }
    }

    public void onTakeSnapshot(ExclusivePublication snapshotPublication) {
        writeSnapshot((buffer, offset, length) -> {
            while (true) {
                long result = snapshotPublication.offer(buffer, offset, length);
                if (result >= 0) {
                    return;
                }
                if (result != Publication.BACK_PRESSURED && result != Publication.ADMIN_ACTION) {
                    throw new IllegalStateException("snapshot offer failed: " + result);
                }
                cluster.idleStrategy().idle(0);
            }
        });
    }

    public void onRoleChange(Cluster.Role newRole) {
        System.out.println("role change: " + newRole);
    }

    public void onTerminate(Cluster cluster) {
    }
}
