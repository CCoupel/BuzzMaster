package server

import (
	"bytes"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// #270 (LOT-2, milestone v11.2.2) — smoke test « dist servi par Go ».
//
// Filet de sécurité de la migration d'outillage web (vite 5 → 8, vitest 1 → 5,
// Node 22, minification CSS lightningcss) : les tests vitest ne voient jamais
// le BUILD. Ce test prend le `web/dist` réellement produit par `npm run build`
// et le fait servir par le vrai routage HTTP du serveur (server.mux, celui de
// setupRoutes()), comme le fait le binaire (go:embed all:dist → SetEmbeddedFS),
// puis vérifie ce que le navigateur demandera :
//
//   - les routes SPA (/, /admin, /tv, /anim, /player…) répondent 200 text/html
//     avec l'index.html du build, jamais mis en cache ;
//   - index.html référence des assets hachés (script module + feuille de style) ;
//     chacun répond 200, avec le bon type MIME et le contenu exact du fichier ;
//   - TOUS les fichiers de dist/assets (dont les chunks chargés à la demande, ex.
//     esp32*.js) et de dist/fonts répondent 200 avec le bon MIME : aucun 404 ;
//   - toute ressource `url(/…)` du CSS minifié (polices) est servie ;
//   - un asset inexistant répond 404 (et non l'index.html en 200).
//
// Les deux modes de service du serveur sont couverts : FS embarqué (binaire
// portable) et répertoire disque (reactDir, mode dev).
//
// Non-régression pure : vert sur le dist actuel (vite 5) ET après vite 8.
//
// PRÉREQUIS : `web/dist` doit exister (`cd server-go/web && npm run build`) —
// il est gitignoré. Ce test ne l'importe pas via le paquet `web` (go:embed) pour
// ne pas imposer le build à toute compilation de `internal/server`. Dist absent :
//   - hors CI : le test est SAUTÉ (message explicite) ;
//   - en CI (variable CI définie, cf. scripts/ci.sh étape 1 : npm ci + build) :
//     le test ÉCHOUE — un build manquant ne doit jamais passer au vert.
// ---------------------------------------------------------------------------

// locateDist270 résout web/dist depuis le répertoire du paquet. À appeler AVANT
// setupTestHTTPServer, qui fait t.Chdir(TempDir).
func locateDist270(t *testing.T) string {
	t.Helper()
	dist, err := filepath.Abs(filepath.Join("..", "..", "web", "dist"))
	if err != nil {
		t.Fatalf("résolution de web/dist : %v", err)
	}
	if st, err := os.Stat(filepath.Join(dist, "index.html")); err != nil || st.IsDir() {
		msg := "web/dist/index.html absent — lancer `cd server-go/web && npm run build` avant ce test (#270)"
		if os.Getenv("CI") != "" {
			t.Fatalf("%s (CI : un build manquant est un échec)", msg)
		}
		t.Skip(msg)
	}
	return dist
}

type distMode270 struct {
	name  string
	setup func(h *HTTPServer, dist string)
}

var distModes270 = []distMode270{
	{"embedded-fs", func(h *HTTPServer, dist string) { h.SetEmbeddedFS(os.DirFS(dist)) }},
	{"react-dir", func(h *HTTPServer, dist string) { h.SetReactDir(dist) }},
}

func get270(h *HTTPServer, target string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", target, nil)
	w := httptest.NewRecorder()
	h.mux.ServeHTTP(w, req)
	return w
}

// isJSMime270 : Go (Windows/Linux) et le serveur servent application/javascript ; text/javascript est l'autre forme valide.
func isJSMime270(ct string) bool {
	ct = strings.ToLower(ct)
	return strings.HasPrefix(ct, "application/javascript") || strings.HasPrefix(ct, "text/javascript")
}

func expectMime270(t *testing.T, urlPath, ct string) {
	t.Helper()
	switch strings.ToLower(path.Ext(urlPath)) {
	case ".js", ".mjs":
		if !isJSMime270(ct) {
			t.Errorf("GET %s : Content-Type = %q, attendu application/javascript ou text/javascript", urlPath, ct)
		}
	case ".css":
		if !strings.HasPrefix(strings.ToLower(ct), "text/css") {
			t.Errorf("GET %s : Content-Type = %q, attendu text/css", urlPath, ct)
		}
	case ".woff2":
		if ct != "font/woff2" {
			t.Errorf("GET %s : Content-Type = %q, attendu font/woff2", urlPath, ct)
		}
	case ".woff":
		if ct != "font/woff" {
			t.Errorf("GET %s : Content-Type = %q, attendu font/woff", urlPath, ct)
		}
	}
}

var (
	scriptSrc270 = regexp.MustCompile(`<script\b[^>]*\bsrc\s*=\s*["']([^"']+)["']`)
	linkHref270  = regexp.MustCompile(`<link\b[^>]*\bhref\s*=\s*["']([^"']+)["']`)
	cssURL270    = regexp.MustCompile(`url\(\s*["']?(/[^)"'\s]+)["']?\s*\)`)
)

func sortedKeys270(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// indexRefs270 extrait de index.html les ressources locales (chemins absolus) référencées.
func indexRefs270(t *testing.T, html string) (scripts, styles []string) {
	t.Helper()
	for _, m := range scriptSrc270.FindAllStringSubmatch(html, -1) {
		if strings.HasPrefix(m[1], "/") {
			scripts = append(scripts, m[1])
		}
	}
	for _, m := range linkHref270.FindAllStringSubmatch(html, -1) {
		if strings.HasPrefix(m[1], "/") && strings.HasSuffix(strings.ToLower(m[1]), ".css") {
			styles = append(styles, m[1])
		}
	}
	return
}

func forEachMode270(t *testing.T, fn func(t *testing.T, h *HTTPServer, dist string)) {
	dist := locateDist270(t)
	for _, mode := range distModes270 {
		mode := mode
		t.Run(mode.name, func(t *testing.T) {
			h, _ := setupTestHTTPServer(t)
			mode.setup(h, dist)
			fn(t, h, dist)
		})
	}
}

// Routes SPA : 200 text/html, corps = index.html du build, jamais mis en cache.
func TestDistSmoke270_SPARoutesServeIndexHTML(t *testing.T) {
	forEachMode270(t, func(t *testing.T, h *HTTPServer, dist string) {
		want, err := os.ReadFile(filepath.Join(dist, "index.html"))
		if err != nil {
			t.Fatal(err)
		}
		for _, route := range []string{
			"/", "/admin", "/admin/", "/admin/teams", "/admin/quiz", "/admin/backstage",
			"/admin/logs", "/tv", "/anim", "/player",
		} {
			w := get270(h, route)
			if w.Code != http.StatusOK {
				t.Errorf("GET %s : %d, attendu 200", route, w.Code)
				continue
			}
			if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(strings.ToLower(ct), "text/html") {
				t.Errorf("GET %s : Content-Type = %q, attendu text/html", route, ct)
			}
			if !bytes.Equal(w.Body.Bytes(), want) {
				t.Errorf("GET %s : le corps n'est pas l'index.html du dist (%d octets servis, %d attendus)", route, w.Body.Len(), len(want))
			}
			if cc := w.Header().Get("Cache-Control"); !strings.Contains(cc, "no-cache") && !strings.Contains(cc, "no-store") {
				t.Errorf("GET %s : Cache-Control = %q — index.html ne doit jamais être mis en cache (sinon les assets hachés d'une ancienne version)", route, cc)
			}
		}
	})
}

// index.html : un script module + une feuille de style, tous servis avec le bon MIME et le contenu exact.
func TestDistSmoke270_IndexReferencedAssetsAreServed(t *testing.T) {
	forEachMode270(t, func(t *testing.T, h *HTTPServer, dist string) {
		html, err := os.ReadFile(filepath.Join(dist, "index.html"))
		if err != nil {
			t.Fatal(err)
		}
		scripts, styles := indexRefs270(t, string(html))
		if len(scripts) == 0 {
			t.Fatalf("index.html ne référence aucun <script src=\"/…\"> : bundle d'entrée absent")
		}
		if len(styles) == 0 {
			t.Fatalf("index.html ne référence aucune feuille de style <link href=\"/….css\"> : CSS absent")
		}
		if !strings.Contains(string(html), `type="module"`) {
			t.Errorf("index.html : aucun <script type=\"module\"> (bundle ESM attendu)")
		}
		hashed := regexp.MustCompile(`^/assets/.+-[A-Za-z0-9_-]{6,}\.(js|css)$`)
		for _, ref := range append(append([]string{}, scripts...), styles...) {
			if !hashed.MatchString(ref) {
				t.Errorf("référence %q : attendu un asset haché sous /assets/ (cache immutable)", ref)
			}
			w := get270(h, ref)
			if w.Code != http.StatusOK {
				t.Errorf("GET %s (référencé par index.html) : %d, attendu 200", ref, w.Code)
				continue
			}
			expectMime270(t, ref, w.Header().Get("Content-Type"))
			onDisk, err := os.ReadFile(filepath.Join(dist, filepath.FromSlash(strings.TrimPrefix(ref, "/"))))
			if err != nil {
				t.Errorf("%s référencé par index.html mais absent de dist : %v", ref, err)
				continue
			}
			if !bytes.Equal(w.Body.Bytes(), onDisk) {
				t.Errorf("GET %s : corps différent du fichier dist (%d vs %d octets)", ref, w.Body.Len(), len(onDisk))
			}
			if len(onDisk) == 0 {
				t.Errorf("%s est vide", ref)
			}
			if cc := w.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") {
				t.Errorf("GET %s : Cache-Control = %q, attendu immutable pour un asset haché", ref, cc)
			}
		}
	})
}

// Chaque fichier de dist/assets (chunks chargés à la demande inclus) et dist/fonts est servi : aucun 404.
func TestDistSmoke270_EveryDistAssetAndFontIsServedWithRightMime(t *testing.T) {
	forEachMode270(t, func(t *testing.T, h *HTTPServer, dist string) {
		served := 0
		for _, dir := range []string{"assets", "fonts"} {
			root := filepath.Join(dist, dir)
			err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					return nil
				}
				rel, _ := filepath.Rel(dist, p)
				urlPath := "/" + filepath.ToSlash(rel)
				w := get270(h, urlPath)
				served++
				if w.Code != http.StatusOK {
					t.Errorf("GET %s : %d, attendu 200 (fichier présent dans dist)", urlPath, w.Code)
					return nil
				}
				expectMime270(t, urlPath, w.Header().Get("Content-Type"))
				onDisk, rerr := os.ReadFile(p)
				if rerr != nil {
					return rerr
				}
				if !bytes.Equal(w.Body.Bytes(), onDisk) {
					t.Errorf("GET %s : corps différent du fichier dist", urlPath)
				}
				if body := strings.ToLower(strings.TrimSpace(w.Body.String())); strings.HasPrefix(body, "<!doctype html") || strings.HasPrefix(body, "<html") {
					t.Errorf("GET %s : renvoie du HTML (repli SPA) au lieu de la ressource", urlPath)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("parcours de dist/%s : %v", dir, err)
			}
		}
		if served == 0 {
			t.Fatal("dist/assets et dist/fonts ne contiennent aucun fichier : build incomplet")
		}
	})
}

