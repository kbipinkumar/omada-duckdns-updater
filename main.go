package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// version is injected at build time via -ldflags.
var version = "dev"

func main() {
	versionFlag := flag.Bool("version", false, "Print application version")
	serviceFlag := flag.String("service", "", "Windows service control: install, uninstall, start, stop (Windows only)")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("omada-duckdns-updater version %s\n", version)
		os.Exit(0)
	}

	if *serviceFlag != "" {
		if err := handleServiceCommand(*serviceFlag); err != nil {
			log.Fatalf("Service command failed: %v", err)
		}
		return
	}

	if err := runMaybeAsService(); err != nil {
		log.Fatal(err)
	}
}

// runApp starts the web UI and background updater until ctx is cancelled.
func runApp(ctx context.Context) error {
	go startWebServer(ctx)

	logInfo("background updater starting (version=%s data_dir=%q)", version, getDataDir())

	// Initial run when configuration is complete.
	if config, err := loadConfig(); err == nil && configIsComplete(config) {
		logInfo("running initial update (%s)", describeConfig(config))
		if err := runUpdate(true); err != nil {
			logError("initial update failed: %v", err)
		} else {
			logInfo("initial update successful")
		}
	} else if err != nil {
		logError("initial update skipped: failed to load configuration: %v", err)
	} else {
		logInfo("initial update skipped: incomplete configuration (%s)", describeConfig(config))
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logInfo("shutting down updater")
			return nil
		case <-ticker.C:
			config, err := loadConfig()
			if err != nil {
				logError("scheduled update check failed to load configuration: %v", err)
				continue
			}
			interval := 5
			if config != nil && config.UpdateInterval > 0 {
				interval = config.UpdateInterval
			}

			globalState.RLock()
			lastRun := globalState.LastRunTime
			globalState.RUnlock()

			if config == nil || !configIsComplete(config) {
				logIncompleteConfigThrottled(config)
				continue
			}

			if !lastRun.IsZero() && time.Since(lastRun) < time.Duration(interval)*time.Minute {
				continue
			}

			logInfo("timer triggered update (force=false interval=%dm %s)", interval, describeConfig(config))
			if err := runUpdate(false); err != nil {
				logError("scheduled update failed: %v", err)
			} else {
				logInfo("scheduled update successful")
			}
		}
	}
}

// runConsole runs the app in the foreground until SIGINT/SIGTERM.
func runConsole() error {
	closer := initLogging()
	if closer != nil {
		defer func() {
			if err := closer.Close(); err != nil {
				log.Printf("failed to close log file: %v", err)
			}
		}()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runApp(ctx)
}
