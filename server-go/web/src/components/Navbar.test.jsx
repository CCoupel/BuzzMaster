import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import Navbar from './Navbar'

// ---------------------------------------------------------------------------
// Tests : Navbar — compteurs participants X/Y + coloration par sévérité
//
// Voir plan _work/reports/planner-20260725-105503-final.md §3/§5/§9.
// Contrat implémenté (computeParticipantCounts/aggregateSeverity, Navbar.jsx) :
//
//   - admin/tv : inchangés, nombre brut depuis `clientCounts`, jamais colorés.
//   - `.client-count.vplayer` : compteur participants VJoueurs (IS_VPLAYER
//     === true && TEAM non vide), texte "connectés/participants".
//   - `.client-count.buzzer` : compteur participants buzzers physiques
//     (!IS_VIRTUAL && !IS_VPLAYER && TEAM non vide), même format.
//   - Connecté (pour le X) = CONN_STATE ∈ {"", "green"}.
//   - Classe de sévérité TOUJOURS présente : `severity-red` | `severity-orange`
//     | `severity-neutral` (priorité stricte red > orange > neutre).
// ---------------------------------------------------------------------------

vi.mock('./Navbar.css', () => ({}))

vi.mock('../hooks/useUpdates', () => ({
  useUpdates: () => ({
    updateInfo: null,
    checkForUpdates: vi.fn(),
  }),
}))

// #207 — Navbar consomme useLightingStatus (ampoule de l'entrée « Ambiance »,
// GET /api/lighting/status au montage + toutes les 30 s). Mock ici pour la
// même raison que useUpdates : les tests #175 ci-dessous comptent les appels
// `fetch` globaux (« aucun fetch », « fetch('/shutdown') appelé une fois »).
// Couverture propre de l'entrée dans Navbar.ambiance.test.jsx.
vi.mock('../hooks/useLightingStatus', () => ({
  useLightingStatus: () => ({ status: { state: 'disabled' }, refresh: vi.fn() }),
}))

// ENTRACTE (#119, delta C2) — Navbar consomme désormais useGame() (bouton
// ENTRACTE / FIN D'ENTRACTE, cf. Navbar.entracte.test.jsx pour sa propre
// couverture). Mock minimal ici, additif et sans rapport avec les tests
// préexistants de ce fichier (compteurs participants) — sans lui, TOUT
// rendu de <Navbar> planterait ("useGame must be used within a
// GameProvider"), ce fichier entier serait rouge pour une raison hors de
// son sujet.
vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(() => ({
    gameState: { phase: 'STOPPED', entracte: false },
    setEntracte: vi.fn(),
  })),
}))

const renderNavbar = (props = {}) =>
  render(
    <MemoryRouter initialEntries={['/admin']}>
      <Navbar
        connectionStatus="connected"
        clientCounts={{ admin: 2, tv: 1, vplayer: 0 }}
        serverVersion="5.7.11"
        bumpers={{}}
        {...props}
      />
    </MemoryRouter>
  )

// ResizeObserver n'existe pas nativement en jsdom (#179 — Navbar mesure
// désormais sa hauteur via useElementHeightVar, comme RegieMessageBar
// depuis #177). Mock GLOBAL au fichier (même piège que RegieMessageBar.
// test.jsx #177 : scoper le mock à un seul bloc casse TOUS les autres
// rendus de Navbar de ce fichier avec "ResizeObserver is not defined").
class ResizeObserverMock {
  constructor(callback) {
    this.callback = callback
    this.observed = []
    ResizeObserverMock.instances.push(this)
  }
  observe(target) { this.observed.push(target) }
  unobserve() {}
  disconnect() { this.disconnected = true }
  fire(height) {
    this.callback([{ contentRect: { height } }], this)
  }
}
ResizeObserverMock.instances = []

beforeEach(() => {
  global.fetch = vi.fn().mockResolvedValue({ ok: true, json: async () => ({}) })
  ResizeObserverMock.instances = []
  global.ResizeObserver = ResizeObserverMock
  document.documentElement.style.removeProperty('--navbar-h')
})

// ---------------------------------------------------------------------------
// admin/tv : format inchangé (nombre brut, jamais coloré)
// ---------------------------------------------------------------------------

