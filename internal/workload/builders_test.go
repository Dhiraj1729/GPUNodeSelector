package workload

import (
	ai "github.com/Dhiraj1729/GpuNodeSelector/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/utils/ptr"
	"reflect"
	"strings"
	"testing"
)

func sample() *ai.AIWorkload {
	return &ai.AIWorkload{ObjectMeta: metav1.ObjectMeta{Name: "mock-llama", Namespace: "ai-workloads", UID: "12345678-1234-1234-1234-123456789abc"}, Spec: ai.AIWorkloadSpec{Image: "mock:dev", GPU: ai.GPURequirements{Product: "A100", Count: 1}}}
}

func TestTranslation(t *testing.T) {
	for _, tc := range []struct {
		name      string
		replicas  *int32
		port      *int32
		count     int32
		product   string
		resources bool
	}{
		{"defaults", nil, nil, 1, "A100", false},
		{"zero", ptr.To(int32(0)), ptr.To(int32(9090)), 2, "H100", true},
		{"above-capacity", ptr.To(int32(3)), ptr.To(int32(8081)), 3, "A100", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := sample()
			w.Spec.Replicas = tc.replicas
			w.Spec.Port = tc.port
			w.Spec.GPU.Count = tc.count
			w.Spec.GPU.Product = tc.product
			if tc.resources {
				cpu, memory := resource.MustParse("100m"), resource.MustParse("128Mi")
				w.Spec.Resources = &ai.WorkloadResources{CPU: &cpu, Memory: &memory}
			}
			before := w.DeepCopy()
			d, err := Deployment(w)
			if err != nil {
				t.Fatal(err)
			}
			s, err := Service(w)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(w, before) {
				t.Fatal("builder mutated input")
			}
			if *d.Spec.Replicas != ptr.Deref(tc.replicas, 1) {
				t.Fatal("replicas lost")
			}
			p := d.Spec.Template.Spec
			c := p.Containers[0]
			if p.NodeName != "" || p.NodeSelector[GPUNodeLabel] != "true" || p.NodeSelector[GPUProductLabel] != tc.product {
				t.Fatal("placement incorrect")
			}
			wantToleration := corev1.Toleration{Key: GPUTaintKey, Operator: corev1.TolerationOpEqual, Value: "true", Effect: corev1.TaintEffectNoSchedule}
			if len(p.Tolerations) != 1 || p.Tolerations[0] != wantToleration {
				t.Fatal("incorrect GPU toleration")
			}
			req, lim := c.Resources.Requests[GPUResource], c.Resources.Limits[GPUResource]
			if req.Value() != int64(tc.count) || req.Cmp(lim) != 0 {
				t.Fatal("GPU request/limit mismatch")
			}
			if len(c.Resources.Limits) != 1 {
				t.Fatal("CPU/memory limits invented")
			}
			if tc.resources {
				cpu, mem := c.Resources.Requests[corev1.ResourceCPU], c.Resources.Requests[corev1.ResourceMemory]
				if cpu.Cmp(*w.Spec.Resources.CPU) != 0 || mem.Cmp(*w.Spec.Resources.Memory) != 0 {
					t.Fatal("requests differ")
				}
			} else if len(c.Resources.Requests) != 1 {
				t.Fatal("optional requests invented")
			}
			if c.Image != w.Spec.Image || c.ImagePullPolicy != corev1.PullIfNotPresent || c.Ports[0].ContainerPort != ptr.Deref(tc.port, 8080) {
				t.Fatal("container translation")
			}
			if c.Env[0].Name != "PORT" {
				t.Fatal("port not configured")
			}
			if c.LivenessProbe.HTTPGet.Path != "/healthz" || c.ReadinessProbe.HTTPGet.Path != "/readyz" || c.ReadinessProbe.HTTPGet.Port.StrVal != PortName {
				t.Fatal("probes incorrect")
			}
			if !*p.SecurityContext.RunAsNonRoot || *p.AutomountServiceAccountToken || *c.SecurityContext.AllowPrivilegeEscalation || !*c.SecurityContext.ReadOnlyRootFilesystem {
				t.Fatal("security context incorrect")
			}
			if d.Spec.Strategy.RollingUpdate.MaxSurge.IntVal != 0 || d.Spec.Strategy.RollingUpdate.MaxUnavailable.IntVal != 1 {
				t.Fatal("saturated rollout can deadlock")
			}
			if !reflect.DeepEqual(d.Spec.Selector.MatchLabels, d.Spec.Template.Labels) || !reflect.DeepEqual(s.Spec.Selector, d.Spec.Template.Labels) {
				t.Fatal("selectors do not match")
			}
			if s.Spec.Type != corev1.ServiceTypeClusterIP || s.Spec.ClusterIP != "" || len(s.Spec.ClusterIPs) != 0 || s.Spec.Ports[0].Port != ptr.Deref(tc.port, 8080) || s.Spec.Ports[0].TargetPort.StrVal != PortName {
				t.Fatal("service translation")
			}
			for _, m := range []metav1.ObjectMeta{d.ObjectMeta, s.ObjectMeta} {
				if m.Namespace != w.Namespace || m.Name != ChildName(w.Name) || len(m.OwnerReferences) != 1 {
					t.Fatal("metadata incorrect")
				}
				o := m.OwnerReferences[0]
				if o.APIVersion != ai.GroupVersion.String() || o.Kind != "AIWorkload" || o.Name != w.Name || o.UID != w.UID || !*o.Controller {
					t.Fatal("ownership incorrect")
				}
			}
		})
	}
}

