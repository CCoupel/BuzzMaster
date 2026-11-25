import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import BackupPage from './BackupPage'

// #324 (v13.1.0, LOT-2 T14, M2 B) — catégorie « Pools » : case parente à trois états qui déplie « Pool Speedy »
// (clé serveur `rafale`, inchangée) et « Pool Picto » (clé `picto`), en sauvegarde comme en réinitialisation.
vi.mock('../hooks/GameContext', () => ({ useGame: () => ({ sendMessage: vi.fn() }), GameProvider: ({ children }) => children }))

const group = (i) => ({ // i = 0 sauvegarde, 1 réinitialisation
  parent: screen.getAllByLabelText('Pools')[i],
  speedy: screen.getAllByLabelText('Pool Speedy')[i],
  picto: screen.getAllByLabelText('Pool Picto')[i],
})

beforeEach(() => {
  vi.clearAllMocks()
  global.confirm = vi.fn(() => true)
  global.fetch = vi.fn().mockResolvedValue({ ok: true, blob: async () => new Blob([]) })
  global.URL.createObjectURL = vi.fn(() => 'blob:x')
  global.URL.revokeObjectURL = vi.fn()
})

describe('BackupPage — catégorie « Pools »', () => {
  it('plus de « Questions RAFALE » ; Pools, Pool Speedy, Pool Picto dans les deux sections', () => {
    render(<BackupPage />)
    expect(screen.queryByLabelText('Questions RAFALE')).toBeNull()
    ;['Pools', 'Pool Speedy', 'Pool Picto'].forEach((l) => expect(screen.getAllByLabelText(l)).toHaveLength(2))
  })

  it('sauvegarde : tout coché par défaut ; décocher un enfant met le parent en état indéterminé', () => {
    render(<BackupPage />)
    const g = group(0)
    expect([g.parent.checked, g.speedy.checked, g.picto.checked]).toEqual([true, true, true])
    expect(g.parent.indeterminate).toBe(false)
    fireEvent.click(g.picto)
    expect(g.parent.checked).toBe(false)
    expect(g.parent.indeterminate).toBe(true)
  })

  it('le parent coche / décoche les deux pools', () => {
    render(<BackupPage />)
    const g = group(0)
    fireEvent.click(g.parent)
    expect([g.speedy.checked, g.picto.checked]).toEqual([false, false])
    expect(g.parent.indeterminate).toBe(false)
    fireEvent.click(g.parent)
    expect([g.speedy.checked, g.picto.checked]).toEqual([true, true])
  })

  it('la sauvegarde envoie rafale=true (clé inchangée) et picto=true ; seulement les pools cochés', async () => {
    render(<BackupPage />)
    fireEvent.click(screen.getByText('Sauvegarder'))
    await waitFor(() => expect(global.fetch).toHaveBeenCalledTimes(1))
    const url = global.fetch.mock.calls[0][0]
    expect(url).toContain('rafale=true')
    expect(url).toContain('picto=true')
    global.fetch.mockClear()
    fireEvent.click(group(0).picto)
    fireEvent.click(screen.getByText('Sauvegarder'))
    await waitFor(() => expect(global.fetch).toHaveBeenCalledTimes(1))
    expect(global.fetch.mock.calls[0][0]).toContain('rafale=true')
    expect(global.fetch.mock.calls[0][0]).not.toContain('picto=true')
  })

  it('réinitialisation : décochée par défaut ; « Pool Picto » seul envoie picto=true, confirmation « Pool Picto »', async () => {
    render(<BackupPage />)
    const g = group(1)
    expect([g.parent.checked, g.speedy.checked, g.picto.checked]).toEqual([false, false, false])
    fireEvent.click(g.picto)
    expect(g.parent.indeterminate).toBe(true)
    fireEvent.click(screen.getByText('Reinitialiser'))
    await waitFor(() => expect(global.fetch).toHaveBeenCalledTimes(1))
    const url = global.fetch.mock.calls[0][0]
    expect(url).toContain('picto=true')
    expect(url).not.toContain('rafale=true')
    expect(global.confirm.mock.calls[0][0]).toContain('Pool Picto')
  })

  it('réinitialisation par le parent : les deux pools, libellés Pool Speedy et Pool Picto', async () => {
    render(<BackupPage />)
    fireEvent.click(group(1).parent)
    fireEvent.click(screen.getByText('Reinitialiser'))
    await waitFor(() => expect(global.fetch).toHaveBeenCalledTimes(1))
    expect(global.fetch.mock.calls[0][0]).toContain('rafale=true')
    expect(global.fetch.mock.calls[0][0]).toContain('picto=true')
    expect(global.confirm.mock.calls[0][0]).toMatch(/Pool Speedy.*Pool Picto|Pool Picto.*Pool Speedy/)
  })
})
