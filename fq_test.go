package fq

import (
	"fmt"
	"math"
	"math/rand"
	"runtime"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/clock"
)

// adapted from https://github.com/tadglines/wfq/blob/master/wfq_test.go

type flowDesc struct {
	// In
	ftotal int // Total units in flow
	imin   int // Min Packet size
	imax   int // Max Packet size

	// Out
	idealPercent  float64
	actualPercent float64
}

func genFlow(fq *FQScheduler, desc *flowDesc, key int) {
	for i, t := int(1), int(0); t < desc.ftotal; i++ {
		it := new(Packet)
		it.queueidx = key
		if desc.imin == desc.imax {
			it.size = desc.imax
		} else {
			it.size = desc.imin + rand.Intn(desc.imax-desc.imin)
		}
		if t+it.size > desc.ftotal {
			it.size = desc.ftotal - t
		}
		t += it.size
		it.seq = i
		// new packet
		fq.Enqueue(it)
	}
}

func consumeQueue(t *testing.T, fq *FQScheduler, descs []flowDesc) (float64, error) {
	active := make(map[int]bool)
	var total int
	acnt := make(map[int]int)
	cnt := make(map[int]int)
	seqs := make(map[int]int)

	wsum := uint64(len(descs))

	for i, ok := fq.Dequeue(); ok; i, ok = fq.Dequeue() {
		// callback to update virtualtime w/ correct service time for request
		i.finishRequest(fq)

		it := i
		seq := seqs[it.queueidx]
		if seq+1 != it.seq {
			return 0, fmt.Errorf("Packet for flow %d came out of queue out-of-order: expected %d, got %d", it.queueidx, seq+1, it.seq)
		}
		seqs[it.queueidx] = it.seq

		// set the flow this item is a part of to active
		if cnt[it.queueidx] == 0 {
			active[it.queueidx] = true
		}
		cnt[it.queueidx] += it.size

		// if # of active flows is equal to the # of total flows, add to total
		// we are correctly taking a bit from each
		if len(active) == len(descs) {
			acnt[it.queueidx] += it.size
			total += it.size
		}

		// if all items have been processed from the flow, remove it from active
		if cnt[it.queueidx] == descs[it.queueidx].ftotal {
			delete(active, it.queueidx)
		}
	}

	if total == 0 {
		t.Fatalf("expected 'total' to be nonzero")
	}

	var variance float64
	for key := 0; key < len(descs); key++ {
		// flows in this test have same expected # of requests
		// idealPercent = total-all-active/len(flows) / total-all-active
		// "how many bytes/requests you expect for this flow - all-active"
		descs[key].idealPercent = float64(100) / float64(wsum)

		// actualPercent = requests-for-this-flow-all-active / total-reqs
		// "how many bytes/requests you got for this flow - all-active"
		descs[key].actualPercent = (float64(acnt[key]) / float64(total)) * 100

		x := descs[key].idealPercent - descs[key].actualPercent
		x *= x
		variance += x
	}
	variance /= float64(len(descs))

	stdDev := math.Sqrt(variance)
	return stdDev, nil
}

func TestSingleFlow(t *testing.T) {
	var flows = []flowDesc{
		{100, 1, 1, 0, 0},
	}
	flowStdDevTest(t, flows, 0)
}

func TestUniformMultiFlow(t *testing.T) {
	var flows = []flowDesc{
		{100, 1, 1, 0, 0},
		{100, 1, 1, 0, 0},
		{100, 1, 1, 0, 0},
		{100, 1, 1, 0, 0},
		{100, 1, 1, 0, 0},
		{100, 1, 1, 0, 0},
		{100, 1, 1, 0, 0},
		{100, 1, 1, 0, 0},
		{100, 1, 1, 0, 0},
		{100, 1, 1, 0, 0},
	}
	// .35 was expectedStdDev used in
	// https://github.com/tadglines/wfq/blob/master/wfq_test.go
	flowStdDevTest(t, flows, .041)
}

func TestOneBurstingFlow(t *testing.T) {

	var flows = []flowDesc{
		{1000, 1, 1, 0, 0},
		{100, 1, 1, 0, 0},
	}
	flowStdDevTest(t, flows, 0)
}

func flowStdDevTest(t *testing.T, flows []flowDesc, expectedStdDev float64) {
	runtime.GOMAXPROCS(runtime.NumCPU())
	queues := initQueues(len(flows), 0)

	// a fake clock that returns the current time is used for enqueing which
	// returns the same time (now)
	// this simulates all queued requests coming at the same time
	now := time.Now()
	fc := clock.NewFakeClock(now)
	fq := NewFQScheduler(queues, fc)
	for n := 0; n < len(flows); n++ {
		genFlow(fq, &flows[n], n)
	}

	// prior to dequeing, we switch to an interval clock which will simulate
	// each dequeue happening at a fixed interval of time
	ic := &clock.IntervalClock{
		Time:     now,
		Duration: time.Millisecond,
	}
	fq.clock = ic

	stdDev, err := consumeQueue(t, fq, flows)

	if err != nil {
		t.Fatal(err.Error())
	}

	if stdDev > expectedStdDev {
		for k, d := range flows {
			t.Logf("For flow %d: Expected %v%%, got %v%%", k, d.idealPercent, d.actualPercent)
		}
		t.Fatalf("StdDev was expected to be < %f but got %v", expectedStdDev, stdDev)
	}
}
