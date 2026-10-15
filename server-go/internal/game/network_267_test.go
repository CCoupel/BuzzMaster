package game

import (
	"encoding/json"
	"strings"
	"testing"
)

// #267 — GameState.NETWORK : toujours sérialisé, listes jamais null, miroir hérité, copies isolées.

func TestNetwork267_DefaultIsAlwaysSerializedWithNonNullLists(t *testing.T) {
	e := NewEngine()
	js := string(e.GetGameJSON())
	want := `"NETWORK":{"ONLY_LOCALHOST":false,"IPS":[],"IGNORED_INTERFACES":[],"CHECKED_AT":0}`
	if !strings.Contains(js, want) {
		t.Errorf("NETWORK par défaut absent ou mal formé ; attendu %s dans %s", want, js)
	}
	if !strings.Contains(js, `"NETWORK_ONLY_LOCALHOST":false`) {
		t.Error("le miroir hérité NETWORK_ONLY_LOCALHOST doit rester sérialisé")
	}
}

func TestNetwork267_SetNetworkStateKeepsTheLegacyMirrorAndSerializesTheLists(t *testing.T) {
	e := NewEngine()
	e.SetNetworkState(NetworkState{
		OnlyLocalhost:     true,
		IgnoredInterfaces: []IgnoredInterface{{Name: "docker0", IP: "172.17.0.1"}},
		CheckedAt:         1760345000000,
	})
	if !e.GetNetworkOnlyLocalhost() {
		t.Error("miroir NETWORK_ONLY_LOCALHOST doit suivre NETWORK.ONLY_LOCALHOST")
	}
	js := string(e.GetGameJSON())
	for _, want := range []string{
		`"NETWORK_ONLY_LOCALHOST":true`,
		`"NETWORK":{"ONLY_LOCALHOST":true,"IPS":[],"IGNORED_INTERFACES":[{"NAME":"docker0","IP":"172.17.0.1"}],"CHECKED_AT":1760345000000}`,
	} {
		if !strings.Contains(js, want) {
			t.Errorf("manque %s dans %s", want, js)
		}
	}

	e.SetNetworkState(NetworkState{OnlyLocalhost: false, IPs: []string{"192.168.1.84"}})
	if e.GetNetworkOnlyLocalhost() {
		t.Error("le miroir doit revenir à false")
	}
}

func TestNetwork267_LegacySetterKeepsNetworkInSync(t *testing.T) {
	e := NewEngine()
	e.SetNetworkOnlyLocalhost(true)
	if !e.GetNetworkState().OnlyLocalhost {
		t.Error("SetNetworkOnlyLocalhost doit aussi mettre NETWORK.ONLY_LOCALHOST")
	}
}

func TestNetwork267_GetNetworkStateReturnsAnIsolatedCopy(t *testing.T) {
	e := NewEngine()
	e.SetNetworkState(NetworkState{IPs: []string{"192.168.1.84"}})
	got := e.GetNetworkState()
	got.IPs[0] = "MUTATED"
	if again := e.GetNetworkState(); again.IPs[0] != "192.168.1.84" {
		t.Errorf("le moteur ne doit pas partager ses slices : %v", again.IPs)
	}
}

func TestNetwork267_NilListsMarshalAsEmptyArrays(t *testing.T) {
	raw, err := json.Marshal(NetworkState{})
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"ONLY_LOCALHOST":false,"IPS":[],"IGNORED_INTERFACES":[],"CHECKED_AT":0}` {
		t.Errorf("NetworkState zéro = %s", raw)
	}
}
