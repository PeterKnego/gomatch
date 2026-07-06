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
