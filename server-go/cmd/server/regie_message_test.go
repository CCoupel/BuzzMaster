package main

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"encoding/json"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// ---------------------------------------------------------------------------
// Tests: v6.4.x (#167) — Messagerie régie, handleRegieMessageSend /
// handleRegieMessageClear (plan tâches B4/T2/T3/T4/T4b,
// contracts/websocket-actions.md §"Messagerie régie").
//
// RegieMessagePayload is reused for the SEND request body (server only reads
// .Text on that direction) — same "shared by both directions" precedent as
// CreditPointsPayload, see broadcast_anim_test.go's comment on the same
// pattern.
//
// Harness: newAnimTestApp/startAnimAllowlistTestServer/dialWS/learnClientID/
// sendAction/readActionMatching/collectActionsT/containsAction — all defined
// in inbound_allowlist_anim_test.go, send_state_to_client_anim_test.go and
// player_evicted_test.go (same package, no local redefinition needed).
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// T2 — SEND: trim, refus du vide, troncature à 140 runes sur texte accentué.
// ---------------------------------------------------------------------------

func TestHandleRegieMessageSend_TrimsSurroundingWhitespace(t *testing.T) {
	app := newAnimTestApp(t)
	baseURL := startAnimAllowlistTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, admin)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)

	sendAction(t, app, admin, protocol.ActionRegieMessageSend, protocol.RegieMessagePayload{Text: "   Question 12 annulée   "})

	_, raw := readActionMatching(t, anim, protocol.ActionRegieMessage)
	var envelope struct {
		Msg protocol.RegieMessagePayload `json:"MSG"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		t.Fatalf("failed to unmarshal REGIE_MESSAGE: %v (raw: %s)", err, raw)
	}
	if envelope.Msg.Text != "Question 12 annulée" {
		t.Errorf("expected leading/trailing whitespace trimmed, got %q", envelope.Msg.Text)
	}
}

// TestHandleRegieMessageSend_EmptyAfterTrim_IgnoredNoStateChange covers the
// contract's rule 2: a text that is empty (or only whitespace) after trim is
// IGNORED — no state change, no diffusion. Sending an empty message never
// erases the currently active one: only REGIE_MESSAGE_CLEAR does that.
func TestHandleRegieMessageSend_EmptyAfterTrim_IgnoredNoStateChange(t *testing.T) {
	for _, text := range []string{"", "   ", "\t\n  "} {
		t.Run("text="+text, func(t *testing.T) {
			app := newAnimTestApp(t)
			baseURL := startAnimAllowlistTestServer(t, app)
			admin := dialWS(t, baseURL, "/ws/admin")
			learnClientID(t, app, admin)
			anim := dialWS(t, baseURL, "/ws/anim")
			learnClientID(t, app, anim)

			// Arm an active message first, so we can also confirm an empty
			// SEND does NOT clear it (contract: "un message vide n'efface
			// pas le message courant : l'effacement passe uniquement par
			// REGIE_MESSAGE_CLEAR"). REGIE_MESSAGE reaches BOTH admin and
			// anim (unlike anim-exclusive actions elsewhere in this
			// codebase) — drain admin's own copy too, or it pollutes a
			// later read on that connection.
			sendAction(t, app, admin, protocol.ActionRegieMessageSend, protocol.RegieMessagePayload{Text: "Consigne existante"})
			readActionMatching(t, admin, protocol.ActionRegieMessage)
			readActionMatching(t, anim, protocol.ActionRegieMessage)

			sendAction(t, app, admin, protocol.ActionRegieMessageSend, protocol.RegieMessagePayload{Text: text})

			animActions := collectActionsT(t, anim, 300*time.Millisecond)
			if containsAction(animActions, protocol.ActionRegieMessage) {
				t.Errorf("an empty (post-trim) SEND must not diffuse anything — got actions: %v", animActions)
			}

			if app.regieMessage == nil || !app.regieMessage.Active || app.regieMessage.Text != "Consigne existante" {
				t.Errorf("an empty (post-trim) SEND must not change the active message — got %+v", app.regieMessage)
			}

			warnCount := 0
			for _, entry := range app.logger.GetRecent(50) {
				if entry.Level == game.LogLevelWarn && strings.Contains(entry.Message, protocol.ActionRegieMessageSend) {
					warnCount++
				}
			}
			if warnCount == 0 {
				t.Error("expected at least one WARN log entry for the ignored empty SEND")
			}
		})
	}
}

// TestHandleRegieMessageSend_TruncatesTo140Runes_AccentedTextStaysValidUTF8
// is the contract's rule 3, on the exact case it warns about: French text
// with 2-byte-per-rune accented characters. A byte-based truncation would
// slice mid-character and produce invalid UTF-8; a rune-based truncation
// (the contract's requirement) must not.
func TestHandleRegieMessageSend_TruncatesTo140Runes_AccentedTextStaysValidUTF8(t *testing.T) {
	app := newAnimTestApp(t)
	baseURL := startAnimAllowlistTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, admin)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)

	// 150 runes, every one of them 2 bytes in UTF-8 (300 bytes total) — a
	// byte-based [:140] slice would land mid-character.
	longAccented := strings.Repeat("é", 150)

	sendAction(t, app, admin, protocol.ActionRegieMessageSend, protocol.RegieMessagePayload{Text: longAccented})

	_, raw := readActionMatching(t, anim, protocol.ActionRegieMessage)
	var envelope struct {
		Msg protocol.RegieMessagePayload `json:"MSG"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		t.Fatalf("failed to unmarshal REGIE_MESSAGE: %v (raw: %s)", err, raw)
	}

	if !utf8.ValidString(envelope.Msg.Text) {
		t.Fatalf("truncated text is not valid UTF-8: %q", envelope.Msg.Text)
	}
	if got := utf8.RuneCountInString(envelope.Msg.Text); got != 140 {
		t.Errorf("expected exactly 140 runes after truncation, got %d (text=%q)", got, envelope.Msg.Text)
	}
	if want := strings.Repeat("é", 140); envelope.Msg.Text != want {
		t.Errorf("truncated text mismatch: got %q, want the first 140 é characters", envelope.Msg.Text)
	}
}

