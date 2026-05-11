package main

import "testing"

// TestResolvePort verifies that the --port CLI flag takes precedence over config.json.
func TestResolvePort(t *testing.T) {
	tests := []struct {
		name       string
		configPort int
		flagPort   int
		wantPort   int
	}{
		{
			name:       "flag overrides config",
			configPort: 8080,
			flagPort:   9090,
			wantPort:   9090,
		},
		{
			name:       "no flag — config port used",
			configPort: 8080,
			flagPort:   0,
			wantPort:   8080,
		},
		{
			name:       "flag=0 treated as unset — config port used",
			configPort: 3000,
			flagPort:   0,
			wantPort:   3000,
		},
		{
			name:       "flag and config identical — no change",
			configPort: 8080,
			flagPort:   8080,
			wantPort:   8080,
		},
		{
			name:       "flag=1 (minimal valid port) overrides config",
			configPort: 8080,
			flagPort:   1,
			wantPort:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolvePort(tt.configPort, tt.flagPort)
			if got != tt.wantPort {
				t.Errorf("resolvePort(%d, %d) = %d, want %d",
					tt.configPort, tt.flagPort, got, tt.wantPort)
			}
		})
	}
}
