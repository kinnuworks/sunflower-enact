package placement

import (
	"strings"
	"testing"
	"time"
)

func f(v float64) *float64 { return &v }

var t0 = time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)

func node(name string, green float64) *Node {
	return &Node{Name: name, Ready: true, GreenRatio: f(green), Region: "eu-west"}
}

func policy(chosen string) Policy {
	return Policy{Name: "greencharge", ChosenNode: chosen, DecisionFresh: true, GreenMin: f(0.6), Regions: []string{"eu-west"}}
}

func TestStaysWhenOperatorHasNoDecision(t *testing.T) {
	d := Decide(Input{Now: t0, Policy: Policy{}, Current: node("a", 0.3)}, DefaultSettings())
	if d.Action != Stay {
		t.Fatalf("got %s: %s", d.Action, d.Reason)
	}
	stale := policy("b")
	stale.DecisionFresh = false
	if d := Decide(Input{Now: t0, Policy: stale, Current: node("a", 0.3)}, DefaultSettings()); d.Action != Stay {
		t.Fatalf("stale decision: got %s", d.Action)
	}
}

func TestFirstPlacementIsImmediate(t *testing.T) {
	d := Decide(Input{Now: t0, Policy: policy("b")}, DefaultSettings())
	if d.Action != Move || d.Target != "b" {
		t.Fatalf("got %s -> %q", d.Action, d.Target)
	}
}

func TestStaysOnTheChosenNode(t *testing.T) {
	d := Decide(Input{Now: t0, Policy: policy("a"), Current: node("a", 0.9)}, DefaultSettings())
	if d.Action != Stay {
		t.Fatalf("got %s", d.Action)
	}
}

// Observed on the cluster: with two equally good nodes the operator's choice flips back
// and forth. Following it would bounce the app, so a tie must never cause a move.
func TestHoldsWhenTwoNodesAreEquallyGood(t *testing.T) {
	in := Input{Now: t0, Policy: policy("b"), Current: node("a", 0.9), Chosen: node("b", 0.9),
		Candidate: "b", CandidateSince: t0.Add(-time.Hour)}
	d := Decide(in, DefaultSettings())
	if d.Action != Hold {
		t.Fatalf("got %s: %s", d.Action, d.Reason)
	}
	// Slightly greener is still not worth a move.
	in.Chosen = node("b", 0.95)
	if d := Decide(in, DefaultSettings()); d.Action != Hold {
		t.Fatalf("small gain: got %s", d.Action)
	}
}

func TestWaitsThenMovesWhenTheCurrentNodeBreaksTheGreenRule(t *testing.T) {
	s := DefaultSettings()
	in := Input{Now: t0, Policy: policy("b"), Current: node("a", 0.3), Chosen: node("b", 0.85)}

	first := Decide(in, s)
	if first.Action != Wait || first.Candidate != "b" || !first.CandidateSince.Equal(t0) {
		t.Fatalf("first look: %s candidate=%q since=%v", first.Action, first.Candidate, first.CandidateSince)
	}

	in.Now = t0.Add(10 * time.Second)
	in.Candidate, in.CandidateSince = first.Candidate, first.CandidateSince
	if d := Decide(in, s); d.Action != Wait {
		t.Fatalf("at 10s: got %s", d.Action)
	}

	in.Now = t0.Add(s.Settle)
	d := Decide(in, s)
	if d.Action != Move || d.Target != "b" {
		t.Fatalf("after settle: got %s: %s", d.Action, d.Reason)
	}
	for _, want := range []string{"30%", "60%", "b", "85%"} {
		if !strings.Contains(d.Reason, want) {
			t.Errorf("reason %q should mention %q", d.Reason, want)
		}
	}
}

func TestAShortDipIsIgnored(t *testing.T) {
	s := DefaultSettings()
	dip := Decide(Input{Now: t0, Policy: policy("b"), Current: node("a", 0.3), Chosen: node("b", 0.85)}, s)
	if dip.Action != Wait {
		t.Fatalf("dip: got %s", dip.Action)
	}
	// Ten seconds later the node has recovered and the operator names it again.
	back := Decide(Input{Now: t0.Add(10 * time.Second), Policy: policy("a"), Current: node("a", 0.9),
		Candidate: dip.Candidate, CandidateSince: dip.CandidateSince}, s)
	if back.Action != Stay || back.Candidate != "" {
		t.Fatalf("recovered: got %s candidate=%q", back.Action, back.Candidate)
	}
}

func TestTheWaitRestartsWhenTheOperatorChangesItsMind(t *testing.T) {
	s := DefaultSettings()
	in := Input{Now: t0.Add(15 * time.Second), Policy: policy("c"), Current: node("a", 0.3), Chosen: node("c", 0.8),
		Candidate: "b", CandidateSince: t0}
	d := Decide(in, s)
	if d.Action != Wait || d.Candidate != "c" || !d.CandidateSince.Equal(in.Now) {
		t.Fatalf("got %s candidate=%q since=%v", d.Action, d.Candidate, d.CandidateSince)
	}
}

func TestMovesForAClearlyGreenerNodeEvenWhenTheCurrentOneIsCompliant(t *testing.T) {
	in := Input{Now: t0.Add(time.Minute), Policy: policy("b"), Current: node("a", 0.65), Chosen: node("b", 0.9),
		Candidate: "b", CandidateSince: t0}
	if d := Decide(in, DefaultSettings()); d.Action != Move {
		t.Fatalf("got %s: %s", d.Action, d.Reason)
	}
}

func TestRespectsTheMinimumStay(t *testing.T) {
	s := DefaultSettings()
	in := Input{Now: t0.Add(30 * time.Second), Policy: policy("b"), Current: node("a", 0.3), Chosen: node("b", 0.85),
		Candidate: "b", CandidateSince: t0, LastMove: t0.Add(-10 * time.Second)}
	d := Decide(in, s)
	if d.Action != Wait || d.RetryAfter <= 0 {
		t.Fatalf("got %s retry=%v", d.Action, d.RetryAfter)
	}
	in.LastMove = t0.Add(-2 * time.Minute)
	if d := Decide(in, s); d.Action != Move {
		t.Fatalf("after dwell: got %s", d.Action)
	}
}

func TestRespectsTheHourlyMoveBudget(t *testing.T) {
	s := DefaultSettings()
	in := Input{Now: t0.Add(time.Hour), Policy: policy("b"), Current: node("a", 0.3), Chosen: node("b", 0.85),
		Candidate: "b", CandidateSince: t0, RecentMoves: s.MaxMovesPerHour}
	if d := Decide(in, s); d.Action != Hold {
		t.Fatalf("got %s", d.Action)
	}
}

func TestLeavesADeadNodeWithoutWaiting(t *testing.T) {
	dead := node("a", 0.9)
	dead.Ready = false
	in := Input{Now: t0, Policy: policy("b"), Current: dead, Chosen: node("b", 0.9), LastMove: t0.Add(-time.Second)}
	if d := Decide(in, DefaultSettings()); d.Action != Move {
		t.Fatalf("got %s: %s", d.Action, d.Reason)
	}
}

func TestMovesOutOfAForbiddenRegion(t *testing.T) {
	away := node("a", 0.9)
	away.Region = "us-east"
	in := Input{Now: t0.Add(time.Minute), Policy: policy("b"), Current: away, Chosen: node("b", 0.9),
		Candidate: "b", CandidateSince: t0}
	d := Decide(in, DefaultSettings())
	if d.Action != Move || !strings.Contains(d.Reason, "us-east") {
		t.Fatalf("got %s: %s", d.Action, d.Reason)
	}
}
