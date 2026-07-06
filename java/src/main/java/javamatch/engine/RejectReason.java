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
