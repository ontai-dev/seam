// Package e2e provides shared test helpers for the Seam platform e2e test suites.
//
// No test build tags are required — this package compiles as part of the normal
// module build. All helper types are exported for import by operator e2e suites
// via the seam-core replace directive in each operator's go.mod.
//
// Usage pattern in operator e2e suites:
//
//	import e2ehelpers "github.com/ontai-dev/seam/pkg/e2e"
//
//	var mgmt *e2ehelpers.ClusterClient
//
//	BeforeSuite(func() {
//	    var err error
//	    mgmt, err = e2ehelpers.NewClusterClient("ccs-mgmt", kubeconfigPath)
//	    Expect(err).NotTo(HaveOccurred())
//	})
package e2e

import (
	"fmt"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// ClusterClient bundles a typed Kubernetes client and a dynamic client for a
// named cluster. The name is carried for error context only — it does not affect
// the client configuration.
type ClusterClient struct {
	// Name is the human-readable cluster identifier (e.g. "ccs-mgmt", "ccs-test").
	// Carried for error messages and log context.
	Name string

	// Typed is the standard Kubernetes typed client.
	Typed kubernetes.Interface

	// Dynamic is the dynamic client for unstructured CR operations.
	Dynamic dynamic.Interface
}

// NewClusterClient constructs a ClusterClient from a kubeconfig file path.
// Returns an error if the kubeconfig cannot be loaded or the clients cannot be
// constructed. Does not make any network calls.
func NewClusterClient(name, kubeconfigPath string) (*ClusterClient, error) {
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("e2e: cluster %q: load kubeconfig %q: %w", name, kubeconfigPath, err)
	}

	typed, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("e2e: cluster %q: build typed client: %w", name, err)
	}

	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("e2e: cluster %q: build dynamic client: %w", name, err)
	}

	return &ClusterClient{
		Name:    name,
		Typed:   typed,
		Dynamic: dyn,
	}, nil
}
