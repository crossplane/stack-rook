/*
Copyright 2019 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"os"
	"path/filepath"

	"gopkg.in/alecthomas/kingpin.v2"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	runtimelog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"github.com/crossplane/stack-rook/apis"
	"github.com/crossplane/stack-rook/pkg/controller"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	crossplaneapis "github.com/crossplane/crossplane/apis"
)

func main() {
	var (
		// top level app definition
		app        = kingpin.New(filepath.Base(os.Args[0]), "An open source multicloud control plane.").DefaultEnvars()
		debug      = app.Flag("debug", "Run with debug logging.").Short('d').Bool()
		syncPeriod = app.Flag("sync", "Controller manager sync period duration such as 300ms, 1.5h or 2h45m").
				Short('s').Default("1h").Duration()

		// default crossplane command and args, this is the default main entry point for Crossplane's
		// multi-cloud control plane functionality
		crossplaneCmd = app.Command(filepath.Base(os.Args[0]), "An open source multicloud control plane.").Default()
	)
	cmd := kingpin.MustParse(app.Parse(os.Args[1:]))

	zl := zap.New(zap.UseDevMode(*debug))
	log := logging.NewLogrLogger(zl.WithName("stack-rook"))
	if *debug {
		// The controller-runtime runs with a no-op logger by default. It is
		// *very* verbose even at info level, so we only provide it a real
		// logger when we're running in debug mode.
		runtimelog.SetLogger(zl)
	}

	var setupWithManagerFunc func(manager.Manager) error

	// Determine the command being called and execute the corresponding logic
	switch cmd {
	case crossplaneCmd.FullCommand():
		// the default Crossplane command is being run, add all the regular controllers to the manager
		setupWithManagerFunc = controllerSetupWithManager
	default:
		kingpin.FatalUsage("unknown command %s", cmd)
	}

	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		kingpin.FatalIfError(err, "Cannot get config")
	}

	log.Info("Sync period", "duration", syncPeriod.String())

	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := manager.New(cfg, manager.Options{SyncPeriod: syncPeriod})
	if err != nil {
		kingpin.FatalIfError(err, "Cannot create manager")
	}

	log.Info("Adding schemes")

	// add all resources to the manager's runtime scheme
	if err := addToScheme(mgr.GetScheme()); err != nil {
		kingpin.FatalIfError(err, "Cannot add APIs to scheme")
	}

	log.Info("Adding controllers")

	// Setup all Controllers
	if err := setupWithManagerFunc(mgr); err != nil {
		kingpin.FatalIfError(err, "Cannot add controllers to manager")
	}

	log.Info("Starting the manager")

	// Start the Cmd
	kingpin.FatalIfError(mgr.Start(signals.SetupSignalHandler()), "Cannot start controller")
}

func controllerSetupWithManager(mgr manager.Manager) error {
	if err := (&controller.Controllers{}).SetupWithManager(mgr); err != nil {
		return err
	}

	return nil
}

// addToScheme adds all resources to the runtime scheme.
func addToScheme(scheme *runtime.Scheme) error {
	if err := apis.AddToScheme(scheme); err != nil {
		return err
	}

	if err := crossplaneapis.AddToScheme(scheme); err != nil {
		return err
	}

	return nil
}
