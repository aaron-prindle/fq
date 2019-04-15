package fq

import (
	"math"
	"sync"
	"time"
)

const (
	scaledOne uint64 = 1 << 16
)

// mods for our algo
// use min heap vs selectQueue
// 1) we are dispatching requests to be served rather than packets to be transmitted
// 2) the actual service time (i.e., duration) is not known until a request is done being served
//
// 1 & 2 can be handled by using duration time instead of size

type Packet struct {
	// request   http.Request
	item      interface{}
	virfinish uint64
	size      uint64
	queue     *Queue
	//
	key uint64
	seq uint64
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

type Queue struct {
	Packets       []*Packet
	key           uint64
	lastvirfinish uint64
}

func (q *Queue) enqueue(packet *Packet) {
	q.Packets = append(q.Packets, packet)
}

func (q *Queue) dequeue() (*Packet, bool) {
	if len(q.Packets) == 0 {
		return nil, false
	}
	packet := q.Packets[0]
	q.Packets = q.Packets[1:]
	return packet, true
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
	// order of updatetime and enqueue changes things?
	// q.updateTime(packet, queue)
	// queue.enqueue(packet)
	queue.enqueue(packet)
	q.updateTime(packet, queue)

}

func max(a, b uint64) uint64 {
	if a >= b {
		return a
	}
	return b
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
	packet, ok := queue.dequeue()
	return packet, ok
}

func (q *fqscheduler) selectQueue() *Queue {
	minvirfinish := uint64(math.MaxUint64)
	minqueue := q.queues[0]
	for _, queue := range q.queues {
		if len(queue.Packets) != 0 && queue.Packets[0].virfinish < minvirfinish {
			minvirfinish = queue.Packets[0].virfinish
			minqueue = queue
		}
	}
	return minqueue
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

// func echoHandler(w http.ResponseWriter, r *http.Request) {
// 	if sleepTimeMillis, err := strconv.Atoi(r.Header.Get("sleep")); err == nil {
// 		sleepTime := time.Millisecond * time.Duration(float64(sleepTimeMillis)*(1+rand.Float64()))
// 		time.Sleep(sleepTime)
// 	}
// 	w.Write([]byte("loktar ogar"))
// }

// func main() {
// 	runtime.GOMAXPROCS(2)
// 	FQServer(echoHandler)
// }

// func FQServer(actualHandler http.HandlerFunc) {
// 	http.HandleFunc("/", newHandler(initQueues(100, 1), actualHandler))
// 	log.Fatal(http.ListenAndServe(":8080", nil))
// }

// func newHandler(allQueues []*Queue, delegateHandler http.HandlerFunc) http.HandlerFunc {
// 	if len(allQueues) == 0 {
// 		return delegateHandler
// 	}
// 	return func(w http.ResponseWriter, r *http.Request) {
// 		// w.WriteHeader(http.StatusConflict)
// 		// fmt.Println("limitted!!")
// 		// w.Write([]byte("limitted"))
// 		// panic("not matching buckets")
// 		delegateHandler(w, r)
// 	}
// }
