package protocol

import (
	"encoding/json"
	"testing"
)

// #240 — le forçage de state.Page = GAME est diffusé via GAME.REMOTE. Ce test
// verrouille que les sérialiseurs TV (SerializeForWebClient) et VPlayer
// (SerializeForVPlayerCommon) ne retirent JAMAIS REMOTE : sinon le forçage
// serveur n'atteindrait pas les écrans (plan §4, « à vérifier par un test »).

func TestRemoteField_NeverStrippedFromTVOrVPlayerPayload_AcrossActions(t *testing.T) {
	actions := []string{ActionUpdate, ActionStart, ActionContinue, ActionPause, ActionStop, ActionUpdateTimer}
	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			msg := buildArdoiseLeakMsg(t, action, "PREPARE")
			// injecte REMOTE=GAME dans le nœud GAME du message
			var raw map[string]interface{}
			if err := json.Unmarshal(msg.Msg, &raw); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			raw["GAME"].(map[string]interface{})["REMOTE"] = "GAME"
			b, _ := json.Marshal(raw)
			msg.Msg = b

			for name, ser := range map[string]func() ([]byte, error){
				"TV/anim (SerializeForWebClient)":     msg.SerializeForWebClient,
				"VPlayer (SerializeForVPlayerCommon)": msg.SerializeForVPlayerCommon,
			} {
				data, err := ser()
				if err != nil {
					t.Fatalf("%s: %v", name, err)
				}
				game := gameNodeOf(t, data)
				if game["REMOTE"] != "GAME" {
					t.Errorf("%s / %s: GAME.REMOTE must reach the client (got %v)", name, action, game["REMOTE"])
				}
			}
		})
	}
}

func TestRemoteField_NotInAnyStripList(t *testing.T) {
	for _, f := range AdminOnlyGameFields {
		if f == "REMOTE" {
			t.Error("REMOTE must not be in AdminOnlyGameFields (would hide the forced view from TV/VPlayer)")
		}
	}
	for _, f := range VPlayerOnlyGameFields {
		if f == "REMOTE" {
			t.Error("REMOTE must not be in VPlayerOnlyGameFields (the VPlayer follows the forced view)")
		}
	}
}
