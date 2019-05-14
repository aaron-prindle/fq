package fq

import (
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
}

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
		C:      300,
		G:      60000,
		vt:     NowAsUnixMilli(),
	}
	return fq
}

// TODO(aaron-prindle) verify that the time units are correct/matching
func NowAsUnixMilli() uint64 {
	return uint64(time.Now().UnixNano() / 1e6)
}

func DurationAsMilli(t time.Duration) uint64 {
	return uint64(t.Nanoseconds() / 1000000)
}

func (q *fqscheduler) processround() (*Packet, bool) {
	q.tick()
	return q.dequeue()
}

func (q *fqscheduler) enqueue(packet *Packet) {
	q.lock.Lock()
	defer q.lock.Unlock()

	queue := q.chooseQueue(packet)

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
		if len(queue.Packets) > 0 || len(queue.RequestsExecuting) > 0 {
			NEQ += 1
		}
	}

	// no active flows
	if NEQ == 0 {
		return
	}

	// min(sum[over q] reqs(q, t), C) / NEQ(t)
	q.vt += min(uint64(reqs), uint64(q.C)) / uint64(NEQ)
}

func (q *fqscheduler) updateTime(packet *Packet, queue *Queue) {
	// When a request arrives to an empty queue with no requests executing

	// TODO(aaron-prindle) len(queue.Packets) // len(queue.Packets)-1?
	if len(queue.Packets)-1 == 0 && len(queue.RequestsExecuting) == 0 {
		// the queue’s virtual start time is set to now().
		queue.virstart = q.now()
	}

	// this is done to not give queues priority for not being used recently
	queue.virstart = max(q.now(), queue.lastvirfinish())
}

func (q *fqscheduler) dequeue() (*Packet, bool) {
	q.lock.Lock()
	defer q.lock.Unlock()

	queue := q.selectQueue()
	if queue == nil {
		return nil, false
	}
	packet, ok := queue.dequeue()
	queue.virstart += q.G
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
		if len(queue.Packets) != 0 && queue.Packets[0].virfinish() < minvirfinish {
			minvirfinish = queue.Packets[0].virfinish()
			minqueue = queue
		}
	}
	return minqueue
}
