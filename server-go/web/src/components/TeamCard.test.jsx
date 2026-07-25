import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import TeamCard from './TeamCard'

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

// CSS import — no-op in test env
vi.mock('./TeamCard.css', () => ({}))

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/**
 * Builds a minimal buzzer object for TeamCard.buzzers prop.
 * All flags default to the "normal physical buzzer" state.
 */
const makeBuzzer = (overrides = {}) => ({
  mac: 'AA:BB:CC:DD:EE:01',
  name: 'Buzzer1',
  score: 0,
  isVPlayer: false,
  isVirtual: false,
  // connState: badge de connexion 4 états ("" | "orange" | "red" | "green").
  // Remplace l'ancien booléen `connected` (#109 / CONN_STATE — voir
  // _work/reports/planner-20260725-105503-final.md §1/§6). "" = HIDDEN =
  // connecté sans souci, équivalent de l'ancien connected=true.
  connState: '',
  ackPending: false,
  ...overrides,
})

/**
 * Renders a minimal TeamCard with one buzzer.
 */
const renderWithBuzzer = (buzzerOverrides = {}) =>
  render(
    <TeamCard
      name="Équipe Test"
      color={[239, 68, 68]}
      score={0}
      buzzers={[makeBuzzer(buzzerOverrides)]}
    />
  )

// ---------------------------------------------------------------------------
// Tests : badge ACK_PENDING (#54)
// ---------------------------------------------------------------------------

describe('TeamCard — badge ACK_PENDING (v3.8.0 #54)', () => {

  // --- Cas nominal : badge visible ---

  it('affiche le badge "En attente de confirmation" quand ackPending=true', () => {
    renderWithBuzzer({ ackPending: true })

    const badge = screen.getByTitle('En attente de confirmation')
    expect(badge).toBeInTheDocument()
  })

  it('le badge contient une icône SVG (horloge)', () => {
    renderWithBuzzer({ ackPending: true })

    const badge = screen.getByTitle('En attente de confirmation')
    // Badge is a span with an SVG child
    const svg = badge.querySelector('svg')
    expect(svg).not.toBeNull()
  })

  // --- Cas : badge absent ---

  it('badge absent quand ackPending=false', () => {
    renderWithBuzzer({ ackPending: false })

    expect(screen.queryByTitle('En attente de confirmation')).toBeNull()
  })

  it('badge absent quand ackPending est undefined (firmware pré-v3.8.0)', () => {
    renderWithBuzzer({ ackPending: undefined })

    expect(screen.queryByTitle('En attente de confirmation')).toBeNull()
  })

  // --- Cas : buzzer virtuel exclut le badge ---

  it('badge absent pour un buzzer IS_VIRTUAL même si ackPending=true', () => {
    renderWithBuzzer({ ackPending: true, isVirtual: true })

    expect(screen.queryByTitle('En attente de confirmation')).toBeNull()
  })

  // --- Cas : VPlayer exclut le badge ---

  it('badge absent pour un VJoueur (isVPlayer=true) même si ackPending=true', () => {
    renderWithBuzzer({ ackPending: true, isVPlayer: true })

    expect(screen.queryByTitle('En attente de confirmation')).toBeNull()
  })

  // --- Coexistence avec le badge de connexion (CONN_STATE) ---
  // Migré de `connected` (bool) vers `connState` (4 états) — voir plan
  // _work/reports/planner-20260725-105503-final.md §1/§6 (test-writer, batch1).

  it('badge ACK_PENDING et badge déconnexion coexistent quand les deux sont actifs', () => {
    renderWithBuzzer({ ackPending: true, connState: 'orange' })

    expect(screen.getByTitle('En attente de confirmation')).toBeInTheDocument()
    expect(screen.getByTitle('Déconnecté')).toBeInTheDocument()
  })

  it('badge ACK_PENDING absent quand buzzer connecté et ackPending=false', () => {
    renderWithBuzzer({ ackPending: false, connState: '' })

    expect(screen.queryByTitle('En attente de confirmation')).toBeNull()
    expect(screen.queryByTitle('Déconnecté')).toBeNull()
  })

  // --- Plusieurs buzzers : seul le buzzer pending a le badge ---

  it('badge uniquement sur le buzzer avec ackPending=true (plusieurs buzzers)', () => {
    render(
      <TeamCard
        name="Équipe Multi"
        color={[34, 197, 94]}
        score={0}
        buzzers={[
          makeBuzzer({ mac: 'AA:BB:CC:DD:EE:01', name: 'Normal',  ackPending: false }),
          makeBuzzer({ mac: 'AA:BB:CC:DD:EE:02', name: 'Pending', ackPending: true }),
          makeBuzzer({ mac: 'AA:BB:CC:DD:EE:03', name: 'Virtual', ackPending: true, isVirtual: true }),
        ]}
      />
    )

    // Only one badge — the physical pending buzzer
    const badges = screen.getAllByTitle('En attente de confirmation')
    expect(badges).toHaveLength(1)
  })
})

// ---------------------------------------------------------------------------
// Tests : icône de déconnexion pour les VJoueurs (#109 — commit a4716ff)
//
// Avant #109, le badge "Buzzer déconnecté" excluait volontairement les buzzers
// isVPlayer/isVirtual, masquant à tort la déconnexion d'un VJoueur. Le fix retire
// cette exclusion : seul buzzer.connected faisait foi, comme pour un buzzer physique.
// Voir plan _work/reports/plan-20260711-160927.md (tâche 9).
//
// [CHANGED — batch1 badge 4 états] `buzzer.connected` (bool) est remplacé par
// `buzzer.connState` (CONN_STATE : "" | "orange" | "red" | "green") et le
// badge ad-hoc "Buzzer déconnecté" par le composant unique <ConnectionBadge />
// (titre "Déconnecté"). Voir _work/reports/planner-20260725-105503-final.md
// §1/§6. Tests migrés en conséquence (contenu équivalent, nouveau contrat).
// ---------------------------------------------------------------------------

