package fq

import (
	"fmt"
	"math"
	"sort"
	"sync"

	"k8s.io/apimachinery/pkg/util/clock"
)

type FQScheduler struct {
	lock   sync.Mutex
	queues []*Queue
	clock  clock.Clock
	// Verify float64 has enough bits
	// We will want to be careful about having enough bits. For example, in float64, 1e20 + 1e0 == 1e20.
	vt           float64
	C            float64
	G            float64
	lastrealtime float64
	robinidx     int
}

// TODO(aaron-prindle) add concurrency enforcement - 'C'

func (q *FQScheduler) chooseQueue(packet *Packet) *Queue {
	for _, queue := range q.queues {
		if packet.key == queue.key {
			return queue
		}
	}
	panic("no matching queue for packet")
}

func newFQScheduler(queues []*Queue, clock clock.Clock) *FQScheduler {
	fq := &FQScheduler{
		lock:   sync.Mutex{},
		queues: queues,
		clock:  clock,
		// R(t) = (server start time) + (1 ns) * (number of rounds since server start).
		vt: 0,
		// vt: now,
	}
	now := fq.NowAsUnixNano()
	fq.lastrealtime = now
	fq.vt = now
	return fq
}

// TODO(aaron-prindle) verify that the time units are correct/matching
func (q *FQScheduler) NowAsUnixNano() float64 {
	return float64(q.clock.Now().UnixNano())
}

func (q *FQScheduler) Enqueue(packet *Packet) {
	q.lock.Lock()
	defer q.lock.Unlock()
	q.synctime()

	fmt.Printf("enqueue: %d\n", packet.key)

	queue := q.chooseQueue(packet)
	packet.starttime = q.NowAsUnixNano()
	packet.queue = queue

	// TODO(aaron-prindle) verify the order here
	queue.enqueue(packet)
	q.updateTime(packet, queue)
}

func (q *FQScheduler) now() float64 {
	return q.vt
}

func (q *FQScheduler) synctime() {
	// anything that looks at now updates the time?
	// updatetime?
	now := q.NowAsUnixNano()
	timesincelast := now - q.lastrealtime
	// fmt.Printf("timesincelast: %f\n", timesincelast)
	// fmt.Printf("q.getvirtualtimeratio: %f\n", q.getvirtualtimeratio())
	q.lastrealtime = now

	q.vt += timesincelast * q.getvirtualtimeratio()
}

func (q *FQScheduler) getvirtualtimeratio() float64 {
	NEQ := 0
	reqs := 0

	for _, queue := range q.queues {
		reqs += len(queue.RequestsExecuting)
		reqs += len(queue.Packets)
		if len(queue.Packets) > 0 || len(queue.RequestsExecuting) > 0 {
			NEQ++
		}
	}

	// no active flows
	if NEQ == 0 {
		return 0
	}

	// q.vt can be 0 if NEQ >> min(sum[over q] reqs(q,t), C)
	// ceil used to guarantee q.vt advances each step
	return min(float64(reqs), float64(C)) / float64(NEQ)
}

func (q *FQScheduler) updateTime(packet *Packet, queue *Queue) {
	// When a request arrives to an empty queue with no requests executing
	// (enqueue has just happened prior)
	if len(queue.Packets) == 1 && len(queue.RequestsExecuting) == 0 {
		// the queue’s virtual start time is set to now().
		queue.virstart = q.now()
	}
}

func (q *FQScheduler) Dequeue() (*Packet, bool) {
	q.lock.Lock()
	defer q.lock.Unlock()
	// TODO(aaron-prindle) unclear if this should be here...
	// q.synctime()

	// print sorted queue
	q.printsortedqueue()

	queue := q.selectQueue()
	printdequeue(queue)

	if queue == nil {
		return nil, false
	}

	packet, ok := queue.dequeue()

	if ok {
		// When a request is dequeued for service the queue’s virtual start
		// time is advanced by G
		queue.virstart += G
	}
	// queue.RequestsExecuting = append(queue.RequestsExecuting, packet)
	return packet, ok
}

func (q *FQScheduler) getroundrobinqueue() (*Queue, int) {
	// curidx := q.robinidx

	q.robinidx = (q.robinidx + 1) % len(q.queues)
	queue := q.queues[q.robinidx]
	return queue, q.robinidx
}

func (q *FQScheduler) selectQueue() *Queue {
	minvirfinish := math.Inf(1)
	// minvirfinish := float64(math.Maxfloat64)
	var minqueue *Queue
	for range q.queues {
		queue, idx := q.getroundrobinqueue()
		// if len(queue.Packets) != 0 {
		// 	fmt.Printf("i: %d\n", i)
		// 	fmt.Printf("queue.key: %d, queue.Packets[0].virfinish(0): %f\n", queue.key, queue.Packets[0].virfinish(0))
		// 	fmt.Printf("%s======\n", queue.String())
		// }

		if len(queue.Packets) != 0 && queue.Packets[0].virfinish(0) < minvirfinish {
			// fmt.Printf("queue.key: %d, queue.Packets[0].virfinish(0): %f\n", queue.key, queue.Packets[0].virfinish(0))
			// fmt.Printf("%s======\n", queue.String())
			minvirfinish = queue.Packets[0].virfinish(0)
			minqueue = queue
			q.robinidx = idx
		}
	}
	// fmt.Printf("q.robinidx: %d\n", q.robinidx)
	return minqueue
}

// ====

func (q *FQScheduler) printsortedqueue() {
	sorted := append(q.queues[:0:0], q.queues...)
	sort.Slice(sorted, func(i, j int) bool {
		var x, y float64
		if len(sorted[i].Packets) == 0 {
			x = math.Inf(-1)
		} else {
			x = sorted[i].Packets[0].virfinish(0)
		}
		if len(sorted[j].Packets) == 0 {
			y = math.Inf(-1)
		} else {
			y = sorted[j].Packets[0].virfinish(0)

		}
		return x < y
	})
	for _, queue := range sorted {
		fmt.Println("===sortedqueues===")
		if len(queue.Packets) != 0 {
			fmt.Printf("queue.key: %d\n", queue.key)
			fmt.Printf("queue.Packets[0].virfinish(0): %f\n", queue.Packets[0].virfinish(0))
			fmt.Printf("%s======\n", queue.String())
		}
		fmt.Println("===sortedqueuesDONE===")
	}
}

func printdequeue(queue *Queue) {
	fmt.Println("***dequeue***")
	fmt.Printf("dequeue: %d\n", queue.key)
	fmt.Printf("%s\n", queue.String())
	fmt.Println("***")
}
