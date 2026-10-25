package controller

import (
	"context"
	"fmt"
	"time"

	ai "github.com/Dhiraj1729/GpuNodeSelector/api/v1alpha1"
	"github.com/Dhiraj1729/GpuNodeSelector/internal/workload"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	equality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type observedPod struct {
	pod     core.Pod
	current bool
}

// Compare ReplicaSet templates ignoring only the controller's unique hash.
func sameRevision(a, b core.PodTemplateSpec) bool {
	a = *a.DeepCopy()
	b = *b.DeepCopy()
	delete(a.Labels, apps.DefaultDeploymentUniqueLabelKey)
	delete(b.Labels, apps.DefaultDeploymentUniqueLabelKey)
	return equality.Semantic.DeepEqual(a, b)
}

func (r *AIWorkloadReconciler) ownedPods(ctx context.Context, w *ai.AIWorkload, d *apps.Deployment) ([]observedPod, error) {
	sets := &apps.ReplicaSetList{}
	if err := r.List(ctx, sets, client.InNamespace(w.Namespace)); err != nil {
		return nil, err
	}
	owned := map[types.UID]*apps.ReplicaSet{}
	for i := range sets.Items {
		rs := &sets.Items[i]
		if controlledBy(rs, d, "Deployment", apps.SchemeGroupVersion.String()) {
			owned[rs.UID] = rs
		}
	}
	pods := &core.PodList{}
	if err := r.List(ctx, pods, client.InNamespace(w.Namespace), client.MatchingLabels{workload.IdentityLabel: d.Spec.Selector.MatchLabels[workload.IdentityLabel]}); err != nil {
		return nil, err
	}
	result := []observedPod{}
	for _, p := range pods.Items {
		ref := metav1.GetControllerOf(&p)
		if ref == nil {
			continue
		}
		rs := owned[ref.UID]
		if rs == nil || !controlledBy(&p, rs, "ReplicaSet", apps.SchemeGroupVersion.String()) || p.Annotations[workload.WorkloadNameAnnotation] != w.Name {
			continue
		}
		result = append(result, observedPod{pod: p, current: sameRevision(rs.Spec.Template, d.Spec.Template)})
	}
	return result, nil
}

func (r *AIWorkloadReconciler) observeStatus(ctx context.Context, w *ai.AIWorkload, want *apps.Deployment, service *core.Service) (ctrl.Result, error) {
	d := &apps.Deployment{}
	if err := r.Get(ctx, client.ObjectKeyFromObject(want), d); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		d = nil
	}
	s := &core.Service{}
	if err := r.Get(ctx, client.ObjectKeyFromObject(service), s); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		s = nil
	}
	// A cache may still contain pre-patch state. Do not report that as current.
	if (d != nil && !controlledBy(d, w, "AIWorkload", ai.GroupVersion.String())) || (s != nil && !controlledBy(s, w, "AIWorkload", ai.GroupVersion.String())) {
		err := &ownershipConflict{name: want.Namespace + "/" + want.Name}
		if statusErr := r.status(ctx, w, "OwnershipConflict", err.Error(), ""); statusErr != nil {
			return ctrl.Result{}, statusErr
		}
		return ctrl.Result{}, err
	}
	current := d != nil && s != nil && d.DeletionTimestamp.IsZero() && s.DeletionTimestamp.IsZero()
	if current {
		merged := d.DeepCopy()
		err := mergeDeployment(merged, want)
		current = err == nil && equality.Semantic.DeepEqual(merged.Spec, d.Spec) && equality.Semantic.DeepEqual(s.Spec.Selector, service.Spec.Selector) && equality.Semantic.DeepEqual(s.Spec.Ports, service.Spec.Ports) && s.Spec.Type == service.Spec.Type
	}
	var pods []observedPod
	var err error
	if d != nil && controlledBy(d, w, "AIWorkload", ai.GroupVersion.String()) {
		pods, err = r.ownedPods(ctx, w, d)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	endpoint := ""
	if s != nil && controlledBy(s, w, "AIWorkload", ai.GroupVersion.String()) {
		endpoint = fmt.Sprintf("http://%s.%s.svc:%d", service.Name, service.Namespace, service.Spec.Ports[0].Port)
	}
	status := deriveStatus(w, d, pods, current, endpoint)
	if err := r.patchStatus(ctx, w, status); err != nil {
		return ctrl.Result{}, err
	}
	if status.Phase != ai.PhaseRunning && status.Phase != ai.PhaseFailed {
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	}
	return ctrl.Result{}, nil
}

