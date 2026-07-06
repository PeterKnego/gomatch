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
