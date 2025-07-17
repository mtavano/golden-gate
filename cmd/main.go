package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/kelseyhightower/envconfig"
	"github.com/mtavano/golden-gate/config"
	internalConfig "github.com/mtavano/golden-gate/internal/config"
	"github.com/mtavano/golden-gate/internal/dashboard"
	"github.com/mtavano/golden-gate/internal/proxy"
	"github.com/mtavano/golden-gate/internal/service"
	"github.com/mtavano/golden-gate/internal/storage"
	_ "github.com/mtavano/golden-gate/migrations"
	"github.com/pressly/goose/v3"
)

func main() {
	// Load env
	var conf config.Config
	err := envconfig.Process("", &conf)
	check(err)

	// Load configuration
	cfg, err := internalConfig.LoadConfig(internalConfig.GetConfigPath())
	check(err)

	// Create data directory if it doesn't exist
	dataDir := filepath.Dir(conf.DBPath)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("Error creating data directory: %v", err)
	}

	// Create database connection
	db, err := storage.NewSqlStore(conf.DBDriver, conf.DBPath)
	check(err)

	// Run migrations
	err = goose.SetDialect(conf.DBDriver)
	check(err)

	err = goose.Up(db.DB.DB, "./migrations")
	check(err)

	// Create the request service with database
	requestSvc := service.NewRequestSvc(db)

	// Create the router
	r := mux.NewRouter()

	// Set up the dashboard
	dashboardHandler := dashboard.NewHandler(requestSvc)
	r.Handle("/dashboard", dashboardHandler)

	// Set up proxies for each service
	for _, serviceConfig := range cfg.Services {
		proxyConfig := &proxy.Config{
			BasePrefix: serviceConfig.BasePrefix,
			Target:     serviceConfig.Target,
		}
		proxyHandler := proxy.NewProxy(proxyConfig, requestSvc)
		r.PathPrefix(serviceConfig.BasePrefix).Handler(proxyHandler)
	}

	// Start the server
	p := fmt.Sprintf(":%d", conf.Port)
	log.Printf("Starting server on %s", p)
	if err := http.ListenAndServe(p, r); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