describe('Navbar — compteurs admin/tv (inchangés)', () => {
  it('affiche le nombre brut admin/tv depuis clientCounts, sans coloration', () => {
    const { container } = renderNavbar({ clientCounts: { admin: 3, tv: 2, vplayer: 0 } })

    const adminChip = container.querySelector('.client-count.admin')
    const tvChip = container.querySelector('.client-count.tv')
    expect(adminChip.textContent).toContain('3')
    expect(tvChip.textContent).toContain('2')
    expect(adminChip.className).not.toMatch(/severity-/)
    expect(tvChip.className).not.toMatch(/severity-/)
  })
})

// ---------------------------------------------------------------------------
// #155 (F3) — badge animateur (.client-count.anim) : même motif simple
// (icône + valeur) que .admin/.tv, jamais coloré par sévérité.
// ---------------------------------------------------------------------------

describe('Navbar — compteur animateur (.client-count.anim)', () => {
  it('affiche le nombre brut animateur depuis clientCounts.anim, sans coloration', () => {
    const { container } = renderNavbar({ clientCounts: { admin: 1, tv: 0, vplayer: 0, anim: 2 } })

    const animChip = container.querySelector('.client-count.anim')
    expect(animChip).not.toBeNull()
    expect(animChip.textContent).toContain('2')
    expect(animChip.className).not.toMatch(/severity-/)
  })

  it('affiche 0 quand aucun animateur n\'est connecté', () => {
    const { container } = renderNavbar({ clientCounts: { admin: 1, tv: 0, vplayer: 0, anim: 0 } })

    const animChip = container.querySelector('.client-count.anim')
    expect(animChip.textContent).toContain('0')
  })
})

// ---------------------------------------------------------------------------
// #155 (D4/F3) — TV, Joueur et Animateur ouvrent un nouvel onglet
// (target="_blank" + rel="noopener") ; les entrées non-absolute (Jeu,
// Config, ...) restent dans l'onglet courant.
// ---------------------------------------------------------------------------

describe('Navbar — raccourcis TV/Joueur/Animateur (nouvel onglet, D4)', () => {
  it('ajoute target="_blank" et rel="noopener" sur les 3 entrées absolute', () => {
    // Scoped to .nav-group-tv — "TV" apparaît aussi dans le badge de
    // compteur (.client-count.tv .count-icon), getByText('TV') seul serait
    // ambigu (screen porte sur tout le body).
    const { container } = renderNavbar()
    const links = container.querySelectorAll('.nav-group-tv a.nav-link')
    expect(links).toHaveLength(3)

    links.forEach(link => {
      expect(link).toHaveAttribute('target', '_blank')
      expect(link).toHaveAttribute('rel', 'noopener')
    })

    const hrefs = Array.from(links).map(l => l.getAttribute('href'))
    expect(hrefs).toEqual(['/tv', '/player', '/anim'])
  })

  it('ne met pas target="_blank" sur les entrées non-absolute (ex: Scores)', () => {
    const { container } = renderNavbar()
    const scoresLink = container.querySelector('.nav-group-game a.nav-link[href="/admin/scoreboard"]')
    expect(scoresLink).not.toBeNull()
    expect(scoresLink).not.toHaveAttribute('target')
    expect(scoresLink).not.toHaveAttribute('rel')
  })
})

// ---------------------------------------------------------------------------
// vjoueur (.client-count.vplayer) / buzzer : format X/Y participants
// ---------------------------------------------------------------------------

describe('Navbar — compteur VJoueurs participants (.client-count.vplayer)', () => {
  it('compte uniquement les VJoueurs participants (TEAM assignée), X=connectés/Y=total', () => {
    const bumpers = {
      v1: { NAME: 'Alice', IS_VPLAYER: true, IS_VIRTUAL: true, TEAM: 'red', CONN_STATE: '' },
      v2: { NAME: 'Bob', IS_VPLAYER: true, IS_VIRTUAL: true, TEAM: 'blue', CONN_STATE: 'orange' },
      // Non-participant (TEAM vide) : exclu du total
      v3: { NAME: 'Carla', IS_VPLAYER: true, IS_VIRTUAL: true, TEAM: '', CONN_STATE: 'orange' },
    }
    const { container } = renderNavbar({ bumpers })

    const chip = container.querySelector('.client-count.vplayer')
    expect(chip.textContent).toContain('1/2')
  })

  it('un VJoueur en CONN_STATE="green" compte comme connecté (X)', () => {
    const bumpers = {
      v1: { NAME: 'Alice', IS_VPLAYER: true, IS_VIRTUAL: true, TEAM: 'red', CONN_STATE: 'green' },
    }
    const { container } = renderNavbar({ bumpers })

    const chip = container.querySelector('.client-count.vplayer')
    expect(chip.textContent).toContain('1/1')
  })
})

