package server

import "testing"

// waitRegistered attend que le hub ait inscrit au moins `want` clients (#274).
// STUB : signature figée, implémentation à venir.
func waitRegistered(t *testing.T, hub *WebSocketHub, want int) {
	t.Helper()
	_, _ = hub, want
}
