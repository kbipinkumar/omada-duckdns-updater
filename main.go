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

	log.Println("Starting background updater...")

	// Initial run if config exists
	if config, err := loadConfig(); err == nil && config.OmadaURL != "" {
		log.Println("Running initial update...")
		if err := runUpdate(true); err != nil {
			log.Printf("Initial update failed: %v", err)
		} else {
			log.Println("Initial update successful.")
		}
	} else {
		log.Println("Config not found or incomplete. Please configure via Web UI.")
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Shutting down updater...")
			return nil
		case <-ticker.C:
			config, _ := loadConfig()
			interval := 5
			if config != nil && config.UpdateInterval > 0 {
				interval = config.UpdateInterval
			}

			globalState.RLock()
			lastRun := globalState.LastRunTime
			globalState.RUnlock()

			if time.Since(lastRun) >= time.Duration(interval)*time.Minute {
				log.Println("Timer triggered, running update...")
				if err := runUpdate(false); err != nil {
					log.Printf("Update failed: %v", err)
				} else {
					log.Println("Update successful.")
				}
			}
		}
	}
}

// runConsole runs the app in the foreground until SIGINT/SIGTERM.
func runConsole() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runApp(ctx)
}
