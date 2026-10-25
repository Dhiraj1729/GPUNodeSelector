package controller

import (
	"context"
	"strings"
	"testing"
	"time"

	ai "github.com/Dhiraj1729/GpuNodeSelector/api/v1alpha1"
	"github.com/Dhiraj1729/GpuNodeSelector/internal/workload"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func statusFixture() (*ai.AIWorkload, *apps.Deployment, []observedPod) {
	w := &ai.AIWorkload{ObjectMeta: metav1.ObjectMeta{Name: "mock", Namespace: "test", UID: "w", Generation: 2}, Spec: ai.AIWorkloadSpec{Image: "mock:v1", Replicas: ptr.To(int32(1)), GPU: ai.GPURequirements{Product: "A100", Count: 1}}}
	d, _ := workload.Deployment(w)
	d.Generation = 3
	d.UID = "d"
	d.Status = apps.DeploymentStatus{ObservedGeneration: 3, Replicas: 1, UpdatedReplicas: 1, ReadyReplicas: 1, AvailableReplicas: 1}
	pods := []observedPod{{current: true, pod: core.Pod{Status: core.PodStatus{Phase: core.PodRunning, Conditions: []core.PodCondition{{Type: core.PodReady, Status: core.ConditionTrue}, {Type: core.PodScheduled, Status: core.ConditionTrue}}}}}}
	return w, d, pods
}
func unscheduled(p *observedPod, message string) {
	p.pod.Status = core.PodStatus{Phase: core.PodPending, Conditions: []core.PodCondition{{Type: core.PodScheduled, Status: core.ConditionFalse, Reason: core.PodReasonUnschedulable, Message: message}}}
}

func TestStatusPhases(t *testing.T) {
	cases := []struct {
		name     string
		change   func(*ai.AIWorkload, *apps.Deployment, *[]observedPod)
		phase    ai.Phase
		reason   string
		capacity metav1.ConditionStatus
	}{
		{"available", func(*ai.AIWorkload, *apps.Deployment, *[]observedPod) {}, ai.PhaseRunning, "Available", metav1.ConditionTrue},
		{"zero", func(w *ai.AIWorkload, d *apps.Deployment, p *[]observedPod) {
			w.Spec.Replicas = ptr.To(int32(0))
			d.Status = apps.DeploymentStatus{ObservedGeneration: 3}
			*p = nil
		}, ai.PhaseRunning, "ScaledToZero", metav1.ConditionUnknown},
		{"stale", func(w *ai.AIWorkload, d *apps.Deployment, p *[]observedPod) { d.Status.ObservedGeneration = 2 }, ai.PhaseProvisioning, "AwaitingObservation", metav1.ConditionUnknown},
		{"rollout", func(w *ai.AIWorkload, d *apps.Deployment, p *[]observedPod) { d.Status.UpdatedReplicas = 0 }, ai.PhaseProvisioning, "RollingOut", metav1.ConditionUnknown},
		{"missing-pods", func(w *ai.AIWorkload, d *apps.Deployment, p *[]observedPod) { *p = nil }, ai.PhaseProvisioning, "RollingOut", metav1.ConditionUnknown},
		{"terminating", func(w *ai.AIWorkload, d *apps.Deployment, p *[]observedPod) {
			(*p)[0].pod.DeletionTimestamp = ptr.To(metav1.Now())
		}, ai.PhaseProvisioning, "RollingOut", metav1.ConditionUnknown},
		{"old-failure", func(w *ai.AIWorkload, d *apps.Deployment, p *[]observedPod) {
			old := (*p)[0]
			old.current = false
			old.pod.Status.ContainerStatuses = []core.ContainerStatus{{State: core.ContainerState{Waiting: &core.ContainerStateWaiting{Reason: "ImagePullBackOff"}}}}
			*p = append(*p, old)
		}, ai.PhaseProvisioning, "RollingOut", metav1.ConditionUnknown},
		{"image", func(w *ai.AIWorkload, d *apps.Deployment, p *[]observedPod) {
			(*p)[0].pod.Status.ContainerStatuses = []core.ContainerStatus{{State: core.ContainerState{Waiting: &core.ContainerStateWaiting{Reason: "ImagePullBackOff"}}}}
		}, ai.PhaseDegraded, "ImagePullFailure", metav1.ConditionUnknown},
		{"crash", func(w *ai.AIWorkload, d *apps.Deployment, p *[]observedPod) {
			(*p)[0].pod.Status.InitContainerStatuses = []core.ContainerStatus{{State: core.ContainerState{Waiting: &core.ContainerStateWaiting{Reason: "CrashLoopBackOff"}}}}
		}, ai.PhaseDegraded, "ContainerFailure", metav1.ConditionUnknown},
		{"terminated", func(w *ai.AIWorkload, d *apps.Deployment, p *[]observedPod) {
			(*p)[0].pod.Status.ContainerStatuses = []core.ContainerStatus{{State: core.ContainerState{Terminated: &core.ContainerStateTerminated{ExitCode: 1}}}}
		}, ai.PhaseDegraded, "ContainerFailure", metav1.ConditionUnknown},
		{"deadline", func(w *ai.AIWorkload, d *apps.Deployment, p *[]observedPod) {
			d.Status.Conditions = []apps.DeploymentCondition{{Type: apps.DeploymentProgressing, Status: core.ConditionFalse, Reason: "ProgressDeadlineExceeded"}}
		}, ai.PhaseFailed, "ProgressDeadlineExceeded", metav1.ConditionUnknown},
		{"gpu-only", func(w *ai.AIWorkload, d *apps.Deployment, p *[]observedPod) {
			unscheduled(&(*p)[0], "0/1 nodes are available: 1 Insufficient ai.dhiraj.io/gpu. preemption: 0/1 nodes are available: No preemption victims found.")
		}, ai.PhaseCapacityBlocked, "InsufficientGPU", metav1.ConditionFalse},
		{"mixed", func(w *ai.AIWorkload, d *apps.Deployment, p *[]observedPod) {
			unscheduled(&(*p)[0], "0/3 nodes are available: 1 Insufficient ai.dhiraj.io/gpu, 1 node(s) didn't match Pod's node affinity/selector, 1 node(s) had untolerated taint {dedicated: other}.")
		}, ai.PhasePending, "MultipleSchedulingConstraints", metav1.ConditionUnknown},
		{"selector", func(w *ai.AIWorkload, d *apps.Deployment, p *[]observedPod) {
			unscheduled(&(*p)[0], "0/1 nodes are available: 1 node(s) didn't match Pod's node affinity/selector.")
		}, ai.PhasePending, "NodeSelectorMismatch", metav1.ConditionUnknown},
		{"taint", func(w *ai.AIWorkload, d *apps.Deployment, p *[]observedPod) {
			unscheduled(&(*p)[0], "0/1 nodes are available: 1 node(s) had untolerated taint {dedicated: other}.")
		}, ai.PhasePending, "UntoleratedTaint", metav1.ConditionUnknown},
		{"unknown", func(w *ai.AIWorkload, d *apps.Deployment, p *[]observedPod) {
			unscheduled(&(*p)[0], "new scheduler message")
		}, ai.PhasePending, "SchedulingBlocked", metav1.ConditionUnknown},
		{"readiness-loss", func(w *ai.AIWorkload, d *apps.Deployment, p *[]observedPod) {
			w.Status.Phase = ai.PhaseRunning
			w.Status.ObservedGeneration = w.Generation
			d.Status.AvailableReplicas = 0
			(*p)[0].pod.Status.Conditions[0].Status = core.ConditionFalse
		}, ai.PhaseDegraded, "DeploymentUnavailable", metav1.ConditionUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, d, p := statusFixture()
			tc.change(w, d, &p)
			s := deriveStatus(w, d, p, true, "endpoint")
			c := meta.FindStatusCondition(s.Conditions, string(ai.ConditionReady))
			capacity := meta.FindStatusCondition(s.Conditions, string(ai.ConditionCapacityAvailable))
			if s.Phase != tc.phase || c.Reason != tc.reason || capacity.Status != tc.capacity {
				t.Fatalf("phase=%s reason=%s capacity=%s", s.Phase, c.Reason, capacity.Status)
			}
			if len(s.Conditions) != 4 || s.ObservedGeneration != w.Generation {
				t.Fatal("incomplete conditions/generation")
			}
			for _, c := range s.Conditions {
				if c.ObservedGeneration != w.Generation {
					t.Fatal("stale condition generation")
				}
			}
			// Identical observations must not alter any condition timestamps or values.
			w.Status = s
			again := deriveStatus(w, d, p, true, "endpoint")
			for i, c := range s.Conditions {
				if c != again.Conditions[i] {
					t.Fatalf("condition churn: %v -> %v", c, again.Conditions[i])
				}
			}
		})
	}
}

