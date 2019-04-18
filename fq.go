package fq

import (
	"math"
	"sync"
)

type fqscheduler struct {
	lock   *sync.Mutex
	queues []*Queue
	vt     *virtimer
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
		lock:   &sync.Mutex{},
		queues: queues,
		vt:     &virtimer{},
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

type virtimer struct {
	round uint64
}

func (vt *virtimer) now() uint64 {
	vt.round++
	return vt.round
}

func (q *fqscheduler) updateTime(packet *Packet, queue *Queue) {
	virStart := max(q.vt.now(), queue.lastvirfinish)
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
