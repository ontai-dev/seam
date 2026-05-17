// Package v1alpha1 contains API types for the seam.ontai.dev/v1alpha1 API group.
//
// This package is the Kubernetes API contract for seam-core. All CRD types that
// seam-core owns are registered here. Breaking changes require a version bump
// and coordination with all operators that reference these types.
//
// +groupName=seam.ontai.dev
// +kubebuilder:object:generate=true
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is the group and version for all types in this package.
	// API group: seam.ontai.dev. INV-008 — this value is ground truth. MIGRATION-3.8.
	GroupVersion = schema.GroupVersion{Group: "seam.ontai.dev", Version: "v1alpha1"}

	// SchemeBuilder is used to add Go types to the Kubernetes runtime scheme.
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds all types in this package to the provided scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)
