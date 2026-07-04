package service

import (
	"bytes"
	"fmt"
	"io"

	"github.com/lirm/aeron-go/aeron"
	"github.com/lirm/aeron-go/aeron/atomic"
	"github.com/lirm/aeron-go/aeron/logbuffer"
	"github.com/lirm/aeron-go/aeron/logging"
	"github.com/lirm/aeron-go/cluster"
	ccodecs "github.com/lirm/aeron-go/cluster/codecs"

	"gomatch/engine"
	"gomatch/protocol/codecs"
)

var logger = logging.MustGetLogger("gomatch")

const (
	sbeHeaderLength       = 8
	schemaId              = 901
	newOrderTemplateId    = 1
	cancelOrderTemplateId = 2
	snapshotChunkSize     = 1024
)

// MatchingService is the ClusteredService: it decodes ingress, drives the
// matching engine, and routes engine events to cluster egress.
type MatchingService struct {
	cluster    cluster.Cluster
	book       *engine.OrderBook
	sessions   map[int64]cluster.ClientSession
	sessionIds []int64 // deterministic broadcast order (insertion order)
	marshaller *codecs.SbeGoMarshaller
}

func NewMatchingService() *MatchingService {
	return &MatchingService{
		book:       engine.NewOrderBook(),
		sessions:   map[int64]cluster.ClientSession{},
		marshaller: codecs.NewSbeGoMarshaller(),
	}
}

func (s *MatchingService) OnStart(c cluster.Cluster, image aeron.Image) {
	s.cluster = c
	if image == nil {
		return
	}
	var stream bytes.Buffer
	for {
		polled := image.Poll(func(b *atomic.Buffer, offset, length int32, _ *logbuffer.Header) {
			stream.Write(b.GetBytesArray(offset, length))
		}, 64)
		if image.IsEndOfStream() || image.IsClosed() {
			break
		}
		if polled == 0 {
			c.IdleStrategy().Idle(0)
		}
	}
	if err := s.restoreSnapshot(&stream); err != nil {
		panic(fmt.Sprintf("gomatch: snapshot restore failed: %v", err))
	}
}

func (s *MatchingService) restoreSnapshot(r io.Reader) error {
	book, err := engine.RestoreOrderBook(r)
	if err != nil {
		return err
	}
	s.book = book
	return nil
}

func (s *MatchingService) OnSessionOpen(session cluster.ClientSession, timestamp int64) {
	s.sessions[session.Id()] = session
	s.sessionIds = append(s.sessionIds, session.Id())
}

func (s *MatchingService) OnSessionClose(session cluster.ClientSession, timestamp int64, _ ccodecs.CloseReasonEnum) {
	delete(s.sessions, session.Id())
	for i, id := range s.sessionIds {
		if id == session.Id() {
			s.sessionIds = append(s.sessionIds[:i], s.sessionIds[i+1:]...)
			break
		}
	}
}

func (s *MatchingService) OnSessionMessage(
	session cluster.ClientSession,
	timestamp int64,
	buffer *atomic.Buffer,
	offset int32,
	length int32,
	header *logbuffer.Header,
) {
	if length < sbeHeaderLength {
		return
	}
	blockLength := buffer.GetUInt16(offset)
	templateId := buffer.GetUInt16(offset + 2)
	msgSchemaId := buffer.GetUInt16(offset + 4)
	version := buffer.GetUInt16(offset + 6)
	if msgSchemaId != schemaId {
		logger.Errorf("unexpected schemaId=%d templateId=%d", msgSchemaId, templateId)
		return
	}
	body := &bytes.Buffer{}
	buffer.WriteBytes(body, offset+sbeHeaderLength, length-sbeHeaderLength)

	var events []engine.Event
	switch templateId {
	case newOrderTemplateId:
		msg := codecs.NewOrder{}
		if err := msg.Decode(s.marshaller, body, version, blockLength, true); err != nil {
			logger.Errorf("NewOrder decode error: %v", err)
			return
		}
		events = s.book.NewLimitOrder(engine.NewOrderCmd{
			ClientOrderId: msg.ClientOrderId,
			Owner:         session.Id(),
			Side:          engine.Side(msg.Side),
			Price:         msg.Price,
			Qty:           msg.Qty,
		})
	case cancelOrderTemplateId:
		msg := codecs.CancelOrder{}
		if err := msg.Decode(s.marshaller, body, version, blockLength, true); err != nil {
			logger.Errorf("CancelOrder decode error: %v", err)
			return
		}
		events = s.book.Cancel(msg.OrderId, session.Id())
	default:
		logger.Debugf("ignoring unknown templateId=%d", templateId)
		return
	}
	s.route(events, timestamp)
}

