import { describe, it, expect, beforeEach } from 'vitest'
import {
  clearVPlayerSession, clearOwnVPlayerSession, getStoredVPlayerToken, playerConnectPayload,
} from './vplayerSession'
import { REDIRECT_MESSAGES } from './playerConnectMessages'

// #319 (contrat vplayer-session.md §5) — jeton de session côté tablette.
beforeEach(() => localStorage.clear())

describe('vplayer_token — stockage et règle d\'onglet', () => {
  it('le jeton n\'est celui de l\'onglet que si l\'ID stocké est le sien', () => {
    localStorage.setItem('vplayer_id', 'b1'); localStorage.setItem('vplayer_token', 'tok')
    expect(getStoredVPlayerToken('b1')).toBe('tok')
    expect(getStoredVPlayerToken('b2')).toBe('') // un autre onglet a enregistré une autre session
    expect(getStoredVPlayerToken(null)).toBe('')
  })
  it('PLAYER_CONNECT : { NAME, ID, TOKEN } avec jeton ; { NAME, ID } sans (session d\'avant la mise à jour, migration 1c)', () => {
    localStorage.setItem('vplayer_id', 'b1')
    expect(playerConnectPayload('Alice', 'b1')).toEqual({ NAME: 'Alice', ID: 'b1' })
    localStorage.setItem('vplayer_token', 'tok')
    expect(playerConnectPayload('Alice', 'b1')).toEqual({ NAME: 'Alice', ID: 'b1', TOKEN: 'tok' })
  })
  it('le jeton est effacé avec la session (point d\'entrée unique)', () => {
    localStorage.setItem('vplayer_name', 'A'); localStorage.setItem('vplayer_session', 's')
    localStorage.setItem('vplayer_id', 'b1'); localStorage.setItem('vplayer_token', 'tok')
    clearVPlayerSession()
    expect(localStorage.getItem('vplayer_token')).toBeNull()
  })
  it('un onglet ne purge pas le jeton d\'une autre session', () => {
    localStorage.setItem('vplayer_id', 'b1'); localStorage.setItem('vplayer_token', 'tok')
    expect(clearOwnVPlayerSession('b2')).toBe(false)
    expect(localStorage.getItem('vplayer_token')).toBe('tok')
    expect(clearOwnVPlayerSession('b1')).toBe(true)
    expect(localStorage.getItem('vplayer_token')).toBeNull()
  })
})

describe('INVALID_SESSION — motif de redirection', () => {
  it('porte le texte du contrat §4.4', () => {
    expect(REDIRECT_MESSAGES.INVALID_SESSION).toBe('Cette place est déjà liée à un autre appareil. Demandez à l’animateur de libérer la place, puis reconnectez-vous avec votre nom.')
  })
})
