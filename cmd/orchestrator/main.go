package main

import (
	"log"
	"os"

	"github.com/utaaa/uta-travel-agent/internal/agent"
	"github.com/utaaa/uta-travel-agent/internal/router"
	"github.com/utaaa/uta-travel-agent/internal/scheduler"
)

func main() {
	log.Println("Starting UTA Travel Agent Orchestrator...")

	// Initialize agent registry
	registry := agent.NewRegistry()

	// Initialize scheduler
	sched := scheduler.NewScheduler(registry)

	// Initialize router
	r := router.NewRouter(registry, sched)

	// Get port from environment
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Start the server
	if err := r.Start(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