func TestStatusCountsAndTransitions(t *testing.T) {
	w, d, p := statusFixture()
	old := p[0]
	old.current = false
	unscheduled(&old, "unknown")
	p = append(p, old)
	terminating := old
	terminating.pod = *old.pod.DeepCopy()
	terminating.pod.DeletionTimestamp = ptr.To(metav1.Now())
	p = append(p, terminating)
	s := deriveStatus(w, d, p, true, "")
	if s.PendingReplicas != 1 || s.ReadyReplicas != 1 || s.UpdatedReplicas != 1 {
		t.Fatal("incorrect pending/observed counts")
	}
	timestamp := metav1.NewTime(time.Unix(100, 0))
	for i := range s.Conditions {
		s.Conditions[i].LastTransitionTime = timestamp
	}
	w.Status = s
	d.Status.ObservedGeneration = 0
	next := deriveStatus(w, d, p, true, "")
	ready := meta.FindStatusCondition(next.Conditions, string(ai.ConditionReady))
	if ready.LastTransitionTime != timestamp {
		t.Fatal("false-to-false transition changed time")
	}
	d.Status.ObservedGeneration = d.Generation
	w.Status = next
	next = deriveStatus(w, d, p[:1], true, "")
	ready = meta.FindStatusCondition(next.Conditions, string(ai.ConditionReady))
	if ready.Status != metav1.ConditionTrue || ready.LastTransitionTime == timestamp {
		t.Fatal("readiness transition not recorded")
	}
	if s := deriveStatus(w, d, p[:1], false, ""); s.Phase == ai.PhaseRunning {
		t.Fatal("pre-patch desired state reported running")
	}
	if s := deriveStatus(w, nil, nil, false, ""); s.Phase != ai.PhaseProvisioning {
		t.Fatal("missing child misclassified")
	}
}

