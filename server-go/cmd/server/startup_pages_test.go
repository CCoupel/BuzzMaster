package main

import "testing"

// ---------------------------------------------------------------------------
// Tests: #164 — startupPages(debug bool) []startupPage
//
// Fonction pure extraite de displayAndOpenURLs (main.go) pour permettre ce
// test sans lancer de processus navigateur. Plan de référence :
// _work/reports/plan-20260814-101626.md, tâche B1/T1.
//
// Critères couverts :
//   - /admin présent dans la liste
//   - 4 pages hors debug, 5 en debug (ajout de /logs uniquement)
//   - /logs présent uniquement en debug
//   - aucun doublon de chemin
//   - le libellé de /anim n'est plus "admin" (régression de l'alias supprimé
//     par #155 : le log de démarrage annonçait faussement "admin")
// ---------------------------------------------------------------------------

func TestStartupPages_AdminPresent(t *testing.T) {
	for _, debug := range []bool{false, true} {
		pages := startupPages(debug)
		found := false
		for _, p := range pages {
			if p.path == "/admin" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("startupPages(%v): /admin absent de la liste", debug)
		}
	}
}

func TestStartupPages_CountWithoutDebug(t *testing.T) {
	pages := startupPages(false)
	if len(pages) != 4 {
		t.Fatalf("startupPages(false): attendu 4 pages, obtenu %d (%+v)", len(pages), pages)
	}
}

func TestStartupPages_CountWithDebug(t *testing.T) {
	pages := startupPages(true)
	if len(pages) != 5 {
		t.Fatalf("startupPages(true): attendu 5 pages, obtenu %d (%+v)", len(pages), pages)
	}
}

func TestStartupPages_LogsOnlyInDebug(t *testing.T) {
	withoutDebug := startupPages(false)
	for _, p := range withoutDebug {
		if p.path == "/logs" {
			t.Errorf("startupPages(false): /logs ne doit pas être présent hors debug")
		}
	}

	withDebug := startupPages(true)
	found := false
	for _, p := range withDebug {
		if p.path == "/logs" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("startupPages(true): /logs absent en mode debug")
	}
}

func TestStartupPages_NoDuplicatePaths(t *testing.T) {
	for _, debug := range []bool{false, true} {
		pages := startupPages(debug)
		seen := make(map[string]bool, len(pages))
		for _, p := range pages {
			if seen[p.path] {
				t.Errorf("startupPages(%v): chemin dupliqué %q", debug, p.path)
			}
			seen[p.path] = true
		}
	}
}

// #164 — corrige le libellé de /anim, aujourd'hui "admin" (reliquat de
// l'alias /anim -> /admin supprimé par #155), qui produisait un log de
// démarrage trompeur ("Opening browser: http://…/anim (admin)").
func TestStartupPages_AnimLabelIsNotAdmin(t *testing.T) {
	for _, debug := range []bool{false, true} {
		pages := startupPages(debug)
		for _, p := range pages {
			if p.path == "/anim" && p.name == "admin" {
				t.Errorf("startupPages(%v): le libellé de /anim est encore %q (reliquat de l'alias supprimé par #155)", debug, p.name)
			}
		}
	}
}

func TestStartupPages_AllExpectedPathsWithoutDebug(t *testing.T) {
	pages := startupPages(false)
	expected := map[string]bool{"/admin": false, "/anim": false, "/tv": false, "/": false}
	for _, p := range pages {
		if _, ok := expected[p.path]; ok {
			expected[p.path] = true
		}
	}
	for path, seen := range expected {
		if !seen {
			t.Errorf("startupPages(false): chemin attendu %q absent", path)
		}
	}
}
