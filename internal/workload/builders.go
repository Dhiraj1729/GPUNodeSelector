// Package workload constructs desired objects without making Kubernetes API calls.
package workload

import (
	"crypto/sha256"
	"fmt"
	"math/big"
	"strings"

	ai "github.com/Dhiraj1729/GpuNodeSelector/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/utils/ptr"
)

const (
	GPUResource            corev1.ResourceName = "ai.dhiraj.io/gpu"
	GPUNodeLabel                               = "ai.dhiraj.io/gpu-node"
	GPUProductLabel                            = "ai.dhiraj.io/gpu-product"
	GPUTaintKey                                = "ai.dhiraj.io/gpu"
	IdentityLabel                              = "ai.dhiraj.io/workload-id"
	WorkloadNameAnnotation                     = "ai.dhiraj.io/workload-name"
	ContainerName                              = "inference"
	PortName                                   = "http"
)

// ChildName preserves conventional short names and bounds all other valid CR
// names to a DNS-1035 Service name. The suffix distinguishes truncation/collisions.
func ChildName(name string) string {
	if len(validation.IsDNS1035Label(name)) == 0 {
		return name
	}
	hash := sha256.Sum256([]byte(name))
	prefix := strings.ReplaceAll(name, ".", "-")
	if len(prefix) > 40 {
		prefix = prefix[:40]
	}
	return fmt.Sprintf("ai-%s-%x", strings.Trim(prefix, "-"), hash[:6])
}

func validate(w *ai.AIWorkload) (int32, int32, error) {
	if w == nil {
		return 0, 0, fmt.Errorf("AIWorkload is nil")
	}
	if len(validation.IsDNS1123Subdomain(w.Name)) > 0 || w.Name == "" {
		return 0, 0, fmt.Errorf("invalid workload name %q", w.Name)
	}
	if w.Namespace == "" || len(validation.IsDNS1123Label(w.Namespace)) > 0 || w.UID == "" {
		return 0, 0, fmt.Errorf("persisted workload namespace and UID are required")
	}
	if strings.TrimSpace(w.Spec.Image) == "" {
		return 0, 0, fmt.Errorf("image is required")
	}
	if w.Spec.GPU.Count <= 0 || w.Spec.GPU.Product == "" || len(validation.IsValidLabelValue(w.Spec.GPU.Product)) > 0 {
		return 0, 0, fmt.Errorf("positive GPU count and valid product are required")
	}
	replicas, port := int32(1), int32(8080)
	if w.Spec.Replicas != nil {
		replicas = *w.Spec.Replicas
	}
	if w.Spec.Port != nil {
		port = *w.Spec.Port
	}
	if replicas < 0 || port < 1 || port > 65535 {
		return 0, 0, fmt.Errorf("invalid replicas or port")
	}
	if r := w.Spec.Resources; r != nil {
		if r.Memory != nil && r.Memory.Sign() < 0 {
			return 0, 0, fmt.Errorf("memory cannot be negative")
		}
		if r.CPU != nil {
			if r.CPU.Sign() < 0 {
				return 0, 0, fmt.Errorf("CPU cannot be negative")
			}
			cpuCopy := r.CPU.DeepCopy()
			decimal := cpuCopy.AsDec()
			if decimal.Scale() > 3 {
				divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimal.Scale()-3)), nil)
				if new(big.Int).Rem(decimal.UnscaledBig(), divisor).Sign() != 0 {
					return 0, 0, fmt.Errorf("CPU precision must be whole millicores")
				}
			}
		}
	}
	return replicas, port, nil
}

func identity(w *ai.AIWorkload) map[string]string {
	hash := sha256.Sum256([]byte(w.UID))
	return map[string]string{IdentityLabel: fmt.Sprintf("%x", hash[:16])}
}

func metadata(w *ai.AIWorkload) metav1.ObjectMeta {
	return metav1.ObjectMeta{Name: ChildName(w.Name), Namespace: w.Namespace, Labels: identity(w), Annotations: map[string]string{WorkloadNameAnnotation: w.Name}, OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(w, ai.GroupVersion.WithKind("AIWorkload"))}}
}

