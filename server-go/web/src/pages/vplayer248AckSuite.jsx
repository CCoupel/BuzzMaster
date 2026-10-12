// Suite commune des tests de spécification #248 (VPlayerPage, accusé de réception) — pas un test seul :
// importée par VPlayerPage.qcm-ack.248.test.jsx et VPlayerPage.speedy-ack.248.test.jsx.
// Source : maquette VALIDÉE docs/mockup/v11.2.1/ui/vplayer_qcm_answer__ack.html et docs/mockup/DECISIONS.md
// (vplayer_qcm_answer). Comportement observable : messages envoyés (nombre, contenu), texte affiché, cadence
// (horloge simulée). Aucune dépendance au mécanisme interne (hook, refs).
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, act, fireEvent } from '@testing-library/react'
import VPlayerPage from './VPlayerPage'
import { useGame } from '../hooks/GameContext'
import { createStorage, seedSession, makeGame, BOUND, PAUL, sent } from './vplayer246Helpers'

export const FLOWS = {
  qcm: {
    label: 'réponse QCM',
    question: (id) => ({ ID: id, TYPE: 'QCM', QCM_ANSWERS: { RED: 'A', GREEN: 'B', YELLOW: 'C', BLUE: 'D' } }),
    action: 'VPLAYER_QCM_ANSWER',
    payload: { ID: PAUL, ANSWER_COLOR: 'GREEN' },
    press: 'qcm-GREEN',
    otherPress: 'qcm-BLUE',
    pending: /Envoi de votre réponse…/,
    resend: /Renvoi de votre réponse…/,
    ack: /✓ Réponse reçue/,
    lost: /Connexion perdue — votre réponse sera envoyée dès le retour du réseau/,
    paused: /Jeu en pause/,
    pausedPending: /envoi de votre réponse…/,
    fail: /✕ Réponse non reçue — la question est terminée/,
    team: /ℹ Votre équipe a déjà répondu/,
  },
  speedy: {
    label: 'buzz SPEEDY',
    question: (id) => ({ ID: id, TYPE: 'SPEEDY' }),
    action: 'BUTTON',
    payload: { ID: PAUL, button: 'A' },
    press: 'buzz',
    otherPress: 'buzz',
    pending: /Envoi de votre buzz…/,
    resend: /Renvoi de votre buzz…/,
    ack: /✓ Buzz reçu/,
    lost: /Connexion perdue — votre buzz sera envoyé dès le retour du réseau/,
    paused: /Jeu en pause/,
    pausedPending: /envoi de votre buzz…/,
    fail: /✕ Buzz non reçu — la question est terminée/,
    team: /ℹ Votre équipe a déjà buzzé/,
  },
}

const mockNavigate = vi.hoisted(() => vi.fn())
vi.mock('react-router-dom', () => ({ useNavigate: () => mockNavigate }))
vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn() }))
vi.mock('./VPlayerPage.css', () => ({}))
vi.mock('nosleep.js', () => ({ default: class { enable() { return Promise.resolve() } disable() {} } }))
// Doublure interactive : un élément cliquable par geste du joueur (buzz, 4 couleurs QCM).
vi.mock('./PlayerDisplay', () => ({
  default: ({ onMediaClick, onQCMAnswer }) => (
    <div>
      <button data-testid="buzz" onClick={() => onMediaClick && onMediaClick()} />
      {['RED', 'GREEN', 'YELLOW', 'BLUE'].map((c) => (
        <button key={c} data-testid={`qcm-${c}`} onClick={() => onQCMAnswer && onQCMAnswer(c)} />
      ))}
    </div>
  ),
}))

const text = (c) => c.textContent.replace(/\s+/g, ' ')
const CADENCE_MS = 4000
const CAP = 30

