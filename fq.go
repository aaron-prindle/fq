package fq

import (
	"math"
	"sync"
	"time"
)

// mods for our algo
// use min heap vs selectQueue
// 1) we are dispatching requests to be served rather than packets to be transmitted
// 2) the actual service time (i.e., duration) is not known until a request is done being served
//
// 1 & 2 can be handled by using duration time instead of size

func (q *fqscheduler) chooseQueue(packet *Packet) *Queue {
	for _, queue := range q.queues {
		if packet.key == queue.key {
			// use shuffle sharding to get a queue
			packet.queue = queue
			return queue
		}
	}
	// fmt.Printf("packet w/ key: %v\n", packet.key)
	panic("no matching queue for packet")
}

type fqscheduler struct {
	lock   sync.Mutex
	queues []*Queue
	closed bool
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

	// STARTING PACKET SERVICING
	packet.starttime = uint64(time.Now().UnixNano())

	packet.updateTimeQueued()
}

func (q *fqscheduler) dequeue() (*Packet, bool) {
	q.lock.Lock()
	defer q.lock.Unlock()

	queue := q.selectQueue()
	if queue == nil {
		return nil, false
	}
	packet, ok := queue.dequeue()
	if ok {
		packet.endtime = uint64(time.Now().UnixNano())
	}
	if ok {
		packet.updateTimeDequeued()
	}
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
