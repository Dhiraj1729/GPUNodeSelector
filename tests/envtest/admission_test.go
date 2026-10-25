//go:build envtest

package envtest_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	ai "github.com/Dhiraj1729/GpuNodeSelector/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func TestAdmission(t *testing.T) {
	if os.Getenv("KUBEBUILDER_ASSETS") == "" {
		t.Fatal("KUBEBUILDER_ASSETS is required; run make envtest")
	}
	environment := &envtest.Environment{CRDDirectoryPaths: []string{filepath.Join("..", "..", "config", "crd", "bases")}, ErrorIfCRDPathMissing: true}
	t.Cleanup(func() {
		if err := environment.Stop(); err != nil {
			t.Errorf("stop envtest: %v", err)
		}
	})
	cfg, err := environment.Start()
	if err != nil {
		t.Fatalf("start API server and install CRD: %v", err)
	}
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := ai.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	c, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "admission-test"}}
	if err := c.Create(ctx, ns); err != nil {
		t.Fatal(err)
	}
	// Raw objects exercise wire values that Go int32/resource.Quantity cannot represent.
	cases := []struct {
		name  string
		field string
		value any
		omit  bool
		valid bool
	}{
		{name: "minimal", valid: true},
		{name: "zero-replicas", field: "replicas", value: int64(0), valid: true},
		{name: "negative-replicas", field: "replicas", value: int64(-1)},
		{name: "fractional-replicas", field: "replicas", value: 1.5},
		{name: "port-zero", field: "port", value: int64(0)},
		{name: "port-too-large", field: "port", value: int64(65536)},
		{name: "port-upper-bound", field: "port", value: int64(65535), valid: true},
		{name: "missing-image", field: "image", omit: true},
		{name: "empty-image", field: "image", value: ""},
		{name: "missing-gpu", field: "gpu", omit: true},
		{name: "missing-product", field: "gpu.product", omit: true},
		{name: "empty-product", field: "gpu.product", value: ""},
		{name: "invalid-product", field: "gpu.product", value: "A100/invalid"},
		{name: "long-product", field: "gpu.product", value: strings.Repeat("x", 64)},
		{name: "missing-count", field: "gpu.count", omit: true},
		{name: "zero-count", field: "gpu.count", value: int64(0)},
		{name: "negative-count", field: "gpu.count", value: int64(-1)},
		{name: "fractional-count", field: "gpu.count", value: 1.5},
		{name: "above-capacity", field: "gpu.count", value: int64(3), valid: true},
		{name: "empty-resources", field: "resources", value: map[string]any{}, valid: true},
		{name: "cpu-milli", field: "resources.cpu", value: "100m", valid: true},
		{name: "cpu-decimal", field: "resources.cpu", value: "0.001", valid: true},
		{name: "cpu-integer", field: "resources.cpu", value: int64(1), valid: true},
		{name: "cpu-zero", field: "resources.cpu", value: "0", valid: true},
		{name: "cpu-negative", field: "resources.cpu", value: "-100m"},
		{name: "cpu-malformed", field: "resources.cpu", value: "bad"},
		{name: "cpu-too-precise", field: "resources.cpu", value: "0.0001"},
		{name: "cpu-fractional-milli", field: "resources.cpu", value: "0.1m"},
		{name: "cpu-exponent", field: "resources.cpu", value: "1e-3"},
		{name: "memory-binary", field: "resources.memory", value: "128Mi", valid: true},
		{name: "memory-decimal", field: "resources.memory", value: "1G", valid: true},
		{name: "memory-zero", field: "resources.memory", value: int64(0), valid: true},
		{name: "memory-negative", field: "resources.memory", value: "-1Mi"},
		{name: "memory-malformed", field: "resources.memory", value: "wrong"},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u := &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "ai.dhiraj.io/v1alpha1", "kind": "AIWorkload",
				"metadata": map[string]any{"name": fmt.Sprintf("case-%d", i), "namespace": ns.Name},
				"spec":     map[string]any{"image": "example/mock:dev", "gpu": map[string]any{"product": "A100", "count": int64(1)}},
			}}
			if tc.field != "" {
				path := append([]string{"spec"}, strings.Split(tc.field, ".")...)
				if tc.omit {
					unstructured.RemoveNestedField(u.Object, path...)
				} else if err := unstructured.SetNestedField(u.Object, tc.value, path...); err != nil {
					t.Fatal(err)
				}
			}
			err := c.Create(ctx, u)
			if !tc.valid {
				if !apierrors.IsInvalid(err) {
					t.Fatalf("want admission Invalid, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			stored := &ai.AIWorkload{}
			if err := c.Get(ctx, client.ObjectKeyFromObject(u), stored); err != nil {
				t.Fatal(err)
			}
			expectedReplicas := int32(1)
			if tc.name == "zero-replicas" {
				expectedReplicas = 0
			}
			if stored.Spec.Replicas == nil || *stored.Spec.Replicas != expectedReplicas {
				t.Fatalf("bad replicas: %v", stored.Spec.Replicas)
			}
			if tc.field != "port" && (stored.Spec.Port == nil || *stored.Spec.Port != 8080) {
				t.Fatalf("bad port: %v", stored.Spec.Port)
			}
		})
	}
	t.Run("status-subresource", func(t *testing.T) {
		obj := &ai.AIWorkload{ObjectMeta: metav1.ObjectMeta{Name: "status-test", Namespace: ns.Name}, Spec: ai.AIWorkloadSpec{Image: "example/mock:dev", GPU: ai.GPURequirements{Product: "A100", Count: 1}}, Status: ai.AIWorkloadStatus{Phase: ai.PhaseRunning}}
		if err := c.Create(ctx, obj); err != nil {
			t.Fatal(err)
		}
		key := client.ObjectKeyFromObject(obj)
		if err := c.Get(ctx, key, obj); err != nil {
			t.Fatal(err)
		}
		if obj.Status.Phase != "" {
			t.Fatal("create must ignore status")
		}
		generation := obj.Generation
		obj.Status = ai.AIWorkloadStatus{Phase: ai.PhasePending, ObservedGeneration: generation, Conditions: []metav1.Condition{{Type: string(ai.ConditionReady), Status: metav1.ConditionFalse, Reason: "AwaitingController", Message: "API-only milestone", ObservedGeneration: generation, LastTransitionTime: metav1.Now()}}}
		obj.Spec.Image = "must-not-change"
		if err := c.Status().Update(ctx, obj); err != nil {
			t.Fatal(err)
		}
		if err := c.Get(ctx, key, obj); err != nil {
			t.Fatal(err)
		}
		if obj.Spec.Image != "example/mock:dev" || obj.Status.Phase != ai.PhasePending || obj.Generation != generation {
			t.Fatal("status subresource changed spec/generation or lost status")
		}
		if err := c.Patch(ctx, obj, client.RawPatch(types.MergePatchType, []byte(`{"spec":{"replicas":2},"status":{"phase":"Running"}}`))); err != nil {
			t.Fatal(err)
		}
		if err := c.Get(ctx, key, obj); err != nil {
			t.Fatal(err)
		}
		if obj.Status.Phase != ai.PhasePending || *obj.Spec.Replicas != 2 || obj.Generation <= generation {
			t.Fatal("main resource update did not isolate status")
		}
		obj.Status.Phase = ai.Phase("InvalidPhase")
		if err := c.Status().Update(ctx, obj); !apierrors.IsInvalid(err) {
			t.Fatalf("invalid phase accepted: %v", err)
		}
	})
}
