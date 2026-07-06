# javamatch Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Port gomatch (Go matching engine on Aeron Cluster) to Java at `/home/claude/ultima/javamatch`, wire- and snapshot-compatible with the Go version, with all unit tests, systests, and a loadgen ported.

**Architecture:** Structural mirror of gomatch with idiomatic Java/Aeron mechanics: sbe-tool flyweight codecs decoded in place, reused encode buffers, Agrona collections. Packages: `javamatch.engine` (pure matching core), `javamatch.protocol.codecs` (generated), `javamatch.service` (ClusteredService), `javamatch.client`, `javamatch.app` (EngineMain, LoadGen), `src/systest/java` (integration tests, own source set).

**Tech Stack:** Java 21 (Temurin installed), Gradle 8.10.2 (dist already cached at `~/.gradle/wrapper/dists/gradle-8.10.2-bin/a04bxjujx95o3nb99gddekhwo/gradle-8.10.2/bin/gradle`), `io.aeron:aeron-all:1.52.0` (bundles Agrona), `uk.co.real-logic:sbe-all:1.38.1` (codegen only), JUnit 5.

**Reference:** the Go sources at `/home/claude/ultima/gomatch` — every class/test here mirrors a Go file. Read the Go file next to each task when in doubt; behavior must match exactly.

## Global Constraints

- Project root: `/home/claude/ultima/javamatch` (own git repo, already initialized; spec committed).
- SBE schema id 901; `gomatch-schema.xml` copied **verbatim** from `/home/claude/ultima/gomatch/protocol/gomatch-schema.xml` (only generated-code namespace differs: `javamatch.protocol.codecs`). Never edit the schema.
- Snapshot bytes must be identical to Go: little-endian, magic `0x474D5331`, version 1, header 28 bytes (`uint32 magic, int32 version, int64 instrument=1, int64 nextOrderId, int32 count`), then per order 41 bytes (`int64 id, int64 clientOrderId, int64 owner, int8 side, int64 price, int64 qty`), orders written bids best-first FIFO then asks best-first FIFO.
- Event emission order and payloads must match the Go engine exactly (the ported tests encode this).
- Run all commands from `/home/claude/ultima/javamatch`. Gradle: `./gradlew` (wrapper committed in Task 1).
- Generated code is not committed (`build/` is gitignored).
- Every Java file starts with its package declaration; imports are explicit (no wildcards needed except where shown).
- Aeron 1.52 API facts (verified against the jar): `ClientSession`, `Cluster`, `ClusteredService` are interfaces; `ClusteredService.onSessionMessage(ClientSession, long, DirectBuffer, int, int, Header)`; `onTakeSnapshot(ExclusivePublication)`; `onSessionClose(ClientSession, long, CloseReason)`; `ClusterTool.snapshot(File, PrintStream)` is static; `ClusteredMediaDriver.launch(MediaDriver.Context, Archive.Context, ConsensusModule.Context)`.

---

### Task 1: Project scaffold, SBE codegen, codec round-trip tests

**Files:**
- Create: `settings.gradle`, `build.gradle`, `.gitignore`, `gradle wrapper files`
- Create: `src/main/resources/gomatch-schema.xml` (copy)
- Test: `src/test/java/javamatch/protocol/CodecsTest.java`

**Interfaces:**
- Produces: generated codecs in package `javamatch.protocol.codecs`: `MessageHeaderEncoder`/`MessageHeaderDecoder` (`ENCODED_LENGTH = 8`), `NewOrderEncoder`/`NewOrderDecoder`, `CancelOrderEncoder`/`CancelOrderDecoder`, `ExecutionReportEncoder`/`ExecutionReportDecoder`, `TradeEventEncoder`/`TradeEventDecoder`, `BookUpdateEncoder`/`BookUpdateDecoder`, enums `Side`, `OrderStatus`, `RejectReason`. Encoders have `wrapAndApplyHeader(MutableDirectBuffer, int, MessageHeaderEncoder)` and fluent field setters; `TEMPLATE_ID`, `SCHEMA_ID`, `BLOCK_LENGTH` constants.
- Produces: Gradle tasks `test`, `systest` (empty for now), `generateSbe`.

- [ ] **Step 1: Scaffold build files**

`settings.gradle`:
```groovy
rootProject.name = 'javamatch'
```

`.gitignore`:
```
build/
.gradle/
*.iml
.idea/
```

`build.gradle`:
```groovy
plugins {
    id 'java'
}

repositories {
    mavenCentral()
}

java {
    toolchain {
        languageVersion = JavaLanguageVersion.of(21)
    }
}

configurations {
    sbe
}

dependencies {
    implementation 'io.aeron:aeron-all:1.52.0'
    sbe 'uk.co.real-logic:sbe-all:1.38.1'
    testImplementation 'org.junit.jupiter:junit-jupiter:5.10.2'
    testRuntimeOnly 'org.junit.platform:junit-platform-launcher'
}

def sbeDir = layout.buildDirectory.dir('generated/sbe')

tasks.register('generateSbe', JavaExec) {
    classpath = configurations.sbe
    mainClass = 'uk.co.real_logic.sbe.SbeTool'
    systemProperty 'sbe.output.dir', sbeDir.get().asFile.absolutePath
    systemProperty 'sbe.target.language', 'Java'
    systemProperty 'sbe.target.namespace', 'javamatch.protocol.codecs'
    args file('src/main/resources/gomatch-schema.xml').absolutePath
    inputs.file 'src/main/resources/gomatch-schema.xml'
    outputs.dir sbeDir
}

sourceSets.main.java.srcDir sbeDir
tasks.named('compileJava') { dependsOn 'generateSbe' }

tasks.named('test', Test) {
    useJUnitPlatform()
}

sourceSets {
    systest {
        java.srcDir 'src/systest/java'
        compileClasspath += sourceSets.main.output
        runtimeClasspath += sourceSets.main.output
    }
}

configurations {
    systestImplementation.extendsFrom implementation, testImplementation
    systestRuntimeOnly.extendsFrom runtimeOnly, testRuntimeOnly
}

tasks.register('systest', Test) {
    description = 'Integration tests against an in-process ClusteredMediaDriver'
    testClassesDirs = sourceSets.systest.output.classesDirs
    classpath = sourceSets.systest.runtimeClasspath
    useJUnitPlatform()
    jvmArgs '--add-opens=java.base/sun.nio.ch=ALL-UNNAMED',
            '--add-exports=java.base/jdk.internal.misc=ALL-UNNAMED'
    timeout = Duration.ofMinutes(10)
    outputs.upToDateWhen { false }
}
```

Add `import java.time.Duration` at the top of `build.gradle` if Gradle complains about `Duration`; otherwise use `java.time.Duration.ofMinutes(10)` inline.

Copy the schema:
```bash
mkdir -p src/main/resources
cp /home/claude/ultima/gomatch/protocol/gomatch-schema.xml src/main/resources/gomatch-schema.xml
```

- [ ] **Step 2: Generate the Gradle wrapper**

```bash
cd /home/claude/ultima/javamatch
~/.gradle/wrapper/dists/gradle-8.10.2-bin/a04bxjujx95o3nb99gddekhwo/gradle-8.10.2/bin/gradle wrapper --gradle-version 8.10.2 --no-daemon
./gradlew --version
```
Expected: wrapper files created (`gradlew`, `gradle/wrapper/*`), version prints Gradle 8.10.2.

- [ ] **Step 3: Write the failing codec test** (port of `protocol/codecs_test.go`)

`src/test/java/javamatch/protocol/CodecsTest.java`:
```java
package javamatch.protocol;

import javamatch.protocol.codecs.ExecutionReportDecoder;
import javamatch.protocol.codecs.ExecutionReportEncoder;
import javamatch.protocol.codecs.MessageHeaderDecoder;
import javamatch.protocol.codecs.MessageHeaderEncoder;
import javamatch.protocol.codecs.NewOrderDecoder;
import javamatch.protocol.codecs.NewOrderEncoder;
import javamatch.protocol.codecs.OrderStatus;
import javamatch.protocol.codecs.RejectReason;
import javamatch.protocol.codecs.Side;
import org.agrona.concurrent.UnsafeBuffer;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertEquals;

class CodecsTest {
    @Test
    void newOrderRoundTrip() {
        UnsafeBuffer buf = new UnsafeBuffer(new byte[128]);
        new NewOrderEncoder().wrapAndApplyHeader(buf, 0, new MessageHeaderEncoder())
            .clientOrderId(42).side(Side.SELL).price(101).qty(7);

        MessageHeaderDecoder hdr = new MessageHeaderDecoder().wrap(buf, 0);
        assertEquals(NewOrderDecoder.TEMPLATE_ID, hdr.templateId());
        NewOrderDecoder dec = new NewOrderDecoder()
            .wrap(buf, hdr.encodedLength(), hdr.blockLength(), hdr.version());
        assertEquals(42, dec.clientOrderId());
        assertEquals(Side.SELL, dec.side());
        assertEquals(101, dec.price());
        assertEquals(7, dec.qty());
    }

    @Test
    void executionReportRoundTrip() {
        UnsafeBuffer buf = new UnsafeBuffer(new byte[128]);
        new ExecutionReportEncoder().wrapAndApplyHeader(buf, 0, new MessageHeaderEncoder())
            .orderId(1).clientOrderId(42).status(OrderStatus.PARTIALLY_FILLED)
            .reason(RejectReason.NONE).side(Side.BUY)
            .price(100).qty(30).remainingQty(20).timestamp(999);

        MessageHeaderDecoder hdr = new MessageHeaderDecoder().wrap(buf, 0);
        ExecutionReportDecoder dec = new ExecutionReportDecoder()
            .wrap(buf, hdr.encodedLength(), hdr.blockLength(), hdr.version());
        assertEquals(1, dec.orderId());
        assertEquals(42, dec.clientOrderId());
        assertEquals(OrderStatus.PARTIALLY_FILLED, dec.status());
        assertEquals(RejectReason.NONE, dec.reason());
        assertEquals(Side.BUY, dec.side());
        assertEquals(100, dec.price());
        assertEquals(30, dec.qty());
        assertEquals(20, dec.remainingQty());
        assertEquals(999, dec.timestamp());
    }

    @Test
    void schemaIdentity() {
        assertEquals(901, NewOrderEncoder.SCHEMA_ID);
    }
}
```

- [ ] **Step 4: Run the test**

```bash
./gradlew test --tests 'javamatch.protocol.CodecsTest'
```
Expected: PASS (codegen ran, codecs compiled, round trips hold). If codegen fails, inspect `build/generated/sbe` and the SbeTool output.

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "Scaffold Gradle build with SBE codegen; codec round-trip tests"
```

---

### Task 2: Engine types + resting/reject behavior

**Files:**
- Create: `src/main/java/javamatch/engine/Side.java`, `EventType.java`, `RejectReason.java`, `Event.java`, `NewOrderCmd.java`, `OrderBook.java`
- Test: `src/test/java/javamatch/engine/OrderBookTest.java`, `src/test/java/javamatch/engine/EngineAsserts.java`

**Interfaces:**
- Produces (relied on by every later task):
  - `enum Side { BUY, SELL }` with `byte code()` (0/1) and `static Side fromCode(byte)`.
  - `enum EventType { ACCEPTED, REJECTED, TRADE, FILLED, CANCELED, BOOK_UPDATE }`
  - `enum RejectReason { NONE, BAD_QTY, BAD_PRICE, UNKNOWN_ORDER, NOT_OWNER }` with `byte code()`.
  - `Event` — mutable, public fields, fluent setters named after fields, `equals/hashCode/toString` over all fields.
  - `record NewOrderCmd(long clientOrderId, long owner, Side side, long price, long qty)`
  - `OrderBook`: `List<Event> newLimitOrder(NewOrderCmd)`, `List<Event> cancel(long orderId, long requestingOwner)` (cancel arrives in Task 4; declare now, stub `throw new UnsupportedOperationException()` until then is NOT allowed — implement fully in Task 4, so in this task simply omit `cancel`).

- [ ] **Step 1: Write the failing test** (port of `engine/order_book_test.go`)

`src/test/java/javamatch/engine/EngineAsserts.java`:
```java
package javamatch.engine;

