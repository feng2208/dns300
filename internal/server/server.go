package server

import (
	"context"
	"dns300/internal/config"
	"dns300/internal/device"
	"dns300/internal/upstream"
	"log"
	"net"

	"github.com/miekg/dns"
)

type Server struct {
	cfg        *config.Config
	devManager *device.Manager
	client     *upstream.Client
}

func NewServer(cfg *config.Config, devManager *device.Manager, client *upstream.Client) *Server {
	return &Server{
		cfg:        cfg,
		devManager: devManager,
		client:     client,
	}
}

func (s *Server) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	// Parse Client IP
	clientAddr := w.RemoteAddr()
	ipStr, _, err := net.SplitHostPort(clientAddr.String())
	if err != nil {
		log.Printf("Failed to parse client IP: %v", err)
		return
	}
	clientIP := net.ParseIP(ipStr)

	// Determine Upstreams
	var upstreams []string
	var tlsVerify bool = true

	dev := s.devManager.Get(clientIP)
	if dev != nil {
		// Device found, use device settings
		upstreams = dev.Upstreams
		tlsVerify = dev.TLSVerify
	} else {
		// Use default settings
		upstreams = s.cfg.Upstreams
	}

	// Forward Query
	ctx := context.Background()
	resp, err := s.client.Exchange(ctx, r, upstreams, tlsVerify)
	if err != nil {
		log.Printf("Failed to exchange query: %v", err)
		// Return SERVFAIL
		m := new(dns.Msg)
		m.SetRcode(r, dns.RcodeServerFailure)
		w.WriteMsg(m)
		return
	}

	// Write Response
	resp.Id = r.Id // Use request ID
	w.WriteMsg(resp)
}
