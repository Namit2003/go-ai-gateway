package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"time"
)

func newHandler(errorRate, dropRate float64, delay time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delay)
		if rand.Float64() < dropRate {
			panic(http.ErrAbortHandler)
		}
		if rand.Float64() < errorRate {
			http.Error(w, "injected failure", http.StatusServiceUnavailable)
			return
		}

		var req struct {
			Model  string `json:"model"`
			Stream bool   `json:"stream"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		if req.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			for _, part := range []string{"hello", " world"} {
				fmt.Fprintf(w, "data: {\"id\":\"x\",\"object\":\"chat.completion.chunk\",\"model\":%q,\"choices\":[{\"index\":0,\"delta\":{\"content\":%q}}]}\n\n", req.Model, part)
				w.(http.Flusher).Flush()
			}
			fmt.Fprint(w, "data: [DONE]\n\n")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":"x","object":"chat.completion","model":%q,"choices":[{"index":0,"message":{"role":"assistant","content":"hello world"},"finish_reason":"stop"}]}`, req.Model)
	})
}

func main() {
	addr := flag.String("addr", ":9001", "listen address")
	errorRate := flag.Float64("error-rate", 0, "fraction of requests answered with 503 (0 to 1)")
	dropRate := flag.Float64("drop-rate", 0, "fraction of requests whose connection is closed without a response (0 to 1)")
	delay := flag.Duration("delay", 0, "wait this long before answering every request")
	flag.Parse()

	log.Printf("fake upstream listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, newHandler(*errorRate, *dropRate, *delay)))
}
