package fq

import (
	"fmt"
	"math"
	"sync"
	"time"
)

type fqscheduler struct {
	lock   *sync.Mutex
	queues []*Queue
	vt     uint64
	C      uint64
	G      uint64
	seen   bool
}

// TODO(aaron-prindle) add concurrency enforcement - 'C'

func (q *fqscheduler) chooseQueue(packet *Packet) *Queue {
	for _, queue := range q.queues {
		if packet.key == queue.key {
			return queue
		}
	}
	panic("no matching queue for packet")
}

func newfqscheduler(queues []*Queue) *fqscheduler {
	fq := &fqscheduler{
		lock:   &sync.Mutex{},
		queues: queues,
		// R(t) = (server start time) + (1 ns) * (number of rounds since server start).
		vt: NowAsUnixMilli(),
	}
	// TODO(aaron-prindle) verify if this is needed?
	for i := range fq.queues {
		fq.queues[i].virstart = fq.vt
	}

	return fq
}

// TODO(aaron-prindle) verify that the time units are correct/matching
func NowAsUnixMilli() uint64 {
	return uint64(time.Now().UnixNano() / 1e6)
}

func (q *fqscheduler) processround() (*Packet, bool) {
	q.tick()
	return q.dequeue()
}

func (q *fqscheduler) enqueue(packet *Packet) {
	q.lock.Lock()
	defer q.lock.Unlock()

	fmt.Printf("enqueue: %d\n", packet.key)
	q.seen = true

	queue := q.chooseQueue(packet)
	// TODO(aaron-prindle) q.now() or get virstart of the queue you go in?
	packet.starttime = q.now()
	packet.queue = queue

	// TODO(aaron-prindle) verify the order here
	queue.enqueue(packet)
	q.updateTime(packet, queue)

}

func (q *fqscheduler) now() uint64 {
	return q.vt
}

func (q *fqscheduler) tick() {
	NEQ := 0
	reqs := 0

	for _, queue := range q.queues {
		reqs += len(queue.RequestsExecuting)
		reqs += len(queue.Packets)
		if len(queue.Packets) > 0 || len(queue.RequestsExecuting) > 0 {
			NEQ += 1
		}
	}

	// no active flows
	if NEQ == 0 {
		return
	}

	// min(sum[over q] reqs(q, t), C) / NEQ(t)
	// fmt.Printf("vt/dt %d\n", min(uint64(reqs), uint64(C))/uint64(NEQ))
	fmt.Printf("vt/dt %d\n", uint64(math.Ceil(float64(min(uint64(reqs), uint64(C)))/float64(uint64(NEQ)))))

	q.vt += uint64(math.Ceil(float64(min(uint64(reqs), uint64(C))) / float64(uint64(NEQ))))
}

func (q *fqscheduler) updateTime(packet *Packet, queue *Queue) {

	// When a request arrives to an empty queue with no requests executing
	// (enqueue has just happened prior)
	if len(queue.Packets) == 1 && len(queue.RequestsExecuting) == 0 {
		// the queue’s virtual start time is set to now().
		queue.virstart = q.now()
	}

	// below was done in orig fq to not give queues priority for not being used recently
	queue.virstart = max(q.now(), queue.lastvirfinish())
}

func (q *fqscheduler) dequeue() (*Packet, bool) {
	q.lock.Lock()
	defer q.lock.Unlock()

	queue := q.selectQueue()
	if queue == nil {
		return nil, false
	}

	fmt.Println(queue.String())

	packet, ok := queue.dequeue()

	if ok {
		fmt.Printf("dequeue: %d\n", packet.key)
		// When a request is dequeued for service the queue’s virtual start
		// time is advanced by G
		queue.virstart += G
	}
	// queue.RequestsExecuting = append(queue.RequestsExecuting, packet)
	return packet, ok
}

func (q *fqscheduler) selectQueue() *Queue {
	// While the queue is empty and has no requests executing
	// the value of its virtual start time variable is ignored and its last
	// virtual finish time is considered to be in the virtual past

	minvirfinish := uint64(math.MaxUint64)
	var minqueue *Queue
	for _, queue := range q.queues {
		if len(queue.Packets) != 0 && queue.Packets[0].virfinish(0) < minvirfinish {
			minvirfinish = queue.Packets[0].virfinish(0)
			minqueue = queue
		}
	}
	return minqueue
}
