import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
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

beforeEach(() => {
  global.fetch = vi.fn().mockResolvedValue({ ok: true, json: async () => ({}) })
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
