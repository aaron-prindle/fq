package fq

import (
	"fmt"
	"math"
	"sort"
)

func max(a, b float64) float64 {
	if a >= b {
		return a
	}
	return b
}

func min(a, b float64) float64 {
	if a <= b {
		return a
	}
	return b
}

func (q *FQScheduler) printsortedqueue() {
	sorted := append(q.queues[:0:0], q.queues...)
	sort.Slice(sorted, func(i, j int) bool {
		var x, y float64
		if len(sorted[i].Packets) == 0 {
			x = math.Inf(-1)
		} else {
			x = sorted[i].Packets[0].virfinish(0)
		}
		if len(sorted[j].Packets) == 0 {
			y = math.Inf(-1)
		} else {
			y = sorted[j].Packets[0].virfinish(0)

		}
		return x < y
	})
	for i, queue := range sorted {
		fmt.Println("===sortedqueues===")
		if len(queue.Packets) != 0 {
			fmt.Printf("queue.idx: %d\n", i)
			fmt.Printf("queue.Packets[0].virfinish(0): %f\n", queue.Packets[0].virfinish(0))
			fmt.Printf("%s======\n", queue.String())
		}
		fmt.Println("===sortedqueuesDONE===")
	}
}

func printdequeue(queue *Queue, idx int) {
	fmt.Println("***dequeue***")
	fmt.Printf("dequeue: %d\n", idx)
	fmt.Printf("%s\n", queue.String())
	fmt.Println("***")
}
