package fq

import (
	"fmt"
	"math"
	"sync"
	"time"
)

type FQScheduler struct {
	lock         sync.Mutex
	queues       []*Queue
	vt           uint64
	C            uint64
	G            uint64
	lastrealtime uint64
	robinidx     int
}

// TODO(aaron-prindle) add concurrency enforcement - 'C'

func (q *FQScheduler) chooseQueue(packet *Packet) *Queue {
	for _, queue := range q.queues {
		if packet.key == queue.key {
			return queue
		}
	}
	panic("no matching queue for packet")
}

func newFQScheduler(queues []*Queue) *FQScheduler {
	fq := &FQScheduler{
		lock:   sync.Mutex{},
		queues: queues,
		// R(t) = (server start time) + (1 ns) * (number of rounds since server start).
		vt: 0,
		// vt: now,
	}
	return fq
}

// TODO(aaron-prindle) verify that the time units are correct/matching
func NowAsUnixMilli() uint64 {
	return uint64(time.Now().UnixNano() / 1e6)
}

func (q *FQScheduler) Enqueue(packet *Packet) {
	q.lock.Lock()
	defer q.lock.Unlock()

	fmt.Printf("enqueue: %d\n", packet.key)

	queue := q.chooseQueue(packet)
	packet.starttime = NowAsUnixMilli()
	packet.queue = queue

	// TODO(aaron-prindle) verify the order here
	queue.enqueue(packet)
	q.updateTime(packet, queue)
}

func (q *FQScheduler) now() uint64 {
	return q.vt
}

func (q *FQScheduler) synctime() {
	// anything that looks at now updates the time?
	// updatetime?
	now := NowAsUnixMilli()
	timesincelast := q.lastrealtime - now
	q.vt += timesincelast * q.getvirtualtimeratio()
	q.lastrealtime = now
	// return q.vt
}

func (q *FQScheduler) getvirtualtimeratio() uint64 {
	NEQ := 0
	reqs := 0

	for _, queue := range q.queues {
		reqs += len(queue.RequestsExecuting)
		reqs += len(queue.Packets)
		if len(queue.Packets) > 0 || len(queue.RequestsExecuting) > 0 {
			NEQ++
		}
	}

	// no active flows
	if NEQ == 0 {
		return 0
	}

	// q.vt can be 0 if NEQ >> min(sum[over q] reqs(q,t), C)
	// ceil used to guarantee q.vt advances each step
	return uint64(math.Ceil(float64(min(uint64(reqs), uint64(C))) / float64(uint64(NEQ))))
}

func (q *FQScheduler) updateTime(packet *Packet, queue *Queue) {
	// When a request arrives to an empty queue with no requests executing
	// (enqueue has just happened prior)
	if len(queue.Packets) == 1 && len(queue.RequestsExecuting) == 0 {
		// the queue’s virtual start time is set to now().
		queue.virstart = q.now()
	}
}

func (q *FQScheduler) Dequeue() (*Packet, bool) {
	q.lock.Lock()
	defer q.lock.Unlock()

	queue := q.selectQueue()
	if queue == nil {
		return nil, false
	}

	fmt.Println("***dequeue***")
	fmt.Printf("dequeue: %d\n", queue.key)

	fmt.Printf("%s****\n", queue.String())

	packet, ok := queue.dequeue()

	if ok {
		// When a request is dequeued for service the queue’s virtual start
		// time is advanced by G
		queue.virstart += G
	}
	// queue.RequestsExecuting = append(queue.RequestsExecuting, packet)
	return packet, ok
}

func (q *FQScheduler) getroundrobinqueue() (*Queue, int) {
	queue := q.queues[q.robinidx]
	curidx := q.robinidx

	q.robinidx++
	if q.robinidx >= len(q.queues) {
		q.robinidx = 0
	}
	return queue, curidx
}

func (q *FQScheduler) selectQueue() *Queue {
	minvirfinish := uint64(math.MaxUint64)
	var minqueue *Queue
	fmt.Println("===selectQueue===")
	for range q.queues {
		queue, idx := q.getroundrobinqueue()
		// fmt.Printf("%s======\n", queue.String())
		// if len(queue.Packets) != 0 {
		// 	fmt.Printf("queue.Packets[0].virfinish(0): %d\n", queue.Packets[0].virfinish(0))
		// }
		if len(queue.Packets) != 0 && queue.Packets[0].virfinish(0) < minvirfinish {
			fmt.Printf("queue.key: %d, queue.Packets[0].virfinish(0): %d\n", queue.key, queue.Packets[0].virfinish(0))
			fmt.Printf("%s======\n", queue.String())
			minvirfinish = queue.Packets[0].virfinish(0)
			minqueue = queue
			q.robinidx = idx
		}
	}

	return minqueue
}
