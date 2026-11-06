import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, act, within } from '@testing-library/react'
import CategoryManagerDialog, { PUSH_GRACE_MS } from './CategoryManagerDialog'

// #285 (re-revue LOT-4, mineur 3) — repli si CATEGORIES_CHANGED est perdu : après une écriture réussie, la liste
// (prop `categories`) doit être renouvelée par le push ; sinon onChanged() (GET parent) après PUSH_GRACE_MS.

vi.mock('./CategoryManagerDialog.css', () => ({}))
vi.mock('./CategoryColorSwatch.css', () => ({}))

const LIST = () => [
  { key: 'CINEMA', name: 'Cinéma', imageURL: '/files/categories/CINEMA.png', color: '#0ea5e9', isCustom: true, questionCount: 1, rafaleCount: 0 },
]

beforeEach(() => {
  vi.useFakeTimers()
  global.fetch = vi.fn(async () => ({ ok: true, status: 200, json: async () => ({ ok: true }), text: async () => '{}' }))
})
afterEach(() => { vi.useRealTimers(); vi.restoreAllMocks() })

async function recolor(onChanged, categories) {
  const view = render(<CategoryManagerDialog categories={categories} onClose={() => {}} onChanged={onChanged} />)
  const dlg = screen.getByRole('dialog')
  const row = within(dlg).getByText('Cinéma').closest('tr')
  fireEvent.click(within(row).getByRole('button', { name: /couleur/i }))
  await act(async () => { fireEvent.click(screen.getByRole('button', { name: /aucune/i })) })
  return view
}

describe('CategoryManagerDialog — repli si le push est perdu (#285)', () => {
  it('aucun push dans le délai ⇒ onChanged() appelé une fois', async () => {
    const onChanged = vi.fn()
    await recolor(onChanged, LIST())
    expect(onChanged).not.toHaveBeenCalled()
    await act(async () => { vi.advanceTimersByTime(PUSH_GRACE_MS + 50) })
    expect(onChanged).toHaveBeenCalledTimes(1)
  })

  it('push reçu (nouveau tableau `categories`) avant le délai ⇒ onChanged() jamais appelé', async () => {
    const onChanged = vi.fn()
    const view = await recolor(onChanged, LIST())
    view.rerender(<CategoryManagerDialog categories={LIST()} onClose={() => {}} onChanged={onChanged} />)
    await act(async () => { vi.advanceTimersByTime(PUSH_GRACE_MS + 50) })
    expect(onChanged).not.toHaveBeenCalled()
  })
})
