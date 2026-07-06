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
