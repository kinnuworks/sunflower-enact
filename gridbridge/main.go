// Gridbridge replays real, published grid data into the cluster.
//
// For each half-hour of data it does two things: it writes the carbon feed file GreenCharge
// reads (carbon intensity per district), and it sets each node's enact.eu/green-ratio label
// to the renewable share of that node's grid zone, through the ENACT policy operator's own
// Label API. One data feed therefore drives both the app's routing and where the app runs.
//
// It can also inject a clearly labelled drill: a temporary override of one node's ratio.
package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

//go:embed dataset/gb-regional.json
var datasetJSON []byte

type dataset struct {
	Source      string            `json:"source"`
	Licence     string            `json:"licence"`
	StepMinutes int               `json:"stepMinutes"`
	Regions     map[string]string `json:"regions"`
	Periods     []struct {
		T string                `json:"t"`
		R map[string][2]float64 `json:"r"` // region id -> [intensity gCO2/kWh, renewable share]
	} `json:"periods"`
}

type drill struct {
	Node       string    `json:"node"`
	GreenRatio float64   `json:"greenRatio"`
	Until      time.Time `json:"until"`
}

type nodeView struct {
	Node       string    `json:"node"`
	Zone       string    `json:"zone"`
	GreenRatio float64   `json:"greenRatio"`
	Intensity  float64   `json:"intensity"`
	Drill      bool      `json:"drill"`
	History    []float64 `json:"history"`
}

type bridge struct {
	data      dataset
	nodes     map[string]string // node -> region id
	districts map[string]string // district -> region id
	feedPath  string
	labelAPI  string
	step      time.Duration
	start     int
	length    int

	mu       sync.Mutex
	index    int
	playing  bool
	drill    *drill
	lastSent map[string]string
	history  map[string][]float64
	lastErr  string
}

func (b *bridge) period() int { return b.start + b.index }

// ratioFor returns the ratio to publish for a node, and whether a drill is overriding the data.
func (b *bridge) ratioFor(node string, now time.Time) (float64, bool) {
	if b.drill != nil && b.drill.Node == node && now.Before(b.drill.Until) {
		return b.drill.GreenRatio, true
	}
	return b.data.Periods[b.period()].R[b.nodes[node]][1], false
}

// publish writes the feed file and updates any node label whose value changed.
func (b *bridge) publish(now time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.drill != nil && !now.Before(b.drill.Until) {
		b.drill = nil
	}
	p := b.data.Periods[b.period()]

	feed := map[string]float64{}
	for district, region := range b.districts {
		feed[district] = p.R[region][0]
	}
	if err := writeAtomically(b.feedPath, feed); err != nil {
		b.lastErr = err.Error()
	}

	for node := range b.nodes {
		ratio, _ := b.ratioFor(node, now)
		value := fmt.Sprintf("%.2f", ratio)
		if b.lastSent[node] == value {
			continue // every node change makes the operator re-rank, so only real changes are sent
		}
		if err := b.label(node, value); err != nil {
			b.lastErr = err.Error()
			continue
		}
		b.lastSent[node] = value
		b.lastErr = ""
	}
}

func (b *bridge) recordHistory(now time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for node := range b.nodes {
		ratio, _ := b.ratioFor(node, now)
		h := append(b.history[node], math.Round(ratio*100)/100)
		if len(h) > 120 {
			h = h[len(h)-120:]
		}
		b.history[node] = h
	}
}

func (b *bridge) label(node, value string) error {
	body, _ := json.Marshal(map[string]any{"add": map[string]string{"enact.eu/green-ratio": value}})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, b.labelAPI+"/api/v1/nodes/"+node+"/labels", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("label API answered %s for %s", resp.Status, node)
	}
	return nil
}

// writeAtomically replaces the feed file in one step, so a reader never sees half a file.
func writeAtomically(path string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	tmp := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (b *bridge) run(ctx context.Context) {
	b.publish(time.Now())
	b.recordHistory(time.Now())
	step := time.NewTicker(b.step)
	fine := time.NewTicker(time.Second)
	defer step.Stop()
	defer fine.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-step.C:
			b.mu.Lock()
			if b.playing && b.index < b.length-1 {
				b.index++
			} else if b.playing {
				b.playing = false
			}
			b.mu.Unlock()
			b.publish(time.Now())
			b.recordHistory(time.Now())
		case <-fine.C:
			// Picks up a drill starting or ending between two data steps.
			b.publish(time.Now())
		}
	}
}

