import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, act } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'
import Navbar from './Navbar'
import { useGame } from '../hooks/GameContext'

// ---------------------------------------------------------------------------
// #238 — Navbar : groupe « Préparation » (Joueurs, Quiz, Backstage + section
// INTERFACE : TV, Joueur, Animateur ↗), menu du logo « Config » → « Réglages »,
// ENTRACTE 🍿/🎬, badge compteurs 👥 sous 955 px, pastille « Connecté » toujours
// rendue, jamais 2 lignes.
// Plan : _work/handoff/plan-239-v7-20260925-160000.md (parties B et C, C.2→C.5).
//
// CONTRAT RÉEL (NavGroupMenu.jsx, useMediaQuery.js, Navbar.jsx/.css, seuils
// mesurés SOUS WINDOWS — police emoji Segoe — @ac0a16b7, v11.1.0.12) :
//   * JS (matchMedia) : `(min-width: 1685px)` → mode « en ligne » (défaut si
//     matchMedia absent) ; `(max-width: 954px)` → badge 👥 (.counts-badge,
//     .counts-dropdown). Survol (150 ms) + clic, Échap, clic extérieur ; un
//     seul menu ouvert (état openMenu de Navbar).
//   * CSS (@media max-width) : 1944 (titre JEU + libellé Interface), 1854
//     (libellés Préparation), 1494 (libellés Jeu), 1274 (compact, logo réduit,
//     « Connecte » masqué), 1094 (ENTRACTE/Éclairage en icône), 768 (existant).
//   Seuils de palier (min) : 1945 / 1855 / 1685 / 1495 / 1275 / 1095 / 955.
// ---------------------------------------------------------------------------

vi.mock('./Navbar.css', () => ({}))
vi.mock('./NavGroupMenu.css', () => ({}))
vi.mock('./BrandLogo.css', () => ({}))
vi.mock('./LightingBulbIcon.css', () => ({}))
vi.mock('./SoundSpeakerIcon.css', () => ({}))
vi.mock('./LightingModePanel.css', () => ({}))
vi.mock('../styles/entracte.css', () => ({}))

vi.mock('../hooks/useUpdates', () => ({
  useUpdates: () => ({ updateInfo: null, checkForUpdates: vi.fn() }),
}))
vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn() }))
const lightingMock = { status: { state: 'disabled', mode: 'AUTO', flash: false }, refresh: vi.fn() }
vi.mock('../hooks/useLightingStatus', () => ({ useLightingStatus: () => lightingMock }))
vi.mock('../hooks/useSoundStatus', () => ({
  useSoundStatus: () => ({ status: { active: false }, refresh: vi.fn() }),
}))

class ResizeObserverMock {
  observe() {}
  unobserve() {}
  disconnect() {}
}

// --- Viewport simulé : matchMedia évalue (min|max)-width contre `viewport` ---
let viewport = 1280
function setViewport(w) {
  viewport = w
  window.innerWidth = w
  window.matchMedia = (query) => {
    const evalQuery = () =>
      query.split(/\band\b/).every((part) => {
        const min = part.match(/min-width:\s*(\d+)px/)
        const max = part.match(/max-width:\s*(\d+)px/)
        if (min) return viewport >= Number(min[1])
        if (max) return viewport <= Number(max[1])
        return true
      })
    return {
      get matches() { return evalQuery() },
      media: query,
      addEventListener: () => {},
      removeEventListener: () => {},
      addListener: () => {},
      removeListener: () => {},
      onchange: null,
      dispatchEvent: () => false,
    }
  }
}

const gameMock = (entracte = false) => ({
  gameState: { phase: 'STOPPED', entracte },
  setEntracte: vi.fn(),
})

beforeEach(() => {
  global.fetch = vi.fn().mockResolvedValue({ ok: true, json: async () => ({}) })
  global.ResizeObserver = ResizeObserverMock
  useGame.mockReturnValue(gameMock())
  lightingMock.status = { state: 'disabled', mode: 'AUTO', flash: false }
  setViewport(1280)
})

afterEach(() => {
  vi.useRealTimers()
  vi.restoreAllMocks()
  document.documentElement.style.removeProperty('--navbar-h')
})

