package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

func main() {
	addr := flag.String("addr", "127.0.0.1", "Bind address")
	port := flag.Int("port", 8081, "HTTP port")
	statsPath := flag.String("stats", "/run/dump1090-fa/stats.json", "Path to stats.json")
	pollInterval := flag.Duration("poll", 60*time.Second, "Stats poll interval (minimum 10s)")
	flag.Parse()

	// Enforce minimum poll interval
	if *pollInterval < 10*time.Second {
		fmt.Fprintf(os.Stderr, "poll interval too short (%v), minimum is 10s\n", *pollInterval)
		os.Exit(1)
	}

	// Verify stats file is readable
	if _, err := os.Stat(*statsPath); err != nil {
		log.Printf("Warning: stats file not accessible: %v", err)
	}

	broker := NewBroker()
	server := NewServer(broker, *statsPath, *pollInterval)

	listenAddr := net.JoinHostPort(*addr, fmt.Sprintf("%d", *port))
	log.Printf("Starting ADS-B dashboard on %s", listenAddr)

	if err := http.ListenAndServe(listenAddr, server); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
