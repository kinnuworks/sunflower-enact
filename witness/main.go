// Witness sends steady traffic to one or more copies of an app and records, for every
// request, which node answered and whether it succeeded. It is the measuring instrument
// behind the race screen and behind every number in the README.
//
// It never changes anything in the cluster: it only calls the apps and, when given
// Kubernetes access, reads nodes, policies, deployments and events.
package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

//go:embed web
var webFS embed.FS

// Sample is one request and its outcome.
type Sample struct {
	Time      time.Time `json:"t"`
	Target    string    `json:"target"`
	Node      string    `json:"node,omitempty"`
	Pod       string    `json:"pod,omitempty"`
	Status    int       `json:"status"`
	LatencyMs float64   `json:"ms"`
	OK        bool      `json:"ok"`
	Err       string    `json:"err,omitempty"`
}

// Totals summarises everything seen for one target since start (or the last reset).
type Totals struct {
	Target   string         `json:"target"`
	Requests int            `json:"requests"`
	Failed   int            `json:"failed"`
	Slow     int            `json:"slow"`
	ByNode   map[string]int `json:"byNode"`
	LastNode string         `json:"lastNode"`
	P50Ms    float64        `json:"p50Ms"`
	P95Ms    float64        `json:"p95Ms"`
}

type target struct {
	name string
	url  string
}

type hub struct {
	mu        sync.Mutex
	totals    map[string]*Totals
	latencies map[string][]float64
	recent    []Sample
	subs      map[chan []byte]struct{}
	slowMs    float64
	out       io.Writer
}

func newHub(slowMs float64, out io.Writer) *hub {
	return &hub{
		totals:    map[string]*Totals{},
		latencies: map[string][]float64{},
		subs:      map[chan []byte]struct{}{},
		slowMs:    slowMs,
		out:       out,
	}
}

const recentKept = 600

func (h *hub) record(s Sample) {
	line, _ := json.Marshal(s)
	h.mu.Lock()
	t := h.totals[s.Target]
	if t == nil {
		t = &Totals{Target: s.Target, ByNode: map[string]int{}}
		h.totals[s.Target] = t
	}
	t.Requests++
	if !s.OK {
		t.Failed++
	} else {
		if s.LatencyMs > h.slowMs {
			t.Slow++
		}
		t.ByNode[s.Node]++
		t.LastNode = s.Node
		h.latencies[s.Target] = append(h.latencies[s.Target], s.LatencyMs)
	}
	h.recent = append(h.recent, s)
	if len(h.recent) > recentKept {
		h.recent = h.recent[len(h.recent)-recentKept:]
	}
	if h.out != nil {
		h.out.Write(append(line, '\n'))
	}
	for ch := range h.subs {
		select {
		case ch <- line:
		default: // a slow viewer drops samples rather than stalling the measurement
		}
	}
	h.mu.Unlock()
}

func (h *hub) snapshot() (totals []Totals, recent []Sample) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for name, t := range h.totals {
		c := *t
		c.ByNode = map[string]int{}
		for k, v := range t.ByNode {
			c.ByNode[k] = v
		}
		c.P50Ms, c.P95Ms = percentiles(h.latencies[name])
		totals = append(totals, c)
	}
	recent = append(recent, h.recent...)
	return
}

func (h *hub) reset() {
	h.mu.Lock()
	h.totals = map[string]*Totals{}
	h.latencies = map[string][]float64{}
	h.recent = nil
	h.mu.Unlock()
}

func (h *hub) subscribe() chan []byte {
	ch := make(chan []byte, 256)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *hub) unsubscribe(ch chan []byte) {
	h.mu.Lock()
	delete(h.subs, ch)
	h.mu.Unlock()
}

// drive sends requests to one target at a fixed rate. The rate does not slow down when the
// target does (an open-loop generator), so a stalled app shows up as failures and latency
// rather than as fewer requests.
func drive(ctx context.Context, t target, perSecond float64, timeout time.Duration, h *hub) {
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 4,
			IdleConnTimeout:     30 * time.Second,
		},
	}
	tick := time.NewTicker(time.Duration(float64(time.Second) / perSecond))
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			go func() { h.record(probe(ctx, client, t)) }()
		}
	}
}

