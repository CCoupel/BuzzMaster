package game

// #319 (v13.0.0, LOT-2.2) — cycle de vie des empreintes du jeton de session (contracts/vplayer-session.md §3, §7, §9) :
// émission, empreinte SEULE dans vplayer_sessions.json (jamais le jeton, jamais dans bumpers.json ni dans l'état),
// persistance au redémarrage, suppression d'un bumper, purge NEW_GAME, élagage à la restauration.
// Moteur seul (pas de réseau) : la matrice filaire est dans cmd/server/vplayer_session_319_test.go.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

type sessEngine319 struct {
	e                      *Engine
	dir, sessions, bumpers string
}

func newSessEngine319(t *testing.T) *sessEngine319 {
	t.Helper()
	dir := t.TempDir()
	s := &sessEngine319{e: NewEngine(), dir: dir, sessions: filepath.Join(dir, "vplayer_sessions.json"), bumpers: filepath.Join(dir, "bumpers.json")}
	s.e.SetBumpersPath(s.bumpers)
	s.e.SetVPlayerSessionsPath(s.sessions)
	s.e.SetPhase(PhaseEnroll)
	// Les sauvegardes du moteur sont asynchrones (safeGo) : on laisse finir avant que TempDir ne soit nettoyé.
	t.Cleanup(func() { time.Sleep(300 * time.Millisecond) })
	return s
}

