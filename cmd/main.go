package main

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/mtavano/golden-gate/internal/config"
	"github.com/mtavano/golden-gate/internal/dashboard"
	"github.com/mtavano/golden-gate/internal/proxy"
	"github.com/mtavano/golden-gate/internal/types"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig(config.GetConfigPath())
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// Create the request store
	requestStore := types.NewRequestStore(100) // Keep the last 100 requests

	// Create the router
	r := mux.NewRouter()

	// Set up the dashboard
	dashboardHandler := dashboard.NewHandler(requestStore)
	r.Handle("/dashboard", dashboardHandler)

	// Set up proxies for each service
	for _, serviceConfig := range cfg.Services {
		proxyConfig := &proxy.Config{
			BasePrefix: serviceConfig.BasePrefix,
			Target:     serviceConfig.Target,
		}
		proxyHandler := proxy.NewProxy(proxyConfig, requestStore)
		r.PathPrefix(serviceConfig.BasePrefix).Handler(proxyHandler)
	}

	// Start the server
	log.Printf("Starting server on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
} 