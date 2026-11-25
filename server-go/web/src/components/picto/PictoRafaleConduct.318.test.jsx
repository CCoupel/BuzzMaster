import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, within } from '@testing-library/react'
import PictoPanel from './PictoPanel'
import { AnimPictoView } from './AnimPicto'
import { makePictoActions } from './pictoState'
import { createPictoCanvasStore } from './pictoCanvasStore'

beforeEach(() => { HTMLCanvasElement.prototype.getContext = () => null })

// #318 (v13.1.0, LOT-3 T26/T27, maquettes M6 anim et M7 admin) — conduite d'une manche Rafale Picto.
const CATS = [{ key: 'HOME', name: 'Vie quotidienne', isCustom: true }, { key: 'ANIMALS', name: 'Animaux', isCustom: true }]
const Q = { ID: 'q', TYPE: 'RAFALE_PICTO', CATEGORY: 'ANIMALS', POINTS: 10, TIME: 90, RAFALE_PICTO_PASS_COST: 5, RAFALE_PICTO_PASS_LIMIT: 0, RAFALE_PICTO_SOURCE: 'RESERVOIR', RAFALE_PICTO_POOL_END: 'LOOP' }
const gs = (o = {}) => ({
  phase: 'STARTED', timer: 42, totalTime: 90, question: Q,
  PICTO_TEAM: 'Bleue', PICTO_DRAWER: 'b1', PICTO_DRAWER_NAME: 'Alice', PICTO_DRAWER_CONNECTED: true, PICTO_CANDIDATES: [],
  PICTO_CANVAS_EPOCH: 5, PICTO_WORD_CATEGORY: 'HOME', PICTO_OUTCOME: '', PICTO_REVEALED_WORD: '',
  PICTO_FOUND_COUNT: 3, PICTO_FOUND_WORDS: ['Chat', 'Soleil', 'Vélo'], PICTO_PASS_COUNT: 1, PICTO_PASSES_LEFT: -1, PICTO_POOL_AVAILABLE: 38, PICTO_ROUND_POINTS: 25, ...o,
})
const WORD = { WORD: 'Maison', EPOCH: 5 }
const mkActions = () => { const sent = []; return { sent, ...makePictoActions((a, p) => sent.push([a, p])) } }
const teamList = [{ name: 'Bleue', color: [0, 0, 255] }]
const admin = (g, a = mkActions(), w = WORD) => render(<PictoPanel gameState={g} actions={a} teamList={teamList} pictoWord={w} customCategories={CATS} />)
const anim = (g, a = mkActions(), w = WORD, extra = {}) => render(<AnimPictoView gameState={g} teams={{ Bleue: { COLOR: [0, 0, 255] } }} bumpers={{ b1: { TEAM: 'Bleue' } }} actions={a} pictoWord={w} store={createPictoCanvasStore()} customCategories={CATS} {...extra} />)

