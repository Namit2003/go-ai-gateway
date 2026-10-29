package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
)

func main() {
	addr := flag.String("addr", ":9001", "listen address")
	flag.Parse()

	http.HandleFunc("POST /", func(w http.ResponseWriter, r *http.Request) {
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

	log.Printf("fake upstream listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