describe('Navbar — compteur buzzers participants (.client-count.buzzer)', () => {
  it('compte uniquement les buzzers physiques participants (exclut IS_VIRTUAL/IS_VPLAYER)', () => {
    const bumpers = {
      b1: { NAME: 'Buzzer1', TEAM: 'red', CONN_STATE: '' },
      b2: { NAME: 'Buzzer2', TEAM: 'blue', CONN_STATE: 'red' },
      v1: { NAME: 'Alice', IS_VPLAYER: true, IS_VIRTUAL: true, TEAM: 'red', CONN_STATE: '' },
    }
    const { container } = renderNavbar({ bumpers })

    const chip = container.querySelector('.client-count.buzzer')
    expect(chip.textContent).toContain('1/2')
  })

  it('exclut les buzzers non participants (TEAM vide) du total', () => {
    const bumpers = {
      b1: { NAME: 'Buzzer1', TEAM: '', CONN_STATE: '' },
      b2: { NAME: 'Buzzer2', TEAM: 'blue', CONN_STATE: '' },
    }
    const { container } = renderNavbar({ bumpers })

    const chip = container.querySelector('.client-count.buzzer')
    expect(chip.textContent).toContain('1/1')
  })
})

// ---------------------------------------------------------------------------
// Coloration par sévérité agrégée (priorité red > orange > neutre)
// ---------------------------------------------------------------------------

describe('Navbar — coloration par sévérité du groupe', () => {
  it('chip severity-neutral quand tous les participants sont connectés', () => {
    const bumpers = {
      b1: { NAME: 'Buzzer1', TEAM: 'red', CONN_STATE: '' },
      b2: { NAME: 'Buzzer2', TEAM: 'blue', CONN_STATE: 'green' },
    }
    const { container } = renderNavbar({ bumpers })

    const chip = container.querySelector('.client-count.buzzer')
    expect(chip.className).toContain('severity-neutral')
  })

  it('chip severity-orange quand au moins un participant est en CONN_STATE=orange (aucun red)', () => {
    const bumpers = {
      b1: { NAME: 'Buzzer1', TEAM: 'red', CONN_STATE: '' },
      b2: { NAME: 'Buzzer2', TEAM: 'blue', CONN_STATE: 'orange' },
    }
    const { container } = renderNavbar({ bumpers })

    const chip = container.querySelector('.client-count.buzzer')
    expect(chip.className).toContain('severity-orange')
    expect(chip.className).not.toContain('severity-red')
  })

  it('chip severity-red quand au moins un participant est en CONN_STATE=red', () => {
    const bumpers = {
      b1: { NAME: 'Buzzer1', TEAM: 'red', CONN_STATE: 'orange' },
      b2: { NAME: 'Buzzer2', TEAM: 'blue', CONN_STATE: 'red' },
    }
    const { container } = renderNavbar({ bumpers })

    const chip = container.querySelector('.client-count.buzzer')
    expect(chip.className).toContain('severity-red')
  })

  it("priorité stricte red > orange : rouge l'emporte même largement minoritaire", () => {
    const bumpers = {
      b1: { NAME: 'B1', TEAM: 'red', CONN_STATE: 'orange' },
      b2: { NAME: 'B2', TEAM: 'red', CONN_STATE: 'orange' },
      b3: { NAME: 'B3', TEAM: 'red', CONN_STATE: 'orange' },
      b4: { NAME: 'B4', TEAM: 'blue', CONN_STATE: 'red' },
    }
    const { container } = renderNavbar({ bumpers })

    const chip = container.querySelector('.client-count.buzzer')
    expect(chip.className).toContain('severity-red')
    expect(chip.className).not.toContain('severity-orange')
  })

  it('un groupe "green" reste severity-neutral (reconnecté = compte connecté)', () => {
    const bumpers = {
      v1: { NAME: 'Alice', IS_VPLAYER: true, IS_VIRTUAL: true, TEAM: 'red', CONN_STATE: 'green' },
    }
    const { container } = renderNavbar({ bumpers })

    const chip = container.querySelector('.client-count.vplayer')
    expect(chip.className).toContain('severity-neutral')
  })

  it('les groupes vjoueur et buzzer sont colorés indépendamment', () => {
    const bumpers = {
      v1: { NAME: 'Alice', IS_VPLAYER: true, IS_VIRTUAL: true, TEAM: 'red', CONN_STATE: 'red' },
      b1: { NAME: 'Buzzer1', TEAM: 'blue', CONN_STATE: '' },
    }
    const { container } = renderNavbar({ bumpers })

    expect(container.querySelector('.client-count.vplayer').className).toContain('severity-red')
    expect(container.querySelector('.client-count.buzzer').className).toContain('severity-neutral')
  })
})

