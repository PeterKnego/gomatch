package javamatch.service;

import javamatch.engine.Event;
import javamatch.protocol.codecs.BookUpdateEncoder;
import javamatch.protocol.codecs.ExecutionReportEncoder;
import javamatch.protocol.codecs.MessageHeaderEncoder;
import javamatch.protocol.codecs.OrderStatus;
import javamatch.protocol.codecs.TradeEventEncoder;
import org.agrona.MutableDirectBuffer;

/**
 * Encodes engine events into SBE egress frames (header + body) at offset 0
 * of the caller's buffer, reusing flyweights. Returns the frame length.
 */
final class Encoding {
    private final MessageHeaderEncoder header = new MessageHeaderEncoder();
    private final ExecutionReportEncoder executionReport = new ExecutionReportEncoder();
    private final TradeEventEncoder tradeEvent = new TradeEventEncoder();
    private final BookUpdateEncoder bookUpdate = new BookUpdateEncoder();

    int encodeExecutionReport(MutableDirectBuffer buf, Event ev, long timestamp) {
        executionReport.wrapAndApplyHeader(buf, 0, header)
            .orderId(ev.orderId)
            .clientOrderId(ev.clientOrderId)
            .status(statusOf(ev))
            .reason(reasonOf(ev.reason))
            .side(sideOf(ev.side))
            .price(ev.price)
            .qty(ev.qty)
            .remainingQty(ev.remainingQty)
            .timestamp(timestamp);
        return MessageHeaderEncoder.ENCODED_LENGTH + executionReport.encodedLength();
    }

    int encodeTrade(MutableDirectBuffer buf, Event ev, long timestamp) {
        tradeEvent.wrapAndApplyHeader(buf, 0, header)
            .price(ev.price)
            .qty(ev.qty)
            .makerOrderId(ev.makerOrderId)
            .takerOrderId(ev.takerOrderId)
            .timestamp(timestamp);
        return MessageHeaderEncoder.ENCODED_LENGTH + tradeEvent.encodedLength();
    }

    int encodeBookUpdate(MutableDirectBuffer buf, Event ev, long timestamp) {
        bookUpdate.wrapAndApplyHeader(buf, 0, header)
            .side(sideOf(ev.side))
            .price(ev.price)
            .aggregateQty(ev.aggregateQty)
            .timestamp(timestamp);
        return MessageHeaderEncoder.ENCODED_LENGTH + bookUpdate.encodedLength();
    }

    static OrderStatus statusOf(Event ev) {
        return switch (ev.type) {
            case ACCEPTED -> OrderStatus.ACCEPTED;
            case CANCELED -> OrderStatus.CANCELED;
            case FILLED -> ev.remainingQty == 0 ? OrderStatus.FILLED : OrderStatus.PARTIALLY_FILLED;
            default -> OrderStatus.REJECTED;
        };
    }

    static javamatch.protocol.codecs.Side sideOf(javamatch.engine.Side side) {
        return side == javamatch.engine.Side.BUY
            ? javamatch.protocol.codecs.Side.BUY
            : javamatch.protocol.codecs.Side.SELL;
    }

    static javamatch.protocol.codecs.RejectReason reasonOf(javamatch.engine.RejectReason reason) {
        return switch (reason) {
            case NONE -> javamatch.protocol.codecs.RejectReason.NONE;
            case BAD_QTY -> javamatch.protocol.codecs.RejectReason.BAD_QTY;
            case BAD_PRICE -> javamatch.protocol.codecs.RejectReason.BAD_PRICE;
            case UNKNOWN_ORDER -> javamatch.protocol.codecs.RejectReason.UNKNOWN_ORDER;
            case NOT_OWNER -> javamatch.protocol.codecs.RejectReason.NOT_OWNER;
        };
    }
}
