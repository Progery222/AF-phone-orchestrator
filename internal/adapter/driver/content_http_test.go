package driver_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mobilefarm/af/phone-orchestrator/internal/adapter/driver"
	"github.com/mobilefarm/af/phone-orchestrator/internal/config"
	"github.com/mobilefarm/af/phone-orchestrator/internal/port"
)

func TestContentHTTP_RegisterAndList(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /content/register", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"content_id": "c1", "serial": "stub", "object_key": "posts/a.jpg", "status": "queued",
		})
	})
	mux.HandleFunc("GET /content/{serial}", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"serial": r.PathValue("serial"), "items": []any{}})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfg := config.Config{ContentDistributorHTTPURL: srv.URL}
	client := driver.NewContentHTTP(cfg)
	item, err := client.Register(context.Background(), port.ContentRegisterRequest{
		Serial: "stub", ObjectKey: "posts/a.jpg", Filename: "a.jpg", MediaType: "photo",
	})
	if err != nil {
		t.Fatal(err)
	}
	if item.ContentID != "c1" {
		t.Fatalf("content_id=%s", item.ContentID)
	}
	items, err := client.ListForSerial(context.Background(), "stub")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("items=%d", len(items))
	}
}
