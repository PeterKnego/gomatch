package javamatch.engine;

import org.junit.jupiter.api.Test;

import java.util.ArrayList;
import java.util.List;
import java.util.Random;

import static org.junit.jupiter.api.Assertions.assertArrayEquals;
import static org.junit.jupiter.api.Assertions.assertEquals;

/**
 * The engine must be a pure function of its command sequence: the same
 * stream applied to two books yields identical events and identical snapshot
 * bytes. Seeded Random is fine here - it generates the *inputs*.
 */
class DeterminismTest {
    record Run(List<Event> events, byte[] snapshot) {}

    @Test
    void deterministicReplay() {
        List<NewOrderCmd> commands = new ArrayList<>(2000);
        Random rng = new Random(42);
        for (int i = 0; i < 2000; i++) {
            commands.add(new NewOrderCmd(
                i,
                rng.nextInt(5) + 1,
                Side.fromCode((byte) rng.nextInt(2)),
                rng.nextInt(20) + 90,
                rng.nextInt(50) + 1));
        }
        Run r1 = run(commands);
        Run r2 = run(commands);
        assertEquals(r1.events(), r2.events(), "event streams differ between identical runs");
        assertArrayEquals(r1.snapshot(), r2.snapshot(), "snapshots differ between identical runs");
    }

    private static Run run(List<NewOrderCmd> commands) {
        OrderBook b = new OrderBook();
        List<Event> events = new ArrayList<>();
        for (int i = 0; i < commands.size(); i++) {
            NewOrderCmd cmd = commands.get(i);
            events.addAll(b.newLimitOrder(cmd));
            if (i % 7 == 3) { // deterministic sprinkle of cancels
                events.addAll(b.cancel(i / 2 + 1, cmd.owner()));
            }
        }
        return new Run(events, SnapshotTest.snapshotBytes(b));
    }
}
