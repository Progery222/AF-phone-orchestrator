package driver_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	natsio "github.com/nats-io/nats.go"

	"github.com/mobilefarm/af/phone-orchestrator/internal/adapter/driver"
	"github.com/mobilefarm/af/phone-orchestrator/internal/config"
)

func TestContentNATS_PublishDownload(t *testing.T) {
	ns, url := startNATSServer(t)
	defer ns.Shutdown()

	recv := make(chan map[string]string, 1)
	nc, err := natsio.Connect(url)
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	_, err = nc.Subscribe("af.content.download", func(msg *natsio.Msg) {
		var m map[string]string
		if json.Unmarshal(msg.Data, &m) == nil {
			recv <- m
		}
	})
	if err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{
		NATSURL:                    url,
		NATSSubjectContentDownload: "af.content.download",
		NATSSubjectContentDelete:   "af.content.delete",
	}
	pub, cleanup, err := driver.NewContentNATS(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	if err := pub.PublishDownload(context.Background(), "phone_7", "cid-1", ""); err != nil {
		t.Fatal(err)
	}

	select {
	case m := <-recv:
		if m["serial"] != "phone_7" || m["content_id"] != "cid-1" {
			t.Fatalf("msg=%v", m)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
}

func startNATSServer(t *testing.T) (*server.Server, string) {
	t.Helper()
	ns, err := server.NewServer(&server.Options{Port: -1})
	if err != nil {
		t.Fatal(err)
	}
	go ns.Start()
	if !ns.ReadyForConnections(5 * time.Second) {
		t.Fatal("nats not ready")
	}
	return ns, ns.ClientURL()
}