func (s *MatchingService) route(events []engine.Event, timestamp int64) {
	for _, ev := range events {
		switch ev.Type {
		case engine.EvAccepted, engine.EvRejected, engine.EvCanceled:
			s.sendTo(ev.Owner, mustEncode(encodeExecutionReport(s.marshaller, ev, timestamp)))
		case engine.EvFilled:
			s.sendTo(ev.Owner, mustEncode(encodeExecutionReport(s.marshaller, ev, timestamp)))
		case engine.EvTrade:
			s.broadcast(mustEncode(encodeTrade(s.marshaller, ev, timestamp)))
		case engine.EvBookUpdate:
			s.broadcast(mustEncode(encodeBookUpdate(s.marshaller, ev, timestamp)))
		}
	}
}

// mustEncode: encoding into a bytes.Buffer cannot fail for valid messages;
// treat failure as a programming error.
func mustEncode(frame []byte, err error) []byte {
	if err != nil {
		panic(err)
	}
	return frame
}

func (s *MatchingService) sendTo(sessionId int64, frame []byte) {
	if sess, ok := s.sessions[sessionId]; ok {
		s.offer(sess, frame)
	}
}

func (s *MatchingService) broadcast(frame []byte) {
	for _, id := range s.sessionIds {
		s.offer(s.sessions[id], frame)
	}
}

func (s *MatchingService) offer(sess cluster.ClientSession, frame []byte) {
	buf := atomic.MakeBuffer(frame)
	for {
		result := sess.Offer(buf, 0, buf.Capacity(), nil)
		if result >= 0 { // includes cluster.ClientSessionMockedOffer on non-leaders
			return
		}
		if result != aeron.BackPressured && result != aeron.AdminAction {
			logger.Errorf("egress offer failed - sessionId=%d result=%d", sess.Id(), result)
			return
		}
		s.cluster.IdleStrategy().Idle(0)
	}
}

func (s *MatchingService) OnTimerEvent(correlationId, timestamp int64) {}

func (s *MatchingService) writeSnapshot(emit func([]byte) error) error {
	var stream bytes.Buffer
	if err := s.book.Snapshot(&stream); err != nil {
		return err
	}
	data := stream.Bytes()
	for len(data) > 0 {
		n := snapshotChunkSize
		if len(data) < n {
			n = len(data)
		}
		if err := emit(data[:n]); err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}

func (s *MatchingService) OnTakeSnapshot(publication *aeron.Publication) {
	err := s.writeSnapshot(func(chunk []byte) error {
		buf := atomic.MakeBuffer(chunk)
		for {
			result := publication.Offer(buf, 0, buf.Capacity(), nil)
			if result >= 0 {
				return nil
			}
			if result != aeron.BackPressured && result != aeron.AdminAction {
				return fmt.Errorf("snapshot offer failed: %d", result)
			}
			s.cluster.IdleStrategy().Idle(0)
		}
	})
	if err != nil {
		logger.Errorf("snapshot failed: %v", err)
	}
}

func (s *MatchingService) OnRoleChange(role cluster.Role) {
	logger.Infof("role change: %v", role)
}

func (s *MatchingService) OnTerminate(c cluster.Cluster) {}

func (s *MatchingService) OnNewLeadershipTermEvent(
	leadershipTermId, logPosition, timestamp, termBaseLogPosition int64,
	leaderMemberId, logSessionId int32,
	timeUnit ccodecs.ClusterTimeUnitEnum, appVersion int32,
) {
}