func TestIdentityAndIsolation(t *testing.T) {
	w := sample()
	cpu := resource.MustParse("100m")
	w.Spec.Resources = &ai.WorkloadResources{CPU: &cpu}
	first, _ := Deployment(w)
	second, _ := Deployment(w)
	if !reflect.DeepEqual(first, second) {
		t.Fatal("nondeterministic build")
	}
	first.Labels[IdentityLabel] = "changed"
	first.Spec.Template.Labels[IdentityLabel] = "changed"
	*first.Spec.Replicas = 9
	first.Spec.Template.Spec.Containers[0].Resources.Requests[corev1.ResourceCPU] = resource.MustParse("2")
	if reflect.DeepEqual(first, second) || second.Spec.Selector.MatchLabels[IdentityLabel] == "changed" || w.Spec.Resources.CPU.String() != "100m" {
		t.Fatal("shared mutable state")
	}
	for _, edit := range []func(*ai.AIWorkload){
		func(w *ai.AIWorkload) { w.Spec.Image = "mock:v2" }, func(w *ai.AIWorkload) { w.Spec.Replicas = ptr.To(int32(0)) }, func(w *ai.AIWorkload) { w.Spec.Port = ptr.To(int32(9000)) }, func(w *ai.AIWorkload) { w.Spec.GPU.Product = "H100"; w.Spec.GPU.Count = 2 },
	} {
		changed := w.DeepCopy()
		edit(changed)
		d, _ := Deployment(changed)
		if !reflect.DeepEqual(d.Spec.Selector, second.Spec.Selector) {
			t.Fatal("selector changed on update")
		}
	}
	changed := w.DeepCopy()
	changed.UID = "replacement-uid"
	d, _ := Deployment(changed)
	if reflect.DeepEqual(d.Spec.Selector, second.Spec.Selector) {
		t.Fatal("new CR UID reused old identity")
	}
}

func TestNames(t *testing.T) {
	for _, name := range []string{"mock-llama", "123-model", "a.b", strings.Repeat("a", 63), strings.Repeat("a", 200)} {
		w := sample()
		w.Name = name
		d, err := Deployment(w)
		if err != nil {
			t.Fatal(err)
		}
		if errs := validation.IsDNS1035Label(d.Name); len(errs) > 0 {
			t.Fatalf("invalid child name %q: %v", d.Name, errs)
		}
		if d.Annotations[WorkloadNameAnnotation] != name {
			t.Fatal("full name lost")
		}
		for _, v := range d.Labels {
			if len(validation.IsValidLabelValue(v)) > 0 {
				t.Fatal("invalid identity label")
			}
		}
	}
	if ChildName("mock-llama") != "mock-llama" || ChildName(strings.Repeat("a", 100)+"x") == ChildName(strings.Repeat("a", 100)+"y") {
		t.Fatal("name derivation incorrect")
	}
}

func TestInvalidInputs(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*ai.AIWorkload)
	}{
		{"uid", func(w *ai.AIWorkload) { w.UID = "" }}, {"namespace", func(w *ai.AIWorkload) { w.Namespace = "" }}, {"name", func(w *ai.AIWorkload) { w.Name = "INVALID" }},
		{"image", func(w *ai.AIWorkload) { w.Spec.Image = "" }}, {"replicas", func(w *ai.AIWorkload) { w.Spec.Replicas = ptr.To(int32(-1)) }}, {"port", func(w *ai.AIWorkload) { w.Spec.Port = ptr.To(int32(0)) }},
		{"count", func(w *ai.AIWorkload) { w.Spec.GPU.Count = 0 }}, {"product", func(w *ai.AIWorkload) { w.Spec.GPU.Product = "bad/product" }},
		{"cpu-precision", func(w *ai.AIWorkload) {
			q := resource.MustParse("0.0001")
			w.Spec.Resources = &ai.WorkloadResources{CPU: &q}
		}},
		{"negative-memory", func(w *ai.AIWorkload) {
			q := resource.MustParse("-1Mi")
			w.Spec.Resources = &ai.WorkloadResources{Memory: &q}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := sample()
			tc.edit(w)
			if _, e := Deployment(w); e == nil {
				t.Fatal("invalid deployment accepted")
			}
			if _, e := Service(w); e == nil {
				t.Fatal("invalid service accepted")
			}
		})
	}
	if _, err := Deployment(nil); err == nil {
		t.Fatal("nil accepted")
	}
}
