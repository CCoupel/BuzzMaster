package config

import (
	"encoding/json"
	"log"
	"os"
	"sync"
)

// Config holds all server configuration
type Config struct {
	Server     ServerConfig     `json:"server"`
	WiFi       WiFiConfig       `json:"wifi"`
	Game       GameConfig       `json:"game"`
	Storage    StorageConfig    `json:"storage"`
	NeonEffect NeonEffectConfig `json:"neon_effect"`
	Version    string           `json:"version"`
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

type NeonEffectConfig struct {
	Enabled        bool    `json:"enabled"`
	Mode           string  `json:"mode"`             // "halo" or "bar", default "bar"
	ArcWidth       int     `json:"arc_width"`        // 30-180 degrees, default 60
	IntensityGap   int     `json:"intensity_gap"`    // 0-100%, default 80
	RotationSpeed  float64 `json:"rotation_speed"`   // 1-10 seconds, default 4
	BarOffset      int     `json:"bar_offset"`       // 10-100 pixels from edge, default 20
	BarThickness   int     `json:"bar_thickness"`    // 2-20 pixels, default 4
	ArcBlur        int     `json:"arc_blur"`         // 0-200% of bar thickness, default 100
	GlowPulseSpeed float64 `json:"glow_pulse_speed"` // 0.5-5 seconds, default 2
	GlowPulseMin   int     `json:"glow_pulse_min"`   // 0-100%, minimum glow opacity, default 30
	GlowPulseMax   int     `json:"glow_pulse_max"`   // 0-100%, maximum glow opacity, default 50
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
		cfg.Version = "2.46.1"
	}

	// NeonEffect defaults
	if cfg.NeonEffect.Mode == "" {
		cfg.NeonEffect.Mode = "bar" // Default to bar mode
	}
	if cfg.NeonEffect.ArcWidth == 0 {
		cfg.NeonEffect.ArcWidth = 60
	}
	if cfg.NeonEffect.IntensityGap == 0 {
		cfg.NeonEffect.IntensityGap = 80
	}
	if cfg.NeonEffect.RotationSpeed == 0 {
		cfg.NeonEffect.RotationSpeed = 4.0
	}
	if cfg.NeonEffect.BarOffset == 0 {
		cfg.NeonEffect.BarOffset = 20 // 20 pixels from edge
	}
	if cfg.NeonEffect.BarThickness == 0 {
		cfg.NeonEffect.BarThickness = 4 // 4 pixels thick
	}
	if cfg.NeonEffect.ArcBlur == 0 {
		cfg.NeonEffect.ArcBlur = 100 // 100% of bar thickness
	}
	if cfg.NeonEffect.GlowPulseSpeed == 0 {
		cfg.NeonEffect.GlowPulseSpeed = 2.0 // 2 seconds
	}
	if cfg.NeonEffect.GlowPulseMin == 0 {
		cfg.NeonEffect.GlowPulseMin = 30 // 30% minimum glow
	}
	if cfg.NeonEffect.GlowPulseMax == 0 {
		cfg.NeonEffect.GlowPulseMax = 50 // 50% maximum glow
	}
	// Enabled defaults to false (zero value)

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
				NeonEffect: NeonEffectConfig{
					Enabled:        false, // Default: disabled
					Mode:           "bar", // Default: bar mode
					ArcWidth:       60,
					IntensityGap:   80,
					RotationSpeed:  4.0,
					BarOffset:      20,
					BarThickness:   4,
					ArcBlur:        100,
					GlowPulseSpeed: 2.0,
					GlowPulseMin:   30,
					GlowPulseMax:   50,
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

// ValidateAndClampNeonEffect validates and clamps neon effect values to acceptable ranges
func (c *Config) ValidateAndClampNeonEffect() {
	// Validate mode
	if c.NeonEffect.Mode != "halo" && c.NeonEffect.Mode != "bar" {
		c.NeonEffect.Mode = "bar"
	}

	// Clamp arc_width to 30-180 degrees
	if c.NeonEffect.ArcWidth < 30 {
		c.NeonEffect.ArcWidth = 30
	} else if c.NeonEffect.ArcWidth > 180 {
		c.NeonEffect.ArcWidth = 180
	}

	// Clamp intensity_gap to 0-100%
	if c.NeonEffect.IntensityGap < 0 {
		c.NeonEffect.IntensityGap = 0
	} else if c.NeonEffect.IntensityGap > 100 {
		c.NeonEffect.IntensityGap = 100
	}

	// Clamp rotation_speed to 1.0-10.0 seconds
	if c.NeonEffect.RotationSpeed < 1.0 {
		c.NeonEffect.RotationSpeed = 1.0
	} else if c.NeonEffect.RotationSpeed > 10.0 {
		c.NeonEffect.RotationSpeed = 10.0
	}

	// Clamp bar_offset to 10-100 pixels
	if c.NeonEffect.BarOffset < 10 {
		c.NeonEffect.BarOffset = 10
	} else if c.NeonEffect.BarOffset > 100 {
		c.NeonEffect.BarOffset = 100
	}

	// Clamp bar_thickness to 2-20 pixels
	if c.NeonEffect.BarThickness < 2 {
		c.NeonEffect.BarThickness = 2
	} else if c.NeonEffect.BarThickness > 20 {
		c.NeonEffect.BarThickness = 20
	}

	// Clamp arc_blur to 0-200%
	if c.NeonEffect.ArcBlur < 0 {
		c.NeonEffect.ArcBlur = 0
	} else if c.NeonEffect.ArcBlur > 200 {
		c.NeonEffect.ArcBlur = 200
	}

	// Clamp glow_pulse_speed to 0.5-5.0 seconds
	if c.NeonEffect.GlowPulseSpeed < 0.5 {
		c.NeonEffect.GlowPulseSpeed = 0.5
	} else if c.NeonEffect.GlowPulseSpeed > 5.0 {
		c.NeonEffect.GlowPulseSpeed = 5.0
	}

	// Clamp glow_pulse_min to 0-100%
	if c.NeonEffect.GlowPulseMin < 0 {
		c.NeonEffect.GlowPulseMin = 0
	} else if c.NeonEffect.GlowPulseMin > 100 {
		c.NeonEffect.GlowPulseMin = 100
	}

	// Clamp glow_pulse_max to 0-100%
	if c.NeonEffect.GlowPulseMax < 0 {
		c.NeonEffect.GlowPulseMax = 0
	} else if c.NeonEffect.GlowPulseMax > 100 {
		c.NeonEffect.GlowPulseMax = 100
	}

	// Ensure min <= max
	if c.NeonEffect.GlowPulseMin > c.NeonEffect.GlowPulseMax {
		c.NeonEffect.GlowPulseMin, c.NeonEffect.GlowPulseMax = c.NeonEffect.GlowPulseMax, c.NeonEffect.GlowPulseMin
	}
}
