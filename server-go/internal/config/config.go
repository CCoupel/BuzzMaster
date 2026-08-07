package config

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// Config holds all server configuration
type Config struct {
	Server       ServerConfig       `json:"server"`
	WiFi         WiFiConfig         `json:"wifi"`
	Game         GameConfig         `json:"game"`
	Storage      StorageConfig      `json:"storage"`
	NeonEffect   NeonEffectConfig   `json:"neon_effect"`
	WiFiDefaults WiFiDefaultsConfig `json:"wifi_defaults"`
	AI           AIConfig           `json:"ai"`
	Version      string             `json:"version"`
}

// AIConfig holds settings for the AI question generator (v6.0.0, #8).
type AIConfig struct {
	AnthropicAPIKey string `json:"anthropic_api_key"`
	Model           string `json:"model"`
	TimeoutSeconds  int    `json:"timeout_seconds"`
	MaxQuestions    int    `json:"max_questions"`
	// ClearAPIKey is a request-only field (POST /config.json): true erases the
	// stored key regardless of AnthropicAPIKey. Never persisted (always false
	// on the in-memory singleton and on disk), never returned by GET.
	ClearAPIKey bool `json:"clear_api_key,omitempty"`
	// APIKeyConfigured is derived, never persisted: set only on the copy built
	// for a GET/POST JSON response (contract ai-generation.md §2).
	APIKeyConfigured bool `json:"api_key_configured,omitempty"`

	// Batched generation (v6.1.0, #137 Phase 1) — provider-agnostic: a single
	// oversized call whose structured-output decoding gets truncated loses
	// the whole response; splitting into shorter sequential calls contains
	// the damage to one batch instead of the entire request. Defaults applied
	// in ApplyDefaults (contract ai-multi-provider.md §2-§4).
	BatchSize              int `json:"batch_size"`               // default 20, bounds 1..50
	InterBatchDelayMs      int `json:"inter_batch_delay_ms"`      // default 60000
	ContextTokenBudget     int `json:"context_token_budget"`      // default 1500 (character/4 estimate)
	MaxConsecutiveFailures int `json:"max_consecutive_failures"`  // default 2

	// Multi-provider selection (v6.1.0, #137 Phase 3) — "anthropic" | "groq".
	// The Groq key follows exactly the same secret rules as AnthropicAPIKey
	// (contract ai-multi-provider.md §8, ai-generation.md §2): never returned
	// by GET, empty in POST preserves it, ClearGroqAPIKey erases it
	// explicitly, GroqAPIKeyConfigured is derived/response-only.
	//
	// NOTE: these fields are config plumbing only at this stage — no Groq API
	// call exists yet (ai_groq.go, Phase 3 T3.2, is gated on the empirical
	// schema/rate-limit calibration of T0.1, which requires a real Groq API
	// key nobody has supplied to this environment yet). Selecting
	// provider="groq" without ai_groq.go implemented is inert until that
	// lands; it does not by itself cause any outbound Groq request.
	Provider             string `json:"provider"`               // default "anthropic"
	GroqAPIKey           string `json:"groq_api_key"`
	GroqModel            string `json:"groq_model"`             // default "openai/gpt-oss-120b"
	GroqAPIKeyConfigured bool   `json:"groq_api_key_configured,omitempty"`
	ClearGroqAPIKey      bool   `json:"clear_groq_api_key,omitempty"`
}

// Environment variable names for the two secrets AIConfig holds (security
// incident 2026-08-07: a Groq key committed in cleartext to a tracked
// config.json blocked the PROD deployment — see docs/ADMIN_GUIDE.md
// "Configurer les clés API IA en production" and
// _work/reports/deployer-20260807-120500-SECURITY-BLOCKER.md). Read fresh on
// every EffectiveXxxAPIKey() call, never cached into the Config struct — see
// that method's doc comment for why that matters.
const (
	EnvAnthropicAPIKey = "BUZZCONTROL_ANTHROPIC_API_KEY"
	EnvGroqAPIKey      = "BUZZCONTROL_GROQ_API_KEY"
)

