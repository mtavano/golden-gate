package main

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/mtavano/golden-gate/internal/config"
	"github.com/mtavano/golden-gate/internal/dashboard"
	"github.com/mtavano/golden-gate/internal/models"
	"github.com/mtavano/golden-gate/internal/proxy"
)

func main() {
	// Cargar configuración
	cfg, err := config.LoadConfig(config.GetConfigPath())
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// Crear el store de requests
	requestStore := models.NewRequestStore(100) // Mantener últimos 100 requests

	// Crear el router
	r := mux.NewRouter()

	// Configurar el dashboard
	dashboardHandler := dashboard.NewHandler(requestStore)
	r.Handle("/dashboard", dashboardHandler)

	// Configurar los proxies para cada servicio
	for _, serviceConfig := range cfg.Services {
		proxyConfig := &proxy.Config{
			BasePrefix: serviceConfig.BasePrefix,
			Target:     serviceConfig.Target,
		}
		proxyHandler := proxy.NewProxy(proxyConfig, requestStore)
		r.PathPrefix(serviceConfig.BasePrefix).Handler(proxyHandler)
	}

	// Iniciar el servidor
	log.Printf("Starting server on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
} 