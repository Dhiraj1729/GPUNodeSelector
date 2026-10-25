package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"testing"
)

func TestSchemeAndDeepCopy(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if _, err := scheme.New(GroupVersion.WithKind("AIWorkload")); err != nil {
		t.Fatal(err)
	}
	zero := int32(0)
	cpu := resource.MustParse("100m")
	original := &AIWorkload{Spec: AIWorkloadSpec{Replicas: &zero, Resources: &WorkloadResources{CPU: &cpu}}}
	copied := original.DeepCopy()
	*copied.Spec.Replicas = 2
	copied.Spec.Resources.CPU.Set(2)
	if *original.Spec.Replicas != 0 || original.Spec.Resources.CPU.String() != "100m" {
		t.Fatal("deep copy aliases mutable spec fields")
	}
}
