package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mobilefarm/af/phone-orchestrator/internal/domain"
	"github.com/mobilefarm/af/phone-orchestrator/internal/port"
	"github.com/mobilefarm/af/phone-orchestrator/internal/service"
)

type PhonesHTTP struct {
	phones       *service.PhoneService
	orchestrator *service.OrchestratorService
	connector    port.ConnectorClient
	observer     port.ObserverClient
	executor     port.ExecutorClient
}

func NewPhonesHTTP(phones *service.PhoneService, orchestrator *service.OrchestratorService, connector port.ConnectorClient, observer port.ObserverClient, executor port.ExecutorClient) *PhonesHTTP {
	return &PhonesHTTP{phones: phones, orchestrator: orchestrator, connector: connector, observer: observer, executor: executor}
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
		var body domain.AddPhoneRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Serial == "" {
			http.Error(w, "укажите serial в JSON", http.StatusBadRequest)
			return
		}
		phone, err := h.phones.AddPhone(r.Context(), body)
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
			phone, err := h.phones.AddPhone(r.Context(), domain.AddPhoneRequest{Serial: serial})
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
		case "screen":
			if r.Method != http.MethodGet {
				http.Error(w, "только GET", http.StatusMethodNotAllowed)
				return
			}
			h.captureScreen(w, r, serial)
		case "ui":
			if r.Method != http.MethodGet {
				http.Error(w, "только GET", http.StatusMethodNotAllowed)
				return
			}
			h.dumpUI(w, r, serial)
		case "observe":
			if r.Method != http.MethodGet {
				http.Error(w, "только GET", http.StatusMethodNotAllowed)
				return
			}
			h.observe(w, r, serial)
		case "tap":
			if r.Method != http.MethodPost {
				http.Error(w, "только POST", http.StatusMethodNotAllowed)
				return
			}
			h.executorTap(w, r, serial)
		case "swipe":
			if r.Method != http.MethodPost {
				http.Error(w, "только POST", http.StatusMethodNotAllowed)
				return
			}
			h.executorSwipe(w, r, serial)
		case "key":
			if r.Method != http.MethodPost {
				http.Error(w, "только POST", http.StatusMethodNotAllowed)
				return
			}
			h.executorKey(w, r, serial)
		case "wifi":
			if r.Method != http.MethodPost {
				http.Error(w, "только POST", http.StatusMethodNotAllowed)
				return
			}
			h.phoneWifi(w, r, serial)
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
	Serial             string  `json:"serial"`
	State              string  `json:"state"`
	Model              string  `json:"model,omitempty"`
	AndroidVersion     string  `json:"android_version,omitempty"`
	IP                 string  `json:"ip,omitempty"`
	LastHeartbeat      string  `json:"last_heartbeat,omitempty"`
	HeartbeatCount     int     `json:"heartbeat_count,omitempty"`
	Error              string  `json:"error,omitempty"`
	LastErrorHash      string  `json:"last_error_hash,omitempty"`
	RecoveryInProgress bool    `json:"recovery_in_progress,omitempty"`
	UptimeHours        float64 `json:"uptime_hours,omitempty"`
}

func toPhoneJSON(p domain.Phone) phoneJSON {
	j := phoneJSON{
		Serial: p.Serial, State: string(p.State), Model: p.Model,
		AndroidVersion: p.AndroidVersion, IP: p.CurrentIP,
		Error: p.LastError, LastErrorHash: p.LastErrorHash,
		RecoveryInProgress: p.RecoveryInProgress,
		HeartbeatCount:     p.HeartbeatCount,
	}
	if p.LastHeartbeat != nil {
		j.LastHeartbeat = p.LastHeartbeat.UTC().Format(time.RFC3339)
	}
	if p.ReadyAt != nil {
		j.UptimeHours = time.Since(*p.ReadyAt).Hours()
	}
	return j
}