describe('TeamCard — icône de déconnexion pour les VJoueurs (#109)', () => {

  it('badge "Déconnecté" visible pour un VJoueur déconnecté (isVPlayer=true, connState=orange)', () => {
    renderWithBuzzer({ isVPlayer: true, isVirtual: true, connState: 'orange' })

    expect(screen.getByTitle('Déconnecté')).toBeInTheDocument()
  })

  it('badge "Déconnecté" absent pour un VJoueur connecté (isVPlayer=true, connState="")', () => {
    renderWithBuzzer({ isVPlayer: true, isVirtual: true, connState: '' })

    expect(screen.queryByTitle('Déconnecté')).toBeNull()
  })

  it('classe CSS "disconnected" appliquée au conteneur du VJoueur déconnecté (connState=orange)', () => {
    const { container } = renderWithBuzzer({ isVPlayer: true, isVirtual: true, connState: 'orange' })

    const buzzerMini = container.querySelector('.buzzer-mini')
    expect(buzzerMini).not.toBeNull()
    expect(buzzerMini.className).toContain('disconnected')
  })

  it('classe CSS "disconnected" appliquée aussi en connState=red (message perdu)', () => {
    const { container } = renderWithBuzzer({ isVPlayer: true, isVirtual: true, connState: 'red' })

    const buzzerMini = container.querySelector('.buzzer-mini')
    expect(buzzerMini.className).toContain('disconnected')
  })

  it('classe CSS "reconnecting" appliquée en connState=green (nouvel état, fenêtre de grâce)', () => {
    const { container } = renderWithBuzzer({ isVPlayer: true, isVirtual: true, connState: 'green' })

    const buzzerMini = container.querySelector('.buzzer-mini')
    expect(buzzerMini.className).toContain('reconnecting')
    expect(buzzerMini.className).not.toContain('disconnected')
  })

  it('classe CSS "disconnected" absente pour un VJoueur connecté (connState="")', () => {
    const { container } = renderWithBuzzer({ isVPlayer: true, isVirtual: true, connState: '' })

    const buzzerMini = container.querySelector('.buzzer-mini')
    expect(buzzerMini.className).not.toContain('disconnected')
    expect(buzzerMini.className).not.toContain('reconnecting')
  })

  // --- Non-régression : le comportement du buzzer physique reste inchangé ---

  it('non-régression : badge toujours visible pour un buzzer physique déconnecté (isVPlayer=false)', () => {
    renderWithBuzzer({ isVPlayer: false, isVirtual: false, connState: 'orange' })

    expect(screen.getByTitle('Déconnecté')).toBeInTheDocument()
  })

  it('non-régression : badge toujours absent pour un buzzer physique connecté', () => {
    renderWithBuzzer({ isVPlayer: false, isVirtual: false, connState: '' })

    expect(screen.queryByTitle('Déconnecté')).toBeNull()
  })

  // --- Plusieurs buzzers dans la même équipe (mix physique + VJoueur) ---

  it('badge affiché uniquement sur les buzzers déconnectés (mix physique + VJoueur)', () => {
    render(
      <TeamCard
        name="Équipe Mixte"
        color={[59, 130, 246]}
        score={0}
        buzzers={[
          makeBuzzer({ mac: 'AA:BB:CC:DD:EE:01', name: 'PhysiqueConnecte', isVPlayer: false, connState: '' }),
          makeBuzzer({ mac: 'vjoueur_Alice', name: 'Alice', isVPlayer: true, isVirtual: true, connState: 'orange' }),
          makeBuzzer({ mac: 'AA:BB:CC:DD:EE:02', name: 'PhysiqueDeconnecte', isVPlayer: false, connState: 'red' }),
        ]}
      />
    )

    // Two disconnected buzzers (one physical, one VPlayer) => two badges
    // (titres différents : "Déconnecté" pour orange, "Déconnecté — message(s) perdu(s)" pour red)
    const badges = [
      ...screen.queryAllByTitle('Déconnecté'),
      ...screen.queryAllByTitle('Déconnecté — message(s) perdu(s)'),
    ]
    expect(badges).toHaveLength(2)
  })

  // --- Non-régression : le badge ACK_PENDING reste exclu pour les VJoueurs (hors scope #109) ---

  it('non-régression : badge ACK_PENDING reste absent pour un VJoueur (comportement inchangé)', () => {
    renderWithBuzzer({ isVPlayer: true, isVirtual: true, ackPending: true })

    expect(screen.queryByTitle('En attente de confirmation')).toBeNull()
  })
})

// ---------------------------------------------------------------------------
// Tests : cohérence ConnectionBadge — même composant, mêmes titres, quel que
// soit le contexte de rendu (TeamCard vs. TeamsPage utilisent tous deux
// <ConnectionBadge />, voir plan §9 "cohérence GamePage/TeamsPage").
// ---------------------------------------------------------------------------

describe('TeamCard — cohérence du badge de connexion avec ConnectionBadge', () => {
  it('le titre du badge red rendu via TeamCard est identique à celui du composant ConnectionBadge isolé', () => {
    renderWithBuzzer({ connState: 'red' })
    // Même libellé que ConnectionBadge.test.jsx pour state="red" — garantit
    // que TeamCard ne duplique pas sa propre logique de badge.
    expect(screen.getByTitle('Déconnecté — message(s) perdu(s)')).toBeInTheDocument()
  })
})
