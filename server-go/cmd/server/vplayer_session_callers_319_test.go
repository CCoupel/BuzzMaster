package main

// #319 (v13.0.0, LOT-2.2) — garde d'architecture : AUCUN chemin de production ne lie une connexion à un VJoueur sans
// avoir vérifié le jeton (contracts/vplayer-session.md §2 : « toute reconnexion par ID doit présenter ce jeton »).
// ReconnectOrCreateVirtualPlayer (sans jeton) reste exporté pour les outils et les tests, mais elle ne vérifie rien :
// l'appeler depuis le code de production rouvrirait la faille M1. Les liaisons de connexion (BindClientPlayerID,
// SetClientPlayerID) ne doivent venir que de handlePlayerConnect, après la décision de la matrice §4.2.
// Test de source (même technique que questionTypeMeta.test.js) : un nouveau chemin de liaison doit être décidé ici.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"buzzcontrol/internal/protocol"

	"github.com/gorilla/websocket"
)

func productionGoFiles319(t *testing.T) map[string]string {
	t.Helper()
	root := filepath.Join("..", "..") // server-go/
	files := map[string]string{}
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if n := info.Name(); n == "node_modules" || n == "web" || n == ".git" || n == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") {
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			files[filepath.ToSlash(p)] = string(b)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 20 {
		t.Fatalf("lecture des sources de production : seulement %d fichiers — chemin de recherche cassé ?", len(files))
	}
	return files
}

// calls retourne les lignes de code (hors commentaires) qui contiennent le motif.
func codeLines319(src string, re *regexp.Regexp) []string {
	var out []string
	for _, line := range strings.Split(src, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "//") {
			continue
		}
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		if re.MatchString(line) {
			out = append(out, trim)
		}
	}
	return out
}

func TestVPlayerSession319_Critical_NoProductionPathCallsTheTokenlessReconnect(t *testing.T) {
	re := regexp.MustCompile(`\.ReconnectOrCreateVirtualPlayer\(`)
	for path, src := range productionGoFiles319(t) {
		if hits := codeLines319(src, re); len(hits) > 0 {
			t.Errorf("%s appelle ReconnectOrCreateVirtualPlayer (sans vérification du jeton) : %v — utiliser ReconnectOrCreateVirtualPlayerWithToken (§2)", path, hits)
		}
	}
}

// La seule route de PLAYER_CONNECT passe par la variante avec jeton.
func TestVPlayerSession319_Critical_HandlePlayerConnectUsesTheTokenVariant(t *testing.T) {
	src := productionGoFiles319(t)["../../cmd/server/main.go"]
	if src == "" {
		t.Fatal("cmd/server/main.go introuvable")
	}
	start := strings.Index(src, "func (a *App) handlePlayerConnect(")
	if start < 0 {
		t.Fatal("handlePlayerConnect introuvable")
	}
	body := src[start:]
	if end := strings.Index(body[10:], "\nfunc "); end > 0 {
		body = body[:end+10]
	}
	if !strings.Contains(body, "ReconnectOrCreateVirtualPlayerWithToken(") {
		t.Errorf("handlePlayerConnect doit appeler ReconnectOrCreateVirtualPlayerWithToken")
	}
	// Le contrôle précède la liaison de la connexion : jamais de Bind avant la décision de la matrice (1b ne lie pas).
	iVerify := strings.Index(body, "ReconnectOrCreateVirtualPlayerWithToken(")
	iBind := strings.Index(body, "BindClientPlayerID(")
	if iBind >= 0 && iVerify > iBind {
		t.Errorf("BindClientPlayerID est appelé AVANT la vérification du jeton (§4.2 : le contrôle précède la liaison)")
	}
}

// Les liaisons connexion → VJoueur ne naissent que dans handlePlayerConnect.
func TestVPlayerSession319_Critical_ConnectionBindingOnlyFromHandlePlayerConnect(t *testing.T) {
	re := regexp.MustCompile(`\.(BindClientPlayerID|SetClientPlayerID)\(`)
	files := productionGoFiles319(t)
	for path, src := range files {
		if strings.HasSuffix(path, "internal/server/websocket.go") {
			continue // définitions des méthodes du hub
		}
		for _, hit := range codeLines319(src, re) {
			if path != "../../cmd/server/main.go" {
				t.Errorf("%s lie une connexion à un VJoueur (%s) : seule handlePlayerConnect le fait, après la vérification du jeton", path, hit)
			}
		}
	}
	src := files["../../cmd/server/main.go"]
	n := len(codeLines319(src, re))
	if n != 1 {
		t.Errorf("cmd/server/main.go : %d liaison(s) de connexion, une seule attendue (dans handlePlayerConnect, après le jeton)", n)
	}
	if i := strings.Index(src, "func (a *App) handlePlayerConnect("); i >= 0 {
		bindIdx := strings.Index(src, "BindClientPlayerID(")
		if bindIdx < i {
			t.Errorf("la liaison doit se trouver dans handlePlayerConnect")
		}
	}
}

