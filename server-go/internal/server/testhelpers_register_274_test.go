package server

import (
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// Helpers de test #274 : un dial WebSocket réussit dès l'upgrade HTTP, avant que la boucle
// Run du hub ait inscrit le client dans h.clients. Tout test qui diffuse juste après le dial
// doit donc attendre l'inscription (par condition, jamais par Sleep).

const registerDeadline274 = 3 * time.Second

// waitRegistered attend que le hub ait inscrit au moins `want` clients (échéance 3 s).
func waitRegistered(t *testing.T, hub *WebSocketHub, want int) {
	t.Helper()
	deadline := time.Now().Add(registerDeadline274)
	for hub.ClientCount() < want {
		if time.Now().After(deadline) {
			t.Fatalf("inscription : %d client(s) attendu(s), %d inscrit(s) après %v", want, hub.ClientCount(), registerDeadline274)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// testHubs associe un serveur httptest au hub qu'il sert, pour que dialWSPath (dont la
// signature est partagée) retrouve le hub à attendre.
var testHubs sync.Map // *httptest.Server -> *WebSocketHub

func registerTestHub(srv *httptest.Server, hub *WebSocketHub) { testHubs.Store(srv, hub) }

func hubOf(srv *httptest.Server) *WebSocketHub {
	if v, ok := testHubs.Load(srv); ok {
		return v.(*WebSocketHub)
	}
	return nil
}

// typeCount renvoie le nombre de clients inscrits du type correspondant au chemin du dial
// ("/ws/tv" → TV, "/ws/player" → VPlayer, "/ws/anim" → Anim, sinon Admin : "/ws/admin" et "/").
// Compter par type évite qu'un client d'un autre type qui se déconnecte (dial séquentiels
// d'un même test) fausse l'attente du total.
func typeCount(hub *WebSocketHub, path string) int {
	admin, tv, vplayer, anim := hub.GetClientCounts()
	switch path {
	case "/ws/tv":
		return tv
	case "/ws/player":
		return vplayer
	case "/ws/anim":
		return anim
	}
	return admin
}

// waitRegisteredPath attend que le compteur du type de `path` atteigne `want` (échéance 3 s).
func waitRegisteredPath(t *testing.T, hub *WebSocketHub, path string, want int) {
	t.Helper()
	deadline := time.Now().Add(registerDeadline274)
	for typeCount(hub, path) < want {
		if time.Now().After(deadline) {
			t.Fatalf("inscription %s : %d attendu(s), %d inscrit(s) après %v", path, want, typeCount(hub, path), registerDeadline274)
		}
		time.Sleep(2 * time.Millisecond)
	}
}
