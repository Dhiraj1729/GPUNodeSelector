package helm_test

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	ai "github.com/Dhiraj1729/GpuNodeSelector/api/v1alpha1"
	"github.com/Dhiraj1729/GpuNodeSelector/internal/workload"
	appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func render(t *testing.T, chart string, args ...string) []byte {
	t.Helper()
	a := append([]string{"template", "test", "../../charts/" + chart, "--namespace", "ai-system"}, args...)
	out, err := exec.Command("helm", a...).CombinedOutput()
	if err != nil {
		t.Fatalf("helm: %v\n%s", err, out)
	}
	return out
}
func objects(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	result := map[string][]byte{}
	dec := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
	for {
		var raw map[string]interface{}
		err := dec.Decode(&raw)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if raw == nil {
			continue
		}
		// Decode through JSON-compatible YAML for typed comparisons below.
		kind := raw["kind"].(string)
		encoded, err := json.Marshal(raw)
		if err != nil {
			t.Fatal(err)
		}
		result[kind] = encoded
	}
	return result
}
func TestPlatformContracts(t *testing.T) {
	got := objects(t, render(t, "gpu-node-selector", "--include-crds"))
	for _, kind := range []string{"CustomResourceDefinition", "Deployment", "DaemonSet", "ServiceAccount", "ClusterRole", "ClusterRoleBinding"} {
		if got[kind] == nil {
			t.Fatalf("missing %s", kind)
		}
	}
	if len(got) != 6 {
		t.Fatalf("unexpected kinds: %v", got)
	}
	var deploy appsv1.Deployment
	if err := json.Unmarshal(got["Deployment"], &deploy); err != nil {
		t.Fatal(err)
	}
	var plugin appsv1.DaemonSet
	if err := json.Unmarshal(got["DaemonSet"], &plugin); err != nil {
		t.Fatal(err)
	}
	// Keep scheduling, security, probes and resources identical to the proven manifests.
	for _, tc := range []struct {
		path, kind string
		value      interface{}
	}{
		{"config/manager/manager.yaml", "Deployment", &appsv1.Deployment{}},
		{"config/fake-device-plugin/daemonset.yaml", "DaemonSet", &appsv1.DaemonSet{}},
		{"config/rbac/role.yaml", "ClusterRole", &rbacv1.ClusterRole{}},
	} {
		data, err := os.ReadFile(filepath.Join("../..", tc.path))
		if err != nil {
			t.Fatal(err)
		}
		original := objects(t, data)[tc.kind]
		if err := json.Unmarshal(original, tc.value); err != nil {
			t.Fatal(err)
		}
		switch v := tc.value.(type) {
		case *appsv1.Deployment:
			deploy.Spec.Template.Annotations = nil
			if !reflect.DeepEqual(v.Spec, deploy.Spec) {
				t.Error("operator spec drifted from baseline")
			}
		case *appsv1.DaemonSet:
			plugin.Spec.Template.Annotations = nil
			if !reflect.DeepEqual(v.Spec, plugin.Spec) {
				t.Error("plugin spec drifted from baseline")
			}
		case *rbacv1.ClusterRole:
			var role rbacv1.ClusterRole
			if err := json.Unmarshal(got["ClusterRole"], &role); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(v.Rules, role.Rules) {
				t.Error("chart RBAC differs from generated rules")
			}
		}
	}
}
func TestWorkloadContract(t *testing.T) {
	for _, replicas := range []string{"0", "2", "3"} {
		data := objects(t, render(t, "aiworkloads", "--set", "replicas="+replicas))
		if len(data) != 1 || data["AIWorkload"] == nil {
			t.Fatal("Helm must own only the AIWorkload")
		}
		var w ai.AIWorkload
		if err := json.Unmarshal(data["AIWorkload"], &w); err != nil {
			t.Fatal(err)
		}
		if w.Spec.Replicas == nil {
			t.Fatal("replicas missing")
		}
		w.UID = "helm-test-api-assigned-uid"
		// Exercise actual operator builders against chart output.
		if _, err := workload.Deployment(&w); err != nil {
			t.Fatal(err)
		}
	}
}
func TestInvalidValues(t *testing.T) {
	for _, value := range []string{"replicas=-1", "gpu.count=0", "gpu.product=bad/product", "port=65536", "name=1invalid", "nodeSelector.foo=bar"} {
		cmd := exec.Command("helm", "template", "test", "../../charts/aiworkloads", "--set", value)
		if out, err := cmd.CombinedOutput(); err == nil {
			t.Fatalf("accepted %s: %s", value, out)
		}
	}
}

func TestPluginDeviceCount(t *testing.T) {
	for _, count := range []string{"1", "2", "4", "256"} {
		t.Run(count, func(t *testing.T) {
			got := objects(t, render(t, "gpu-node-selector", "--set", "plugin.devices="+count))
			var plugin appsv1.DaemonSet
			if err := json.Unmarshal(got["DaemonSet"], &plugin); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(plugin.Spec.Template.Spec.Containers[0].Args, []string{"--devices=" + count}) {
				t.Fatalf("unexpected device args: %v", plugin.Spec.Template.Spec.Containers[0].Args)
			}
		})
	}
	for _, value := range []string{"0", "-1", "257", "1.5", "invalid", "true", "null"} {
		cmd := exec.Command("helm", "template", "test", "../../charts/gpu-node-selector", "--set", "plugin.devices="+value)
		if out, err := cmd.CombinedOutput(); err == nil {
			t.Fatalf("accepted invalid device count %s: %s", value, out)
		}
	}
}
