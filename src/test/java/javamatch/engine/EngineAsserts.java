package javamatch.engine;

import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;

final class EngineAsserts {
    private EngineAsserts() {}

    static void assertEvents(List<Event> got, List<Event> want) {
        assertEquals(want, got, () -> "events mismatch\ngot:  " + got + "\nwant: " + want);
    }
}