func (b *bridge) state() map[string]any {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	p := b.data.Periods[b.period()]
	var nodes []nodeView
	for node, region := range b.nodes {
		ratio, drilled := b.ratioFor(node, now)
		nodes = append(nodes, nodeView{Node: node, Zone: b.data.Regions[region], GreenRatio: ratio,
			Intensity: p.R[region][0], Drill: drilled, History: b.history[node]})
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Node < nodes[j].Node })
	districts := map[string]any{}
	for d, region := range b.districts {
		districts[d] = map[string]any{"zone": b.data.Regions[region], "intensity": p.R[region][0]}
	}
	return map[string]any{
		"dataTime": p.T, "index": b.index, "length": b.length, "playing": b.playing,
		"stepSeconds": b.step.Seconds(), "stepMinutes": b.data.StepMinutes,
		"nodes": nodes, "districts": districts, "drill": b.drill,
		"source": b.data.Source, "licence": b.data.Licence, "error": b.lastErr,
	}
}

func parsePairs(spec string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(spec, ",") {
		if k, v, ok := strings.Cut(strings.TrimSpace(part), "="); ok {
			out[k] = v
		}
	}
	return out
}

func main() {
	feedDir := flag.String("feed-dir", "/grid", "folder holding the carbon feed file")
	feedFile := flag.String("feed-file", "carbon.json", "name of the carbon feed file")
	labelAPI := flag.String("label-api", "http://applpm-api.enact.svc.cluster.local:8080", "ENACT policy operator HTTP API")
	nodes := flag.String("nodes", "enact-dev-worker=4,enact-dev-worker2=3", "node=region id pairs: each node's grid zone")
	districts := flag.String("districts", "Harbor=4,Riverside=3,Uptown=8,OldTown=7", "district=region id pairs for the carbon feed")
	startAt := flag.String("start", "2026-09-29T12:00Z", "data timestamp to start the replay from")
	periods := flag.Int("periods", 40, "how many half-hours to replay (0 = to the end of the data)")
	step := flag.Duration("step", 4*time.Second, "wall-clock time per half-hour of data")
	autoplay := flag.Bool("autoplay", false, "start replaying immediately")
	listen := flag.String("listen", ":8095", "address for the control API")
	flag.Parse()

	b := &bridge{nodes: parsePairs(*nodes), districts: parsePairs(*districts), labelAPI: strings.TrimRight(*labelAPI, "/"),
		feedPath: filepath.Join(*feedDir, *feedFile), step: *step, playing: *autoplay,
		lastSent: map[string]string{}, history: map[string][]float64{}}
	if err := json.Unmarshal(datasetJSON, &b.data); err != nil {
		log.Fatalf("gridbridge: bad dataset: %v", err)
	}
	b.start = -1
	for i, p := range b.data.Periods {
		if p.T == *startAt {
			b.start = i
		}
	}
	if b.start < 0 {
		log.Fatalf("gridbridge: no data at %s", *startAt)
	}
	b.length = len(b.data.Periods) - b.start
	if *periods > 0 && *periods < b.length {
		b.length = *periods
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go b.run(ctx)

	mux := http.NewServeMux()
	reply := func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(b.state())
	}
	mux.HandleFunc("GET /state", func(w http.ResponseWriter, r *http.Request) { reply(w) })
	set := func(f func()) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			b.mu.Lock()
			f()
			b.mu.Unlock()
			b.publish(time.Now())
			reply(w)
		}
	}
	mux.HandleFunc("POST /play", set(func() { b.playing = true }))
	mux.HandleFunc("POST /pause", set(func() { b.playing = false }))
	mux.HandleFunc("POST /reset", set(func() {
		b.index, b.playing, b.drill = 0, false, nil
		b.history = map[string][]float64{}
	}))
	mux.HandleFunc("POST /drill", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Node       string  `json:"node"`
			GreenRatio float64 `json:"greenRatio"`
			Seconds    float64 `json:"seconds"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || b.nodes[in.Node] == "" ||
			in.GreenRatio < 0 || in.GreenRatio > 1 || in.Seconds <= 0 || in.Seconds > 600 {
			http.Error(w, `expected {"node": one of the configured nodes, "greenRatio": 0..1, "seconds": 1..600}`, http.StatusBadRequest)
			return
		}
		b.mu.Lock()
		b.drill = &drill{Node: in.Node, GreenRatio: in.GreenRatio, Until: time.Now().Add(time.Duration(in.Seconds * float64(time.Second)))}
		b.mu.Unlock()
		b.publish(time.Now())
		reply(w)
	})

	srv := &http.Server{Addr: *listen, Handler: mux}
	go func() {
		<-ctx.Done()
		srv.Close()
	}()
	log.Printf("gridbridge: replaying %d half-hours from %s, %s each, control API on %s", b.length, *startAt, *step, *listen)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("gridbridge: %v", err)
	}
}
