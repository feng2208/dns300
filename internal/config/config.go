package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Upstreams []string `yaml:"upstreams"`
	Devices   []Device `yaml:"devices"`
}

type Device struct {
	Name      string   `yaml:"name"`
	IPs       []string `yaml:"ips"`
	Upstreams []string `yaml:"upstreams"`
	TLSVerify *bool    `yaml:"tls-verify,omitempty"` // Use pointer to handle default true
}

// DefaultConfigTemplate is the template used when config file is missing
const DefaultConfigTemplate = `upstreams:
  - 1.0.0.1
  - 8.8.8.8

#devices:
#  - name: example
#    ips:
#      - 192.168.0.23
#      - 192.168.2.11
#    upstreams:
#      - 1.0.0.2
#      - https://cloudflare-dns.com/dns-query
#    tls-verify: true
`

func Load(path string) (*Config, error) {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Create default config file
		if err := os.WriteFile(path, []byte(DefaultConfigTemplate), 0644); err != nil {
			return nil, fmt.Errorf("failed to create default config file: %w", err)
		}
		// Continue to load the newly created file
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}
