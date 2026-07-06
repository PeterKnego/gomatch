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
