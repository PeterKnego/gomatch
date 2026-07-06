// loadgen submits a deterministic mix of crossing and resting limit orders
// and reports throughput and submit-to-ack latency percentiles.
package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"sync"
	"time"

	"gomatch/client"
	"gomatch/protocol/codecs"
)

type collector struct {
	mu        sync.Mutex
	submitted map[int64]time.Time
	latencies []time.Duration
	acked     int
}

func (c *collector) OnExecutionReport(e client.ExecReport) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if t0, ok := c.submitted[e.ClientOrderId]; ok {
		c.latencies = append(c.latencies, time.Since(t0))
		delete(c.submitted, e.ClientOrderId)
		c.acked++
	}
}
func (c *collector) OnTrade(client.Trade)     {}
func (c *collector) OnBookUpdate(client.Book) {}

func main() {
	orders := flag.Int("orders", 100000, "number of orders to submit")
	rate := flag.Int("rate", 0, "target submission rate in orders/sec (0 = open-loop)")
	aeronDir := flag.String("aeron-dir", fmt.Sprintf("/dev/shm/aeron-%s", os.Getenv("USER")), "aeron media driver directory")
	ingress := flag.String("ingress", "0=localhost:20000", "cluster ingress endpoints")
	egress := flag.String("egress", "localhost:0", "egress endpoint on this host, reachable from all cluster nodes")
	flag.Parse()

	col := &collector{submitted: make(map[int64]time.Time)}
	c, err := client.ConnectWithEgress(*aeronDir, *ingress, *egress, col)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	var interval time.Duration
	if *rate > 0 {
		interval = time.Second / time.Duration(*rate)
	}

	rng := rand.New(rand.NewSource(1))
	start := time.Now()
	for i := 0; i < *orders; i++ {
		side := codecs.Side.BUY
		if i%2 == 1 {
			side = codecs.Side.SELL
		}
		price := int64(100 + rng.Intn(5) - 2) // 98..102 straddling mid: ~half cross
		id := int64(i + 1)
		sendTime := time.Now()
		if interval > 0 {
			// Latency is measured from the scheduled time, not the actual
			// send, so generator stalls count against the reported numbers
			// (coordinated-omission correction).
			sendTime = start.Add(time.Duration(i) * interval)
			for time.Now().Before(sendTime) {
				c.Poll()
			}
		}
		col.mu.Lock()
		col.submitted[id] = sendTime
		col.mu.Unlock()
		if err := c.SubmitOrder(id, side, price, int64(rng.Intn(10)+1)); err != nil {
			panic(err)
		}
		c.Poll()
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		col.mu.Lock()
		done := col.acked >= *orders
		col.mu.Unlock()
		if done || time.Now().After(deadline) {
			break
		}
		c.Poll()
	}
	elapsed := time.Since(start)

	col.mu.Lock()
	defer col.mu.Unlock()
	sort.Slice(col.latencies, func(i, j int) bool { return col.latencies[i] < col.latencies[j] })
	pct := func(p float64) time.Duration {
		if len(col.latencies) == 0 {
			return 0
		}
		idx := int(p * float64(len(col.latencies)-1))
		return col.latencies[idx]
	}
	target := ""
	if *rate > 0 {
		target = fmt.Sprintf(" target=%d/s", *rate)
	}
	fmt.Printf("orders=%d acked=%d%s elapsed=%v rate=%.0f orders/sec\n",
		*orders, col.acked, target, elapsed, float64(col.acked)/elapsed.Seconds())
	fmt.Printf("ack latency p50=%v p99=%v p99.9=%v\n", pct(0.50), pct(0.99), pct(0.999))
}
