package javamatch.engine;

public record NewOrderCmd(long clientOrderId, long owner, Side side, long price, long qty) {
}
