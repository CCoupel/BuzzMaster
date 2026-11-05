/**
 * #282 (QUALIF 11.4.0.2) — bug « clic RÉPONSE VALIDE / INVALIDE sans effet » (cause racine : handoff
 * _work/handoff/dev-backend-20261105-151500.md). AnimRafaleActions fait `onClick={onValidate}` : React
 * passe l'événement synthétique comme `motionCardId` à rafaleValidate(motionCardId) (#217) ⇒ MOTION_CARD_ID
 * = événement ⇒ JSON.stringify lève (structure circulaire) ⇒ aucune trame. Ici AnimRafaleActions est câblé
 * sur le VRAI hook useWebSocket (WebSocket mocké), comme /anim (AnimPage → AnimConductPanel) : le clic doit
 * produire RAFALE_VALIDATE / RAFALE_INVALIDATE avec MSG sans MOTION_CARD_ID, sans exception.
 * Carte MEMOTION : MOTION_CARD_ID doit, lui, être transmis (wrapper onCardRafaleValidate d'AnimPage).
 */
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, fireEvent, act } from '@testing-library/react'
import useWebSocket from '../hooks/useWebSocket'
import AnimRafaleActions from './AnimRafaleActions'

// WebSocket mocké : le VRAI hook useWebSocket (rafaleValidate/rafaleInvalidate) envoie sur cette socket.
let sent = []
class MockWebSocket {
  constructor() { this.readyState = 0; MockWebSocket.last = this; MockWebSocket.all.push(this) }
  send(data) { sent.push(JSON.parse(data)) }
  close() { this.readyState = 3; this.onclose && this.onclose() }
}
MockWebSocket.all = []
MockWebSocket.CONNECTING = 0; MockWebSocket.OPEN = 1; MockWebSocket.CLOSING = 2; MockWebSocket.CLOSED = 3
const openSocket = async () => {
  await act(async () => { for (const w of MockWebSocket.all) { if (w.readyState === 3) continue; w.readyState = 1; w.onopen && w.onopen() } })
  sent = []
}
const frames = (action) => sent.filter((f) => f.ACTION === action)

// Câblage /anim hors carte : AnimPage passe `onRafaleValidate={rafaleValidate}` tel quel.
function ClassicHarness() {
  const { rafaleValidate, rafaleInvalidate } = useWebSocket('/ws/anim')
  return <AnimRafaleActions onValidate={rafaleValidate} onInvalidate={rafaleInvalidate} />
}
// Câblage carte MEMOTION : AnimPage (l.415) `() => rafaleValidate(hostContext.cardId)`.
function CardHarness({ cardId }) {
  const { rafaleValidate, rafaleInvalidate } = useWebSocket('/ws/anim')
  return (
    <AnimRafaleActions
      onValidate={() => rafaleValidate(cardId)}
      onInvalidate={() => rafaleInvalidate(cardId)}
    />
  )
}

describe('AnimRafaleActions + vrai useWebSocket — /anim hors carte (#282)', () => {
  beforeEach(() => { sent = []; MockWebSocket.all = []; vi.stubGlobal('WebSocket', MockWebSocket) })
  afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks() })

  it('clic RÉPONSE VALIDE : trame RAFALE_VALIDATE avec MSG sans MOTION_CARD_ID, sans exception', async () => {
    render(<ClassicHarness />)
    await openSocket()
    expect(() => fireEvent.click(screen.getByText(/RÉPONSE VALIDE/))).not.toThrow()
    const f = frames('RAFALE_VALIDATE')
    expect(f).toHaveLength(1)
    expect(f[0].MSG).toEqual({})
    expect('MOTION_CARD_ID' in f[0].MSG).toBe(false)
  })

  it('clic RÉPONSE INVALIDE : trame RAFALE_INVALIDATE avec MSG sans MOTION_CARD_ID, sans exception', async () => {
    render(<ClassicHarness />)
    await openSocket()
    expect(() => fireEvent.click(screen.getByText(/RÉPONSE INVALIDE/))).not.toThrow()
    const f = frames('RAFALE_INVALIDATE')
    expect(f).toHaveLength(1)
    expect(f[0].MSG).toEqual({})
  })
})

describe('AnimRafaleActions + vrai useWebSocket — carte MEMOTION (#217, non-régression)', () => {
  beforeEach(() => { sent = []; MockWebSocket.all = []; vi.stubGlobal('WebSocket', MockWebSocket) })
  afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks() })

  it('VALIDE : MOTION_CARD_ID transmis dans MSG', async () => {
    render(<CardHarness cardId="mc-7" />)
    await openSocket()
    fireEvent.click(screen.getByText(/RÉPONSE VALIDE/))
    expect(frames('RAFALE_VALIDATE')).toEqual([{ ACTION: 'RAFALE_VALIDATE', MSG: { MOTION_CARD_ID: 'mc-7' } }])
  })

  it('INVALIDE : MOTION_CARD_ID transmis dans MSG', async () => {
    render(<CardHarness cardId="mc-7" />)
    await openSocket()
    fireEvent.click(screen.getByText(/RÉPONSE INVALIDE/))
    expect(frames('RAFALE_INVALIDATE')).toEqual([{ ACTION: 'RAFALE_INVALIDATE', MSG: { MOTION_CARD_ID: 'mc-7' } }])
  })
})
