package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// advice asks a copy of GreenCharge what its own Application Controller extension makes of the
// node it is running on, by calling the app's POST /adaptation/recommendation.
//
// The node's green share and region are the real values from its labels. The CPU, memory and
// network figures are fixed at values that satisfy the policy model, because the witness does
// not measure them; the verdict therefore turns only on green share and region.
func advice(ctx context.Context, client *http.Client, t target, node NodeView) (action, reason string, ok bool) {
	if node.GreenRatio == nil {
		return "", "", false
	}
	body, _ := json.Marshal(map[string]any{
		"metrics": map[string]any{
			"nodeName": node.Name,
			"cpu":      map[string]any{"cores": 2},
			"memory":   map[string]any{"capacity": "2Gi"},
			"network":  map[string]any{"bandwidth_mbps": 1000, "latency_ms": 40},
		},
		"greenRatio": *node.GreenRatio,
		"region":     node.Region,
	})
	ctx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.url+"/adaptation/recommendation", bytes.NewReader(body))
	if err != nil {
		return "", "", false
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", "", false
	}
	defer resp.Body.Close()
	var out struct {
		Action string `json:"action"`
		Reason string `json:"reason"`
	}
	if resp.StatusCode != http.StatusOK || json.NewDecoder(resp.Body).Decode(&out) != nil || out.Action == "" {
		return "", "", false
	}
	return out.Action, out.Reason, true
}
