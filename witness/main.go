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
	"net/http/httputil"
	"net/url"
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
	// OutOfPolicySeconds is how long the node serving this target has been below the
	// green-energy minimum its RuntimePolicy declares.
	OutOfPolicySeconds float64  `json:"outOfPolicySeconds"`
	Violating          bool     `json:"violating"`
	GreenRatio         *float64 `json:"greenRatio,omitempty"`
	GreenMin           *float64 `json:"greenMin,omitempty"`
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
	failures  []Sample // every failed request since the last reset, capped
	subs      map[chan []byte]struct{}
	slowMs    float64
	out       io.Writer
	timeline  *timeline
}

func newHub(slowMs float64, out io.Writer) *hub {
	return &hub{
		totals:    map[string]*Totals{},
		latencies: map[string][]float64{},
		subs:      map[chan []byte]struct{}{},
		slowMs:    slowMs,
		out:       out,
		timeline:  newTimeline(),
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
		if len(h.failures) < 500 {
			h.failures = append(h.failures, s)
		}
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
	h.timeline.request(s)
}

func (h *hub) failed() []Sample {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]Sample{}, h.failures...)
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

// account adds dt seconds of out-of-policy time to a target when its serving node is below
// the policy's green minimum. Called once per cluster poll.
func (h *hub) account(name string, ratio, min *float64, dt float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	t := h.totals[name]
	if t == nil {
		return
	}
	t.GreenRatio, t.GreenMin = ratio, min
	t.Violating = ratio != nil && min != nil && *ratio+1e-9 < *min
	if t.Violating {
		t.OutOfPolicySeconds += dt
	}
	h.timeline.below(name, t.Violating, time.Now())
}

// servingNode is the node that answered the target's most recent successful request.
func (h *hub) servingNode(name string) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if t := h.totals[name]; t != nil {
		return t.LastNode
	}
	return ""
}

func (h *hub) reset() {
	h.mu.Lock()
	h.totals = map[string]*Totals{}
	h.latencies = map[string][]float64{}
	h.recent = nil
	h.failures = nil
	h.mu.Unlock()
	h.timeline.reset()
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
	asker := &http.Client{Timeout: 2 * time.Second}
	// Each copy is asked for its own policy check when its node or that node's green share
	// changes, and otherwise every few seconds: often enough to follow the day, rarely enough
	// to add nothing to the load being measured.
	type asked struct {
		node  string
		ratio float64
		at    time.Time
	}
	lastAsked := map[string]asked{}
	cluster.onPoll = func(st ClusterState, dt float64) {
		now := time.Now()
		for _, n := range st.Nodes {
			if n.GreenRatio != nil {
				h.timeline.ratio(n.Name, *n.GreenRatio, now)
			}
		}
		h.timeline.mark(st.GridTime, now)
		for _, p := range st.Policies {
			if p.Name == "greencharge-"+targets[len(targets)-1].name {
				h.timeline.chose(p.ChosenNode, now)
			}
		}
		for _, t := range targets {
			node := h.servingNode(t.name)
			var ratio, min *float64
			for _, n := range st.Nodes {
				if n.Name == node {
					ratio = n.GreenRatio
				}
			}
			for _, p := range st.Policies {
				if p.Name == "greencharge-"+t.name {
					min = p.GreenMin
				}
			}
			h.account(t.name, ratio, min, dt)
			for _, n := range st.Nodes {
				if n.Name == node && n.GreenRatio != nil {
					if p := lastAsked[t.name]; p.node == n.Name && p.ratio == *n.GreenRatio && time.Since(p.at) < 5*time.Second {
						continue
					}
					lastAsked[t.name] = asked{node: n.Name, ratio: *n.GreenRatio, at: time.Now()}
					go func(t target, n NodeView) { // off the poll's path: a slow app must not delay the clock
						if action, reason, ok := advice(ctx, asker, t, n); ok {
							h.timeline.advise(t.name, action, reason, time.Now())
						}
					}(t, n)
				}
			}
		}
	}
	go cluster.run(ctx)

	for _, t := range targets {
		go drive(ctx, t, *rate, *timeout, h)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/state", func(w http.ResponseWriter, r *http.Request) {
		totals, recent := h.snapshot()
		writeJSON(w, map[string]any{"totals": totals, "recent": recent, "cluster": cluster.snapshot(), "slowMs": *slowMs, "failures": h.failed()})
	})
	mux.HandleFunc("GET /api/timeline", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, h.timeline.snapshot())
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
	if grid := os.Getenv("WITNESS_GRID"); grid != "" {
		// The screen's replay controls live on the same origin as the screen itself.
		if u, err := url.Parse(grid); err == nil {
			mux.Handle("/grid/", http.StripPrefix("/grid", httputil.NewSingleHostReverseProxy(u)))
		}
	}
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
