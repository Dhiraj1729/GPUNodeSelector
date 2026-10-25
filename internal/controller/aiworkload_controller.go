package controller

import (
	"context"
	"fmt"

	ai "github.com/Dhiraj1729/GpuNodeSelector/api/v1alpha1"
	"github.com/Dhiraj1729/GpuNodeSelector/internal/workload"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	equality "k8s.io/apimachinery/pkg/api/equality"
	errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// AIWorkloadReconciler leaves scheduling and GPU allocation to Kubernetes.
type AIWorkloadReconciler struct{ client.Client }

// +kubebuilder:rbac:groups=ai.dhiraj.io,resources=aiworkloads,verbs=get;list;watch
// +kubebuilder:rbac:groups=ai.dhiraj.io,resources=aiworkloads/status,verbs=get;patch;update
// +kubebuilder:rbac:groups=ai.dhiraj.io,resources=aiworkloads/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;patch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;patch
// +kubebuilder:rbac:groups=apps,resources=replicasets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch

func (r *AIWorkloadReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	w := &ai.AIWorkload{}
	if err := r.Get(ctx, req.NamespacedName, w); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !w.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}
	log := ctrl.LoggerFrom(ctx).WithValues("generation", w.Generation)
	d, err := workload.Deployment(w)
	if err != nil {
		return ctrl.Result{}, r.status(ctx, w, "InvalidSpec", err.Error(), "")
	}
	s, err := workload.Service(w)
	if err != nil {
		return ctrl.Result{}, err
	}
	for _, desired := range []client.Object{d, s} {
		outcome, err := r.reconcileChild(ctx, w, desired)
		if err != nil {
			if _, ok := err.(*ownershipConflict); ok {
				if statusErr := r.status(ctx, w, "OwnershipConflict", err.Error(), ""); statusErr != nil {
					return ctrl.Result{}, statusErr
				}
			}
			return ctrl.Result{}, err
		}
		log.V(1).Info("child reconciled", "kind", fmt.Sprintf("%T", desired), "name", desired.GetName(), "outcome", outcome)
	}
	return r.observeStatus(ctx, w, d, s)
}

type ownershipConflict struct{ name string }

func (e *ownershipConflict) Error() string { return "ownership conflict: refusing to adopt " + e.name }

func controlledBy(obj metav1.Object, owner metav1.Object, kind, version string) bool {
	ref := metav1.GetControllerOf(obj)
	return ref != nil && ref.UID == owner.GetUID() && ref.Name == owner.GetName() && ref.Kind == kind && ref.APIVersion == version
}

// Explicit Create-or-Patch with optimistic locking: concurrent edits produce a
// conflict, which controller-runtime retries with a fresh read and backoff.
func (r *AIWorkloadReconciler) reconcileChild(ctx context.Context, w *ai.AIWorkload, desired client.Object) (string, error) {
	current := desired.DeepCopyObject().(client.Object)
	if err := r.Get(ctx, client.ObjectKeyFromObject(desired), current); err != nil {
		if !errors.IsNotFound(err) {
			return "", err
		}
		return "created", r.Create(ctx, desired)
	}
	if !controlledBy(current, w, "AIWorkload", ai.GroupVersion.String()) {
		return "", &ownershipConflict{name: fmt.Sprintf("%T %s/%s", current, current.GetNamespace(), current.GetName())}
	}
	if !current.GetDeletionTimestamp().IsZero() {
		return "", fmt.Errorf("child %s is terminating", current.GetName())
	}
	before := current.DeepCopyObject().(client.Object)
	mergeMetadata(current, desired)
	switch obj := current.(type) {
	case *apps.Deployment:
		if err := mergeDeployment(obj, desired.(*apps.Deployment)); err != nil {
			return "", err
		}
	case *core.Service:
		want := desired.(*core.Service)
		obj.Spec.Type = want.Spec.Type
		obj.Spec.Selector = want.Spec.Selector
		obj.Spec.Ports = want.Spec.Ports
	}
	if equality.Semantic.DeepEqual(before, current) {
		return "unchanged", nil
	}
	return "patched", r.Patch(ctx, current, client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{}))
}

func mergeMetadata(dst, src metav1.Object) {
	labels := dst.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	for k, v := range src.GetLabels() {
		labels[k] = v
	}
	dst.SetLabels(labels)
	annotations := dst.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	for k, v := range src.GetAnnotations() {
		annotations[k] = v
	}
	dst.SetAnnotations(annotations)
	// Keep non-controller references; repair the managed controller reference.
	refs := dst.GetOwnerReferences()
	for i := range refs {
		if refs[i].Controller != nil && *refs[i].Controller {
			refs[i] = src.GetOwnerReferences()[0]
		}
	}
	dst.SetOwnerReferences(refs)
}

