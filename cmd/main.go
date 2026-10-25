// The manager reconciles AIWorkload children; Kubernetes owns scheduling.
package main

import (
	"flag"
	ai "github.com/Dhiraj1729/GpuNodeSelector/api/v1alpha1"
	"github.com/Dhiraj1729/GpuNodeSelector/internal/controller"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	clientconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var kubeContext string

func main() {
	flag.StringVar(&kubeContext, "context", "", "Explicit kubeconfig context for local execution")
	flag.Parse()
	ctrl.SetLogger(zap.New())
	if err := run(); err != nil {
		ctrl.Log.Error(err, "manager stopped")
		os.Exit(1)
	}
}

func run() error {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		return err
	}
	if err := ai.AddToScheme(scheme); err != nil {
		return err
	}
	config, err := clientconfig.GetConfigWithContext(kubeContext)
	if err != nil {
		return err
	}
	mgr, err := ctrl.NewManager(config, ctrl.Options{Scheme: scheme, Metrics: metricsserver.Options{BindAddress: "0"}, HealthProbeBindAddress: ":8081"})
	if err != nil {
		return err
	}
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return err
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return err
	}
	if err := (&controller.AIWorkloadReconciler{Client: mgr.GetClient()}).SetupWithManager(mgr); err != nil {
		return err
	}
	ctrl.Log.Info("AIWorkload reconciler registered")
	return mgr.Start(ctrl.SetupSignalHandler())
}
