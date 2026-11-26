import { describe, it, expect, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import PoolAlert from './PoolAlert'
import RafalePoolAlert from './RafalePoolAlert'

// #324 (v13.1.0, LOT-2 T13 générique) — alerte de pool paramétrée par instance.
const json = (b) => Promise.resolve({ ok: true, json: () => Promise.resolve(b) })

describe('PoolAlert', () => {
  it('instance picto : interroge /api/picto/pool, unité « mot »', async () => {
    global.fetch = vi.fn(() => json({ AVAILABLE: 1, USED: 0, TOTAL: 1 }))
    render(<PoolAlert instance="picto" categories={['HISTORY']} difficulties={[1]} />)
    await waitFor(() => expect(screen.getByText('1 mot disponible')).toBeTruthy())
    expect(global.fetch.mock.calls[0][0]).toMatch(/^\/api\/picto\/pool\?/)
  })
  it('instance picto, insuffisant : « N mots disponibles — insuffisant »', async () => {
    global.fetch = vi.fn(() => json({ AVAILABLE: 2, USED: 0, TOTAL: 2 }))
    render(<PoolAlert instance="picto" categories={['HISTORY']} difficulties={[1]} roundTime={100} questionTime={10} />)
    await waitFor(() => expect(screen.getByText('2 mots disponibles — insuffisant')).toBeTruthy())
  })
  it('RafalePoolAlert = PoolAlert rafale : /api/rafale/pool, unité « question »', async () => {
    global.fetch = vi.fn(() => json({ AVAILABLE: 3, USED: 0, TOTAL: 3 }))
    render(<RafalePoolAlert categories={['HISTORY']} difficulties={[1]} />)
    await waitFor(() => expect(screen.getByText('3 questions disponibles')).toBeTruthy())
    expect(global.fetch.mock.calls[0][0]).toMatch(/^\/api\/rafale\/pool\?/)
  })
})