// EffectiveAnthropicAPIKey resolves the key actually used to call the
// Anthropic API: the BUZZCONTROL_ANTHROPIC_API_KEY environment variable
// takes priority when set, falling back to AnthropicAPIKey (the value
// stored in config.json, settable via the admin UI / POST /config.json —
// that mechanism is unchanged, for local/dev use that knowingly accepts
// the on-disk exposure).
//
// Deliberately NOT resolved once at Load() time and cached into the field
// itself: os.Getenv is read fresh on every call, on the UNMODIFIED
// AnthropicAPIKey field, specifically so the environment-supplied key can
// NEVER end up written back to config.json — Save() persists whatever is
// currently in the Config struct verbatim, and it's called on ANY settings
// change, not just AI ones, so caching the env value into the struct would
// silently leak it to disk the next time an unrelated setting was saved.
func (a AIConfig) EffectiveAnthropicAPIKey() string {
	if v := os.Getenv(EnvAnthropicAPIKey); v != "" {
		return v
	}
	return a.AnthropicAPIKey
}

// EffectiveGroqAPIKey is EffectiveAnthropicAPIKey's Groq counterpart.
func (a AIConfig) EffectiveGroqAPIKey() string {
	if v := os.Getenv(EnvGroqAPIKey); v != "" {
		return v
	}
	return a.GroqAPIKey
}

// EffectiveAnthropicAPIKeyConfigured/EffectiveGroqAPIKeyConfigured report
// whether a usable key is available from EITHER source (environment or
// config.json) — the correctness signal everywhere the previous plain
// "AnthropicAPIKey != \"\"" check was used: GET /config.json's
// api_key_configured (the frontend's sole "✨ Générer via IA" enable
// signal, contract ai-generation.md §2) and the no_api_key 409 gate
// (providerAPIKeyConfigured, ai_provider.go) both need to reflect an
// environment-only deployment as "configured" too.
func (a AIConfig) EffectiveAnthropicAPIKeyConfigured() bool {
	return a.EffectiveAnthropicAPIKey() != ""
}

func (a AIConfig) EffectiveGroqAPIKeyConfigured() bool {
	return a.EffectiveGroqAPIKey() != ""
}

type ServerConfig struct {
	HTTPPort         int    `json:"http_port"`
	WebSocketPath    string `json:"websocket_path"`
	AutoOpenBrowsers bool   `json:"auto_open_browsers"` // Auto-open browsers on startup
	Debug            bool   `json:"debug"`               // Enable debug mode (includes /logs)
	AutoCheckUpdates bool   `json:"auto_check_updates"`  // Auto-check for updates on startup
	// ACK protocol settings (added in v3.8.0)
	AckTimeoutMs  int `json:"ack_timeout_ms"`  // Milliseconds before retrying an unacknowledged message (default 2000)
	AckMaxRetries int `json:"ack_max_retries"` // Maximum retry attempts before giving up (default 3)
}

type WiFiConfig struct {
	SSID     string `json:"ssid"`
	Password string `json:"password"`
}

type WiFiDefaultsConfig struct {
	SSID       string `json:"ssid"`
	Password   string `json:"password"`
	ServerIP   string `json:"server_ip"`
	ServerPort int    `json:"server_port"`
	SSID2      string `json:"ssid2,omitempty"`
	Password2  string `json:"password2,omitempty"`
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

	// api_key_configured is derived and clear_api_key is request-only — neither
	// is ever persisted by Save(), but Load() must not trust a stray value from
	// a hand-edited or foreign config.json either (contract ai-generation.md §1).
	cfg.AI.APIKeyConfigured = false
	cfg.AI.ClearAPIKey = false
	// Same rule for the Groq secret's derived/request-only fields (v6.1.0, #137
	// — contract ai-multi-provider.md §8, same treatment as the Anthropic key).
	cfg.AI.GroqAPIKeyConfigured = false
	cfg.AI.ClearGroqAPIKey = false

	ApplyDefaults(&cfg)

	return &cfg, nil
}

