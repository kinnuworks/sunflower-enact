// Package controller carries out placement decisions on Deployments that opt in.
package controller

import (
	"context"
	"strconv"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kinnuworks/sunflower-enact/sunflower/internal/placement"
)

const (
	prefix = "sunflower.enact.eu/"

	// PolicyAnnotation opts a Deployment in: its value names a RuntimePolicy in the same namespace.
	PolicyAnnotation = prefix + "policy"

	candidateAnn      = prefix + "candidate"
	candidateSinceAnn = prefix + "candidate-since"
	candidateCauseAnn = prefix + "candidate-cause"
	lastMoveAnn       = prefix + "last-move"
	movesAnn          = prefix + "moves"
	moveStartedAnn    = prefix + "move-started"
	moveFromAnn       = prefix + "move-from"
	moveCauseAnn      = prefix + "move-cause"
	statusAnn         = prefix + "status"

	// HostnameLabel is the pin the ENACT SDK itself sets at deploy time; Sunflower uses the same one.
	HostnameLabel = "kubernetes.io/hostname"

	greenRatioLabel = "enact.eu/green-ratio"
	regionLabel     = "enact.eu/region"
)

var runtimePolicyGVK = schema.GroupVersionKind{Group: "enact.eu", Version: "v1alpha1", Kind: "RuntimePolicy"}

// Reconciler moves a Deployment to the node its RuntimePolicy has chosen, when the
// placement rule says the move is justified.
type Reconciler struct {
	client.Client
	Recorder record.EventRecorder
	Settings placement.Settings
	// MoveTimeout is how long a move may take before it is undone.
	MoveTimeout time.Duration
	// DryRun reports what would happen without changing any Deployment.
	DryRun bool
	Now    func() time.Time
}

func (r *Reconciler) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var dep appsv1.Deployment
	if err := r.Get(ctx, req.NamespacedName, &dep); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	policyName := dep.Annotations[PolicyAnnotation]
	if policyName == "" || !dep.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}
	now := r.now()

	if started := parseTime(dep.Annotations[moveStartedAnn]); !started.IsZero() {
		return r.followMove(ctx, &dep, started, now)
	}

	pol, err := r.readPolicy(ctx, dep.Namespace, policyName)
	if err != nil {
		return ctrl.Result{}, err
	}
	in := placement.Input{
		Now:            now,
		Policy:         pol,
		Candidate:      dep.Annotations[candidateAnn],
		CandidateSince: parseTime(dep.Annotations[candidateSinceAnn]),
		LastMove:       parseTime(dep.Annotations[lastMoveAnn]),
		RecentMoves:    len(recentMoves(dep.Annotations[movesAnn], now)),
	}
	if pinned := dep.Spec.Template.Spec.NodeSelector[HostnameLabel]; pinned != "" {
		n, err := r.readNode(ctx, pinned)
		if err != nil {
			return ctrl.Result{}, err
		}
		in.Current = n
	}
	if pol.ChosenNode != "" {
		n, err := r.readNode(ctx, pol.ChosenNode)
		if err != nil {
			return ctrl.Result{}, err
		}
		in.Chosen = n
	}

	d := placement.Decide(in, r.Settings)
	log.FromContext(ctx).V(1).Info("decision", "action", d.Action, "target", d.Target, "reason", d.Reason)

	if d.Action == placement.Move {
		return r.startMove(ctx, &dep, in, d, now)
	}

	// A dip that ended before the waiting period ran out is worth saying out loud: restraint
	// is part of the behaviour, not an absence of it.
	if had := dep.Annotations[candidateAnn]; had != "" && d.Candidate == "" && d.Action == placement.Stay {
		waited := now.Sub(parseTime(dep.Annotations[candidateSinceAnn])).Round(time.Second)
		r.Recorder.Eventf(&dep, corev1.EventTypeNormal, "Ignored",
			"Ignored a %s dip: %s, but it recovered before the %s waiting period ended. No move.",
			waited, lowerFirst(dep.Annotations[candidateCauseAnn]), r.Settings.Settle.Round(time.Second))
	}
	if d.Action == placement.Wait && dep.Annotations[candidateAnn] != d.Candidate {
		r.Recorder.Event(&dep, corev1.EventTypeNormal, "Watching", d.Reason)
	}
	if d.Action == placement.Hold && dep.Annotations[statusAnn] != string(placement.Hold) {
		r.Recorder.Event(&dep, corev1.EventTypeNormal, "Holding", d.Reason)
	}

	if err := r.remember(ctx, &dep, map[string]string{
		candidateAnn:      d.Candidate,
		candidateSinceAnn: formatTime(d.CandidateSince),
		candidateCauseAnn: d.Cause,
		statusAnn:         string(d.Action),
	}); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: d.RetryAfter}, nil
}

