package controller

import (
	"context"
	"testing"

	ai "github.com/Dhiraj1729/GpuNodeSelector/api/v1alpha1"
	"github.com/Dhiraj1729/GpuNodeSelector/internal/workload"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func fixture(t *testing.T) (*AIWorkloadReconciler, *ai.AIWorkload) {
	t.Helper()
	s := runtime.NewScheme()
	_ = ai.AddToScheme(s)
	_ = apps.AddToScheme(s)
	_ = core.AddToScheme(s)
	w := &ai.AIWorkload{ObjectMeta: metav1.ObjectMeta{Name: "mock", Namespace: "test", UID: "workload-uid"}, Spec: ai.AIWorkloadSpec{Image: "mock:v1", GPU: ai.GPURequirements{Product: "A100", Count: 1}}}
	return &AIWorkloadReconciler{Client: fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(w).WithObjects(w).Build()}, w
}

type conflictClient struct {
	client.Client
	conflict bool
}

func (c *conflictClient) Patch(ctx context.Context, obj client.Object, p client.Patch, opts ...client.PatchOption) error {
	if c.conflict {
		c.conflict = false
		return apierrors.NewConflict(schema.GroupResource{Group: "apps", Resource: "deployments"}, obj.GetName(), nil)
	}
	return c.Client.Patch(ctx, obj, p, opts...)
}
func TestConflictRetry(t *testing.T) {
	r, w := fixture(t)
	ctx := context.Background()
	req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(w)}
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}

	d := &apps.Deployment{}
	if err := r.Get(ctx, req.NamespacedName, d); err != nil {
		t.Fatal(err)
	}
	d.Spec.Template.Spec.Containers[0].Image = "drift"
	if err := r.Update(ctx, d); err != nil {
		t.Fatal(err)
	}
	c := &conflictClient{Client: r.Client, conflict: true}
	r.Client = c
	if _, err := r.Reconcile(ctx, req); !apierrors.IsConflict(err) {
		t.Fatalf("want propagated conflict: %v", err)
	}
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
}
func TestPodMapping(t *testing.T) {
	r, w := fixture(t)
	ctx := context.Background()
	d, _ := workload.Deployment(w)
	d.UID = "deployment-uid"
	rs := &apps.ReplicaSet{ObjectMeta: metav1.ObjectMeta{Name: "rs", Namespace: w.Namespace, UID: "rs-uid", OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(d, apps.SchemeGroupVersion.WithKind("Deployment"))}}}
	pod := &core.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: w.Namespace, Labels: d.Spec.Template.Labels, Annotations: d.Spec.Template.Annotations, OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(rs, apps.SchemeGroupVersion.WithKind("ReplicaSet"))}}}
	if err := r.Create(ctx, d); err != nil {
		t.Fatal(err)
	}
	if err := r.Create(ctx, rs); err != nil {
		t.Fatal(err)
	}
	if got := r.MapPod(ctx, pod); len(got) != 1 || got[0].NamespacedName != client.ObjectKeyFromObject(w) {
		t.Fatalf("valid mapping: %v", got)
	}
	for _, mutate := range []func(*core.Pod){func(p *core.Pod) { p.Labels[workload.IdentityLabel] = "stale" }, func(p *core.Pod) { p.OwnerReferences[0].UID = "stale" }, func(p *core.Pod) { p.OwnerReferences = nil }, func(p *core.Pod) { p.Annotations[workload.WorkloadNameAnnotation] = "forged" }} {
		p := pod.DeepCopy()
		mutate(p)
		if len(r.MapPod(ctx, p)) != 0 {
			t.Fatal("forged or stale pod mapped")
		}
	}
	rs.OwnerReferences[0].UID = "old-deployment"
	if err := r.Update(ctx, rs); err != nil {
		t.Fatal(err)
	}
	if len(r.MapPod(ctx, pod)) != 0 {
		t.Fatal("stale ReplicaSet chain mapped")
	}
}
func TestMissingAndDeleting(t *testing.T) {
	r, w := fixture(t)
	ctx := context.Background()
	if err := r.Delete(ctx, w); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(w)}); err != nil {
		t.Fatal(err)
	}
	w.ResourceVersion = ""
	w.Finalizers = []string{"test/hold"}
	if err := r.Create(ctx, w); err != nil {
		t.Fatal(err)
	}
	if err := r.Delete(ctx, w); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(w)}); err != nil {
		t.Fatal(err)
	}
	list := &apps.DeploymentList{}
	if err := r.List(ctx, list); err != nil || len(list.Items) != 0 {
		t.Fatal("deleting workload created children")
	}
}
