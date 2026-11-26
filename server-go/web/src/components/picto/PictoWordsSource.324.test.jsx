import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import PictoWordsSource, { PICTO_KEYS, isPictoFilterValid } from './PictoWordsSource'
import { isPictoReady } from './PictoEditor'

// #324 (v13.1.0, LOT-2 T14) — sous-éditeur partagé « Mots à dessiner » (picto.md §17.2) : source, filtre, alerte.
vi.mock('../PoolAlert', () => ({ default: ({ categories, difficulties }) => <div data-testid="pool-alert">{categories.join(',')}|{difficulties.join(',')}</div> }))

const CATS = [{ key: 'ANIMALS', name: 'Animaux', isCustom: true }]
const base = (o = {}) => ({ category: 'ANIMALS', pictoSource: 'RESERVOIR', pictoCategories: ['ANIMALS'], pictoDifficulties: [1, 2, 3], time: '90', pictoWords: [], ...o })

describe('PictoWordsSource', () => {
  it('source Pool : filtre + alerte, sans les mots saisis (children)', () => {
    render(<PictoWordsSource values={base()} onChange={() => {}} customCategories={CATS}><div data-testid="words-editor" /></PictoWordsSource>)
    expect(screen.getByTestId('pool-alert').textContent).toBe('ANIMALS|1,2,3')
    expect(screen.queryByTestId('words-editor')).toBeNull()
  })
  it('source Mots saisis : children affichés, pas d\'alerte', () => {
    render(<PictoWordsSource values={base({ pictoSource: 'WORDS' })} onChange={() => {}} customCategories={CATS}><div data-testid="words-editor" /></PictoWordsSource>)
    expect(screen.getByTestId('words-editor')).toBeTruthy()
    expect(screen.queryByTestId('pool-alert')).toBeNull()
  })
  it('source absente = mots saisis (question v13.0.0)', () => {
    render(<PictoWordsSource values={base({ pictoSource: undefined })} onChange={() => {}}><div data-testid="words-editor" /></PictoWordsSource>)
    expect(screen.getByTestId('words-editor')).toBeTruthy()
  })
  it('le basculement émet la source par le nom de champ paramétré (keys)', () => {
    const onChange = vi.fn()
    render(<PictoWordsSource values={base()} onChange={onChange} keys={{ source: 'rpSource', categories: 'rpCats', difficulties: 'rpDiffs' }}><span /></PictoWordsSource>)
    fireEvent.click(screen.getByRole('button', { name: '✍️ Saisis dans la question' }))
    expect(onChange).toHaveBeenCalledWith('rpSource', 'WORDS')
  })
  it('filtre par défaut = catégorie de la question quand aucune catégorie n\'est choisie', () => {
    const onChange = vi.fn()
    render(<PictoWordsSource values={base({ pictoCategories: [] })} onChange={onChange} customCategories={CATS}><span /></PictoWordsSource>)
    expect(onChange).toHaveBeenCalledWith(PICTO_KEYS.categories, ['ANIMALS'])
  })
  it('isPictoFilterValid : au moins une catégorie et des difficultés 1 à 3', () => {
    expect(isPictoFilterValid(['A'], [1])).toBe(true)
    expect(isPictoFilterValid([], [1])).toBe(false)
    expect(isPictoFilterValid(['A'], [])).toBe(false)
    expect(isPictoFilterValid(['A'], [4])).toBe(false)
  })
  it('isPictoReady : source Pool valide sans mots ; filtre vide ou durée invalide refusés ; mots saisis inchangé', () => {
    expect(isPictoReady(base())).toBe(true)
    expect(isPictoReady(base({ pictoCategories: [] }))).toBe(false)
    expect(isPictoReady(base({ time: '20' }))).toBe(false)
    expect(isPictoReady(base({ pictoSource: 'WORDS' }))).toBe(false)
    expect(isPictoReady(base({ pictoSource: 'WORDS', pictoWords: ['chat'] }))).toBe(true)
  })
})

describe('PictoWordsSource — variante (cycle 5)', () => {
  it('Picto (un mot) : libellé « Catégories du pool » et étoiles vides (ghost), comme avant le cycle 5', () => {
    render(<PictoWordsSource values={base()} onChange={() => {}} customCategories={CATS}><span /></PictoWordsSource>)
    expect(document.body.textContent).toContain('Catégories du pool')
    expect(document.body.textContent).toContain('★☆☆')
  })
  it('Rafale Picto : libellé par défaut (comme Rafale Speedy) et étoiles pleines seulement', () => {
    render(<PictoWordsSource variant="rafale" values={base()} onChange={() => {}} customCategories={CATS}><span /></PictoWordsSource>)
    expect(document.body.textContent).not.toContain('Catégories du pool')
    expect(document.body.textContent).not.toContain('☆')
  })
})
