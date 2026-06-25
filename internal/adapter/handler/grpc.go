package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/mobilefarm/af/phone-orchestrator/internal/domain"
	"github.com/mobilefarm/af/phone-orchestrator/internal/port"
	"github.com/mobilefarm/af/phone-orchestrator/internal/service"
	"google.golang.org/grpc"
)

type OrchestratorHandler struct {
	svc *service.RecoveryFlowService
	log port.Logger
}

func NewOrchestratorHandler(svc *service.RecoveryFlowService, log port.Logger) *OrchestratorHandler {
	return &OrchestratorHandler{svc: svc, log: log}
}

func (h *OrchestratorHandler) Register(s *grpc.Server) {
	_ = s // orchestratorpb.RegisterOrchestratorServiceServer(s, h)
}

type HealthDeps struct {
	Observer    port.ObserverClient
	Recovery    port.RecoveryClient
	Executor    port.ExecutorClient
	Provisioner port.ProvisionClient
}

type HealthHandler struct {
	deps HealthDeps
}

func NewHealthHandler(deps HealthDeps) *HealthHandler {
	return &HealthHandler{deps: deps}
}

func (h *HealthHandler) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/ready", h.ready)
	return mux
}

func (h *HealthHandler) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.deps.Observer.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not ready", "observer": err.Error()})
		return
	}
	if err := h.deps.Recovery.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not ready", "recovery": err.Error()})
		return
	}
	if err := h.deps.Executor.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not ready", "executor": err.Error()})
		return
	}
	if h.deps.Provisioner != nil {
		if err := h.deps.Provisioner.Ping(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not ready", "provisioner": err.Error()})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// RunRecoveryHTTP — временный REST для отладки до регистрации gRPC.
func (h *OrchestratorHandler) RunRecoveryHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "только POST", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Serial   string `json:"serial"`
		Scenario string `json:"scenario"`
		Context  string `json:"context"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "некорректный JSON", http.StatusBadRequest)
		return
	}
	plan, err := h.svc.RunRecovery(r.Context(), body.Serial, body.Scenario, body.Context)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func (h *OrchestratorHandler) ReportOutcomeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "только POST", http.StatusMethodNotAllowed)
		return
	}
	var req domain.RecoveryOutcomeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "некорректный JSON", http.StatusBadRequest)
		return
	}
	if err := h.svc.ReportOutcome(r.Context(), req); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"accepted": "true"})
}
