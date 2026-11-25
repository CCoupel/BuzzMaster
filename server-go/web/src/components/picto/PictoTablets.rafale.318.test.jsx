import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import PictoTv from './PictoTv'
import { createPictoCanvasStore } from './pictoCanvasStore'

vi.mock('../Timer', () => ({ default: ({ currentTime }) => <div data-testid="timer">{currentTime}</div> }))
beforeEach(() => { HTMLCanvasElement.prototype.getContext = () => null })

// #318 (v13.1.0, LOT-3 T25, maquette M5) — tablettes Rafale Picto : le dessinateur n'a AUCUN bouton « Passer ».
const gs = (o = {}) => ({
  phase: 'STARTED', timer: 42, totalTime: 90,
  question: { ID: 'q', TYPE: 'RAFALE_PICTO', CATEGORY: 'ANIMALS', POINTS: 10, TIME: 90 },
  PICTO_TEAM: 'Bleue', PICTO_DRAWER: 'b1', PICTO_DRAWER_NAME: 'Alice', PICTO_CANVAS_EPOCH: 2,
  PICTO_FOUND_COUNT: 3, PICTO_FOUND_WORDS: ['Chat', 'Soleil', 'Vélo'], PICTO_PASS_COUNT: 1, PICTO_PASSES_LEFT: -1,
  PICTO_REVEALED_WORD: '', ...o,
})
const send = vi.fn()
const tablet = (g, who, extra = {}) => render(<PictoTv gameState={g} isVPlayer playerId={who.id} teamName={who.team} pictoWord={{ WORD: 'Maison', EPOCH: 2 }} store={createPictoCanvasStore()} sendMessage={send} catMeta={{ icon: '🏠', label: 'Vie quotidienne' }} {...extra} />)
const ALICE = { id: 'b1', team: 'Bleue' }, BRUNO = { id: 'b2', team: 'Bleue' }, CHLOE = { id: 'b3', team: 'Rouge' }

describe('Tablette du dessinateur — Rafale Picto', () => {
  it('mot en grand, catégorie « Ton mot · … », compteur de mots trouvés', () => {
    tablet(gs(), ALICE)
    expect(screen.getByTestId('picto-drawer-word').textContent).toBe('Maison')
    expect(screen.getByTestId('picto-drawer-cat').textContent).toContain('Ton mot')
    expect(screen.getByTestId('picto-drawer-cat').textContent).toContain('Vie quotidienne')
    expect(screen.getByTestId('picto-drawer-found').textContent).toContain('3 mots')
  })
  it('AUCUN bouton « Passer » ni aucune action PICTO_PASS / PICTO_FOUND sur la tablette', () => {
    tablet(gs(), ALICE)
    expect(screen.queryByRole('button', { name: /pass/i })).toBeNull()
    expect(document.body.textContent).not.toMatch(/Passer/)
    expect(send.mock.calls.some(c => /PICTO_PASS|PICTO_FOUND/.test(c[0]))).toBe(false)
  })
  it('le type Picto (un mot) : pas de compteur ni de catégorie « Ton mot »', () => {
    tablet(gs({ question: { ID: 'q', TYPE: 'PICTO', CATEGORY: 'ANIMALS' } }), ALICE)
    expect(screen.queryByTestId('picto-drawer-found')).toBeNull()
    expect(screen.queryByTestId('picto-drawer-cat')).toBeNull()
  })
})

describe('Tablettes équipes — Rafale Picto', () => {
  it('équipe qui devine : « Devinez ! », mots trouvés et passes utilisées', () => {
    tablet(gs(), BRUNO)
    expect(document.body.textContent).toContain('Devinez !')
    const c = screen.getByTestId('picto-guesser-counts').textContent
    expect(c).toContain('3 mots trouvés')
    expect(c).toContain('1 passe utilisée')
  })
  it('autres équipes : l\'équipe qui devine et le compteur, sans le détail des passes', () => {
    tablet(gs(), CHLOE)
    expect(document.body.textContent).toContain('Bleue devine le dessin d\'Alice')
    const c = screen.getByTestId('picto-guesser-counts').textContent
    expect(c).toContain('3 mots trouvés')
    expect(c).not.toContain('passe')
  })
  it('révélation : mot révélé, liste des mots trouvés, « 3 mots trouvés · 1 passe » ; jamais un mot passé', () => {
    tablet(gs({ phase: 'REVEALED', PICTO_REVEALED_WORD: 'Maison' }), BRUNO)
    expect(document.body.textContent).toContain('Mot révélé')
    expect(document.body.textContent).toContain('Maison')
    const list = screen.getByTestId('picto-guesser-found-list').textContent
    ;['Chat', 'Soleil', 'Vélo'].forEach(w => expect(list).toContain(w))
    expect(screen.getByTestId('picto-guesser-recap').textContent).toBe('3 mots trouvés · 1 passe')
    expect(document.body.textContent).not.toContain('Girafe')
  })
  it('révélation du dessinateur : « Mot révélé » sans compteur de manche', () => {
    tablet(gs({ phase: 'REVEALED', PICTO_REVEALED_WORD: 'Maison' }), ALICE)
    expect(document.body.textContent).toContain('Mot révélé')
    expect(screen.queryByTestId('picto-drawer-found')).toBeNull()
  })
  it('Picto un mot : écran de l\'équipe inchangé (pas de compteur)', () => {
    tablet(gs({ question: { ID: 'q', TYPE: 'PICTO', CATEGORY: 'ANIMALS' } }), BRUNO)
    expect(screen.queryByTestId('picto-guesser-counts')).toBeNull()
    expect(document.body.textContent).toContain('Devinez !')
  })
})
