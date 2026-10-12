import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, act } from '@testing-library/react'
import VPlayerPage from './VPlayerPage'
import { useGame } from '../hooks/GameContext'
import { createStorage, seedSession, makeGame, BOUND, PAUL } from './vplayer246Helpers'

// #248 lot 5b — retournement de carte MEMORY / MEMOTION avec accusé du serveur (FLIP_ID, FLIP_MEMORY_CARD_ACK).
// Sources : maquette VALIDÉE docs/mockup/v11.2.1/ui/vplayer_qcm_answer__ack.html (flux MEMORY), DECISIONS.md
// (vplayer_qcm_answer), contrat websocket-actions.md §FLIP_MEMORY_CARD (FLIP_ID) / §FLIP_MEMORY_CARD_ACK.
// Vrai PlayerDisplay dans VPlayerPage : on touche une vraie carte. Observable côté réseau = tout ce qui part vers
// le serveur en FLIP_MEMORY_CARD, que ce soit via flipMemoryCard(cardId, motionCardId, playerId, flipId) ou via
// sendMessage('FLIP_MEMORY_CARD', payload) (renvois du hook) : les deux sont ramenés à un même payload.
// Accusé : le contexte expose flipMemoryCardAck = { flipId, cardId, motionCardId, status, reason, duplicate }
// (nom choisi par useWebSocket.flipAck.248.test.js). Pastilles : mêmes textes que la maquette.

const mockNavigate = vi.hoisted(() => vi.fn())
vi.mock('react-router-dom', () => ({ useNavigate: () => mockNavigate }))
vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn(), GameProvider: ({ children }) => children }))
vi.mock('nosleep.js', () => ({ default: class { enable() { return Promise.resolve() } disable() {} } }))
vi.mock('canvas-confetti', () => ({ default: vi.fn() }))
vi.mock('../components/Timer', () => ({ default: () => <div data-testid="timer" /> }))
vi.mock('../components/Podium', () => ({ default: () => <div data-testid="podium" /> }))
vi.mock('../components/QRCodeOverlay', () => ({ default: () => null }))
vi.mock('../components/QRCodeDisplay', () => ({ default: () => null }))
vi.mock('./QuestionsPage', () => ({ CATEGORIES: [] }))
vi.mock('../constants/colors', () => ({ getCategoryColor: vi.fn(() => '#8b5cf6') }))
vi.mock('../utils/colorUtils', () => ({ getRgbColor: vi.fn((c) => (Array.isArray(c) ? `rgb(${c.join(',')})` : c)) }))
vi.mock('./PlayerDisplay.css', () => ({}))
vi.mock('./VPlayerPage.css', () => ({}))
vi.mock('../styles/neon.css', () => ({}))
vi.mock('../components/TvPointsAward.css', () => ({}))

const text = (c) => c.textContent.replace(/\s+/g, ' ')
const CADENCE = 4000
const CAP = 30
const FLIP_ID_RE = /^[A-Za-z0-9_-]{1,64}$/
const PAIRS = [
  { ID: 1, CARD1: { TEXT: 'Napoléon' }, CARD2: { TEXT: '1804' } },
  { ID: 2, CARD1: { TEXT: 'De Gaulle' }, CARD2: { TEXT: '1958' } },
]
const MOTION_CARD = { ID: 'card-3', TYPE: 'MEMORY', RECTO_THEME: 'Histoire', DIFFICULTY: 1, MEMORY_PAIRS: PAIRS }

const HOSTS = {
  MEMORY: {
    label: 'MEMORY (question)',
    motionCardId: undefined,
    gameState: (st) => ({
      phase: st.phase,
      question: { ID: st.qid, TYPE: 'MEMORY', MEMORY_PAIRS: PAIRS, MEMORY_CONFIG: {} },
      MEMORY_CURRENT_TEAM: st.mainTeam, memoryFlippedCards: [], memoryMatchedPairs: [],
    }),
    cards: (c) => [...c.querySelectorAll('.memory-card:not(.flipped)')],
  },
  MEMOTION: {
    label: 'carte MEMOTION de type MEMORY',
    motionCardId: 'card-3',
    gameState: (st) => ({
      phase: st.phase,
      question: { ID: st.qid, TYPE: 'MEMOTION', MOTION_CARDS: [MOTION_CARD], MOTION_CONFIG: { POINTS_1_STAR: 1, POINTS_2_STAR: 3, POINTS_3_STAR: 5 } },
      MEMOTION_SUBPHASE: 'QUESTION', MEMOTION_SELECTED: 'card-3', MEMOTION_CARD_STATES: { 'card-3': 'ACTIVE' }, MEMOTION_CARD_TEAMS: {},
      MEMOTION_CURRENT_TEAM: st.mainTeam, MEMOTION_CURRENT_TEAM_COLOR: [0, 0, 255], MEMOTION_PARTICIPATING_TEAMS: ['Les Bleus', 'Les Rouges'],
      MEMOTION_ACTIVE: { CARD_ID: 'card-3', TYPE: 'MEMORY', STATE: { MEMORY_FLIPPED_CARDS: [], MEMORY_MATCHED_PAIRS: [], MEMORY_ERRORS: 0 } },
    }),
    cards: (c) => [...c.querySelectorAll('.memotion-tv-memory-card:not(.up)')],
  },
}

