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

	observer := driver.NewObserverHTTP(cfg)

	var recovery port.RecoveryClient
	var closeRecovery func()
	if strings.EqualFold(os.Getenv("RECOVERY_MODE"), "stub") {
		recovery = driver.NewStubRecovery()
		closeRecovery = func() {}
		logger.Warn("recovery stub mode")
	} else {
		rc, cleanup, err := driver.NewRecoveryNATS(cfg)
		if err != nil {
			logger.Warn("recovery nats unavailable, using stub", "error", err)
			recovery = driver.NewStubRecovery()
			closeRecovery = func() {}
		} else {
			recovery = rc
			closeRecovery = cleanup
		}
	}
	defer closeRecovery()

	executor := driver.NewStubExecutor()

	flow := service.NewRecoveryFlowService(observer, recovery, executor, logger)
	orchHandler := handler.NewOrchestratorHandler(flow, logger)

	grpcServer := grpc.NewServer()
	orchHandler.Register(grpcServer)

	health := handler.NewHealthHandler(handler.HealthDeps{
		Observer: observer,
		Recovery: recovery,
		Executor: executor,
	})
	mux := health.Routes()
	mux.HandleFunc("/recovery/run", orchHandler.RunRecoveryHTTP)
	mux.HandleFunc("/recovery/outcome", orchHandler.ReportOutcomeHTTP)
	healthServer := &http.Server{Addr: cfg.HealthAddr, Handler: mux}

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
