// Binary seam-core is the controller-runtime manager entry point for the
// Seam Core schema controller.
//
// It registers one LineageReconciler per root-declaration GVK and one
// DSNSReconciler per DSNS GVK, all sharing the manager's informer cache.
// Seam Core installs before all operators (SC-INV-003).
//
// seam-core-schema.md §8 Decision 1 — DSNS is a controller within seam-core,
// registered alongside LineageController in this file.
package main

import (
	"flag"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	ctrlwebhook "sigs.k8s.io/controller-runtime/pkg/webhook"

	seamv1alpha1 "github.com/ontai-dev/seam-core/api/v1alpha1"
	"github.com/ontai-dev/seam-core/internal/controller"
	idns "github.com/ontai-dev/seam-core/internal/dns"
	"github.com/ontai-dev/seam-core/internal/webhook"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(seamv1alpha1.AddToScheme(scheme))
}

func main() {
	var (
		metricsAddr          string
		healthProbeAddr      string
		enableLeaderElection bool
		webhookPort          int
	)

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080",
		"The address the metrics endpoint binds to.")
	flag.StringVar(&healthProbeAddr, "health-probe-bind-address", ":8081",
		"The address the health and readiness probes bind to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", true,
		"Enable leader election for controller manager. "+
			"Ensures only one instance is active at a time.")
	flag.IntVar(&webhookPort, "webhook-port", 9443,
		"The port the admission webhook server binds to.")

	opts := zap.Options{}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	setupLog := ctrl.Log.WithName("setup")

	// SC-INV-003: seam-core installs before all operators.
	// Leader election lease: seam-core-leader in seam-system.
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                  scheme,
		Metrics:                 metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress:  healthProbeAddr,
		LeaderElection:          enableLeaderElection,
		LeaderElectionID:        "seam-core-leader",
		LeaderElectionNamespace: "seam-system",
		WebhookServer: ctrlwebhook.NewServer(ctrlwebhook.Options{
			Port: webhookPort,
		}),
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Register one LineageReconciler per root-declaration GVK.
	// Each reconciler watches its GVK via unstructured and creates one
	// InfrastructureLineageIndex per observed root declaration.
	// CLAUDE.md §14 Decision 4 — one index per root declaration.
	for _, gvk := range controller.RootDeclarationGVKs {
		r := &controller.LineageReconciler{
			Client: mgr.GetClient(),
			Scheme: mgr.GetScheme(),
			GVK:    gvk,
		}
		if err := r.SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create LineageReconciler",
				"gvk", gvk.String())
			os.Exit(1)
		}
	}

	// Register DSNSReconciler — one instance per DSNS GVK, all sharing one DSNSState.
	// seam-core-schema.md §8 Decision 1 — DSNS shares the existing informer cache.
	dsnsState := idns.NewDSNSState(mgr.GetClient())

	// Seed the static authority.conductor record from the environment variable
	// CONDUCTOR_SIGNING_KEY_FINGERPRINT. If absent, the record is not emitted.
	// seam-core-schema.md §8 Decision 4 — Conductor authority record.
	if fingerprint := os.Getenv("CONDUCTOR_SIGNING_KEY_FINGERPRINT"); fingerprint != "" {
		dsnsState.SetStaticRecord(idns.Record{
			Name:  "authority.conductor",
			Type:  idns.RecordTypeTXT,
			Value: fingerprint,
		})
		setupLog.Info("DSNS: seeded authority.conductor static record")
	}

	for _, gvk := range controller.DSNSGVKs {
		r := &controller.DSNSReconciler{
			Client: mgr.GetClient(),
			GVK:    gvk,
			State:  dsnsState,
		}
		if err := r.SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create DSNSReconciler",
				"gvk", gvk.String())
			os.Exit(1)
		}
	}
	setupLog.Info("DSNS registered", "gvks", len(controller.DSNSGVKs))

	// Register admission webhooks for InfrastructureLineageIndex.
	// Both webhooks must be registered before mgr.Start.
	//
	// RegisterImmutability: rejects UPDATE requests that modify spec.rootBinding.
	// seam-core-schema.md §3.1, domain-core-schema.md §2.1.
	//
	// RegisterAuthorship: rejects CREATE/UPDATE from any principal other than the
	// LineageController ServiceAccount. CLAUDE.md §14 Decision 3.
	webhookServer := webhook.NewAdmissionWebhookServer(mgr)
	webhookServer.RegisterImmutability()
	webhookServer.RegisterAuthorship()

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting seam-core manager",
		"rootDeclarationGVKs", len(controller.RootDeclarationGVKs))
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