func probe(path string) *corev1.Probe {
	return &corev1.Probe{ProbeHandler: corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: path, Port: intstr.FromString(PortName)}}, PeriodSeconds: 5, TimeoutSeconds: 2, FailureThreshold: 3}
}

// Deployment builds a fresh desired Deployment. Caller owns API mutation/merging.
func Deployment(w *ai.AIWorkload) (*appsv1.Deployment, error) {
	replicas, port, err := validate(w)
	if err != nil {
		return nil, err
	}
	requests := corev1.ResourceList{GPUResource: *resource.NewQuantity(int64(w.Spec.GPU.Count), resource.DecimalSI)}
	if r := w.Spec.Resources; r != nil {
		if r.CPU != nil {
			requests[corev1.ResourceCPU] = r.CPU.DeepCopy()
		}
		if r.Memory != nil {
			requests[corev1.ResourceMemory] = r.Memory.DeepCopy()
		}
	}
	return &appsv1.Deployment{ObjectMeta: metadata(w), Spec: appsv1.DeploymentSpec{
		Replicas: ptr.To(replicas), Selector: &metav1.LabelSelector{MatchLabels: identity(w)},
		Strategy: appsv1.DeploymentStrategy{Type: appsv1.RollingUpdateDeploymentStrategyType, RollingUpdate: &appsv1.RollingUpdateDeployment{MaxSurge: ptr.To(intstr.FromInt32(0)), MaxUnavailable: ptr.To(intstr.FromInt32(1))}},
		Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: identity(w), Annotations: map[string]string{WorkloadNameAnnotation: w.Name}}, Spec: corev1.PodSpec{
			AutomountServiceAccountToken: ptr.To(false), TerminationGracePeriodSeconds: ptr.To(int64(10)),
			NodeSelector:    map[string]string{GPUNodeLabel: "true", GPUProductLabel: w.Spec.GPU.Product},
			Tolerations:     []corev1.Toleration{{Key: GPUTaintKey, Operator: corev1.TolerationOpEqual, Value: "true", Effect: corev1.TaintEffectNoSchedule}},
			SecurityContext: &corev1.PodSecurityContext{RunAsNonRoot: ptr.To(true), RunAsUser: ptr.To(int64(65532)), RunAsGroup: ptr.To(int64(65532)), SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}},
			Containers: []corev1.Container{{Name: ContainerName, Image: w.Spec.Image, ImagePullPolicy: corev1.PullIfNotPresent,
				Env:           []corev1.EnvVar{{Name: "PORT", Value: fmt.Sprint(port)}},
				Ports:         []corev1.ContainerPort{{Name: PortName, ContainerPort: port, Protocol: corev1.ProtocolTCP}},
				Resources:     corev1.ResourceRequirements{Requests: requests, Limits: corev1.ResourceList{GPUResource: *resource.NewQuantity(int64(w.Spec.GPU.Count), resource.DecimalSI)}},
				LivenessProbe: probe("/healthz"), ReadinessProbe: probe("/readyz"),
				SecurityContext: &corev1.SecurityContext{AllowPrivilegeEscalation: ptr.To(false), ReadOnlyRootFilesystem: ptr.To(true), Capabilities: &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}}},
			}},
		}},
	}}, nil
}

// Service leaves server-allocated fields empty. Day 5 must preserve allocations
// when merging this desired object into an existing Service.
func Service(w *ai.AIWorkload) (*corev1.Service, error) {
	_, port, err := validate(w)
	if err != nil {
		return nil, err
	}
	return &corev1.Service{ObjectMeta: metadata(w), Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP, Selector: identity(w), Ports: []corev1.ServicePort{{Name: PortName, Port: port, TargetPort: intstr.FromString(PortName), Protocol: corev1.ProtocolTCP}}}}, nil
}
