//go:build envtest

package envtest_test

import (
	"context"
	"testing"
	"time"

	ai "github.com/Dhiraj1729/GpuNodeSelector/api/v1alpha1"
	operator "github.com/Dhiraj1729/GpuNodeSelector/internal/controller"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metrics "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

func TestObservedStatus(t *testing.T) {
	environment := &envtest.Environment{CRDDirectoryPaths: []string{"../../config/crd/bases"}, ErrorIfCRDPathMissing: true}
	cfg, err := environment.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := environment.Stop(); err != nil {
			t.Error(err)
		}
	})
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	scheme := runtime.NewScheme()
	must(ai.AddToScheme(scheme))
	must(apps.AddToScheme(scheme))
	must(core.AddToScheme(scheme))
	c, err := client.New(cfg, client.Options{Scheme: scheme})
	must(err)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	must(c.Create(ctx, &core.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "status-observation"}}))
	w := &ai.AIWorkload{ObjectMeta: metav1.ObjectMeta{Name: "mock", Namespace: "status-observation"}, Spec: ai.AIWorkloadSpec{Image: "mock:v1", GPU: ai.GPURequirements{Product: "A100", Count: 1}}}
	must(c.Create(ctx, w))
	key := client.ObjectKeyFromObject(w)
	r := &operator.AIWorkloadReconciler{Client: c}
	run := func() {
		t.Helper()
		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: key})
		must(err)
		must(c.Get(ctx, key, w))
	}
	expect := func(phase ai.Phase, reason string) {
		t.Helper()
		must(c.Get(ctx, key, w))
		condition := meta.FindStatusCondition(w.Status.Conditions, "Ready")
		if w.Status.Phase != phase || condition == nil || condition.Reason != reason || w.Status.ObservedGeneration != w.Generation {
			t.Fatalf("status=%+v", w.Status)
		}
	}
	stable := func() {
		t.Helper()
		must(c.Get(ctx, key, w))
		rv := w.ResourceVersion
		for i := 0; i < 3; i++ {
			run()
		}
		if w.ResourceVersion != rv {
			t.Fatal("unchanged observations wrote status")
		}
	}
	run()
	expect(ai.PhaseProvisioning, "AwaitingObservation")
	stable()
	d := &apps.Deployment{}
	must(c.Get(ctx, key, d))
	rs := &apps.ReplicaSet{ObjectMeta: metav1.ObjectMeta{Name: "current", Namespace: w.Namespace, OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(d, apps.SchemeGroupVersion.WithKind("Deployment"))}}, Spec: apps.ReplicaSetSpec{Replicas: ptr.To(int32(1)), Selector: d.Spec.Selector.DeepCopy(), Template: *d.Spec.Template.DeepCopy()}}
	rs.Spec.Template.Labels[apps.DefaultDeploymentUniqueLabelKey] = "test-hash"
	must(c.Create(ctx, rs))
	p := &core.Pod{ObjectMeta: metav1.ObjectMeta{Name: "current-pod", Namespace: w.Namespace, Labels: rs.Spec.Template.Labels, Annotations: rs.Spec.Template.Annotations, OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(rs, apps.SchemeGroupVersion.WithKind("ReplicaSet"))}}, Spec: *rs.Spec.Template.Spec.DeepCopy()}
	must(c.Create(ctx, p))
	setPod := func(status core.PodStatus) {
		t.Helper()
		must(c.Get(ctx, client.ObjectKeyFromObject(p), p))
		p.Status = status
		must(c.Status().Update(ctx, p))
	}
	ready := core.PodStatus{Phase: core.PodRunning, Conditions: []core.PodCondition{{Type: core.PodScheduled, Status: core.ConditionTrue}, {Type: core.PodReady, Status: core.ConditionTrue}}}
	setPod(ready)
	d.Status = apps.DeploymentStatus{ObservedGeneration: d.Generation, Replicas: 1, UpdatedReplicas: 1, ReadyReplicas: 1, AvailableReplicas: 1}
	must(c.Status().Update(ctx, d))
	run()
	expect(ai.PhaseRunning, "Available")
	stable()
	t.Run("capacity-and-mixed", func(t *testing.T) {
		pending := core.PodStatus{Phase: core.PodPending, Conditions: []core.PodCondition{{Type: core.PodScheduled, Status: core.ConditionFalse, Reason: core.PodReasonUnschedulable, Message: "0/1 nodes are available: 1 Insufficient ai.dhiraj.io/gpu."}}}
		setPod(pending)
		run()
		expect(ai.PhaseCapacityBlocked, "InsufficientGPU")
		if w.Status.PendingReplicas != 1 {
			t.Fatal("pending count")
		}
		stable()
		previous := meta.FindStatusCondition(w.Status.Conditions, "Ready").LastTransitionTime
		pending.Conditions[0].Message = "0/2 nodes are available: 1 Insufficient ai.dhiraj.io/gpu, 1 Insufficient cpu."
		setPod(pending)
		run()
		expect(ai.PhasePending, "MultipleSchedulingConstraints")
		stable()
		if meta.FindStatusCondition(w.Status.Conditions, "Ready").LastTransitionTime != previous {
			t.Fatal("false-to-false timestamp churn")
		}
		setPod(ready)
		run()
		expect(ai.PhaseRunning, "Available")
	})
	t.Run("stale-generation", func(t *testing.T) {
		must(c.Get(ctx, key, d))
		d.Status.ObservedGeneration = 0
		must(c.Status().Update(ctx, d))
		run()
		expect(ai.PhaseProvisioning, "AwaitingObservation")
		stable()
		must(c.Get(ctx, key, d))
		d.Status.ObservedGeneration = d.Generation
		must(c.Status().Update(ctx, d))
		run()
		expect(ai.PhaseRunning, "Available")
	})
	t.Run("pod-watch-readiness", func(t *testing.T) {
		// Sequential tests use separate API servers but share a process-global name registry.
		mgr, err := ctrl.NewManager(cfg, ctrl.Options{Scheme: scheme, Metrics: metrics.Options{BindAddress: "0"}, HealthProbeBindAddress: "0", Controller: controllerconfig.Controller{SkipNameValidation: ptr.To(true)}})
		must(err)
		must((&operator.AIWorkloadReconciler{Client: mgr.GetClient()}).SetupWithManager(mgr))
		managerCtx, stop := context.WithCancel(ctx)
		done := make(chan error, 1)
		go func() { done <- mgr.Start(managerCtx) }()
		defer func() {
			stop()
			select {
			case err := <-done:
				must(err)
			case <-time.After(10 * time.Second):
				t.Error("manager shutdown timeout")
			}
		}()
		if !mgr.GetCache().WaitForCacheSync(managerCtx) {
			t.Fatal("cache sync failed")
		}
		// Only a Pod status event changes; Deployment status is intentionally unchanged.
		notReady := *ready.DeepCopy()
		notReady.Conditions[1].Status = core.ConditionFalse
		setPod(notReady)
		deadline := time.Now().Add(8 * time.Second)
		for {
			must(c.Get(ctx, key, w))
			if w.Status.Phase == ai.PhaseDegraded {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("Pod watch did not update status")
			}
			time.Sleep(50 * time.Millisecond)
		}
		expect(ai.PhaseDegraded, "DeploymentUnavailable")
		setPod(ready)
		deadline = time.Now().Add(8 * time.Second)
		for {
			must(c.Get(ctx, key, w))
			if w.Status.Phase == ai.PhaseRunning {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("readiness recovery not observed")
			}
			time.Sleep(50 * time.Millisecond)
		}
		expect(ai.PhaseRunning, "Available")
	})
}
