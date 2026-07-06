package javamatch.systest;

import javamatch.client.MatchClient;
import javamatch.protocol.codecs.Side;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertEquals;

class RestartSystemTest {
    @Test
    void restingOrderSurvivesRestart() throws Exception {
        try (ClusterHarness harness = ClusterHarness.launch()) {
            Recorder rec = new Recorder();
            long orderId;
            try (MatchClient seller = MatchClient.connect(
                     harness.aeronDir(), "0=" + harness.ingressEndpoint(), rec)) {
                seller.submitOrder(1, Side.SELL, 105, 40);
                rec.await(seller, () -> rec.reports.size() >= 1, "sell ack");
                orderId = rec.reports.get(0).orderId();
                harness.snapshot();
            }

            harness.shutdown();
            harness.restart();

            Recorder buyerRec = new Recorder();
            try (MatchClient buyer = MatchClient.connect(
                     harness.aeronDir(), "0=" + harness.ingressEndpoint(), buyerRec)) {
                buyer.submitOrder(2, Side.BUY, 105, 40);
                buyerRec.await(buyer, () -> buyerRec.trades.size() >= 1, "trade after restart");
                assertEquals(orderId, buyerRec.trades.get(0).makerOrderId(),
                    "expected fill against restored order");
                assertEquals(105, buyerRec.trades.get(0).price());
                assertEquals(40, buyerRec.trades.get(0).qty());
            }
        }
    }
}
