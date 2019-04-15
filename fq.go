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
	// STARTING PACKET SERVICING, not sure if this is right place to start
	// timing
	packet.starttime = uint64(time.Now().UnixNano())

	q.updateTimeQueued(packet, queue)
}

func (q *fqscheduler) updateTimeQueued(packet *Packet, queue *Queue) {
	packet.estservicetime = 60000
	packet.actservicetime = 60000
	if len(queue.Packets) == 0 && !queue.requestsexecuting {
		// queue.virStart is ignored
		// queues.lastvirfinish is in the virtualpast
		queue.virstart = uint64(time.Now().UnixNano())
	}
	if len(queue.Packets) == 0 && queue.requestsexecuting {
		queue.virstart = queue.lastvirfinish
	}

	packet.virfinish = (uint64(len(queue.Packets)+1))*packet.estservicetime + queue.virstart

	if len(queue.Packets) > 0 {
		// last virtual finish time of the queue is the virtual finish
		// time of the last request in the queue
		queue.lastvirfinish = packet.virfinish // this pkt is last pkt
	}
}

func (q *fqscheduler) updateTimeDequeued(packet *Packet, queue *Queue) {
	queue.virstart += packet.estservicetime
}

func (q *fqscheduler) updateTimeFinished(packet *Packet, queue *Queue) {
	packet.endtime = uint64(time.Now().UnixNano())
	packet.actservicetime = packet.starttime - packet.endtime

	S := packet.actservicetime
	G := packet.estservicetime
	queue.virstart -= (G - S)
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
		q.updateTimeDequeued(packet, queue)
		q.updateTimeFinished(packet, queue)
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
