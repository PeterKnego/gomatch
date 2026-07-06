package javamatch.systest;

import javamatch.client.MatchClient;

import java.util.ArrayList;
import java.util.List;
import java.util.function.BooleanSupplier;

import static org.junit.jupiter.api.Assertions.fail;

final class Recorder implements MatchClient.Listener {
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

    void await(MatchClient client, BooleanSupplier condition, String what) {
        long deadline = System.nanoTime() + 30_000_000_000L;
        while (!condition.getAsBoolean()) {
            client.poll();
            if (System.nanoTime() > deadline) {
                fail("timed out waiting for " + what);
            }
            Thread.yield();
        }
    }
}