describe.each([['admin (M7)', admin, 'admin'], ['anim (M6)', anim, 'anim']])('Conduite Rafale Picto — %s', (_n, mount, variant) => {
  it('compteurs : trouvés, passes · illimitées, coût, restants, crédit suggéré du serveur', () => {
    mount(gs())
    const st = screen.getByTestId('picto-rafale-stats').textContent
    expect(st).toContain('3 trouvés')
    expect(st).toContain('1 passe · illimitées')
    expect(st).toContain('−5 pts / passe')
    expect(st).toContain('38 restants')
    expect(screen.getByTestId('picto-rafale-credit').textContent).toContain('25 pts')
    expect(screen.getByTestId('picto-rafale').className).toContain(`picto-rafale-${variant}`)
  })
  it('mot en cours en grand avec son numéro et sa catégorie ; après rechargement, les mots trouvés (publics) reconstituent le fil', () => {
    mount(gs({ PICTO_FOUND_COUNT: 0, PICTO_FOUND_WORDS: [], PICTO_PASS_COUNT: 0 }))
    expect(screen.getByTestId('picto-rafale-word').textContent).toBe('Maison')
    expect(document.body.textContent).toContain('Mot 1')
    expect(document.body.textContent).toContain('Vie quotidienne')
  })
  it('rechargement en cours de manche : 3 mots trouvés connus → le mot en cours est le « Mot 4 »', () => {
    mount(gs())
    expect(document.body.textContent).toContain('Mot 4')
    const items = within(screen.getByTestId('picto-thread')).getAllByRole('listitem').map(li => li.dataset.status)
    expect(items).toEqual(['found', 'found', 'found', 'current'])
  })
  it('« Trouvé » et « Passer » envoient l\'ÉPOQUE du mot affiché', () => {
    const a = mkActions()
    mount(gs(), a)
    fireEvent.click(screen.getByTestId('picto-rafale-found'))
    fireEvent.click(screen.getByTestId('picto-rafale-pass'))
    expect(a.sent).toEqual([['PICTO_FOUND', { EPOCH: 5 }], ['PICTO_PASS', { EPOCH: 5 }]])
  })
  it('« Passer » grisé quand il ne reste aucune passe, et hors STARTED ; « Trouvé » reste actif en pause', () => {
    const { unmount } = mount(gs({ PICTO_PASSES_LEFT: 0 }))
    expect(screen.getByTestId('picto-rafale-pass').disabled).toBe(true)
    expect(screen.getByTestId('picto-rafale-found').disabled).toBe(false)
    unmount()
    mount(gs({ phase: 'PAUSED' }))
    expect(screen.getByTestId('picto-rafale-pass').disabled).toBe(true)
    expect(screen.getByTestId('picto-rafale-found').disabled).toBe(false)
  })
  it('temps écoulé : « Trouvé sur le gong » compte le mot ; sinon « Révéler »', () => {
    const a = mkActions(); const onReveal = vi.fn()
    mount(gs({ phase: 'STOPPED', PICTO_OUTCOME: 'TIMEOUT' }), a, WORD, variant === 'anim' ? { onReveal } : {})
    expect(screen.getByTestId('picto-rafale-gong')).toBeTruthy()
    expect(screen.getByTestId('picto-rafale-pass').disabled).toBe(true)
    fireEvent.click(screen.getByTestId('picto-rafale-found'))
    expect(a.sent).toEqual([['PICTO_FOUND', { EPOCH: 5 }]])
    expect(screen.getByTestId('picto-rafale-found').textContent).toContain('sur le gong')
  })
  it('fin du pool : message dédié, pas de « Trouvé »/« Passer » proposés', () => {
    mount(gs({ phase: 'STOPPED', PICTO_OUTCOME: 'POOL_END' }))
    expect(screen.getByTestId('picto-rafale-poolend')).toBeTruthy()
    expect(screen.getByTestId('picto-rafale-found').disabled).toBe(true)
  })
  it('révélation : mot révélé, récapitulatif avec la formule du crédit, plus de boutons', () => {
    mount(gs({ phase: 'REVEALED', PICTO_REVEALED_WORD: 'Maison', PICTO_OUTCOME: 'TIMEOUT' }))
    expect(screen.getByTestId('picto-rafale-word').textContent).toBe('Maison')
    expect(screen.getByTestId('picto-rafale-recap').textContent).toContain('3 × 10 − 1 × 5 = 25')
    expect(screen.queryByTestId('picto-rafale-found')).toBeNull()
    expect(screen.queryByTestId('picto-rafale-pass')).toBeNull()
  })
  it('révélation après fin de pool : aucun mot révélé (« — »)', () => {
    mount(gs({ phase: 'REVEALED', PICTO_REVEALED_WORD: '', PICTO_OUTCOME: 'POOL_END' }))
    expect(screen.getByTestId('picto-rafale-word').textContent).toBe('—')
    expect(screen.getByTestId('picto-rafale-recap').textContent).toContain('Fin du pool')
  })
})