// TestHandleRegieMessageSend_ExactlyAt140Runes_NotTruncated is the boundary
// control for the truncation rule above.
func TestHandleRegieMessageSend_ExactlyAt140Runes_NotTruncated(t *testing.T) {
	app := newAnimTestApp(t)
	baseURL := startAnimAllowlistTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, admin)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)

	exact140 := strings.Repeat("à", 140)
	sendAction(t, app, admin, protocol.ActionRegieMessageSend, protocol.RegieMessagePayload{Text: exact140})

	_, raw := readActionMatching(t, anim, protocol.ActionRegieMessage)
	var envelope struct {
		Msg protocol.RegieMessagePayload `json:"MSG"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		t.Fatalf("failed to unmarshal REGIE_MESSAGE: %v (raw: %s)", err, raw)
	}
	if envelope.Msg.Text != exact140 {
		t.Errorf("a 140-rune text must not be altered: got %d runes, want 140", utf8.RuneCountInString(envelope.Msg.Text))
	}
}

// ---------------------------------------------------------------------------
// T3 — CLEAR: ClearedBy déduit du ClientType ; no-op idempotent si aucun
// message actif.
// ---------------------------------------------------------------------------

func TestHandleRegieMessageClear_ClearedByDeducedFromClientType(t *testing.T) {
	tests := []struct {
		name          string
		connectPath   string
		wantClearedBy string
	}{
		{"anim acquits", "/ws/anim", "ANIM"},
		{"admin retracts", "/ws/admin", "REGIE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newAnimTestApp(t)
			baseURL := startAnimAllowlistTestServer(t, app)
			admin := dialWS(t, baseURL, "/ws/admin")
			learnClientID(t, app, admin)
			anim := dialWS(t, baseURL, "/ws/anim")
			learnClientID(t, app, anim)

			// REGIE_MESSAGE reaches BOTH admin and anim — drain admin's own
			// copy of the SEND broadcast too, or the CLEAR assertion below
			// reads it instead of the CLEAR's broadcast.
			sendAction(t, app, admin, protocol.ActionRegieMessageSend, protocol.RegieMessagePayload{Text: "Consigne à effacer"})
			readActionMatching(t, admin, protocol.ActionRegieMessage)
			readActionMatching(t, anim, protocol.ActionRegieMessage)

			clearer := admin
			if tt.connectPath == "/ws/anim" {
				clearer = anim
			}
			sendAction(t, app, clearer, protocol.ActionRegieMessageClear, protocol.RegieMessagePayload{})

			_, raw := readActionMatching(t, admin, protocol.ActionRegieMessage)
			var envelope struct {
				Msg protocol.RegieMessagePayload `json:"MSG"`
			}
			if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
				t.Fatalf("failed to unmarshal REGIE_MESSAGE: %v (raw: %s)", err, raw)
			}
			if envelope.Msg.Active {
				t.Errorf("cleared message must be Active=false, got %+v", envelope.Msg)
			}
			if envelope.Msg.ClearedBy != tt.wantClearedBy {
				t.Errorf("ClearedBy = %q, want %q — must be DEDUCED from the sender's ClientType, never trusted from the payload", envelope.Msg.ClearedBy, tt.wantClearedBy)
			}
		})
	}
}

// TestHandleRegieMessageClear_ClearedByIgnoresClientSuppliedField guards the
// "jamais lu depuis le payload" half of the rule: even if a client sent a
// CLEARED_BY field in its CLEAR request (it shouldn't, but nothing stops a
// non-conforming client from trying), the server must still deduce it from
// the connection's real ClientType.
func TestHandleRegieMessageClear_ClearedByIgnoresClientSuppliedField(t *testing.T) {
	app := newAnimTestApp(t)
	baseURL := startAnimAllowlistTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, admin)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)

	sendAction(t, app, admin, protocol.ActionRegieMessageSend, protocol.RegieMessagePayload{Text: "Consigne"})
	readActionMatching(t, admin, protocol.ActionRegieMessage)
	readActionMatching(t, anim, protocol.ActionRegieMessage)

	// The anim client lies about who cleared it — must be ignored.
	sendAction(t, app, anim, protocol.ActionRegieMessageClear, protocol.RegieMessagePayload{ClearedBy: "REGIE"})

	_, raw := readActionMatching(t, admin, protocol.ActionRegieMessage)
	var envelope struct {
		Msg protocol.RegieMessagePayload `json:"MSG"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		t.Fatalf("failed to unmarshal REGIE_MESSAGE: %v (raw: %s)", err, raw)
	}
	if envelope.Msg.ClearedBy != "ANIM" {
		t.Errorf("ClearedBy must be deduced from ClientType (ANIM), not trusted from the client payload (%q) — got %q", "REGIE", envelope.Msg.ClearedBy)
	}
}

