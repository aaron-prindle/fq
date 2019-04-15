package fq

import (
	"math"
	"sync"
	"time"
)

type fqscheduler struct {
	lock   sync.Mutex
	queues []*Queue
}

func (q *fqscheduler) chooseQueue(packet *Packet) *Queue {
	for _, queue := range q.queues {
		if packet.key == queue.key {
			return queue
		}
	}
	// fmt.Printf("packet w/ key: %v\n", packet.key)
	panic("no matching queue for packet")
}

func newfqscheduler(queues []*Queue) *fqscheduler {
	fq := &fqscheduler{
		queues: queues,
	}
	return fq
}

func (q *fqscheduler) enqueue(packet *Packet) {
	q.lock.Lock()
	defer q.lock.Unlock()

	queue := q.chooseQueue(packet)
	queue.enqueue(packet)
	q.updateTime(packet, queue)

}

func (q *fqscheduler) updateTime(packet *Packet, queue *Queue) {
	// virStart is the virtual start of service
	virStart := max(uint64(time.Now().UnixNano()), queue.lastvirfinish)
	// adding multiplier dramatically increases test ratios
	// packet.virfinish = packet.size + virStart
	packet.virfinish = packet.size*(scaledOne) + virStart
	queue.lastvirfinish = packet.virfinish
}

func (q *fqscheduler) dequeue() (*Packet, bool) {
	q.lock.Lock()
	defer q.lock.Unlock()

	queue := q.selectQueue()
	if queue == nil {
		return nil, false
	}
	packet, ok := queue.dequeue()
	return packet, ok
}

func (q *fqscheduler) selectQueue() *Queue {
	minvirfinish := uint64(math.MaxUint64)
	var minqueue *Queue
	for _, queue := range q.queues {
		if len(queue.Packets) != 0 && queue.Packets[0].virfinish < minvirfinish {
			minvirfinish = queue.Packets[0].virfinish
			minqueue = queue
		}
	}
	return minqueue
}
