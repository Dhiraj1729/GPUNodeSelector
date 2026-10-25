package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AIWorkloadSpec expresses workload requirements, never a node assignment.
type AIWorkloadSpec struct {
	// Image is required; admission does not check registry availability.
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`
	// Replicas preserves explicit zero; omission defaults at API admission.
	// +optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	Replicas *int32 `json:"replicas,omitempty"`
	// +optional
	// +kubebuilder:default=8080
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port *int32          `json:"port,omitempty"`
	GPU  GPURequirements `json:"gpu"`
	// +optional
	Resources *WorkloadResources `json:"resources,omitempty"`
}

type GPURequirements struct {
	// Product must fit a nonempty Kubernetes label value.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9]([-a-zA-Z0-9_.]*[a-zA-Z0-9])?$`
	Product string `json:"product"`
	// Count has no capacity-based maximum; the scheduler decides feasibility.
	// +kubebuilder:validation:Minimum=1
	Count int32 `json:"count"`
}

type WorkloadResources struct {
	// CPU is a request in decimal cores (up to three fractional digits) or integer millicores.
	// +optional
	// +kubebuilder:validation:XValidation:rule="string(self).matches('^[0-9]+([.][0-9]{1,3})?m?$') && (!string(self).endsWith('m') || !string(self).contains('.'))",message="CPU must use nonnegative decimal cores with at most 3 fractional digits or integer millicores"
	CPU *resource.Quantity `json:"cpu,omitempty"`
	// Memory is a nonnegative Kubernetes quantity, including binary suffixes.
	// +optional
	// +kubebuilder:validation:XValidation:rule="isQuantity(string(self)) ? quantity(string(self)).compareTo(quantity('0')) >= 0 : false",message="memory must be a valid nonnegative Kubernetes quantity"
	Memory *resource.Quantity `json:"memory,omitempty"`
}

// Phase summarizes observed state; conditions carry detailed reasons.
// +kubebuilder:validation:Enum=Pending;Provisioning;Running;Degraded;CapacityBlocked;Failed
type Phase string

const (
	PhasePending         Phase = "Pending"
	PhaseProvisioning    Phase = "Provisioning"
	PhaseRunning         Phase = "Running"
	PhaseDegraded        Phase = "Degraded"
	PhaseCapacityBlocked Phase = "CapacityBlocked"
	PhaseFailed          Phase = "Failed"
)

type ConditionType string

const (
	ConditionReady             ConditionType = "Ready"
	ConditionProgressing       ConditionType = "Progressing"
	ConditionScheduled         ConditionType = "Scheduled"
	ConditionCapacityAvailable ConditionType = "CapacityAvailable"
)

type AIWorkloadStatus struct {
	// +optional
	// +kubebuilder:validation:Minimum=0
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// +optional
	Phase Phase `json:"phase,omitempty"`
	// +optional
	// +kubebuilder:validation:Minimum=0
	DesiredReplicas int32 `json:"desiredReplicas,omitempty"`
	// +optional
	// +kubebuilder:validation:Minimum=0
	UpdatedReplicas int32 `json:"updatedReplicas,omitempty"`
	// +optional
	// +kubebuilder:validation:Minimum=0
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`
	// +optional
	// +kubebuilder:validation:Minimum=0
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`
	// +optional
	// +kubebuilder:validation:Minimum=0
	PendingReplicas int32 `json:"pendingReplicas,omitempty"`
	// +optional
	Endpoint string `json:"endpoint,omitempty"`
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Desired",type=integer,JSONPath=`.spec.replicas`
// +kubebuilder:printcolumn:name="Ready",type=integer,JSONPath=`.status.readyReplicas`
// +kubebuilder:printcolumn:name="Pending",type=integer,JSONPath=`.status.pendingReplicas`
type AIWorkload struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              AIWorkloadSpec `json:"spec"`
	// +optional
	Status AIWorkloadStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type AIWorkloadList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AIWorkload `json:"items"`
}
