import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import AnimConductPanel from './AnimConductPanel'

// ---------------------------------------------------------------------------
// AnimConductPanel — zone de conduite SPEEDY, page animateur (#156, F5)
//
// Contextuel à la phase — un seul geste (ou une seule paire) visible à la
// fois, jamais un bouton "pour information". Table des phases : plan
// _work/reports/plan-20260813-094321.md §4 F5.
// ---------------------------------------------------------------------------

function baseProps(overrides = {}) {
  return {
    phase: 'STOPPED',
    isPlaying: false,
    canStart: false,
    canReveal: false,
    nextQuestion: null,
    onStart: vi.fn(),
    onPause: vi.fn(),
    onContinue: vi.fn(),
    onStop: vi.fn(),
    onReveal: vi.fn(),
    onSelectNext: vi.fn(),
    ...overrides,
  }
}

describe('AnimConductPanel — PREPARE', () => {
  it('affiche l\'attente, aucun bouton', () => {
    const { container } = render(<AnimConductPanel {...baseProps({ phase: 'PREPARE' })} />)
    expect(screen.getByText('En attente des joueurs…')).toBeInTheDocument()
    expect(container.querySelectorAll('button')).toHaveLength(0)
  })
})

describe('AnimConductPanel — STARTED/PAUSED (isPlaying)', () => {
  it('STARTED : affiche PAUSE + STOP', () => {
    const props = baseProps({ phase: 'STARTED', isPlaying: true })
    render(<AnimConductPanel {...props} />)
    expect(screen.getByText('PAUSE')).toBeInTheDocument()
    expect(screen.getByText('STOP')).toBeInTheDocument()
    expect(screen.queryByText('CONTINUER')).not.toBeInTheDocument()

    screen.getByText('PAUSE').click()
    expect(props.onPause).toHaveBeenCalledTimes(1)
    expect(props.onContinue).not.toHaveBeenCalled()

    screen.getByText('STOP').click()
    expect(props.onStop).toHaveBeenCalledTimes(1)
  })

  it('PAUSED : affiche CONTINUER + STOP', () => {
    const props = baseProps({ phase: 'PAUSED', isPlaying: true })
    render(<AnimConductPanel {...props} />)
    expect(screen.getByText('CONTINUER')).toBeInTheDocument()
    expect(screen.getByText('STOP')).toBeInTheDocument()
    expect(screen.queryByText('PAUSE')).not.toBeInTheDocument()

    screen.getByText('CONTINUER').click()
    expect(props.onContinue).toHaveBeenCalledTimes(1)
    expect(props.onPause).not.toHaveBeenCalled()
  })

  it('ne montre ni RÉPONSE ni enchaînement pendant le jeu', () => {
    render(<AnimConductPanel {...baseProps({ phase: 'STARTED', isPlaying: true, nextQuestion: { ID: '9' } })} />)
    expect(screen.queryByText('RÉPONSE')).not.toBeInTheDocument()
    expect(screen.queryByText('À suivre')).not.toBeInTheDocument()
  })
})

describe('AnimConductPanel — STOPPED après jeu (canReveal)', () => {
  it('affiche RÉPONSE seule, même avec une question suivante disponible', () => {
    const props = baseProps({ phase: 'STOPPED', canReveal: true, nextQuestion: { ID: '9' } })
    render(<AnimConductPanel {...props} />)
    expect(screen.getByText('RÉPONSE')).toBeInTheDocument()
    expect(screen.queryByText('À suivre')).not.toBeInTheDocument()

    screen.getByText('RÉPONSE').click()
    expect(props.onReveal).toHaveBeenCalledTimes(1)
  })
})

describe('AnimConductPanel — READY', () => {
  it('affiche LANCER + enchaînement quand une question suivante est disponible', () => {
    const props = baseProps({ phase: 'READY', canStart: true, nextQuestion: { ID: '7' } })
    render(<AnimConductPanel {...props} />)
    expect(screen.getByText('LANCER')).toBeInTheDocument()
    expect(screen.getByText('À suivre')).toBeInTheDocument()
    expect(screen.getByText('#7')).toBeInTheDocument()

    screen.getByText('LANCER').click()
    expect(props.onStart).toHaveBeenCalledTimes(1)

    screen.getByText('À suivre').click()
    expect(props.onSelectNext).toHaveBeenCalledWith('7')
  })

  it('affiche LANCER seul quand aucune question suivante n\'est connue', () => {
    render(<AnimConductPanel {...baseProps({ phase: 'READY', canStart: true, nextQuestion: null })} />)
    expect(screen.getByText('LANCER')).toBeInTheDocument()
    expect(screen.queryByText('À suivre')).not.toBeInTheDocument()
  })
})

describe('AnimConductPanel — STOPPED idle / NEW_GAME / REVEALED (enchaînement)', () => {
  it('STOPPED idle (canReveal=false) : affiche l\'enchaînement, pas RÉPONSE', () => {
    render(<AnimConductPanel {...baseProps({ phase: 'STOPPED', canReveal: false, nextQuestion: { ID: '3' } })} />)
    expect(screen.getByText('À suivre')).toBeInTheDocument()
    expect(screen.queryByText('RÉPONSE')).not.toBeInTheDocument()
  })

  it('NEW_GAME : affiche l\'enchaînement', () => {
    render(<AnimConductPanel {...baseProps({ phase: 'NEW_GAME', nextQuestion: { ID: '1' } })} />)
    expect(screen.getByText('À suivre')).toBeInTheDocument()
  })

  it('REVEALED : affiche l\'enchaînement (la réponse elle-même est affichée en zone A)', () => {
    render(<AnimConductPanel {...baseProps({ phase: 'REVEALED', nextQuestion: { ID: '2' } })} />)
    expect(screen.getByText('À suivre')).toBeInTheDocument()
  })

  it('aucune question suivante, pas LANCER non plus : message "aucune question disponible"', () => {
    render(<AnimConductPanel {...baseProps({ phase: 'REVEALED', nextQuestion: null })} />)
    expect(screen.getByText('Aucune question disponible')).toBeInTheDocument()
  })
})
