// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/dotandev/hintents/internal/cmd"
	"github.com/dotandev/hintents/internal/config"
	"github.com/dotandev/hintents/internal/crashreport"
)

// Build-time variables injected via -ldflags.
var (
	version   = "dev"
	commitSHA = "unknown"
)

func main() {
	ctx := context.Background()

	// Load config to determine whether crash reporting is opted in.
	cfg, err := config.LoadConfig()
	if err != nil {
		// Non-fatal: fall back to a reporter that is disabled by default.
		cfg = config.DefaultConfig()
	}

	reporter := crashreport.New(crashreport.Config{
		Enabled:   cfg.CrashReporting,
		SentryDSN: cfg.CrashSentryDSN,
		Endpoint:  cfg.CrashEndpoint,
		Version:   version,
		CommitSHA: commitSHA,
	})

	// Catch any unrecovered panic, report it, then re-panic.
	defer reporter.HandlePanic(ctx, "erst")

	if execErr := cmd.Execute(); execErr != nil {
		// Report fatal command errors that were not recovered as panics.
		if reporter.IsEnabled() {
			stack := debug.Stack()
			_ = reporter.Send(ctx, execErr, stack, "erst")
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", execErr)
		os.Exit(1)
	}
}
