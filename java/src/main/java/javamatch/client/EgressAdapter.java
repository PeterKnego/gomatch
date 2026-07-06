package javamatch.client;

import io.aeron.cluster.client.EgressListener;
import io.aeron.logbuffer.Header;
import javamatch.protocol.codecs.BookUpdateDecoder;
import javamatch.protocol.codecs.ExecutionReportDecoder;
import javamatch.protocol.codecs.MessageHeaderDecoder;
import javamatch.protocol.codecs.TradeEventDecoder;
import org.agrona.DirectBuffer;

/**
 * Decodes gomatch egress frames into typed callbacks. Also receives the
 * cluster client's session-level events.
 */
final class EgressAdapter implements EgressListener {
    private final MatchClient.Listener listener;
    private final MessageHeaderDecoder headerDecoder = new MessageHeaderDecoder();
    private final ExecutionReportDecoder executionReport = new ExecutionReportDecoder();
    private final TradeEventDecoder tradeEvent = new TradeEventDecoder();
    private final BookUpdateDecoder bookUpdate = new BookUpdateDecoder();

    EgressAdapter(MatchClient.Listener listener) {
        this.listener = listener;
    }

    @Override
    public void onMessage(
        long clusterSessionId,
        long timestamp,
        DirectBuffer buffer,
        int offset,
        int length,
        Header header) {
        if (length < MessageHeaderDecoder.ENCODED_LENGTH) {
            return;
        }
        headerDecoder.wrap(buffer, offset);
        int bodyOffset = offset + MessageHeaderDecoder.ENCODED_LENGTH;
        int blockLength = headerDecoder.blockLength();
        int version = headerDecoder.version();
        switch (headerDecoder.templateId()) {
            case ExecutionReportDecoder.TEMPLATE_ID -> {
                executionReport.wrap(buffer, bodyOffset, blockLength, version);
                listener.onExecutionReport(new MatchClient.ExecReport(
                    executionReport.orderId(), executionReport.clientOrderId(),
                    executionReport.status(), executionReport.reason(), executionReport.side(),
                    executionReport.price(), executionReport.qty(),
                    executionReport.remainingQty(), executionReport.timestamp()));
            }
            case TradeEventDecoder.TEMPLATE_ID -> {
                tradeEvent.wrap(buffer, bodyOffset, blockLength, version);
                listener.onTrade(new MatchClient.Trade(
                    tradeEvent.price(), tradeEvent.qty(),
                    tradeEvent.makerOrderId(), tradeEvent.takerOrderId(), tradeEvent.timestamp()));
            }
            case BookUpdateDecoder.TEMPLATE_ID -> {
                bookUpdate.wrap(buffer, bodyOffset, blockLength, version);
                listener.onBookUpdate(new MatchClient.Book(
                    bookUpdate.side(), bookUpdate.price(),
                    bookUpdate.aggregateQty(), bookUpdate.timestamp()));
            }
            default -> {
                // ignore unknown egress templates
            }
        }
    }
}
