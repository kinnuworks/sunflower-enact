package main

import (
	"math"
	"sync"
	"time"
)

// timeline is the history the race screen draws: every request since the last reset, each
// node's green share over time, and the stretches when each copy sat on a node below its
// policy's green minimum. Times are seconds since the reset.
type timeline struct {
	mu       sync.Mutex
	start    time.Time
	nodes    []string
	nodeIdx  map[string]int
	requests map[string][][2]float64 // target -> [seconds, node index; -1 = failed]
	ratios   map[string][][2]float64 // node -> [seconds, green share], one point per change
	breaking map[string][][2]float64 // target -> [from, to]; to = -1 while still below
	marks    []timeMark              // when the grid replay reached each data timestamp
	choice   [][2]float64            // [seconds, node index]: the node the policy operator names, one point per change
	advised  map[string][]adviceMark // target -> what the app's own policy check said, one entry per change
}

// adviceMark is one answer from a copy's POST /adaptation/recommendation.
type adviceMark struct {
	T      float64 `json:"t"`
	Action string  `json:"action"`
	Reason string  `json:"reason"`
}

type timeMark struct {
	T    float64 `json:"t"`
	Data string  `json:"data"`
}

// requestsKept bounds memory on long runs: about 30 minutes per target at 5 requests a second.
const requestsKept = 9000

func newTimeline() *timeline {
	tl := &timeline{}
	tl.reset()
	return tl
}

func (tl *timeline) reset() {
	tl.mu.Lock()
	tl.start = time.Now()
	tl.nodes, tl.nodeIdx = nil, map[string]int{}
	tl.requests, tl.ratios, tl.breaking = map[string][][2]float64{}, map[string][][2]float64{}, map[string][][2]float64{}
	tl.marks, tl.choice = nil, nil
	tl.advised = map[string][]adviceMark{}
	tl.mu.Unlock()
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

func (tl *timeline) since(t time.Time) float64 { return round2(t.Sub(tl.start).Seconds()) }

// node returns the index used for a node name in the request list. Callers hold tl.mu.
func (tl *timeline) node(name string) int {
	i, ok := tl.nodeIdx[name]
	if !ok {
		i = len(tl.nodes)
		tl.nodes = append(tl.nodes, name)
		tl.nodeIdx[name] = i
	}
	return i
}

func (tl *timeline) request(s Sample) {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	if s.Time.Before(tl.start) {
		return // sent before the reset, answered after it
	}
	n := -1.0
	if s.OK {
		n = float64(tl.node(s.Node))
	}
	r := append(tl.requests[s.Target], [2]float64{tl.since(s.Time), n})
	if len(r) > requestsKept {
		r = r[len(r)-requestsKept:]
	}
	tl.requests[s.Target] = r
}

// ratio records a node's green share, keeping a point only when the value changes.
func (tl *timeline) ratio(node string, share float64, now time.Time) {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	tl.node(node)
	pts := tl.ratios[node]
	if n := len(pts); n > 0 && pts[n-1][1] == share {
		return
	}
	tl.ratios[node] = append(pts, [2]float64{tl.since(now), share})
}

// below opens or closes a "below the green minimum" stretch for a target.
func (tl *timeline) below(target string, violating bool, now time.Time) {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	spans := tl.breaking[target]
	open := len(spans) > 0 && spans[len(spans)-1][1] < 0
	switch {
	case violating && !open:
		tl.breaking[target] = append(spans, [2]float64{tl.since(now), -1})
	case !violating && open:
		spans[len(spans)-1][1] = tl.since(now)
	}
}

func (tl *timeline) mark(data string, now time.Time) {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	if n := len(tl.marks); data == "" || (n > 0 && tl.marks[n-1].Data == data) {
		return
	}
	tl.marks = append(tl.marks, timeMark{T: tl.since(now), Data: data})
}

// chose records the node the policy operator currently names, keeping a point per change.
func (tl *timeline) chose(node string, now time.Time) {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	if node == "" {
		return
	}
	i := float64(tl.node(node))
	if n := len(tl.choice); n > 0 && tl.choice[n-1][1] == i {
		return
	}
	tl.choice = append(tl.choice, [2]float64{tl.since(now), i})
}

// advise records what a copy's own policy check recommended, keeping an entry per change of action.
func (tl *timeline) advise(target, action, reason string, now time.Time) {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	marks := tl.advised[target]
	if n := len(marks); n > 0 && marks[n-1].Action == action {
		return
	}
	tl.advised[target] = append(marks, adviceMark{T: tl.since(now), Action: action, Reason: reason})
}

func (tl *timeline) snapshot() map[string]any {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	requests := map[string][][2]float64{}
	for k, v := range tl.requests {
		requests[k] = append([][2]float64(nil), v...)
	}
	ratios := map[string][][2]float64{}
	for k, v := range tl.ratios {
		ratios[k] = append([][2]float64(nil), v...)
	}
	breaking := map[string][][2]float64{}
	for k, v := range tl.breaking {
		breaking[k] = append([][2]float64(nil), v...)
	}
	advised := map[string][]adviceMark{}
	for k, v := range tl.advised {
		advised[k] = append([]adviceMark(nil), v...)
	}
	return map[string]any{
		"start": tl.start.UTC().Format(time.RFC3339Nano), "seconds": tl.since(time.Now()),
		"nodes": append([]string(nil), tl.nodes...), "requests": requests, "ratios": ratios,
		"breaking": breaking, "marks": append([]timeMark(nil), tl.marks...),
		"choice": append([][2]float64(nil), tl.choice...), "advice": advised,
	}
}
