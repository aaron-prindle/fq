package fq

import (
	"fmt"
	"math"
	"strings"
)

// TODO(aaron-prindle) currently testing with one concurrent request
const C = 1 // const C = 300

// TODO(aaron-prindle) currently service time "G" is not implemented entirely
const G = 100000 //   100000 nanoseconds = 100 milliseconds || const G = 60000000 nanoseconds = 1 minute

type Packet struct {
	// request   http.Request
	item interface{}
	// virfinish float64
	size  uint64
	queue *Queue
	//
	key       uint64
	seq       uint64
	starttime float64
}

type Queue struct {
	Packets           []*Packet
	key               uint64
	virstart          float64
	RequestsExecuting []*Packet
}

func (q *Queue) String() string {

	var b strings.Builder

	fmt.Fprintf(&b, "queue: key: %d, virstart: %f, lastvirfinish: %f, len: %d\n", q.key, q.virstart, q.lastvirfinish(), len(q.Packets))
	fmt.Fprintf(&b, "|")
	for i, p := range q.Packets {
		fmt.Fprintf(&b, "packet %d: starttime: %f, virfinish: %f|", i, p.starttime, p.virfinish(i))

	}
	fmt.Fprintf(&b, "\n")
	return b.String()
}

func (q *Queue) enqueue(packet *Packet) {
	q.Packets = append(q.Packets, packet)
}

func (q *Queue) lastvirfinish() float64 {

	// While the queue is empty and has no requests executing
	// the value of its virtual start time variable is ignored and its last
	// virtual finish time is considered to be in the virtual past
	if len(q.Packets) == 0 && len(q.RequestsExecuting) == 0 {
		return float64(0)
	}

	// While the queue is empty and has a request executing: the last virtual
	// finish time is the queue’s virtual start time.
	if len(q.Packets) == 0 && len(q.RequestsExecuting) > 0 {
		return q.virstart
	}

	// While the queue is non-empty:
	// the last virtual finish time of the queue is
	// the virtual finish time of the last request in the queue.
	last := len(q.Packets) - 1
	// fmt.Println(last)
	return q.Packets[last].virfinish(last)
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

func (p *Packet) virfinish(J int) float64 {
	// The virtual finish time of request number J in the queue
	// (counting from J=1 for the head) is J * G + (virtual start time).

	J++ // counting from J=1 for the head
	// if J*G overflows
	//    throw an error?
	jg := float64(J * G)
	if math.IsInf(jg, 1) {
		panic("float overflowed")
	}
	return jg + p.queue.virstart

}

func (p *Packet) finishRequest(q *FQScheduler) {
	q.synctime()
	S := q.NowAsUnixNano() - p.starttime

	// When a request finishes being served, and the actual service time was S,
	// the queue’s virtual start time is decremented by G - S.
	p.queue.virstart -= G - S
}
