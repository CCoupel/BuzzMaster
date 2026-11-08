// #285 (v11.4.0, LOT-4, révision 2 — 0ee50546) — CATEGORIES_CHANGED sur le fil, par la vraie frontière HTTP + WebSocket :
// une écriture de catégorie (POST /api/categories, PUT, DELETE) atteint admin, tv, vplayer et anim — jamais buzzer —
// avec une charge identique pour les quatre, égale à GET /api/categories, CHANGE {TYPE, KEY}, et AVANT QUESTIONS (admin).
// Source : contracts/websocket-actions.md §CATEGORIES_CHANGED. Aucun symbole absent du dépôt : serveur HTTP réel sur un
// port libre + clients gorilla. Préfixe tw285.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
)

type tw285Wire struct {
	base  string
	app   *App
	conns map[string]*websocket.Conn
	mu    sync.Mutex
	buf   map[string][][]byte
}

func tw285WireStart(t *testing.T) *tw285Wire {
	t.Helper()
	app := newAnimTestApp(t)
	saved := *config.Get()
	t.Cleanup(func() { config.SetInstance(&saved) })
	dataDir := t.TempDir()
	cfg := saved
	cfg.Storage.DataDir = dataDir
	config.SetInstance(&cfg)
	for _, d := range []string{"files/categories", "files/questions"} {
		if err := os.MkdirAll(filepath.Join(dataDir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	app.logsHub = server.NewLogsWebSocketHub(10)
	app.httpServer = server.NewHTTPServer(port, app.engine, app.wsHub, app.buzzerHub, app.logsHub)
	app.ackManager = server.NewAckManager(&app.config.Server)
	app.setupCallbacks()
	go app.wsHub.Run()
	go app.buzzerHub.Run()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := app.httpServer.Start(ctx); err != nil {
		t.Fatalf("démarrage du serveur HTTP : %v", err)
	}
	w := &tw285Wire{base: fmt.Sprintf("http://127.0.0.1:%d", port), app: app, conns: map[string]*websocket.Conn{}, buf: map[string][][]byte{}}
	eventually264(t, "serveur HTTP prêt", func() bool {
		r, err := http.Get(w.base + "/api/categories")
		if err != nil {
			return false
		}
		_ = r.Body.Close()
		return r.StatusCode == http.StatusOK
	})
	// setupCallbacks consomme app.wsHub.Incoming : on attend l'inscription par le compteur du hub (pas learnClientID).
	for i, c := range []struct{ name, path string }{
		{"admin", "/ws/admin"}, {"tv", "/ws/tv"}, {"vplayer", "/ws/player"}, {"anim", "/ws/anim"},
	} {
		w.listen(c.name, dialWS(t, w.base, c.path))
		want := i + 1
		eventually264(t, "inscription du client "+c.name, func() bool { return app.wsHub.ClientCount() >= want })
	}
	w.listen("buzzer", dialWS(t, w.base, "/ws/buzzer?mac=AA:BB:CC:DD:EE:01"))
	eventually264(t, "buzzer enregistré", func() bool { return app.buzzerHub.ConnectedCount() == 1 })
	return w
}

// settle (#291, Phase A) remplace frames(window) : barrière positive — un témoin est diffusé par le wsHub (admin, tv,
// vplayer, anim) ET par le buzzerHub ; une connexion livre ses trames dans l'ordre, donc le témoin reçu garantit que tout ce
// qui a été émis avant lui est arrivé. Puis marge de silence de 100 ms (envois différés par un saut de goroutine) et
// témoin effacé. Rend (et vide) les trames par client, comme frames.
func (w *tw285Wire) settle(t *testing.T) map[string][][]byte {
	t.Helper()
	id := fmt.Sprintf("BARRIER291-%d", barrierSeq291.Add(1))
	msg, err := protocol.NewMessage("BARRIER_291", map[string]string{"ID": id})
	if err != nil {
		t.Fatal(err)
	}
	w.app.wsHub.BroadcastToTypes(msg, server.ClientTypeAdmin, server.ClientTypeTV, server.ClientTypeVPlayer, server.ClientTypeAnim)
	w.app.buzzerHub.Broadcast(msg)
	seen := func(name string) bool {
		w.mu.Lock()
		defer w.mu.Unlock()
		for _, f := range w.buf[name] {
			if strings.Contains(string(f), id) {
				return true
			}
		}
		return false
	}
	deadline := time.Now().Add(2 * time.Second)
	for name := range w.conns {
		for !seen(name) {
			if time.Now().After(deadline) {
				t.Fatalf("settle : témoin %s jamais reçu par %s", id, name)
			}
			time.Sleep(2 * time.Millisecond)
		}
	}
	time.Sleep(settleQuiet291)
	w.mu.Lock()
	defer w.mu.Unlock()
	out := map[string][][]byte{}
	for name, frames := range w.buf {
		for _, f := range frames {
			if !strings.Contains(string(f), id) {
				out[name] = append(out[name], f)
			}
		}
	}
	w.buf = map[string][][]byte{}
	return out
}

// frames laisse s'écouler window puis rend (et vide) les trames reçues par client. Un lecteur par connexion tourne en
// continu (une lecture expirée est définitive avec gorilla : on ne pose aucune échéance de lecture).
func (w *tw285Wire) frames(window time.Duration) map[string][][]byte {
	time.Sleep(window)
	w.mu.Lock()
	defer w.mu.Unlock()
	out := w.buf
	w.buf = map[string][][]byte{}
	return out
}

func (w *tw285Wire) listen(name string, conn *websocket.Conn) {
	w.conns[name] = conn
	go func() {
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			w.mu.Lock()
			w.buf[name] = append(w.buf[name], data)
			w.mu.Unlock()
		}
	}()
}

func tw285Action(frame []byte) string {
	var e struct {
		Action string `json:"ACTION"`
	}
	_ = json.Unmarshal(frame, &e)
	return e.Action
}

func tw285Of(frames [][]byte, action string) [][]byte {
	var out [][]byte
	for _, f := range frames {
		if tw285Action(f) == action {
			out = append(out, f)
		}
	}
	return out
}

func (w *tw285Wire) postCategory(t *testing.T, name string) {
	t.Helper()
	buf := &bytes.Buffer{}
	mw := multipart.NewWriter(buf)
	_ = mw.WriteField("name", name)
	_ = mw.WriteField("color", "#0ea5e9")
	part, _ := mw.CreateFormFile("file", "i.png")
	_, _ = part.Write(append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 64)...))
	_ = mw.Close()
	resp, err := http.Post(w.base+"/api/categories", mw.FormDataContentType(), buf)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/categories : %d %s", resp.StatusCode, b)
	}
}

