package main

import (
	"testing"
	"time"
)

func TestTimelineKeepsOnePointPerChange(t *testing.T) {
	tl := newTimeline()
	now := tl.start
	tl.ratio("a", 0.8, now)
	tl.ratio("a", 0.8, now.Add(time.Second))
	tl.ratio("a", 0.5, now.Add(2*time.Second))
	tl.chose("a", now)
	tl.chose("a", now.Add(time.Second))
	tl.chose("b", now.Add(3*time.Second))
	if got := len(tl.ratios["a"]); got != 2 {
		t.Fatalf("ratio points = %d, want 2", got)
	}
	if got := len(tl.choice); got != 2 || tl.choice[1][0] != 3 {
		t.Fatalf("choice = %v, want two points with the second at 3s", tl.choice)
	}
}

func TestTimelineBreakingSpansOpenAndClose(t *testing.T) {
	tl := newTimeline()
	now := tl.start
	tl.below("app", false, now)
	tl.below("app", true, now.Add(2*time.Second))
	tl.below("app", true, now.Add(3*time.Second))
	if spans := tl.breaking["app"]; len(spans) != 1 || spans[0] != [2]float64{2, -1} {
		t.Fatalf("open span = %v, want [[2 -1]]", spans)
	}
	tl.below("app", false, now.Add(5*time.Second))
	if spans := tl.breaking["app"]; spans[0] != [2]float64{2, 5} {
		t.Fatalf("closed span = %v, want [2 5]", spans[0])
	}
}

func TestTimelineRequestsMarkFailuresAndIgnoreOldOnes(t *testing.T) {
	tl := newTimeline()
	now := tl.start
	tl.request(Sample{Time: now.Add(-time.Second), Target: "app", Node: "a", OK: true})
	tl.request(Sample{Time: now.Add(time.Second), Target: "app", Node: "a", OK: true})
	tl.request(Sample{Time: now.Add(2 * time.Second), Target: "app", OK: false})
	got := tl.requests["app"]
	if len(got) != 2 || got[0] != [2]float64{1, 0} || got[1] != [2]float64{2, -1} {
		t.Fatalf("requests = %v, want [[1 0] [2 -1]]", got)
	}
}