// TestHandleRegieMessageClear_NoActiveMessage_NoOpIdempotent covers the
// "reçue alors qu'aucun message n'est actif" half of the contract's CLEAR
// sémantique — a fresh app that never sent anything.
func TestHandleRegieMessageClear_NoActiveMessage_NoOpIdempotent(t *testing.T) {
	app := newAnimTestApp(t)
	baseURL := startAnimAllowlistTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, admin)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)

	sendAction(t, app, anim, protocol.ActionRegieMessageClear, protocol.RegieMessagePayload{})

	adminActions := collectActionsT(t, admin, 300*time.Millisecond)
	if containsAction(adminActions, protocol.ActionRegieMessage) {
		t.Errorf("CLEAR with no active message must not diffuse anything — got actions: %v", adminActions)
	}
	animActions := collectActionsT(t, anim, 300*time.Millisecond)
	if containsAction(animActions, protocol.ActionRegieMessage) {
		t.Errorf("CLEAR with no active message must not diffuse anything (self included) — got actions: %v", animActions)
	}
}

// TestHandleRegieMessageClear_TwoSimultaneousAcquittals_OnlyOneDiffusion is
// the contract's own concurrency example: "Deux tablettes qui acquittent au
// même instant ne produisent donc qu'une seule diffusion d'effacement, et
// jamais d'incohérence."
func TestHandleRegieMessageClear_TwoSimultaneousAcquittals_OnlyOneDiffusion(t *testing.T) {
	app := newAnimTestApp(t)
	baseURL := startAnimAllowlistTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, admin)
	animA := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, animA)
	animB := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, animB)

	// admin is not a party to the race — it's the clean observation point:
	// drain its own copy of the SEND broadcast, then watch it for exactly
	// one further REGIE_MESSAGE (the first CLEAR), and none after.
	sendAction(t, app, admin, protocol.ActionRegieMessageSend, protocol.RegieMessagePayload{Text: "Consigne"})
	readActionMatching(t, admin, protocol.ActionRegieMessage)

	// Tablet A acquits first.
	sendAction(t, app, animA, protocol.ActionRegieMessageClear, protocol.RegieMessagePayload{})
	readActionMatching(t, admin, protocol.ActionRegieMessage) // the one legitimate clear diffusion

	// Tablet B, unaware A already cleared it, acquits "at the same time".
	sendAction(t, app, animB, protocol.ActionRegieMessageClear, protocol.RegieMessagePayload{})

	adminActions := collectActionsT(t, admin, 300*time.Millisecond)
	if containsAction(adminActions, protocol.ActionRegieMessage) {
		t.Errorf("a second CLEAR on an already-inactive message must not produce a second diffusion — got actions: %v", adminActions)
	}
}

// ---------------------------------------------------------------------------
// T4 — Remplacement : un SEND de texte différent écrase le contenu et réarme
// SENT_AT, sans file.
// ---------------------------------------------------------------------------

