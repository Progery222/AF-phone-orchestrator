package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mobilefarm/af/phone-orchestrator/internal/domain"
	"github.com/mobilefarm/af/phone-orchestrator/internal/port"
	"github.com/mobilefarm/af/phone-orchestrator/internal/service"
)

type PhonesHTTP struct {
	phones         *service.PhoneService
	orchestrator   *service.OrchestratorService
	connector      port.ConnectorClient
	observer       port.ObserverClient
	executor       port.ExecutorClient
	content        port.ContentClient
	contacts       port.ContactsClient
	video          port.VideoClient
	scenarios      port.ScenariosClient
	scenarioRunner *service.ScenarioRunner
}

func NewPhonesHTTP(phones *service.PhoneService, orchestrator *service.OrchestratorService, connector port.ConnectorClient, observer port.ObserverClient, executor port.ExecutorClient, content port.ContentClient, contacts port.ContactsClient, video port.VideoClient, scenarios port.ScenariosClient, scenarioRunner *service.ScenarioRunner) *PhonesHTTP {
	return &PhonesHTTP{phones: phones, orchestrator: orchestrator, connector: connector, observer: observer, executor: executor, content: content, contacts: contacts, video: video, scenarios: scenarios, scenarioRunner: scenarioRunner}
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
			if errors.Is(err, domain.ErrSandboxSerial) {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
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
				if errors.Is(err, domain.ErrSandboxSerial) {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
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
		case "stand-seq":
			if r.Method != http.MethodPatch && r.Method != http.MethodPut {
				http.Error(w, "только PATCH", http.StatusMethodNotAllowed)
				return
			}
			var body domain.UpdateStandSeqRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "неверный JSON"})
				return
			}
			phone, err := h.phones.SetStandSeqNumber(r.Context(), serial, body.StandSeqNumber)
			if err != nil {
				if errors.Is(err, domain.ErrPhoneNotFound) {
					writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
					return
				}
				writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, toPhoneJSON(phone))
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
		case "type":
			if r.Method != http.MethodPost {
				http.Error(w, "только POST", http.StatusMethodNotAllowed)
				return
			}
			h.executorType(w, r, serial)
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
		case "content":
			h.phoneContent(w, r, serial, nil)
		case "contacts":
			h.phoneContacts(w, r, serial, nil)
		case "video":
			h.phoneVideo(w, r, serial, nil)
		case "scenarios":
			h.phoneScenarios(w, r, serial, nil)
		case "apps":
			h.phoneApps(w, r, serial, nil)
		default:
			http.NotFound(w, r)
		}
		return
	}
	if len(parts) >= 2 && parts[1] == "content" {
		h.phoneContent(w, r, serial, parts[2:])
		return
	}
	if len(parts) >= 2 && parts[1] == "contacts" {
		h.phoneContacts(w, r, serial, parts[2:])
		return
	}
	if len(parts) >= 2 && parts[1] == "video" {
		h.phoneVideo(w, r, serial, parts[2:])
		return
	}
	if len(parts) >= 2 && parts[1] == "scenarios" {
		h.phoneScenarios(w, r, serial, parts[2:])
		return
	}
	if len(parts) >= 2 && parts[1] == "apps" {
		h.phoneApps(w, r, serial, parts[2:])
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
	ScreenResX         int     `json:"screen_res_x,omitempty"`
	ScreenResY         int     `json:"screen_res_y,omitempty"`
	StandSeqNumber     *int16  `json:"stand_seq_number,omitempty"`
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
		ScreenResX: p.ScreenResX, ScreenResY: p.ScreenResY,
		StandSeqNumber: p.StandSeqNumber,
		Error:          p.LastError, LastErrorHash: p.LastErrorHash,
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

func (h *PhonesHTTP) executorType(w http.ResponseWriter, r *http.Request, serial string) {
	if h.executor == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "executor не настроен"})
		return
	}
	var body struct {
		Text  string `json:"text"`
		Typos *bool  `json:"typos"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Text) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "укажите text в JSON"})
		return
	}
	typos := true
	if body.Typos != nil {
		typos = *body.Typos
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	res, err := h.executor.TypeText(ctx, serial, body.Text, typos)
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

func (h *PhonesHTTP) phoneContent(w http.ResponseWriter, r *http.Request, serial string, sub []string) {
	if h.content == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "content-distributor не настроен"})
		return
	}
	ctx := r.Context()
	if len(sub) == 0 {
		switch r.Method {
		case http.MethodGet:
			items, err := h.content.ListForSerial(ctx, serial)
			if err != nil {
				writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"serial": serial, "items": items})
		case http.MethodDelete:
			if err := h.content.DeleteForSerial(ctx, serial); err != nil {
				writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"serial": serial, "message": "контент удалён"})
		default:
			http.Error(w, "метод не поддерживается", http.StatusMethodNotAllowed)
		}
		return
	}
	switch sub[0] {
	case "register":
		if r.Method != http.MethodPost {
			http.Error(w, "только POST", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			ObjectKey string `json:"object_key"`
			Filename  string `json:"filename"`
			MediaType string `json:"media_type"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ObjectKey == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "укажите object_key (файл уже в MinIO)"})
			return
		}
		item, err := h.content.Register(ctx, port.ContentRegisterRequest{
			Serial: serial, ObjectKey: body.ObjectKey, Filename: body.Filename, MediaType: body.MediaType,
		})
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, item)
	case "download":
		if r.Method != http.MethodPost {
			http.Error(w, "только POST", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			ContentID string `json:"content_id"`
			ObjectKey string `json:"object_key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "некорректный JSON"})
			return
		}
		if body.ContentID == "" && body.ObjectKey == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "укажите content_id или object_key"})
			return
		}
		if err := h.content.DownloadAsync(ctx, serial, body.ContentID, body.ObjectKey); err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{
			"serial": serial, "content_id": body.ContentID, "object_key": body.ObjectKey, "status": "accepted",
			"message": "загрузка на телефон запущена",
		})
	case "device":
		if r.Method != http.MethodDelete {
			http.Error(w, "только DELETE", http.StatusMethodNotAllowed)
			return
		}
		if err := h.content.DeleteDeviceForSerial(ctx, serial); err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"serial": serial, "message": "контент удалён с телефона"})
	case "storage":
		if r.Method != http.MethodDelete {
			http.Error(w, "только DELETE", http.StatusMethodNotAllowed)
			return
		}
		if err := h.content.DeleteStorageForSerial(ctx, serial, r.URL.Query().Get("extra_object_key")); err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"serial": serial, "message": "контент удалён из хранилища"})
	default:
		if r.Method != http.MethodDelete {
			http.Error(w, "только DELETE", http.StatusMethodNotAllowed)
			return
		}
		if err := h.content.DeleteByContentID(ctx, serial, sub[0]); err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"serial": serial, "content_id": sub[0], "message": "удалено"})
	}
}

func (h *PhonesHTTP) phoneContacts(w http.ResponseWriter, r *http.Request, serial string, sub []string) {
	if h.contacts == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "contacts-manager не настроен"})
		return
	}
	ctx := r.Context()
	if len(sub) == 0 {
		switch r.Method {
		case http.MethodGet:
			items, state, err := h.contacts.ListContacts(ctx, serial)
			if err != nil {
				writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"serial": serial, "contacts": items, "state": state})
		default:
			http.Error(w, "метод не поддерживается", http.StatusMethodNotAllowed)
		}
		return
	}
	switch sub[0] {
	case "upload":
		if r.Method != http.MethodPost {
			http.Error(w, "только POST", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Source      string   `json:"source"`
			GroupFilter []string `json:"group_filter"`
			VCardKey    string   `json:"vcard_key"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		state, err := h.contacts.Upload(ctx, serial, body.Source, body.GroupFilter, body.VCardKey)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, state)
	case "sync":
		if r.Method != http.MethodPost {
			http.Error(w, "только POST", http.StatusMethodNotAllowed)
			return
		}
		state, err := h.contacts.Sync(ctx, serial)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, state)
	case "merge":
		if r.Method != http.MethodPost {
			http.Error(w, "только POST", http.StatusMethodNotAllowed)
			return
		}
		state, err := h.contacts.Merge(ctx, serial)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, state)
	case "groups":
		if r.Method != http.MethodPost {
			http.Error(w, "только POST", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Groups []struct {
				Name       string   `json:"name"`
				ContactIDs []string `json:"contact_ids"`
			} `json:"groups"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Groups) == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "укажите groups"})
			return
		}
		groups := make([]port.ContactGroup, len(body.Groups))
		for i, g := range body.Groups {
			groups[i] = port.ContactGroup{Name: g.Name, ContactIDs: g.ContactIDs}
		}
		state, err := h.contacts.ApplyGroups(ctx, serial, groups)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, state)
	case "export":
		if r.Method != http.MethodGet {
			http.Error(w, "только GET", http.StatusMethodNotAllowed)
			return
		}
		format := r.URL.Query().Get("format")
		data, outFmt, err := h.contacts.Export(ctx, serial, format)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "text/vcard; charset=utf-8")
		w.Header().Set("X-Export-Format", outFmt)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	default:
		if r.Method != http.MethodDelete {
			http.Error(w, "только DELETE", http.StatusMethodNotAllowed)
			return
		}
		if err := h.contacts.DeleteContact(ctx, sub[0]); err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"serial": serial, "contact_id": sub[0], "message": "удалено"})
	}
}

func (h *PhonesHTTP) phoneVideo(w http.ResponseWriter, r *http.Request, serial string, sub []string) {
	if h.video == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "video-generator не настроен"})
		return
	}
	ctx := r.Context()
	if len(sub) == 0 {
		http.Error(w, "укажите подпуть: screenshots, ai, edit, jobs/{id}", http.StatusNotFound)
		return
	}
	switch sub[0] {
	case "screenshots":
		if r.Method != http.MethodPost {
			http.Error(w, "только POST", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			ScreenshotKeys []string                `json:"screenshot_keys"`
			AudioKey       string                  `json:"audio_key"`
			OverlayText    string                  `json:"overlay_text"`
			Profile        port.VideoOutputProfile `json:"profile"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.ScreenshotKeys) == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "укажите screenshot_keys"})
			return
		}
		job, err := h.video.CreateFromScreenshots(ctx, serial, body.ScreenshotKeys, body.AudioKey, body.OverlayText, body.Profile)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusAccepted, job)
	case "ai":
		if r.Method != http.MethodPost {
			http.Error(w, "только POST", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Prompt      string                  `json:"prompt"`
			Provider    string                  `json:"provider"`
			DurationSec float64                 `json:"duration_sec"`
			Profile     port.VideoOutputProfile `json:"profile"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Prompt == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "укажите prompt"})
			return
		}
		job, err := h.video.GenerateAI(ctx, serial, body.Prompt, body.Provider, body.DurationSec, body.Profile)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusAccepted, job)
	case "edit":
		if r.Method != http.MethodPost {
			http.Error(w, "только POST", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			SourceKey  string             `json:"source_key"`
			Operations []port.VideoEditOp `json:"operations"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.SourceKey == "" || len(body.Operations) == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "укажите source_key и operations"})
			return
		}
		job, err := h.video.EditVideo(ctx, serial, body.SourceKey, body.Operations)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusAccepted, job)
	case "jobs":
		if len(sub) < 2 || sub[1] == "" {
			http.NotFound(w, r)
			return
		}
		jobID := sub[1]
		switch r.Method {
		case http.MethodGet:
			job, err := h.video.GetJob(ctx, jobID)
			if err != nil {
				writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, job)
		case http.MethodDelete:
			if err := h.video.DeleteVideo(ctx, jobID); err != nil {
				writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"id": jobID, "deleted": "true"})
		default:
			http.Error(w, "метод не поддерживается", http.StatusMethodNotAllowed)
		}
	default:
		http.NotFound(w, r)
	}
}

