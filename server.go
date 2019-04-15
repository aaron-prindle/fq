package fq

import (
	"log"
	"math/rand"
	"net/http"
	"runtime"
	"strconv"
	"time"
)

func echoHandler(w http.ResponseWriter, r *http.Request) {
	if sleepTimeMillis, err := strconv.Atoi(r.Header.Get("sleep")); err == nil {
		sleepTime := time.Millisecond * time.Duration(float64(sleepTimeMillis)*(1+rand.Float64()))
		time.Sleep(sleepTime)
	}
	w.Write([]byte("loktar ogar"))
}

func main() {
	runtime.GOMAXPROCS(2)
	FQServer(echoHandler)
}

func FQServer(actualHandler http.HandlerFunc) {
	http.HandleFunc("/", newHandler(initQueues(100, 1), actualHandler))
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func newHandler(allQueues []*Queue, delegateHandler http.HandlerFunc) http.HandlerFunc {
	if len(allQueues) == 0 {
		return delegateHandler
	}
	return func(w http.ResponseWriter, r *http.Request) {
		// w.WriteHeader(http.StatusConflict)
		// fmt.Println("limitted!!")
		// w.Write([]byte("limitted"))
		// panic("not matching buckets")
		delegateHandler(w, r)
	}
}
