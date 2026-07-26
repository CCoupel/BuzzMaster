import { describe, it, expect } from 'vitest'
import { existsSync, readFileSync, statSync, readdirSync } from 'node:fs'
import path from 'node:path'

// ---------------------------------------------------------------------------
// Tests : polices embarquées localement (#115, déploiement air-gapped)
//
// Avant le fix, `index.html` chargeait Fredoka/Inter via
// <link href="https://fonts.googleapis.com/...">, précédé de deux
// <link rel="preconnect"> vers fonts.googleapis.com/fonts.gstatic.com. Le
// réseau de déploiement étant air-gapped, ce CSS ne se chargeait jamais —
// dégradation silencieuse vers les fonts système, jamais détectée par la
// suite de tests existante (aucun test ne portait sur index.html/index.css).
//
// Ces tests figent l'invariant inverse : plus aucune référence réseau vers
// Google Fonts dans les sources servies au navigateur, et les fichiers woff2
// locaux sont bien présents là où `@font-face` (src/styles/index.css) va les
// chercher (`/fonts/*.woff2` → servi depuis `public/fonts/`, copié tel quel
// dans `dist/fonts/` par Vite — voir handoff dev-frontend-20260726-140932.md).
// ---------------------------------------------------------------------------

const WEB_ROOT = path.resolve(__dirname, '..')
const GOOGLE_FONTS_PATTERN = /fonts\.googleapis\.com|fonts\.gstatic\.com/

const EXPECTED_FONT_FILES = ['fredoka-latin.woff2', 'inter-latin.woff2']

