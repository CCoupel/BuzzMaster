import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { clearOwnVPlayerSession, isStaleVPlayerSession } from './vplayerSession'

// #246 — le localStorage est partagé entre onglets : jamais d'effacement de la session d'un autre.

function createStorage() {
  let store = {}
  return {
    getItem: (k) => (k in store ? store[k] : null),
    setItem: (k, v) => { store[k] = String(v) },
    removeItem: (k) => { delete store[k] },
  }
}

describe('vplayerSession (#246)', () => {
  let ls
  beforeEach(() => {
    ls = createStorage()
    vi.stubGlobal('localStorage', ls)
    ls.setItem('vplayer_name', 'Paul')
    ls.setItem('vplayer_session', '1')
  })
  afterEach(() => vi.unstubAllGlobals())

  it('efface quand l\'ID stocké est celui de l\'appelant', () => {
    ls.setItem('vplayer_id', 'A')
    expect(clearOwnVPlayerSession('A')).toBe(true)
    expect(ls.getItem('vplayer_name')).toBeNull()
    expect(ls.getItem('vplayer_id')).toBeNull()
  })

  it('efface quand rien n\'est stocké (ID absent)', () => {
    expect(clearOwnVPlayerSession('A')).toBe(true)
    expect(ls.getItem('vplayer_name')).toBeNull()
  })

  it('n\'efface rien quand l\'ID stocké est celui d\'un autre onglet', () => {
    ls.setItem('vplayer_id', 'B')
    expect(clearOwnVPlayerSession('A')).toBe(false)
    expect(ls.getItem('vplayer_id')).toBe('B')
    expect(ls.getItem('vplayer_name')).toBe('Paul')
  })

  it.each([null, undefined, ''])('ownId %j et ID stocké non vide : n\'efface rien', (own) => {
    ls.setItem('vplayer_id', 'B')
    expect(clearOwnVPlayerSession(own)).toBe(false)
    expect(ls.getItem('vplayer_id')).toBe('B')
    expect(ls.getItem('vplayer_session')).toBe('1')
  })

  it('isStaleVPlayerSession : périmé seulement si un autre ID non vide est stocké', () => {
    ls.setItem('vplayer_id', 'B')
    expect(isStaleVPlayerSession('A')).toBe(true)
    expect(isStaleVPlayerSession('B')).toBe(false)
    ls.removeItem('vplayer_id')
    expect(isStaleVPlayerSession('A')).toBe(false)
  })
})
