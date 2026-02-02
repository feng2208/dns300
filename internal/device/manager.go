package device

import (
	"dns300/internal/config"
	"net"
)

type Device struct {
	Name      string
	Upstreams []string
	TLSVerify bool
}

type Manager struct {
	ipMap map[string]*Device
}

func NewManager(cfg *config.Config) *Manager {
	m := &Manager{
		ipMap: make(map[string]*Device),
	}

	for _, d := range cfg.Devices {
		tlsVerify := true
		if d.TLSVerify != nil {
			tlsVerify = *d.TLSVerify
		}

		dev := &Device{
			Name:      d.Name,
			Upstreams: d.Upstreams,
			TLSVerify: tlsVerify,
		}

		for _, ipStr := range d.IPs {
			ip := net.ParseIP(ipStr)
			if ip != nil {
				m.ipMap[ip.String()] = dev
			}
		}
	}
	return m
}

func (m *Manager) Get(ip net.IP) *Device {
	if ip == nil {
		return nil
	}
	return m.ipMap[ip.String()]
}
