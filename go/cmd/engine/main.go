package main

import (
	"fmt"
	"os"

	"github.com/lirm/aeron-go/aeron"
	"github.com/lirm/aeron-go/cluster"

	"gomatch/service"
)

func main() {
	ctx := aeron.NewContext()
	if aeronDir := os.Getenv("AERON_DIR"); aeronDir != "" {
		ctx.AeronDir(aeronDir)
	} else if _, err := os.Stat("/dev/shm"); err == nil {
		ctx.AeronDir(fmt.Sprintf("/dev/shm/aeron-%s", aeron.UserName))
	}
	opts := cluster.NewOptions()
	if clusterDir := os.Getenv("CLUSTER_DIR"); clusterDir != "" {
		opts.ClusterDir = clusterDir
	}
	agent, err := cluster.NewClusteredServiceAgent(ctx, opts, service.NewMatchingService())
	if err != nil {
		panic(err)
	}
	if err := agent.StartAndRun(); err != nil {
		panic(err)
	}
}
