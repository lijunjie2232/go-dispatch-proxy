package main

import (
	"io/ioutil"
	"log"
	"net"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the structure of the configuration file
type Config struct {
	ListenHost    string               `yaml:"listen_host"`
	ListenPort    int                  `yaml:"listen_port"`
	LoadBalancers []LoadBalancerConfig `yaml:"load_balancers"`
	TunnelMode    bool                 `yaml:"tunnel_mode"`
	QuietMode     bool                 `yaml:"quiet_mode"`
	UseDevices    bool                 `yaml:"use_devices"`
}

// LoadBalancerConfig represents individual load balancer configuration
type LoadBalancerConfig struct {
	Address   string `yaml:"address"` // Can be IP or device@IP format
	Device    string `yaml:"device"`  // Device name (alternative to device@IP format)
	ContRatio int    `yaml:"cont_ratio"`
}

// LoadConfig loads configuration from YAML file
func LoadConfig(filename string) (*Config, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	config := &Config{}
	err = yaml.Unmarshal(data, config)
	if err != nil {
		return nil, err
	}

	// Set default values if not specified
	if config.ListenHost == "" {
		config.ListenHost = "127.0.0.1"
	}
	if config.ListenPort == 0 {
		config.ListenPort = 8080
	}

	return config, nil
}

// ValidateConfig validates the loaded configuration
func ValidateConfig(config *Config) error {
	// Validate listen host
	if net.ParseIP(config.ListenHost).To4() == nil {
		log.Fatal("[FATAL] Invalid host in config: ", config.ListenHost)
	}

	// Validate listen port
	if config.ListenPort < 1 || config.ListenPort > 65535 {
		log.Fatal("[FATAL] Invalid port in config: ", config.ListenPort)
	}

	// Validate load balancers
	if len(config.LoadBalancers) == 0 {
		log.Fatal("[FATAL] Please specify one or more load balancers in config")
	}

	for i, lb := range config.LoadBalancers {
		// Handle device@IP format or separate device field
		address := lb.Address
		device := lb.Device

		if device != "" {
			// If device is specified, use device@IP format
			address = device + "@" + address
		} else if strings.Contains(address, "@") {
			// Already in device@IP format, extract device name
			parts := strings.Split(address, "@")
			if len(parts) != 2 {
				log.Fatal("[FATAL] Invalid address format for load balancer ", i+1, ": ", address)
			}
			device = parts[0]
		}

		if address == "" {
			log.Fatal("[FATAL] Load balancer ", i+1, " has empty address")
		}

		if lb.ContRatio <= 0 {
			log.Fatal("[FATAL] Invalid contention ratio for load balancer ", i+1, ": ", lb.ContRatio)
		}
	}

	return nil
}
