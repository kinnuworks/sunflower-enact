package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"
)

// ClusterState is what the screen shows about the cluster next to the request strip.
type ClusterState struct {
	Reachable   bool         `json:"reachable"`
	Error       string       `json:"error,omitempty"`
	Nodes       []NodeView   `json:"nodes"`
	Policies    []PolicyView `json:"policies"`
	Deployments []DeployView `json:"deployments"`
	Events      []EventView  `json:"events"`
	// Receipts is Sunflower's own log of actions, newest last.
	Receipts json.RawMessage `json:"receipts,omitempty"`
}

type NodeView struct {
	Name       string   `json:"name"`
	Ready      bool     `json:"ready"`
	GreenRatio *float64 `json:"greenRatio,omitempty"`
	Region     string   `json:"region,omitempty"`
	Zone       string   `json:"zone,omitempty"`
	Role       string   `json:"role,omitempty"`
	GridZone   string   `json:"gridZone,omitempty"`
}

type PolicyView struct {
	Name       string   `json:"name"`
	ChosenNode string   `json:"chosenNode"`
	Reason     string   `json:"reason,omitempty"`
	GreenMin   *float64 `json:"greenMin,omitempty"`
	GreenMode  string   `json:"greenMode,omitempty"`
}

type DeployView struct {
	Name        string            `json:"name"`
	Replicas    int               `json:"replicas"`
	Ready       int               `json:"ready"`
	PinnedNode  string            `json:"pinnedNode,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type EventView struct {
	Time    string `json:"time"`
	Object  string `json:"object"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
	Source  string `json:"source,omitempty"`
}

// clusterWatcher polls the Kubernetes API read-only. With KUBE_API set (for example
// http://127.0.0.1:8001 from `kubectl proxy`) it uses that; otherwise it uses the pod's
// service account. Without either it reports the cluster as unreachable and the request
// measurements carry on regardless.
type clusterWatcher struct {
	base      string
	token     string
	client    *http.Client
	namespace string
	receipts  string // URL of Sunflower's /receipts, optional
	// onPoll, when set, is called after each successful poll with the seconds since the last one.
	onPoll func(ClusterState, float64)

	mu    sync.Mutex
	state ClusterState
}

const saDir = "/var/run/secrets/kubernetes.io/serviceaccount"

func newClusterWatcher() *clusterWatcher {
	w := &clusterWatcher{namespace: env("WITNESS_NAMESPACE", "enact"), receipts: os.Getenv("WITNESS_RECEIPTS"),
		client: &http.Client{Timeout: 4 * time.Second}}
	if api := os.Getenv("KUBE_API"); api != "" {
		w.base = api
		return w
	}
	token, err := os.ReadFile(saDir + "/token")
	if err != nil {
		return w
	}
	w.token = string(token)
	w.base = "https://kubernetes.default.svc"
	pool := x509.NewCertPool()
	if ca, err := os.ReadFile(saDir + "/ca.crt"); err == nil {
		pool.AppendCertsFromPEM(ca)
	}
	w.client.Transport = &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}}
	return w
}

func (w *clusterWatcher) snapshot() ClusterState {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.state
}

