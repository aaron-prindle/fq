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
	// TODO(aaron-prindle) verify that init times is correct
	// witnessed bugs in test without this initialization
	//   as test enqueues packets prior
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

	// fmt.Printf("enqueue: %d\n", packet.key)

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
	// still verifying where it makes sense to call this...
	now := q.NowAsUnixNano()
	timesincelast := now - q.lastrealtime
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

	// DEBUG(aaron-prindle)
	// q.printsortedqueue()

	queue := q.selectQueue()

	if queue == nil {
		return nil, false
	}

	// DEBUG(aaron-prindle)
	// printdequeue(queue, q.robinidx)

	packet, ok := queue.dequeue()

	if ok {
		// When a request is dequeued for service -> q.virstart += G
		queue.virstart += G
	}
	// queue.RequestsExecuting = append(queue.RequestsExecuting, packet)
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
	// DEBUG(aaron-prindle)
	// fmt.Printf("q.robinidx: %d\n", q.robinidx)
	return minqueue
}
