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
