package engine

import (
	"bytes"
	"math/rand"
	"reflect"
	"testing"
)

// The engine must be a pure function of its command sequence: the same
// stream applied to two books yields identical events and identical
// snapshot bytes. Seeded rand is fine here - it generates the *inputs*.
func TestDeterministicReplay(t *testing.T) {
	commands := make([]NewOrderCmd, 0, 2000)
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 2000; i++ {
		commands = append(commands, NewOrderCmd{
			ClientOrderId: int64(i),
			Owner:         int64(rng.Intn(5) + 1),
			Side:          Side(rng.Intn(2)),
			Price:         int64(rng.Intn(20) + 90),
			Qty:           int64(rng.Intn(50) + 1),
		})
	}
	run := func() ([]Event, []byte) {
		b := NewOrderBook()
		var events []Event
		for i, cmd := range commands {
			events = append(events, b.NewLimitOrder(cmd)...)
			if i%7 == 3 { // deterministic sprinkle of cancels
				events = append(events, b.Cancel(int64(i/2+1), cmd.Owner)...)
			}
		}
		var snap bytes.Buffer
		if err := b.Snapshot(&snap); err != nil {
			t.Fatal(err)
		}
		return events, snap.Bytes()
	}
	ev1, snap1 := run()
	ev2, snap2 := run()
	if !reflect.DeepEqual(ev1, ev2) {
		t.Fatal("event streams differ between identical runs")
	}
	if !bytes.Equal(snap1, snap2) {
		t.Fatal("snapshots differ between identical runs")
	}
}
