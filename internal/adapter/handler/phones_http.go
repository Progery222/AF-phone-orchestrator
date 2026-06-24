package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/mobilefarm/af/phone-orchestrator/internal/domain"
	"github.com/mobilefarm/af/phone-orchestrator/internal/service"
)

type PhonesHTTP struct {
	phones       *service.PhoneService
	orchestrator *service.OrchestratorService
}

func NewPhonesHTTP(phones *service.PhoneService, orchestrator *service.OrchestratorService) *PhonesHTTP {
	return &PhonesHTTP{phones: phones, orchestrator: orchestrator}
}

func (h *PhonesHTTP) Register(mux *http.ServeMux) {
	mux.HandleFunc("/phones", h.listOrAdd)
	mux.HandleFunc("/phones/", h.phoneBySerial)
	mux.HandleFunc("/stats", h.stats)
}

func (h *PhonesHTTP) listOrAdd(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		phones, stats, err := h.phones.ListPhones(r.Context())
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
			return
		}
		out := make([]phoneJSON, 0, len(phones))
		for _, p := range phones {
			out = append(out, toPhoneJSON(p))
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"phones": out,
			"total":  stats.Total,
			"stats": map[string]int{
				"working": stats.Working, "paused": stats.Paused,
				"error": stats.Error, "setting_up": stats.SettingUp,
			},
		})
	case http.MethodPost:
		var body struct {
			Serial string `json:"serial"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Serial == "" {
			http.Error(w, "укажите serial в JSON", http.StatusBadRequest)
			return
		}
		phone, err := h.phones.AddPhone(r.Context(), body.Serial)
		if err != nil {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, toPhoneJSON(phone))
	default:
		http.Error(w, "метод не поддерживается", http.StatusMethodNotAllowed)
	}
}

func (h *PhonesHTTP) phoneBySerial(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/phones/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	serial := parts[0]
	if len(parts) == 1 && r.Method == http.MethodGet {
		phone, err := h.phones.GetPhone(r.Context(), serial)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, toPhoneJSON(phone))
		return
	}
	if len(parts) == 2 {
		switch parts[1] {
		case "add":
			if r.Method != http.MethodPost {
				http.Error(w, "только POST", http.StatusMethodNotAllowed)
				return
			}
			phone, err := h.phones.AddPhone(r.Context(), serial)
			if err != nil {
				writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusCreated, toPhoneJSON(phone))
		case "remove":
			if r.Method != http.MethodPost {
				http.Error(w, "только POST", http.StatusMethodNotAllowed)
				return
			}
			if err := h.phones.RemovePhone(r.Context(), serial); err != nil {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "retired"})
		case "pause":
			if r.Method != http.MethodPost {
				http.Error(w, "только POST", http.StatusMethodNotAllowed)
				return
			}
			reason := r.URL.Query().Get("reason")
			if reason == "" {
				reason = "зависание экрана"
			}
			if err := h.orchestrator.PausePhone(r.Context(), serial, reason); err != nil {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "paused"})
		case "resume":
			if r.Method != http.MethodPost {
				http.Error(w, "только POST", http.StatusMethodNotAllowed)
				return
			}
			if err := h.orchestrator.ResumePhone(r.Context(), serial); err != nil {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "working"})
		case "reprovision":
			if r.Method != http.MethodPost {
				http.Error(w, "только POST", http.StatusMethodNotAllowed)
				return
			}
			if err := h.orchestrator.ReprovisionPhone(r.Context(), serial); err != nil {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "new"})
		default:
			http.NotFound(w, r)
		}
		return
	}
	http.NotFound(w, r)
}

func (h *PhonesHTTP) stats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "только GET", http.StatusMethodNotAllowed)
		return
	}
	_, stats, err := h.phones.ListPhones(r.Context())
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

type phoneJSON struct {
	Serial               string  `json:"serial"`
	State                string  `json:"state"`
	Model                string  `json:"model,omitempty"`
	AndroidVersion       string  `json:"android_version,omitempty"`
	IP                   string  `json:"ip,omitempty"`
	LastHeartbeat        string  `json:"last_heartbeat,omitempty"`
	Error                string  `json:"error,omitempty"`
	RecoveryInProgress     bool    `json:"recovery_in_progress,omitempty"`
	UptimeHours          float64 `json:"uptime_hours,omitempty"`
}

func toPhoneJSON(p domain.Phone) phoneJSON {
	j := phoneJSON{
		Serial: p.Serial, State: string(p.State), Model: p.Model,
		AndroidVersion: p.AndroidVersion, IP: p.CurrentIP,
		Error: p.LastError, RecoveryInProgress: p.RecoveryInProgress,
	}
	if p.LastHeartbeat != nil {
		j.LastHeartbeat = p.LastHeartbeat.UTC().Format(time.RFC3339)
	}
	if p.ReadyAt != nil {
		j.UptimeHours = time.Since(*p.ReadyAt).Hours()
	}
	return j
}
