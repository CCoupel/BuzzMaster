// Package vplayer builds the individualized UPDATE payloads sent to VPlayer
// tablets (#127, contract vplayer-payload-filter.md). Pure functions, no
// WebSocketHub/network involvement — extracted from cmd/server (#291, Phase B).
package vplayer

import (
	"encoding/json"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
)

// BuildPayloads is the CPU-bound core of the #127 individualized
// VPlayer fan-out: one reduced payload per recipient (contract §2), given
// the full UPDATE message and the set of identified VPlayer recipients.
// Deliberately factored out of broadcastUpdateToVPlayers, with no
// WebSocketHub/network involvement at all, so BenchmarkVPlayerFanout (T2.4)
// measures exactly this cost in isolation.
//
// ok=false means msg doesn't qualify for reduction (not an UPDATE, GAME.PHASE
// outside PREPARE/READY, or malformed JSON) — the caller falls back to the
// shared complete payload for every VPlayer instead.
//
// Perf-critical (CA10/R3, contract §3): GAME and teams are extracted once as
// json.RawMessage and never re-parsed/re-marshaled as a whole per recipient
// (up to ~11KB for a MEMOTION question) — only the small "bumpers"
// sub-object is rebuilt per recipient.
func BuildPayloads(msg *protocol.Message, recipients []server.VPlayerRecipient) (map[string][]byte, bool) {
	if msg.Action != protocol.ActionUpdate {
		return nil, false
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(msg.Msg, &envelope); err != nil {
		return nil, false
	}

	var phaseProbe struct {
		Phase string `json:"PHASE"`
	}
	if err := json.Unmarshal(envelope["GAME"], &phaseProbe); err != nil {
		return nil, false
	}
	// #258: COUNTDOWN / STARTED / PAUSED are "answers open" phases too: each tablette
	// gets its OWN frame (own team intact, other teams neutralised), built by the
	// reference serializer. STOPPED and later keep the shared frame (fallback).
	switch phaseProbe.Phase {
	case string(game.PhaseCountdown), string(game.PhaseStarted), string(game.PhasePaused):
		return buildVPlayerOpenPhasePayloads(msg, envelope, phaseProbe.Phase, recipients)
	case string(game.PhasePrepare), string(game.PhaseReady):
	default:
		return nil, false
	}

	var bumpers map[string]json.RawMessage
	if err := json.Unmarshal(envelope["bumpers"], &bumpers); err != nil {
		return nil, false
	}

	// #127 T2.4 checkpoint follow-up: BenchmarkVPlayerFanout showed the
	// obvious approach (build a map[string]json.RawMessage per recipient and
	// hand it to json.Marshal / Message.SerializeForWebSocket) was ~10-11x
	// costlier than the pre-#127 shared path for a MEMOTION question at 30
	// VPlayers (~3.3-3.7ms vs ~0.32ms) — encoding/json calls compact() on
	// every json.RawMessage value it marshals, an O(len) rescan of GAME
	// (up to ~11KB) repeated per recipient, not the re-parse we'd guarded
	// against but a real cost anyway. gameRaw/teamsRaw/actionJSON/versionJSON
	// are computed ONCE here (constant across recipients); the per-recipient
	// loop below builds each final frame by byte concatenation instead
	// (see buildVPlayerMessageBytes) — no json.Marshal ever touches
	// GAME/teams again after this point. Only the small per-recipient bumper
	// object (a few hundred bytes) and the playerID key still go through
	// json.Marshal — cheap, and it's the standard library's own string
	// escaping, reused rather than hand-rolled, for the "no raw string
	// concatenation of user-controlled data" rule this project follows.
	// See _work/reports/dev-backend-t2-benchmark-*.md for the byte-for-byte
	// equality tests and rebenchmark this rewrite was validated against.
	actionJSON, err := json.Marshal(msg.Action)
	if err != nil {
		return nil, false
	}
	var versionJSON []byte
	if msg.Version != "" {
		if versionJSON, err = json.Marshal(msg.Version); err != nil {
			return nil, false
		}
	}
	// Strip admin-only GAME fields (QUIZ_OBJECTIVES, ENTRACTE_CONFIG_SAVED)
	// AND VPlayer-hidden fields (ARDOISE_ANSWERS, #128) once here, before the
	// per-recipient loop — this hot path bypasses SerializeForWebClient/
	// SerializeForVPlayer entirely (it keeps GAME as json.RawMessage and
	// splices it in verbatim, see buildVPlayerMessageBytes below), so
	// without this call gameRaw would carry both to every VPlayer during
	// PREPARE/READY — the confidentiality/exclusivity rules from
	// contracts/game-state.md and vplayer-payload-filter.md §6 would be
	// enforced everywhere except here.
	gameRaw, err := stripVPlayerHiddenGameFields(envelope["GAME"])
	if err != nil {
		return nil, false
	}
	teamsRaw := envelope["teams"]
	// #258: shared parts of the PREPARE/READY frame go through the same single filter,
	// once per fan-out (recipient "": every team's answer/time fields neutral — they are
	// zero after Ready() anyway; QUESTION.QCM_CORRECT / ANSWER removed).
	var gameNode, teamsNode interface{}
	if json.Unmarshal(gameRaw, &gameNode) == nil {
		root := map[string]interface{}{"GAME": gameNode}
		if teamsRaw != nil && json.Unmarshal(teamsRaw, &teamsNode) == nil {
			root["teams"] = teamsNode
		}
		protocol.FilterVPlayerAnswers(root, "", phaseProbe.Phase)
		if g, err := json.Marshal(root["GAME"]); err == nil {
			gameRaw = g
		}
		if _, ok := root["teams"]; ok {
			if t, err := json.Marshal(root["teams"]); err == nil {
				teamsRaw = t
			}
		}
	}

	payloads := make(map[string][]byte, len(recipients))
	for _, r := range recipients {
		ownRaw, present := bumpers[r.PlayerID]
		if !present {
			// Evicted between GetGameJSON() and this call — SendRawToVPlayers
			// falls back to the complete payload for this recipient.
			continue
		}
		var own map[string]interface{}
		if err := json.Unmarshal(ownRaw, &own); err != nil {
			continue
		}
		for _, field := range protocol.AdminOnlyBumperFields {
			delete(own, field)
		}
		strippedBumper, err := json.Marshal(own)
		if err != nil {
			continue
		}
		playerIDJSON, err := json.Marshal(r.PlayerID)
		if err != nil {
			continue
		}
		payloads[r.PlayerID] = buildVPlayerMessageBytes(actionJSON, versionJSON, gameRaw, teamsRaw, playerIDJSON, strippedBumper)
	}

	return payloads, true
}

// buildVPlayerOpenPhasePayloads builds the per-recipient frames of an UPDATE sent while
// answers are open (COUNTDOWN / STARTED / PAUSED, #258): GAME is filtered and
// serialised ONCE (QCM_CORRECT/ANSWER removed, admin/vplayer-hidden fields stripped);
// bumpers and teams — small — are decoded, filtered for the recipient
// (protocol.FilterVPlayerAnswers: own team intact, other teams neutral) and
// serialised per tablette. Frame layout is the one of buildVPlayerMessageBytes.
func buildVPlayerOpenPhasePayloads(msg *protocol.Message, envelope map[string]json.RawMessage, phase string, recipients []server.VPlayerRecipient) (map[string][]byte, bool) {
	actionJSON, err := json.Marshal(msg.Action)
	if err != nil {
		return nil, false
	}
	var versionJSON []byte
	if msg.Version != "" {
		if versionJSON, err = json.Marshal(msg.Version); err != nil {
			return nil, false
		}
	}
	gameRaw, err := stripVPlayerHiddenGameFields(envelope["GAME"])
	if err != nil {
		return nil, false
	}
	var gameNode interface{}
	if json.Unmarshal(gameRaw, &gameNode) != nil {
		return nil, false
	}
	gameRoot := map[string]interface{}{"GAME": gameNode}
	protocol.FilterVPlayerAnswers(gameRoot, "", phase)
	if gameRaw, err = json.Marshal(gameRoot["GAME"]); err != nil {
		return nil, false
	}

	// bumpers / teams decoded ONCE (admin-only bumper fields stripped once); each
	// recipient works on a deep copy it is free to neutralise in place.
	var bumpersAny, teamsAny interface{}
	if json.Unmarshal(envelope["bumpers"], &bumpersAny) != nil {
		return nil, false
	}
	baseBumpers, ok := bumpersAny.(map[string]interface{})
	if !ok {
		return nil, false
	}
	for _, b := range baseBumpers {
		if bumper, ok := b.(map[string]interface{}); ok {
			for _, field := range protocol.AdminOnlyBumperFields {
				delete(bumper, field)
			}
		}
	}
	hasTeams := envelope["teams"] != nil && json.Unmarshal(envelope["teams"], &teamsAny) == nil

	payloads := make(map[string][]byte, len(recipients))
	for _, r := range recipients {
		if _, present := baseBumpers[r.PlayerID]; !present {
			continue // evicted since GetGameJSON(): SendRawToVPlayers falls back to the shared frame
		}
		root := map[string]interface{}{"bumpers": deepCopyJSON(baseBumpers)}
		if hasTeams {
			root["teams"] = deepCopyJSON(teamsAny)
		}
		protocol.FilterVPlayerAnswers(root, r.PlayerID, phase)
		bumpersRaw, err1 := json.Marshal(root["bumpers"])
		teamsRaw, err2 := json.Marshal(root["teams"])
		if err1 != nil || err2 != nil {
			continue
		}
		buf := make([]byte, 0, len(gameRaw)+len(bumpersRaw)+len(teamsRaw)+len(actionJSON)+len(versionJSON)+64)
		buf = append(buf, `{"ACTION":`...)
		buf = append(buf, actionJSON...)
		if len(versionJSON) > 0 {
			buf = append(buf, `,"VERSION":`...)
			buf = append(buf, versionJSON...)
		}
		buf = append(buf, `,"MSG":{"GAME":`...)
		buf = append(buf, gameRaw...)
		buf = append(buf, `,"bumpers":`...)
		buf = append(buf, bumpersRaw...)
		buf = append(buf, `,"teams":`...)
		buf = append(buf, teamsRaw...)
		buf = append(buf, `}}`...)
		payloads[r.PlayerID] = buf
	}
	return payloads, true
}

// deepCopyJSON copies a decoded JSON tree (maps, slices, scalars).
func deepCopyJSON(v interface{}) interface{} {
	switch x := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(x))
		for k, e := range x {
			out[k] = deepCopyJSON(e)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(x))
		for i, e := range x {
			out[i] = deepCopyJSON(e)
		}
		return out
	default:
		return v
	}
}