func (s *sessEngine319) enroll(t *testing.T, name string) (id, token string) {
	t.Helper()
	id, _, _, token, err := s.e.ReconnectOrCreateVirtualPlayerWithToken("", name, "")
	if err != nil {
		t.Fatalf("enrôlement %s : %v", name, err)
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`).MatchString(token) {
		t.Fatalf("jeton %q : 43 caractères base64url attendus (§3)", token)
	}
	return id, token
}

// persist écrit les deux fichiers de façon synchrone et lit les empreintes sur disque.
func (s *sessEngine319) persist(t *testing.T) map[string]string {
	t.Helper()
	if err := s.e.SaveBumpers(); err != nil {
		t.Fatal(err)
	}
	if err := s.e.SaveVPlayerSessions(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(s.sessions)
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Tokens map[string]string `json:"TOKENS"`
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("vplayer_sessions.json illisible : %v (%s)", err, raw)
	}
	return f.Tokens
}

func fingerprint319(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func isInvalidSession319(err error) bool {
	var ee *EnrollmentError
	return errors.As(err, &ee) && ee.Reason == "INVALID_SESSION"
}

// §3 : seule l'empreinte SHA-256 est stockée, sous {"TOKENS": {"<ID>": "<sha256 hex>"}}.
func TestVPlayerSessions319_File_OnlyTheFingerprint(t *testing.T) {
	s := newSessEngine319(t)
	id, token := s.enroll(t, "Alice")
	tokens := s.persist(t)

	if got := tokens[id]; got != fingerprint319(token) {
		t.Errorf("empreinte = %q, attendu SHA-256 hex du jeton %q", got, fingerprint319(token))
	}
	raw, _ := os.ReadFile(s.sessions)
	if strings.Contains(string(raw), token) {
		t.Errorf("le jeton en clair ne doit jamais être écrit dans vplayer_sessions.json")
	}
	if !strings.Contains(string(raw), `"TOKENS"`) {
		t.Errorf("forme attendue {\"TOKENS\": {…}} : %s", raw)
	}
	bumpers, _ := os.ReadFile(s.bumpers)
	if strings.Contains(string(bumpers), token) || strings.Contains(string(bumpers), fingerprint319(token)) || strings.Contains(string(bumpers), "TOKEN") {
		t.Errorf("rien du jeton ni de son empreinte dans bumpers.json (la structure Bumper ne le porte jamais, §3)")
	}
	if g := string(s.e.GetGameJSON()); strings.Contains(g, token) || strings.Contains(g, fingerprint319(token)) {
		t.Errorf("rien du jeton ni de son empreinte dans le GameState")
	}
}

func TestVPlayerSessions319_File_Mode0600_AtomicNoLeftover(t *testing.T) {
	s := newSessEngine319(t)
	s.enroll(t, "Alice")
	s.persist(t)
	st, err := os.Stat(s.sessions)
	if err != nil {
		t.Fatal(err)
	}
	if runtimeIsUnix319() && st.Mode().Perm() != 0o600 {
		t.Errorf("droits du fichier = %v, attendu 0600 (empreintes)", st.Mode().Perm())
	}
	entries, _ := os.ReadDir(s.dir)
	for _, en := range entries {
		if strings.Contains(en.Name(), ".tmp") {
			t.Errorf("écriture atomique : fichier temporaire %q laissé sur le disque", en.Name())
		}
	}
}

func runtimeIsUnix319() bool { return os.PathSeparator == '/' }

// §7 « Redémarrage du serveur » : les empreintes sont rechargées, la place reste protégée, le jeton reste valable.
func TestVPlayerSessions319_Persistence_SurvivesRestart(t *testing.T) {
	s := newSessEngine319(t)
	id, token := s.enroll(t, "Alice")
	s.persist(t)

	e2 := NewEngine()
	e2.SetBumpersPath(s.bumpers)
	if err := e2.LoadBumpers(); err != nil {
		t.Fatal(err)
	}
	e2.SetVPlayerSessionsPath(s.sessions)

	gotID, _, reconnected, tok, err := e2.ReconnectOrCreateVirtualPlayerWithToken(id, "Alice", token)
	if err != nil || !reconnected || gotID != id {
		t.Fatalf("1a après redémarrage : (%q, reconnected=%v, err=%v)", gotID, reconnected, err)
	}
	if tok != token {
		t.Errorf("le jeton reste le même après redémarrage")
	}
	for _, bad := range []string{"", "faux", token + "x"} {
		if _, _, _, _, err := e2.ReconnectOrCreateVirtualPlayerWithToken(id, "Alice", bad); !isInvalidSession319(err) {
			t.Errorf("après redémarrage, jeton %q : INVALID_SESSION attendu, got %v", bad, err)
		}
	}
}

// Fichier absent ou illisible : magasin vide (les places sont alors « sans empreinte », cas 1c) — jamais de plantage.
func TestVPlayerSessions319_File_MissingOrCorrupt_IsAnEmptyStore(t *testing.T) {
	s := newSessEngine319(t)
	id, _ := s.enroll(t, "Alice")
	s.persist(t)
	time.Sleep(300 * time.Millisecond) // les sauvegardes asynchrones de l'enrôlement doivent être finies avant de corrompre le fichier
	if err := os.WriteFile(s.sessions, []byte("{ceci n'est pas du json"), 0o600); err != nil {
		t.Fatal(err)
	}
	e2 := NewEngine()
	e2.SetBumpersPath(s.bumpers)
	_ = e2.LoadBumpers()
	e2.SetVPlayerSessionsPath(s.sessions) // ne doit pas paniquer
	_, _, _, tok, err := e2.ReconnectOrCreateVirtualPlayerWithToken(id, "Alice", "")
	if err != nil || tok == "" {
		t.Errorf("magasin vide : place sans empreinte = cas 1c (nouveau jeton), got (%q, %v)", tok, err)
	}
}

// §7 « Suppression d'un bumper » : l'empreinte est supprimée, et le jeton ne rouvre rien.
func TestVPlayerSessions319_DeleteBumper_RemovesFingerprint(t *testing.T) {
	s := newSessEngine319(t)
	idA, _ := s.enroll(t, "Alice")
	idB, _ := s.enroll(t, "Bob")
	if _, ok := s.persist(t)[idA]; !ok {
		t.Fatal("précondition : empreinte d'Alice présente")
	}
	s.e.DeleteBumper(idA)
	tokens := s.persist(t)
	if _, ok := tokens[idA]; ok {
		t.Errorf("l'empreinte d'un bumper supprimé doit disparaître (§7)")
	}
	if _, ok := tokens[idB]; !ok {
		t.Errorf("l'empreinte des autres places est conservée")
	}
}

// §7 « NEW_GAME qui purge les joueurs » : toutes les empreintes des bumpers supprimés sont retirées.
func TestVPlayerSessions319_NewGamePurge_RemovesAllFingerprints(t *testing.T) {
	s := newSessEngine319(t)
	ids := []string{}
	for _, n := range []string{"Alice", "Bob", "Carol"} {
		id, _ := s.enroll(t, n)
		ids = append(ids, id)
	}
	if len(s.persist(t)) != 3 {
		t.Fatal("précondition : 3 empreintes")
	}
	s.e.InitGameWith(DefaultNewGameOptions()) // purge scores + joueurs + questions
	tokens := s.persist(t)
	for _, id := range ids {
		if _, ok := tokens[id]; ok {
			t.Errorf("NEW_GAME avec purge des joueurs : l'empreinte de %s doit être retirée", id)
		}
	}
}

// NEW_GAME sans purge des joueurs : les places et leurs empreintes sont conservées.
func TestVPlayerSessions319_NewGameWithoutPlayerPurge_KeepsFingerprints(t *testing.T) {
	s := newSessEngine319(t)
	id, token := s.enroll(t, "Alice")
	s.e.InitGameWith(NewGameOptions{PurgeScores: true, PurgePlayers: false})
	if _, ok := s.persist(t)[id]; !ok {
		t.Errorf("sans purge des joueurs, l'empreinte reste")
	}
	if _, _, _, _, err := s.e.ReconnectOrCreateVirtualPlayerWithToken(id, "Alice", token); err != nil {
		t.Errorf("le jeton reste valable : %v", err)
	}
}

// §7 + restauration : l'élagage ne garde que les places du roster de référence.
func TestVPlayerSessions319_LoadSessions_PrunesSeatsThatNoLongerExist(t *testing.T) {
	s := newSessEngine319(t)
	id, _ := s.enroll(t, "Alice")
	s.persist(t)
	// Une archive restaurée apporte une empreinte pour une place que bumpers.json ne contient pas.
	raw, _ := os.ReadFile(s.sessions)
	var f map[string]map[string]string
	_ = json.Unmarshal(raw, &f)
	f["TOKENS"]["vjoueur_fantome"] = fingerprint319("fantome")
	out, _ := json.Marshal(f)
	_ = os.WriteFile(s.sessions, out, 0o600)

	s.e.LoadVPlayerSessions()
	tokens := s.persist(t)
	if _, ok := tokens["vjoueur_fantome"]; ok {
		t.Errorf("une empreinte sans bumper doit être élaguée à la restauration")
	}
	if _, ok := tokens[id]; !ok {
		t.Errorf("l'empreinte d'une place existante est conservée")
	}
}

// §4.2 lignes 2 à 4 / §7 : un nouvel enrôlement émet toujours un jeton distinct (crypto/rand, jamais rejoué).
func TestVPlayerSessions319_Tokens_AreUniqueAcrossEnrolments(t *testing.T) {
	s := newSessEngine319(t)
	seen := map[string]bool{}
	for i := 0; i < 15; i++ { // plafond d'inscription : 20 VJoueurs
		_, tok := s.enroll(t, "Joueur"+strings.Repeat("x", i%5)+string(rune('A'+i%26))+string(rune('a'+i/26)))
		if seen[tok] {
			t.Fatalf("jeton rejoué : %q", tok)
		}
		seen[tok] = true
	}
}

// Le chemin « sans jeton » reste toléré pour les outils et tests existants (place sans empreinte = 1c), mais une place
// AVEC empreinte n'y est jamais vérifiée : c'est pourquoi aucun chemin de production ne doit l'appeler
// (cmd/server/vplayer_session_callers_319_test.go).
func TestVPlayerSessions319_LegacyEntryPoint_DoesNotVerify_WithTokenDoes(t *testing.T) {
	s := newSessEngine319(t)
	id, _ := s.enroll(t, "Alice")
	if _, _, _, err := s.e.ReconnectOrCreateVirtualPlayer(id, "Alice"); err != nil {
		t.Skipf("la variante historique refuse désormais : %v (durcissement : ce test n'a plus d'objet)", err)
	}
	if _, _, _, _, err := s.e.ReconnectOrCreateVirtualPlayerWithToken(id, "Alice", ""); !isInvalidSession319(err) {
		t.Errorf("la variante avec jeton DOIT refuser une place protégée sans jeton, got %v", err)
	}
}
