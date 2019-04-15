package fq

import (
	"sync"
	"time"
)

// const (
// 	scaledOne uint64 = 1 << 16
// )

type Packetable interface {
	getitem() interface{}
	getvirfinish() uint64
	getsize() uint64
	getqueue() uint64
	getstarttime() uint64
	getendtime() uint64
	getkey() uint64
	getseq() uint64
	getestservicetime() uint64
	getactservicetime() uint64
}

type Packet struct {
	// request   http.Request
	item      interface{}
	virfinish uint64
	size      uint64
	queue     *Queue
	starttime uint64
	endtime   uint64
	//
	key uint64
	seq uint64
	//
	estservicetime uint64
	actservicetime uint64
}

type Queue struct {
	lock          sync.Mutex
	Packets       []*Packet
	key           uint64
	lastvirfinish uint64
	//
	requestsexecuting bool
	virstart          uint64
	// instead composed from 3 values
	// queuelen len(Packets)
	// packet requestPosition
	// virstart?
}

func (q *Queue) enqueue(packet *Packet) {
	// fmt.Printf("enqueue - queue[%d]: %d packets\n", q.key, len(q.Packets))
	q.Packets = append(q.Packets, packet)
}

func (q *Queue) dequeue() (*Packet, bool) {
	// fmt.Printf("dequeue - queue[%d]: %d packets\n", q.key, len(q.Packets))
	if len(q.Packets) == 0 {
		return nil, false
	}
	packet := q.Packets[0]
	q.Packets = q.Packets[1:]
	return packet, true
}

func (p *Packet) updateTimeQueued() {
	p.queue.lock.Lock()
	defer p.queue.lock.Unlock()

	if len(p.queue.Packets) == 0 && !p.queue.requestsexecuting {
		// p.queue.virStart is ignored
		// queues.lastvirfinish is in the virtualpast
		p.queue.virstart = uint64(time.Now().UnixNano())
	}
	if len(p.queue.Packets) == 0 && p.queue.requestsexecuting {
		p.queue.virstart = p.queue.lastvirfinish
	}

	p.virfinish = (uint64(len(p.queue.Packets)+1))*p.estservicetime + p.queue.virstart

	if len(p.queue.Packets) > 0 {
		// last virtual finish time of the queue is the virtual finish
		// time of the last request in the queue
		p.queue.lastvirfinish = p.virfinish // this pkt is last pkt
	}
}

func (p *Packet) updateTimeDequeued() {
	p.queue.lock.Lock()
	defer p.queue.lock.Unlock()

	p.queue.virstart += p.estservicetime
}

// used when Packet's request is filled to store actual service time
func (p *Packet) updateTimeFinished() {
	p.queue.lock.Lock()
	defer p.queue.lock.Unlock()

	p.endtime = uint64(time.Now().UnixNano())
	p.actservicetime = p.starttime - p.endtime

	S := p.actservicetime
	G := p.estservicetime
	p.queue.virstart -= (G - S)
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
