package javamatch.client;

import javamatch.protocol.codecs.BookUpdateEncoder;
import javamatch.protocol.codecs.ExecutionReportEncoder;
import javamatch.protocol.codecs.MessageHeaderEncoder;
import javamatch.protocol.codecs.OrderStatus;
import javamatch.protocol.codecs.RejectReason;
import javamatch.protocol.codecs.Side;
import javamatch.protocol.codecs.TradeEventEncoder;
import org.agrona.ExpandableArrayBuffer;
import org.agrona.concurrent.UnsafeBuffer;
import org.junit.jupiter.api.Test;

import java.util.ArrayList;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;

class EgressAdapterTest {
    static final class Recording implements MatchClient.Listener {
        final List<MatchClient.ExecReport> reports = new ArrayList<>();
        final List<MatchClient.Trade> trades = new ArrayList<>();
        final List<MatchClient.Book> books = new ArrayList<>();

        @Override
        public void onExecutionReport(MatchClient.ExecReport e) {
            reports.add(e);
        }

        @Override
        public void onTrade(MatchClient.Trade t) {
            trades.add(t);
        }

        @Override
        public void onBookUpdate(MatchClient.Book b) {
            books.add(b);
        }
    }

    @Test
    void egressDispatch() {
        Recording rec = new Recording();
        EgressAdapter adapter = new EgressAdapter(rec);
        ExpandableArrayBuffer buf = new ExpandableArrayBuffer();
        MessageHeaderEncoder hdr = new MessageHeaderEncoder();

        ExecutionReportEncoder er = new ExecutionReportEncoder().wrapAndApplyHeader(buf, 0, hdr);
        er.orderId(5).clientOrderId(42).status(OrderStatus.ACCEPTED).reason(RejectReason.NONE)
            .side(Side.BUY).price(100).qty(10).remainingQty(10).timestamp(7);
        adapter.onMessage(0, 7, buf, 0, MessageHeaderEncoder.ENCODED_LENGTH + er.encodedLength(), null);

        TradeEventEncoder te = new TradeEventEncoder().wrapAndApplyHeader(buf, 0, hdr);
        te.price(100).qty(10).makerOrderId(1).takerOrderId(5).timestamp(8);
        adapter.onMessage(0, 8, buf, 0, MessageHeaderEncoder.ENCODED_LENGTH + te.encodedLength(), null);

        BookUpdateEncoder bu = new BookUpdateEncoder().wrapAndApplyHeader(buf, 0, hdr);
        bu.side(Side.SELL).price(100).aggregateQty(0).timestamp(9);
        adapter.onMessage(0, 9, buf, 0, MessageHeaderEncoder.ENCODED_LENGTH + bu.encodedLength(), null);

        assertEquals(1, rec.reports.size());
        assertEquals(42, rec.reports.get(0).clientOrderId());
        assertEquals(OrderStatus.ACCEPTED, rec.reports.get(0).status());
        assertEquals(1, rec.trades.size());
        assertEquals(5, rec.trades.get(0).takerOrderId());
        assertEquals(1, rec.books.size());
        assertEquals(0, rec.books.get(0).aggregateQty());
    }
}