func baseStatus(w *ai.AIWorkload, endpoint string) ai.AIWorkloadStatus {
	desired := int32(1)
	if w.Spec.Replicas != nil {
		desired = *w.Spec.Replicas
	}
	if desired < 0 {
		desired = 0
	}
	return ai.AIWorkloadStatus{ObservedGeneration: w.Generation, DesiredReplicas: desired, Endpoint: endpoint, Conditions: append([]metav1.Condition(nil), w.Status.Conditions...)}
}

func condition(s *ai.AIWorkloadStatus, kind ai.ConditionType, value metav1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition(&s.Conditions, metav1.Condition{Type: string(kind), Status: value, Reason: reason, Message: message, ObservedGeneration: s.ObservedGeneration})
}

func phase(s *ai.AIWorkloadStatus, p ai.Phase, reason, message string) {
	s.Phase = p
	ready := metav1.ConditionFalse
	progress := metav1.ConditionTrue
	if p == ai.PhaseRunning {
		ready = metav1.ConditionTrue
		progress = metav1.ConditionFalse
	}
	if p == ai.PhaseFailed || p == ai.PhaseDegraded || p == ai.PhasePending || p == ai.PhaseCapacityBlocked {
		progress = metav1.ConditionFalse
	}
	condition(s, ai.ConditionReady, ready, reason, message)
	condition(s, ai.ConditionProgressing, progress, reason, message)
}

func deriveStatus(w *ai.AIWorkload, d *apps.Deployment, pods []observedPod, current bool, endpoint string) (s ai.AIWorkloadStatus) {
	s = baseStatus(w, endpoint)
	// Compare final conditions, not intermediate Unknown -> True/False steps.
	defer func() { preserveTransitions(w.Status, &s) }()
	condition(&s, ai.ConditionScheduled, metav1.ConditionUnknown, "AwaitingObservation", "Scheduling evidence is not yet conclusive")
	condition(&s, ai.ConditionCapacityAvailable, metav1.ConditionUnknown, "AwaitingObservation", "No capacity conclusion from current observations")
	if d != nil {
		s.UpdatedReplicas = d.Status.UpdatedReplicas
		s.ReadyReplicas = d.Status.ReadyReplicas
		s.AvailableReplicas = d.Status.AvailableReplicas
	}
	active, old, ready, scheduled := 0, 0, 0, 0
	draining := false
	causes := map[string]bool{}
	blocked := false
	failure := ""
	terminalFailure := false
	for _, item := range pods {
		p := &item.pod
		if !p.DeletionTimestamp.IsZero() {
			draining = true
			continue
		}
		if p.Status.Phase == core.PodPending {
			s.PendingReplicas++
		}
		if !item.current {
			if p.Status.Phase != core.PodSucceeded && p.Status.Phase != core.PodFailed {
				old++
			}
			continue
		}
		if p.Status.Phase == core.PodSucceeded || p.Status.Phase == core.PodFailed {
			terminalFailure = terminalFailure || p.Status.Phase == core.PodFailed
			continue
		}
		f := containerFailure(p)
		if f == "ImagePullFailure" || failure == "" {
			failure = f
		}
		active++
		for _, c := range p.Status.Conditions {
			if c.Type == core.PodReady && c.Status == core.ConditionTrue {
				ready++
			}
			if c.Type == core.PodScheduled && c.Status == core.ConditionTrue {
				scheduled++
			}
			if c.Type == core.PodScheduled && c.Status == core.ConditionFalse {
				blocked = true
				if c.Reason == core.PodReasonUnschedulable {
					for cause := range schedulingCauses(c.Message) {
						causes[cause] = true
					}
				} else {
					causes["unknown"] = true
				}
			}
		}
	}
	if terminalFailure && active == 0 && s.DesiredReplicas > 0 && failure == "" {
		failure = "ContainerFailure"
	}
	if d == nil || !current || d.Status.ObservedGeneration < d.Generation {
		phase(&s, ai.PhaseProvisioning, "AwaitingObservation", "Waiting for current desired Deployment and controller observations")
		return s
	}
	if blocked {
		condition(&s, ai.ConditionScheduled, metav1.ConditionFalse, "SchedulingBlocked", "Current revision Pods report scheduling blockers")
	} else if active > 0 && scheduled == active {
		condition(&s, ai.ConditionScheduled, metav1.ConditionTrue, "PodsScheduled", "All observed active current revision Pods are scheduled")
	}
	if causes["gpu"] && len(causes) == 1 {
		condition(&s, ai.ConditionCapacityAvailable, metav1.ConditionFalse, "InsufficientGPU", "Scheduler reports only insufficient ai.dhiraj.io/gpu for blocked Pods")
	}
	for _, c := range d.Status.Conditions {
		if c.Type == apps.DeploymentProgressing && c.Status == core.ConditionFalse && c.Reason == "ProgressDeadlineExceeded" {
			phase(&s, ai.PhaseFailed, "ProgressDeadlineExceeded", "Deployment exceeded its progress deadline")
			return s
		}
	}
	if failure != "" {
		phase(&s, ai.PhaseDegraded, failure, "Current revision Pod reports an image or container failure")
		return s
	}
	if blocked {
		reason := "SchedulingBlocked"
		p := ai.PhasePending
		message := "Scheduler reports an unknown or unsupported constraint"
		if len(causes) > 1 {
			reason = "MultipleSchedulingConstraints"
			message = "Scheduler reports multiple constraints; GPU capacity alone is not the diagnosis"
		} else if causes["gpu"] {
			reason = "InsufficientGPU"
			p = ai.PhaseCapacityBlocked
			message = "Scheduler reports insufficient ai.dhiraj.io/gpu"
		} else if causes["selector"] {
			reason = "NodeSelectorMismatch"
			message = "Scheduler reports node affinity/selector mismatch; specific product mismatch is not proven"
		} else if causes["taint"] {
			reason = "UntoleratedTaint"
			message = "Scheduler reports an untolerated taint"
		}
		phase(&s, p, reason, message)
		return s
	}
	n := s.DesiredReplicas
	converged := !draining && d.Status.Replicas == n && d.Status.UpdatedReplicas == n && d.Status.ReadyReplicas == n && d.Status.AvailableReplicas == n && d.Status.UnavailableReplicas == 0 && old == 0 && int32(active) == n && int32(ready) == n && int32(scheduled) == n
	if converged {
		if n == 0 {
			phase(&s, ai.PhaseRunning, "ScaledToZero", "Deployment has converged to zero replicas")
			condition(&s, ai.ConditionScheduled, metav1.ConditionTrue, "NoReplicasRequested", "No replicas require scheduling")
			condition(&s, ai.ConditionCapacityAvailable, metav1.ConditionUnknown, "NoDemand", "Zero replicas do not prove spare capacity")
		} else {
			phase(&s, ai.PhaseRunning, "Available", "Current revision replicas are ready and available")
			condition(&s, ai.ConditionCapacityAvailable, metav1.ConditionTrue, "RequestsSatisfied", "Current workload requests are satisfied; this does not indicate spare GPUs")
		}
		return s
	}
	previous := meta.FindStatusCondition(w.Status.Conditions, string(ai.ConditionReady))
	wasAvailable := w.Status.Phase == ai.PhaseRunning || (previous != nil && previous.Reason == "DeploymentUnavailable")
	if n > 0 && wasAvailable && w.Status.ObservedGeneration == w.Generation && d.Status.UpdatedReplicas == n && old == 0 && !draining {
		phase(&s, ai.PhaseDegraded, "DeploymentUnavailable", "Previously available current revision has lost readiness or availability")
	} else {
		phase(&s, ai.PhaseProvisioning, "RollingOut", "Waiting for current revision replicas to become ready and available")
	}
	return s
}