// Le CSS minifié (lightningcss avec vite 8) référence encore ses polices : chaque url(/…) est servie.
func TestDistSmoke270_CSSUrlReferencesAreServed(t *testing.T) {
	forEachMode270(t, func(t *testing.T, h *HTTPServer, dist string) {
		cssFiles, _ := filepath.Glob(filepath.Join(dist, "assets", "*.css"))
		if len(cssFiles) == 0 {
			t.Fatal("aucun fichier CSS dans dist/assets")
		}
		refs := map[string]bool{}
		for _, f := range cssFiles {
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			if len(bytes.TrimSpace(b)) == 0 {
				t.Errorf("%s est vide", f)
			}
			// Une feuille minifiée tient sur très peu de lignes : jamais de source map inline ni de HTML.
			if bytes.HasPrefix(bytes.TrimSpace(b), []byte("<")) {
				t.Errorf("%s ressemble à du HTML, pas à du CSS", f)
			}
			for _, m := range cssURL270.FindAllStringSubmatch(string(b), -1) {
				if !strings.HasPrefix(m[1], "//") {
					refs[m[1]] = true
				}
			}
		}
		for _, ref := range sortedKeys270(refs) {
			if w := get270(h, ref); w.Code != http.StatusOK {
				t.Errorf("GET %s (url() du CSS) : %d, attendu 200", ref, w.Code)
			} else {
				expectMime270(t, ref, w.Header().Get("Content-Type"))
			}
		}
	})
}

// Garde-fou inverse : un asset inexistant reste un 404 (pas d'index.html en 200 qui masquerait un build cassé).
func TestDistSmoke270_MissingAssetIs404(t *testing.T) {
	forEachMode270(t, func(t *testing.T, h *HTTPServer, dist string) {
		for _, p := range []string{"/assets/inexistant-AAAAAAAA.js", "/assets/inexistant-AAAAAAAA.css", "/fonts/inexistante.woff2"} {
			if w := get270(h, p); w.Code != http.StatusNotFound {
				t.Errorf("GET %s : %d, attendu 404", p, w.Code)
			}
		}
	})
}

// L'index.html du dist reste cohérent avec le répertoire : chaque asset local référencé existe sur disque.
func TestDistSmoke270_IndexReferencesExistOnDisk(t *testing.T) {
	dist := locateDist270(t)
	html, err := os.ReadFile(filepath.Join(dist, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	scripts, styles := indexRefs270(t, string(html))
	for _, ref := range append(scripts, styles...) {
		if _, err := os.Stat(filepath.Join(dist, filepath.FromSlash(strings.TrimPrefix(ref, "/")))); err != nil {
			t.Errorf("index.html référence %s, absent de dist : %v", ref, err)
		}
	}
}