func TestHandleRegieMessageSend_DifferentText_ReplacesAndRearmsSentAt(t *testing.T) {
	app := newAnimTestApp(t)
	baseURL := startAnimAllowlistTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, admin)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)

	sendAction(t, app, admin, protocol.ActionRegieMessageSend, protocol.RegieMessagePayload{Text: "Première consigne"})
	_, raw1 := readActionMatching(t, anim, protocol.ActionRegieMessage)
	var first struct {
		Msg protocol.RegieMessagePayload `json:"MSG"`
	}
	if err := json.Unmarshal([]byte(raw1), &first); err != nil {
		t.Fatalf("failed to unmarshal first REGIE_MESSAGE: %v", err)
	}

	time.Sleep(5 * time.Millisecond) // guarantee a distinct millisecond timestamp

	sendAction(t, app, admin, protocol.ActionRegieMessageSend, protocol.RegieMessagePayload{Text: "Deuxième consigne, différente"})
	_, raw2 := readActionMatching(t, anim, protocol.ActionRegieMessage)
	var second struct {
		Msg protocol.RegieMessagePayload `json:"MSG"`
	}
	if err := json.Unmarshal([]byte(raw2), &second); err != nil {
		t.Fatalf("failed to unmarshal second REGIE_MESSAGE: %v", err)
	}

	if second.Msg.Text != "Deuxième consigne, différente" {
		t.Errorf("second SEND must replace the content: got %q", second.Msg.Text)
	}
	if second.Msg.SentAt <= first.Msg.SentAt {
		t.Errorf("SENT_AT must be rearmed on a genuinely different SEND: first=%d, second=%d", first.Msg.SentAt, second.Msg.SentAt)
	}
	if !second.Msg.Active {
		t.Error("replaced message must still be Active=true")
	}

	// No queue: no further REGIE_MESSAGE is pending behind the second one.
	moreActions := collectActionsT(t, anim, 300*time.Millisecond)
	if containsAction(moreActions, protocol.ActionRegieMessage) {
		t.Errorf("replacement must not leave a queued third message — got actions: %v", moreActions)
	}
}

// ---------------------------------------------------------------------------
// T4b — Idempotence : un SEND de texte identique au message actif ne réarme
// pas SENT_AT, n'efface pas CLEARED_BY et ne diffuse pas — y compris après
// un acquittement, où le message ne doit pas ressusciter.
//
// This is the "resurrection" bug the whole D1-amendment (règle 4) exists to
// close (plan risk table, "Élevée sans garde" / "Élevé"): the régie types,
// the debounce fires SEND, the animateur acquits, the régie then clicks
// elsewhere — blur fires SEND again with the SAME (untouched) text. Without
// this guard, that second SEND would rearm SENT_AT and clear CLEARED_BY,
// making an already-acquitted message reappear on every tablet.
// ---------------------------------------------------------------------------

func TestHandleRegieMessageSend_IdenticalTextWhileActive_NoRearmNoDiffusion(t *testing.T) {
	app := newAnimTestApp(t)
	baseURL := startAnimAllowlistTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, admin)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)

	sendAction(t, app, admin, protocol.ActionRegieMessageSend, protocol.RegieMessagePayload{Text: "Consigne stable"})
	_, raw1 := readActionMatching(t, anim, protocol.ActionRegieMessage)
	var first struct {
		Msg protocol.RegieMessagePayload `json:"MSG"`
	}
	if err := json.Unmarshal([]byte(raw1), &first); err != nil {
		t.Fatalf("failed to unmarshal first REGIE_MESSAGE: %v", err)
	}

	time.Sleep(5 * time.Millisecond)

	// Debounce/blur/Entrée firing again with the exact same (trimmed) text —
	// this is the automatic-send scenario, not a deliberate resend.
	sendAction(t, app, admin, protocol.ActionRegieMessageSend, protocol.RegieMessagePayload{Text: "Consigne stable"})

	animActions := collectActionsT(t, anim, 300*time.Millisecond)
	if containsAction(animActions, protocol.ActionRegieMessage) {
		t.Errorf("an identical-text SEND while the message is still active must NOT diffuse anything — got actions: %v", animActions)
	}
	if app.regieMessage == nil {
		t.Fatal("regieMessage must still reflect the active message")
	}
	if app.regieMessage.SentAt != first.Msg.SentAt {
		t.Errorf("SENT_AT must NOT be rearmed by an identical-text SEND: got %d, want unchanged %d", app.regieMessage.SentAt, first.Msg.SentAt)
	}
}

