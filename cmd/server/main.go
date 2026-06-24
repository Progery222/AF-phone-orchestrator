package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/mobilefarm/af/phone-orchestrator/internal/adapter/driver"
	"github.com/mobilefarm/af/phone-orchestrator/internal/adapter/handler"
	"github.com/mobilefarm/af/phone-orchestrator/internal/adapter/repository"
	"github.com/mobilefarm/af/phone-orchestrator/internal/config"
	"github.com/mobilefarm/af/phone-orchestrator/internal/port"
	"github.com/mobilefarm/af/phone-orchestrator/internal/service"
	"google.golang.org/grpc"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store, cleanupStore, err := openStore(ctx, cfg, logger)
	if err != nil {
		logger.Error("postgres unavailable", "error", err)
		os.Exit(1)
	}
	defer cleanupStore()

	observer := openObserver(cfg, logger)
	connector := driver.NewStubConnector()
	provision := driver.NewStubProvisioner()
	executor := driver.NewStubExecutor()

	recovery, closeRecovery := openRecovery(cfg, logger)
	defer closeRecovery()

	events, closeEvents := openEvents(cfg, logger)
	defer closeEvents()

	lock := repository.NewMemoryPhoneLock()
	flow := service.NewRecoveryFlowService(observer, recovery, executor, logger)
	orch := service.NewOrchestratorService(
		store, lock, connector, provision, flow, events, logger,
		cfg.PhoneLockTTLSec, cfg.OrchestratorTickSec,
	)
	phones := service.NewPhoneService(store)

	orchHandler := handler.NewOrchestratorHandler(flow, logger)
	phonesHTTP := handler.NewPhonesHTTP(phones, orch)

	grpcServer := grpc.NewServer()
	orchHandler.Register(grpcServer)

	mux := handler.NewHealthHandler(handler.HealthDeps{
		Observer: observer, Recovery: recovery, Executor: executor,
	}).Routes()
	phonesHTTP.Register(mux)
	mux.HandleFunc("/recovery/run", orchHandler.RunRecoveryHTTP)
	mux.HandleFunc("/recovery/outcome", orchHandler.ReportOutcomeHTTP)
	healthServer := &http.Server{Addr: cfg.HealthAddr, Handler: mux}

	go orch.Run(ctx)

	go func() {
		lis, err := net.Listen("tcp", cfg.GRPCAddr)
		if err != nil {
			logger.Error("grpc listen", "error", err)
			os.Exit(1)
		}
		logger.Info("grpc server started", "addr", cfg.GRPCAddr)
		_ = grpcServer.Serve(lis)
	}()

	go func() {
		logger.Info("http server started", "addr", cfg.HealthAddr)
		_ = healthServer.ListenAndServe()
	}()

	<-ctx.Done()
	logger.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	grpcServer.GracefulStop()
	_ = healthServer.Shutdown(shutdownCtx)
}

func openObserver(cfg config.Config, log *slog.Logger) port.ObserverClient {
	if strings.EqualFold(os.Getenv("OBSERVER_MODE"), "stub") {
		log.Warn("observer stub mode")
		return driver.NewStubObserver()
	}
	return driver.NewObserverHTTP(cfg)
}

func openStore(ctx context.Context, cfg config.Config, log *slog.Logger) (port.PhoneStore, func(), error) {
	if strings.EqualFold(os.Getenv("STORE_MODE"), "memory") {
		log.Warn("using in-memory phone store")
		return repository.NewMemoryPhoneStore(), func() {}, nil
	}
	pg, err := repository.NewPostgresPhoneStore(ctx, cfg.PostgresDSN)
	if err != nil {
		return nil, func() {}, err
	}
	return pg, pg.Close, nil
}

func openRecovery(cfg config.Config, log *slog.Logger) (port.RecoveryClient, func()) {
	if strings.EqualFold(os.Getenv("RECOVERY_MODE"), "stub") {
		log.Warn("recovery stub mode")
		return driver.NewStubRecovery(), func() {}
	}
	rc, cleanup, err := driver.NewRecoveryNATS(cfg)
	if err != nil {
		log.Warn("recovery nats unavailable, using stub", "error", err)
		return driver.NewStubRecovery(), func() {}
	}
	return rc, cleanup
}

func openEvents(cfg config.Config, log *slog.Logger) (port.EventPublisher, func()) {
	pub, cleanup, err := repository.NewNATSEventPublisher(cfg)
	if err != nil {
		log.Warn("nats events unavailable, using noop", "error", err)
		return repository.NewNoopEventPublisher(), func() {}
	}
	return pub, cleanup
}