func (h *PhonesHTTP) captureScreen(w http.ResponseWriter, r *http.Request, serial string) {
	if h.observer == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "observer не настроен"})
		return
	}
	timeout := observerTimeout(r, 30)
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	screen, err := h.observer.CaptureScreen(ctx, serial)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"serial":         screen.Serial,
		"minio_key":      screen.MinioKey,
		"screenshot_url": screen.ScreenshotURL,
		"resolution": map[string]int{
			"width":  screen.Width,
			"height": screen.Height,
		},
	})
}

func (h *PhonesHTTP) dumpUI(w http.ResponseWriter, r *http.Request, serial string) {
	if h.observer == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "observer не настроен"})
		return
	}
	timeout := observerTimeout(r, 30)
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	ui, err := h.observer.DumpUI(ctx, serial)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"serial":       ui.Serial,
		"package_name": ui.Package,
		"xml_dump":     ui.XMLDump,
	})
}

func (h *PhonesHTTP) observe(w http.ResponseWriter, r *http.Request, serial string) {
	if h.observer == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "observer не настроен"})
		return
	}
	timeout := observerTimeout(r, 60)
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	screen, err := h.observer.CaptureScreen(ctx, serial)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	ui, err := h.observer.DumpUI(ctx, serial)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"serial":         screen.Serial,
		"minio_key":      screen.MinioKey,
		"screenshot_url": screen.ScreenshotURL,
		"resolution": map[string]int{
			"width":  screen.Width,
			"height": screen.Height,
		},
		"package_name": ui.Package,
		"xml_dump":     ui.XMLDump,
	})
}

func observerTimeout(r *http.Request, defaultSec int) time.Duration {
	timeout := time.Duration(defaultSec) * time.Second
	if raw := r.URL.Query().Get("timeout_sec"); raw != "" {
		if sec, err := strconv.Atoi(raw); err == nil && sec > 0 {
			timeout = time.Duration(sec) * time.Second
		}
	}
	return timeout
}

func (h *PhonesHTTP) executorTap(w http.ResponseWriter, r *http.Request, serial string) {
	if h.executor == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "executor не настроен"})
		return
	}
	var body struct {
		X int32 `json:"x"`
		Y int32 `json:"y"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.X <= 0 || body.Y <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "укажите x и y в JSON"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	res, err := h.executor.Tap(ctx, serial, body.X, body.Y)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"serial": serial, "result": res})
}

func (h *PhonesHTTP) executorSwipe(w http.ResponseWriter, r *http.Request, serial string) {
	if h.executor == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "executor не настроен"})
		return
	}
	var body struct {
		X0 int32 `json:"x0"`
		Y0 int32 `json:"y0"`
		X1 int32 `json:"x1"`
		Y1 int32 `json:"y1"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "укажите x0,y0,x1,y1 в JSON"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	res, err := h.executor.Swipe(ctx, serial, body.X0, body.Y0, body.X1, body.Y1)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"serial": serial, "result": res})
}

func (h *PhonesHTTP) executorKey(w http.ResponseWriter, r *http.Request, serial string) {
	if h.executor == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "executor не настроен"})
		return
	}
	var body struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Key == "" {
		key := r.URL.Query().Get("key")
		if key == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "укажите key в JSON или query"})
			return
		}
		body.Key = key
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	res, err := h.executor.Key(ctx, serial, body.Key)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"serial": serial, "result": res})
}

func (h *PhonesHTTP) phoneWifi(w http.ResponseWriter, r *http.Request, serial string) {
	if h.connector == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "connector не настроен"})
		return
	}
	var body struct {
		Action   string `json:"action"`
		SSID     string `json:"ssid"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Action == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "укажите action (enable|disable) в JSON"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	var err error
	switch strings.ToLower(body.Action) {
	case "enable":
		err = h.connector.EnableWiFi(ctx, serial, body.SSID, body.Password)
	case "disable":
		err = h.connector.DisableWiFi(ctx, serial)
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "action должен быть enable или disable"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"serial": serial, "status": body.Action})
}
