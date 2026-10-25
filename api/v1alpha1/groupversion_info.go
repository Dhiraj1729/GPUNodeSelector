// Package v1alpha1 contains the ai.dhiraj.io API.
// +kubebuilder:object:generate=true
// +groupName=ai.dhiraj.io
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	SchemeGroupVersion = schema.GroupVersion{Group: "ai.dhiraj.io", Version: "v1alpha1"}
	GroupVersion       = SchemeGroupVersion
	SchemeBuilder      = runtime.NewSchemeBuilder(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &AIWorkload{}, &AIWorkloadList{})
		metav1.AddToGroupVersion(s, GroupVersion)
		return nil
	})
	AddToScheme = SchemeBuilder.AddToScheme
)
