// SPDX-License-Identifier: Apache-2.0
package main

import (
	"flag"
	"os"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/erayyilmmaz/ai-workload-control-plane/internal/manager"
)

func main() {
	options := manager.Options{
		WatchNamespace:   os.Getenv("WATCH_NAMESPACE"),
		ManagerNamespace: os.Getenv("MANAGER_NAMESPACE"),
	}
	flag.StringVar(&options.ProbeAddress, "health-probe-bind-address", ":8081", "Health/readiness address")
	flag.StringVar(&options.MetricsBindAddress, "metrics-bind-address", ":8443", "Authenticated HTTPS metrics address")
	flag.BoolVar(&options.LeaderElection, "leader-elect", true, "Enable manager leader election")
	logOptions := zap.Options{Development: false}
	logOptions.BindFlags(flag.CommandLine)
	flag.Parse()
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&logOptions)))
	log := ctrl.Log.WithName("setup")
	if err := options.Validate(); err != nil {
		log.Error(err, "Invalid namespace configuration")
		os.Exit(1)
	}
	cfg, err := ctrl.GetConfig()
	if err != nil {
		log.Error(err, "Could not load Kubernetes configuration")
		os.Exit(1)
	}
	mgr, err := manager.New(cfg, options)
	if err != nil {
		log.Error(err, "Could not create manager")
		os.Exit(1)
	}
	log.Info("Starting AWCP manager", "namespace", options.WatchNamespace)
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Error(err, "Manager stopped with error")
		os.Exit(1)
	}
}
