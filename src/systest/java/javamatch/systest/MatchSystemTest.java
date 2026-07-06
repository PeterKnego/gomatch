package javamatch.systest;

import javamatch.client.MatchClient;
import javamatch.protocol.codecs.OrderStatus;
import javamatch.protocol.codecs.Side;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;

class MatchSystemTest {
    @Test
    void matchAndMarketData() throws Exception {
        try (ClusterHarness harness = ClusterHarness.launch()) {
            Recorder sellerRec = new Recorder();
            Recorder buyerRec = new Recorder();
            try (MatchClient seller = MatchClient.connect(
                     harness.aeronDir(), "0=" + harness.ingressEndpoint(), sellerRec);
                 MatchClient buyer = MatchClient.connect(
                     harness.aeronDir(), "0=" + harness.ingressEndpoint(), buyerRec)) {

                seller.submitOrder(1, Side.SELL, 100, 50);
                sellerRec.await(seller, () -> sellerRec.reports.size() >= 1, "sell ack");

                buyer.submitOrder(2, Side.BUY, 100, 50);
                buyerRec.await(buyer, () -> buyerRec.reports.size() >= 2, "buy ack+fill");
                sellerRec.await(seller, () -> sellerRec.reports.size() >= 2, "sell fill");
                // Both parties see the trade broadcast; the seller also polls.
                buyerRec.await(buyer, () -> buyerRec.trades.size() >= 1, "buyer trade");
                sellerRec.await(seller, () -> sellerRec.trades.size() >= 1, "seller trade");

                MatchClient.ExecReport fill = buyerRec.reports.get(1);
                assertEquals(OrderStatus.FILLED, fill.status());
                assertEquals(50, fill.qty());
                assertEquals(100, fill.price());
                assertEquals(100, buyerRec.trades.get(0).price());
                assertEquals(50, buyerRec.trades.get(0).qty());
                assertFalse(buyerRec.books.isEmpty(), "expected book updates broadcast to buyer");
            }
        }
    }
}