describe('Fil des mots (admin et anim)', () => {
  const step = (rerender, g, w) => rerender(<PictoPanel gameState={g} actions={mkActions()} teamList={teamList} pictoWord={w} customCategories={CATS} />)
  it('trouvés ✅, passés ⏭, en cours ✏️ ; aucun mot à venir ; le mot passé reste visible seulement dans ce fil', () => {
    const { rerender } = admin(gs({ PICTO_FOUND_COUNT: 0, PICTO_FOUND_WORDS: [], PICTO_PASS_COUNT: 0, PICTO_CANVAS_EPOCH: 1 }), mkActions(), { WORD: 'Chat', EPOCH: 1 })
    step(rerender, gs({ PICTO_FOUND_COUNT: 1, PICTO_FOUND_WORDS: ['Chat'], PICTO_PASS_COUNT: 0, PICTO_CANVAS_EPOCH: 2 }), { WORD: 'Soleil', EPOCH: 2 })
    step(rerender, gs({ PICTO_FOUND_COUNT: 1, PICTO_FOUND_WORDS: ['Chat'], PICTO_PASS_COUNT: 1, PICTO_CANVAS_EPOCH: 3 }), { WORD: 'Girafe', EPOCH: 3 })
    step(rerender, gs({ PICTO_FOUND_COUNT: 2, PICTO_FOUND_WORDS: ['Chat', 'Girafe'], PICTO_PASS_COUNT: 1, PICTO_CANVAS_EPOCH: 4 }), { WORD: 'Maison', EPOCH: 4 })
    const items = within(screen.getByTestId('picto-thread')).getAllByRole('listitem').map(li => [li.dataset.status, li.textContent])
    expect(items.map(i => i[0])).toEqual(['found', 'passed', 'found', 'current'])
    expect(items[0][1]).toContain('1 · ✅ Chat')
    expect(items[1][1]).toContain('2 · ⏭ Soleil')
    expect(items[3][1]).toContain('4 · ✏️ Maison')
  })
  it('révélation : le mot en cours devient ⌛', () => {
    const { rerender } = admin(gs({ PICTO_FOUND_COUNT: 0, PICTO_FOUND_WORDS: [], PICTO_PASS_COUNT: 0, PICTO_CANVAS_EPOCH: 1 }), mkActions(), { WORD: 'Chat', EPOCH: 1 })
    step(rerender, gs({ phase: 'REVEALED', PICTO_FOUND_COUNT: 0, PICTO_FOUND_WORDS: [], PICTO_PASS_COUNT: 0, PICTO_CANVAS_EPOCH: 1, PICTO_REVEALED_WORD: 'Chat' }), { WORD: 'Chat', EPOCH: 1 })
    expect(within(screen.getByTestId('picto-thread')).getByRole('listitem').dataset.status).toBe('revealed')
  })
  it('nouvelle question ou relance (retour en préparation) : fil vide', () => {
    const { rerender } = admin(gs({ PICTO_FOUND_COUNT: 0, PICTO_FOUND_WORDS: [], PICTO_PASS_COUNT: 0, PICTO_CANVAS_EPOCH: 1 }), mkActions(), { WORD: 'Chat', EPOCH: 1 })
    expect(screen.getByTestId('picto-thread')).toBeTruthy()
    step(rerender, gs({ phase: 'READY', PICTO_FOUND_COUNT: 0, PICTO_FOUND_WORDS: [], PICTO_PASS_COUNT: 0 }), null)
    expect(screen.queryByTestId('picto-thread')).toBeNull()
  })
  it('le mot d\'une autre époque n\'entre jamais dans le fil (PICTO_WORD en retard sur l\'UPDATE)', () => {
    admin(gs({ PICTO_CANVAS_EPOCH: 5, PICTO_FOUND_COUNT: 0, PICTO_FOUND_WORDS: [], PICTO_PASS_COUNT: 0 }), mkActions(), { WORD: 'Ancien', EPOCH: 4 })
    expect(screen.queryByTestId('picto-thread')).toBeNull()
    expect(screen.getByTestId('picto-rafale-word').textContent).toBe('—')
  })
})

describe('Préparation et type Picto un mot', () => {
  it('préparation : « N mots disponibles, environ M nécessaires pour 90 s » ; alerte si insuffisant', () => {
    admin(gs({ phase: 'READY', PICTO_POOL_AVAILABLE: 42 }), mkActions(), null)
    expect(screen.getByTestId('picto-rafale-prep-hint').textContent).toContain('42 mots disponibles, environ 9 nécessaires pour 90 s')
    expect(screen.getByTestId('picto-rafale-prep-hint').className).not.toContain('picto-err')
  })
  it('pool insuffisant : alerte rouge, démarrage non bloqué par le front', () => {
    admin(gs({ phase: 'READY', PICTO_POOL_AVAILABLE: 4 }), mkActions(), null)
    expect(screen.getByTestId('picto-rafale-prep-hint').className).toContain('picto-err')
  })
  it('le type Picto (un mot) n\'a ni « Passer » ni fil ni compteurs', () => {
    admin(gs({ question: { ...Q, TYPE: 'PICTO' } }))
    expect(screen.queryByTestId('picto-rafale')).toBeNull()
    expect(screen.queryByTestId('picto-rafale-pass')).toBeNull()
    expect(screen.getByTestId('picto-found')).toBeTruthy()
  })
  it('« Trouvé » du type Picto envoie un payload vide (EPOCH facultatif, pas l\'événement du clic)', () => {
    const a = mkActions()
    admin(gs({ question: { ...Q, TYPE: 'PICTO' } }), a)
    fireEvent.click(screen.getByTestId('picto-found'))
    expect(a.sent).toEqual([['PICTO_FOUND', {}]])
  })
  it('titre et source des mots propres à Rafale Picto', () => {
    admin(gs())
    expect(screen.getByTestId('picto-panel').textContent).toContain('🖍️ Rafale Picto — manche en cours')
    expect(document.body.textContent).toContain('Pool Picto')
  })
})