describe('index.html — aucune dépendance réseau externe vers Google Fonts (#115)', () => {
  const html = readFileSync(path.join(WEB_ROOT, 'index.html'), 'utf-8')

  it('ne contient aucune référence à fonts.googleapis.com ou fonts.gstatic.com', () => {
    expect(html).not.toMatch(GOOGLE_FONTS_PATTERN)
  })

  it("n'a plus de <link rel=\"preconnect\"> vers un domaine Google Fonts", () => {
    expect(html).not.toMatch(/rel=["']preconnect["'][^>]*fonts\.g(oogleapis|static)/)
  })

  it("n'a plus de <link rel=\"stylesheet\"> externe vers l'API CSS2 de Google Fonts", () => {
    expect(html).not.toMatch(/<link[^>]*stylesheet[^>]*fonts\.googleapis\.com/)
  })
})

describe('src/styles/index.css — @font-face pointe vers des chemins locaux (#115)', () => {
  const css = readFileSync(path.join(WEB_ROOT, 'src/styles/index.css'), 'utf-8')
  // Le fichier source contient un commentaire expliquant le fix (mentionnant
  // volontairement les anciennes URLs Google Fonts pour la traçabilité) — on
  // le retire avant de vérifier l'absence de référence *active* (url(...),
  // @import). Le CSS buildé (dist/, testé plus bas) n'a de toute façon plus
  // aucun commentaire, donc c'est bien ce test-là qui reflète le fichier livré.
  const cssWithoutComments = css.replace(/\/\*[\s\S]*?\*\//g, '')

  it('ne contient aucune référence active (hors commentaire) à fonts.googleapis.com ou fonts.gstatic.com', () => {
    expect(cssWithoutComments).not.toMatch(GOOGLE_FONTS_PATTERN)
  })

  it("déclare une règle @font-face locale pour Fredoka (src: url('/fonts/fredoka-latin.woff2'))", () => {
    const fredokaBlock = css.match(/@font-face\s*{[^}]*font-family:\s*['"]Fredoka['"][^}]*}/s)
    expect(fredokaBlock).not.toBeNull()
    expect(fredokaBlock[0]).toMatch(/src:\s*url\(['"]?\/fonts\/fredoka-latin\.woff2['"]?\)/)
  })

  it("déclare une règle @font-face locale pour Inter (src: url('/fonts/inter-latin.woff2'))", () => {
    const interBlock = css.match(/@font-face\s*{[^}]*font-family:\s*['"]Inter['"][^}]*}/s)
    expect(interBlock).not.toBeNull()
    expect(interBlock[0]).toMatch(/src:\s*url\(['"]?\/fonts\/inter-latin\.woff2['"]?\)/)
  })

  it('les variables --font-display/--font-body référencent bien Fredoka/Inter', () => {
    expect(css).toMatch(/--font-display:\s*['"]Fredoka['"]/)
    expect(css).toMatch(/--font-body:\s*['"]Inter['"]/)
  })
})

describe('public/fonts/ — fichiers woff2 présents à la source (copiés tels quels dans dist/ par Vite, #115)', () => {
  const fontsDir = path.join(WEB_ROOT, 'public/fonts')

  it('le dossier public/fonts/ existe', () => {
    expect(existsSync(fontsDir)).toBe(true)
  })

  EXPECTED_FONT_FILES.forEach((filename) => {
    it(`${filename} est présent et non vide`, () => {
      const filePath = path.join(fontsDir, filename)
      expect(existsSync(filePath)).toBe(true)
      expect(statSync(filePath).size).toBeGreaterThan(0)
    })
  })
})

// ---------------------------------------------------------------------------
// Vérification du build (dist/) — ce que le navigateur reçoit réellement.
// Ces tests s'exécutent seulement si `dist/` existe déjà (post `npm run
// build`) : ce dépôt ne fait pas tourner `npm run build` avant `npm test`
// (voir package.json / CI), donc ils ne doivent jamais faire échouer une
// exécution normale de la suite — ils valident le build quand il est présent
// (ex. juste après `npm run build`, ou en CI si l'ordre est un jour inversé),
// sans imposer ce prérequis à la suite de tests unitaires du quotidien.
// ---------------------------------------------------------------------------
const distDir = path.join(WEB_ROOT, 'dist')
const distExists = existsSync(distDir)

describe.skipIf(!distExists)('dist/ (build) — aucune référence Google Fonts, polices embarquées (#115)', () => {
  it('dist/index.html ne contient aucune référence à fonts.googleapis.com ou fonts.gstatic.com', () => {
    const distHtml = readFileSync(path.join(distDir, 'index.html'), 'utf-8')
    expect(distHtml).not.toMatch(GOOGLE_FONTS_PATTERN)
  })

  it('dist/fonts/ contient les 2 fichiers woff2 embarqués', () => {
    const distFontsDir = path.join(distDir, 'fonts')
    expect(existsSync(distFontsDir)).toBe(true)
    EXPECTED_FONT_FILES.forEach((filename) => {
      const filePath = path.join(distFontsDir, filename)
      expect(existsSync(filePath)).toBe(true)
      expect(statSync(filePath).size).toBeGreaterThan(0)
    })
  })

  it('aucun CSS généré dans dist/assets/ ne référence fonts.googleapis.com ou fonts.gstatic.com', () => {
    const assetsDir = path.join(distDir, 'assets')
    const cssFiles = readdirSync(assetsDir).filter((f) => f.endsWith('.css'))
    expect(cssFiles.length).toBeGreaterThan(0)
    cssFiles.forEach((file) => {
      const content = readFileSync(path.join(assetsDir, file), 'utf-8')
      expect(content).not.toMatch(GOOGLE_FONTS_PATTERN)
    })
  })

  it('le CSS généré référence les polices locales /fonts/*.woff2', () => {
    const assetsDir = path.join(distDir, 'assets')
    const cssFiles = readdirSync(assetsDir).filter((f) => f.endsWith('.css'))
    const allCss = cssFiles.map((f) => readFileSync(path.join(assetsDir, f), 'utf-8')).join('\n')
    expect(allCss).toMatch(/url\(\/fonts\/fredoka-latin\.woff2\)/)
    expect(allCss).toMatch(/url\(\/fonts\/inter-latin\.woff2\)/)
  })
})
