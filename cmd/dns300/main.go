package main

import (
	"dns300/internal/config"
	"dns300/internal/device"
	"dns300/internal/server"
	"dns300/internal/upstream"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/miekg/dns"
)

var Version = "0.0.0"

func main() {
	configFile := flag.String("config", "config.yaml", "Path to configuration file")
	port := flag.Int("port", 53, "Port to listen on")
	v := flag.Bool("v", false, "Show version")
	version := flag.Bool("version", false, "Show version")
	flag.Parse()

	if *v || *version {
		fmt.Printf("%s\n", Version)
		return
	}

	// Load Configuration
	cfg, err := config.Load(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	log.Printf("Configuration loaded from %s", *configFile)

	// Initialize Components
	devManager := device.NewManager(cfg)
	client := upstream.NewClient()
	srv := server.NewServer(cfg, devManager, client)

	// Start Servers
	addr := fmt.Sprintf(":%d", *port)

	udpServer := &dns.Server{Addr: addr, Net: "udp", Handler: srv}
	tcpServer := &dns.Server{Addr: addr, Net: "tcp", Handler: srv}

	go func() {
		log.Printf("Starting UDP server on %s", addr)
		if err := udpServer.ListenAndServe(); err != nil {
			log.Fatalf("Failed to start UDP server: %v", err)
		}
	}()

	go func() {
		log.Printf("Starting TCP server on %s", addr)
		if err := tcpServer.ListenAndServe(); err != nil {
			log.Fatalf("Failed to start TCP server: %v", err)
		}
	}()

	// Wait for signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Println("Shutting down...")
}