describe('Navbar — cas limite : aucun participant', () => {
  it('affiche 0/0 severity-neutral quand bumpers est vide', () => {
    const { container } = renderNavbar({ bumpers: {} })

    const vjoueurChip = container.querySelector('.client-count.vplayer')
    const buzzerChip = container.querySelector('.client-count.buzzer')
    expect(vjoueurChip.textContent).toContain('0/0')
    expect(buzzerChip.textContent).toContain('0/0')
    expect(vjoueurChip.className).toContain('severity-neutral')
    expect(buzzerChip.className).toContain('severity-neutral')
  })

  it('ne plante pas quand bumpers est undefined (valeur par défaut)', () => {
    expect(() => renderNavbar({ bumpers: undefined })).not.toThrow()
  })
})

// ---------------------------------------------------------------------------
// #175 — entrée « Quitter » du menu déroulant (T1-T4).
//
// Plan : _work/reports/plan-20260818-140953.md, tâches F1/F2. Contrat :
// aucun (GET /shutdown existe déjà, contracts/http-endpoints.md:554).
//
// AC6 est le point de vigilance central : cette entrée est la SEULE du menu
// qui ne soit pas une navigation — un `href`/`to` résiduel serait
// préchargeable par le navigateur (arrêt du serveur au survol du menu).
// ---------------------------------------------------------------------------

// openMenu clicks the brand-logo button (aria-label="Menu de navigation")
// that toggles .navbar-menu-dropdown — no dedicated test id exists for it,
// this is the only way to reach the dropdown's content.
function openMenu() {
  fireEvent.click(screen.getByLabelText('Menu de navigation'))
}

function getDropdown(container) {
  return container.querySelector('.navbar-menu-dropdown')
}

afterEach(() => {
  vi.restoreAllMocks()
  document.documentElement.style.removeProperty('--navbar-h')
})

describe('#175 — entrée « Quitter », présence et nature (T1, AC1, AC6)', () => {
  it('apparaît dans le menu déroulant, en DERNIÈRE position, après Logs', () => {
    const { container } = renderNavbar()
    openMenu()
    const dropdown = getDropdown(container)
    expect(dropdown).not.toBeNull()

    const labels = Array.from(dropdown.querySelectorAll('.menu-label')).map(el => el.textContent)
    expect(labels[labels.length - 1]).toBe('Quitter')
    expect(labels).toContain('Logs')
    expect(labels.indexOf('Quitter')).toBeGreaterThan(labels.indexOf('Logs'))
  })

  it("n'est PAS un lien — aucun élément <a> dans le menu ne porte le texte « Quitter », aucun href", () => {
    const { container } = renderNavbar()
    openMenu()
    const dropdown = getDropdown(container)

    const quitLink = Array.from(dropdown.querySelectorAll('a')).find(a => a.textContent.includes('Quitter'))
    expect(quitLink).toBeUndefined()

    const quitButton = Array.from(dropdown.querySelectorAll('button')).find(b => b.textContent.includes('Quitter'))
    expect(quitButton).not.toBeUndefined()
    expect(quitButton).not.toHaveAttribute('href')
    expect(quitButton.tagName).toBe('BUTTON')
  })

  it('les quatre entrées de navigation existantes restent des <a href> inchangées (AC9)', () => {
    const { container } = renderNavbar()
    openMenu()
    const dropdown = getDropdown(container)

    ;['Config', 'Backup/Restaure', 'Mises à jour', 'Logs'].forEach(label => {
      const link = Array.from(dropdown.querySelectorAll('a')).find(a => a.textContent.includes(label))
      expect(link, `entrée "${label}" doit rester un <a>`).not.toBeUndefined()
      expect(link).toHaveAttribute('href')
    })
  })
})

