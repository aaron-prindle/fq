package fq

// const (
// 	scaledOne uint64 = 1 << 16
// )

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
