import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import CategoryPicker from './CategoryPicker'

// #285 (LOT-4, retour QUALIF) — la gestion quitte la zone des questions : le sélecteur de l'éditeur n'a ni « Gérer »
// ni « + Créer » ; « ⚙ Gérer… » reste sur le filtre de la zone d'équilibre (prop allowManage). La création est
// dans CategoryManagerDialog (CategoryManagerDialog.create285.test.jsx).
vi.mock('./CategoryPicker.css', () => ({}))
vi.mock('./CategoryBadge.css', () => ({}))

const CATS = [{ key: 'CINEMA', name: 'Cinéma', imageURL: '/files/categories/CINEMA.png', color: '#0ea5e9', isCustom: true }]

describe('CategoryPicker — plus de gestion dans l\'éditeur de questions (#285)', () => {
  it('sélecteur unique (éditeur) : ni « Gérer » ni « Créer / Ajouter »', () => {
    render(<CategoryPicker mode="single" value="" onChange={() => {}} customCategories={CATS} />)
    expect(screen.getByRole('radio', { name: /Cinéma/ })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Gérer/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Créer|Ajouter|\+/ })).not.toBeInTheDocument()
  })

  it('l\'ancienne prop allowCreate n\'ajoute plus rien', () => {
    render(<CategoryPicker mode="single" value="" onChange={() => {}} customCategories={CATS} allowCreate />)
    expect(screen.queryByRole('button', { name: /Gérer/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Créer|Ajouter|\+/ })).not.toBeInTheDocument()
  })

  it('filtre de la zone d\'équilibre avec allowManage : « Gérer » présent, jamais « Créer / Ajouter »', () => {
    render(<CategoryPicker variant="filter" value={new Set()} onToggle={() => {}} customCategories={CATS} availableCategories={['CINEMA']} allowManage />)
    expect(screen.getByRole('button', { name: /Gérer/ })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Créer|Ajouter|\+/ })).not.toBeInTheDocument()
  })

  it('filtre sans allowManage : pas de « Gérer »', () => {
    render(<CategoryPicker variant="filter" value={new Set()} onToggle={() => {}} customCategories={CATS} availableCategories={['CINEMA']} />)
    expect(screen.queryByRole('button', { name: /Gérer/ })).not.toBeInTheDocument()
  })
})
