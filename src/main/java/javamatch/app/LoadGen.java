package javamatch.app;

import javamatch.client.MatchClient;
import javamatch.protocol.codecs.Side;
import org.agrona.collections.Long2LongHashMap;
import org.agrona.collections.LongArrayList;

import java.util.Random;

/**
 * Submits a deterministic mix of crossing and resting limit orders and
 * reports throughput and submit-to-ack latency percentiles.
 *
 * Open-loop by default (throughput numbers; latency reflects burst
 * queueing). Pass -rate N to pace submission at N orders/sec for honest
 * per-order latency percentiles — measured from each order's *scheduled*
 * send time, so generator stalls count (coordinated-omission correction).
 */
public final class LoadGen {
    private LoadGen() {}

    static final class Collector implements MatchClient.Listener {
        final Long2LongHashMap submitted = new Long2LongHashMap(Long.MIN_VALUE);
        final LongArrayList latencies = new LongArrayList();
        int acked;

        @Override
        public void onExecutionReport(MatchClient.ExecReport e) {
            long t0 = submitted.remove(e.clientOrderId());
            if (t0 != Long.MIN_VALUE) {
                latencies.addLong(System.nanoTime() - t0);
                acked++;
            }
        }

        @Override
        public void onTrade(MatchClient.Trade t) {
        }

        @Override
        public void onBookUpdate(MatchClient.Book b) {
        }
    }

    public static void main(String[] args) {
        int orders = 100_000;
        int rate = 0;
        String aeronDir = "/dev/shm/aeron-" + System.getProperty("user.name");
        String ingress = "0=localhost:20000";
        String egress = "localhost:0";
        for (int i = 0; i + 1 < args.length; i += 2) {
            switch (args[i]) {
                case "-orders" -> orders = Integer.parseInt(args[i + 1]);
                case "-rate" -> rate = Integer.parseInt(args[i + 1]);
                case "-aeron-dir" -> aeronDir = args[i + 1];
                case "-ingress" -> ingress = args[i + 1];
                case "-egress" -> egress = args[i + 1];
                default -> throw new IllegalArgumentException("unknown flag " + args[i]);
            }
        }

        Collector col = new Collector();
        try (MatchClient c = MatchClient.connectWithEgress(aeronDir, ingress, egress, col)) {
            long intervalNanos = rate > 0 ? 1_000_000_000L / rate : 0;
            Random rng = new Random(1);
            long start = System.nanoTime();
            for (int i = 0; i < orders; i++) {
                Side side = i % 2 == 1 ? Side.SELL : Side.BUY;
                long price = 100 + rng.nextInt(5) - 2; // 98..102 straddling mid: ~half cross
                long id = i + 1;
                long sendTime = System.nanoTime();
                if (intervalNanos > 0) {
                    // Latency is measured from the scheduled time, not the
                    // actual send, so generator stalls count against the
                    // reported numbers (coordinated-omission correction).
                    sendTime = start + (long) i * intervalNanos;
                    while (System.nanoTime() < sendTime) {
                        c.poll();
                    }
                }
                col.submitted.put(id, sendTime);
                c.submitOrder(id, side, price, rng.nextInt(10) + 1);
                c.poll();
            }
            long deadline = System.nanoTime() + 30_000_000_000L;
            while (col.acked < orders && System.nanoTime() < deadline) {
                c.poll();
            }
            long elapsedNanos = System.nanoTime() - start;

            long[] latencies = col.latencies.toLongArray();
            java.util.Arrays.sort(latencies);
            String target = rate > 0 ? " target=" + rate + "/s" : "";
            System.out.printf("orders=%d acked=%d%s elapsed=%s rate=%.0f orders/sec%n",
                orders, col.acked, target, formatNanos(elapsedNanos),
                col.acked / (elapsedNanos / 1e9));
            System.out.printf("ack latency p50=%s p99=%s p99.9=%s%n",
                formatNanos(percentile(latencies, 0.50)),
                formatNanos(percentile(latencies, 0.99)),
                formatNanos(percentile(latencies, 0.999)));
        }
    }

    static long percentile(long[] sorted, double p) {
        if (sorted.length == 0) {
            return 0;
        }
        return sorted[(int) (p * (sorted.length - 1))];
    }

    static String formatNanos(long nanos) {
        if (nanos < 1_000) {
            return nanos + "ns";
        }
        if (nanos < 1_000_000) {
            return String.format("%.3fµs", nanos / 1e3);
        }
        if (nanos < 1_000_000_000) {
            return String.format("%.3fms", nanos / 1e6);
        }
        return String.format("%.3fs", nanos / 1e9);
    }
}