// stripVPlayerHiddenGameFields returns a copy of the "GAME" node JSON with
// BOTH protocol.AdminOnlyGameFields (QUIZ_OBJECTIVES, ENTRACTE_CONFIG_SAVED)
// AND protocol.VPlayerOnlyGameFields (ARDOISE_ANSWERS, #128) removed —
// renamed from stripAdminOnlyGameFields (v6.5.2, #128) because it always
// served the VPlayer fan-out path exclusively, and a name mentioning only
// "admin-only" undersold what it actually strips once VPlayerOnlyGameFields
// existed to strip too. Called once per broadcastUpdateToVPlayers fan-out
// (not per recipient) by BuildPayloads, since this hot path keeps
// GAME as json.RawMessage and splices it into every recipient's frame
// verbatim (buildVPlayerMessageBytes) — it never goes through
// Message.SerializeForWebClient/SerializeForVPlayer, which apply the same
// two lists on their own code paths (internal/protocol/messages.go). All
// three sites must agree (contracts/vplayer-payload-filter.md §6 "trois
// sites, une seule liste" — now two lists, same discipline).
func stripVPlayerHiddenGameFields(raw json.RawMessage) (json.RawMessage, error) {
	var gameNode map[string]interface{}
	if err := json.Unmarshal(raw, &gameNode); err != nil {
		return nil, err
	}
	for _, field := range protocol.AdminOnlyGameFields {
		delete(gameNode, field)
	}
	for _, field := range protocol.VPlayerOnlyGameFields {
		delete(gameNode, field)
	}
	return json.Marshal(gameNode)
}