func (r *AIWorkloadReconciler) patchStatus(ctx context.Context, w *ai.AIWorkload, status ai.AIWorkloadStatus) error {
	preserveTransitions(w.Status, &status)
	if equality.Semantic.DeepEqual(w.Status, status) {
		return nil
	}
	before := w.DeepCopy()
	w.Status = status
	return r.Status().Patch(ctx, w, client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{}))
}

func preserveTransitions(previous ai.AIWorkloadStatus, next *ai.AIWorkloadStatus) {
	for i := range next.Conditions {
		old := meta.FindStatusCondition(previous.Conditions, next.Conditions[i].Type)
		if old != nil && old.Status == next.Conditions[i].Status {
			next.Conditions[i].LastTransitionTime = old.LastTransitionTime
		}
	}
}

// Invalid desired state and ownership collisions take precedence over observations.
func (r *AIWorkloadReconciler) status(ctx context.Context, w *ai.AIWorkload, reason, message, endpoint string) error {
	s := baseStatus(w, endpoint)
	phase(&s, ai.PhaseFailed, reason, message)
	condition(&s, ai.ConditionScheduled, metav1.ConditionUnknown, reason, "Scheduling cannot be evaluated")
	condition(&s, ai.ConditionCapacityAvailable, metav1.ConditionUnknown, reason, "Capacity cannot be evaluated")
	return r.patchStatus(ctx, w, s)
}
