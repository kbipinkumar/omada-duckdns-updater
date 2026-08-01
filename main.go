package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"
)

var version = "dev"

func main() {
	versionFlag := flag.Bool("version", false, "Print application version")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("omada-duckdns-updater version %s\n", version)
		os.Exit(0)
	}

	// Start the web server in a goroutine
	go startWebServer()

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

	for {
		time.Sleep(30 * time.Second) // Check every 30 seconds
		
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
