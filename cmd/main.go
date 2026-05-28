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
	"github.com/mtavano/golden-gate/migrations"
	"github.com/pressly/goose/v3"
)

func main() {
	var conf config.Config
	err := envconfig.Process("", &conf)
	check(err)

	cfg, err := internalConfig.LoadConfig(internalConfig.GetConfigPath())
	check(err)

	dataDir := filepath.Dir(conf.DBPath)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("Error creating data directory: %v", err)
	}

	db, err := storage.NewSqlStore(conf.DBDriver, conf.DBPath)
	check(err)

	check(goose.SetDialect(conf.DBDriver))
	goose.SetBaseFS(migrations.EmbeddedFS)
	check(goose.Up(db.DB.DB, "."))

	requestSvc := service.NewRequestSvc(db)

	r := mux.NewRouter()

	dashboardHandler := dashboard.NewHandler(requestSvc, cfg)
	r.Handle("/dashboard", dashboardHandler).Methods(http.MethodGet)
	r.Handle("/dashboard/services/{name}", dashboardHandler.ExploreHandler()).Methods(http.MethodGet)

	for name, serviceConfig := range cfg.Services {
		proxyConfig := &proxy.Config{
			ServiceName:  name,
			BasePrefix:   serviceConfig.BasePrefix,
			Target:       serviceConfig.Target,
			MaxBodyBytes: conf.MaxBodyBytes,
		}
		proxyHandler := proxy.NewProxy(proxyConfig, requestSvc)
		r.PathPrefix(serviceConfig.BasePrefix).Handler(proxyHandler)
	}

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