// ApplyDefaults fills every zero-valued field with its default value. It is the
// single source of truth for defaults, shared by Load, by Get's in-memory
// fallback (when config.json cannot be read), and by the additive POST
// /config.json handler (internal/server/http.go) so that a section reset to
// zero before a partial merge picks up sane values instead of staying at Go's
// zero value (contract ai-generation.md §0 — previously duplicated and never
// applied on POST, which is how questions_dir/files_dir ended up empty on disk).
func ApplyDefaults(cfg *Config) {
	if cfg.Server.HTTPPort == 0 {
		cfg.Server.HTTPPort = 80
	}
	if cfg.Server.WebSocketPath == "" {
		cfg.Server.WebSocketPath = "/ws"
	}
	// AutoOpenBrowsers/Debug/AutoCheckUpdates are plain bools: Go's zero value
	// (false) is indistinguishable from an explicit false, so they are not
	// defaulted here (unchanged behavior).

	if cfg.Game.DefaultDelay == 0 {
		cfg.Game.DefaultDelay = 30
	}
	if cfg.Storage.DataDir == "" {
		cfg.Storage.DataDir = "./data"
	}
	if cfg.Storage.QuestionsDir == "" {
		cfg.Storage.QuestionsDir = "./data/files/questions"
	}
	if cfg.Storage.FilesDir == "" {
		cfg.Storage.FilesDir = "./data/files"
	}
	if cfg.Version == "" {
		cfg.Version = "2.46.1"
	}

	// ACK protocol defaults (v3.8.0)
	if cfg.Server.AckTimeoutMs == 0 {
		cfg.Server.AckTimeoutMs = 2000
	}
	if cfg.Server.AckMaxRetries == 0 {
		cfg.Server.AckMaxRetries = 3
	}

	// WiFiDefaults defaults
	if cfg.WiFiDefaults.ServerPort == 0 {
		cfg.WiFiDefaults.ServerPort = 80
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

	// AI generator defaults (v6.0.0, #8 — contract ai-generation.md §1)
	if cfg.AI.Model == "" {
		cfg.AI.Model = "claude-opus-5"
	}
	if cfg.AI.TimeoutSeconds == 0 {
		cfg.AI.TimeoutSeconds = 300
	}
	if cfg.AI.MaxQuestions == 0 {
		cfg.AI.MaxQuestions = 200
	}
	// AnthropicAPIKey has no default (empty = not configured).
	// ClearAPIKey / APIKeyConfigured are request/response-only, never defaulted.

	// Batched generation defaults (v6.1.0, #137 — contract ai-multi-provider.md §2-§4)
	if cfg.AI.BatchSize == 0 {
		cfg.AI.BatchSize = 20
	}
	if cfg.AI.InterBatchDelayMs == 0 {
		cfg.AI.InterBatchDelayMs = 60000
	}
	if cfg.AI.ContextTokenBudget == 0 {
		cfg.AI.ContextTokenBudget = 1500
	}
	if cfg.AI.MaxConsecutiveFailures == 0 {
		cfg.AI.MaxConsecutiveFailures = 2
	}

	// Multi-provider defaults (v6.1.0, #137 — contract ai-multi-provider.md §8)
	if cfg.AI.Provider == "" {
		cfg.AI.Provider = "anthropic"
	}
	if cfg.AI.GroqModel == "" {
		cfg.AI.GroqModel = "openai/gpt-oss-120b"
	}
	// GroqAPIKey has no default (empty = not configured), same as AnthropicAPIKey.
}

var (
	instance   *Config
	instanceMu sync.RWMutex
	once       sync.Once
)

// Get returns the singleton config instance
func Get() *Config {
	// If instance was set via SetInstance, return it directly
	instanceMu.RLock()
	cur := instance
	instanceMu.RUnlock()
	if cur != nil {
		return cur
	}
	once.Do(func() {
		loaded, err := Load("config.json")
		if err != nil {
			log.Printf("Warning: Could not load config.json, using defaults: %v", err)
			loaded = &Config{}
			ApplyDefaults(loaded)
			// AutoOpenBrowsers/AutoCheckUpdates default to true in this
			// fallback path only (bools aren't covered by ApplyDefaults,
			// see comment there) — matches the previous hardcoded literal.
			loaded.Server.AutoOpenBrowsers = true
			loaded.Server.AutoCheckUpdates = true
			loaded.Version = "2.0.0"
		}
		instanceMu.Lock()
		instance = loaded
		instanceMu.Unlock()
	})
	instanceMu.RLock()
	defer instanceMu.RUnlock()
	return instance
}

// SetInstance sets the singleton config instance (must be called before Get)
func SetInstance(cfg *Config) {
	instanceMu.Lock()
	instance = cfg
	instanceMu.Unlock()
}

// Save persists cfg to config.json atomically: it writes to a temporary file
// in the same directory and renames it into place, so a crash or power loss
// mid-write never leaves a truncated/corrupt config.json on disk (contract
// ai-generation.md §0).
func Save(cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	const configPath = "config.json"
	dir := filepath.Dir(configPath)
	if dir == "" {
		dir = "."
	}

	tmp, err := os.CreateTemp(dir, ".config.json.tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}

	if err := os.Rename(tmpPath, configPath); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
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
