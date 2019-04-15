package fq

import "time"

type DequeueRunner struct {
	fqs FQScheduler
}

func (r *DequeueRunner) Run() {
	go func() {
		for {
			time.Sleep(100 * time.Millisecond)
			r.fqs.Dequeue()
		}
	}()
}
