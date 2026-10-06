// Package placement holds Sunflower's decision rule: given what the ENACT policy operator
// has chosen and where the app is now, should the app move?
//
// The rule never picks a node. The operator's choice is the only place a node can come
// from; this package only decides whether, and when, to act on it.
package placement

import (
	"fmt"
	"math"
	"time"
)

// Action is what the controller should do after one look at the world.
type Action string

const (
	// Stay: the app is where the operator wants it, or there is no decision to act on.
	Stay Action = "Stay"
	// Hold: the operator names another node, but moving is not justified.
	Hold Action = "Hold"
	// Wait: a move looks justified but has not been true for long enough yet.
	Wait Action = "Wait"
	// Move: re-pin the app to the operator's chosen node now.
	Move Action = "Move"
)

// Node is what Sunflower knows about one node, read from its labels and conditions.
type Node struct {
	Name       string
	Ready      bool
	GreenRatio *float64 // nil when the node carries no green-ratio label
	Region     string
}

// Policy is the part of a RuntimePolicy the rule needs.
type Policy struct {
	Name          string
	ChosenNode    string   // empty when the operator has no decision
	DecisionFresh bool     // the status reflects the current spec and reports Available
	GreenMin      *float64 // spec.greenEnergy.minRatio
	Regions       []string // spec.location.regions, when the location constraint is Hard
}

// Settings are the guard rails. The defaults are in DefaultSettings.
type Settings struct {
	// Settle is how long the operator must keep naming the same node, with the move still
	// justified, before Sunflower acts. Shorter dips are ignored.
	Settle time.Duration
	// Dwell is the minimum time between two moves of the same app.
	Dwell time.Duration
	// Margin is how much greener the chosen node must be before Sunflower leaves a node
	// that still satisfies the policy. It is what stops two equally good nodes from
	// trading the app back and forth.
	Margin float64
	// MaxMovesPerHour caps how often one app may be moved.
	MaxMovesPerHour int
}

func DefaultSettings() Settings {
	return Settings{Settle: 20 * time.Second, Dwell: 60 * time.Second, Margin: 0.10, MaxMovesPerHour: 6}
}

// Input is everything one decision depends on.
type Input struct {
	Now            time.Time
	Policy         Policy
	Current        *Node  // nil when the app is not pinned yet
	Chosen         *Node  // nil when the chosen node could not be read
	Candidate      string // node Sunflower has been waiting on, from an earlier look
	CandidateSince time.Time
	LastMove       time.Time
	RecentMoves    int // moves in the last hour
}

// Decision is the outcome, with a sentence a person can read.
type Decision struct {
	Action Action
	Target string
	// Reason explains the decision in plain language.
	Reason string
	// Cause is why leaving the current node is justified; empty unless Action is Wait or Move.
	Cause string
	// Candidate and CandidateSince are what the controller should remember for next time.
	Candidate      string
	CandidateSince time.Time
	// RetryAfter is when to look again; zero means "when something changes".
	RetryAfter time.Duration
}

// Decide applies the rule.
func Decide(in Input, s Settings) Decision {
	p := in.Policy
	if p.ChosenNode == "" || !p.DecisionFresh {
		return Decision{Action: Stay, Reason: "The policy operator has no current decision, so the app stays where it is."}
	}
	if in.Current == nil {
		return Decision{Action: Move, Target: p.ChosenNode, Cause: "first placement",
			Reason: fmt.Sprintf("First placement: the policy operator chose %s.", p.ChosenNode)}
	}
	if in.Current.Name == p.ChosenNode {
		return Decision{Action: Stay, Reason: fmt.Sprintf("The app is on %s, the node the policy operator chose.", p.ChosenNode)}
	}

	cause, urgent := whyLeave(in.Current, in.Chosen, p, s)
	if cause == "" {
		return Decision{Action: Hold, Target: p.ChosenNode, Reason: fmt.Sprintf(
			"The policy operator now names %s, but %s still satisfies the policy and %s is not at least %s greener. Not moving.",
			p.ChosenNode, in.Current.Name, p.ChosenNode, percent(s.Margin))}
	}

	since := in.CandidateSince
	if in.Candidate != p.ChosenNode || since.IsZero() {
		since = in.Now
	}
	d := Decision{Target: p.ChosenNode, Cause: cause, Candidate: p.ChosenNode, CandidateSince: since}

	if !urgent {
		if waited := in.Now.Sub(since); waited < s.Settle {
			d.Action = Wait
			d.RetryAfter = s.Settle - waited
			d.Reason = fmt.Sprintf("%s. Waiting %s to see whether it lasts before moving to %s.",
				cause, s.Settle.Round(time.Second), p.ChosenNode)
			return d
		}
		if !in.LastMove.IsZero() {
			if rested := in.Now.Sub(in.LastMove); rested < s.Dwell {
				d.Action = Wait
				d.RetryAfter = s.Dwell - rested
				d.Reason = fmt.Sprintf("%s. The app moved %s ago; waiting out the %s minimum stay before moving again.",
					cause, rested.Round(time.Second), s.Dwell.Round(time.Second))
				return d
			}
		}
		if s.MaxMovesPerHour > 0 && in.RecentMoves >= s.MaxMovesPerHour {
			d.Action = Hold
			d.RetryAfter = time.Minute
			d.Reason = fmt.Sprintf("%s. The app has already moved %d times in the last hour, which is the limit. Not moving.",
				cause, in.RecentMoves)
			return d
		}
	}

	d.Action = Move
	d.Reason = fmt.Sprintf("%s. The policy operator chose %s%s.", cause, p.ChosenNode, describe(in.Chosen))
	return d
}

// whyLeave returns the reason leaving the current node is justified, or "" if it is not.
// urgent is true when waiting would serve no purpose (the node is down).
func whyLeave(current, chosen *Node, p Policy, s Settings) (cause string, urgent bool) {
	if !current.Ready {
		return fmt.Sprintf("%s is not ready", current.Name), true
	}
	if len(p.Regions) > 0 && current.Region != "" && !contains(p.Regions, current.Region) {
		return fmt.Sprintf("%s is in region %s, outside the policy's %v", current.Name, current.Region, p.Regions), false
	}
	if p.GreenMin != nil && current.GreenRatio != nil && *current.GreenRatio+1e-9 < *p.GreenMin {
		return fmt.Sprintf("Green energy on %s is %s, below the policy's %s",
			current.Name, percent(*current.GreenRatio), percent(*p.GreenMin)), false
	}
	if chosen != nil && chosen.GreenRatio != nil && current.GreenRatio != nil {
		if gain := *chosen.GreenRatio - *current.GreenRatio; gain+1e-9 >= s.Margin {
			return fmt.Sprintf("%s is %s greener than %s (%s against %s)", chosen.Name,
				percent(gain), current.Name, percent(*chosen.GreenRatio), percent(*current.GreenRatio)), false
		}
	}
	return "", false
}

func describe(n *Node) string {
	if n == nil || n.GreenRatio == nil {
		return ""
	}
	return fmt.Sprintf(" (%s green)", percent(*n.GreenRatio))
}

func percent(ratio float64) string {
	return fmt.Sprintf("%d%%", int(math.Round(ratio*100)))
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