describe('#175 — confirmation refusée (T2, AC4)', () => {
  it('window.confirm() renvoyant false : aucun fetch, le menu se referme', () => {
    vi.spyOn(window, 'confirm').mockReturnValue(false)
    global.fetch = vi.fn()
    const { container } = renderNavbar()
    openMenu()

    const quitButton = Array.from(getDropdown(container).querySelectorAll('button')).find(b => b.textContent.includes('Quitter'))
    fireEvent.click(quitButton)

    expect(window.confirm).toHaveBeenCalledTimes(1)
    expect(global.fetch).not.toHaveBeenCalled()
    expect(getDropdown(container)).toBeNull() // menu refermé même en cas d'annulation
  })
})

describe('#175 — confirmation acceptée (T3, AC5, AC7)', () => {
  it("window.confirm() renvoyant true : fetch('/shutdown') appelé une fois, menu refermé", () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    global.fetch = vi.fn().mockResolvedValue({ ok: true })
    const { container } = renderNavbar()
    openMenu()

    const quitButton = Array.from(getDropdown(container).querySelectorAll('button')).find(b => b.textContent.includes('Quitter'))
    fireEvent.click(quitButton)

    expect(global.fetch).toHaveBeenCalledTimes(1)
    expect(global.fetch).toHaveBeenCalledWith('/shutdown')
    expect(getDropdown(container)).toBeNull()
  })

  it('le message de confirmation mentionne explicitement la conséquence (AC3)', () => {
    vi.spyOn(window, 'confirm').mockReturnValue(false) // pas besoin d'aller plus loin
    const { container } = renderNavbar()
    openMenu()
    const quitButton = Array.from(getDropdown(container).querySelectorAll('button')).find(b => b.textContent.includes('Quitter'))
    fireEvent.click(quitButton)

    expect(window.confirm).toHaveBeenCalledTimes(1)
    const message = window.confirm.mock.calls[0][0]
    expect(message).toMatch(/arrêter le serveur/i)
    expect(message).toMatch(/déconnectés/i)
  })
})

describe('#175 — échec réseau silencieux (T4)', () => {
  it("un fetch('/shutdown') rejeté ne lève pas d'exception et ne déclenche aucune alerte", async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    vi.spyOn(window, 'alert').mockImplementation(() => {})
    global.fetch = vi.fn().mockRejectedValue(new TypeError('Failed to fetch'))
    const { container } = renderNavbar()
    openMenu()

    const quitButton = Array.from(getDropdown(container).querySelectorAll('button')).find(b => b.textContent.includes('Quitter'))
    expect(() => fireEvent.click(quitButton)).not.toThrow()

    // Laisse la microtask du .catch() silencieux se résoudre.
    await new Promise(resolve => setTimeout(resolve, 0))

    expect(window.alert).not.toHaveBeenCalled()
  })
})

// ---------------------------------------------------------------------------
// #179 — Navbar mesure sa propre hauteur via useElementHeightVar, écrit
// --navbar-h (plan _work/reports/plan-20260818-212304.md, tâche F3). Le
// hook lui-même a sa couverture exhaustive dans useElementHeightVar.test.js
// (T1) ; ce bloc (T2) vérifie uniquement le CÂBLAGE : Navbar l'appelle bien
// avec '--navbar-h' sur son élément racine, montage ET démontage (AC2, AC4)
// — la Navbar est démontée à chaque bascule vers une route plein écran
// (App.jsx : `{!hideNavbar && <Navbar ... />}`), le nettoyage n'est donc
// pas un détail.
// ---------------------------------------------------------------------------

describe('Navbar — mesure de hauteur --navbar-h (#179, T2)', () => {
  it('pose --navbar-h quand une mesure arrive après le montage (AC2)', () => {
    renderNavbar()
    expect(ResizeObserverMock.instances).toHaveLength(1)
    const observer = ResizeObserverMock.instances[0]

    observer.fire(80)

    expect(document.documentElement.style.getPropertyValue('--navbar-h')).toBe('80px')
  })

  it('observe son élément <nav> racine', () => {
    const { container } = renderNavbar()
    const observer = ResizeObserverMock.instances[0]
    expect(observer.observed).toHaveLength(1)
    expect(observer.observed[0]).toBe(container.querySelector('nav.navbar'))
  })

  it('remet --navbar-h à 0px au démontage (AC4 — bascule vers une route plein écran)', () => {
    const { unmount } = renderNavbar()
    const observer = ResizeObserverMock.instances[0]
    observer.fire(80)
    expect(document.documentElement.style.getPropertyValue('--navbar-h')).toBe('80px')

    unmount()

    expect(document.documentElement.style.getPropertyValue('--navbar-h')).toBe('0px')
  })
})
