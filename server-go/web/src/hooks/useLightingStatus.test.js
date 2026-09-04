import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import {
  useLightingStatus,
  notifyLightingChanged,
  LIGHTING_STATUS_URL,
  LIGHTING_STATUS_INTERVAL_MS,
  EMPTY_LIGHTING_STATUS,
} from './useLightingStatus'

// ---------------------------------------------------------------------------
// #207 — GET /api/lighting/status : au montage, toutes les 30 s, et après tout
// enregistrement (contrat hue-bridge.md §7.1). Le précédent useUpdates
// n'appelle qu'au montage — insuffisant, un pont peut tomber en session.
// ---------------------------------------------------------------------------

const okResponse = (body) => ({ ok: true, status: 200, json: async () => body })

beforeEach(() => {
  vi.useFakeTimers()
  global.fetch = vi.fn().mockResolvedValue(okResponse({ state: 'ok', bridge_ip: '192.168.1.101', lights_ok: 2, lights_total: 3 }))
})

afterEach(() => {
  vi.useRealTimers()
  vi.restoreAllMocks()
})

const flush = async () => { await act(async () => { await Promise.resolve() }) }

describe('useLightingStatus — cadence des appels', () => {
  it('interroge au montage', async () => {
    const { result } = renderHook(() => useLightingStatus())
    await flush()
    expect(global.fetch).toHaveBeenCalledTimes(1)
    expect(global.fetch).toHaveBeenCalledWith(LIGHTING_STATUS_URL)
    expect(result.current.status.state).toBe('ok')
    expect(result.current.status.lights_ok).toBe(2)
  })

  it('ré-interroge toutes les 30 s (30 000 ms exactement)', async () => {
    renderHook(() => useLightingStatus())
    await flush()
    expect(LIGHTING_STATUS_INTERVAL_MS).toBe(30_000)

    await act(async () => { await vi.advanceTimersByTimeAsync(29_999) })
    expect(global.fetch).toHaveBeenCalledTimes(1)

    await act(async () => { await vi.advanceTimersByTimeAsync(1) })
    expect(global.fetch).toHaveBeenCalledTimes(2)

    await act(async () => { await vi.advanceTimersByTimeAsync(30_000) })
    expect(global.fetch).toHaveBeenCalledTimes(3)
  })

  it('se rafraîchit immédiatement après un enregistrement (notifyLightingChanged)', async () => {
    const { result } = renderHook(() => useLightingStatus())
    await flush()
    expect(global.fetch).toHaveBeenCalledTimes(1)

    global.fetch.mockResolvedValueOnce(okResponse({ state: 'unreachable' }))
    await act(async () => { notifyLightingChanged(); await Promise.resolve() })
    await flush()

    expect(global.fetch).toHaveBeenCalledTimes(2)
    expect(result.current.status.state).toBe('unreachable')
  })

  it('arrête l\'intervalle et l\'écoute au démontage', async () => {
    const { unmount } = renderHook(() => useLightingStatus())
    await flush()
    unmount()

    await act(async () => { await vi.advanceTimersByTimeAsync(90_000) })
    notifyLightingChanged()
    expect(global.fetch).toHaveBeenCalledTimes(1)
  })

  it('expose refresh() qui interroge à la demande', async () => {
    const { result } = renderHook(() => useLightingStatus())
    await flush()
    await act(async () => { await result.current.refresh() })
    expect(global.fetch).toHaveBeenCalledTimes(2)
  })
})

describe('useLightingStatus — robustesse', () => {
  it('état initial = non configuré (disabled), avant toute réponse', () => {
    global.fetch = vi.fn(() => new Promise(() => {}))
    const { result } = renderHook(() => useLightingStatus())
    expect(result.current.status).toEqual(EMPTY_LIGHTING_STATUS)
  })

  it('une réponse {} (mock générique des autres tests) vaut disabled', async () => {
    global.fetch = vi.fn().mockResolvedValue(okResponse({}))
    const { result } = renderHook(() => useLightingStatus())
    await flush()
    expect(result.current.status.state).toBe('disabled')
  })

  it('un state inconnu est normalisé en disabled', async () => {
    global.fetch = vi.fn().mockResolvedValue(okResponse({ state: 'weird' }))
    const { result } = renderHook(() => useLightingStatus())
    await flush()
    expect(result.current.status.state).toBe('disabled')
  })

  it('404 (serveur sans le module) → disabled', async () => {
    global.fetch = vi.fn()
      .mockResolvedValueOnce(okResponse({ state: 'ok' }))
      .mockResolvedValueOnce({ ok: false, status: 404, json: async () => ({}) })
    const { result } = renderHook(() => useLightingStatus())
    await flush()
    expect(result.current.status.state).toBe('ok')
    await act(async () => { await result.current.refresh() })
    expect(result.current.status.state).toBe('disabled')
  })

  it('erreur réseau vers NOTRE serveur : dernier état conservé, pas d\'exception', async () => {
    global.fetch = vi.fn()
      .mockResolvedValueOnce(okResponse({ state: 'ok' }))
      .mockRejectedValueOnce(new TypeError('Failed to fetch'))
    const { result } = renderHook(() => useLightingStatus())
    await flush()
    expect(result.current.status.state).toBe('ok')
    await act(async () => { await result.current.refresh() })
    expect(result.current.status.state).toBe('ok')
  })

  it('fetch absent (environnement sans réseau) : ne plante pas', async () => {
    global.fetch = undefined
    const { result } = renderHook(() => useLightingStatus())
    await flush()
    expect(result.current.status.state).toBe('disabled')
  })
})
