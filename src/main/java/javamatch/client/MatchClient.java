package javamatch.client;

import io.aeron.cluster.client.AeronCluster;
import javamatch.protocol.codecs.CancelOrderEncoder;
import javamatch.protocol.codecs.MessageHeaderEncoder;
import javamatch.protocol.codecs.NewOrderEncoder;
import javamatch.protocol.codecs.OrderStatus;
import javamatch.protocol.codecs.RejectReason;
import javamatch.protocol.codecs.Side;
import org.agrona.CloseHelper;
import org.agrona.ExpandableDirectByteBuffer;
import org.agrona.concurrent.BackoffIdleStrategy;
import org.agrona.concurrent.IdleStrategy;

import java.util.concurrent.TimeUnit;

/** Typed gomatch client over the Aeron Cluster client. */
public final class MatchClient implements AutoCloseable {
    public record ExecReport(
        long orderId, long clientOrderId, OrderStatus status, RejectReason reason,
        Side side, long price, long qty, long remainingQty, long timestamp) {}

    public record Trade(long price, long qty, long makerOrderId, long takerOrderId, long timestamp) {}

    public record Book(Side side, long price, long aggregateQty, long timestamp) {}

    public interface Listener {
        void onExecutionReport(ExecReport e);

        void onTrade(Trade t);

        void onBookUpdate(Book b);
    }

    private static final long OFFER_TIMEOUT_NS = TimeUnit.SECONDS.toNanos(10);

    private final AeronCluster cluster;
    private final IdleStrategy idleStrategy = new BackoffIdleStrategy();
    private final ExpandableDirectByteBuffer buffer = new ExpandableDirectByteBuffer(64);
    private final MessageHeaderEncoder header = new MessageHeaderEncoder();
    private final NewOrderEncoder newOrder = new NewOrderEncoder();
    private final CancelOrderEncoder cancelOrder = new CancelOrderEncoder();

    private MatchClient(AeronCluster cluster) {
        this.cluster = cluster;
    }

    /** Connects to the cluster. ingressEndpoints example: "0=localhost:20000". */
    public static MatchClient connect(String aeronDir, String ingressEndpoints, Listener listener) {
        return connectWithEgress(aeronDir, ingressEndpoints, "localhost:0", listener);
    }

    /**
     * connect with an explicit egress endpoint: the address:port on this host
     * that cluster nodes send responses to. It must be reachable from every
     * node — the default localhost:0 only works when the leader runs on the
     * same host as the client.
     */
    public static MatchClient connectWithEgress(
        String aeronDir, String ingressEndpoints, String egressEndpoint, Listener listener) {
        AeronCluster cluster = AeronCluster.connect(new AeronCluster.Context()
            .aeronDirectoryName(aeronDir)
            .ingressChannel("aeron:udp?alias=gomatch-ingress")
            .ingressEndpoints(ingressEndpoints)
            .egressChannel("aeron:udp?alias=gomatch-egress|endpoint=" + egressEndpoint)
            .egressListener(new EgressAdapter(listener))
            .messageTimeoutNs(TimeUnit.SECONDS.toNanos(30)));
        return new MatchClient(cluster);
    }

    public void submitOrder(long clientOrderId, Side side, long price, long qty) {
        newOrder.wrapAndApplyHeader(buffer, 0, header)
            .clientOrderId(clientOrderId).side(side).price(price).qty(qty);
        offer(MessageHeaderEncoder.ENCODED_LENGTH + newOrder.encodedLength());
    }

    public void cancelOrder(long orderId) {
        cancelOrder.wrapAndApplyHeader(buffer, 0, header).orderId(orderId);
        offer(MessageHeaderEncoder.ENCODED_LENGTH + cancelOrder.encodedLength());
    }

    private void offer(int length) {
        long deadline = System.nanoTime() + OFFER_TIMEOUT_NS;
        while (cluster.offer(buffer, 0, length) < 0) {
            if (System.nanoTime() > deadline) {
                throw new IllegalStateException("timed out offering to cluster");
            }
            idleStrategy.idle(cluster.pollEgress());
        }
        idleStrategy.reset();
    }

    public int poll() {
        return cluster.pollEgress();
    }

    @Override
    public void close() {
        CloseHelper.quietClose(cluster);
    }
}