func (r *Reconciler) startMove(ctx context.Context, dep *appsv1.Deployment, in placement.Input, d placement.Decision, now time.Time) (ctrl.Result, error) {
	from := ""
	if in.Current != nil {
		from = in.Current.Name
	}
	if r.DryRun {
		r.Recorder.Eventf(dep, corev1.EventTypeNormal, "WouldMove", "Dry run: would move to %s. %s", d.Target, d.Reason)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
	patch := client.MergeFrom(dep.DeepCopy())
	if dep.Spec.Template.Spec.NodeSelector == nil {
		dep.Spec.Template.Spec.NodeSelector = map[string]string{}
	}
	dep.Spec.Template.Spec.NodeSelector[HostnameLabel] = d.Target
	setAnnotations(dep, map[string]string{
		moveStartedAnn:    formatTime(now),
		moveFromAnn:       from,
		moveCauseAnn:      d.Reason,
		candidateAnn:      "",
		candidateSinceAnn: "",
		candidateCauseAnn: "",
		statusAnn:         "Moving",
	})
	if err := r.Patch(ctx, dep, patch, client.FieldOwner("sunflower")); err != nil {
		return ctrl.Result{}, err
	}
	if from == "" {
		r.Recorder.Eventf(dep, corev1.EventTypeNormal, "Placing", "Placing on %s. %s", d.Target, d.Reason)
	} else {
		r.Recorder.Eventf(dep, corev1.EventTypeNormal, "Moving", "Moving from %s to %s. %s", from, d.Target, d.Reason)
	}
	return ctrl.Result{RequeueAfter: time.Second}, nil
}

// followMove watches a move that is under way: it records the result when the new pod has
// taken over, and puts the app back if the new node cannot run it in time.
func (r *Reconciler) followMove(ctx context.Context, dep *appsv1.Deployment, started, now time.Time) (ctrl.Result, error) {
	target := dep.Spec.Template.Spec.NodeSelector[HostnameLabel]
	from := dep.Annotations[moveFromAnn]
	took := now.Sub(started)

	if rolledOut(dep) {
		cause := dep.Annotations[moveCauseAnn]
		notes := map[string]string{moveStartedAnn: "", moveFromAnn: "", moveCauseAnn: "", statusAnn: string(placement.Stay)}
		if from != "" {
			// The first placement is not a move: it does not start the minimum-stay clock or
			// count against the hourly budget.
			notes[lastMoveAnn] = formatTime(now)
			notes[movesAnn] = joinTimes(append(recentMoves(dep.Annotations[movesAnn], now), now))
		}
		if err := r.remember(ctx, dep, notes); err != nil {
			return ctrl.Result{}, err
		}
		if from == "" {
			r.Recorder.Eventf(dep, corev1.EventTypeNormal, "Placed", "Running on %s after %s.", target, took.Round(100*time.Millisecond))
		} else {
			r.Recorder.Eventf(dep, corev1.EventTypeNormal, "Moved", "Moved from %s to %s in %s. %s",
				from, target, took.Round(100*time.Millisecond), cause)
		}
		return ctrl.Result{}, nil
	}

	if took > r.MoveTimeout && from != "" {
		patch := client.MergeFrom(dep.DeepCopy())
		dep.Spec.Template.Spec.NodeSelector[HostnameLabel] = from
		setAnnotations(dep, map[string]string{
			moveStartedAnn: "", moveFromAnn: "", moveCauseAnn: "",
			lastMoveAnn: formatTime(now),
			statusAnn:   "Aborted",
		})
		if err := r.Patch(ctx, dep, patch, client.FieldOwner("sunflower")); err != nil {
			return ctrl.Result{}, err
		}
		r.Recorder.Eventf(dep, corev1.EventTypeWarning, "Aborted",
			"The app was not ready on %s after %s, so it stays on %s. The old copy kept serving throughout.",
			target, r.MoveTimeout.Round(time.Second), from)
		return ctrl.Result{}, nil
	}
	return ctrl.Result{RequeueAfter: 500 * time.Millisecond}, nil
}

// rolledOut reports whether every pod is from the current template, available, and no old pod remains.
func rolledOut(dep *appsv1.Deployment) bool {
	want := int32(1)
	if dep.Spec.Replicas != nil {
		want = *dep.Spec.Replicas
	}
	s := dep.Status
	return s.ObservedGeneration >= dep.Generation &&
		s.UpdatedReplicas == want && s.AvailableReplicas == want && s.Replicas == want
}

func (r *Reconciler) readPolicy(ctx context.Context, namespace, name string) (placement.Policy, error) {
	p := placement.Policy{Name: name}
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(runtimePolicyGVK)
	if err := r.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, u); err != nil {
		if apierrors.IsNotFound(err) {
			return p, nil // no policy yet means no decision; the rule answers Stay
		}
		return p, err
	}
	p.ChosenNode, _, _ = unstructured.NestedString(u.Object, "status", "chosenNode")
	observed, _, _ := unstructured.NestedInt64(u.Object, "status", "observedGeneration")
	p.DecisionFresh = observed == u.GetGeneration() && conditionTrue(u, "Available")
	if min, ok, _ := unstructured.NestedFloat64(u.Object, "spec", "greenEnergy", "minRatio"); ok {
		p.GreenMin = &min
	} else if min, ok, _ := unstructured.NestedInt64(u.Object, "spec", "greenEnergy", "minRatio"); ok {
		f := float64(min)
		p.GreenMin = &f
	}
	if mode, _, _ := unstructured.NestedString(u.Object, "spec", "location", "mode"); mode == "Hard" {
		p.Regions, _, _ = unstructured.NestedStringSlice(u.Object, "spec", "location", "regions")
	}
	return p, nil
}

