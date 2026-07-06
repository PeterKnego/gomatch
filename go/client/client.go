package client

import (
	"bytes"
	"fmt"
	"io"
	"time"

	"github.com/lirm/aeron-go/aeron"
	"github.com/lirm/aeron-go/aeron/atomic"
	"github.com/lirm/aeron-go/aeron/logbuffer"
	aeroncluster "github.com/lirm/aeron-go/cluster/client"

	"gomatch/protocol/codecs"
)

type Client struct {
	ac      *aeroncluster.AeronCluster
	adapter *egressAdapter
	opts    *aeroncluster.Options
}

// clusterEgress adapts egressAdapter to the cluster client's EgressListener.
type clusterEgress struct{ a *egressAdapter }

func (c *clusterEgress) OnConnect(*aeroncluster.AeronCluster)                 {}
func (c *clusterEgress) OnDisconnect(*aeroncluster.AeronCluster, string)      {}
func (c *clusterEgress) OnNewLeader(*aeroncluster.AeronCluster, int64, int32) {}
func (c *clusterEgress) OnError(_ *aeroncluster.AeronCluster, detail string) {
	logger.Errorf("cluster error: %s", detail)
}
func (c *clusterEgress) OnMessage(ac *aeroncluster.AeronCluster, timestamp int64,
	buffer *atomic.Buffer, offset int32, length int32, header *logbuffer.Header) {
	c.a.onMessage(ac, timestamp, buffer, offset, length, header)
}

// Connect connects to the cluster. ingressEndpoints example: "0=localhost:20000".
func Connect(aeronDir, ingressEndpoints string, l Listener) (*Client, error) {
	return ConnectWithEgress(aeronDir, ingressEndpoints, "localhost:0", l)
}

// ConnectWithEgress is Connect with an explicit egress endpoint: the
// address:port on this host that cluster nodes send responses to. It must be
// reachable from every node — the default localhost:0 only works when the
// leader runs on the same host as the client.
func ConnectWithEgress(aeronDir, ingressEndpoints, egressEndpoint string, l Listener) (*Client, error) {
	adapter := newEgressAdapter(l)
	opts := aeroncluster.NewOptions()
	opts.IngressChannel = "aeron:udp?alias=gomatch-ingress"
	opts.IngressEndpoints = ingressEndpoints
	opts.EgressChannel = fmt.Sprintf("aeron:udp?alias=gomatch-egress|endpoint=%s", egressEndpoint)
	ac, err := aeroncluster.NewAeronCluster(
		aeron.NewContext().AeronDir(aeronDir), opts, &clusterEgress{a: adapter})
	if err != nil {
		return nil, err
	}
	c := &Client{ac: ac, adapter: adapter, opts: opts}
	deadline := time.Now().Add(30 * time.Second)
	for !ac.IsConnected() {
		if time.Now().After(deadline) {
			ac.Close()
			return nil, fmt.Errorf("timed out connecting to cluster")
		}
		opts.IdleStrategy.Idle(ac.Poll())
	}
	return c, nil
}

func (c *Client) offer(msg interface {
	Encode(*codecs.SbeGoMarshaller, io.Writer, bool) error
	SbeBlockLength() uint16
	SbeTemplateId() uint16
	SbeSchemaId() uint16
	SbeSchemaVersion() uint16
}) error {
	m := codecs.NewSbeGoMarshaller()
	var buf bytes.Buffer
	hdr := codecs.MessageHeader{BlockLength: msg.SbeBlockLength(), TemplateId: msg.SbeTemplateId(),
		SchemaId: msg.SbeSchemaId(), Version: msg.SbeSchemaVersion()}
	if err := hdr.Encode(m, &buf); err != nil {
		return err
	}
	if err := msg.Encode(m, &buf, true); err != nil {
		return err
	}
	payload := atomic.MakeBuffer(buf.Bytes())
	deadline := time.Now().Add(10 * time.Second)
	for c.ac.Offer(payload, 0, payload.Capacity()) < 0 {
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out offering to cluster")
		}
		c.opts.IdleStrategy.Idle(c.ac.Poll())
	}
	return nil
}

func (c *Client) SubmitOrder(clientOrderId int64, side codecs.SideEnum, price, qty int64) error {
	return c.offer(&codecs.NewOrder{ClientOrderId: clientOrderId, Side: side, Price: price, Qty: qty})
}

func (c *Client) CancelOrder(orderId int64) error {
	return c.offer(&codecs.CancelOrder{OrderId: orderId})
}

func (c *Client) Poll() int { return c.ac.Poll() }

func (c *Client) Close() { c.ac.Close() }
