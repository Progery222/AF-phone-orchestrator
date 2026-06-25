package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	GRPCAddr               string
	HealthAddr             string
	PostgresDSN            string
	ObserverHTTPURL        string
	RecoveryGRPCAddr         string
	ExecutorGRPCAddr       string
	ConnectorGRPCAddr      string
	ProvisionerHTTPURL     string
	ProvisionerDefaultProxyIP   string
	ProvisionerDefaultProxyPort int
	ProvisionerDefaultProxyUser string
	ProvisionerDefaultProxyPass string
	ProvisionerDefaultWiFiSSID    string
	ProvisionerDefaultWiFiPass    string
	ProvisionerDefaultAppsJSON    string
	ContentDistributorHTTPURL     string
	ContactsGRPCAddr              string
	VideoGRPCAddr                 string
	NATSURL                string
	NATSSubjectRecoveryIn  string
	NATSSubjectRecoveryOut string
	NATSSubjectOutcome     string
	NATSSubjectStateChanged string
	NATSSubjectContentDownload string
	NATSSubjectContentDelete   string
	NATSSubjectContentReady    string
	RecoveryTimeoutSec     int
	OrchestratorTickSec    int
	PhoneLockTTLSec        int
	LogLevel               slog.Level
}

func Load() Config {
	return Config{
		GRPCAddr:                env("GRPC_ADDR", ":50050"),
		HealthAddr:              env("HEALTH_ADDR", ":9090"),
		PostgresDSN:             env("POSTGRES_DSN", "postgres://orchestrator:orchestrator@localhost:5434/orchestrator?sslmode=disable"),
		ObserverHTTPURL:         env("OBSERVER_HTTP_URL", "http://127.0.0.1:19090"),
		RecoveryGRPCAddr:        env("RECOVERY_GRPC_ADDR", "localhost:50054"),
		ExecutorGRPCAddr:        env("EXECUTOR_GRPC_ADDR", "localhost:50051"),
		ConnectorGRPCAddr:       env("CONNECTOR_GRPC_ADDR", "localhost:50052"),
		ProvisionerHTTPURL:      env("PROVISIONER_HTTP_URL", "http://127.0.0.1:19092"),
		ProvisionerDefaultProxyIP:   env("PROVISIONER_DEFAULT_PROXY_IP", ""),
		ProvisionerDefaultProxyPort: envInt("PROVISIONER_DEFAULT_PROXY_PORT", 3128),
		ProvisionerDefaultProxyUser: env("PROVISIONER_DEFAULT_PROXY_USER", ""),
		ProvisionerDefaultProxyPass: env("PROVISIONER_DEFAULT_PROXY_PASS", ""),
		ProvisionerDefaultWiFiSSID:  env("PROVISIONER_DEFAULT_WIFI_SSID", ""),
		ProvisionerDefaultWiFiPass:  env("PROVISIONER_DEFAULT_WIFI_PASS", ""),
		ProvisionerDefaultAppsJSON: env("PROVISIONER_DEFAULT_APPS_JSON", ""),
		ContentDistributorHTTPURL:  env("CONTENT_DISTRIBUTOR_HTTP_URL", "http://127.0.0.1:19094"),
		ContactsGRPCAddr:           env("CONTACTS_GRPC_ADDR", "localhost:50055"),
		VideoGRPCAddr:              env("VIDEO_GRPC_ADDR", "localhost:50056"),
		NATSURL:                 env("NATS_URL", "nats://localhost:4222"),
		NATSSubjectRecoveryIn:   env("NATS_SUBJECT_RECOVERY_IN", "af.recovery.request"),
		NATSSubjectRecoveryOut:  env("NATS_SUBJECT_RECOVERY_OUT", "af.recovery.response"),
		NATSSubjectOutcome:      env("NATS_SUBJECT_OUTCOME", "af.recovery.outcome"),
		NATSSubjectStateChanged: env("NATS_SUBJECT_STATE_CHANGED", "phone.state.changed"),
		NATSSubjectContentDownload: env("NATS_SUBJECT_CONTENT_DOWNLOAD", "af.content.download"),
		NATSSubjectContentDelete:   env("NATS_SUBJECT_CONTENT_DELETE", "af.content.delete"),
		NATSSubjectContentReady:    env("NATS_SUBJECT_CONTENT_READY", "af.content.ready"),
		RecoveryTimeoutSec:      envInt("RECOVERY_TIMEOUT_SEC", 120),
		OrchestratorTickSec:     envInt("ORCHESTRATOR_TICK_SEC", 2),
		PhoneLockTTLSec:         envInt("PHONE_LOCK_TTL_SEC", 30),
		LogLevel:                parseLogLevel(env("LOG_LEVEL", "info")),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

func parseLogLevel(raw string) slog.Level {
	switch strings.ToLower(raw) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
