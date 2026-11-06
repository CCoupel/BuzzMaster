package server

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Revue LOT-3 (mineur 1) : remplacer / restaurer le décompte doit d'abord libérer le lecteur
// (Windows : os.Rename échoue sur un fichier encore ouvert). Le fournisseur reçoit StopCountdown
// AVANT l'écriture, sur les deux routes.
type stopCountdownProvider struct {
	tw287Provider
	stops int
}

func (p *stopCountdownProvider) StopCountdown() { p.stops++ }

func TestCountdownReplaceAndRestore_StopPlayerBeforeWriting(t *testing.T) {
	s, dataDir := tw287Seeded(t)
	p := &stopCountdownProvider{}
	s.Sound = p

	w := tw230Post(t, s, "/api/sounds/decompte/restore", nil, "")
	if w.Code != http.StatusOK || p.stops != 1 {
		t.Fatalf("restore : code=%d stops=%d, attendu 200 et 1 arrêt", w.Code, p.stops)
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", "d.wav")
	_, _ = fw.Write(tw287DecompteWAV(t, dataDir))
	_ = mw.Close()
	req := httptest.NewRequest("POST", "/api/sounds/decompte", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || p.stops != 2 {
		t.Fatalf("replace : code=%d stops=%d (%s), attendu 200 et 2 arrêts cumulés", rec.Code, p.stops, rec.Body.String())
	}
}
