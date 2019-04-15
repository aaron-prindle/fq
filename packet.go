package fq

import "time"

type Packet struct {
	// request   http.Request
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

func (p *Packet) updateTimeQueued() {
	p.queue.lock.Lock()
	defer p.queue.lock.Unlock()

	if len(p.queue.Packets) == 0 && p.queue.requestsexecuting == 0 {
		// p.queue.virStart is ignored
		// queues.lastvirfinish is in the virtualpast
		p.queue.virstart = uint64(time.Now().UnixNano())
	}
	if len(p.queue.Packets) == 0 && p.queue.requestsexecuting > 0 {
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
	p.queue.requestsexecuting--

	p.queue.lock.Lock()
	defer p.queue.lock.Unlock()

	p.endtime = uint64(time.Now().UnixNano())
	p.actservicetime = p.starttime - p.endtime

	S := p.actservicetime
	G := p.estservicetime
	p.queue.virstart -= (G - S)
}
