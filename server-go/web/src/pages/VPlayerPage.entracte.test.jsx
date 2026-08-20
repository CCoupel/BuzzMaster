import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, fireEvent, act } from '@testing-library/react'
import VPlayerPage from './VPlayerPage'
import { useGame } from '../hooks/GameContext'

// ---------------------------------------------------------------------------
// VPlayerPage — mode ENTRACTE (#119), tâches F5/T3/T5 du plan
// _work/reports/plan-entracte-119-20260820-140825.md.
//
// T5 : handleBuzz n'émet rien pendant l'entracte (garde déjà en place dans
// le code source au moment de l'écriture de ce test — voir
// VPlayerPage.jsx:519-528 — ce test la verrouille comme non-régression).
// T3 : absence de double filtrage — PlayerDisplay est mocké (son propre
// comportement isVPlayer est couvert par PlayerDisplay.entracte.test.jsx),
// donc UN SEUL nœud `.entracte-dim` doit apparaître dans tout l'arbre rendu :
// celui de VPlayerPage lui-même (F5).
//
// Mêmes techniques de mock que VPlayerPage.buzz-queue.test.jsx (#118, F7) :
// PlayerDisplay réduit à un stub interactif exposant onMediaClick.
// ---------------------------------------------------------------------------

const mockNavigate = vi.hoisted(() => vi.fn())

vi.mock('react-router-dom', () => ({
  useNavigate: () => mockNavigate,
}))

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
}))

vi.mock('./VPlayerPage.css', () => ({}))
vi.mock('../styles/entracte.css', () => ({}))

vi.mock('nosleep.js', () => ({
  default: class NoSleep {
    enable() { return Promise.resolve() }
    disable() {}
  },
}))

vi.mock('./PlayerDisplay', () => ({
  default: ({ onMediaClick }) => (
    <div data-testid="buzz-target" onClick={() => onMediaClick && onMediaClick()} />
  ),
}))
vi.mock('../components/ArdoiseKeyboard', () => ({ default: () => null }))

vi.mock('../components/EntractePanel', () => ({
  default: ({ config }) => <div data-testid="entracte-panel" data-title={config?.TITLE} />,
}))

function createStorageMock() {
  let store = {}
  return {
    getItem: (k) => (Object.prototype.hasOwnProperty.call(store, k) ? store[k] : null),
    setItem: (k, v) => { store[k] = String(v) },
    removeItem: (k) => { delete store[k] },
    clear: () => { store = {} },
  }
}

const BUMPER_ID = 'vjoueur_alice_1'

// Fix (dev-frontend, découvert en implémentant F5) : `...overrides` était
// spreadé APRÈS la clé `gameState` déjà fusionnée avec `...overrides.gameState`
// — l'objet `gameState` fusionné (avec ses défauts phase/question/...) était
// donc entièrement écrasé par `overrides.gameState` BRUT (non fusionné) dès
// qu'un appelant passait `{ gameState: { entracte: ... } }` sans repréciser
// `phase`. Résultat : `gameState.phase` devenait `undefined` dans le composant
// rendu, ce qui bloquait `handleBuzz` AVANT même la garde entracte — le test
// "hors entracte, l'appui envoie bien BUTTON" échouait pour une raison sans
// rapport avec #119. `...overrides` est maintenant spreadé EN PREMIER, et
// `gameState` (déjà correctement fusionné) est calculé APRÈS pour ne jamais
// être re-écrasé.
const makeGameMock = (overrides = {}) => ({
  sendMessage: vi.fn(),
  bumpers: {
    [BUMPER_ID]: { NAME: 'Alice', IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: true },
  },
  teams: {},
  status: 'connected',
  playerConnectStatus: null,
  clearPlayerConnectStatus: vi.fn(),
  playerEvictedStatus: null,
  clearPlayerEvictedStatus: vi.fn(),
  ...overrides,
  gameState: {
    phase: 'STARTED',
    question: { ID: 'q1', TYPE: 'NORMAL' },
    enrollmentActive: true,
    entracte: false,
    entracteConfig: { TITLE: 'ENTRACTE', SUBTITLE: '', IMAGE_IS_CUSTOM: false, PANEL_SIZE: 65, ANIM_PERIOD: 10, ANIM_INTENSITY: 20 },
    ...overrides.gameState,
  },
})

function pressBuzz(container) {
  fireEvent.click(container.querySelector('[data-testid="buzz-target"]'))
}

beforeEach(() => {
  vi.useFakeTimers()
  vi.stubGlobal('localStorage', createStorageMock())
  vi.stubGlobal('sessionStorage', createStorageMock())
  localStorage.setItem('vplayer_name', 'Alice')
  localStorage.setItem('vplayer_session', '1234567890')
  localStorage.setItem('vplayer_id', BUMPER_ID)
  useGame.mockReset()
})

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

describe('VPlayerPage — handleBuzz inerte pendant l\'entracte (T5)', () => {
  it("un appui pendant l'entracte n'envoie AUCUN message BUTTON", async () => {
    const mock = makeGameMock({ gameState: { entracte: true } })
    useGame.mockReturnValue(mock)
    const { container } = render(<VPlayerPage />)
    await act(async () => {})

    pressBuzz(container)

    expect(mock.sendMessage).not.toHaveBeenCalledWith('BUTTON', expect.anything())
  })

  it('hors entracte, dans les mêmes conditions, l\'appui envoie bien BUTTON (non-régression, condition de contrôle)', async () => {
    const mock = makeGameMock({ gameState: { entracte: false } })
    useGame.mockReturnValue(mock)
    const { container } = render(<VPlayerPage />)
    await act(async () => {})

    pressBuzz(container)

    expect(mock.sendMessage).toHaveBeenCalledWith('BUTTON', { ID: BUMPER_ID, button: 'A' })
  })
})

describe('VPlayerPage — filtre et panneau ENTRACTE (T3)', () => {
  it("aucun filtre .entracte-dim et aucun panneau quand entracte est false", async () => {
    const mock = makeGameMock({ gameState: { entracte: false } })
    useGame.mockReturnValue(mock)
    const { container } = render(<VPlayerPage />)
    await act(async () => {})

    expect(container.querySelectorAll('.entracte-dim')).toHaveLength(0)
    expect(container.querySelector('[data-testid="entracte-panel"]')).toBeNull()
  })

  it("exactement UN nœud .entracte-dim est présent quand entracte est true — pas de double filtrage avec PlayerDisplay (F4 conditionne le sien par !isVPlayer)", async () => {
    const mock = makeGameMock({ gameState: { entracte: true } })
    useGame.mockReturnValue(mock)
    const { container } = render(<VPlayerPage />)
    await act(async () => {})

    expect(container.querySelectorAll('.entracte-dim')).toHaveLength(1)
  })

  it('rend le panneau ENTRACTE avec la config reçue', async () => {
    const mock = makeGameMock({
      gameState: {
        entracte: true,
        entracteConfig: { TITLE: 'Pause déjeuner', SUBTITLE: '', IMAGE_IS_CUSTOM: false, PANEL_SIZE: 65, ANIM_PERIOD: 10, ANIM_INTENSITY: 20 },
      },
    })
    useGame.mockReturnValue(mock)
    const { container } = render(<VPlayerPage />)
    await act(async () => {})

    const panel = container.querySelector('[data-testid="entracte-panel"]')
    expect(panel).not.toBeNull()
    expect(panel.getAttribute('data-title')).toBe('Pause déjeuner')
  })
})
