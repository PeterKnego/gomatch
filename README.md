# gomatch — aeron-go vs Java on Aeron Cluster

Exploratory project comparing **aeron-go** against the reference **Java**
Aeron Cluster stack, using the same application on both: a single-instrument
limit order book matching engine running as a ClusteredService.

The two implementations are structural mirrors and fully **wire- and
snapshot-compatible** (same SBE schema 901, byte-identical snapshot format),
so either side's loadgen can drive either engine.

## Layout

- [`go/`](go/) — Go implementation on the `PeterKnego/aeron-go` fork
  (engine, SBE codecs, ClusteredService, client, loadgen, systests)
- [`java/`](java/) — Java 21 implementation on `io.aeron:aeron-all` 1.52
  (same layout; sbe-tool-generated codecs, Gradle)
- [`bench-infra/`](bench-infra/) — terraform + ansible rig that provisions a
  3-node cloud cluster and runs a paced rate-ladder sweep against either
  engine (`make bench ENGINE=go|java`)

## Results so far

See [`java/docs/BENCHMARKS.md`](java/docs/BENCHMARKS.md). Headline from the
3-node AWS sweep (c7i.4xlarge, Go loadgen driving both engines): the Java
service container holds a ~3× lower median (≈220µs vs ≈700µs ack latency),
its p99 knee arrives at ~100k orders/s vs ~60k/s for Go, and its open-loop
ceiling is ~28% higher (354k vs 277k orders/s). On a shared single-node VM
the engines are within ~2% — the divergence only shows on real hardware.

## Cloud benchmark

`bench-infra/` is a self-contained terraform + ansible rig: it provisions a
3-node fleet (AWS by default — single AZ, cluster placement group; Hetzner
and GCP via `cloud=`), OS-tunes the hosts, starts the Java
`ClusteredMediaDriver` on every node with either engine's service container
(`ENGINE=go|java`), and runs a paced rate-ladder sweep (1k→150k orders/s,
coordinated-omission corrected) plus an open-loop ceiling run from node0.
The Go loadgen drives both engines, so the engine is the only variable.
Credentials and personal values live only in gitignored files
(`.env`, `terraform.tfvars`) — the committed rig is account-independent.

    cd bench-infra
    cp example.aws.tfvars terraform.tfvars   # edit ssh key + allow_ssh_cidr
    cp .env.example .env                     # fill in AWS credentials
    make init && make up                     # provision 3 nodes
    make bench-both                          # Go sweep, then Java sweep
    make destroy                             # nothing auto-reaps!

Results land in `bench-infra/bench-out/<timestamp>/results-<engine>.txt`.

## Test

    cd go   && go test ./engine/ ./protocol/ ./service/ ./client/   # Go unit
    cd java && ./gradlew test systest                               # Java unit + integration

Go systests need the aeron-all jar — see [`go/README.md`](go/README.md).