func conditionTrue(u *unstructured.Unstructured, kind string) bool {
	conds, _, _ := unstructured.NestedSlice(u.Object, "status", "conditions")
	for _, c := range conds {
		m, ok := c.(map[string]any)
		if ok && m["type"] == kind {
			return m["status"] == "True"
		}
	}
	return false
}

func (r *Reconciler) readNode(ctx context.Context, name string) (*placement.Node, error) {
	var n corev1.Node
	if err := r.Get(ctx, types.NamespacedName{Name: name}, &n); err != nil {
		if apierrors.IsNotFound(err) {
			return &placement.Node{Name: name}, nil // a node that is gone is, at least, not ready
		}
		return nil, err
	}
	out := &placement.Node{Name: name, Region: n.Labels[regionLabel]}
	if raw, ok := n.Labels[greenRatioLabel]; ok {
		if v, err := strconv.ParseFloat(raw, 64); err == nil {
			out.GreenRatio = &v
		}
	}
	for _, c := range n.Status.Conditions {
		if c.Type == corev1.NodeReady {
			out.Ready = c.Status == corev1.ConditionTrue
		}
	}
	return out, nil
}

// remember stores Sunflower's notes on the Deployment itself, so a restart loses nothing.
func (r *Reconciler) remember(ctx context.Context, dep *appsv1.Deployment, notes map[string]string) error {
	changed := false
	for k, v := range notes {
		if dep.Annotations[k] != v {
			changed = true
		}
	}
	if !changed {
		return nil
	}
	patch := client.MergeFrom(dep.DeepCopy())
	setAnnotations(dep, notes)
	return r.Patch(ctx, dep, patch, client.FieldOwner("sunflower"))
}

func setAnnotations(dep *appsv1.Deployment, notes map[string]string) {
	if dep.Annotations == nil {
		dep.Annotations = map[string]string{}
	}
	for k, v := range notes {
		if v == "" {
			delete(dep.Annotations, k)
		} else {
			dep.Annotations[k] = v
		}
	}
}

func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func recentMoves(raw string, now time.Time) []time.Time {
	var out []time.Time
	for _, part := range strings.Split(raw, ",") {
		if t := parseTime(part); !t.IsZero() && now.Sub(t) < time.Hour {
			out = append(out, t)
		}
	}
	return out
}

func joinTimes(ts []time.Time) string {
	parts := make([]string, len(ts))
	for i, t := range ts {
		parts[i] = formatTime(t)
	}
	return strings.Join(parts, ",")
}

func lowerFirst(s string) string {
	if s == "" {
		return "conditions changed"
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// SetupWithManager wires the reconciler to Deployments, and re-checks them whenever a
// RuntimePolicy or a Node changes.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	policies := &unstructured.Unstructured{}
	policies.SetGroupVersionKind(runtimePolicyGVK)

	optedIn := func(ctx context.Context, namespace string, match func(*appsv1.Deployment) bool) []reconcile.Request {
		var list appsv1.DeploymentList
		if err := mgr.GetClient().List(ctx, &list, client.InNamespace(namespace)); err != nil {
			return nil
		}
		var reqs []reconcile.Request
		for i := range list.Items {
			d := &list.Items[i]
			if d.Annotations[PolicyAnnotation] != "" && match(d) {
				reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: d.Namespace, Name: d.Name}})
			}
		}
		return reqs
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named("sunflower").
		For(&appsv1.Deployment{}).
		Watches(policies, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
			return optedIn(ctx, obj.GetNamespace(), func(d *appsv1.Deployment) bool {
				return d.Annotations[PolicyAnnotation] == obj.GetName()
			})
		})).
		Watches(&corev1.Node{}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, _ client.Object) []reconcile.Request {
			return optedIn(ctx, "", func(*appsv1.Deployment) bool { return true })
		})).
		Complete(r)
}
