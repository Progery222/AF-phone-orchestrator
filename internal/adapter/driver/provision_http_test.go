package driver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/mobilefarm/af/phone-orchestrator/internal/config"
	"github.com/mobilefarm/af/phone-orchestrator/internal/domain"
)

func TestProvisionHTTP_AdvanceSetup(t *testing.T) {
	var mu sync.Mutex
	started := false
	status := ""

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/health":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/provision":
			mu.Lock()
			started = true
			status = "provisioning"
			mu.Unlock()
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]string{"serial": "DEV-1", "status": "provisioning"})
		case r.Method == http.MethodGet && r.URL.Path == "/status":
			mu.Lock()
			s := status
			st := started
			mu.Unlock()
			if !st {
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "не найдено"})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"serial": "DEV-1", "status": s})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	cfg := config.Config{
		ProvisionerHTTPURL:          srv.URL,
		ProvisionerDefaultProxyIP:   "10.0.0.1",
		ProvisionerDefaultProxyPort: 3128,
	}
	client := NewProvisionHTTP(cfg)
	phone := domain.Phone{Serial: "DEV-1", State: domain.StateWifiSetup}

	next, err := client.AdvanceSetup(context.Background(), phone)
	if err != nil {
		t.Fatal(err)
	}
	if next != domain.StateWifiSetup {
		t.Fatalf("want wifi_setup, got %s", next)
	}
	mu.Lock()
	ok := started
	mu.Unlock()
	if !ok {
		t.Fatal("expected provision POST")
	}

	mu.Lock()
	status = "ready"
	mu.Unlock()

	next, err = client.AdvanceSetup(context.Background(), phone)
	if err != nil {
		t.Fatal(err)
	}
	if next != domain.StateReady {
		t.Fatalf("want ready, got %s", next)
	}
}

func TestProvisionHTTP_AdvanceSetupFailed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/status" {
			_ = json.NewEncoder(w).Encode(map[string]string{
				"serial": "DEV-2", "status": "failed", "error": "vpn не поднялся",
			})
			return
		}
		if r.URL.Path == "/provision" {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewProvisionHTTP(config.Config{
		ProvisionerHTTPURL:          srv.URL,
		ProvisionerDefaultProxyIP:   "10.0.0.1",
		ProvisionerDefaultProxyPort: 3128,
	})
	phone := domain.Phone{Serial: "DEV-2", State: domain.StateWifiSetup}

	_, err := client.AdvanceSetup(context.Background(), phone)
	if err == nil {
		t.Fatal("expected error on failed provision")
	}
}

func TestProvisionHTTP_BuildRequestDefaults(t *testing.T) {
	client := NewProvisionHTTP(config.Config{
		ProvisionerDefaultProxyIP:   "1.2.3.4",
		ProvisionerDefaultProxyPort: 9999,
		ProvisionerDefaultWiFiSSID:  "DefaultWiFi",
	})
	body := client.buildRequest(domain.Phone{Serial: "S1"})
	proxy, _ := body["proxy"].(map[string]any)
	if proxy["ip"] != "1.2.3.4" {
		t.Fatalf("proxy ip: %v", proxy["ip"])
	}
	if body["wifi_ssid"] != "DefaultWiFi" {
		t.Fatalf("wifi: %v", body["wifi_ssid"])
	}
}