func (w *tw285Wire) request(t *testing.T, method, path, body string) {
	t.Helper()
	req, _ := http.NewRequest(method, w.base+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s %s : %d %s", method, path, resp.StatusCode, b)
	}
}

func (w *tw285Wire) getCategories(t *testing.T) interface{} {
	t.Helper()
	resp, err := http.Get(w.base + "/api/categories")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var v interface{}
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

type tw285ChangedMsg struct {
	Msg struct {
		Categories json.RawMessage `json:"CATEGORIES"`
		Change     struct {
			Type string `json:"TYPE"`
			Key  string `json:"KEY"`
		} `json:"CHANGE"`
		TS int64 `json:"TS"`
	} `json:"MSG"`
}

func TestCategoriesChanged285_Wire_RecipientsPayloadChangeAndOrder(t *testing.T) {
	w := tw285WireStart(t)

	w.postCategory(t, "Cinéma")
	got := w.settle(t)

	// destinataires : exactement admin, tv, vplayer, anim ; jamais le buzzer
	for _, name := range []string{"admin", "tv", "vplayer", "anim"} {
		if n := len(tw285Of(got[name], "CATEGORIES_CHANGED")); n != 1 {
			t.Errorf("%s : %d CATEGORIES_CHANGED reçu(s), attendu 1", name, n)
		}
	}
	if n := len(tw285Of(got["buzzer"], "CATEGORIES_CHANGED")); n != 0 {
		t.Errorf("buzzer : %d CATEGORIES_CHANGED reçu(s), attendu 0 (jamais de diffusion aux buzzers)", n)
	}

	// charge : identique pour les quatre types, = GET /api/categories, CHANGE {created, CINÉMA}
	var ref []byte
	var refMsg tw285ChangedMsg
	for _, name := range []string{"admin", "tv", "vplayer", "anim"} {
		fr := tw285Of(got[name], "CATEGORIES_CHANGED")
		if len(fr) != 1 {
			continue
		}
		var m tw285ChangedMsg
		if err := json.Unmarshal(fr[0], &m); err != nil {
			t.Fatalf("%s : %v", name, err)
		}
		if m.Msg.Change.Type != "created" || m.Msg.Change.Key != "CINÉMA" || m.Msg.TS <= 0 {
			t.Errorf("%s : CHANGE/TS = %+v, attendu {created, CINÉMA} et TS > 0", name, m.Msg)
		}
		var env struct {
			Msg json.RawMessage `json:"MSG"`
		}
		_ = json.Unmarshal(fr[0], &env)
		if ref == nil {
			ref, refMsg = env.Msg, m
		} else if !bytes.Equal(ref, env.Msg) {
			t.Errorf("%s : la charge doit être identique pour les quatre types\n ref : %s\n %s : %s", name, ref, name, env.Msg)
		}
	}
	if ref != nil {
		var fromWire, fromGet interface{}
		_ = json.Unmarshal(refMsg.Msg.Categories, &fromWire)
		fromGet = w.getCategories(t)
		a, _ := json.Marshal(fromWire)
		b, _ := json.Marshal(fromGet)
		if string(a) != string(b) {
			t.Errorf("CATEGORIES doit être octet pour octet la réponse de GET /api/categories\n wire : %s\n GET  : %s", a, b)
		}
		if !strings.Contains(string(a), `"CINÉMA"`) {
			t.Error("la catégorie créée doit figurer dans CATEGORIES")
		}
	}

	// ordre côté admin : CATEGORIES_CHANGED avant QUESTIONS
	var order []string
	for _, f := range got["admin"] {
		if a := tw285Action(f); a == "CATEGORIES_CHANGED" || a == "QUESTIONS" {
			order = append(order, a)
		}
	}
	if len(order) < 2 || order[0] != "CATEGORIES_CHANGED" || order[1] != "QUESTIONS" {
		t.Errorf("ordre admin = %v, attendu CATEGORIES_CHANGED puis QUESTIONS", order)
	}
	// QUESTIONS reste réservé à l'admin
	for _, name := range []string{"tv", "vplayer", "anim", "buzzer"} {
		if n := len(tw285Of(got[name], "QUESTIONS")); n != 0 {
			t.Errorf("%s : QUESTIONS ne doit pas lui être diffusé (admin seul), reçu %d", name, n)
		}
	}
}

func TestCategoriesChanged285_Wire_ChangeTypePerWrite_RenamedBeatsColor_DeletedAbsentFromList(t *testing.T) {
	w := tw285WireStart(t)
	w.postCategory(t, "Cinema")
	w.settle(t) // purge

	steps := []struct {
		method, path, body, wantType string
	}{
		{"PUT", "/api/categories/CINEMA", `{"color":"#ff0000"}`, "colored"},
		{"PUT", "/api/categories/CINEMA", `{"name":"Ciné","color":"#00ff00"}`, "renamed"},
		{"DELETE", "/api/categories/CINEMA", ``, "deleted"},
	}
	for _, s := range steps {
		w.request(t, s.method, s.path, s.body)
		got := w.settle(t)
		fr := tw285Of(got["tv"], "CATEGORIES_CHANGED")
		if len(fr) != 1 {
			t.Fatalf("%s %s : %d CATEGORIES_CHANGED sur la TV, attendu 1", s.method, s.path, len(fr))
		}
		var m tw285ChangedMsg
		_ = json.Unmarshal(fr[0], &m)
		if m.Msg.Change.Type != s.wantType || m.Msg.Change.Key != "CINEMA" {
			t.Errorf("%s %s : CHANGE = %+v, attendu {%s, CINEMA}", s.method, s.path, m.Msg.Change, s.wantType)
		}
		if s.wantType == "deleted" && strings.Contains(string(m.Msg.Categories), `"CINEMA"`) {
			t.Error("une catégorie supprimée ne figure plus dans CATEGORIES")
		}
		if n := len(tw285Of(got["buzzer"], "CATEGORIES_CHANGED")); n != 0 {
			t.Errorf("buzzer : %d CATEGORIES_CHANGED, attendu 0", n)
		}
	}
}