func (h *PhonesHTTP) phoneScenarios(w http.ResponseWriter, r *http.Request, serial string, sub []string) {
	if h.scenarios == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "AF-scenarios не настроен"})
		return
	}
	ctx := r.Context()
	if len(sub) == 0 {
		if r.Method != http.MethodGet {
			http.Error(w, "только GET", http.StatusMethodNotAllowed)
			return
		}
		list, err := h.scenarios.ListForSerial(ctx, serial)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, list)
		return
	}
	if sub[0] == "active" {
		switch r.Method {
		case http.MethodGet:
			list, err := h.scenarios.ListForSerial(ctx, serial)
			if err != nil {
				writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"serial": serial, "active_scenario_id": list.ActiveScenarioID})
		case http.MethodPut:
			var body struct {
				ScenarioID string `json:"scenario_id"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ScenarioID == "" {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "укажите scenario_id"})
				return
			}
			if err := h.scenarios.SetActiveScenario(ctx, serial, body.ScenarioID); err != nil {
				writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"message": "активный сценарий установлен", "active_scenario_id": body.ScenarioID})
		default:
			http.Error(w, "GET/PUT active", http.StatusMethodNotAllowed)
		}
		return
	}
	if sub[0] == "validate" {
		if r.Method != http.MethodPost {
			http.Error(w, "только POST", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			ScenarioYAML  string `json:"scenario_yaml"`
			VariablesYAML string `json:"variables_yaml"`
			Normalize     bool   `json:"normalize"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ScenarioYAML == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "укажите scenario_yaml"})
			return
		}
		result, err := h.scenarios.Validate(ctx, serial, body.ScenarioYAML, body.VariablesYAML, body.Normalize)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, result)
		return
	}
	if sub[0] == "generate" {
		if r.Method != http.MethodPost {
			http.Error(w, "только POST", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Prompt string `json:"prompt"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Prompt == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "укажите prompt"})
			return
		}
		payload, err := h.scenarios.GenerateFull(ctx, serial, body.Prompt)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, payload)
		return
	}
	if sub[0] == "run-step" {
		if r.Method != http.MethodPost {
			http.Error(w, "только POST", http.StatusMethodNotAllowed)
			return
		}
		if h.scenarioRunner == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "scenario runner не настроен"})
			return
		}
		var body struct {
			ScenarioID     string            `json:"scenario_id"`
			StepID         string            `json:"step_id"`
			Action         string            `json:"action"`
			Params         map[string]string `json:"params"`
			Uses           string            `json:"uses"`
			VariablesYAML  string            `json:"variables_yaml"`
			ScenarioYAML   string            `json:"scenario_yaml"`
			ScreenshotKeys []string          `json:"screenshot_keys"`
			VideoOutputKey string            `json:"video_output_key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ScenarioID == "" || body.StepID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "укажите scenario_id и step_id"})
			return
		}
		action := body.Action
		if action == "" {
			action = body.Params["action"]
		}
		params := body.Params
		if params == nil {
			params = map[string]string{}
		}
		scenarioYAML := body.ScenarioYAML
		if scenarioYAML == "" && h.scenarios != nil {
			files, err := h.scenarios.Get(ctx, serial, body.ScenarioID)
			if err == nil {
				scenarioYAML = files.ScenarioYAML
				if body.VariablesYAML == "" {
					body.VariablesYAML = files.VariablesYAML
				}
			}
		}
		var mergeErr error
		action, body.Uses, params, mergeErr = service.MergeStepRequest(scenarioYAML, body.StepID, action, body.Uses, params)
		if mergeErr != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": mergeErr.Error()})
			return
		}
		if action == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "укажите action или корректный step_id в scenario.yaml"})
			return
		}
		result, err := h.scenarioRunner.RunStep(ctx, service.ScenarioStepRequest{
			Serial:         serial,
			ScenarioID:     body.ScenarioID,
			StepID:         body.StepID,
			Action:         action,
			Params:         params,
			Uses:           body.Uses,
			VariablesYAML:  body.VariablesYAML,
			ScenarioYAML:   scenarioYAML,
			ScreenshotKeys: body.ScreenshotKeys,
			VideoOutputKey: body.VideoOutputKey,
		})
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]any{
				"serial": serial, "scenario_id": body.ScenarioID, "step_id": body.StepID,
				"status": "failed", "error": err.Error(), "result": result,
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"serial": serial, "scenario_id": body.ScenarioID, "step_id": body.StepID,
			"action": action, "status": result.Status, "message": result.Message,
			"screenshot_keys": result.ScreenshotKeys, "video_job_id": result.VideoJobID,
			"video_output_key": result.VideoOutputKey, "duration_sec": result.DurationSec,
		})
		return
	}
	scenarioID := sub[0]
	if len(sub) == 1 {
		switch r.Method {
		case http.MethodGet:
			files, err := h.scenarios.Get(ctx, serial, scenarioID)
			if err != nil {
				writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, files)
		case http.MethodPut:
			var files port.ScenarioFiles
			if err := json.NewDecoder(r.Body).Decode(&files); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "некорректный JSON"})
				return
			}
			if err := h.scenarios.Put(ctx, serial, scenarioID, files); err != nil {
				writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"message": "сценарий сохранён"})
		case http.MethodDelete:
			if err := h.scenarios.Delete(ctx, serial, scenarioID); err != nil {
				writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"message": "сценарий удалён"})
		default:
			http.Error(w, "метод не поддерживается", http.StatusMethodNotAllowed)
		}
		return
	}
	switch sub[1] {
	case "status":
		if r.Method != http.MethodGet {
			http.Error(w, "только GET", http.StatusMethodNotAllowed)
			return
		}
		st, err := h.scenarios.GetStatus(ctx, serial, scenarioID)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, st)
	case "logs":
		if r.Method != http.MethodGet {
			http.Error(w, "только GET", http.StatusMethodNotAllowed)
			return
		}
		logs, err := h.scenarios.GetLogs(ctx, serial, scenarioID, r.URL.Query().Get("date"))
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"logs": logs})
	default:
		http.NotFound(w, r)
	}
}
