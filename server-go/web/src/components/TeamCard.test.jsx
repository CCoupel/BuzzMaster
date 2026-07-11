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
  connected: true,
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

  // --- Coexistence avec le badge CONNECTED ---

  it('badge ACK_PENDING et badge déconnexion coexistent quand les deux sont actifs', () => {
    renderWithBuzzer({ ackPending: true, connected: false })

    expect(screen.getByTitle('En attente de confirmation')).toBeInTheDocument()
    expect(screen.getByTitle('Buzzer déconnecté')).toBeInTheDocument()
  })

  it('badge ACK_PENDING absent quand buzzer connecté et ackPending=false', () => {
    renderWithBuzzer({ ackPending: false, connected: true })

    expect(screen.queryByTitle('En attente de confirmation')).toBeNull()
    expect(screen.queryByTitle('Buzzer déconnecté')).toBeNull()
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
// cette exclusion : seul buzzer.connected fait foi, comme pour un buzzer physique.
// Voir plan _work/reports/plan-20260711-160927.md (tâche 9).
// ---------------------------------------------------------------------------

describe('TeamCard — icône de déconnexion pour les VJoueurs (#109)', () => {

  it('badge "Buzzer déconnecté" visible pour un VJoueur déconnecté (isVPlayer=true, connected=false)', () => {
    renderWithBuzzer({ isVPlayer: true, isVirtual: true, connected: false })

    expect(screen.getByTitle('Buzzer déconnecté')).toBeInTheDocument()
  })

  it('badge "Buzzer déconnecté" absent pour un VJoueur connecté (isVPlayer=true, connected=true)', () => {
    renderWithBuzzer({ isVPlayer: true, isVirtual: true, connected: true })

    expect(screen.queryByTitle('Buzzer déconnecté')).toBeNull()
  })

  it('classe CSS "disconnected" appliquée au conteneur du VJoueur déconnecté', () => {
    const { container } = renderWithBuzzer({ isVPlayer: true, isVirtual: true, connected: false })

    const buzzerMini = container.querySelector('.buzzer-mini')
    expect(buzzerMini).not.toBeNull()
    expect(buzzerMini.className).toContain('disconnected')
  })

  it('classe CSS "disconnected" absente pour un VJoueur connecté', () => {
    const { container } = renderWithBuzzer({ isVPlayer: true, isVirtual: true, connected: true })

    const buzzerMini = container.querySelector('.buzzer-mini')
    expect(buzzerMini.className).not.toContain('disconnected')
  })

  // --- Non-régression : le comportement du buzzer physique reste inchangé ---

  it('non-régression : badge toujours visible pour un buzzer physique déconnecté (isVPlayer=false)', () => {
    renderWithBuzzer({ isVPlayer: false, isVirtual: false, connected: false })

    expect(screen.getByTitle('Buzzer déconnecté')).toBeInTheDocument()
  })

  it('non-régression : badge toujours absent pour un buzzer physique connecté', () => {
    renderWithBuzzer({ isVPlayer: false, isVirtual: false, connected: true })

    expect(screen.queryByTitle('Buzzer déconnecté')).toBeNull()
  })

  // --- Plusieurs buzzers dans la même équipe (mix physique + VJoueur) ---

  it('badge affiché uniquement sur les buzzers déconnectés (mix physique + VJoueur)', () => {
    render(
      <TeamCard
        name="Équipe Mixte"
        color={[59, 130, 246]}
        score={0}
        buzzers={[
          makeBuzzer({ mac: 'AA:BB:CC:DD:EE:01', name: 'PhysiqueConnecte', isVPlayer: false, connected: true }),
          makeBuzzer({ mac: 'vjoueur_Alice', name: 'Alice', isVPlayer: true, isVirtual: true, connected: false }),
          makeBuzzer({ mac: 'AA:BB:CC:DD:EE:02', name: 'PhysiqueDeconnecte', isVPlayer: false, connected: false }),
        ]}
      />
    )

    // Two disconnected buzzers (one physical, one VPlayer) => two badges
    const badges = screen.getAllByTitle('Buzzer déconnecté')
    expect(badges).toHaveLength(2)
  })

  // --- Non-régression : le badge ACK_PENDING reste exclu pour les VJoueurs (hors scope #109) ---

  it('non-régression : badge ACK_PENDING reste absent pour un VJoueur (comportement inchangé)', () => {
    renderWithBuzzer({ isVPlayer: true, isVirtual: true, ackPending: true })

    expect(screen.queryByTitle('En attente de confirmation')).toBeNull()
  })
})
