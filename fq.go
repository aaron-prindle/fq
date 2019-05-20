package fq

import (
	"math"
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
	if packet.queueidx < 0 || packet.queueidx > len(q.queues) {
		panic("no matching queue for packet")
	}
	return q.queues[packet.queueidx]
}

func NewFQScheduler(queues []*Queue, clock clock.Clock) *FQScheduler {
	fq := &FQScheduler{
		lock:   sync.Mutex{},
		queues: queues,
		clock:  clock,
		vt:     0,
	}
	return fq
}

// TODO(aaron-prindle) verify that the time units are correct/matching
func (q *FQScheduler) NowAsUnixNano() float64 {
	return float64(q.clock.Now().UnixNano())
}``

func (q *FQScheduler) Enqueue(packet *Packet) {
	q.lock.Lock()
	defer q.lock.Unlock()
	q.synctime()

	queue := q.chooseQueue(packet)
	packet.starttime = q.NowAsUnixNano()
	packet.queue = queue

	queue.enqueue(packet)
	q.updateTime(packet, queue)
}

func (q *FQScheduler) now() float64 {
	return q.vt
}

func (q *FQScheduler) synctime() {
	now := q.NowAsUnixNano()
	timesincelast := now - q.lastrealtime
	q.lastrealtime = now
	q.vt += timesincelast * q.getvirtualtimeratio()
}

func (q *FQScheduler) getvirtualtimeratio() float64 {
	NEQ := 0
	reqs := 0
	for _, queue := range q.queues {
		reqs += queue.RequestsExecuting
		reqs += len(queue.Packets)
		if len(queue.Packets) > 0 || queue.RequestsExecuting > 0 {
			NEQ++
		}
	}
	// no active flows, virtual time does not advance (also avoid div by 0)
	if NEQ == 0 {
		return 0
	}
	return min(float64(reqs), float64(C)) / float64(NEQ)
}

func (q *FQScheduler) updateTime(packet *Packet, queue *Queue) {
	// When a request arrives to an empty queue with no requests executing
	// len(queue.Packets) == 1 as enqueue has just happened prior (vs  == 0)
	if len(queue.Packets) == 1 && queue.RequestsExecuting == 0 {
		// the queue’s virtual start time is set to now().
		queue.virstart = q.now()
	}
}

func (q *FQScheduler) Dequeue() (*Packet, bool) {
	q.lock.Lock()
	defer q.lock.Unlock()
	// TODO(aaron-prindle) unclear if this should be here...
	// q.synctime()
	queue := q.selectQueue()

	if queue == nil {
		return nil, false
	}
	packet, ok := queue.dequeue()

	if ok {
		// When a request is dequeued for service -> q.virstart += G
		queue.virstart += G
	}
	queue.RequestsExecuting++
	return packet, ok
}

func (q *FQScheduler) roundrobinqueue() int {
	q.robinidx = (q.robinidx + 1) % len(q.queues)
	return q.robinidx
}

func (q *FQScheduler) selectQueue() *Queue {
	minvirfinish := math.Inf(1)
	var minqueue *Queue
	for range q.queues {
		idx := q.roundrobinqueue()
		queue := q.queues[idx]
		if len(queue.Packets) != 0 && queue.Packets[0].virfinish(0) < minvirfinish {
			minvirfinish = queue.Packets[0].virfinish(0)
			minqueue = queue
			q.robinidx = idx
		}
	}
	return minqueue
}