export function defineAckSuite(flowName) {
  const F = FLOWS[flowName]

  describe(`VPlayerPage — ${F.label} avec accusé du serveur (#248)`, () => {
    let ls
    beforeEach(() => {
      vi.useFakeTimers()
      ls = createStorage()
      vi.stubGlobal('localStorage', ls)
      vi.stubGlobal('sessionStorage', createStorage())
      useGame.mockReset()
      seedSession(ls)
    })
    afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals() })

    // État piloté par le test ; le spy `send` est conservé d'un rendu à l'autre.
    function mount() {
      const send = vi.fn(() => true)
      const st = { phase: 'STARTED', status: 'connected', qid: 'q1', ownTime: 0, teamTime: 0, bumperConnected: true }
      let cur = null, key = ''
      useGame.mockImplementation(() => {
        const k = JSON.stringify(st)
        if (k !== key) {
          cur = makeGame(send, {
            status: st.status,
            playerConnectStatus: BOUND(),
            gameState: { phase: st.phase, timer: 30, question: F.question(st.qid) },
            bumpers: { [PAUL]: { NAME: 'Paul', IS_VIRTUAL: true, IS_VPLAYER: true, TEAM: 'Les Bleus', CONNECTED: st.bumperConnected, TIME: st.ownTime } },
          })
          cur.teams = { 'Les Bleus': { NAME: 'Les Bleus', COLOR: [0, 0, 255], TIME: st.teamTime } }
          key = k
        }
        return cur
      })
      const view = render(<VPlayerPage />)
      const apply = async (patch) => { Object.assign(st, patch); await act(async () => { view.rerender(<VPlayerPage />) }) }
      const advance = async (ms) => { await act(async () => { vi.advanceTimersByTime(ms) }) }
      const press = async (id = F.press) => { await act(async () => { fireEvent.click(view.container.querySelector(`[data-testid="${id}"]`)) }) }
      const count = () => sent(send, F.action).length
      return { send, view, apply, advance, press, count, text: () => text(view.container) }
    }

    async function mountBound() {
      const m = mount()
      await m.advance(100) // liaison établie (PLAYER_CONNECTED)
      return m
    }

    it('Critical — appui : un seul envoi au bon format, « envoi… » affiché, ni « ✓ » ni « reçu » avant l\'accusé', async () => {
      const m = await mountBound()
      await m.press()
      expect(sent(m.send, F.action)).toEqual([F.payload])
      expect(m.text()).toMatch(F.pending)
      expect(m.text()).not.toMatch(F.ack)
      expect(m.text()).not.toMatch(/✓/)
    })

    it('Critical — accusé (TIME > 0 sur son propre bumper) : « ✓ … reçu », plus de message d\'attente, plus aucun renvoi', async () => {
      const m = await mountBound()
      await m.press()
      await m.apply({ ownTime: 1234567 })
      expect(m.text()).toMatch(F.ack)
      expect(m.text()).not.toMatch(F.pending)
      await m.advance(CADENCE_MS * 10)
      expect(m.count()).toBe(1)
    })

    it('un seul appui en attente : un second geste (autre choix) pendant l\'attente n\'envoie rien de plus', async () => {
      const m = await mountBound()
      await m.press()
      await m.press(F.otherPress)
      expect(m.count()).toBe(1)
      expect(sent(m.send, F.action)).toEqual([F.payload])
    })

    it('sans accusé, renvoi silencieux toutes les 4 s (rien avant 4 s), même contenu, message d\'attente inchangé', async () => {
      const m = await mountBound()
      await m.press()
      await m.advance(CADENCE_MS - 100)
      expect(m.count()).toBe(1)
      await m.advance(200)
      expect(m.count()).toBe(2)
      await m.advance(CADENCE_MS)
      expect(m.count()).toBe(3)
      expect(sent(m.send, F.action).every((p) => JSON.stringify(p) === JSON.stringify(F.payload))).toBe(true)
      expect(m.text()).toMatch(F.pending)
    })

    it('plafond : 30 envois au total puis échec définitif, plus aucun envoi ensuite', async () => {
      const m = await mountBound()
      await m.press()
      await m.advance(CADENCE_MS * (CAP + 10))
      expect(m.count()).toBe(CAP)
      expect(m.text()).toMatch(F.fail)
      await m.advance(CADENCE_MS * 5)
      expect(m.count()).toBe(CAP)
    })

    it('coupure avant l\'accusé : message « connexion perdue », aucun envoi pendant la coupure, renvoi « Renvoi… » à la reliaison', async () => {
      const m = await mountBound()
      await m.press()
      await m.apply({ status: 'disconnected' })
      expect(m.text()).toMatch(F.lost)
      const before = m.count()
      await m.advance(CADENCE_MS * 5)
      expect(m.count()).toBe(before)
      await m.apply({ status: 'connected' })
      await m.advance(100)
      expect(m.count()).toBe(before + 1)
      expect(m.text()).toMatch(F.resend)
      expect(m.text()).not.toMatch(F.ack)
    })

    it('appui pendant la coupure : message « connexion perdue », envoyé une seule fois à la reliaison', async () => {
      const m = await mountBound()
      await m.apply({ status: 'disconnected' })
      await m.press()
      expect(m.count()).toBe(0)
      expect(m.text()).toMatch(F.lost)
      await m.apply({ status: 'connected' })
      await m.advance(100)
      expect(m.count()).toBe(1)
    })

    it('retour au premier plan (visibilitychange) sans accusé : renvoi immédiat', async () => {
      const m = await mountBound()
      await m.press()
      Object.defineProperty(document, 'visibilityState', { value: 'visible', configurable: true })
      await act(async () => { document.dispatchEvent(new Event('visibilitychange')) })
      expect(m.count()).toBe(2)
    })

    it('PAUSE avant l\'accusé : « Jeu en pause · envoi… », aucun envoi pendant la pause, renvoi au retour en STARTED', async () => {
      const m = await mountBound()
      await m.press()
      await m.apply({ phase: 'PAUSED' })
      expect(m.text()).toMatch(F.paused)
      expect(m.text()).toMatch(F.pausedPending)
      const before = m.count()
      await m.advance(CADENCE_MS * 5)
      expect(m.count()).toBe(before)
      await m.apply({ phase: 'STARTED' })
      await m.advance(100)
      expect(m.count()).toBe(before + 1)
    })

    it('question terminée sans accusé (REVEALED) : échec définitif annoncé, plus aucun envoi', async () => {
      const m = await mountBound()
      await m.press()
      await m.apply({ phase: 'REVEALED' })
      expect(m.text()).toMatch(F.fail)
      const before = m.count()
      await m.advance(CADENCE_MS * 5)
      expect(m.count()).toBe(before)
    })

    it('question changée sans accusé : échec définitif, jamais de renvoi vers la nouvelle question', async () => {
      const m = await mountBound()
      await m.press()
      await m.apply({ qid: 'q2' })
      expect(m.text()).toMatch(F.fail)
      const before = m.count()
      await m.advance(CADENCE_MS * 5)
      expect(m.count()).toBe(before)
    })

    it('Critical — équipe déjà servie (un coéquipier a un TIME, pas ce joueur) : « ℹ … équipe déjà … », renvois arrêtés, pas une erreur', async () => {
      const m = await mountBound()
      await m.press()
      await m.apply({ teamTime: 999 })
      expect(m.text()).toMatch(F.team)
      expect(m.text()).not.toMatch(F.fail)
      const before = m.count()
      await m.advance(CADENCE_MS * 10)
      expect(m.count()).toBe(before)
    })

    it('Critical (défaut majeur de revue) — équipe DÉJÀ servie au moment de l\'appui : « ℹ équipe déjà … », jamais le faux « non reçue », même après 30 intervalles', async () => {
      const m = await mountBound()
      await m.apply({ teamTime: 999 }) // un coéquipier a répondu / buzzé avant ce joueur
      await m.press()
      await m.advance(CADENCE_MS * (CAP + 5))
      expect(m.text()).toMatch(F.team)
      expect(m.text()).not.toMatch(F.fail)
      expect(m.count()).toBeLessThanOrEqual(1)
    })

    it('liaison confirmée mais bumper.CONNECTED = false (drapeau diffusé en retard) : le message part quand même (garde d\'envoi = liaison)', async () => {
      const m = mount()
      await m.apply({ bumperConnected: false })
      await m.advance(100)
      await m.press()
      expect(sent(m.send, F.action)).toEqual([F.payload])
    })

    it('échec définitif : aucune invitation à réessayer ni à rejouer', async () => {
      const m = await mountBound()
      await m.press()
      await m.apply({ phase: 'REVEALED' })
      expect(m.text()).toMatch(F.fail)
      expect(m.text()).not.toMatch(/réessay|rejou|recommenc|renvoy|essayez/i)
    })

    it('la pastille disparaît à la question suivante (PREPARE) : plus aucun message d\'état', async () => {
      const m = await mountBound()
      await m.press()
      await m.apply({ phase: 'REVEALED' })
      await m.apply({ phase: 'PREPARE', qid: 'q2' })
      expect(m.text()).not.toMatch(F.fail)
      expect(m.text()).not.toMatch(F.pending)
    })
  })
}
