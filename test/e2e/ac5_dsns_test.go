package e2e_test

// AC-5: DSNS lineage tracking in seam.ontave.dev acceptance contract.
//
// Scenario: After seam-core is installed on ccs-mgmt and DSNSReconciler is running,
// root declaration CR state must be projected to the seam.ontave.dev zone within
// the dsns-zone ConfigMap in ont-system. Each CRD family must produce a correctly
// typed DNS record:
//
//   - TalosCluster Ready        -> A record at {cluster}.seam.ontave.dev
//   - PackInstance Ready/Succeeded -> pack.{pack}.{version}.wrapper.{cluster} TXT
//   - IdentityBinding resolved  -> identity.{hash}.guardian.{cluster} TXT
//   - IdentityProvider Valid    -> idp.{name}.guardian TXT
//   - RunnerConfig terminal     -> run.{cluster} TXT
//
// Zone must contain $ORIGIN seam.ontave.dev., SOA, and NS records.
//
// Promotion condition: requires live cluster with MGMT_KUBECONFIG and
// TENANT-CLUSTER-E2E closed (ccs-dev onboarded as tenant cluster).
//
// seam-core-schema.md §8 Decisions 1 and 4.

import (
	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("AC-5: DSNS lineage tracking in seam.ontave.dev", func() {
	It("Ready TalosCluster ccs-mgmt produces A record in seam.ontave.dev zone", func() {
		Skip("requires live cluster with MGMT_KUBECONFIG and TENANT-CLUSTER-E2E closed")
	})

	It("Ready PackInstance produces pack-lineage TXT record in seam.ontave.dev zone", func() {
		Skip("requires live cluster with MGMT_KUBECONFIG and TENANT-CLUSTER-E2E closed")
	})

	It("Resolved IdentityBinding produces identity-plane TXT record in seam.ontave.dev zone", func() {
		Skip("requires live cluster with MGMT_KUBECONFIG and TENANT-CLUSTER-E2E closed")
	})

	It("Valid IdentityProvider produces idp TXT record in seam.ontave.dev zone", func() {
		Skip("requires live cluster with MGMT_KUBECONFIG and TENANT-CLUSTER-E2E closed")
	})

	It("RunnerConfig with capabilities produces execution-authority TXT record in seam.ontave.dev zone", func() {
		Skip("requires live cluster with MGMT_KUBECONFIG and TENANT-CLUSTER-E2E closed")
	})

	It("seam.ontave.dev zone contains SOA and NS records regardless of managed record count", func() {
		Skip("requires live cluster with MGMT_KUBECONFIG and TENANT-CLUSTER-E2E closed")
	})

	It("deleting a root declaration removes its DNS record from seam.ontave.dev zone within 30s", func() {
		Skip("requires live cluster with MGMT_KUBECONFIG and TENANT-CLUSTER-E2E closed")
	})
})
