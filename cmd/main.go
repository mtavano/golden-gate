package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

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
	"go.uber.org/zap"
)

func main() {
	var conf config.Config
	err := envconfig.Process("", &conf)
	check(err)

	logger, _ := zap.NewProduction()
	defer logger.Sync()

	if err := os.MkdirAll(filepath.Dir(conf.DBPath), 0o755); err != nil {
		log.Fatalf("Error creating data directory: %v", err)
	}

	db, err := storage.NewSqlStore(conf.DBDriver, conf.DBPath)
	check(err)

	check(goose.SetDialect(conf.DBDriver))
	goose.SetBaseFS(migrations.EmbeddedFS)
	check(goose.Up(db.DB.DB, "."))

	cfgMgr, err := internalConfig.NewManager(conf.ConfigPath, logger)
	check(err)

	requestSvc := service.NewRequestSvc(db)

	dispatcher := proxy.NewDispatcher(requestSvc, conf.MaxBodyBytes, logger)
	cfgMgr.Subscribe(dispatcher.Update)

	dashboardHandler := dashboard.NewHandler(requestSvc, cfgMgr)

	r := mux.NewRouter()
	r.Handle("/dashboard", dashboardHandler).Methods(http.MethodGet)
	r.Handle("/dashboard/services/{name}", dashboardHandler.ExploreHandler()).Methods(http.MethodGet)
	r.PathPrefix("/").Handler(dispatcher)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go func() {
		if err := cfgMgr.Watch(ctx); err != nil {
			logger.Error("config watcher exited", zap.Error(err))
		}
	}()

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", conf.Port),
		Handler: r,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithCancel(context.Background())
		defer shutdownCancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	logger.Info("starting server", zap.String("addr", srv.Addr), zap.String("config", conf.ConfigPath))
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Error starting server: %v", err)
	}
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
