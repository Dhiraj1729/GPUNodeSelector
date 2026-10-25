//go:build envtest

package envtest_test

import (
	"context"
	"testing"
	"time"

	ai "github.com/Dhiraj1729/GpuNodeSelector/api/v1alpha1"
	operator "github.com/Dhiraj1729/GpuNodeSelector/internal/controller"
	"github.com/Dhiraj1729/GpuNodeSelector/internal/workload"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metrics "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

type concurrentEditClient struct {
	client.Client
	once bool
}

func (c *concurrentEditClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if _, ok := obj.(*apps.Deployment); ok && !c.once {
		c.once = true
		live := &apps.Deployment{}
		if err := c.Get(ctx, client.ObjectKeyFromObject(obj), live); err != nil {
			return err
		}
		live.Annotations["concurrent/note"] = "keep"
		if err := c.Update(ctx, live); err != nil {
			return err
		}
	}
	return c.Client.Patch(ctx, obj, patch, opts...)
}

func TestReconciliation(t *testing.T) {
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
	scheme := runtime.NewScheme()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(core.AddToScheme(scheme))
	must(apps.AddToScheme(scheme))
	must(ai.AddToScheme(scheme))
	c, err := client.New(cfg, client.Options{Scheme: scheme})
	must(err)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	must(c.Create(ctx, &core.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "reconcile-test"}}))
	w := &ai.AIWorkload{ObjectMeta: metav1.ObjectMeta{Name: "mock", Namespace: "reconcile-test"}, Spec: ai.AIWorkloadSpec{Image: "mock:v1", GPU: ai.GPURequirements{Product: "A100", Count: 1}}}
	must(c.Create(ctx, w))
	key := client.ObjectKeyFromObject(w)
	r := &operator.AIWorkloadReconciler{Client: c}
	run := func() { t.Helper(); _, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: key}); must(err) }
	dep := func() *apps.Deployment { t.Helper(); d := &apps.Deployment{}; must(c.Get(ctx, key, d)); return d }
	svc := func() *core.Service { t.Helper(); s := &core.Service{}; must(c.Get(ctx, key, s)); return s }
	stable := func() {
		t.Helper()
		d, s := dep(), svc()
		must(c.Get(ctx, key, w))
		rv := w.ResourceVersion
		for i := 0; i < 3; i++ {
			run()
		}
		must(c.Get(ctx, key, w))
		if d.ResourceVersion != dep().ResourceVersion || s.ResourceVersion != svc().ResourceVersion || rv != w.ResourceVersion {
			t.Fatal("repeat reconciliation wrote API-defaulted children or status")
		}
	}
	run()
	stable()
	ip := svc().Spec.ClusterIP
	if ip == "" {
		t.Fatal("no Service allocation")
	}
	t.Run("updates-and-metadata", func(t *testing.T) {
		d, s := dep(), svc()
		d.Annotations["user/note"] = "keep"
		d.Spec.Template.Annotations["user/template"] = "keep"
		d.Spec.Template.Spec.NodeSelector["user/pool"] = "custom"
		d.Spec.Template.Spec.Tolerations = append(d.Spec.Template.Spec.Tolerations, core.Toleration{Key: "user/taint", Operator: core.TolerationOpExists})
		d.Spec.Template.Spec.Containers = append(d.Spec.Template.Spec.Containers, core.Container{Name: "sidecar", Image: "sidecar:v1"})
		must(c.Update(ctx, d))
		s.Annotations["user/note"] = "keep"
		must(c.Update(ctx, s))
		must(c.Get(ctx, key, w))
		w.Spec.Image = "mock:v2"
		w.Spec.Replicas = ptr.To(int32(3))
		w.Spec.Port = ptr.To(int32(9090))
		w.Spec.GPU.Count = 3
		w.Spec.GPU.Product = "H100"
		must(c.Update(ctx, w))
		run()
		stable()
		d, s = dep(), svc()
		p := d.Spec.Template.Spec
		gpu := p.Containers[0].Resources.Limits[workload.GPUResource]
		if *d.Spec.Replicas != 3 || p.Containers[0].Image != "mock:v2" || gpu.Value() != 3 || p.NodeSelector[workload.GPUProductLabel] != "H100" || s.Spec.Ports[0].Port != 9090 || s.Spec.ClusterIP != ip || d.Annotations["user/note"] != "keep" || s.Annotations["user/note"] != "keep" || d.Spec.Template.Annotations["user/template"] != "keep" || p.NodeSelector["user/pool"] != "custom" || len(p.Containers) != 2 || len(p.Tolerations) != 2 {
			t.Fatal("update lost desired or unrelated fields")
		}
	})
	t.Run("drift", func(t *testing.T) {
		d := dep()
		d.Spec.Replicas = ptr.To(int32(0))
		d.Spec.Template.Spec.Containers[0].Resources = core.ResourceRequirements{}
		d.Spec.Template.Spec.NodeSelector[workload.GPUProductLabel] = "wrong"
		d.Spec.Template.Spec.Tolerations = nil
		must(c.Update(ctx, d))
		s := svc()
		s.Spec.Selector = map[string]string{"wrong": "selector"}
		must(c.Update(ctx, s))
		run()
		stable()
		d = dep()
		if *d.Spec.Replicas != 3 || d.Spec.Template.Spec.NodeSelector[workload.GPUProductLabel] != "H100" || len(d.Spec.Template.Spec.Tolerations) != 1 || len(d.Spec.Template.Spec.Containers[0].Resources.Limits) != 1 || svc().Spec.Selector[workload.IdentityLabel] != d.Spec.Selector.MatchLabels[workload.IdentityLabel] {
			t.Fatal("drift not repaired")
		}
	})
	t.Run("recreation", func(t *testing.T) {
		oldD, oldS := dep(), svc()
		must(c.Delete(ctx, oldD))
		must(c.Delete(ctx, oldS))
		run()
		if dep().UID == oldD.UID || svc().UID == oldS.UID {
			t.Fatal("children not recreated")
		}
		stable()
	})
	t.Run("ownership-collisions", func(t *testing.T) {
		for _, kind := range []string{"deployment", "service"} {
			for _, owner := range []string{"unowned", "foreign", "stale"} {
				var obj client.Object = dep()
				if kind == "service" {
					obj = svc()
				}
				refs := obj.GetOwnerReferences()
				saved := append([]metav1.OwnerReference(nil), refs...)
				if owner == "unowned" {
					obj.SetOwnerReferences(nil)
				} else {
					refs[0].UID = types.UID(owner)
					obj.SetOwnerReferences(refs)
				}
				must(c.Update(ctx, obj))
				rv := obj.GetResourceVersion()
				_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: key})
				if err == nil {
					t.Fatal("collision accepted")
				}
				must(c.Get(ctx, key, obj))
				if obj.GetResourceVersion() != rv {
					t.Fatal("foreign child changed")
				}
				obj.SetOwnerReferences(saved)
				must(c.Update(ctx, obj))
				run()
			}
		}
		stable()
	})
	t.Run("zero-replicas", func(t *testing.T) {
		must(c.Get(ctx, key, w))
		w.Spec.Replicas = ptr.To(int32(0))
		must(c.Update(ctx, w))
		run()
		if *dep().Spec.Replicas != 0 {
			t.Fatal("zero replicas lost")
		}
		stable()
	})
	t.Run("optimistic-conflict", func(t *testing.T) {
		d := dep()
		d.Spec.Replicas = ptr.To(int32(9))
		must(c.Update(ctx, d))
		r.Client = &concurrentEditClient{Client: c}
		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: key})
		if !apierrors.IsConflict(err) {
			t.Fatalf("expected real API conflict, got %v", err)
		}
		r.Client = c
		run()
		stable()
		if dep().Annotations["concurrent/note"] != "keep" || *dep().Spec.Replicas != 0 {
			t.Fatal("retry lost concurrent metadata or desired replicas")
		}
	})
	t.Run("manager-watches", func(t *testing.T) {
		mgr, err := ctrl.NewManager(cfg, ctrl.Options{Scheme: scheme, Metrics: metrics.Options{BindAddress: "0"}, HealthProbeBindAddress: "0"})
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
				t.Error("manager did not stop")
			}
		}()
		if !mgr.GetCache().WaitForCacheSync(managerCtx) {
			t.Fatal("cache sync failed")
		}
		// Child watch must repair without a manual Reconcile call.
		d := dep()
		d.Spec.Replicas = ptr.To(int32(7))
		must(c.Update(ctx, d))
		deadline := time.Now().Add(10 * time.Second)
		for *dep().Spec.Replicas != 0 {
			if time.Now().After(deadline) {
				t.Fatal("Deployment watch did not repair drift")
			}
			time.Sleep(50 * time.Millisecond)
		}
		must(c.Delete(ctx, svc()))
		deadline = time.Now().Add(10 * time.Second)
		for {
			s := &core.Service{}
			if c.Get(ctx, key, s) == nil {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("Service watch did not recreate child")
			}
			time.Sleep(50 * time.Millisecond)
		}
	})
}
