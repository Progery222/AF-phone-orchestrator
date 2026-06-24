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
	NATSURL                string
	NATSSubjectRecoveryIn  string
	NATSSubjectRecoveryOut string
	NATSSubjectOutcome     string
	NATSSubjectStateChanged string
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
		NATSURL:                 env("NATS_URL", "nats://localhost:4222"),
		NATSSubjectRecoveryIn:   env("NATS_SUBJECT_RECOVERY_IN", "af.recovery.request"),
		NATSSubjectRecoveryOut:  env("NATS_SUBJECT_RECOVERY_OUT", "af.recovery.response"),
		NATSSubjectOutcome:      env("NATS_SUBJECT_OUTCOME", "af.recovery.outcome"),
		NATSSubjectStateChanged: env("NATS_SUBJECT_STATE_CHANGED", "phone.state.changed"),
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
