// NATS broker для локальной разработки и e2e (порт 4222).
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/nats-io/nats-server/v2/server"
)

func main() {
	port := 4222
	if p := os.Getenv("NATS_PORT"); p != "" {
		fmt.Sscanf(p, "%d", &port)
	}

	opts := &server.Options{
		Host: "127.0.0.1",
		Port: port,
	}
	s, err := server.NewServer(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nats server: %v\n", err)
		os.Exit(1)
	}
	s.Start()
	if !s.ReadyForConnections(5 * 1e9) {
		fmt.Fprintln(os.Stderr, "nats server not ready")
		os.Exit(1)
	}
	fmt.Printf("nats-dev listening on nats://127.0.0.1:%d\n", port)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	s.Shutdown()
}