import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;

final class EngineAsserts {
    private EngineAsserts() {}

    static void assertEvents(List<Event> got, List<Event> want) {
        assertEquals(want, got, () -> "events mismatch\ngot:  " + got + "\nwant: " + want);
    }
}
```

`src/test/java/javamatch/engine/OrderBookTest.java`:
```java
package javamatch.engine;

import org.junit.jupiter.api.Test;

import java.util.List;

import static javamatch.engine.EngineAsserts.assertEvents;
import static org.junit.jupiter.api.Assertions.assertEquals;

class OrderBookTest {
    @Test
    void rejectsInvalidOrders() {
        OrderBook b = new OrderBook();
        assertEvents(b.newLimitOrder(new NewOrderCmd(7, 1, Side.BUY, 10, 0)),
            List.of(new Event(EventType.REJECTED).clientOrderId(7).owner(1).reason(RejectReason.BAD_QTY)));
        assertEvents(b.newLimitOrder(new NewOrderCmd(8, 1, Side.BUY, -5, 10)),
            List.of(new Event(EventType.REJECTED).clientOrderId(8).owner(1).reason(RejectReason.BAD_PRICE)));
    }

    @Test
    void restingOrderAcceptedWithBookUpdate() {
        OrderBook b = new OrderBook();
        assertEvents(b.newLimitOrder(new NewOrderCmd(7, 1, Side.BUY, 100, 30)), List.of(
            new Event(EventType.ACCEPTED).orderId(1).clientOrderId(7).owner(1).side(Side.BUY).price(100).qty(30),
            new Event(EventType.BOOK_UPDATE).side(Side.BUY).price(100).aggregateQty(30)));
    }

    @Test
    void orderIdsIncrease() {
        OrderBook b = new OrderBook();
        List<Event> first = b.newLimitOrder(new NewOrderCmd(1, 1, Side.BUY, 100, 1));
        List<Event> second = b.newLimitOrder(new NewOrderCmd(2, 1, Side.SELL, 200, 1));
        assertEquals(1, first.get(0).orderId);
        assertEquals(2, second.get(0).orderId);
    }

