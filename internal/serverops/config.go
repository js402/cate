package serverops

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
)

type Config struct {
	DatabaseURL     string `json:"database_url"`
	Port            string `json:"port"`
	Addr            string `json:"addr"`
	AllowedOrigins  string `json:"allowed_origins"`
	AllowedMethods  string `json:"allowed_methods"`
	AllowedHeaders  string `json:"allowed_headers"`
	SigningKey      string `json:"signing_key"`
	EncryptionKey   string `json:"encryption_key"`
	JWTSecret       string `json:"jwt_secret"`
	JWTExpiry       string `json:"jwt_expiry"`
	TiKVPDEndpoint  string `json:"tikv_pd_endpoint"`
	NATSURL         string `json:"nats_url"`
	NATSUser        string `json:"nats_user"`
	NATSPassword    string `json:"nats_password"`
	SecurityEnabled string `json:"security_enabled"`
	OpensearchURL   string `json:"opensearch_url"`
	ProxyOrigin     string `json:"proxy_origin"`
	UIBaseURL       string `json:"ui_base_url"`
}

func LoadConfig(cfg *Config) error {
	config := map[string]string{}
	for _, kvPair := range os.Environ() {
		ar := strings.SplitN(kvPair, "=", 2)
		if len(ar) < 2 {
			continue
		}
		key := strings.ToLower(ar[0])
		value := ar[1]
		config[key] = value
	}

	// Marshal the environment variables and unmarshal into the config struct.
	b, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("this is a bug, loadConfig failed to marshal environment variables: %w", err)
	}
	err = json.Unmarshal(b, cfg)
	if err != nil {
		return fmt.Errorf("this is a bug, loadConfig failed to unmarshal config: %w", err)
	}

	return nil
}

func ValidateConfig(cfg *Config) error {
	if cfg.DatabaseURL == "" {
		return fmt.Errorf("missing required configuration: database_url")
	}
	if cfg.Port == "" {
		return fmt.Errorf("missing required configuration: port")
	}
	if len(cfg.Addr) == 0 {
		cfg.Addr = "0.0.0.0" // Default to all interfaces
	}
	if len(cfg.AllowedMethods) == 0 {
		cfg.AllowedMethods = "GET, POST, PUT, DELETE, OPTIONS"
		log.Println("allowed_methods not set, using default:", cfg.AllowedMethods)
	}
	if len(cfg.AllowedHeaders) == 0 {
		cfg.AllowedHeaders = "Content-Type, Authorization"
		log.Println("allowed_headers not set, using default:", cfg.AllowedHeaders)
	}
	if len(cfg.AllowedOrigins) == 0 {
		cfg.AllowedOrigins = "*" // Default to allow all origins
		log.Println("allowed_origins not set, using default:", cfg.AllowedOrigins)
	}
	// Validate SigningKey: require at least 16 characters.
	if len(cfg.SigningKey) < 16 {
		return fmt.Errorf("missing or invalid required configuration: signing_key (must be at least 16 characters)")
	}
	// Validate EncryptionKey: require at least 16 characters.
	if len(cfg.EncryptionKey) < 16 {
		return fmt.Errorf("missing or invalid required configuration: encryption_key (must be at least 16 characters)")
	}
	// Validate JWTSecret: require at least 16 characters.
	if len(cfg.JWTSecret) < 16 {
		return fmt.Errorf("missing or invalid required configuration: jwt_secret (must be at least 16 characters)")
	}
	// Ensure UIBaseURL is provided for the reverse proxy.
	if cfg.UIBaseURL == "" {
		return fmt.Errorf("missing required configuration: ui_base_url")
	}
	// Validate SecurityEnabled: must be "true" or "false" (case-insensitive).
	secEnabled := strings.ToLower(strings.TrimSpace(cfg.SecurityEnabled))
	if secEnabled != "true" && secEnabled != "false" {
		return fmt.Errorf("invalid configuration: security_enabled must be 'true' or 'false'")
	}

	return nil
}
