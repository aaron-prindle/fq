package fq

import (
	"fmt"
	"strings"
)

// TODO(aaron-prindle) currently testing with one concurrent request
const C = 1 // const C = 300

// TODO(aaron-prindle) currently service time "G" is not implemented entirely
const G = 1 // const G = 60000

type Packet struct {
	// request   http.Request
	item interface{}
	// virfinish uint64
	size  uint64
	queue *Queue
	//
	key       uint64
	seq       uint64
	starttime uint64
}

type Queue struct {
	Packets           []*Packet
	key               uint64
	virstart          uint64
	RequestsExecuting []*Packet
	// lastvirfinish     uint64
}

func (q *Queue) String(virstart uint64) string {
	var b strings.Builder
	fmt.Fprintf(&b, "queue: %d, %d, %d, %d\n", q.key, virstart, q.lastvirfinish(virstart), len(q.Packets))
	// fmt.Fprintf(&b, "queue: %d, %d, %d, %d\n", q.key, q.virstart, q.lastvirfinish(), len(q.Packets))
	fmt.Fprintf(&b, "|")
	for i, p := range q.Packets {
		fmt.Fprintf(&b, "%d,%d|", p.starttime, p.virfinish(i, virstart))
	}
	fmt.Fprintf(&b, "\n")
	return b.String()
}

func (q *Queue) enqueue(packet *Packet) {
	q.Packets = append(q.Packets, packet)
}

func (q *Queue) lastvirfinish(virstart uint64) uint64 {
	// While the queue is empty and has no requests executing
	// the value of its virtual start time variable is ignored and its last
	// virtual finish time is considered to be in the virtual past
	// ?
	if len(q.Packets) == 0 && len(q.RequestsExecuting) == 0 {
		return uint64(0)
	}

	// While the queue is empty and has a request executing: the last virtual
	// finish time is the queue’s virtual start time.
	if len(q.Packets) == 0 && len(q.RequestsExecuting) > 0 {
		return virstart
		// return q.virstart
	}

	// While the queue is non-empty:
	// the last virtual finish time of the queue is
	// the virtual finish time of the last request in the queue.
	// J * G + (virtual start time)
	// J is len(q.Packets) for the last request in the queue

	// last := len(q.Packets) - 1
	// fmt.Println(last)
	return q.Packets[0].virfinish(0, virstart)

	// last := len(q.Packets) - 1
	// fmt.Println(last)
	// return q.Packets[last].virfinish(last)
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

func (p *Packet) virfinish(J int, virstart uint64) uint64 {
	// The virtual finish time of request number J in the queue
	// (counting from J=1 for the head) is J * G + (virtual start time).

	J += 1 // counting from J=1 for the head
	return uint64(J*G) + virstart
	// return uint64(J*G) + p.queue.virstart
}

func (p *Packet) finishRequest(virstart uint64) {
	// S=servicetime
	// S := time.Now().Sub(p.starttime)
	S := NowAsUnixMilli() - p.starttime

	// TODO(aaron-prindle) verify time units are correct
	// When a request finishes being served, and the actual service time was S,
	// the queue’s virtual start time is decremented by G - S.
	virstart -= G - S
	// p.queue.virstart -= G - S
}
