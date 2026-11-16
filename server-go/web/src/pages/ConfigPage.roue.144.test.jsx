/**
 * #144 (v12.0.0, LOT-1.2) — ConfigPage : la carte « Roue de la Fortune » (maquette M4) n'apparaît que si le serveur
 * renvoie la section `wheel` de GET /game-config.json (contracts/roue.md §10) ; `wheel_default` alimente « Roue par défaut ».
 */
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import ConfigPage from './ConfigPage'

vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn(), GameProvider: ({ children }) => children }))
vi.mock('../components/USBConfigModal', () => ({ default: () => null }))
vi.mock('../components/TeamCard', () => ({ OtaAllModal: () => null }))
import { useGame } from '../hooks/GameContext'

const DEF = { max_points: 1000, step: 50, bankrupt_count: 2, lose_turn_count: 1, spin_period_ms: 2500, spin_max_ms: 10000, stop_duration_ms: 4000, flip_interval_ms: 600 }

function mockFetch(gameConfig) {
  global.fetch = vi.fn((url, options) => {
    if (url === '/game-config.json' && (!options || !options.method)) return Promise.resolve({ ok: true, json: async () => gameConfig })
    if (url === '/api/roue/wheel-preview') return Promise.resolve({ ok: true, json: async () => ({ COUNT: 2, step: 50, SEGMENTS: [{ ID: 's0', KIND: 'value', VALUE: 1000 }, { ID: 's1', KIND: 'value', VALUE: 50 }] }) })
    if (url === '/config.json') return Promise.resolve({ ok: true, json: async () => ({ server: {}, ai: {} }) })
    if (url === '/api/wifi/defaults') return Promise.resolve({ ok: true, json: async () => ({ ssid: '', password: '', server_ip: '', server_port: 80 }) })
    if (url === '/api/firmware/buzzclick/version') return Promise.resolve({ ok: true, json: async () => ({ EXISTS: false }) })
    return Promise.resolve({ ok: false, json: async () => ({}), text: async () => '' })
  })
}

beforeEach(() => {
  useGame.mockReturnValue({
    teams: {}, bumpers: {}, gameState: { backgrounds: [] }, updateConfig: vi.fn(), sendMessage: vi.fn(),
    version: '12.0.0', firmwareInfo: { EXISTS: true, IS_MERGED: true, VERSION: '3.1.1', FILENAME: 'b.bin', SIZE: 1 },
  })
})

describe('ConfigPage — carte Roue (#144)', () => {
  it('affiche la carte quand GET /game-config.json renvoie `wheel`', async () => {
    mockFetch({ wheel: DEF, wheel_default: DEF })
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByTestId('wheel-config-card')).toBeTruthy(), { timeout: 3000 })
    expect(screen.getByLabelText('Points maximum').value).toBe('1000')
    expect(screen.getByLabelText('Pas').value).toBe('50')
  })

  it('n\'affiche pas la carte quand la section `wheel` est absente (ancien serveur / fichier sans section)', async () => {
    mockFetch({})
    render(<ConfigPage />)
    await waitFor(() => expect(global.fetch).toHaveBeenCalledWith('/game-config.json'), { timeout: 3000 })
    await new Promise(r => setTimeout(r, 100))
    expect(screen.queryByTestId('wheel-config-card')).toBeNull()
  })
})
