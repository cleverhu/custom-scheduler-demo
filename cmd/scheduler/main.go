/*
Copyright © 2024 Custom Scheduler Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/
package main

import (
	"fmt"
	"os"

	"github.com/cleverhu/custom-scheduler/pkg/scheduler/plugins/custom"
	"k8s.io/component-base/logs"
	"k8s.io/kubernetes/cmd/kube-scheduler/app"

	// Ensure scheme package is initialized.
	_ "k8s.io/kubernetes/pkg/apis/core/install"
)

func main() {
	logs.InitLogs()
	defer logs.FlushLogs()

	// Get the command with our options
	cmd := app.NewSchedulerCommand(
		app.WithPlugin(custom.Name, custom.New),
	)

	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
