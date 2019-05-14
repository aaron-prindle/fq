package fq

import (
	"time"
)

// const (
// 	scaledOne uint64 = 1 << 16
// )

// mods for our algo
// use min heap vs selectQueue
// 1) we are dispatching requests to be served rather than packets to be transmitted
// 2) the actual service time (i.e., duration) is not known until a request is done being served
//
// 1 & 2 can be handled by using duration time instead of size

type Packet struct {
	// request   http.Request
	item interface{}
	// virfinish uint64
	size  uint64
	queue *Queue
	//
	key       uint64
	seq       uint64
	starttime time.Time
}

type Queue struct {
	Packets           []*Packet
	key               uint64
	virstart          uint64
	RequestsExecuting []*Packet
	// lastvirfinish     uint64
}

func (q *Queue) enqueue(packet *Packet) {
	// TODO(aaron-prindle) verify this is correct?
	packet.queue = q
	q.Packets = append(q.Packets, packet)
}

func (q *Queue) lastvirfinish() uint64 {
	// While the queue is empty and has a request executing: the last virtual
	// finish time is the queue’s virtual start time.
	if len(q.Packets) == 0 && len(q.RequestsExecuting) > 0 {
		return q.virstart
	}

	// While the queue is non-empty:
	// the last virtual finish time of the queue is the virtual finish time of
	// the last request in the queue.
	return q.Packets[0].virfinish()
}

func (q *Queue) dequeue() (*Packet, bool) {
	if len(q.Packets) == 0 {
		return nil, false
	}
	packet := q.Packets[0]
	q.Packets = q.Packets[1:]
	return packet, true
}

func initQueues(n int, key uint64) []*Queue {
	queues := []*Queue{}
	for i := 0; i < n; i++ {
		qkey := key
		if key == 0 {
			qkey = uint64(i)
		}
		queues = append(queues, &Queue{
			Packets: []*Packet{},
			key:     qkey,
		})
	}

	return queues
}

const G = 60000

func (p *Packet) virfinish() uint64 {
	// The virtual finish time of request number J in the queue
	// (counting from J=1 for the head) is J * G + (virtual start time).

	// get index?
	// J is always 1 as you only inspect the packet at head?
	J := 1
	// TODO(aaron-prindle) p.queue is nil a lot...
	return uint64(J*G) + p.queue.virstart
}

func (p *Packet) finishRequest() {
	// S=servicetime
	S := time.Now().Sub(p.starttime)
	// TODO(aaron-prindle) verify time untis are correct
	p.queue.virstart -= G - DurationAsMilli(S)
}