func (w *clusterWatcher) run(ctx context.Context) {
	if w.base == "" {
		w.mu.Lock()
		w.state = ClusterState{Error: "no Kubernetes access configured"}
		w.mu.Unlock()
		return
	}
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	last := time.Now()
	for {
		st, err := w.poll(ctx)
		if err != nil {
			st = ClusterState{Error: err.Error()}
		} else if w.onPoll != nil {
			now := time.Now()
			w.onPoll(st, now.Sub(last).Seconds())
			last = now
		}
		w.mu.Lock()
		w.state = st
		w.mu.Unlock()
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

func (w *clusterWatcher) get(ctx context.Context, path string, into any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, w.base+path, nil)
	if err != nil {
		return err
	}
	if w.token != "" {
		req.Header.Set("Authorization", "Bearer "+w.token)
	}
	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", path, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(into)
}

type k8sMeta struct {
	Name              string            `json:"name"`
	Labels            map[string]string `json:"labels"`
	Annotations       map[string]string `json:"annotations"`
	CreationTimestamp string            `json:"creationTimestamp"`
}

func (w *clusterWatcher) poll(ctx context.Context) (ClusterState, error) {
	st := ClusterState{Reachable: true}

	var nodes struct {
		Items []struct {
			Metadata k8sMeta `json:"metadata"`
			Status   struct {
				Conditions []struct{ Type, Status string } `json:"conditions"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := w.get(ctx, "/api/v1/nodes", &nodes); err != nil {
		return st, err
	}
	for _, n := range nodes.Items {
		v := NodeView{
			Name:     n.Metadata.Name,
			Region:   n.Metadata.Labels["enact.eu/region"],
			Zone:     n.Metadata.Labels["enact.eu/zone"],
			Role:     n.Metadata.Labels["enact.eu/role"],
			GridZone: n.Metadata.Annotations["sunflower.enact.eu/grid-zone"],
		}
		if raw, ok := n.Metadata.Labels["enact.eu/green-ratio"]; ok {
			if f, err := strconv.ParseFloat(raw, 64); err == nil {
				v.GreenRatio = &f
			}
		}
		for _, c := range n.Status.Conditions {
			if c.Type == "Ready" {
				v.Ready = c.Status == "True"
			}
		}
		st.Nodes = append(st.Nodes, v)
	}

	var policies struct {
		Items []struct {
			Metadata k8sMeta `json:"metadata"`
			Spec     struct {
				GreenEnergy *struct {
					MinRatio *float64 `json:"minRatio"`
					Mode     string   `json:"mode"`
				} `json:"greenEnergy"`
			} `json:"spec"`
			Status struct {
				ChosenNode string `json:"chosenNode"`
				Reason     string `json:"reason"`
			} `json:"status"`
		} `json:"items"`
	}
	// A cluster without the RuntimePolicy CRD is still worth watching, so this is best-effort.
	if err := w.get(ctx, "/apis/enact.eu/v1alpha1/namespaces/"+w.namespace+"/runtimepolicies", &policies); err == nil {
		for _, p := range policies.Items {
			v := PolicyView{Name: p.Metadata.Name, ChosenNode: p.Status.ChosenNode, Reason: p.Status.Reason}
			if p.Spec.GreenEnergy != nil {
				v.GreenMin = p.Spec.GreenEnergy.MinRatio
				v.GreenMode = p.Spec.GreenEnergy.Mode
			}
			st.Policies = append(st.Policies, v)
		}
	}

	var deploys struct {
		Items []struct {
			Metadata k8sMeta `json:"metadata"`
			Spec     struct {
				Replicas int `json:"replicas"`
				Template struct {
					Spec struct {
						NodeSelector map[string]string `json:"nodeSelector"`
					} `json:"spec"`
				} `json:"template"`
			} `json:"spec"`
			Status struct {
				ReadyReplicas int `json:"readyReplicas"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := w.get(ctx, "/apis/apps/v1/namespaces/"+w.namespace+"/deployments?labelSelector=app.kubernetes.io/name=greencharge", &deploys); err == nil {
		for _, d := range deploys.Items {
			notes := map[string]string{}
			for k, val := range d.Metadata.Annotations {
				if len(k) > 19 && k[:19] == "sunflower.enact.eu/" {
					notes[k[19:]] = val
				}
			}
			st.Deployments = append(st.Deployments, DeployView{
				Name:        d.Metadata.Name,
				Replicas:    d.Spec.Replicas,
				Ready:       d.Status.ReadyReplicas,
				PinnedNode:  d.Spec.Template.Spec.NodeSelector["kubernetes.io/hostname"],
				Annotations: notes,
			})
		}
	}

	var events struct {
		Items []struct {
			Metadata           k8sMeta                     `json:"metadata"`
			InvolvedObject     struct{ Kind, Name string } `json:"involvedObject"`
			Reason             string                      `json:"reason"`
			Message            string                      `json:"message"`
			LastTimestamp      string                      `json:"lastTimestamp"`
			EventTime          string                      `json:"eventTime"`
			Source             struct{ Component string }  `json:"source"`
			ReportingComponent string                      `json:"reportingComponent"`
		} `json:"items"`
	}
	if err := w.get(ctx, "/api/v1/namespaces/"+w.namespace+"/events?fieldSelector=involvedObject.kind=Deployment", &events); err == nil {
		for _, e := range events.Items {
			src := e.Source.Component
			if src == "" {
				src = e.ReportingComponent
			}
			if src != "sunflower" {
				continue
			}
			when := e.LastTimestamp
			if when == "" {
				when = e.EventTime
			}
			if when == "" {
				when = e.Metadata.CreationTimestamp
			}
			st.Events = append(st.Events, EventView{Time: when, Object: e.InvolvedObject.Name, Reason: e.Reason, Message: e.Message, Source: src})
		}
		sort.Slice(st.Events, func(i, j int) bool { return st.Events[i].Time > st.Events[j].Time })
		if len(st.Events) > 30 {
			st.Events = st.Events[:30]
		}
	}
	if w.receipts != "" {
		if resp, err := http.Get(w.receipts); err == nil {
			if body, err := io.ReadAll(resp.Body); err == nil && resp.StatusCode == http.StatusOK && json.Valid(body) {
				st.Receipts = body
			}
			resp.Body.Close()
		}
	}
	return st, nil
}