func TestConservativeSchedulerParsing(t *testing.T) {
	for _, message := range []string{"Insufficient ai.dhiraj.io/gpu-extra", "Insufficient ai.dhiraj.io/gpu and unknown blocker", strings.Repeat("x", 4097), ""} {
		c := schedulingCauses(message)
		if c["gpu"] && len(c) == 1 {
			t.Fatalf("false capacity diagnosis: %q", message)
		}
	}
	w, d, p := statusFixture()
	unscheduled(&p[0], "1 Insufficient ai.dhiraj.io/gpu")
	other := p[0]
	other.pod = *p[0].pod.DeepCopy()
	unscheduled(&other, "1 Insufficient cpu")
	p = append(p, other)
	if s := deriveStatus(w, d, p, true, ""); s.Phase != ai.PhasePending {
		t.Fatal("mixed blockers across Pods ignored")
	}
	d.Status.Conditions = []apps.DeploymentCondition{{Type: apps.DeploymentProgressing, Status: core.ConditionFalse, Reason: "ProgressDeadlineExceeded"}}
	if s := deriveStatus(w, d, p, true, ""); s.Phase != ai.PhaseFailed {
		t.Fatal("deadline precedence lost")
	}
}

func TestTerminalAndDrainingPods(t *testing.T) {
	w, d, p := statusFixture()
	failed := p[0]
	failed.pod = *p[0].pod.DeepCopy()
	failed.pod.Status.Phase = core.PodFailed
	s := deriveStatus(w, d, append(p, failed), true, "")
	if s.Phase != ai.PhaseRunning {
		t.Fatal("historical failed Pod hid healthy replacement")
	}
	s = deriveStatus(w, d, []observedPod{failed}, true, "")
	if s.Phase != ai.PhaseDegraded {
		t.Fatal("failed Pod without replacement not diagnosed")
	}
	w.Spec.Replicas = ptr.To(int32(0))
	d.Status = apps.DeploymentStatus{ObservedGeneration: d.Generation}
	p[0].pod.DeletionTimestamp = ptr.To(metav1.Now())
	if s := deriveStatus(w, d, p, true, ""); s.Phase == ai.PhaseRunning {
		t.Fatal("scaled to zero before terminating Pod disappeared")
	}
}