func mergeDeployment(dst, want *apps.Deployment) error {
	if !equality.Semantic.DeepEqual(dst.Spec.Selector, want.Spec.Selector) {
		return fmt.Errorf("Deployment %s has incompatible immutable selector", dst.Name)
	}
	dst.Spec.Replicas = want.Spec.Replicas
	dst.Spec.Strategy = want.Spec.Strategy
	// Template has no owner references, so merge its maps independently.
	if dst.Spec.Template.Labels == nil {
		dst.Spec.Template.Labels = map[string]string{}
	}
	for k, v := range want.Spec.Template.Labels {
		dst.Spec.Template.Labels[k] = v
	}
	if dst.Spec.Template.Annotations == nil {
		dst.Spec.Template.Annotations = map[string]string{}
	}
	for k, v := range want.Spec.Template.Annotations {
		dst.Spec.Template.Annotations[k] = v
	}
	p, q := &dst.Spec.Template.Spec, &want.Spec.Template.Spec
	p.NodeName = ""
	p.SchedulerName = core.DefaultSchedulerName
	p.AutomountServiceAccountToken = q.AutomountServiceAccountToken
	p.TerminationGracePeriodSeconds = q.TerminationGracePeriodSeconds
	p.SecurityContext = q.SecurityContext
	if p.NodeSelector == nil {
		p.NodeSelector = map[string]string{}
	}
	for k, v := range q.NodeSelector {
		p.NodeSelector[k] = v
	}
	tolerations := make([]core.Toleration, 0, len(p.Tolerations)+1)
	for _, t := range p.Tolerations {
		if t.Key != workload.GPUTaintKey {
			tolerations = append(tolerations, t)
		}
	}
	p.Tolerations = append(tolerations, q.Tolerations...)
	// Preserve sidecars. The inference container is wholly operator-managed.
	c := q.Containers[0]
	c.TerminationMessagePath = core.TerminationMessagePathDefault
	c.TerminationMessagePolicy = core.TerminationMessageReadFile
	for _, probe := range []*core.Probe{c.LivenessProbe, c.ReadinessProbe} {
		probe.SuccessThreshold = 1
		probe.HTTPGet.Scheme = core.URISchemeHTTP
	}
	found := false
	for i := range p.Containers {
		if p.Containers[i].Name == c.Name {
			p.Containers[i] = c
			found = true
		}
	}
	if !found {
		p.Containers = append(p.Containers, c)
	}
	return nil
}

// MapPod verifies both the immutable identity label and complete current owner chain.
func (r *AIWorkloadReconciler) MapPod(ctx context.Context, obj client.Object) []reconcile.Request {
	pod, ok := obj.(*core.Pod)
	if !ok {
		return nil
	}
	name := pod.Annotations[workload.WorkloadNameAnnotation]
	if name == "" {
		return nil
	}
	w := &ai.AIWorkload{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: pod.Namespace, Name: name}, w); err != nil {
		return nil
	}
	desired, err := workload.Deployment(w)
	if err != nil || !w.DeletionTimestamp.IsZero() || pod.Labels[workload.IdentityLabel] != desired.Labels[workload.IdentityLabel] {
		return nil
	}
	ref := metav1.GetControllerOf(pod)
	if ref == nil || ref.Kind != "ReplicaSet" || ref.APIVersion != apps.SchemeGroupVersion.String() {
		return nil
	}
	rs := &apps.ReplicaSet{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: pod.Namespace, Name: ref.Name}, rs); err != nil || !controlledBy(pod, rs, "ReplicaSet", apps.SchemeGroupVersion.String()) {
		return nil
	}
	dep := &apps.Deployment{}
	if err := r.Get(ctx, client.ObjectKeyFromObject(desired), dep); err != nil || !controlledBy(rs, dep, "Deployment", apps.SchemeGroupVersion.String()) || !controlledBy(dep, w, "AIWorkload", ai.GroupVersion.String()) {
		return nil
	}
	return []reconcile.Request{{NamespacedName: client.ObjectKeyFromObject(w)}}
}

func (r *AIWorkloadReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(&ai.AIWorkload{}).Owns(&apps.Deployment{}).Owns(&core.Service{}).Watches(&core.Pod{}, handler.EnqueueRequestsFromMapFunc(r.MapPod)).Complete(r)
}
