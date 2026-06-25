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
	connector, closeConnector := openConnector(cfg, logger)
	defer closeConnector()
	provision := openProvisioner(cfg, logger)
	executor, closeExecutor := openExecutor(cfg, logger)
	defer closeExecutor()

	recovery, closeRecovery := openRecovery(cfg, logger)
	defer closeRecovery()

	events, closeEvents := openEvents(cfg, logger)
	defer closeEvents()

	content, closeContent := openContent(cfg, logger)
	defer closeContent()

	contacts, closeContacts := openContacts(cfg, logger)
	defer closeContacts()

	video, closeVideo := openVideo(cfg, logger)
	defer closeVideo()

	lock := repository.NewMemoryPhoneLock()
	flow := service.NewRecoveryFlowService(observer, recovery, executor, logger)
	orch := service.NewOrchestratorService(
		store, lock, connector, provision, flow, events, logger,
		cfg.PhoneLockTTLSec, cfg.OrchestratorTickSec,
	)
	phones := service.NewPhoneService(store)

	orchHandler := handler.NewOrchestratorHandler(flow, logger)
	phonesHTTP := handler.NewPhonesHTTP(phones, orch, connector, observer, executor, content, contacts, video)

	grpcServer := grpc.NewServer()
	orchHandler.Register(grpcServer)

	mux := handler.NewHealthHandler(handler.HealthDeps{
		Observer: observer, Recovery: recovery, Executor: executor, Provisioner: provision, Content: content, Contacts: contacts, Video: video,
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

func openConnector(cfg config.Config, log *slog.Logger) (port.ConnectorClient, func()) {
	if mode := os.Getenv("CONNECTOR_MODE"); mode == "" || strings.EqualFold(mode, "stub") {
		log.Warn("connector stub mode")
		return driver.NewStubConnector(), func() {}
	}
	conn, cleanup, err := driver.NewConnectorGRPC(cfg)
	if err != nil {
		log.Warn("connector grpc unavailable, using stub", "error", err)
		return driver.NewStubConnector(), func() {}
	}
	log.Info("connector grpc client", "addr", cfg.ConnectorGRPCAddr)
	return conn, cleanup
}

func openProvisioner(cfg config.Config, log *slog.Logger) port.ProvisionClient {
	if mode := os.Getenv("PROVISIONER_MODE"); mode == "" || strings.EqualFold(mode, "stub") {
		log.Warn("provisioner stub mode")
		return driver.NewStubProvisioner()
	}
	if cfg.ProvisionerHTTPURL == "" {
		log.Warn("PROVISIONER_HTTP_URL пуст, provisioner stub")
		return driver.NewStubProvisioner()
	}
	log.Info("provisioner http client", "url", cfg.ProvisionerHTTPURL)
	return driver.NewProvisionHTTP(cfg)
}

func openObserver(cfg config.Config, log *slog.Logger) port.ObserverClient {
	if strings.EqualFold(os.Getenv("OBSERVER_MODE"), "stub") {
		log.Warn("observer stub mode")
		return driver.NewStubObserver()
	}
	return driver.NewObserverHTTP(cfg)
}

func openExecutor(cfg config.Config, log *slog.Logger) (port.ExecutorClient, func()) {
	if strings.EqualFold(os.Getenv("EXECUTOR_MODE"), "stub") {
		log.Warn("executor stub mode")
		return driver.NewStubExecutor(), func() {}
	}
	ex, cleanup, err := driver.NewExecutorGRPC(cfg)
	if err != nil {
		log.Warn("executor grpc unavailable, using stub", "error", err)
		return driver.NewStubExecutor(), func() {}
	}
	return ex, cleanup
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

func openContent(cfg config.Config, log *slog.Logger) (port.ContentClient, func()) {
	if mode := os.Getenv("CONTENT_MODE"); mode == "" || strings.EqualFold(mode, "stub") {
		log.Warn("content-distributor stub mode")
		return driver.NewStubContent(), func() {}
	}
	if cfg.ContentDistributorHTTPURL == "" {
		log.Warn("CONTENT_DISTRIBUTOR_HTTP_URL пуст, content stub")
		return driver.NewStubContent(), func() {}
	}
	httpClient := driver.NewContentHTTP(cfg)
	var natsPub *driver.ContentNATS
	var cleanup func() = func() {}
	if !strings.EqualFold(os.Getenv("CONTENT_NATS_MODE"), "off") {
		np, closeFn, err := driver.NewContentNATS(cfg)
		if err != nil {
			log.Warn("content nats unavailable, download/delete via HTTP sync", "error", err)
		} else {
			natsPub = np
			cleanup = closeFn
			log.Info("content nats publisher", "download", cfg.NATSSubjectContentDownload, "delete", cfg.NATSSubjectContentDelete)
		}
	}
	log.Info("content-distributor http client", "url", cfg.ContentDistributorHTTPURL)
	return driver.NewContentClient(httpClient, natsPub), cleanup
}

func openContacts(cfg config.Config, log *slog.Logger) (port.ContactsClient, func()) {
	if mode := os.Getenv("CONTACTS_MODE"); mode == "" || strings.EqualFold(mode, "stub") {
		log.Warn("contacts-manager stub mode")
		return driver.NewStubContacts(), func() {}
	}
	client, cleanup, err := driver.NewContactsGRPC(cfg)
	if err != nil {
		log.Warn("contacts-manager grpc unavailable, stub", "error", err)
		return driver.NewStubContacts(), func() {}
	}
	log.Info("contacts-manager grpc client", "addr", cfg.ContactsGRPCAddr)
	return client, cleanup
}

func openVideo(cfg config.Config, log *slog.Logger) (port.VideoClient, func()) {
	if mode := os.Getenv("VIDEO_MODE"); mode == "" || strings.EqualFold(mode, "stub") {
		log.Warn("video-generator stub mode")
		return driver.NewStubVideo(), func() {}
	}
	client, cleanup, err := driver.NewVideoGRPC(cfg)
	if err != nil {
		log.Warn("video-generator grpc unavailable, stub", "error", err)
		return driver.NewStubVideo(), func() {}
	}
	log.Info("video-generator grpc client", "addr", cfg.VideoGRPCAddr)
	return client, cleanup
}