const MSG = {
  pending: /Retournement de la carte…/,
  lost: /Connexion perdue — la carte sera retournée dès le retour du réseau/,
  resend: /Renvoi du retournement…/,
  paused: /Jeu en pause/,
  pausedPending: /retournement en attente…/,
  failAny: /✕ Carte non retournée/,
  notTurn: /✕ Carte non retournée — ce n.est plus votre tour/,
}

describe.each(Object.keys(HOSTS))('VPlayerPage — retournement %s avec accusé (#248 lot 5b)', (hostKey) => {
  const H = HOSTS[hostKey]
  let ls
  beforeEach(() => {
    vi.useFakeTimers()
    ls = createStorage()
    vi.stubGlobal('localStorage', ls)
    vi.stubGlobal('sessionStorage', createStorage())
    document.documentElement.requestFullscreen = vi.fn().mockResolvedValue(undefined)
    useGame.mockReset()
    seedSession(ls)
  })
  afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals() })

  function mount(over = {}) {
    const sendMessage = vi.fn(() => true)
    const flipMemoryCard = vi.fn()
    const st = { phase: 'STARTED', status: 'connected', qid: 'qm1', mainTeam: 'Les Bleus', ack: null, ...over }
    let cur = null, key = ''
    useGame.mockImplementation(() => {
      const k = JSON.stringify(st)
      if (k !== key) {
        cur = makeGame(sendMessage, {
          status: st.status, playerConnectStatus: BOUND(),
          gameState: { ...H.gameState(st), timer: 30, totalTime: 30, remote: 'GAME', MEMORY_PARTICIPATING_TEAMS: [], newGameBackgrounds: [], SCORE_CHANGE: { SEQ: 0, DISPLAY: true, TEAMS: [], PLAYERS: [], GAINS: [] } },
        })
        cur.teams = { 'Les Bleus': { NAME: 'Les Bleus', COLOR: [0, 0, 255], SCORE: 0 }, 'Les Rouges': { NAME: 'Les Rouges', COLOR: [255, 0, 0], SCORE: 0 } }
        cur.flipMemoryCard = flipMemoryCard
        cur.flipMemoryCardAck = st.ack
        cur.showQRCode = false
        cur.selectMotionCard = vi.fn()
        cur.sendClientTrace = vi.fn()
        key = k
      }
      return cur
    })
    const view = render(<VPlayerPage />)
    const apply = async (patch) => { Object.assign(st, patch); await act(async () => { view.rerender(<VPlayerPage />) }) }
    const advance = async (ms) => { await act(async () => { vi.advanceTimersByTime(ms) }) }
    // tous les FLIP_MEMORY_CARD partis vers le serveur, ramenés à un payload
    const flips = () => {
      const viaHook = flipMemoryCard.mock.calls.map(([cardId, motionCardId, playerId, flipId]) => {
        const p = { CARD_ID: cardId }
        if (motionCardId) p.MOTION_CARD_ID = motionCardId
        if (playerId) p.ID = playerId
        if (flipId) p.FLIP_ID = flipId
        return p
      })
      const viaSend = sendMessage.mock.calls.filter((c) => c[0] === 'FLIP_MEMORY_CARD').map((c) => c[1])
      return [...viaHook, ...viaSend]
    }
    const tap = async (i = 0) => { await act(async () => { H.cards(view.container)[i].click() }) }
    return { view, apply, advance, flips, tap, text: () => text(view.container), st }
  }
  async function mountBound(over) { const m = mount(over); await m.advance(100); return m }
  const ackOf = (flip, status = 'APPLIED', reason = '', duplicate = false) => ({
    flipId: flip.FLIP_ID, cardId: flip.CARD_ID, motionCardId: flip.MOTION_CARD_ID || '', status, reason, duplicate,
  })

  it('Critical — équipe qui a la main : un seul envoi, avec ID, FLIP_ID au bon format (et MOTION_CARD_ID pour la carte MEMOTION), pastille d\'attente', async () => {
    const m = await mountBound()
    await m.tap()
    const f = m.flips()
    expect(f).toHaveLength(1) // pas de double retournement local (ancien chemin + hook)
    expect(f[0].ID).toBe(PAUL)
    expect(f[0].FLIP_ID, 'le geste doit porter un FLIP_ID').toBeTruthy()
    expect(f[0].FLIP_ID).toMatch(FLIP_ID_RE)
    if (H.motionCardId) expect(f[0].MOTION_CARD_ID).toBe(H.motionCardId)
    expect(m.text()).toMatch(MSG.pending)
    expect(m.text()).not.toMatch(/✓/)
  })

  it('Critical — accusé APPLIED : plus de pastille, plus aucun renvoi (la carte suit l\'état reçu)', async () => {
    const m = await mountBound()
    await m.tap()
    const first = m.flips()[0]
    await m.apply({ ack: ackOf(first) })
    expect(m.text()).not.toMatch(MSG.pending)
    expect(m.text()).not.toMatch(MSG.failAny)
    await m.advance(CADENCE * 10)
    expect(m.flips()).toHaveLength(1)
  })

  it('accusé APPLIED + DUPLICATE (renvoi dédupliqué) : traité comme un accusé, aucun faux échec', async () => {
    const m = await mountBound()
    await m.tap()
    await m.apply({ ack: ackOf(m.flips()[0], 'APPLIED', '', true) })
    expect(m.text()).not.toMatch(MSG.failAny)
    await m.advance(CADENCE * 10)
    expect(m.flips()).toHaveLength(1)
  })

  it('un accusé pour un AUTRE FLIP_ID ne clôt pas le geste en cours', async () => {
    const m = await mountBound()
    await m.tap()
    await m.apply({ ack: { flipId: 'un-autre-geste', cardId: '9-9', motionCardId: '', status: 'APPLIED', reason: '', duplicate: false } })
    expect(m.text()).toMatch(MSG.pending)
    await m.advance(CADENCE + 100)
    expect(m.flips().length).toBeGreaterThan(1)
  })

  it('Critical — sans accusé : renvoi toutes les 4 s avec le MÊME FLIP_ID', async () => {
    const m = await mountBound()
    await m.tap()
    await m.advance(CADENCE - 100)
    expect(m.flips()).toHaveLength(1)
    await m.advance(200)
    await m.advance(CADENCE)
    const f = m.flips()
    expect(f).toHaveLength(3)
    expect(new Set(f.map((x) => x.FLIP_ID)).size).toBe(1)
    expect(f.every((x) => x.CARD_ID === f[0].CARD_ID)).toBe(true)
  })

  it('coupure : « connexion perdue — la carte sera retournée… », aucun envoi, puis renvoi du même FLIP_ID à la reliaison', async () => {
    const m = await mountBound()
    await m.tap()
    const id = m.flips()[0].FLIP_ID
    await m.apply({ status: 'disconnected' })
    expect(m.text()).toMatch(MSG.lost)
    const before = m.flips().length
    await m.advance(CADENCE * 5)
    expect(m.flips().length).toBe(before)
    await m.apply({ status: 'connected' })
    await m.advance(100)
    const f = m.flips()
    expect(f.length).toBe(before + 1)
    expect(f[f.length - 1].FLIP_ID).toBe(id)
    expect(m.text()).toMatch(MSG.resend)
  })

  it('PAUSE avant l\'accusé : « Jeu en pause · retournement en attente… », aucun envoi, renvoi à la reprise', async () => {
    const m = await mountBound()
    await m.tap()
    await m.apply({ phase: 'PAUSED' })
    expect(m.text()).toMatch(MSG.paused)
    expect(m.text()).toMatch(MSG.pausedPending)
    const before = m.flips().length
    await m.advance(CADENCE * 5)
    expect(m.flips().length).toBe(before)
    await m.apply({ phase: 'STARTED' })
    await m.advance(100)
    expect(m.flips().length).toBe(before + 1)
  })

  it('plafond : 30 envois au total puis « ✕ Carte non retournée », plus aucun envoi', async () => {
    const m = await mountBound()
    await m.tap()
    await m.advance(CADENCE * (CAP + 10))
    expect(m.flips()).toHaveLength(CAP)
    expect(m.text()).toMatch(MSG.failAny)
    await m.advance(CADENCE * 5)
    expect(m.flips()).toHaveLength(CAP)
  })

  it('Critical — IGNORED + NOT_TURN : « ✕ Carte non retournée — ce n\'est plus votre tour », renvois arrêtés', async () => {
    const m = await mountBound()
    await m.tap()
    await m.apply({ ack: ackOf(m.flips()[0], 'IGNORED', 'NOT_TURN') })
    expect(m.text()).toMatch(MSG.notTurn)
    const n = m.flips().length
    await m.advance(CADENCE * 10)
    expect(m.flips().length).toBe(n)
  })

  it('IGNORED (autre motif, ex. NOT_FLIPPABLE) : « ✕ Carte non retournée » sans « plus votre tour », renvois arrêtés', async () => {
    const m = await mountBound()
    await m.tap()
    await m.apply({ ack: ackOf(m.flips()[0], 'IGNORED', 'NOT_FLIPPABLE') })
    expect(m.text()).toMatch(MSG.failAny)
    expect(m.text()).not.toMatch(/plus votre tour/)
    const n = m.flips().length
    await m.advance(CADENCE * 10)
    expect(m.flips().length).toBe(n)
  })

  it('échec définitif : aucune invitation à réessayer', async () => {
    const m = await mountBound()
    await m.tap()
    await m.apply({ ack: ackOf(m.flips()[0], 'IGNORED', 'NOT_TURN') })
    expect(m.text()).toMatch(MSG.failAny)
    expect(m.text()).not.toMatch(/réessay|rejou|recommenc|essayez/i)
  })

  it('grille verrouillée pendant l\'attente : un second tap sur une autre carte n\'envoie rien', async () => {
    const m = await mountBound()
    await m.tap(0)
    await m.tap(1)
    expect(m.flips()).toHaveLength(1)
  })

  it('Critical — geste suivant après l\'accusé : un NOUVEAU FLIP_ID', async () => {
    const m = await mountBound()
    await m.tap(0)
    const first = m.flips()[0]
    await m.apply({ ack: ackOf(first) })
    await m.apply({ ack: null })
    await m.tap(1)
    const f = m.flips()
    expect(f).toHaveLength(2)
    expect(f[1].FLIP_ID, 'le 2e geste doit porter un FLIP_ID').toBeTruthy()
    expect(f[1].FLIP_ID).toMatch(FLIP_ID_RE)
    expect(f[1].FLIP_ID).not.toBe(first.FLIP_ID)
  })

  it('question changée sans accusé : échec définitif, plus aucun renvoi vers la nouvelle question', async () => {
    const m = await mountBound()
    await m.tap()
    await m.apply({ qid: 'qm2' })
    expect(m.text()).toMatch(MSG.failAny)
    const n = m.flips().length
    await m.advance(CADENCE * 5)
    expect(m.flips().length).toBe(n)
  })

  it('phase terminale sans accusé (REVEALED) : échec définitif, plus aucun renvoi ; PREPARE purge la pastille', async () => {
    const m = await mountBound()
    await m.tap()
    await m.apply({ phase: 'REVEALED' })
    expect(m.text()).toMatch(MSG.failAny)
    const n = m.flips().length
    await m.advance(CADENCE * 5)
    expect(m.flips().length).toBe(n)
    await m.apply({ phase: 'PREPARE', qid: 'qm3' })
    expect(m.text()).not.toMatch(MSG.failAny)
    expect(m.text()).not.toMatch(MSG.pending)
  })

  it('Critical — hors tour (dérogation du 2026-08-24) : envoi SANS FLIP_ID, aucune pastille', async () => {
    const m = await mountBound({ mainTeam: 'Les Rouges' })
    await m.tap()
    const f = m.flips()
    expect(f).toHaveLength(1) // le geste part toujours (le serveur l'ignore en silence)
    expect('FLIP_ID' in f[0]).toBe(false)
    expect(m.text()).not.toMatch(MSG.pending)
    expect(m.text()).not.toMatch(MSG.failAny)
    await m.advance(CADENCE * 5)
    expect(m.flips()).toHaveLength(1) // jamais de renvoi
  })
})
