package protocol

import (
	"encoding/json"
	"testing"
)

// #144 (v12.0.0) — ROUE_WHEEL.SPIN_UP_MS et ROUE_WHEEL.FULL_SPEED sont publics : présents pour la TV et la
// tablette (contracts/roue.md §4), la phrase restant absente.
func TestRoueLeak144_SpinUpFieldsReachTVAndTablet(t *testing.T) {
	m := buildRoueSubphaseMsg(t, "SPINNING")
	// buildRoueSubphaseMsg ne pose pas les deux champs : on les ajoute au GAME.ROUE_WHEEL du message.
	var payload map[string]interface{}
	if err := json.Unmarshal(m.Msg, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	g := payload["GAME"].(map[string]interface{})
	wheel := g["ROUE_WHEEL"].(map[string]interface{})
	wheel["SPIN_UP_MS"] = 10000
	wheel["FULL_SPEED"] = false
	raw, _ := json.Marshal(payload)
	m.Msg = raw
	for name, ser := range map[string]func() ([]byte, error){
		"TV":            m.SerializeForTV,
		"VPlayerCommon": m.SerializeForVPlayerCommon,
		"VPlayer(p-1)":  func() ([]byte, error) { return m.SerializeForVPlayer("p-1") },
	} {
		data, err := ser()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		assertNoPhrase(t, name, data)
		w, ok := gameNodeOf(t, data)["ROUE_WHEEL"].(map[string]interface{})
		if !ok {
			t.Fatalf("%s : ROUE_WHEEL absent", name)
		}
		if v, ok := w["SPIN_UP_MS"]; !ok || v != float64(10000) {
			t.Errorf("%s : SPIN_UP_MS doit rester diffusé, obtenu %v", name, v)
		}
		if v, ok := w["FULL_SPEED"]; !ok || v != false {
			t.Errorf("%s : FULL_SPEED doit rester diffusé (false), obtenu %v", name, v)
		}
	}
}
