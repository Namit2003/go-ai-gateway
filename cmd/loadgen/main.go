package main

import (
	"flag"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"time"
)

type result struct {
	latencies []time.Duration
	statuses  map[int]int
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	return sorted[int(p*float64(len(sorted)-1))]
}

func worker(client *http.Client, url, key, body string, deadline time.Time) result {
	res := result{statuses: make(map[int]int)}
	for time.Now().Before(deadline) {
		req, _ := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+key)
		start := time.Now()
		status := 0
		resp, err := client.Do(req)
		if err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			status = resp.StatusCode
		}
		res.latencies = append(res.latencies, time.Since(start))
		res.statuses[status]++
	}
	return res
}

func run(url, key, body string, concurrency int, duration time.Duration) result {
	client := &http.Client{Transport: &http.Transport{MaxIdleConnsPerHost: concurrency}}
	deadline := time.Now().Add(duration)
	results := make([]result, concurrency)
	var wg sync.WaitGroup
	for i := range concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = worker(client, url, key, body, deadline)
		}()
	}
	wg.Wait()

	total := result{statuses: make(map[int]int)}
	for _, r := range results {
		total.latencies = append(total.latencies, r.latencies...)
		for status, n := range r.statuses {
			total.statuses[status] += n
		}
	}
	return total
}

func main() {
	url := flag.String("url", "http://localhost:8080/v1/chat/completions", "gateway chat completions URL")
	key := flag.String("key", os.Getenv("LOADGEN_API_KEY"), "gateway API key (default: $LOADGEN_API_KEY)")
	model := flag.String("model", "auto", "model to request")
	concurrency := flag.Int("c", 50, "number of concurrent workers")
	duration := flag.Duration("d", 10*time.Second, "how long to run")
	flag.Parse()

	body := fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":"hi"}]}`, *model)
	start := time.Now()
	res := run(*url, *key, body, *concurrency, *duration)
	elapsed := time.Since(start)

	slices.Sort(res.latencies)
	fmt.Printf("requests:   %d (%.0f/s)\n", len(res.latencies), float64(len(res.latencies))/elapsed.Seconds())
	for _, status := range slices.Sorted(maps.Keys(res.statuses)) {
		name := fmt.Sprint(status)
		if status == 0 {
			name = "error"
		}
		fmt.Printf("status %-5s %d\n", name, res.statuses[status])
	}
	fmt.Printf("latency:    p50=%v p95=%v p99=%v max=%v\n",
		percentile(res.latencies, 0.50), percentile(res.latencies, 0.95), percentile(res.latencies, 0.99), percentile(res.latencies, 1))
}
