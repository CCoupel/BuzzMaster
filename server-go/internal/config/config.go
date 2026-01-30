package config

import (
	"encoding/json"
	"log"
	"os"
	"sync"
)

// Config holds all server configuration
type Config struct {
	Server  ServerConfig  `json:"server"`
	WiFi    WiFiConfig    `json:"wifi"`
	Game    GameConfig    `json:"game"`
	Storage StorageConfig `json:"storage"`
	Version string        `json:"version"`
}

type ServerConfig struct {
	HTTPPort         int  `json:"http_port"`
	TCPPort          int  `json:"tcp_port"`
	WebSocketPath    string `json:"websocket_path"`
	AutoOpenBrowsers bool `json:"auto_open_browsers"` // Auto-open browsers on startup
	Debug            bool `json:"debug"`               // Enable debug mode (includes /logs)
}

type WiFiConfig struct {
	SSID     string `json:"ssid"`
	Password string `json:"password"`
}

type GameConfig struct {
	DefaultDelay int `json:"default_delay"`
}

type StorageConfig struct {
	DataDir      string `json:"data_dir"`
	QuestionsDir string `json:"questions_dir"`
	FilesDir     string `json:"files_dir"`
}

var (
	instance *Config
	once     sync.Once
)

// Load reads configuration from file
func Load(path string) (*Config, error) {
	var cfg Config

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Set defaults if not specified
	if cfg.Server.HTTPPort == 0 {
		cfg.Server.HTTPPort = 80
	}
	if cfg.Server.TCPPort == 0 {
		cfg.Server.TCPPort = 1234
	}
	if cfg.Server.WebSocketPath == "" {
		cfg.Server.WebSocketPath = "/ws"
	}
	// AutoOpenBrowsers defaults to true if not specified in JSON
	// (Go's zero value for bool is false, so we need explicit handling)
	// This is handled by the JSON unmarshaling - if missing, it will be false
	// To enable by default, we would need a pointer or custom unmarshaling

	if cfg.Game.DefaultDelay == 0 {
		cfg.Game.DefaultDelay = 30
	}
	if cfg.Storage.DataDir == "" {
		cfg.Storage.DataDir = "./data"
	}
	if cfg.Version == "" {
		cfg.Version = "2.0.0"
	}

	return &cfg, nil
}

// Get returns the singleton config instance
func Get() *Config {
	// If instance was set via SetInstance, return it directly
	if instance != nil {
		return instance
	}
	once.Do(func() {
		var err error
		instance, err = Load("config.json")
		if err != nil {
			log.Printf("Warning: Could not load config.json, using defaults: %v", err)
			instance = &Config{
				Server: ServerConfig{
					HTTPPort:         80,
					TCPPort:          1234,
					WebSocketPath:    "/ws",
					AutoOpenBrowsers: true,  // Default: enabled
					Debug:            false, // Default: disabled
				},
				Game: GameConfig{
					DefaultDelay: 30,
				},
				Storage: StorageConfig{
					DataDir:      "./data",
					QuestionsDir: "./data/files/questions",
					FilesDir:     "./data/files",
				},
				Version: "2.0.0",
			}
		}
	})
	return instance
}

// SetInstance sets the singleton config instance (must be called before Get)
func SetInstance(cfg *Config) {
	instance = cfg
}