    @Test
    void sameLevelAggregates() {
        OrderBook b = new OrderBook();
        b.newLimitOrder(new NewOrderCmd(1, 1, Side.BUY, 100, 30));
        assertEvents(b.newLimitOrder(new NewOrderCmd(2, 2, Side.BUY, 100, 20)), List.of(
            new Event(EventType.ACCEPTED).orderId(2).clientOrderId(2).owner(2).side(Side.BUY).price(100).qty(20),
            new Event(EventType.BOOK_UPDATE).side(Side.BUY).price(100).aggregateQty(50)));
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
./gradlew test --tests 'javamatch.engine.OrderBookTest'
```
Expected: compilation FAILURE (classes don't exist).

- [ ] **Step 3: Implement the engine types and OrderBook (accept/rest path + matching skeleton)**

Note: `Event.side` defaults to `Side.BUY` (Go zero value) and `reason` to `RejectReason.NONE` so untouched fields compare equal to Go-style zero-value expectations.

`src/main/java/javamatch/engine/Side.java`:
```java
package javamatch.engine;

/** Order side; codes match SBE schema 901 and the Go engine (BUY=0, SELL=1). */
public enum Side {
    BUY((byte) 0),
    SELL((byte) 1);

    private final byte code;

    Side(byte code) {
        this.code = code;
    }

    public byte code() {
        return code;
    }

    public static Side fromCode(byte code) {
        return code == 0 ? BUY : SELL;
    }
}
```

`src/main/java/javamatch/engine/EventType.java`:
```java
package javamatch.engine;

public enum EventType {
    ACCEPTED,
    REJECTED,
    TRADE,
    FILLED,
    CANCELED,
    BOOK_UPDATE
}
```

`src/main/java/javamatch/engine/RejectReason.java`:
```java
package javamatch.engine;

public enum RejectReason {
    NONE((byte) 0),
    BAD_QTY((byte) 1),
    BAD_PRICE((byte) 2),
    UNKNOWN_ORDER((byte) 3),
    NOT_OWNER((byte) 4);

    private final byte code;

    RejectReason(byte code) {
        this.code = code;
    }

    public byte code() {
        return code;
    }
}
```

`src/main/java/javamatch/engine/NewOrderCmd.java`:
```java
package javamatch.engine;

public record NewOrderCmd(long clientOrderId, long owner, Side side, long price, long qty) {
}
```

`src/main/java/javamatch/engine/Event.java`:
```java
package javamatch.engine;

import java.util.Objects;

/**
 * Flat tagged union of everything the engine can emit; which fields are
 * meaningful depends on type (see the Go engine's Event doc):
 *
 *  ACCEPTED:    orderId, clientOrderId, owner, side, price, qty
 *  REJECTED:    orderId (cancels), clientOrderId (orders), owner, reason
 *  TRADE:       price, qty, makerOrderId, takerOrderId, makerOwner, takerOwner
 *  FILLED:      orderId, clientOrderId, owner, side, price, qty (fill), remainingQty
 *  CANCELED:    orderId, clientOrderId, owner, side, price, remainingQty (qty canceled)
 *  BOOK_UPDATE: side, price, aggregateQty (0 = level gone)
 */
public final class Event {
    public EventType type;
    public long orderId;
    public long clientOrderId;
    public long owner;
    public Side side = Side.BUY;
    public long price;
    public long qty;
    public long remainingQty;
    public RejectReason reason = RejectReason.NONE;
    public long makerOrderId;
    public long takerOrderId;
    public long makerOwner;
    public long takerOwner;
    public long aggregateQty;

    public Event(EventType type) {
        this.type = type;
    }

    public Event orderId(long v) { orderId = v; return this; }
    public Event clientOrderId(long v) { clientOrderId = v; return this; }
    public Event owner(long v) { owner = v; return this; }
    public Event side(Side v) { side = v; return this; }
    public Event price(long v) { price = v; return this; }
    public Event qty(long v) { qty = v; return this; }
    public Event remainingQty(long v) { remainingQty = v; return this; }
    public Event reason(RejectReason v) { reason = v; return this; }
    public Event makerOrderId(long v) { makerOrderId = v; return this; }
    public Event takerOrderId(long v) { takerOrderId = v; return this; }
    public Event makerOwner(long v) { makerOwner = v; return this; }
    public Event takerOwner(long v) { takerOwner = v; return this; }
    public Event aggregateQty(long v) { aggregateQty = v; return this; }

    @Override
    public boolean equals(Object o) {
        if (this == o) {
            return true;
        }
        if (!(o instanceof Event e)) {
            return false;
        }
        return type == e.type && orderId == e.orderId && clientOrderId == e.clientOrderId
            && owner == e.owner && side == e.side && price == e.price && qty == e.qty
            && remainingQty == e.remainingQty && reason == e.reason
            && makerOrderId == e.makerOrderId && takerOrderId == e.takerOrderId
            && makerOwner == e.makerOwner && takerOwner == e.takerOwner
            && aggregateQty == e.aggregateQty;
    }

    @Override
    public int hashCode() {
        return Objects.hash(type, orderId, clientOrderId, owner, side, price, qty,
            remainingQty, reason, makerOrderId, takerOrderId, makerOwner, takerOwner, aggregateQty);
    }

    @Override
    public String toString() {
        return "Event{" + type + " orderId=" + orderId + " clientOrderId=" + clientOrderId
            + " owner=" + owner + " side=" + side + " price=" + price + " qty=" + qty
            + " remainingQty=" + remainingQty + " reason=" + reason
            + " makerOrderId=" + makerOrderId + " takerOrderId=" + takerOrderId
            + " makerOwner=" + makerOwner + " takerOwner=" + takerOwner
            + " aggregateQty=" + aggregateQty + '}';
    }
}
```

`src/main/java/javamatch/engine/OrderBook.java` — full matching core, a line-for-line port of `engine/order_book.go` (the match loop is included now; Task 3's tests exercise it, Task 4 adds `cancel`):
```java
package javamatch.engine;

import org.agrona.collections.Long2ObjectHashMap;

import java.util.ArrayList;
import java.util.List;

/**
 * Single-instrument limit order book: price levels sorted best-first per
 * side, FIFO within a level. Pure and deterministic — no I/O, integer ticks.
 */
public final class OrderBook {
    static final class Order {
        long id;
        long clientOrderId;
        long owner;
        Side side;
        long price;
        long qty; // remaining quantity
        PriceLevel level;
        Order prev; // FIFO within the level
        Order next;
    }

    static final class PriceLevel {
        final long price;
        long totalQty;
        Order head;
        Order tail;

        PriceLevel(long price) {
            this.price = price;
        }
    }

    private final ArrayList<PriceLevel> bids = new ArrayList<>(); // sorted best-first: highest price first
    private final ArrayList<PriceLevel> asks = new ArrayList<>(); // sorted best-first: lowest price first
    private final Long2ObjectHashMap<Order> orders = new Long2ObjectHashMap<>();
    private long nextOrderId = 1;

    /**
     * Binary-searches levels for price; desc is true for bids. Returns the
     * index when found, else {@code -(insertionIndex) - 1} (the
     * Arrays.binarySearch convention).
     */
    private static int levelIndex(ArrayList<PriceLevel> levels, long price, boolean desc) {
        int lo = 0;
        int hi = levels.size();
        while (lo < hi) {
            int mid = (lo + hi) >>> 1;
            long p = levels.get(mid).price;
            if (p == price) {
                return mid;
            }
            if (desc == (p > price)) {
                lo = mid + 1;
            } else {
                hi = mid;
            }
        }
        return -lo - 1;
    }

    public List<Event> newLimitOrder(NewOrderCmd cmd) {
        List<Event> events = new ArrayList<>();
        if (cmd.qty() <= 0) {
            events.add(new Event(EventType.REJECTED)
                .clientOrderId(cmd.clientOrderId()).owner(cmd.owner()).reason(RejectReason.BAD_QTY));
            return events;
        }
        if (cmd.price() <= 0) {
            events.add(new Event(EventType.REJECTED)
                .clientOrderId(cmd.clientOrderId()).owner(cmd.owner()).reason(RejectReason.BAD_PRICE));
            return events;
        }
        Order taker = new Order();
        taker.id = nextOrderId++;
        taker.clientOrderId = cmd.clientOrderId();
        taker.owner = cmd.owner();
        taker.side = cmd.side();
        taker.price = cmd.price();
        taker.qty = cmd.qty();
        events.add(new Event(EventType.ACCEPTED)
            .orderId(taker.id).clientOrderId(taker.clientOrderId).owner(taker.owner)
            .side(taker.side).price(taker.price).qty(taker.qty));

        // Track levels whose aggregate changed, in order of first change, so
        // the final BookUpdate batch is deterministic without map iteration.
        ArrayList<PriceLevel> changed = new ArrayList<>();
        ArrayList<Side> changedSides = new ArrayList<>();

        match(taker, events, changed, changedSides);
        if (taker.qty > 0) {
            PriceLevel lvl = restOrder(taker);
            touch(changed, changedSides, taker.side, lvl);
        }
        for (int i = 0; i < changed.size(); i++) {
            PriceLevel lvl = changed.get(i);
            events.add(new Event(EventType.BOOK_UPDATE)
                .side(changedSides.get(i)).price(lvl.price).aggregateQty(lvl.totalQty));
        }
        return events;
    }

    private static void touch(ArrayList<PriceLevel> changed, ArrayList<Side> changedSides, Side side, PriceLevel lvl) {
        for (PriceLevel l : changed) {
            if (l == lvl) {
                return;
            }
        }
        changed.add(lvl);
        changedSides.add(side);
    }

    private void match(Order taker, List<Event> events, ArrayList<PriceLevel> changed, ArrayList<Side> changedSides) {
        while (taker.qty > 0) {
            ArrayList<PriceLevel> opp = taker.side == Side.SELL ? bids : asks;
            Side oppSide = taker.side == Side.SELL ? Side.BUY : Side.SELL;
            if (opp.isEmpty()) {
                break;
            }
            PriceLevel best = opp.get(0);
            if (taker.side == Side.BUY && best.price > taker.price) {
                break;
            }
            if (taker.side == Side.SELL && best.price < taker.price) {
                break;
            }
            Order maker = best.head;
            long fill = Math.min(taker.qty, maker.qty);
            maker.qty -= fill;
            taker.qty -= fill;
            best.totalQty -= fill;
            touch(changed, changedSides, oppSide, best);
            events.add(new Event(EventType.TRADE)
                .price(best.price).qty(fill)
                .makerOrderId(maker.id).takerOrderId(taker.id)
                .makerOwner(maker.owner).takerOwner(taker.owner));
            events.add(new Event(EventType.FILLED)
                .orderId(maker.id).clientOrderId(maker.clientOrderId).owner(maker.owner)
                .side(maker.side).price(best.price).qty(fill).remainingQty(maker.qty));
            events.add(new Event(EventType.FILLED)
                .orderId(taker.id).clientOrderId(taker.clientOrderId).owner(taker.owner)
                .side(taker.side).price(best.price).qty(fill).remainingQty(taker.qty));
            if (maker.qty == 0) {
                unlink(maker);
            }
        }
    }

    /**
     * Removes an order from its level FIFO and the order map, and removes the
     * level from its side when it becomes empty. Does not touch totalQty:
     * callers account for quantity themselves.
     */
    private void unlink(Order o) {
        PriceLevel lvl = o.level;
        if (o.prev != null) {
            o.prev.next = o.next;
        } else {
            lvl.head = o.next;
        }
        if (o.next != null) {
            o.next.prev = o.prev;
        } else {
            lvl.tail = o.prev;
        }
        o.prev = null;
        o.next = null;
        o.level = null;
        orders.remove(o.id);
        if (lvl.head == null) {
            ArrayList<PriceLevel> levels = o.side == Side.BUY ? bids : asks;
            boolean desc = o.side == Side.BUY;
            int idx = levelIndex(levels, lvl.price, desc);
            if (idx >= 0) {
                levels.remove(idx);
            }
        }
    }

    PriceLevel restOrder(Order o) {
        ArrayList<PriceLevel> levels = o.side == Side.BUY ? bids : asks;
        boolean desc = o.side == Side.BUY;
        int idx = levelIndex(levels, o.price, desc);
        PriceLevel lvl;
        if (idx >= 0) {
            lvl = levels.get(idx);
        } else {
            lvl = new PriceLevel(o.price);
            levels.add(-idx - 1, lvl);
        }
        o.level = lvl;
        if (lvl.tail == null) {
            lvl.head = o;
            lvl.tail = o;
        } else {
            o.prev = lvl.tail;
            lvl.tail.next = o;
            lvl.tail = o;
        }
        lvl.totalQty += o.qty;
        orders.put(o.id, o);
        return lvl;
    }
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
./gradlew test --tests 'javamatch.engine.OrderBookTest'
```
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "Engine types and order book resting/reject behavior"
```

---

### Task 3: Matching tests

**Files:**
- Test: `src/test/java/javamatch/engine/MatchingTest.java`

**Interfaces:**
- Consumes: `OrderBook.newLimitOrder`, `Event` fluent setters, `EngineAsserts.assertEvents` from Task 2. The match loop already exists; this task pins its behavior with the ported `engine/matching_test.go`.

- [ ] **Step 1: Write the test**

`src/test/java/javamatch/engine/MatchingTest.java`:
```java
package javamatch.engine;

import org.junit.jupiter.api.Test;

import java.util.List;

import static javamatch.engine.EngineAsserts.assertEvents;
import static org.junit.jupiter.api.Assertions.assertEquals;

class MatchingTest {
    @Test
    void fullFillSingleLevel() {
        OrderBook b = new OrderBook();
        b.newLimitOrder(new NewOrderCmd(10, 1, Side.SELL, 100, 50)); // id 1 rests
        List<Event> got = b.newLimitOrder(new NewOrderCmd(20, 2, Side.BUY, 100, 50));
        assertEvents(got, List.of(
            new Event(EventType.ACCEPTED).orderId(2).clientOrderId(20).owner(2).side(Side.BUY).price(100).qty(50),
            new Event(EventType.TRADE).price(100).qty(50).makerOrderId(1).takerOrderId(2).makerOwner(1).takerOwner(2),
            new Event(EventType.FILLED).orderId(1).clientOrderId(10).owner(1).side(Side.SELL).price(100).qty(50).remainingQty(0),
            new Event(EventType.FILLED).orderId(2).clientOrderId(20).owner(2).side(Side.BUY).price(100).qty(50).remainingQty(0),
            new Event(EventType.BOOK_UPDATE).side(Side.SELL).price(100).aggregateQty(0)));
    }

    @Test
    void partialFillRemainderRests() {
        OrderBook b = new OrderBook();
        b.newLimitOrder(new NewOrderCmd(10, 1, Side.SELL, 100, 30)); // id 1
        List<Event> got = b.newLimitOrder(new NewOrderCmd(20, 2, Side.BUY, 100, 50));
        assertEvents(got, List.of(
            new Event(EventType.ACCEPTED).orderId(2).clientOrderId(20).owner(2).side(Side.BUY).price(100).qty(50),
            new Event(EventType.TRADE).price(100).qty(30).makerOrderId(1).takerOrderId(2).makerOwner(1).takerOwner(2),
            new Event(EventType.FILLED).orderId(1).clientOrderId(10).owner(1).side(Side.SELL).price(100).qty(30).remainingQty(0),
            new Event(EventType.FILLED).orderId(2).clientOrderId(20).owner(2).side(Side.BUY).price(100).qty(30).remainingQty(20),
            new Event(EventType.BOOK_UPDATE).side(Side.SELL).price(100).aggregateQty(0),
            new Event(EventType.BOOK_UPDATE).side(Side.BUY).price(100).aggregateQty(20)));
    }

    @Test
    void tradesAtMakerPrice() {
        OrderBook b = new OrderBook();
        b.newLimitOrder(new NewOrderCmd(10, 1, Side.SELL, 100, 10)); // id 1
        List<Event> got = b.newLimitOrder(new NewOrderCmd(20, 2, Side.BUY, 105, 10));
        assertEquals(EventType.TRADE, got.get(1).type);
        assertEquals(100, got.get(1).price);
    }

    @Test
    void nonCrossingOrderRests() {
        OrderBook b = new OrderBook();
        b.newLimitOrder(new NewOrderCmd(10, 1, Side.SELL, 101, 10));
        List<Event> got = b.newLimitOrder(new NewOrderCmd(20, 2, Side.BUY, 100, 10));
        assertEvents(got, List.of(
            new Event(EventType.ACCEPTED).orderId(2).clientOrderId(20).owner(2).side(Side.BUY).price(100).qty(10),
            new Event(EventType.BOOK_UPDATE).side(Side.BUY).price(100).aggregateQty(10)));
    }

    @Test
    void sweepsMultipleLevels() {
        OrderBook b = new OrderBook();
        b.newLimitOrder(new NewOrderCmd(10, 1, Side.SELL, 100, 30)); // id 1
        b.newLimitOrder(new NewOrderCmd(11, 1, Side.SELL, 101, 40)); // id 2
        List<Event> got = b.newLimitOrder(new NewOrderCmd(20, 2, Side.BUY, 101, 100));
        assertEvents(got, List.of(
            new Event(EventType.ACCEPTED).orderId(3).clientOrderId(20).owner(2).side(Side.BUY).price(101).qty(100),
            new Event(EventType.TRADE).price(100).qty(30).makerOrderId(1).takerOrderId(3).makerOwner(1).takerOwner(2),
            new Event(EventType.FILLED).orderId(1).clientOrderId(10).owner(1).side(Side.SELL).price(100).qty(30).remainingQty(0),
            new Event(EventType.FILLED).orderId(3).clientOrderId(20).owner(2).side(Side.BUY).price(100).qty(30).remainingQty(70),
            new Event(EventType.TRADE).price(101).qty(40).makerOrderId(2).takerOrderId(3).makerOwner(1).takerOwner(2),
            new Event(EventType.FILLED).orderId(2).clientOrderId(11).owner(1).side(Side.SELL).price(101).qty(40).remainingQty(0),
            new Event(EventType.FILLED).orderId(3).clientOrderId(20).owner(2).side(Side.BUY).price(101).qty(40).remainingQty(30),
            new Event(EventType.BOOK_UPDATE).side(Side.SELL).price(100).aggregateQty(0),
            new Event(EventType.BOOK_UPDATE).side(Side.SELL).price(101).aggregateQty(0),
            new Event(EventType.BOOK_UPDATE).side(Side.BUY).price(101).aggregateQty(30)));
    }

    @Test
    void fifoPriorityWithinLevel() {
        OrderBook b = new OrderBook();
        b.newLimitOrder(new NewOrderCmd(10, 1, Side.SELL, 100, 30)); // id 1, first
        b.newLimitOrder(new NewOrderCmd(11, 3, Side.SELL, 100, 40)); // id 2, second
        List<Event> got = b.newLimitOrder(new NewOrderCmd(20, 2, Side.BUY, 100, 50));
        // id 1 must fill completely before id 2 trades at all.
        assertEvents(got, List.of(
            new Event(EventType.ACCEPTED).orderId(3).clientOrderId(20).owner(2).side(Side.BUY).price(100).qty(50),
            new Event(EventType.TRADE).price(100).qty(30).makerOrderId(1).takerOrderId(3).makerOwner(1).takerOwner(2),
            new Event(EventType.FILLED).orderId(1).clientOrderId(10).owner(1).side(Side.SELL).price(100).qty(30).remainingQty(0),
            new Event(EventType.FILLED).orderId(3).clientOrderId(20).owner(2).side(Side.BUY).price(100).qty(30).remainingQty(20),
            new Event(EventType.TRADE).price(100).qty(20).makerOrderId(2).takerOrderId(3).makerOwner(3).takerOwner(2),
            new Event(EventType.FILLED).orderId(2).clientOrderId(11).owner(3).side(Side.SELL).price(100).qty(20).remainingQty(20),
            new Event(EventType.FILLED).orderId(3).clientOrderId(20).owner(2).side(Side.BUY).price(100).qty(20).remainingQty(0),
            new Event(EventType.BOOK_UPDATE).side(Side.SELL).price(100).aggregateQty(20)));
    }

    @Test
    void bestPriceOrdering() {
        OrderBook b = new OrderBook();
        b.newLimitOrder(new NewOrderCmd(1, 1, Side.SELL, 102, 10)); // id 1
        b.newLimitOrder(new NewOrderCmd(2, 1, Side.SELL, 100, 10)); // id 2 - better
        List<Event> got = b.newLimitOrder(new NewOrderCmd(3, 2, Side.BUY, 102, 10));
        assertEquals(2, got.get(1).makerOrderId, "expected best ask (id 2 @100) to fill first");
    }
}
```

- [ ] **Step 2: Run the test**

```bash
./gradlew test --tests 'javamatch.engine.MatchingTest'
```
Expected: PASS (7 tests) — the match loop shipped in Task 2. If any fail, fix `OrderBook.match` until the event stream matches; do not adjust expectations (they are the Go engine's exact output).

- [ ] **Step 3: Commit**

```bash
git add -A && git commit -m "Port matching tests pinning trade/fill/book-update event order"
```

---

### Task 4: Cancel

**Files:**
- Modify: `src/main/java/javamatch/engine/OrderBook.java` (add `cancel`)
- Test: `src/test/java/javamatch/engine/CancelTest.java`

**Interfaces:**
- Produces: `public List<Event> cancel(long orderId, long requestingOwner)` on `OrderBook` — used by `MatchingService` (Task 7).

- [ ] **Step 1: Write the failing test** (port of `engine/cancel_test.go`)

`src/test/java/javamatch/engine/CancelTest.java`:
```java
package javamatch.engine;

import org.junit.jupiter.api.Test;

import java.util.List;

import static javamatch.engine.EngineAsserts.assertEvents;
import static org.junit.jupiter.api.Assertions.assertEquals;

class CancelTest {
    @Test
    void cancelRestingOrder() {
        OrderBook b = new OrderBook();
        b.newLimitOrder(new NewOrderCmd(10, 1, Side.SELL, 100, 30)); // id 1
        assertEvents(b.cancel(1, 1), List.of(
            new Event(EventType.CANCELED).orderId(1).clientOrderId(10).owner(1).side(Side.SELL).price(100).remainingQty(30),
            new Event(EventType.BOOK_UPDATE).side(Side.SELL).price(100).aggregateQty(0)));
        // Canceled order must be gone: cancel again is unknown.
        assertEvents(b.cancel(1, 1),
            List.of(new Event(EventType.REJECTED).orderId(1).owner(1).reason(RejectReason.UNKNOWN_ORDER)));
    }

    @Test
    void cancelNotOwner() {
        OrderBook b = new OrderBook();
        b.newLimitOrder(new NewOrderCmd(10, 1, Side.SELL, 100, 30)); // id 1
        assertEvents(b.cancel(1, 99),
            List.of(new Event(EventType.REJECTED).orderId(1).owner(99).reason(RejectReason.NOT_OWNER)));
    }

    @Test
    void cancelLeavesRestOfLevel() {
        OrderBook b = new OrderBook();
        b.newLimitOrder(new NewOrderCmd(10, 1, Side.SELL, 100, 30)); // id 1
        b.newLimitOrder(new NewOrderCmd(11, 1, Side.SELL, 100, 20)); // id 2
        assertEvents(b.cancel(1, 1), List.of(
            new Event(EventType.CANCELED).orderId(1).clientOrderId(10).owner(1).side(Side.SELL).price(100).remainingQty(30),
            new Event(EventType.BOOK_UPDATE).side(Side.SELL).price(100).aggregateQty(20)));
        // id 2 must still match.
        List<Event> fills = b.newLimitOrder(new NewOrderCmd(20, 2, Side.BUY, 100, 20));
        assertEquals(EventType.TRADE, fills.get(1).type);
        assertEquals(2, fills.get(1).makerOrderId);
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
./gradlew test --tests 'javamatch.engine.CancelTest'
```
Expected: compilation FAILURE (`cancel` not defined).

- [ ] **Step 3: Implement `cancel`** — add to `OrderBook`:

```java
    /**
     * Removes a resting order. The engine owns the order map, so it performs
     * the ownership check: a cancel from a non-owner is rejected.
     */
    public List<Event> cancel(long orderId, long requestingOwner) {
        List<Event> events = new ArrayList<>();
        Order o = orders.get(orderId);
        if (o == null) {
            events.add(new Event(EventType.REJECTED)
                .orderId(orderId).owner(requestingOwner).reason(RejectReason.UNKNOWN_ORDER));
            return events;
        }
        if (o.owner != requestingOwner) {
            events.add(new Event(EventType.REJECTED)
                .orderId(orderId).owner(requestingOwner).reason(RejectReason.NOT_OWNER));
            return events;
        }
        PriceLevel lvl = o.level;
        lvl.totalQty -= o.qty;
        unlink(o);
        events.add(new Event(EventType.CANCELED)
            .orderId(o.id).clientOrderId(o.clientOrderId).owner(o.owner)
            .side(o.side).price(o.price).remainingQty(o.qty));
        events.add(new Event(EventType.BOOK_UPDATE)
            .side(o.side).price(lvl.price).aggregateQty(lvl.totalQty));
        return events;
    }
```

- [ ] **Step 4: Run test to verify it passes**

```bash
./gradlew test --tests 'javamatch.engine.CancelTest'
```
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "Engine cancel with ownership check"
```

---

### Task 5: Snapshot + determinism

**Files:**
- Create: `src/main/java/javamatch/engine/Snapshots.java`
- Modify: `src/main/java/javamatch/engine/OrderBook.java` (accessors for snapshot state)
- Test: `src/test/java/javamatch/engine/SnapshotTest.java`, `src/test/java/javamatch/engine/DeterminismTest.java`

**Interfaces:**
- Produces (used by `MatchingService`, Task 7):
  - `Snapshots.write(OrderBook book, ExpandableArrayBuffer out)` → `int` length written at offset 0.
  - `Snapshots.restore(DirectBuffer in, int offset, int length)` → `OrderBook`; throws `IllegalArgumentException` whose message contains `"magic"` on bad magic, `"version"` on bad version, `"truncated"` on short input.
- Byte format: exactly the Go format (see Global Constraints). `Snapshots` lives in the engine package so it can touch `OrderBook` internals (`bids`, `asks`, `orders`, `nextOrderId`, `restOrder`) — keep those package-private, do NOT widen to public.

- [ ] **Step 1: Write the failing tests** (ports of `engine/snapshot_test.go` and `engine/determinism_test.go`)

`src/test/java/javamatch/engine/SnapshotTest.java`:
```java
package javamatch.engine;

import org.agrona.ExpandableArrayBuffer;
import org.agrona.concurrent.UnsafeBuffer;
import org.junit.jupiter.api.Test;

import java.util.Arrays;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

class SnapshotTest {
    static OrderBook populatedBook() {
        OrderBook b = new OrderBook();
        b.newLimitOrder(new NewOrderCmd(1, 1, Side.BUY, 99, 10));
        b.newLimitOrder(new NewOrderCmd(2, 2, Side.BUY, 100, 20));
        b.newLimitOrder(new NewOrderCmd(3, 1, Side.BUY, 100, 5));
        b.newLimitOrder(new NewOrderCmd(4, 3, Side.SELL, 101, 7));
        b.newLimitOrder(new NewOrderCmd(5, 3, Side.SELL, 103, 9));
        return b;
    }

    static byte[] snapshotBytes(OrderBook b) {
        ExpandableArrayBuffer buf = new ExpandableArrayBuffer();
        int len = Snapshots.write(b, buf);
        return Arrays.copyOf(buf.byteArray(), len);
    }

    @Test
    void snapshotRoundTrip() {
        OrderBook b = populatedBook();
        byte[] first = snapshotBytes(b);
        OrderBook restored = Snapshots.restore(new UnsafeBuffer(first), 0, first.length);
        byte[] again = snapshotBytes(restored);
        assertTrue(Arrays.equals(first, again), "snapshot of restored book differs from original");
    }

    @Test
    void restorePreservesIdsAndMatching() {
        OrderBook b = populatedBook();
        byte[] snap = snapshotBytes(b);
        OrderBook restored = Snapshots.restore(new UnsafeBuffer(snap), 0, snap.length);
        // Next id continues the sequence (populatedBook used ids 1-5).
        List<Event> got = restored.newLimitOrder(new NewOrderCmd(9, 9, Side.SELL, 100, 25));
        assertEquals(6, got.get(0).orderId);
        // It must cross the restored best bid level (100: id 2 qty 20 then id 3 qty 5).
        assertEquals(EventType.TRADE, got.get(1).type);
        assertEquals(2, got.get(1).makerOrderId);
        assertEquals(20, got.get(1).qty);
        // Cancels of restored orders work (map rebuilt).
        assertEquals(EventType.CANCELED, restored.cancel(1, 1).get(0).type);
    }

    @Test
    void restoreRejectsBadHeader() {
        // A full-size header (magic + version + instrument + nextOrderId +
        // count = 28 bytes) with the wrong magic must be rejected.
        byte[] bad = new byte[28];
        IllegalArgumentException ex = assertThrows(IllegalArgumentException.class,
            () -> Snapshots.restore(new UnsafeBuffer(bad), 0, bad.length));
        assertTrue(ex.getMessage().contains("magic"), () -> "expected bad-magic error, got " + ex.getMessage());
        // Truncated input must also fail.
        assertThrows(IllegalArgumentException.class,
            () -> Snapshots.restore(new UnsafeBuffer(bad), 0, 8));
    }
}
```

`src/test/java/javamatch/engine/DeterminismTest.java`:
```java
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
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
./gradlew test --tests 'javamatch.engine.SnapshotTest' --tests 'javamatch.engine.DeterminismTest'
```
Expected: compilation FAILURE (`Snapshots` not defined).

- [ ] **Step 3: Implement `Snapshots`**

First expose package-private accessors on `OrderBook` (add to the class):
```java
    ArrayList<PriceLevel> bids() { return bids; }
    ArrayList<PriceLevel> asks() { return asks; }
    int orderCount() { return orders.size(); }
    long nextOrderId() { return nextOrderId; }
    void nextOrderId(long v) { nextOrderId = v; }
```

`src/main/java/javamatch/engine/Snapshots.java`:
```java
package javamatch.engine;

import org.agrona.DirectBuffer;
import org.agrona.ExpandableArrayBuffer;

import java.nio.ByteOrder;
import java.util.ArrayList;

/**
 * Snapshot I/O in the exact gomatch byte format (magic "GMS1", little-endian):
 * header of magic uint32, version int32, reserved instrument int64,
 * nextOrderId int64, count int32; then per resting order id, clientOrderId,
 * owner int64s, side int8, price, qty int64s — bids best-to-worst FIFO, then
 * asks. A Java engine can restore a Go snapshot and vice versa.
 */
public final class Snapshots {
    static final int MAGIC = 0x474D5331; // "GMS1"
    static final int VERSION = 1;
    static final int HEADER_LENGTH = 4 + 4 + 8 + 8 + 4;
    static final int ORDER_LENGTH = 8 + 8 + 8 + 1 + 8 + 8;
    private static final ByteOrder LE = ByteOrder.LITTLE_ENDIAN;

    private Snapshots() {}

    /** Writes the complete book state at offset 0; returns the length. */
    public static int write(OrderBook book, ExpandableArrayBuffer out) {
        int off = 0;
        out.putInt(off, MAGIC, LE);
        off += 4;
        out.putInt(off, VERSION, LE);
        off += 4;
        out.putLong(off, 1L, LE); // reserved instrument id
        off += 8;
        out.putLong(off, book.nextOrderId(), LE);
        off += 8;
        out.putInt(off, book.orderCount(), LE);
        off += 4;
        off = writeSide(book.bids(), out, off);
        off = writeSide(book.asks(), out, off);
        return off;
    }

    private static int writeSide(ArrayList<OrderBook.PriceLevel> levels, ExpandableArrayBuffer out, int off) {
        for (OrderBook.PriceLevel lvl : levels) {
            for (OrderBook.Order o = lvl.head; o != null; o = o.next) {
                out.putLong(off, o.id, LE);
                off += 8;
                out.putLong(off, o.clientOrderId, LE);
                off += 8;
                out.putLong(off, o.owner, LE);
                off += 8;
                out.putByte(off, o.side.code());
                off += 1;
                out.putLong(off, o.price, LE);
                off += 8;
                out.putLong(off, o.qty, LE);
                off += 8;
            }
        }
        return off;
    }

    /**
     * Rebuilds a book from snapshot bytes. Orders are re-rested without
     * matching; the id sequence continues where it left off.
     */
    public static OrderBook restore(DirectBuffer in, int offset, int length) {
        if (length < HEADER_LENGTH) {
            throw new IllegalArgumentException("truncated snapshot header: " + length + " bytes");
        }
        int off = offset;
        int magic = in.getInt(off, LE);
        off += 4;
        if (magic != MAGIC) {
            throw new IllegalArgumentException(String.format("bad snapshot magic 0x%x", magic));
        }
        int version = in.getInt(off, LE);
        off += 4;
        if (version != VERSION) {
            throw new IllegalArgumentException("unsupported snapshot version " + version);
        }
        off += 8; // reserved instrument id
        long nextOrderId = in.getLong(off, LE);
        off += 8;
        int count = in.getInt(off, LE);
        off += 4;
        if (length < HEADER_LENGTH + (long) count * ORDER_LENGTH) {
            throw new IllegalArgumentException("truncated snapshot body: " + length + " bytes for " + count + " orders");
        }
        OrderBook b = new OrderBook();
        for (int i = 0; i < count; i++) {
            OrderBook.Order o = new OrderBook.Order();
            o.id = in.getLong(off, LE);
            off += 8;
            o.clientOrderId = in.getLong(off, LE);
            off += 8;
            o.owner = in.getLong(off, LE);
            off += 8;
            o.side = Side.fromCode(in.getByte(off));
            off += 1;
            o.price = in.getLong(off, LE);
            off += 8;
            o.qty = in.getLong(off, LE);
            off += 8;
            b.restOrder(o);
        }
        b.nextOrderId(nextOrderId);
        return b;
    }
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
./gradlew test --tests 'javamatch.engine.SnapshotTest' --tests 'javamatch.engine.DeterminismTest'
```
Expected: PASS (4 tests).

- [ ] **Step 5: Cross-language snapshot check (one-off, not committed as a test)**

Verify byte-compatibility against Go using the deterministic populated book:
```bash
cd /home/claude/ultima/gomatch && cat > /tmp/snapdump_test.go <<'EOF'
package engine

import (
    "bytes"
    "os"
    "testing"
)

func TestDumpSnapshotForJavaCompat(t *testing.T) {
    b := NewOrderBook()
    b.NewLimitOrder(NewOrderCmd{ClientOrderId: 1, Owner: 1, Side: Buy, Price: 99, Qty: 10})
    b.NewLimitOrder(NewOrderCmd{ClientOrderId: 2, Owner: 2, Side: Buy, Price: 100, Qty: 20})
    b.NewLimitOrder(NewOrderCmd{ClientOrderId: 3, Owner: 1, Side: Buy, Price: 100, Qty: 5})
    b.NewLimitOrder(NewOrderCmd{ClientOrderId: 4, Owner: 3, Side: Sell, Price: 101, Qty: 7})
    b.NewLimitOrder(NewOrderCmd{ClientOrderId: 5, Owner: 3, Side: Sell, Price: 103, Qty: 9})
    var buf bytes.Buffer
    if err := b.Snapshot(&buf); err != nil { t.Fatal(err) }
    os.WriteFile("/tmp/go-snapshot.bin", buf.Bytes(), 0o644)
}
EOF
cp /tmp/snapdump_test.go engine/ && go test ./engine/ -run TestDumpSnapshotForJavaCompat && rm engine/snapdump_test.go
```
Then in javamatch, add a temporary JUnit test (or `jshell`) asserting `snapshotBytes(populatedBook())` equals the bytes of `/tmp/go-snapshot.bin`; simplest is a scratch test file deleted after checking:
```java
// scratch check inside SnapshotTest (delete after running once):
// assertArrayEquals(java.nio.file.Files.readAllBytes(java.nio.file.Path.of("/tmp/go-snapshot.bin")),
//     snapshotBytes(populatedBook()));
```
Expected: byte-identical. If not, fix `Snapshots` (field order/width/endianness) — the Go format is the contract.

- [ ] **Step 6: Commit**

```bash
git add -A && git commit -m "Go-compatible snapshot write/restore; determinism test"
```

---

### Task 6: Service egress encoding

**Files:**
- Create: `src/main/java/javamatch/service/Encoding.java`
- Test: `src/test/java/javamatch/service/EncodingTest.java`

**Interfaces:**
- Produces (used by `MatchingService`, Task 7): class `Encoding` (package-private is fine; same package) holding reused flyweights, with methods that encode a full frame (8-byte SBE header + body) at offset 0 of the given buffer and return the frame length:
  - `int encodeExecutionReport(MutableDirectBuffer buf, Event ev, long timestamp)`
  - `int encodeTrade(MutableDirectBuffer buf, Event ev, long timestamp)`
  - `int encodeBookUpdate(MutableDirectBuffer buf, Event ev, long timestamp)`
  - `static OrderStatus statusOf(Event ev)`; `static javamatch.protocol.codecs.Side sideOf(javamatch.engine.Side s)`; `static javamatch.protocol.codecs.RejectReason reasonOf(javamatch.engine.RejectReason r)`

- [ ] **Step 1: Write the failing test** (port of `service/encoding_test.go`)

`src/test/java/javamatch/service/EncodingTest.java`:
```java
package javamatch.service;

import javamatch.engine.Event;
import javamatch.engine.EventType;
import javamatch.engine.Side;
import javamatch.protocol.codecs.BookUpdateDecoder;
import javamatch.protocol.codecs.ExecutionReportDecoder;
import javamatch.protocol.codecs.MessageHeaderDecoder;
import javamatch.protocol.codecs.OrderStatus;
import javamatch.protocol.codecs.TradeEventDecoder;
import org.agrona.ExpandableArrayBuffer;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertEquals;

class EncodingTest {
    private final Encoding encoding = new Encoding();
    private final ExpandableArrayBuffer buf = new ExpandableArrayBuffer();
    private final MessageHeaderDecoder hdr = new MessageHeaderDecoder();

    @Test
    void encodePartialFillExecutionReport() {
        Event ev = new Event(EventType.FILLED).orderId(3).clientOrderId(20).owner(2)
            .side(Side.BUY).price(100).qty(30).remainingQty(20);
        int length = encoding.encodeExecutionReport(buf, ev, 12345);
        assertEquals(MessageHeaderDecoder.ENCODED_LENGTH + ExecutionReportDecoder.BLOCK_LENGTH, length);

        hdr.wrap(buf, 0);
        assertEquals(ExecutionReportDecoder.TEMPLATE_ID, hdr.templateId());
        ExecutionReportDecoder er = new ExecutionReportDecoder()
            .wrap(buf, hdr.encodedLength(), hdr.blockLength(), hdr.version());
        assertEquals(OrderStatus.PARTIALLY_FILLED, er.status());
        assertEquals(30, er.qty());
        assertEquals(20, er.remainingQty());
        assertEquals(12345, er.timestamp());
        assertEquals(3, er.orderId());
        assertEquals(20, er.clientOrderId());
    }

    @Test
    void encodeFullFillStatus() {
        Event ev = new Event(EventType.FILLED).orderId(1).remainingQty(0).qty(30).side(Side.SELL).price(100);
        encoding.encodeExecutionReport(buf, ev, 1);
        hdr.wrap(buf, 0);
        ExecutionReportDecoder er = new ExecutionReportDecoder()
            .wrap(buf, hdr.encodedLength(), hdr.blockLength(), hdr.version());
        assertEquals(OrderStatus.FILLED, er.status());
    }

    @Test
    void encodeTradeAndBookUpdate() {
        Event trade = new Event(EventType.TRADE).price(100).qty(30).makerOrderId(1).takerOrderId(3);
        encoding.encodeTrade(buf, trade, 7);
        hdr.wrap(buf, 0);
        assertEquals(TradeEventDecoder.TEMPLATE_ID, hdr.templateId());
        TradeEventDecoder te = new TradeEventDecoder()
            .wrap(buf, hdr.encodedLength(), hdr.blockLength(), hdr.version());
        assertEquals(100, te.price());
        assertEquals(30, te.qty());
        assertEquals(1, te.makerOrderId());
        assertEquals(3, te.takerOrderId());
        assertEquals(7, te.timestamp());

        Event bu = new Event(EventType.BOOK_UPDATE).side(Side.SELL).price(100).aggregateQty(0);
        encoding.encodeBookUpdate(buf, bu, 8);
        hdr.wrap(buf, 0);
        assertEquals(BookUpdateDecoder.TEMPLATE_ID, hdr.templateId());
        BookUpdateDecoder b = new BookUpdateDecoder()
            .wrap(buf, hdr.encodedLength(), hdr.blockLength(), hdr.version());
        assertEquals(javamatch.protocol.codecs.Side.SELL, b.side());
        assertEquals(0, b.aggregateQty());
        assertEquals(8, b.timestamp());
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
./gradlew test --tests 'javamatch.service.EncodingTest'
```
Expected: compilation FAILURE (`Encoding` not defined).

- [ ] **Step 3: Implement `Encoding`**

`src/main/java/javamatch/service/Encoding.java`:
```java
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
```

- [ ] **Step 4: Run test to verify it passes**

```bash
./gradlew test --tests 'javamatch.service.EncodingTest'
```
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "Service egress encoding with reused SBE flyweights"
```

---

### Task 7: MatchingService

**Files:**
- Create: `src/main/java/javamatch/service/MatchingService.java`
- Test: `src/test/java/javamatch/service/MatchingServiceTest.java`, `src/test/java/javamatch/service/FakeClientSession.java`, `src/test/java/javamatch/service/FakeCluster.java`

**Interfaces:**
- Consumes: `OrderBook`, `Snapshots`, `Encoding` from earlier tasks.
- Produces: `public final class MatchingService implements io.aeron.cluster.service.ClusteredService` with the standard lifecycle plus two package-private test/reuse hooks mirroring Go:
  - `void writeSnapshot(SnapshotChunkConsumer consumer)` where `interface SnapshotChunkConsumer { void accept(org.agrona.DirectBuffer buffer, int offset, int length); }` (nested in `MatchingService`)
  - `void restoreSnapshot(DirectBuffer buffer, int offset, int length)`
- Used by `EngineMain` (Task 9) and the systest harness (Task 10).

- [ ] **Step 1: Write the failing test** (port of `service/service_test.go`)

`src/test/java/javamatch/service/FakeClientSession.java`:
```java
package javamatch.service;

import io.aeron.DirectBufferVector;
import io.aeron.cluster.service.ClientSession;
import io.aeron.logbuffer.BufferClaim;
import org.agrona.DirectBuffer;

import java.util.ArrayList;
import java.util.List;

final class FakeClientSession implements ClientSession {
    final long id;
    final List<byte[]> frames = new ArrayList<>();

    FakeClientSession(long id) {
        this.id = id;
    }

    @Override
    public long id() {
        return id;
    }

    @Override
    public int responseStreamId() {
        return 0;
    }

    @Override
    public String responseChannel() {
        return "";
    }

    @Override
    public byte[] encodedPrincipal() {
        return new byte[0];
    }

    @Override
    public void close() {
    }

    @Override
    public boolean isClosing() {
        return false;
    }

    @Override
    public long offer(DirectBuffer buffer, int offset, int length) {
        byte[] frame = new byte[length];
        buffer.getBytes(offset, frame);
        frames.add(frame);
        return length;
    }

    @Override
    public long offer(DirectBufferVector[] vectors) {
        throw new UnsupportedOperationException();
    }

    @Override
    public long tryClaim(int length, BufferClaim bufferClaim) {
        throw new UnsupportedOperationException();
    }
}
```

`src/test/java/javamatch/service/FakeCluster.java`:
```java
package javamatch.service;

import io.aeron.Aeron;
import io.aeron.DirectBufferVector;
import io.aeron.cluster.service.ClientSession;
import io.aeron.cluster.service.Cluster;
import io.aeron.cluster.service.ClusteredServiceContainer;
import io.aeron.logbuffer.BufferClaim;
import org.agrona.DirectBuffer;
import org.agrona.concurrent.IdleStrategy;
import org.agrona.concurrent.YieldingIdleStrategy;

import java.util.Collection;
import java.util.List;
import java.util.concurrent.TimeUnit;
import java.util.function.Consumer;

final class FakeCluster implements Cluster {
    final long now;
    private final IdleStrategy idleStrategy = new YieldingIdleStrategy();

    FakeCluster(long now) {
        this.now = now;
    }

    @Override
    public int memberId() {
        return 0;
    }

    @Override
    public Role role() {
        return Role.LEADER;
    }

    @Override
    public long logPosition() {
        return 0;
    }

    @Override
    public Aeron aeron() {
        return null;
    }

    @Override
    public ClusteredServiceContainer.Context context() {
        return null;
    }

    @Override
    public ClientSession getClientSession(long clusterSessionId) {
        return null;
    }

    @Override
    public Collection<ClientSession> clientSessions() {
        return List.of();
    }

    @Override
    public void forEachClientSession(Consumer<? super ClientSession> action) {
    }

    @Override
    public boolean closeClientSession(long clusterSessionId) {
        return false;
    }

    @Override
    public long time() {
        return now;
    }

    @Override
    public TimeUnit timeUnit() {
        return TimeUnit.MILLISECONDS;
    }

    @Override
    public boolean scheduleTimer(long correlationId, long deadline) {
        return true;
    }

    @Override
    public boolean cancelTimer(long correlationId) {
        return true;
    }

    @Override
    public long offer(DirectBuffer buffer, int offset, int length) {
        return 0;
    }

    @Override
    public long offer(DirectBufferVector[] vectors) {
        return 0;
    }

    @Override
    public long tryClaim(int length, BufferClaim bufferClaim) {
        return 0;
    }

    @Override
    public IdleStrategy idleStrategy() {
        return idleStrategy;
    }
}
```

If `Cluster` in aeron-all 1.52.0 declares further methods, implement them with the most inert stub (`return false`, `return null`, no-op) — compile errors list them.

`src/test/java/javamatch/service/MatchingServiceTest.java`:
```java
package javamatch.service;

import javamatch.protocol.codecs.CancelOrderEncoder;
import javamatch.protocol.codecs.ExecutionReportDecoder;
import javamatch.protocol.codecs.MessageHeaderDecoder;
import javamatch.protocol.codecs.MessageHeaderEncoder;
import javamatch.protocol.codecs.NewOrderEncoder;
import javamatch.protocol.codecs.OrderStatus;
import javamatch.protocol.codecs.Side;
import io.aeron.cluster.codecs.CloseReason;
import org.agrona.DirectBuffer;
import org.agrona.ExpandableArrayBuffer;
import org.agrona.concurrent.UnsafeBuffer;
import org.junit.jupiter.api.Test;

import java.io.ByteArrayOutputStream;
import java.nio.ByteOrder;
import java.util.ArrayList;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;

class MatchingServiceTest {
    private static UnsafeBuffer newOrderFrame(long clientOrderId, Side side, long price, long qty) {
        ExpandableArrayBuffer buf = new ExpandableArrayBuffer();
        NewOrderEncoder enc = new NewOrderEncoder().wrapAndApplyHeader(buf, 0, new MessageHeaderEncoder());
        enc.clientOrderId(clientOrderId).side(side).price(price).qty(qty);
        int length = MessageHeaderEncoder.ENCODED_LENGTH + enc.encodedLength();
        byte[] bytes = new byte[length];
        buf.getBytes(0, bytes);
        return new UnsafeBuffer(bytes);
    }

    private static UnsafeBuffer cancelFrame(long orderId) {
        ExpandableArrayBuffer buf = new ExpandableArrayBuffer();
        CancelOrderEncoder enc = new CancelOrderEncoder().wrapAndApplyHeader(buf, 0, new MessageHeaderEncoder());
        enc.orderId(orderId);
        int length = MessageHeaderEncoder.ENCODED_LENGTH + enc.encodedLength();
        byte[] bytes = new byte[length];
        buf.getBytes(0, bytes);
        return new UnsafeBuffer(bytes);
    }

    private static List<Integer> templateIdsOf(List<byte[]> frames) {
        List<Integer> ids = new ArrayList<>();
        for (byte[] frame : frames) {
            ids.add((int) new UnsafeBuffer(frame).getShort(2, ByteOrder.LITTLE_ENDIAN));
        }
        return ids;
    }

    private static ExecutionReportDecoder decodeExecutionReport(byte[] frame) {
        DirectBuffer buf = new UnsafeBuffer(frame);
        MessageHeaderDecoder hdr = new MessageHeaderDecoder().wrap(buf, 0);
        assertEquals(ExecutionReportDecoder.TEMPLATE_ID, hdr.templateId());
        return new ExecutionReportDecoder().wrap(buf, hdr.encodedLength(), hdr.blockLength(), hdr.version());
    }

    @Test
    void matchRoutesReportsAndMarketData() {
        MatchingService s = new MatchingService();
        s.onStart(new FakeCluster(1000), null);
        FakeClientSession seller = new FakeClientSession(1);
        FakeClientSession buyer = new FakeClientSession(2);
        FakeClientSession watcher = new FakeClientSession(3);
        for (FakeClientSession sess : List.of(seller, buyer, watcher)) {
            s.onSessionOpen(sess, 1);
        }

        UnsafeBuffer sell = newOrderFrame(10, Side.SELL, 100, 50);
        s.onSessionMessage(seller, 1, sell, 0, sell.capacity(), null);
        UnsafeBuffer buy = newOrderFrame(20, Side.BUY, 100, 50);
        s.onSessionMessage(buyer, 2, buy, 0, buy.capacity(), null);

        int erId = ExecutionReportDecoder.TEMPLATE_ID;
        int teId = javamatch.protocol.codecs.TradeEventDecoder.TEMPLATE_ID;
        int buId = javamatch.protocol.codecs.BookUpdateDecoder.TEMPLATE_ID;

        // Watcher: only broadcast market data (BookUpdate after rest, then
        // TradeEvent + BookUpdate after the match).
        assertEquals(List.of(buId, teId, buId), templateIdsOf(watcher.frames));
        // Engine event order per command: Accepted, Trade, Filled(maker),
        // Filled(taker), BookUpdate. Routing preserves that order per session.
        assertEquals(List.of(erId, buId, teId, erId, buId), templateIdsOf(seller.frames));
        assertEquals(List.of(buId, erId, teId, erId, buId), templateIdsOf(buyer.frames));

        // Decode the buyer's FILLED report (frame index 3) and check payload.
        ExecutionReportDecoder er = decodeExecutionReport(buyer.frames.get(3));
        assertEquals(OrderStatus.FILLED, er.status());
        assertEquals(20, er.clientOrderId());
        assertEquals(50, er.qty());
        assertEquals(2, er.timestamp());
    }

    @Test
    void cancelUnknownOrderRejected() {
        MatchingService s = new MatchingService();
        s.onStart(new FakeCluster(0), null);
        FakeClientSession sess = new FakeClientSession(1);
        s.onSessionOpen(sess, 1);
        UnsafeBuffer cancel = cancelFrame(99);
        s.onSessionMessage(sess, 1, cancel, 0, cancel.capacity(), null);
        assertEquals(1, sess.frames.size());
    }

    @Test
    void closedSessionSkipped() {
        MatchingService s = new MatchingService();
        s.onStart(new FakeCluster(0), null);
        FakeClientSession a = new FakeClientSession(1);
        s.onSessionOpen(a, 1);
        FakeClientSession b = new FakeClientSession(2);
        s.onSessionOpen(b, 1);
        s.onSessionClose(a, 2, CloseReason.CLIENT_ACTION);
        UnsafeBuffer order = newOrderFrame(1, Side.BUY, 10, 1);
        // Session already closed: engine still applies the (replayed)
        // command, but nothing is offered to the closed session.
        s.onSessionMessage(a, 3, order, 0, order.capacity(), null);
        assertEquals(0, a.frames.size());
        assertEquals(1, b.frames.size());
    }

    @Test
    void snapshotChunksRoundTrip() {
        MatchingService s = new MatchingService();
        s.onStart(new FakeCluster(0), null);
        FakeClientSession sess = new FakeClientSession(1);
        s.onSessionOpen(sess, 1);
        UnsafeBuffer order = newOrderFrame(1, Side.BUY, 10, 5);
        s.onSessionMessage(sess, 1, order, 0, order.capacity(), null);

        ByteArrayOutputStream stream = new ByteArrayOutputStream();
        s.writeSnapshot((buffer, offset, length) -> {
            byte[] chunk = new byte[length];
            buffer.getBytes(offset, chunk);
            stream.writeBytes(chunk);
        });

        MatchingService restored = new MatchingService();
        byte[] snapshot = stream.toByteArray();
        restored.restoreSnapshot(new UnsafeBuffer(snapshot), 0, snapshot.length);
        restored.onStart(new FakeCluster(0), null);
        FakeClientSession sess2 = new FakeClientSession(1);
        restored.onSessionOpen(sess2, 1);
        UnsafeBuffer cancel = cancelFrame(1);
        restored.onSessionMessage(sess2, 2, cancel, 0, cancel.capacity(), null);
        ExecutionReportDecoder er = decodeExecutionReport(sess2.frames.get(0));
        assertEquals(OrderStatus.CANCELED, er.status());
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
./gradlew test --tests 'javamatch.service.MatchingServiceTest'
```
Expected: compilation FAILURE (`MatchingService` not defined).

- [ ] **Step 3: Implement `MatchingService`**

`src/main/java/javamatch/service/MatchingService.java`:
```java
package javamatch.service;

import io.aeron.ExclusivePublication;
import io.aeron.Image;
import io.aeron.Publication;
import io.aeron.cluster.codecs.CloseReason;
import io.aeron.cluster.service.ClientSession;
import io.aeron.cluster.service.Cluster;
import io.aeron.cluster.service.ClusteredService;
import io.aeron.logbuffer.Header;
import javamatch.engine.Event;
import javamatch.engine.NewOrderCmd;
import javamatch.engine.OrderBook;
import javamatch.engine.Snapshots;
import javamatch.protocol.codecs.CancelOrderDecoder;
import javamatch.protocol.codecs.MessageHeaderDecoder;
import javamatch.protocol.codecs.NewOrderDecoder;
import javamatch.engine.Side;
import org.agrona.DirectBuffer;
import org.agrona.ExpandableArrayBuffer;
import org.agrona.ExpandableDirectByteBuffer;
import org.agrona.collections.Long2ObjectHashMap;
import org.agrona.collections.LongArrayList;

import java.util.List;

/**
 * The ClusteredService: decodes ingress, drives the matching engine, and
 * routes engine events to cluster egress. Mirror of gomatch's
 * service.MatchingService.
 */
public final class MatchingService implements ClusteredService {
    static final int SBE_HEADER_LENGTH = MessageHeaderDecoder.ENCODED_LENGTH;
    static final int SCHEMA_ID = 901;
    static final int NEW_ORDER_TEMPLATE_ID = NewOrderDecoder.TEMPLATE_ID;
    static final int CANCEL_ORDER_TEMPLATE_ID = CancelOrderDecoder.TEMPLATE_ID;
    static final int SNAPSHOT_CHUNK_SIZE = 1024;

    /** Receives snapshot bytes in chunks; mirrors Go's writeSnapshot(emit). */
    interface SnapshotChunkConsumer {
        void accept(DirectBuffer buffer, int offset, int length);
    }

    private Cluster cluster;
    private OrderBook book = new OrderBook();
    private final Long2ObjectHashMap<ClientSession> sessions = new Long2ObjectHashMap<>();
    private final LongArrayList sessionIds = new LongArrayList(); // deterministic broadcast order (insertion order)
    private final MessageHeaderDecoder headerDecoder = new MessageHeaderDecoder();
    private final NewOrderDecoder newOrderDecoder = new NewOrderDecoder();
    private final CancelOrderDecoder cancelOrderDecoder = new CancelOrderDecoder();
    private final Encoding encoding = new Encoding();
    private final ExpandableDirectByteBuffer egressBuffer = new ExpandableDirectByteBuffer(256);
    private final ExpandableArrayBuffer snapshotBuffer = new ExpandableArrayBuffer();

    public void onStart(Cluster cluster, Image snapshotImage) {
        this.cluster = cluster;
        if (snapshotImage == null) {
            return;
        }
        ExpandableArrayBuffer stream = new ExpandableArrayBuffer();
        int[] streamLength = {0};
        while (true) {
            int polled = snapshotImage.poll((buffer, offset, length, header) -> {
                stream.putBytes(streamLength[0], buffer, offset, length);
                streamLength[0] += length;
            }, 64);
            if (snapshotImage.isEndOfStream() || snapshotImage.isClosed()) {
                break;
            }
            if (polled == 0) {
                cluster.idleStrategy().idle(0);
            }
        }
        restoreSnapshot(stream, 0, streamLength[0]);
    }

    void restoreSnapshot(DirectBuffer buffer, int offset, int length) {
        book = Snapshots.restore(buffer, offset, length);
    }

    public void onSessionOpen(ClientSession session, long timestamp) {
        sessions.put(session.id(), session);
        sessionIds.addLong(session.id());
    }

    public void onSessionClose(ClientSession session, long timestamp, CloseReason closeReason) {
        sessions.remove(session.id());
        for (int i = 0; i < sessionIds.size(); i++) {
            if (sessionIds.getLong(i) == session.id()) {
                sessionIds.remove(i);
                break;
            }
        }
    }

    public void onSessionMessage(
        ClientSession session,
        long timestamp,
        DirectBuffer buffer,
        int offset,
        int length,
        Header header) {
        if (length < SBE_HEADER_LENGTH) {
            return;
        }
        headerDecoder.wrap(buffer, offset);
        int templateId = headerDecoder.templateId();
        if (headerDecoder.schemaId() != SCHEMA_ID) {
            System.err.println("unexpected schemaId=" + headerDecoder.schemaId() + " templateId=" + templateId);
            return;
        }
        List<Event> events;
        switch (templateId) {
            case NEW_ORDER_TEMPLATE_ID -> {
                newOrderDecoder.wrap(buffer, offset + SBE_HEADER_LENGTH,
                    headerDecoder.blockLength(), headerDecoder.version());
                events = book.newLimitOrder(new NewOrderCmd(
                    newOrderDecoder.clientOrderId(),
                    session.id(),
                    Side.fromCode((byte) newOrderDecoder.side().value()),
                    newOrderDecoder.price(),
                    newOrderDecoder.qty()));
            }
            case CANCEL_ORDER_TEMPLATE_ID -> {
                cancelOrderDecoder.wrap(buffer, offset + SBE_HEADER_LENGTH,
                    headerDecoder.blockLength(), headerDecoder.version());
                events = book.cancel(cancelOrderDecoder.orderId(), session.id());
            }
            default -> {
                return;
            }
        }
        route(events, timestamp);
    }

    private void route(List<Event> events, long timestamp) {
        for (int i = 0; i < events.size(); i++) {
            Event ev = events.get(i);
            switch (ev.type) {
                case ACCEPTED, REJECTED, CANCELED, FILLED ->
                    sendTo(ev.owner, encoding.encodeExecutionReport(egressBuffer, ev, timestamp));
                case TRADE ->
                    broadcast(encoding.encodeTrade(egressBuffer, ev, timestamp));
                case BOOK_UPDATE ->
                    broadcast(encoding.encodeBookUpdate(egressBuffer, ev, timestamp));
            }
        }
    }

    private void sendTo(long sessionId, int frameLength) {
        ClientSession session = sessions.get(sessionId);
        if (session != null) {
            offer(session, frameLength);
        }
    }

    private void broadcast(int frameLength) {
        for (int i = 0; i < sessionIds.size(); i++) {
            ClientSession session = sessions.get(sessionIds.getLong(i));
            if (session != null) {
                offer(session, frameLength);
            }
        }
    }

    private void offer(ClientSession session, int frameLength) {
        while (true) {
            long result = session.offer(egressBuffer, 0, frameLength);
            if (result >= 0 || result == ClientSession.MOCKED_OFFER) { // mocked on non-leaders
                return;
            }
            if (result != Publication.BACK_PRESSURED && result != Publication.ADMIN_ACTION) {
                System.err.println("egress offer failed - sessionId=" + session.id() + " result=" + result);
                return;
            }
            cluster.idleStrategy().idle(0);
        }
    }

    public void onTimerEvent(long correlationId, long timestamp) {
    }

    void writeSnapshot(SnapshotChunkConsumer consumer) {
        int length = Snapshots.write(book, snapshotBuffer);
        int offset = 0;
        while (offset < length) {
            int n = Math.min(SNAPSHOT_CHUNK_SIZE, length - offset);
            consumer.accept(snapshotBuffer, offset, n);
            offset += n;
        }
    }

    public void onTakeSnapshot(ExclusivePublication snapshotPublication) {
        writeSnapshot((buffer, offset, length) -> {
            while (true) {
                long result = snapshotPublication.offer(buffer, offset, length);
                if (result >= 0) {
                    return;
                }
                if (result != Publication.BACK_PRESSURED && result != Publication.ADMIN_ACTION) {
                    throw new IllegalStateException("snapshot offer failed: " + result);
                }
                cluster.idleStrategy().idle(0);
            }
        });
    }

    public void onRoleChange(Cluster.Role newRole) {
        System.out.println("role change: " + newRole);
    }

    public void onTerminate(Cluster cluster) {
    }
}
```

Note: `ClientSession.MOCKED_OFFER` may itself be non-negative; the explicit check is defensive parity with Go. `Side.fromCode((byte) newOrderDecoder.side().value())` — the generated codec enum has `value()`; if the generated method differs (e.g. returns `short`), adapt the cast, not the engine.

- [ ] **Step 4: Run test to verify it passes**

```bash
./gradlew test --tests 'javamatch.service.MatchingServiceTest'
```
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "MatchingService: ingress dispatch, egress routing, chunked snapshots"
```

---

### Task 8: Client

**Files:**
- Create: `src/main/java/javamatch/client/MatchClient.java`, `src/main/java/javamatch/client/EgressAdapter.java`
- Test: `src/test/java/javamatch/client/EgressAdapterTest.java`

**Interfaces:**
- Produces (used by LoadGen, Task 9, and systests, Task 10):
  - `MatchClient.Listener` with `void onExecutionReport(ExecReport e)`, `void onTrade(Trade t)`, `void onBookUpdate(Book b)`
  - records (nested in `MatchClient`): `ExecReport(long orderId, long clientOrderId, OrderStatus status, RejectReason reason, Side side, long price, long qty, long remainingQty, long timestamp)` (codec enums), `Trade(long price, long qty, long makerOrderId, long takerOrderId, long timestamp)`, `Book(Side side, long price, long aggregateQty, long timestamp)`
  - `static MatchClient connect(String aeronDir, String ingressEndpoints, Listener listener)`
  - `static MatchClient connectWithEgress(String aeronDir, String ingressEndpoints, String egressEndpoint, Listener listener)`
  - `void submitOrder(long clientOrderId, Side side, long price, long qty)`, `void cancelOrder(long orderId)`, `int poll()`, `void close()`

- [ ] **Step 1: Write the failing test** (port of `client/egress_test.go`)

`src/test/java/javamatch/client/EgressAdapterTest.java`:
```java
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
./gradlew test --tests 'javamatch.client.EgressAdapterTest'
```
Expected: compilation FAILURE.

- [ ] **Step 3: Implement `EgressAdapter` and `MatchClient`**

`src/main/java/javamatch/client/EgressAdapter.java`:
```java
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
```

`src/main/java/javamatch/client/MatchClient.java`:
```java
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
```

- [ ] **Step 4: Run test to verify it passes**

```bash
./gradlew test --tests 'javamatch.client.EgressAdapterTest'
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "Typed client over AeronCluster with egress decode adapter"
```

---

### Task 9: Apps — EngineMain and LoadGen

**Files:**
- Create: `src/main/java/javamatch/app/EngineMain.java`, `src/main/java/javamatch/app/LoadGen.java`
- Modify: `build.gradle` (add `runEngine`, `runLoadGen` JavaExec tasks)

**Interfaces:**
- Consumes: `MatchingService` (Task 7), `MatchClient` (Task 8).
- Produces: `javamatch.app.EngineMain#main` honoring `AERON_DIR`/`CLUSTER_DIR` env vars; `javamatch.app.LoadGen#main` with flags `-orders N -rate N -aeron-dir D -ingress E -egress E` and gomatch-compatible output lines.

- [ ] **Step 1: Implement `EngineMain`** (no unit test — exercised by systests in Task 10)

`src/main/java/javamatch/app/EngineMain.java`:
```java
package javamatch.app;

import io.aeron.cluster.service.ClusteredServiceContainer;
import javamatch.service.MatchingService;
import org.agrona.concurrent.ShutdownSignalBarrier;

import java.io.File;

/** Runs the matching service as a ClusteredServiceContainer against an external cluster node. */
public final class EngineMain {
    private EngineMain() {}

    public static void main(String[] args) {
        ClusteredServiceContainer.Context ctx = new ClusteredServiceContainer.Context()
            .clusteredService(new MatchingService());
        String aeronDir = System.getenv("AERON_DIR");
        if (aeronDir == null && new File("/dev/shm").exists()) {
            aeronDir = "/dev/shm/aeron-" + System.getProperty("user.name");
        }
        if (aeronDir != null) {
            ctx.aeronDirectoryName(aeronDir);
        }
        String clusterDir = System.getenv("CLUSTER_DIR");
        if (clusterDir != null) {
            ctx.clusterDir(new File(clusterDir));
        }
        try (ClusteredServiceContainer container = ClusteredServiceContainer.launch(ctx)) {
            new ShutdownSignalBarrier().await();
        }
    }
}
```

- [ ] **Step 2: Implement `LoadGen`** (port of `cmd/loadgen/main.go`; single-threaded, so no locking — poll and submit run on the same thread)

`src/main/java/javamatch/app/LoadGen.java`:
```java
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
```

- [ ] **Step 3: Add run tasks to `build.gradle`**

```groovy
tasks.register('runEngine', JavaExec) {
    classpath = sourceSets.main.runtimeClasspath
    mainClass = 'javamatch.app.EngineMain'
    jvmArgs '--add-opens=java.base/sun.nio.ch=ALL-UNNAMED',
            '--add-exports=java.base/jdk.internal.misc=ALL-UNNAMED'
}

tasks.register('runLoadGen', JavaExec) {
    classpath = sourceSets.main.runtimeClasspath
    mainClass = 'javamatch.app.LoadGen'
    jvmArgs '--add-opens=java.base/sun.nio.ch=ALL-UNNAMED',
            '--add-exports=java.base/jdk.internal.misc=ALL-UNNAMED'
    if (project.hasProperty('appArgs')) {
        args project.property('appArgs').split(' ')
    }
}
```

- [ ] **Step 4: Verify compilation**

```bash
./gradlew compileJava
```
Expected: BUILD SUCCESSFUL.

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "EngineMain and LoadGen apps"
```

---

### Task 10: Systest harness + match/market-data test

**Files:**
- Create: `src/systest/java/javamatch/systest/ClusterHarness.java`, `src/systest/java/javamatch/systest/Recorder.java`, `src/systest/java/javamatch/systest/MatchSystemTest.java`

**Interfaces:**
- Consumes: `MatchingService`, `MatchClient`.
- Produces: `ClusterHarness` used by Task 11: `static ClusterHarness launch()`, `String aeronDir()`, `String ingressEndpoint()`, `void snapshot()`, `void shutdown()` (stop, keep dirs), `ClusterHarness restart()`, `void close()` (stop + delete dirs).

- [ ] **Step 1: Write the harness and the test**

`src/systest/java/javamatch/systest/ClusterHarness.java`:
```java
package javamatch.systest;

import io.aeron.archive.Archive;
import io.aeron.archive.ArchiveThreadingMode;
import io.aeron.cluster.ClusterTool;
import io.aeron.cluster.ClusteredMediaDriver;
import io.aeron.cluster.ConsensusModule;
import io.aeron.cluster.service.ClusteredServiceContainer;
import io.aeron.driver.MediaDriver;
import io.aeron.driver.ThreadingMode;
import javamatch.service.MatchingService;
import org.agrona.CloseHelper;
import org.agrona.IoUtil;

import java.io.File;
import java.nio.file.Files;
import java.util.UUID;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;

/**
 * Runs a single-node Aeron Cluster (media driver + archive + consensus
 * module) plus the matching service container in-process. Mirror of
 * gomatch's systest harness, minus the subprocess management: we are
 * already a JVM.
 *
 * Each instance gets its own port block, derived from the pid and an
 * instance counter, so distinct test invocations and distinct harnesses
 * within one process never share ports. Restarts of the same harness
 * deliberately reuse its block and directories.
 */
final class ClusterHarness implements AutoCloseable {
    private static final AtomicInteger INSTANCES = new AtomicInteger();

    private final int basePort;
    private final String aeronDir;
    private final File baseDir;
    private final File clusterDir;
    private final File archiveDir;
    private ClusteredMediaDriver driver;
    private ClusteredServiceContainer container;

    private static int nextBasePort() {
        int instance = INSTANCES.getAndIncrement();
        return 20000 + (int) (ProcessHandle.current().pid() % 100) * 120 + (instance % 6) * 20;
    }

    static ClusterHarness launch() throws Exception {
        File baseDir = Files.createTempDirectory("javamatch-systest").toFile();
        String aeronDir = "/dev/shm/aeron-" + System.getProperty("user.name")
            + "/" + UUID.randomUUID() + "/driver";
        ClusterHarness harness = new ClusterHarness(nextBasePort(), aeronDir, baseDir);
        harness.start();
        return harness;
    }

    private ClusterHarness(int basePort, String aeronDir, File baseDir) {
        this.basePort = basePort;
        this.aeronDir = aeronDir;
        this.baseDir = baseDir;
        this.clusterDir = new File(baseDir, "cluster");
        this.archiveDir = new File(baseDir, "archive");
    }

    private void start() {
        String members = String.format(
            "0,localhost:%d,localhost:%d,localhost:%d,localhost:%d,localhost:%d",
            basePort, basePort + 1, basePort + 2, basePort + 3, basePort + 10);
        driver = ClusteredMediaDriver.launch(
            new MediaDriver.Context()
                .aeronDirectoryName(aeronDir)
                .threadingMode(ThreadingMode.SHARED)
                .dirDeleteOnStart(true)
                .dirDeleteOnShutdown(true)
                .clientLivenessTimeoutNs(TimeUnit.MINUTES.toNanos(1))
                .publicationUnblockTimeoutNs(TimeUnit.MINUTES.toNanos(15)),
            new Archive.Context()
                .aeronDirectoryName(aeronDir)
                .archiveDir(archiveDir)
                .controlChannel("aeron:udp?endpoint=localhost:" + (basePort + 10))
                .replicationChannel("aeron:udp?endpoint=localhost:0")
                .threadingMode(ArchiveThreadingMode.SHARED),
            new ConsensusModule.Context()
                .clusterDir(clusterDir)
                .clusterMembers(members)
                .ingressChannel("aeron:udp?term-length=64k")
                .replicationChannel("aeron:udp?endpoint=localhost:0")
                .serviceCount(1));
        container = ClusteredServiceContainer.launch(
            new ClusteredServiceContainer.Context()
                .aeronDirectoryName(aeronDir)
                .clusterDir(clusterDir)
                .clusteredService(new MatchingService()));
    }

    String aeronDir() {
        return aeronDir;
    }

    String ingressEndpoint() {
        return "localhost:" + basePort;
    }

    void snapshot() {
        if (!ClusterTool.snapshot(clusterDir, System.out)) {
            throw new IllegalStateException("cluster snapshot failed");
        }
    }

    /** Stops the node gracefully, keeping cluster and archive dirs for a restart. */
    void shutdown() {
        CloseHelper.close(container);
        container = null;
        CloseHelper.close(driver);
        driver = null;
    }

    /** Starts the node again over the kept cluster/archive dirs. */
    ClusterHarness restart() {
        start();
        return this;
    }

    @Override
    public void close() {
        shutdown();
        IoUtil.delete(baseDir, true);
        IoUtil.delete(new File(aeronDir).getParentFile(), true);
    }
}
```

`src/systest/java/javamatch/systest/Recorder.java`:
```java
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
```

`src/systest/java/javamatch/systest/MatchSystemTest.java`:
```java
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
```

- [ ] **Step 2: Run the systest**

```bash
./gradlew systest --tests 'javamatch.systest.MatchSystemTest'
```
Expected: PASS in well under a minute. Debug aids: cluster errors via `ClusterTool.describe`, `/dev/shm` for stale aeron dirs.

- [ ] **Step 3: Commit**

```bash
git add -A && git commit -m "systest: in-process single-node cluster harness; match/market-data test"
```

---

### Task 11: Restart-from-snapshot systest

**Files:**
- Create: `src/systest/java/javamatch/systest/RestartSystemTest.java`

**Interfaces:**
- Consumes: `ClusterHarness.snapshot()/shutdown()/restart()` from Task 10.

- [ ] **Step 1: Write the test** (port of `systest/restart_test.go` — a resting order must survive snapshot + full node restart and still match)

`src/systest/java/javamatch/systest/RestartSystemTest.java`:
```java
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
```

- [ ] **Step 2: Run it**

```bash
./gradlew systest
```
Expected: both systest classes PASS.

- [ ] **Step 3: Commit**

```bash
git add -A && git commit -m "systest: resting order survives snapshot + node restart"
```

---

### Task 12: Benchmark tooling + README + full verification

**Files:**
- Create: `scripts/start-cluster.sh`, `README.md`

- [ ] **Step 1: Write the cluster launcher script** (same JVM flags as gomatch's systest harness, fixed port block 20000 so both loadgens' defaults work)

`scripts/start-cluster.sh`:
```bash
#!/usr/bin/env bash
# Single-node Aeron Cluster (media driver + archive + consensus module) for
# benchmarking, on the fixed 20000 port block. The engine (Java runEngine or
# gomatch's cmd/engine) and loadgen attach to it. Same JVM flags as the
# gomatch systest harness so Go and Java benchmarks share identical infra.
set -euo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
JAR="${AERON_ALL_JAR:-$DIR/aeron-all-1.52.0.jar}"
if [ ! -f "$JAR" ]; then
  curl -fL -o "$JAR" https://repo1.maven.org/maven2/io/aeron/aeron-all/1.52.0/aeron-all-1.52.0.jar
fi
BASE="${BASE_DIR:-/tmp/javamatch-bench-cluster}"
rm -rf "$BASE"
mkdir -p "$BASE/cluster" "$BASE/archive"
AERON_DIR="${AERON_DIR:-/dev/shm/aeron-$USER-bench}"
echo "aeron dir:   $AERON_DIR"
echo "cluster dir: $BASE/cluster"
exec java \
  --add-opens=java.base/sun.nio.ch=ALL-UNNAMED \
  --add-exports=java.base/jdk.internal.misc=ALL-UNNAMED \
  -XX:+UnlockDiagnosticVMOptions \
  -XX:GuaranteedSafepointInterval=300000 \
  -Daeron.dir="$AERON_DIR" \
  -Daeron.dir.delete.on.start=true \
  -Daeron.dir.delete.on.shutdown=true \
  -Daeron.threading.mode=SHARED \
  -Daeron.client.liveness.timeout=60000000000 \
  -Daeron.publication.unblock.timeout=900000000000 \
  -Daeron.archive.dir="$BASE/archive" \
  -Daeron.archive.control.channel="aeron:udp?endpoint=localhost:20010" \
  -Daeron.archive.replication.channel="aeron:udp?endpoint=localhost:0" \
  -Daeron.archive.threading.mode=SHARED \
  -Daeron.cluster.dir="$BASE/cluster" \
  -Daeron.cluster.members="0,localhost:20000,localhost:20001,localhost:20002,localhost:20003,localhost:20010" \
  -Daeron.cluster.member.id=0 \
  -Daeron.cluster.ingress.channel="aeron:udp?term-length=64k" \
  -Daeron.cluster.replication.channel="aeron:udp?endpoint=localhost:0" \
  -Daeron.cluster.service.count=1 \
  -cp "$JAR" io.aeron.cluster.ClusteredMediaDriver
```
```bash
chmod +x scripts/start-cluster.sh
```

- [ ] **Step 2: Write `README.md`**

```markdown
# javamatch

Single-instrument limit order book matching engine running as a Java
ClusteredService on Aeron Cluster 1.52. Java port of
[gomatch](../gomatch) — same SBE schema (901), same snapshot format, same
event semantics; either project's loadgen can benchmark either engine.

## Layout

- `javamatch.engine` — pure deterministic matching core
- `javamatch.protocol.codecs` — SBE codecs generated at build from
  `src/main/resources/gomatch-schema.xml`
- `javamatch.service` — ClusteredService glue
- `javamatch.client` — typed client; `javamatch.app.LoadGen` — benchmark tool
- `src/systest/java` — integration tests against an in-process single-node
  cluster

## Test

    ./gradlew test      # unit
    ./gradlew systest   # integration (in-process ClusteredMediaDriver)

## Benchmark

    ./scripts/start-cluster.sh                     # terminal 1: cluster node
    AERON_DIR=/dev/shm/aeron-$USER-bench \
      CLUSTER_DIR=/tmp/javamatch-bench-cluster/cluster \
      ./gradlew runEngine                          # terminal 2: engine
    ./gradlew runLoadGen -PappArgs="-orders 100000 -aeron-dir /dev/shm/aeron-$USER-bench"

Open-loop by default (throughput numbers; latency reflects burst queueing).
Pass `-rate N` in `appArgs` to pace submission for honest per-order latency
percentiles (coordinated-omission corrected, measured from scheduled send
time).

To benchmark the Go engine on the same cluster, run gomatch's
`cmd/engine` with the same `AERON_DIR`/`CLUSTER_DIR` instead of `runEngine`.

## Known limitations (v1)

Inherited from gomatch: if only the service container restarts while the
consensus module keeps running, sessions restored from a cluster snapshot
are not replayed through `onSessionOpen`, so those clients stop receiving
egress until they reconnect. A full node restart does not have this problem.

Design: `docs/superpowers/specs/2026-07-06-javamatch-port-design.md`.
```

- [ ] **Step 3: Full verification**

```bash
./gradlew test systest
```
Expected: all unit tests and both systests PASS.

- [ ] **Step 4: Commit**

```bash
git add -A && git commit -m "Benchmark cluster script and README"
```

---

### Task 13: Benchmark comparison (Java vs Go)

**Files:**
- Create: `docs/BENCHMARKS.md` (results)

No new product code — this task runs the comparison and records it. All runs on this machine, same cluster script, same loadgen parameters.

- [ ] **Step 1: Java engine benchmark**

Terminal-less sequencing (backgrounded processes from the repo root):
```bash
cd /home/claude/ultima/javamatch
./scripts/start-cluster.sh > /tmp/bench-cluster.log 2>&1 &   # wait ~3s for startup
AERON_DIR=/dev/shm/aeron-$USER-bench CLUSTER_DIR=/tmp/javamatch-bench-cluster/cluster \
  ./gradlew -q runEngine > /tmp/bench-engine.log 2>&1 &      # wait for it to join
./gradlew -q runLoadGen -PappArgs="-orders 100000 -aeron-dir /dev/shm/aeron-$USER-bench"
./gradlew -q runLoadGen -PappArgs="-orders 100000 -rate 20000 -aeron-dir /dev/shm/aeron-$USER-bench"
```
Record both output blocks. Kill engine + cluster afterwards.

- [ ] **Step 2: Go engine benchmark** (same cluster infra, Go service container + Go loadgen)

```bash
./scripts/start-cluster.sh > /tmp/bench-cluster-go.log 2>&1 &
cd /home/claude/ultima/gomatch
AERON_DIR=/dev/shm/aeron-$USER-bench CLUSTER_DIR=/tmp/javamatch-bench-cluster/cluster \
  go run ./cmd/engine > /tmp/bench-engine-go.log 2>&1 &
go run ./cmd/loadgen -orders 100000 -aeron-dir /dev/shm/aeron-$USER-bench
go run ./cmd/loadgen -orders 100000 -rate 20000 -aeron-dir /dev/shm/aeron-$USER-bench
```
Record both output blocks. Kill engine + cluster afterwards.

- [ ] **Step 3: Wire-compat cross-check** — Go loadgen against the Java engine (start cluster + `runEngine` as in Step 1, then run the Go loadgen). A few thousand orders suffice; confirm acks arrive.

- [ ] **Step 4: Write `docs/BENCHMARKS.md`** with a table: engine (Go/Java) × mode (open-loop / paced 20k/s) × {throughput, p50, p99, p99.9}, plus machine/JVM/Go versions and the cross-check result. Note the first Java run includes JIT warmup; optionally repeat the loadgen once and report the second run.

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "Benchmark results: Java vs Go engine on identical cluster infra"
```

---

## Self-review notes

- Spec coverage: engine (Tasks 2–5), protocol (Task 1), service (Tasks 6–7), client (Task 8), apps (Task 9), systest (Tasks 10–11), README/benchmark plan (Tasks 12–13), wire/snapshot compat (Task 1 schema copy, Task 5 Step 5 cross-check, Task 13 Step 3). Known-limitations documented in README (Task 12).
- API-uncertainty callouts (resolve at compile time, never by changing engine semantics): generated enum accessor shape in Task 7 (`side().value()`), `Cluster` interface surface in the FakeCluster note, `LongArrayList.remove(int)` overload (if ambiguous with `remove(Object)`, use `sessionIds.removeAt(i)` — Agrona has `fastUnorderedRemove`/`remove(int)`; pick the positional one).
- `Encoding` reuses one egress buffer across broadcast offers — safe because `ClientSession.offer` copies synchronously.

