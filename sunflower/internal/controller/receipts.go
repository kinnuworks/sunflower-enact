package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
)

// Receipt is the record of one thing Sunflower did, or decided not to do.
type Receipt struct {
	Time       time.Time `json:"time"`
	Namespace  string    `json:"namespace"`
	Deployment string    `json:"deployment"`
	Kind       string    `json:"kind"` // Placing, Placed, Watching, Ignored, Holding, Moving, Moved, Aborted, WouldMove
	Message    string    `json:"message"`
	From       string    `json:"from,omitempty"`
	To         string    `json:"to,omitempty"`
	Seconds    float64   `json:"seconds,omitempty"`
}

// Receipts keeps the most recent receipts in memory and serves them over HTTP.
//
// Kubernetes Events are also written, but the client rate-limits repeated events on one
// object, so on a busy day some would be dropped. This log is the complete record.
type Receipts struct {
	mu   sync.Mutex
	list []Receipt
	max  int
}

func NewReceipts(max int) *Receipts { return &Receipts{max: max} }

// add records a receipt and reports whether it was new.
func (r *Receipts) add(rc Receipt) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	// A reconcile that runs again before its own write is visible would repeat itself.
	for i := len(r.list) - 1; i >= 0 && rc.Time.Sub(r.list[i].Time) < 5*time.Second; i-- {
		if p := r.list[i]; p.Deployment == rc.Deployment && p.Namespace == rc.Namespace && p.Kind == rc.Kind && p.Message == rc.Message {
			return false
		}
	}
	r.list = append(r.list, rc)
	if len(r.list) > r.max {
		r.list = r.list[len(r.list)-r.max:]
	}
	return true
}

func (r *Receipts) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	r.mu.Lock()
	out := append([]Receipt{}, r.list...)
	r.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// note writes one receipt and the matching Kubernetes Event.
func (r *Reconciler) note(dep *appsv1.Deployment, eventType, kind string, rc Receipt, format string, args ...any) {
	rc.Time = r.now().UTC()
	rc.Namespace, rc.Deployment, rc.Kind = dep.Namespace, dep.Name, kind
	rc.Message = fmt.Sprintf(format, args...)
	if r.Receipts != nil && !r.Receipts.add(rc) {
		return
	}
	r.Recorder.Event(dep, eventType, kind, rc.Message)
}
