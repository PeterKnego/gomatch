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