// ---- aides partagées : relire le jeton reçu dans PLAYER_CONNECTED (tests qui se reconnectent par ID, contrat §6) ----

func tokenOfFrames319(t *testing.T, frames []string) string {
	t.Helper()
	tok := ""
	for _, f := range frames {
		if frameAction(f) != "PLAYER_CONNECTED" {
			continue
		}
		var p struct {
			Msg struct {
				Token string `json:"TOKEN"`
			} `json:"MSG"`
		}
		if json.Unmarshal([]byte(f), &p) == nil && p.Msg.Token != "" {
			tok = p.Msg.Token
		}
	}
	if tok == "" {
		t.Fatal("aucun PLAYER_CONNECTED avec TOKEN reçu : la reconnexion par ID serait refusée (INVALID_SESSION)")
	}
	return tok
}

// tokenOfClient319 : jeton du dernier PLAYER_CONNECTED reçu par un wireClient.
func tokenOfClient319(t *testing.T, c *wireClient) string {
	t.Helper()
	return tokenOfFrames319(t, c.snapshot())
}

// tokenFromTap319 : même chose pour une connexion espionnée par startTap253.
func tokenFromTap319(t *testing.T, conn *websocket.Conn) string {
	t.Helper()
	tp := tapOf253(conn)
	tp.mu.Lock()
	frames := append([]string(nil), tp.frames...)
	tp.mu.Unlock()
	return tokenOfFrames319(t, frames)
}

// ---- buzzers physiques : /ws/buzzer inchangé (§2 : « aucun changement de firmware ni de protocole ») ----

func TestVPlayerSession319_BuzzerChannelUntouchedBySessionTokens(t *testing.T) {
	files := productionGoFiles319(t)
	buzzer := files["../../internal/server/websocket_buzzer.go"]
	if buzzer == "" {
		t.Fatal("internal/server/websocket_buzzer.go introuvable")
	}
	re := regexp.MustCompile(`(?i)token|INVALID_SESSION|vplayer_sessions|fingerprint`)
	if hits := codeLines319(buzzer, re); len(hits) > 0 {
		t.Errorf("le canal /ws/buzzer ne doit rien savoir des jetons de session des tablettes : %v", hits)
	}
	// Le protocole buzzer (messages minimaux) ne gagne aucun champ de jeton.
	msgs := files["../../internal/protocol/messages.go"]
	for _, name := range []string{"BuzzerHello", "ButtonPayload", "LEDSetPayload", "BuzzerRegister"} {
		if i := strings.Index(msgs, "type "+name+" struct"); i >= 0 {
			body := msgs[i:]
			if end := strings.Index(body, "\n}\n"); end > 0 {
				body = body[:end]
			}
			if strings.Contains(strings.ToUpper(body), "TOKEN") {
				t.Errorf("%s ne doit pas porter de jeton (buzzers non touchés)", name)
			}
		}
	}
}

// connectGetToken319 envoie PLAYER_CONNECT {NAME, ID} (première liaison, place sans empreinte = cas 1c) depuis la
// connexion `cid`, relit son PLAYER_CONNECTED sur `conn` et en retourne le jeton. Les tests #246 qui se reconnectent
// ensuite par ID doivent le présenter (contrat §6, rupture documentée) : reconnectWithToken319.
func (e *env246) connectGetToken319(t *testing.T, conn *websocket.Conn, cid, name, id string) string {
	t.Helper()
	e.connect(t, cid, name, id)
	raw := readActionMatchingWithin(t, conn, protocol.ActionPlayerConnected, 2*time.Second)
	var p struct {
		Token string `json:"TOKEN"`
	}
	_ = json.Unmarshal(raw, &p)
	if p.Token == "" {
		t.Fatal("PLAYER_CONNECTED sans TOKEN : la reconnexion par ID serait refusée")
	}
	return p.Token
}

// reconnectWithToken319 : PLAYER_CONNECT {NAME, ID, TOKEN} depuis la connexion cid.
func (e *env246) reconnectWithToken319(t *testing.T, cid, name, id, token string) {
	t.Helper()
	e.send(t, cid, "vplayer", protocol.ActionPlayerConnect, map[string]interface{}{"NAME": name, "ID": id, "TOKEN": token})
}

// ---- revue M2 : les endroits de production qui CRÉENT une place de VJoueur (donc une place qui doit recevoir un jeton) ----
//
// Une place créée sans passer par l'émission d'un jeton resterait « sans empreinte » (cas 1c : premier arrivé) — c'est
// la migration voulue pour les places d'avant v13.0.0, jamais un chemin neuf. Chemins légitimes (contrat §4.2, §7) :
//   - enrôlement : reconnectOrCreateVirtualPlayer → createVirtualPlayerUnsafe, puis émission du jeton ;
//   - re-clé #122 / #134 : rekeyBumperUnsafe, nouveau jeton (reconnectOrCreateVirtualPlayer / ReleaseSeat).
// CreateVirtualPlayer (exporté, sans jeton) est réservé aux tests et aux outils.

