package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/mobilefarm/af/phone-orchestrator/internal/domain"
)

func (h *PhonesHTTP) phoneApps(w http.ResponseWriter, r *http.Request, serial string, sub []string) {
	if len(sub) == 0 {
		if r.Method != http.MethodGet {
			http.Error(w, "только GET", http.StatusMethodNotAllowed)
			return
		}
		social := domain.PhoneAppsByCategory("social")
		system := domain.PhoneAppsByCategory("system")
		writeJSON(w, http.StatusOK, map[string]any{
			"serial": serial,
			"social": social,
			"system": system,
		})
		return
	}

	switch sub[0] {
	case "open":
		if r.Method != http.MethodPost {
			http.Error(w, "только POST", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Package string `json:"package"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Package) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "укажите package"})
			return
		}
		if h.executor == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "executor не настроен"})
			return
		}
		if err := h.executor.LaunchPackage(r.Context(), serial, body.Package); err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"serial": serial, "package": body.Package, "status": "opened",
		})
	case "close":
		if r.Method != http.MethodPost {
			http.Error(w, "только POST", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Package string `json:"package"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Package) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "укажите package"})
			return
		}
		if h.executor == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "executor не настроен"})
			return
		}
		if err := h.executor.ForceStopPackage(r.Context(), serial, body.Package); err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"serial": serial, "package": body.Package, "status": "closed",
		})
	default:
		http.NotFound(w, r)
	}
}