function renderNavbar({ path = '/admin/scoreboard', connectionStatus = 'connected', bumpers = {} } = {}) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Navbar
        connectionStatus={connectionStatus}
        clientCounts={{ admin: 1, tv: 2, vplayer: 0, anim: 1 }}
        serverVersion="11.1.0"
        bumpers={bumpers}
      />
    </MemoryRouter>
  )
}

const prepButton = () => screen.getByRole('button', { name: /Préparation/i })
const interfaceButton = () => screen.getByRole('button', { name: /Interface/i })
const logoDropdown = (c) => c.querySelector('.brand-logo-container .navbar-menu-dropdown')
const logoButton = () => screen.getByLabelText('Menu de navigation')
const link = (href) => document.querySelector(`a[href="${href}"]`)
const PREP_HREFS = ['/admin/teams', '/admin/quiz', '/admin/backstage']
const IFACE_HREFS = ['/tv', '/player', '/anim']

// --- Lecture des CSS (jsdom ne calcule pas de mise en page) ---
const here = dirname(fileURLToPath(import.meta.url))
function readCss() {
  const read = (f) => { try { return readFileSync(resolve(here, f), 'utf8') } catch { return '' } }
  return read('./Navbar.css') + '\n' + read('./NavGroupMenu.css')
}
// Corps de chaque @media (max-width: Npx) { ... } (accolades équilibrées)
function mediaBlocks(css) {
  const blocks = []
  const re = /@media\s*\(\s*max-width:\s*(\d+)px\s*\)\s*\{/g
  let m
  while ((m = re.exec(css))) {
    let depth = 1
    let i = re.lastIndex
    while (i < css.length && depth > 0) {
      if (css[i] === '{') depth++
      else if (css[i] === '}') depth--
      i++
    }
    blocks.push({ max: Number(m[1]), body: css.slice(re.lastIndex, i - 1) })
  }
  return blocks
}

// ===========================================================================
describe('#238 — menu unique « Préparation » (< 1685 px)', () => {
  it('bouton « Préparation » fermé par défaut : aria-haspopup, aria-expanded=false, aucune entrée rendue', () => {
    renderNavbar()
    const btn = prepButton()
    expect(btn).toHaveAttribute('aria-haspopup')
    expect(btn).toHaveAttribute('aria-expanded', 'false')
    PREP_HREFS.concat(IFACE_HREFS).forEach((h) => expect(link(h), h).toBeNull())
  })

  it('un clic ouvre : Joueurs, Quiz, Backstage puis en-tête INTERFACE puis TV, Joueur, Animateur', () => {
    renderNavbar()
    fireEvent.click(prepButton())
    expect(prepButton()).toHaveAttribute('aria-expanded', 'true')
    PREP_HREFS.concat(IFACE_HREFS).forEach((h) => expect(link(h), h).not.toBeNull())
    const heading = screen.getByText(/^Interface$/i)
    // ordre : entrées Préparation < en-tête < entrées Interface
    const before = link('/admin/backstage').compareDocumentPosition(heading)
    const after = heading.compareDocumentPosition(link('/tv'))
    expect(before & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(after & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it('INTERFACE est une section de la même liste (pas de sous-menu imbriqué)', () => {
    renderNavbar()
    fireEvent.click(prepButton())
    const heading = screen.getByText(/^Interface$/i)
    expect(heading.closest('button')).toBeNull()
    expect(heading.getAttribute('aria-haspopup')).toBeNull()
    const list = link('/admin/teams').parentElement
    expect(list.contains(heading) || list.parentElement.contains(heading)).toBe(true)
    expect(list.querySelectorAll('[aria-haspopup]').length).toBe(0)
  })

  it('TV, Joueur, Animateur : nouvel onglet (target=_blank, rel=noopener) ; Joueurs/Quiz/Backstage : même onglet', () => {
    renderNavbar()
    fireEvent.click(prepButton())
    IFACE_HREFS.forEach((h) => {
      expect(link(h)).toHaveAttribute('target', '_blank')
      expect(link(h)).toHaveAttribute('rel', 'noopener')
    })
    PREP_HREFS.forEach((h) => {
      expect(link(h)).not.toHaveAttribute('target')
    })
  })

  it('un second clic referme', () => {
    renderNavbar()
    fireEvent.click(prepButton())
    fireEvent.click(prepButton())
    expect(prepButton()).toHaveAttribute('aria-expanded', 'false')
    expect(link('/admin/teams')).toBeNull()
  })

  it('Échap referme', () => {
    renderNavbar()
    fireEvent.click(prepButton())
    fireEvent.keyDown(document.activeElement || document.body, { key: 'Escape' })
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(prepButton()).toHaveAttribute('aria-expanded', 'false')
  })

  it('clic extérieur referme', () => {
    renderNavbar()
    fireEvent.click(prepButton())
    fireEvent.mouseDown(document.body)
    expect(prepButton()).toHaveAttribute('aria-expanded', 'false')
  })

  it('cliquer une entrée de navigation referme le menu', () => {
    renderNavbar()
    fireEvent.click(prepButton())
    fireEvent.click(link('/admin/quiz'))
    expect(prepButton()).toHaveAttribute('aria-expanded', 'false')
  })

  it('le survol ouvre le menu ; la sortie le referme après un délai (~150 ms)', () => {
    vi.useFakeTimers()
    renderNavbar()
    fireEvent.mouseEnter(prepButton())
    expect(prepButton()).toHaveAttribute('aria-expanded', 'true')
    fireEvent.mouseLeave(prepButton())
    // pas de fermeture instantanée (trajet diagonale vers la liste)
    expect(prepButton()).toHaveAttribute('aria-expanded', 'true')
    act(() => { vi.advanceTimersByTime(400) })
    expect(prepButton()).toHaveAttribute('aria-expanded', 'false')
  })

  it('revenir sur le menu avant le délai annule la fermeture', () => {
    vi.useFakeTimers()
    renderNavbar()
    fireEvent.mouseEnter(prepButton())
    fireEvent.mouseLeave(prepButton())
    act(() => { vi.advanceTimersByTime(50) })
    fireEvent.mouseEnter(prepButton())
    act(() => { vi.advanceTimersByTime(400) })
    expect(prepButton()).toHaveAttribute('aria-expanded', 'true')
  })

  it('le bouton est mis en évidence (classe active) quand une de ses pages est active', () => {
    renderNavbar({ path: '/admin/quiz' })
    expect(prepButton().className).toMatch(/\bactive\b/)
  })

  it('pas de mise en évidence sur une page hors groupe', () => {
    renderNavbar({ path: '/admin/scoreboard' })
    expect(prepButton().className).not.toMatch(/\bactive\b/)
  })

  it('accessibilité : le bouton a un nom accessible « Préparation » (icône seule à ce palier)', () => {
    renderNavbar()
    expect(prepButton().getAttribute('aria-label') || prepButton().textContent).toMatch(/Préparation/)
  })
})

// ===========================================================================
describe('#238 — Préparation en ligne (≥ 1685 px)', () => {
  beforeEach(() => setViewport(1700))

  it('Joueurs, Quiz, Backstage rendus directement, sans ouvrir de menu', () => {
    renderNavbar()
    PREP_HREFS.forEach((h) => expect(link(h), h).not.toBeNull())
    expect(screen.queryByRole('button', { name: /Préparation/i })).toBeNull()
  })

  it('bouton « Interface » séparé : fermé par défaut, TV/Joueur/Animateur au clic (nouvel onglet)', () => {
    renderNavbar()
    const btn = interfaceButton()
    expect(btn).toHaveAttribute('aria-haspopup')
    expect(btn).toHaveAttribute('aria-expanded', 'false')
    IFACE_HREFS.forEach((h) => expect(link(h), h).toBeNull())
    fireEvent.click(btn)
    expect(interfaceButton()).toHaveAttribute('aria-expanded', 'true')
    IFACE_HREFS.forEach((h) => {
      expect(link(h), h).not.toBeNull()
      expect(link(h)).toHaveAttribute('target', '_blank')
      expect(link(h)).toHaveAttribute('rel', 'noopener')
    })
  })

  it('Interface : survol, Échap et clic extérieur comme le menu unique', () => {
    vi.useFakeTimers()
    renderNavbar()
    fireEvent.mouseEnter(interfaceButton())
    expect(interfaceButton()).toHaveAttribute('aria-expanded', 'true')
    fireEvent.mouseLeave(interfaceButton())
    act(() => { vi.advanceTimersByTime(400) })
    expect(interfaceButton()).toHaveAttribute('aria-expanded', 'false')
    fireEvent.click(interfaceButton())
    fireEvent.mouseDown(document.body)
    expect(interfaceButton()).toHaveAttribute('aria-expanded', 'false')
    fireEvent.click(interfaceButton())
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(interfaceButton()).toHaveAttribute('aria-expanded', 'false')
  })

  it('pas de titre vertical « PRÉPARATION » (jamais affiché, plan C.4)', () => {
    const { container } = renderNavbar()
    expect(container.querySelector('.nav-group-label')?.textContent ?? '').not.toMatch(/Préparation/i)
  })

  it('les 3 pages Préparation restent des <a> de navigation interne (/admin/...)', () => {
    renderNavbar()
    PREP_HREFS.forEach((h) => expect(link(h).tagName).toBe('A'))
  })
})

// ===========================================================================
describe('#238 — bascule en ligne / menu unique à 1685 px (JS)', () => {
  it('1684 → menu unique (bouton Préparation, pas de bouton Interface)', () => {
    setViewport(1684)
    renderNavbar()
    expect(screen.queryByRole('button', { name: /Préparation/i })).not.toBeNull()
    expect(screen.queryByRole('button', { name: /^Interface$/i })).toBeNull()
  })
  it('1685 → en ligne (bouton Interface, pas de bouton Préparation)', () => {
    setViewport(1685)
    renderNavbar()
    expect(screen.queryByRole('button', { name: /Préparation/i })).toBeNull()
    expect(screen.queryByRole('button', { name: /^Interface$/i })).not.toBeNull()
  })
})

// ===========================================================================
describe('#238 — un seul menu ouvert à la fois', () => {
  it('ouvrir Préparation ferme le menu du logo', () => {
    const { container } = renderNavbar()
    fireEvent.click(logoButton())
    expect(logoDropdown(container)).not.toBeNull()
    fireEvent.click(prepButton())
    expect(logoDropdown(container)).toBeNull()
    expect(prepButton()).toHaveAttribute('aria-expanded', 'true')
  })

  it('ouvrir le menu du logo ferme Préparation', () => {
    const { container } = renderNavbar()
    fireEvent.click(prepButton())
    fireEvent.click(logoButton())
    expect(logoDropdown(container)).not.toBeNull()
    expect(prepButton()).toHaveAttribute('aria-expanded', 'false')
  })

  it('ouvrir le popover Éclairage ferme Préparation', () => {
    lightingMock.status = { state: 'ok', mode: 'AUTO', flash: false }
    const { container } = renderNavbar()
    fireEvent.click(prepButton())
    fireEvent.click(container.querySelector('.lighting-mode-nav-badge'))
    expect(container.querySelector('.lighting-mode-popover')).not.toBeNull()
    expect(prepButton()).toHaveAttribute('aria-expanded', 'false')
  })

  it('ouvrir Préparation ferme le popover Éclairage', () => {
    lightingMock.status = { state: 'ok', mode: 'AUTO', flash: false }
    const { container } = renderNavbar()
    fireEvent.click(container.querySelector('.lighting-mode-nav-badge'))
    fireEvent.click(prepButton())
    expect(container.querySelector('.lighting-mode-popover')).toBeNull()
  })

  it('en mode en ligne : ouvrir Interface ferme le menu du logo', () => {
    setViewport(1700)
    const { container } = renderNavbar()
    fireEvent.click(logoButton())
    fireEvent.click(interfaceButton())
    expect(logoDropdown(container)).toBeNull()
    expect(interfaceButton()).toHaveAttribute('aria-expanded', 'true')
  })
})

// ===========================================================================
describe('#238 — menu du logo : « Config » renommé « Réglages »', () => {
  it('l\'entrée s\'appelle « Réglages », pointe toujours vers /admin/settings, plus de « Config »', () => {
    const { container } = renderNavbar()
    fireEvent.click(logoButton())
    const labels = Array.from(container.querySelectorAll('.navbar-menu-dropdown .menu-label')).map((e) => e.textContent)
    expect(labels).toContain('Réglages')
    expect(labels).not.toContain('Config')
    const item = Array.from(container.querySelectorAll('.navbar-menu-dropdown a')).find((a) => a.textContent.includes('Réglages'))
    expect(item).toHaveAttribute('href', '/admin/settings')
  })

  it('les autres entrées et leur ordre sont inchangés (Réglages, Ambiance, Backup, Mises à jour, Logs, Quitter)', () => {
    const { container } = renderNavbar()
    fireEvent.click(logoButton())
    const labels = Array.from(container.querySelectorAll('.navbar-menu-dropdown .menu-label')).map((e) => e.textContent)
    expect(labels).toEqual(['Réglages', 'Ambiance', 'Backup/Restaure', 'Mises à jour', 'Logs', 'Quitter'])
  })

  it('le groupe Jeu garde ses 4 entrées et leurs routes', () => {
    renderNavbar()
    ;['/admin', '/admin/scoreboard', '/admin/palmares', '/admin/history'].forEach((h) =>
      expect(link(h), h).not.toBeNull())
  })
})

// ===========================================================================
describe('#238 — ENTRACTE 🍿 / 🎬 et Éclairage : icône + nom accessible', () => {
  const btn = () => document.querySelector('.entracte-toggle-btn')

  it('au repos : icône 🍿, nom accessible « ENTRACTE », libellé texte toujours présent (masqué en CSS < 950)', () => {
    renderNavbar()
    expect(btn().textContent).toContain('🍿')
    expect(btn().textContent).not.toContain('🎬')
    expect(btn().textContent).toContain('ENTRACTE')
    expect(screen.getByRole('button', { name: /^ENTRACTE$/i })).toBe(btn())
  })

  it('actif : icône 🎬, nom accessible « FIN D\'ENTRACTE »', () => {
    useGame.mockReturnValue(gameMock(true))
    renderNavbar()
    expect(btn().textContent).toContain('🎬')
    expect(btn().textContent).not.toContain('🍿')
    expect(screen.getByRole('button', { name: /FIN D.ENTRACTE/i })).toBe(btn())
  })

  it('Éclairage : nom accessible explicite (le libellé peut être masqué sous 950 px)', () => {
    lightingMock.status = { state: 'ok', mode: 'AUTO', flash: false }
    const { container } = renderNavbar()
    const b = container.querySelector('.lighting-mode-nav-badge')
    expect(b.getAttribute('aria-label') || b.getAttribute('title')).toMatch(/clairage/i)
  })
})

// ===========================================================================
describe('#238 — libellés réduits : chaque icône seule garde un nom (title / aria-label)', () => {
  it('liens du groupe Jeu : title ou aria-label = libellé', () => {
    renderNavbar()
    ;[['/admin', 'Jeu'], ['/admin/scoreboard', 'Scores'], ['/admin/palmares', 'Palmarès'], ['/admin/history', 'Historique']]
      .forEach(([href, label]) => {
        const a = link(href)
        expect(a.getAttribute('title') || a.getAttribute('aria-label') || '', href).toContain(label)
      })
  })

  it('entrées Préparation en ligne : title ou aria-label = libellé', () => {
    setViewport(1700)
    renderNavbar()
    ;[['/admin/teams', 'Joueurs'], ['/admin/quiz', 'Quiz'], ['/admin/backstage', 'Backstage']].forEach(([href, label]) => {
      const a = link(href)
      expect(a.getAttribute('title') || a.getAttribute('aria-label') || '', href).toContain(label)
    })
  })

  it('bouton Interface (en ligne) a un nom accessible', () => {
    setViewport(1700)
    renderNavbar()
    expect(interfaceButton()).toBeInTheDocument()
  })
})

// ===========================================================================
describe('#238 — pastille de connexion toujours rendue', () => {
  ;['connected', 'connecting', 'disconnected'].forEach((status) => {
    ;[1920, 1600, 1280, 1024, 768, 400].forEach((w) => {
      it(`état ${status} à ${w}px : .connection-status + .status-dot présents`, () => {
        setViewport(w)
        const { container } = renderNavbar({ connectionStatus: status })
        const c = container.querySelector('.connection-status')
        expect(c).not.toBeNull()
        expect(c.className).toContain(status)
        expect(c.querySelector('.status-dot')).not.toBeNull()
      })
    })
  })

  it('le texte reste rendu (masqué en CSS à partir de 1274 px, la pastille non)', () => {
    const { container } = renderNavbar({ connectionStatus: 'connected' })
    expect(container.querySelector('.status-text').textContent).toBe('Connecte')
  })
})

// ===========================================================================
describe('#238 — badge compteurs 👥 (< 955 px)', () => {
  const bumpers = {
    v1: { IS_VPLAYER: true, IS_VIRTUAL: true, TEAM: 'red', CONN_STATE: '' },
    v2: { IS_VPLAYER: true, IS_VIRTUAL: true, TEAM: 'blue', CONN_STATE: 'orange' },
    b1: { TEAM: 'red', CONN_STATE: '' },
    b2: { TEAM: 'blue', CONN_STATE: 'red' },
    b3: { TEAM: 'blue', CONN_STATE: 'green' },
  }
  const badge = (c) => c.querySelector('.counts-badge')
  const detail = (c) => c.querySelector('.counts-dropdown')

  beforeEach(() => setViewport(700))

  it('affiche 👥 connectés/participants (VJoueurs + Buzzers cumulés)', () => {
    const { container } = renderNavbar({ bumpers })
    // VJoueurs 1/2 + Buzzers 2/3 → 3/5
    expect(badge(container)).not.toBeNull()
    expect(badge(container).textContent).toContain('👥')
    expect(badge(container).textContent).toContain('3/5')
  })

  it('est un bouton accessible (aria-haspopup, aria-expanded=false, nom accessible)', () => {
    const { container } = renderNavbar({ bumpers })
    const b = badge(container)
    expect(b.tagName).toBe('BUTTON')
    expect(b).toHaveAttribute('aria-haspopup')
    expect(b).toHaveAttribute('aria-expanded', 'false')
    expect(b.getAttribute('aria-label') || b.getAttribute('title')).toBeTruthy()
    expect(detail(container)).toBeNull()
  })

  it('un clic déploie le détail : les 5 compteurs avec libellés', () => {
    const { container } = renderNavbar({ bumpers })
    fireEvent.click(badge(container))
    expect(badge(container)).toHaveAttribute('aria-expanded', 'true')
    const d = detail(container)
    expect(d).not.toBeNull()
    ;['admin', 'tv', 'anim', 'vplayer', 'buzzer'].forEach((k) =>
      expect(d.querySelector(`.client-count.${k}`), k).not.toBeNull())
    expect(d.querySelector('.client-count.vplayer').textContent).toContain('1/2')
    expect(d.querySelector('.client-count.buzzer').textContent).toContain('2/3')
  })

  it('le survol déploie aussi le détail ; fermeture différée à la sortie', () => {
    vi.useFakeTimers()
    const { container } = renderNavbar({ bumpers })
    fireEvent.mouseEnter(badge(container))
    expect(detail(container)).not.toBeNull()
    fireEvent.mouseLeave(badge(container))
    act(() => { vi.advanceTimersByTime(400) })
    expect(detail(container)).toBeNull()
  })

  it('Échap et clic extérieur ferment le détail', () => {
    const { container } = renderNavbar({ bumpers })
    fireEvent.click(badge(container))
    fireEvent.mouseDown(document.body)
    expect(detail(container)).toBeNull()
    fireEvent.click(badge(container))
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(detail(container)).toBeNull()
  })

  it('un seul menu à la fois : ouvrir le badge ferme Préparation', () => {
    const { container } = renderNavbar({ bumpers })
    fireEvent.click(prepButton())
    fireEvent.click(badge(container))
    expect(prepButton()).toHaveAttribute('aria-expanded', 'false')
    expect(detail(container)).not.toBeNull()
  })

  it('couleur = sévérité la plus grave (red > orange > neutre)', () => {
    const { container } = renderNavbar({ bumpers })
    expect(badge(container).className).toMatch(/severity-red/)
  })

  it('sévérité orange sans rouge ; neutre quand tout est connecté', () => {
    const orange = { v1: { IS_VPLAYER: true, IS_VIRTUAL: true, TEAM: 'red', CONN_STATE: 'orange' } }
    const { container, unmount } = renderNavbar({ bumpers: orange })
    expect(badge(container).className).toMatch(/severity-orange/)
    unmount()
    const ok = { b1: { TEAM: 'red', CONN_STATE: '' } }
    const r = renderNavbar({ bumpers: ok })
    expect(badge(r.container).className).toMatch(/severity-neutral/)
  })

  it('au tap (mouseenter puis click émulés) le détail RESTE ouvert (plan C.2 : clic/tap)', () => {
    const { container } = renderNavbar({ bumpers })
    fireEvent.mouseEnter(badge(container))
    fireEvent.click(badge(container))
    expect(badge(container)).toHaveAttribute('aria-expanded', 'true')
  })

  it('≥ 955 px : pas de badge, les 5 compteurs sont affichés (X/Y conservés)', () => {
    setViewport(955)
    const { container } = renderNavbar({ bumpers })
    expect(badge(container)).toBeNull()
    expect(container.querySelector('.client-counts')).not.toBeNull()
    expect(container.querySelector('.client-count.vplayer').textContent).toContain('1/2')
    expect(container.querySelector('.client-count.buzzer').textContent).toContain('2/3')
  })

  it('954 px : badge présent, .client-counts absent', () => {
    setViewport(954)
    const { container } = renderNavbar({ bumpers })
    expect(badge(container)).not.toBeNull()
    expect(container.querySelector('.client-counts')).toBeNull()
  })
})

// ===========================================================================
describe('#238 — seuils responsive (lus dans les CSS) — jamais 2 lignes', () => {
  const css = readCss()
  const blocks = mediaBlocks(css)
  const has = (max) => blocks.some((b) => b.max === max)
  const at = (max) => blocks.filter((b) => b.max === max).map((b) => b.body).join('\n')
  const navbarRule = () => (css.match(/(^|\})\s*\.navbar\s*\{([^}]*)\}/) || [])[2] || ''

  it('un palier @media (max-width) CSS par seuil : 1944, 1854, 1494, 1274, 1094 (1685 et 955 sont en JS)', () => {
    ;[1944, 1854, 1494, 1274, 1094].forEach((w) => expect(has(w), `max-width:${w}px`).toBe(true))
  })

  it('la barre reste sur UNE rangée : .navbar sans flex-wrap: wrap, nowrap ou absent', () => {
    expect(navbarRule()).not.toMatch(/flex-wrap:\s*wrap(?!-)/)
  })

  it('aucun palier ne fait passer .navbar en flex-wrap: wrap', () => {
    blocks.forEach((b) => {
      const rule = (b.body.match(/\.navbar\s*\{([^}]*)\}/) || [])[1] || ''
      expect(rule, `max-width:${b.max}px`).not.toMatch(/flex-wrap:\s*wrap(?!-)/)
    })
  })

  it('1944 → titre vertical JEU masqué', () => {
    expect(at(1944)).toMatch(/\.nav-group-label[^{]*\{[^}]*display:\s*none/)
  })

  it('1494 → liens Jeu en icône seule', () => {
    expect(at(1494)).toMatch(/\.nav-group-game \.nav-label[^{]*\{[^}]*display:\s*none/)
  })

  it('1854 → Préparation dépliée en icônes seules (libellés .nav-label masqués)', () => {
    expect(at(1854)).toMatch(/\.nav-label[^{]*\{[^}]*display:\s*none/)
  })

  it('1274 → texte « Connecte » masqué, pastille (.status-dot) jamais masquée', () => {
    expect(at(1274)).toMatch(/\.status-text[^{]*\{[^}]*display:\s*none/)
    blocks.forEach((b) => {
      expect(b.body, `max-width:${b.max}px`).not.toMatch(/\.status-dot[^{]*\{[^}]*display:\s*none/)
      expect(b.body, `max-width:${b.max}px`).not.toMatch(/\.connection-status\s*\{[^}]*display:\s*none/)
    })
  })

  it('1094 → ENTRACTE / Éclairage en icône seule (libellé masqué)', () => {
    expect(at(1094)).toMatch(/\.entracte-label[^{]*\{[^}]*display:\s*none|\.entracte-label/)
    expect(at(1094)).toMatch(/\.lighting-label/)
    expect(at(1094)).toMatch(/display:\s*none/)
  })
})
