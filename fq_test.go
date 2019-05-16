package fq

import (
	"fmt"
	"math"
	"math/rand"
	"runtime"
	"sync"
	"testing"
	"time"
)

// adapted from https://github.com/tadglines/wfq/blob/master/wfq_test.go

type flowDesc struct {
	// In
	ftotal uint64 // Total units in flow
	imin   uint64 // Min Packet size
	imax   uint64 // Max Packet size

	// Out
	idealPercent  float64
	actualPercent float64
}

func genFlow(fq *FQScheduler, desc *flowDesc, key uint64, done_wg *sync.WaitGroup) {
	for i, t := uint64(1), uint64(0); t < desc.ftotal; i++ {
		it := new(Packet)
		it.key = key
		if desc.imin == desc.imax {
			it.size = desc.imax
		} else {
			it.size = desc.imin + uint64(rand.Int63n(int64(desc.imax-desc.imin)))
		}
		if t+it.size > desc.ftotal {
			it.size = desc.ftotal - t
		}
		t += it.size
		it.seq = i
		// new packet
		fq.Enqueue(it)
	}
	(*done_wg).Done()
}

func consumeQueue(t *testing.T, fq *FQScheduler, descs []flowDesc) (float64, error) {
	active := make(map[uint64]bool)
	var total uint64
	acnt := make(map[uint64]uint64)
	cnt := make(map[uint64]uint64)
	seqs := make(map[uint64]uint64)

	var wsum uint64
	for range descs {
		wsum += uint64(1)
	}

	// for !fq.seen {

	// }
	// TODO(aaron-prindle) remove this
	time.Sleep(1 * time.Second)
	for i, ok := fq.processround(); ok; i, ok = fq.processround() {
		time.Sleep(time.Microsecond) // Simulate constrained bandwidth
		it := i
		seq := seqs[it.key]
		if seq+1 != it.seq {
			return 0, fmt.Errorf("Packet for flow %d came out of queue out-of-order: expected %d, got %d", it.key, seq+1, it.seq)
		}
		seqs[it.key] = it.seq

		// set the flow this item is a part of to active
		if cnt[it.key] == 0 {
			active[it.key] = true
		}
		cnt[it.key] += it.size

		// if # of active flows is equal to the # of total flows, add to total
		// we are correctly taking a bit from each
		if len(active) == len(descs) {
			acnt[it.key] += it.size
			total += it.size
		}

		// if all items have been processed from the flow, remove it from active
		if cnt[it.key] == descs[it.key].ftotal {
			delete(active, it.key)
		}
	}

	var variance float64
	for key := uint64(0); key < uint64(len(descs)); key++ {
		if total == 0 {
			t.Fatalf("expected 'total' to be nonzero")
		}
		// idealPercent = total-all-active/len(flows) / total-all-active
		// idealPercent = ~((10000/10) / 10000) * 100 = ~10
		// "how many bytes/requests you expect for this flow - all-active"
		descs[key].idealPercent = (((float64(total)) / float64(wsum)) / float64(total)) * 100

		// actualPercent = requests-for-this-flow-all-active / total-reqs
		// "how many bytes/requests you got for this flow - all-active"
		descs[key].actualPercent = (float64(acnt[key]) / float64(total)) * 100

		x := descs[key].idealPercent - descs[key].actualPercent
		x *= x
		variance += x
	}
	fmt.Println(descs)

	stdDev := math.Sqrt(variance)
	return stdDev, nil
}

func TestSingleFlow(t *testing.T) {
	runtime.GOMAXPROCS(runtime.NumCPU())
	queues := initQueues(1, 1)
	fq := newFQScheduler(queues)

	go func() {
		for i := 1; i < 10000; i++ {
			it := &Packet{}
			it.key = 1
			it.size = uint64(rand.Int63n(10) + 1)
			it.seq = uint64(i)
			fq.Enqueue(it)
		}
	}()

	var seq uint64
	hasEntered := false
	for hasEntered {
		for it, ok := fq.Dequeue(); ok; it, ok = fq.Dequeue() {
			if seq+1 != it.seq {
				t.Fatalf("Packet came out of queue out-of-order: expected %d, got %d", seq+1, it.seq)
			}
			seq = it.seq
		}
	}
}

func TestTimingFlow(t *testing.T) {
	runtime.GOMAXPROCS(runtime.NumCPU())
	queues := initQueues(100, 0)
	fq := newFQScheduler(queues)

	var swg sync.WaitGroup
	var wg sync.WaitGroup

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

	swg.Add(1)
	wg.Add(len(flows))
	for n := 0; n < len(flows); n++ {
		go genFlow(fq, &flows[n], uint64(n), &wg)
	}

	go func() {
		wg.Wait()
	}()
	swg.Done()

	_, err := consumeQueue(t, fq, flows)
	// stdDev, err := consumeQueue(t, fq, flows)

	if err != nil {
		t.Fatal(err.Error())
	}

	// if stdDev > 0.3 {
	// 	// if stdDev > 0.1 {
	// 	for k, d := range flows {
	// 		t.Logf("For flow %d: Expected %v%%, got %v%%", k, d.idealPercent, d.actualPercent)
	// 	}
	// 	t.Fatalf("StdDev was expected to be < 0.1 but got %v", stdDev)
	// }
}

func TestUniformMultiFlow(t *testing.T) {
	runtime.GOMAXPROCS(runtime.NumCPU())
	queues := initQueues(10, 0)
	fq := newFQScheduler(queues)

	var swg sync.WaitGroup
	var wg sync.WaitGroup

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

	swg.Add(1)
	wg.Add(len(flows))
	for n := 0; n < len(flows); n++ {
		go genFlow(fq, &flows[n], uint64(n), &wg)
	}

	go func() {
		wg.Wait()
	}()
	swg.Done()

	stdDev, err := consumeQueue(t, fq, flows)

	if err != nil {
		t.Fatal(err.Error())
	}

	// if stdDev > 0.2 {
	if stdDev > 0.1 {
		for k, d := range flows {
			t.Logf("For flow %d: Expected %v%%, got %v%%", k, d.idealPercent, d.actualPercent)
		}
		t.Fatalf("StdDev was expected to be < 0.1 but got %v", stdDev)
	}
}

func TestOneBurstingFlow(t *testing.T) {
	runtime.GOMAXPROCS(runtime.NumCPU())
	queues := initQueues(2, 0)
	fq := newFQScheduler(queues)

	var swg sync.WaitGroup
	var wg sync.WaitGroup

	var flows = []flowDesc{
		{100, 1, 1, 0, 0},
		{10, 1, 1, 0, 0},
	}

	swg.Add(1)
	wg.Add(len(flows))
	for n := 0; n < len(flows); n++ {
		go genFlow(fq, &flows[n], uint64(n), &wg)
	}

	go func() {
		wg.Wait()
	}()
	swg.Done()

	stdDev, err := consumeQueue(t, fq, flows)

	if err != nil {
		t.Fatal(err.Error())
	}

	// TODO(aaron-prindle) verify that this high of a stdDev makes sense for one high rate flow
	// if stdDev > 2.0 {
	if stdDev > 0.1 {
		for k, d := range flows {
			t.Logf("For flow %d: Expected %v%%, got %v%%", k, d.idealPercent, d.actualPercent)
		}
		t.Fatalf("StdDev was expected to be < 0.1 but got %v", stdDev)
	}
}