func probe(ctx context.Context, client *http.Client, t target) Sample {
	s := Sample{Time: time.Now().UTC(), Target: t.name}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.url+"/route", strings.NewReader("{}"))
	if err != nil {
		s.Err = err.Error()
		return s
	}
	req.Header.Set("Content-Type", "application/json")
	start := time.Now()
	resp, err := client.Do(req)
	s.LatencyMs = float64(time.Since(start).Microseconds()) / 1000
	if err != nil {
		s.Err = shortErr(err)
		return s
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	s.Status = resp.StatusCode
	s.Node = resp.Header.Get("X-Served-By-Node")
	s.Pod = resp.Header.Get("X-Served-By-Pod")
	s.OK = resp.StatusCode >= 200 && resp.StatusCode < 300
	return s
}

func shortErr(err error) string {
	msg := err.Error()
	if i := strings.LastIndex(msg, ": "); i >= 0 {
		msg = msg[i+2:]
	}
	return msg
}

func percentiles(v []float64) (p50, p95 float64) {
	if len(v) == 0 {
		return 0, 0
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	return s[len(s)*50/100], s[min(len(s)-1, len(s)*95/100)]
}

func parseTargets(spec string) ([]target, error) {
	var out []target
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, url, ok := strings.Cut(part, "=")
		if !ok || name == "" || url == "" {
			return nil, fmt.Errorf("target %q must look like name=http://host:port", part)
		}
		out = append(out, target{name: name, url: strings.TrimRight(url, "/")})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no targets given")
	}
	return out, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	targetsFlag := flag.String("targets", env("WITNESS_TARGETS", ""), "comma-separated name=url pairs")
	rate := flag.Float64("rate", 5, "requests per second, per target")
	timeout := flag.Duration("timeout", 2*time.Second, "per-request timeout")
	slowMs := flag.Float64("slow-ms", 50, "latency above which a request counts as slow (the policy's latency limit)")
	listen := flag.String("listen", env("WITNESS_LISTEN", ":8090"), "address for the screen and API")
	logPath := flag.String("log", env("WITNESS_LOG", ""), "append every sample to this JSONL file")
	duration := flag.Duration("duration", 0, "stop after this long and print a summary (0 = run until stopped)")
	flag.Parse()

	targets, err := parseTargets(*targetsFlag)
	if err != nil {
		log.Fatalf("witness: %v", err)
	}

	var out io.Writer
	if *logPath != "" {
		f, err := os.OpenFile(*logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			log.Fatalf("witness: %v", err)
		}
		defer f.Close()
		out = f
	}

	h := newHub(*slowMs, out)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if *duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *duration)
		defer cancel()
	}

	cluster := newClusterWatcher()
	go cluster.run(ctx)

	for _, t := range targets {
		go drive(ctx, t, *rate, *timeout, h)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/state", func(w http.ResponseWriter, r *http.Request) {
		totals, recent := h.snapshot()
		writeJSON(w, map[string]any{"totals": totals, "recent": recent, "cluster": cluster.snapshot(), "slowMs": *slowMs})
	})
	mux.HandleFunc("POST /api/reset", func(w http.ResponseWriter, r *http.Request) {
		h.reset()
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /api/stream", func(w http.ResponseWriter, r *http.Request) {
		fl, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		ch := h.subscribe()
		defer h.unsubscribe(ch)
		for {
			select {
			case <-r.Context().Done():
				return
			case line := <-ch:
				fmt.Fprintf(w, "data: %s\n\n", line)
				fl.Flush()
			}
		}
	})
	web, _ := fs.Sub(webFS, "web")
	mux.Handle("/", http.FileServer(http.FS(web)))

	srv := &http.Server{Addr: *listen, Handler: mux}
	go func() {
		log.Printf("witness: %d target(s) at %.1f req/s each, screen on %s", len(targets), *rate, *listen)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("witness: %v", err)
		}
	}()

	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	srv.Shutdown(shutdown)

	totals, _ := h.snapshot()
	summary, _ := json.MarshalIndent(totals, "", "  ")
	fmt.Println(string(summary))
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