func TestOwnedPodObservations(t *testing.T) {
	r, w := fixture(t)
	ctx := context.Background()
	d, _ := workload.Deployment(w)
	d.UID = "d"
	rs := &apps.ReplicaSet{ObjectMeta: metav1.ObjectMeta{Name: "rs", Namespace: w.Namespace, UID: "rs", OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(d, apps.SchemeGroupVersion.WithKind("Deployment"))}}, Spec: apps.ReplicaSetSpec{Template: d.Spec.Template}}
	if err := r.Create(ctx, rs); err != nil {
		t.Fatal(err)
	}
	p := &core.Pod{ObjectMeta: metav1.ObjectMeta{Name: "valid", Namespace: w.Namespace, Labels: d.Spec.Template.Labels, Annotations: d.Spec.Template.Annotations, OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(rs, apps.SchemeGroupVersion.WithKind("ReplicaSet"))}}}
	if err := r.Create(ctx, p); err != nil {
		t.Fatal(err)
	}
	forged := p.DeepCopy()
	forged.Name = "forged"
	forged.ResourceVersion = ""
	forged.OwnerReferences[0].UID = "stale"
	if err := r.Create(ctx, forged); err != nil {
		t.Fatal(err)
	}
	pods, err := r.ownedPods(ctx, w, d)
	if err != nil || len(pods) != 1 || !pods[0].current {
		t.Fatalf("ownership filter: %v %v", pods, err)
	}
	rs.Spec.Template = *rs.Spec.Template.DeepCopy()
	rs.Spec.Template.Spec.Containers[0].Image = "old"
	if err := r.Update(ctx, rs); err != nil {
		t.Fatal(err)
	}
	pods, err = r.ownedPods(ctx, w, d)
	if err != nil || len(pods) != 1 || pods[0].current {
		t.Fatal("old revision not recognized")
	}
}

type countedStatus struct {
	client.SubResourceWriter
	count *int
}

func (c countedStatus) Patch(ctx context.Context, obj client.Object, p client.Patch, opts ...client.SubResourcePatchOption) error {
	*c.count++
	return c.SubResourceWriter.Patch(ctx, obj, p, opts...)
}

type statusClient struct {
	client.Client
	count int
}

func (c *statusClient) Status() client.SubResourceWriter {
	return countedStatus{SubResourceWriter: c.Client.Status(), count: &c.count}
}
func TestStatusPatchIdempotency(t *testing.T) {
	r, w := fixture(t)
	c := &statusClient{Client: r.Client}
	r.Client = c
	ctx := context.Background()
	if err := r.status(ctx, w, "OwnershipConflict", "collision", ""); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err := r.status(ctx, w, "OwnershipConflict", "collision", ""); err != nil {
			t.Fatal(err)
		}
	}
	if c.count != 1 {
		t.Fatalf("unchanged failure status patched %d times", c.count)
	}
	if w.Status.Phase != ai.PhaseFailed || len(w.Status.Conditions) != 4 {
		t.Fatal("failure status incomplete")
	}
}
