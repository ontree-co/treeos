package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"ontree-node/internal/agent"
)

func main() {
	var appsDir string
	flag.StringVar(&appsDir, "apps-dir", "/opt/ontree/apps", "Path to the apps directory")
	flag.Parse()

	// Check if directory exists
	if _, err := os.Stat(appsDir); os.IsNotExist(err) {
		log.Fatalf("Apps directory does not exist: %s", appsDir)
	}

	fmt.Printf("Updating app.yml files with actual container names in: %s\n", appsDir)

	if err := agent.UpdateAllAppConfigsWithActualContainers(appsDir); err != nil {
		log.Fatalf("Failed to update app configs: %v", err)
	}

	fmt.Println("Done!")
}