// TestHandleRegieMessageSend_IdenticalTextAfterAcquittal_DoesNotResurrect is
// the exact scenario from the risk table and the plan's GATE-2 write-up:
// SEND -> ANIM acquits -> the SAME text is sent again (blur on an untouched
// field) -> must remain a no-op, the message must NOT reappear.
func TestHandleRegieMessageSend_IdenticalTextAfterAcquittal_DoesNotResurrect(t *testing.T) {
	app := newAnimTestApp(t)
	baseURL := startAnimAllowlistTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, admin)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)

	// 1. The régie types, the debounce fires SEND. REGIE_MESSAGE reaches
	// BOTH admin and anim — drain both copies, or a later read on either
	// connection picks up this stale frame instead of the one it expects.
	sendAction(t, app, admin, protocol.ActionRegieMessageSend, protocol.RegieMessagePayload{Text: "Ne pas ressusciter"})
	readActionMatching(t, admin, protocol.ActionRegieMessage)
	readActionMatching(t, anim, protocol.ActionRegieMessage)

	// 2. The animateur acquits — this broadcast also reaches anim itself
	// (BroadcastToTypes does not exclude the sender), so drain both again.
	sendAction(t, app, anim, protocol.ActionRegieMessageClear, protocol.RegieMessagePayload{})
	_, clearedRaw := readActionMatching(t, admin, protocol.ActionRegieMessage)
	var cleared struct {
		Msg protocol.RegieMessagePayload `json:"MSG"`
	}
	if err := json.Unmarshal([]byte(clearedRaw), &cleared); err != nil {
		t.Fatalf("failed to unmarshal cleared REGIE_MESSAGE: %v", err)
	}
	if cleared.Msg.Active || cleared.Msg.ClearedBy != "ANIM" {
		t.Fatalf("setup failed: expected an acquitted message (Active=false, ClearedBy=ANIM), got %+v", cleared.Msg)
	}
	readActionMatching(t, anim, protocol.ActionRegieMessage) // drain anim's own copy of the CLEAR broadcast

	// 3. The régie's field still contains the same text; a blur (or any of
	// the other two automatic triggers) fires SEND again with it.
	sendAction(t, app, admin, protocol.ActionRegieMessageSend, protocol.RegieMessagePayload{Text: "Ne pas ressusciter"})

	// The critical assertion: NOTHING is diffused — the message must not
	// reappear on any connected tablet nor in régie.
	adminActions := collectActionsT(t, admin, 300*time.Millisecond)
	if containsAction(adminActions, protocol.ActionRegieMessage) {
		t.Errorf("#167 AC1d: an identical-text SEND after acquittal must NOT resurrect the message — admin got actions: %v", adminActions)
	}
	animActions := collectActionsT(t, anim, 300*time.Millisecond)
	if containsAction(animActions, protocol.ActionRegieMessage) {
		t.Errorf("#167 AC1d: an identical-text SEND after acquittal must NOT resurrect the message — anim got actions: %v", animActions)
	}
}

// TestHandleRegieMessageSend_DifferentTextAfterAcquittal_DoesArm is the
// control for the two tests above: the idempotence guard must only block a
// truly IDENTICAL resend, never a genuinely new message sent after an
// acquittal.
func TestHandleRegieMessageSend_DifferentTextAfterAcquittal_DoesArm(t *testing.T) {
	app := newAnimTestApp(t)
	baseURL := startAnimAllowlistTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, admin)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)

	sendAction(t, app, admin, protocol.ActionRegieMessageSend, protocol.RegieMessagePayload{Text: "Première consigne"})
	readActionMatching(t, admin, protocol.ActionRegieMessage)
	readActionMatching(t, anim, protocol.ActionRegieMessage)
	sendAction(t, app, anim, protocol.ActionRegieMessageClear, protocol.RegieMessagePayload{})
	readActionMatching(t, admin, protocol.ActionRegieMessage)
	readActionMatching(t, anim, protocol.ActionRegieMessage)

	sendAction(t, app, admin, protocol.ActionRegieMessageSend, protocol.RegieMessagePayload{Text: "Nouvelle consigne, distincte"})

	_, raw := readActionMatching(t, anim, protocol.ActionRegieMessage)
	var envelope struct {
		Msg protocol.RegieMessagePayload `json:"MSG"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		t.Fatalf("failed to unmarshal REGIE_MESSAGE: %v (raw: %s)", err, raw)
	}
	if !envelope.Msg.Active || envelope.Msg.Text != "Nouvelle consigne, distincte" {
		t.Errorf("a genuinely different SEND after acquittal must arm a new message — got %+v", envelope.Msg)
	}
}
