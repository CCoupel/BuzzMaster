import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import AnimPage from './AnimPage'
import { useGame } from '../hooks/GameContext'
import { useCategories } from '../hooks/useCategories'

// ---------------------------------------------------------------------------
// #194 (v13.0.0, LOT-2.3) — AnimPage : crédit d'une manche PICTO (contrat picto.md §7, maquette M1) : « +N pts »
// pré-rempli aux points de la question pour l'équipe qui A TROUVÉ (issue FOUND) seulement, « 0 pt » pour les autres
// équipes ; pas de motif « pas de buzz » ; verrou d'attribution (#170) ; aucune ligne de crédit hors REVEALED.
// Scaffold : AnimPage.rafale.test.jsx (AnimTeamCard réel).
// ---------------------------------------------------------------------------

vi.mock('nosleep.js', () => ({
  default: class NoSleep {
    constructor() { this.isEnabled = false }
    enable() { this.isEnabled = true; return Promise.resolve() }
    disable() { this.isEnabled = false }
  },
}))

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
  GameProvider: ({ children }) => children,
}))

vi.mock('../hooks/useCategories', () => ({
  useCategories: vi.fn(),
}))

vi.mock('../components/Timer', () => ({
  default: ({ currentTime, phase }) => <div data-testid="timer" data-phase={phase}>{currentTime}</div>,
}))

// #166/F5 — mock de câblage (même patron que AnimPage.test.jsx) : capture
// les props RAFALE reçues (data-attributes + boutons) pour vérifier le
// câblage vers rafaleValidate/rafaleInvalidate, sans dépendre du rendu réel
// d'AnimConductPanel (sa propre couverture exhaustive vit dans
// AnimConductPanel.test.jsx / AnimRafaleActions.rafale.test.jsx).
// #198 (retour QUALIF 8.0.0.13) — le rendu visuel de l'encart
// question+réponse a déménagé dans AnimRafaleQuestion.jsx (zone centrale,
// montée PAR AnimConductPanel), sa propre couverture vit désormais dans
// AnimConductPanel.test.jsx. Ce mock expose donc `props.rafale` tel quel
// via data-attributes : ce fichier vérifie que AnimPage.jsx calcule et
// transmet les BONNES valeurs (câblage), pas le rendu final.
vi.mock('../components/AnimConductPanel', () => ({
  default: (props) => (
    <div
      data-testid="conduct-panel"
      data-phase={props.phase}
      data-rafale-disabled={String(!!props.rafaleDisabled)}
      data-rafale-question={props.rafale?.current?.QUESTION ?? ''}
      data-rafale-answer={props.rafale?.answerValue ?? ''}
      data-rafale-team={props.rafale?.teamName ?? ''}
      data-rafale-team-color-css={props.rafale?.teamColorCss ?? ''}
      data-rafale-cat-label={props.rafale?.catMeta?.label ?? ''}
      data-rafale-difficulty={props.rafale?.current?.DIFFICULTY ?? ''}
    >
      <button onClick={() => props.onRafaleValidate?.()}>MOCK_RAFALE_VALIDATE</button>
      <button onClick={() => props.onRafaleInvalidate?.()}>MOCK_RAFALE_INVALIDATE</button>
    </div>
  ),
}))

vi.mock('../components/AnimAnswerZone', () => ({
  default: (props) => (
    <div data-testid="answer-zone" data-question-id={props.question?.ID ?? ''} data-revealed={String(!!props.revealed)} />
  ),
}))

vi.mock('../components/AnimArdoiseList', () => ({
  default: () => <div data-testid="ardoise-list" />,
}))

// ⚠️ `...overrides` spreadé EN PREMIER — voir le piège documenté dans
// GamePage.rafaleStartGate.test.jsx : un `overrides.gameState` partiel ne
// doit jamais écraser silencieusement la clé `gameState` fusionnée juste
// après.

function makeGameMock(overrides = {}) {
  return {
    ...overrides,
    status: 'connected',
    gameState: {
      phase: 'REVEALED', timer: 0, totalTime: 90, gameTime: 0,
      question: { ID: 'pq1', TYPE: 'PICTO', CATEGORY: 'ANIMALS', POINTS: 10, TIME: 90 },
      PICTO_TEAM: 'Bleue', PICTO_TEAM_COLOR: [0, 0, 255], PICTO_DRAWER: 'b1', PICTO_DRAWER_NAME: 'Alice', PICTO_DRAWER_CONNECTED: true,
      PICTO_CANDIDATES: [], PICTO_WORDS_COUNT: 3, PICTO_CANVAS_EPOCH: 2, PICTO_OUTCOME: 'FOUND', PICTO_REVEALED_WORD: 'Maison', PICTO_REVEALED_CATEGORY: 'ANIMALS',
      ...overrides.gameState,
    },
    teams: overrides.teams ?? { Bleue: { NAME: 'Bleue', COLOR: [0, 0, 255] }, Rouge: { NAME: 'Rouge', COLOR: [255, 0, 0] } },
    bumpers: overrides.bumpers ?? { b1: { TEAM: 'Bleue', NAME: 'Alice' }, r1: { TEAM: 'Rouge', NAME: 'Rita' } },
    nextQuestion: null, questionPosition: { position: 0, total: 0 },
    awardedTeams: overrides.awardedTeams ?? {},
    creditPoints: overrides.creditPoints ?? 10,
    startGame: vi.fn(), stopGame: vi.fn(), pauseGame: vi.fn(), continueGame: vi.fn(), revealAnswer: vi.fn(), selectQuestion: vi.fn(),
    setTeamPoints: overrides.setTeamPoints ?? vi.fn(), setBumperPoints: vi.fn(), flipMemoryCard: vi.fn(),
    regieMessage: { ACTIVE: false, TEXT: '', SENT_AT: 0, CLEARED_BY: '' }, clearRegieMessage: vi.fn(),
    selectMotionCard: vi.fn(), flipMotionCard: vi.fn(), stopMotionTimer: vi.fn(), revealMotionCard: vi.fn(), doneMotionCard: vi.fn(),
    rafaleAnswer: null, rafaleValidate: vi.fn(), rafaleInvalidate: vi.fn(),
    pictoWord: null, pictoError: null, pictoCanvasStore: null,
  }
}