var funcDecl319 = regexp.MustCompile(`^func (\([^)]*\) )?([A-Za-z0-9_]+)\(`)

// enclosingFuncs319 retourne, pour chaque ligne de code qui contient le motif, le nom de la fonction qui l'englobe.
func enclosingFuncs319(src string, re *regexp.Regexp) []string {
	var out []string
	cur := ""
	for _, line := range strings.Split(src, "\n") {
		if m := funcDecl319.FindStringSubmatch(line); m != nil {
			cur = m[2]
			continue
		}
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "//") {
			continue
		}
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		if re.MatchString(line) {
			out = append(out, cur)
		}
	}
	return out
}

func TestVPlayerSession319_Critical_NoProductionCallToExportedSeatCreators(t *testing.T) {
	for _, call := range []string{"CreateVirtualPlayer"} {
		re := regexp.MustCompile(`\.` + call + `\(`)
		for path, src := range productionGoFiles319(t) {
			if hits := codeLines319(src, re); len(hits) > 0 {
				t.Errorf("%s appelle %s : une place créée ainsi n'a pas de jeton (cas 1c). Chemin légitime = ReconnectOrCreateVirtualPlayerWithToken : %v", path, call, hits)
			}
		}
	}
}

func TestVPlayerSession319_Critical_SeatCreationOnlyThroughTheEnrolmentPath(t *testing.T) {
	engine := productionGoFiles319(t)["../../internal/game/engine.go"]
	if engine == "" {
		t.Fatal("internal/game/engine.go introuvable")
	}
	// createVirtualPlayerUnsafe n'est appelée que par CreateVirtualPlayer (outil/tests) et par l'enrôlement.
	got := enclosingFuncs319(engine, regexp.MustCompile(`\bcreateVirtualPlayerUnsafe\(`))
	want := map[string]bool{"CreateVirtualPlayer": true, "reconnectOrCreateVirtualPlayer": true}
	for _, f := range got {
		if !want[f] {
			t.Errorf("createVirtualPlayerUnsafe est appelée depuis %q : nouvelle voie de création de place, à décider (jeton à émettre, §4.2)", f)
		}
	}
	if len(got) != 2 {
		t.Errorf("createVirtualPlayerUnsafe : %d appel(s) attendus 2 (CreateVirtualPlayer, reconnectOrCreateVirtualPlayer), got %v", len(got), got)
	}
	// L'enrôlement émet le jeton de la place créée (issueUnlessLegacyUnsafe / sessionIssueUnsafe après la création).
	i := strings.Index(engine, "func (e *Engine) reconnectOrCreateVirtualPlayer(")
	if i < 0 {
		t.Fatal("reconnectOrCreateVirtualPlayer introuvable")
	}
	body := engine[i:]
	if end := strings.Index(body[10:], "\nfunc "); end > 0 {
		body = body[:end+10]
	}
	create := strings.LastIndex(body, "createVirtualPlayerUnsafe(")
	issue := strings.LastIndex(body, "issueUnlessLegacyUnsafe(")
	if create < 0 || issue < create {
		t.Errorf("reconnectOrCreateVirtualPlayer doit émettre le jeton de la place qu'elle vient de créer (§4.2 ligne 4) : émission après la création attendue")
	}
	// La re-clé supprime l'ancienne empreinte (§7).
	r := strings.Index(engine, "func (e *Engine) rekeyBumperUnsafe(")
	if r < 0 || !strings.Contains(engine[r:r+1500], "sessionRemoveUnsafe(oldID)") {
		t.Errorf("rekeyBumperUnsafe doit retirer l'ancienne empreinte (§7)")
	}
}

// Toute construction littérale d'une place de VJoueur en production est listée : une nouvelle doit être décidée ici.
func TestVPlayerSession319_SeatConstructionSitesAreListed(t *testing.T) {
	re := regexp.MustCompile(`IsVirtual:\s*true`)
	// Seule construction légitime : createVirtualPlayerUnsafe (HandleVirtualPlayerConnect, ancienne entrée sans appelant, a été supprimée).
	allowed := map[string]bool{"createVirtualPlayerUnsafe": true}
	for path, src := range productionGoFiles319(t) {
		for _, f := range enclosingFuncs319(src, re) {
			if !allowed[f] {
				t.Errorf("%s : %s construit une place de VJoueur (IsVirtual: true) hors des chemins connus — jeton à émettre ?", path, f)
			}
		}
	}
}