// buildVPlayerMessageBytes assembles the final WebSocket frame for one
// VPlayer recipient by direct byte concatenation — no json.Marshal call ever
// sees gameRaw or teamsRaw here, avoiding the compact()-rescan cost
// identified by BenchmarkVPlayerFanout (see BuildPayloads' comment
// above this call site).
//
// Must stay byte-for-byte identical to the reference (slow) path:
//
//	json.Marshal(&protocol.Message{Action: <action>, Version: <version>, Msg:
//	  mustMarshal(map[string]json.RawMessage{"GAME": gameRaw, "teams": teamsRaw,
//	    "bumpers": mustMarshal(map[string]json.RawMessage{playerID: strippedBumper})})})
//
// — verified by TestBuildVPlayerMessageBytes_MatchesReferencePath
// (cmd/server/vplayer_fanout_bytes_test.go), including playerID/bumper
// content with quotes, backslashes, and non-ASCII characters. This hardcodes
// two format facts that test locks in as a regression guard:
//   - encoding/json marshals map[string]T with keys sorted lexicographically,
//     so "GAME" < "bumpers" < "teams" (uppercase 'G' sorts before lowercase
//     letters in ASCII) is always the actual key order for both the outer
//     MSG object and Message's own field order (Action, then Version if
//     non-empty per its `omitempty` tag, then MSG — struct fields keep
//     declaration order, unlike map keys).
//   - gameRaw/teamsRaw, sourced from json.Unmarshal into a
//     map[string]json.RawMessage, are already the exact compact-JSON byte
//     span of the original message — no whitespace to strip, safe to splice
//     in verbatim.
//
// actionJSON/versionJSON/gameRaw/teamsRaw are identical for every recipient
// of one broadcast (computed once by the caller); playerIDJSON and
// strippedBumper are the only per-recipient inputs.
func buildVPlayerMessageBytes(actionJSON, versionJSON, gameRaw, teamsRaw, playerIDJSON, strippedBumper []byte) []byte {
	size := len(actionJSON) + len(versionJSON) + len(gameRaw) + len(teamsRaw) +
		len(playerIDJSON) + len(strippedBumper) + 64 // +64: literal braces/keys/colons/commas below
	buf := make([]byte, 0, size)
	buf = append(buf, `{"ACTION":`...)
	buf = append(buf, actionJSON...)
	if len(versionJSON) > 0 {
		buf = append(buf, `,"VERSION":`...)
		buf = append(buf, versionJSON...)
	}
	buf = append(buf, `,"MSG":{"GAME":`...)
	buf = append(buf, gameRaw...)
	buf = append(buf, `,"bumpers":{`...)
	buf = append(buf, playerIDJSON...)
	buf = append(buf, ':')
	buf = append(buf, strippedBumper...)
	buf = append(buf, `},"teams":`...)
	buf = append(buf, teamsRaw...)
	buf = append(buf, `}}`...)
	return buf
}