beforeEach(() => { useCategories.mockReturnValue({ categories: [] }) })
afterEach(() => { vi.clearAllMocks() })

const creditBlock = (container, team) => {
  const cards = Array.from(container.querySelectorAll('.anim-team-card, [class*="anim-team"]'))
  return cards.find(c => c.textContent.includes(team))
}
const buttons = (el) => Array.from(el.querySelectorAll('button')).map(b => b.textContent.trim())

describe('AnimPage — crédit PICTO (#194, §7)', () => {
  it('FOUND : « +10 pts » pré-rempli à l\'équipe qui a trouvé, « 0 pt » seul pour les autres', () => {
    useGame.mockReturnValue(makeGameMock())
    const { container } = render(<AnimPage />)
    const bleue = creditBlock(container, 'Bleue')
    const rouge = creditBlock(container, 'Rouge')
    expect(buttons(bleue)).toContain('+10 pts')
    expect(buttons(bleue)).toContain('0 pt')
    expect(buttons(rouge)).not.toContain('+10 pts')
    expect(buttons(rouge)).toContain('0 pt')
  })

  it('le crédit envoie les points de la question à l\'équipe qui a trouvé (équipe seule, jamais un joueur)', () => {
    const setTeamPoints = vi.fn()
    useGame.mockReturnValue(makeGameMock({ setTeamPoints }))
    const { container } = render(<AnimPage />)
    fireEvent.click(Array.from(creditBlock(container, 'Bleue').querySelectorAll('button')).find(b => b.textContent.trim() === '+10 pts'))
    expect(setTeamPoints).toHaveBeenCalled()
    expect(setTeamPoints.mock.calls[0][0]).toBe('Bleue')
    expect(setTeamPoints.mock.calls[0][1]).toBe(10)
  })

  it('le montant suit creditPoints (ajusté par l\'admin), pas question.POINTS brut (#170)', () => {
    useGame.mockReturnValue(makeGameMock({ creditPoints: 25 }))
    const { container } = render(<AnimPage />)
    expect(buttons(creditBlock(container, 'Bleue'))).toContain('+25 pts')
  })

  it.each(['TIMEOUT', 'STOPPED', 'NO_WORD'])('issue %s : aucune équipe n\'a « +N pts », seulement « 0 pt »', (outcome) => {
    useGame.mockReturnValue(makeGameMock({ gameState: { PICTO_OUTCOME: outcome, PICTO_REVEALED_WORD: outcome === 'NO_WORD' ? '' : 'Maison' } }))
    const { container } = render(<AnimPage />)
    for (const team of ['Bleue', 'Rouge']) {
      const b = buttons(creditBlock(container, team))
      expect(b.filter(x => /^\+\d+ pts$/.test(x)), `${team}/${outcome}`).toEqual([])
      expect(b).toContain('0 pt')
    }
  })

  it('aucun motif « pas de buzz » / « pas de réponse » sur les lignes d\'une manche PICTO (la devinette est orale)', () => {
    useGame.mockReturnValue(makeGameMock())
    const { container } = render(<AnimPage />)
    expect(container.textContent).not.toMatch(/pas de buzz|pas de réponse/)
  })

  it('verrou d\'attribution : une équipe déjà créditée n\'a plus de boutons, et seule elle est verrouillée', () => {
    useGame.mockReturnValue(makeGameMock({ awardedTeams: { Bleue: { POINTS: 10, TIMESTAMP: 1 } } }))
    const { container } = render(<AnimPage />)
    expect(buttons(creditBlock(container, 'Bleue'))).not.toContain('+10 pts')
    expect(creditBlock(container, 'Bleue').textContent).toMatch(/\+10 pts/) // libellé du geste effectué
    expect(buttons(creditBlock(container, 'Rouge'))).toContain('0 pt')
  })

  it('un refus à 0 point verrouille aussi la ligne (« 0 pt », jamais « +0 pts »)', () => {
    useGame.mockReturnValue(makeGameMock({ awardedTeams: { Rouge: { POINTS: 0, TIMESTAMP: 1 } } }))
    const { container } = render(<AnimPage />)
    expect(buttons(creditBlock(container, 'Rouge'))).toEqual([]) // plus aucune action
    expect(creditBlock(container, 'Rouge').textContent).toMatch(/0 pt/)
    expect(creditBlock(container, 'Rouge').textContent).not.toMatch(/\+0 pts/)
  })

  it('l\'équipe qui reçoit « +N pts » est celle qui DESSINE et a trouvé (PICTO_TEAM), pas l\'équipe de la 1re ligne', () => {
    useGame.mockReturnValue(makeGameMock({ gameState: { PICTO_TEAM: 'Rouge' } }))
    const { container } = render(<AnimPage />)
    expect(buttons(creditBlock(container, 'Rouge'))).toContain('+10 pts')
    expect(buttons(creditBlock(container, 'Bleue'))).not.toContain('+10 pts')
  })
})
