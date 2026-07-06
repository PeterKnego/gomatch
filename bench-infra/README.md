# bench-infra — 3-node cloud benchmark rig for gomatch vs javamatch

Provisions a 3-node Aeron Cluster fleet (AWS default; Hetzner/GCP via
`cloud=`), tunes the hosts, starts the Java `ClusteredMediaDriver` on every
node with either the **Go engine** (gomatch, this repo) or the **Java
engine** ([javamatch](../../javamatch), expected as a sibling checkout), and
runs a paced rate-ladder sweep from node0. The Go loadgen drives both
engines, so the client/measurement side is constant and the engine is the
only variable.

Account-independent by design: credentials live only in a gitignored `.env`,
personal values (SSH key, source IP) only in a gitignored `terraform.tfvars`.

## Control-machine setup

Needed on the machine that runs the rig (not the provisioned hosts):
`terraform` >= 1.6, `ansible-core` >= 2.16 (+ collections `ansible.posix`,
`community.general`), `jq`, `rsync`, an SSH keypair, and — for
`ENGINE=java` — a JDK 21 to build the javamatch jar via its Gradle wrapper.

## Credentials

    cp .env.example .env       # gitignored; fill in AWS_ACCESS_KEY_ID/SECRET
                               # (or rely on AWS_PROFILE / the provider chain)

## Quickstart

    cp example.aws.tfvars terraform.tfvars   # edit ssh key + allow_ssh_cidr
    make init
    make up               # provision 3 nodes + write ansible inventory (~2 min)
    make bench            # Go engine: configure, start cluster, sweep (~10 min cold)
    make bench ENGINE=java# Java engine: same fleet, fresh cluster state
    make destroy          # tear down — nothing auto-reaps!

`make bench-both` runs the Go then the Java sweep back to back. Results land
in `bench-out/<timestamp>/results-<engine>.txt` (one loadgen result block per
ladder rung + one open-loop ceiling run). `make status` lists hosts as a cost
guard; `make ssh-node0` opens a shell on the client node.

## Sweep parameters

Rate ladder and open-loop order count: `ansible/roles/gomatch/defaults/main.yml`
(`gomatch_rate_ladder`, `gomatch_openloop_orders`). Each rung submits
`rate × 10` orders (~10 s). Latency is measured from each order's *scheduled*
send time (coordinated-omission corrected) to its ExecutionReport ack.

## Topology

- 3 × cloud hosts, single AZ + cluster placement group (AWS), private-IP mesh.
- Every node: Java `ClusteredMediaDriver` (media driver + archive + consensus
  module) + one engine service container (Go or Java).
- node0 additionally runs the loadgen; its egress endpoint binds the private
  IP so all nodes can reach it.
- Port block 20000..20010 on every host (distinct machines, same block).
