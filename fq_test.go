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

func genFlow(fq *fqscheduler, desc *flowDesc, key uint64, done_wg *sync.WaitGroup) {
	for i, t := uint64(1), uint64(0); t < desc.ftotal; i++ {
		//time.Sleep(time.Microsecond)
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
		fq.enqueue(it)
	}
	(*done_wg).Done()
}

func consumeQueue(t *testing.T, fq *fqscheduler, descs []flowDesc) (float64, error) {
	active := make(map[uint64]bool)
	var total uint64
	acnt := make(map[uint64]uint64)
	cnt := make(map[uint64]uint64)
	seqs := make(map[uint64]uint64)

	var wsum uint64
	for range descs {
		wsum += uint64(0 + 1)
	}

	time.Sleep(1 * time.Second)
	for i, ok := fq.dequeue(); ok; i, ok = fq.dequeue() {
		time.Sleep(time.Microsecond) // Simulate constrained bandwidth
		it := i
		seq := seqs[it.key]
		if seq+1 != it.seq {
			return 0, fmt.Errorf("Packet for flow %d came out of queue out-of-order: expected %d, got %d", it.key, seq+1, it.seq)
		}
		seqs[it.key] = it.seq

		if cnt[it.key] == 0 {
			active[it.key] = true
		}
		cnt[it.key] += it.size

		if len(active) == len(descs) {
			acnt[it.key] += it.size
			total += it.size
		}

		if cnt[it.key] == descs[it.key].ftotal {
			delete(active, it.key)
		}
	}

	var variance float64
	for key := uint64(0); key < uint64(len(descs)); key++ {
		if total == 0 {
			t.Fatalf("expected 'total' to be nonzero")
		}
		descs[key].idealPercent = (((float64(total) * float64(0+1)) / float64(wsum)) / float64(total)) * 100
		descs[key].actualPercent = (float64(acnt[key]) / float64(total)) * 100

		x := descs[key].idealPercent - descs[key].actualPercent
		x *= x
		variance += x
	}

	stdDev := math.Sqrt(variance)
	return stdDev, nil
}

func TestSingleFlow(t *testing.T) {
	runtime.GOMAXPROCS(runtime.NumCPU())
	queues := initQueues(1, 1)
	fq := newfqscheduler(queues)

	go func() {
		for i := 1; i < 10000; i++ {
			it := &Packet{}
			it.key = 1
			it.size = uint64(rand.Int63n(10) + 1)
			it.seq = uint64(i)
			fq.enqueue(it)
		}
	}()

	var seq uint64
	hasEntered := false
	for hasEntered {
		for it, ok := fq.dequeue(); ok; it, ok = fq.dequeue() {
			if seq+1 != it.seq {
				t.Fatalf("Packet came out of queue out-of-order: expected %d, got %d", seq+1, it.seq)
			}
			seq = it.seq
		}
	}
}

func TestUniformMultiFlow(t *testing.T) {
	runtime.GOMAXPROCS(runtime.NumCPU())
	queues := initQueues(10, 0)
	fq := newfqscheduler(queues)

	var swg sync.WaitGroup
	var wg sync.WaitGroup

	var flows = []flowDesc{
		{1000, 1, 1, 0, 0},
		{1000, 1, 1, 0, 0},
		{1000, 1, 1, 0, 0},
		{1000, 1, 1, 0, 0},
		{1000, 1, 1, 0, 0},
		{1000, 1, 1, 0, 0},
		{1000, 1, 1, 0, 0},
		{1000, 1, 1, 0, 0},
		{1000, 1, 1, 0, 0},
		{1000, 1, 1, 0, 0},
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

	if stdDev > 0.2 {
		for k, d := range flows {
			t.Logf("For flow %d: Expected %v%%, got %v%%", k, d.idealPercent, d.actualPercent)
		}
		t.Fatalf("StdDev was expected to be < 0.1 but got %v", stdDev)
	}
}

func TestOneBurstFlow(t *testing.T) {
	runtime.GOMAXPROCS(runtime.NumCPU())
	queues := initQueues(2, 0)
	fq := newfqscheduler(queues)

	var swg sync.WaitGroup
	var wg sync.WaitGroup

	var flows = []flowDesc{
		{1000, 1, 1, 0, 0},
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

	if stdDev > 0.4 {
		for k, d := range flows {
			t.Logf("For flow %d: Expected %v%%, got %v%%", k, d.idealPercent, d.actualPercent)
		}
		t.Fatalf("StdDev was expected to be < 0.1 but got %v", stdDev)
	}
}
