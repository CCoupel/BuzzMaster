import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'

// ---------------------------------------------------------------------------
// #290 (v11.6.0, LOT-4) — en-tête de manche : bouton 🎛 « Ambiance » (pas sur « Sans manche ») et badge
// « Ambiance : N réglages » avec info-bulle. Maquette admin_questions_list__round_ambiance_override.html :
// « ⋮⋮ ▸ Manche 2 — Blind-test … 🎛 Ambiance : 3 réglages ; ▲ ▼ 🎛 ✎ 🗑 » ; le badge résume le nombre de réglages
// surchargés, son info-bulle les liste ; structure v11.2 inchangée. Props existantes de RoundGroup (+ onAmbiance,
// ambianceCount, ambianceTitle). Écrit depuis la maquette.
// ---------------------------------------------------------------------------

vi.mock('./RoundGroup.css', () => ({}))
import RoundGroup from './RoundGroup'

const base = { name: 'Blind-test', index: 1, total: 3, countText: '6 questions', onToggle: vi.fn(), onUp: vi.fn(), onDown: vi.fn(), onRename: vi.fn(), onDelete: vi.fn(), onAddQuestion: vi.fn() }
const ambBtn = () => screen.getByRole('button', { name: /ambiance/i })

describe('RoundGroup — bouton 🎛 Ambiance (#290)', () => {
  it('une manche affiche le bouton 🎛 ; il appelle onAmbiance', () => {
    const onAmbiance = vi.fn()
    render(<RoundGroup {...base} onAmbiance={onAmbiance}><div /></RoundGroup>)
    expect(ambBtn().textContent).toContain('🎛')
    fireEvent.click(ambBtn())
    expect(onAmbiance).toHaveBeenCalledTimes(1)
  })

  it('« Sans manche » n\'a PAS de bouton Ambiance (pas de surcharge pour elle)', () => {
    render(<RoundGroup unassigned title="Sans manche" countText="3 questions" onToggle={vi.fn()} onAmbiance={vi.fn()}><div /></RoundGroup>)
    expect(screen.queryByRole('button', { name: /ambiance/i })).toBeNull()
  })

  it('la structure v11.2 reste : ▲ ▼ ✎ 🗑 présents avec le nouveau bouton', () => {
    render(<RoundGroup {...base} onAmbiance={vi.fn()}><div /></RoundGroup>)
    for (const g of ['▲', '▼', '✎', '🗑']) {
      expect(screen.getAllByRole('button').some((b) => b.textContent.includes(g))).toBe(true)
    }
  })

  it('sans onAmbiance (hôte qui ne gère pas la surcharge) : pas de bouton', () => {
    render(<RoundGroup {...base}><div /></RoundGroup>)
    expect(screen.queryByRole('button', { name: /ambiance/i })).toBeNull()
  })
})

describe('RoundGroup — badge « Ambiance : N réglages » (#290)', () => {
  it('N > 0 : « Ambiance : 3 réglages » avec l\'info-bulle listant les réglages', () => {
    render(<RoundGroup {...base} onAmbiance={vi.fn()} ambianceCount={3} ambianceTitle="Décompte inactif · Indice révélé inactif · Lumière ON"><div /></RoundGroup>)
    const badge = screen.getByText(/Ambiance : 3 réglages/)
    expect(badge).toBeInTheDocument()
    expect(badge).toHaveAttribute('title', 'Décompte inactif · Indice révélé inactif · Lumière ON')
  })

  it('un seul réglage : accord au singulier', () => {
    render(<RoundGroup {...base} onAmbiance={vi.fn()} ambianceCount={1}><div /></RoundGroup>)
    expect(screen.getByText(/Ambiance : 1 réglage(?!s)/)).toBeInTheDocument()
  })

  it('aucune surcharge (N = 0) : pas de badge', () => {
    render(<RoundGroup {...base} onAmbiance={vi.fn()} ambianceCount={0}><div /></RoundGroup>)
    expect(screen.queryByText(/Ambiance : \d/)).toBeNull()
  })

  it('« Sans manche » ne porte jamais de badge', () => {
    render(<RoundGroup unassigned title="Sans manche" countText="3 questions" onToggle={vi.fn()} ambianceCount={2}><div /></RoundGroup>)
    expect(screen.queryByText(/Ambiance : \d/)).toBeNull()
  })
})
