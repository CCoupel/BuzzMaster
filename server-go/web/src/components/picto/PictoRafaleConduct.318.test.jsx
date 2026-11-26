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
    expect(st).toContain('✅ 3')
    expect(st).toContain('⏭ 1')
    expect(st).toContain('38 restants')
    expect(st).toContain('en cours')
    expect(screen.getByTestId('picto-rafale-credit').textContent).toContain('25 pts')
    // formule du crédit en info-bulle SEULEMENT : un seul montant visible
    expect(screen.getByTestId('picto-rafale-credit').title).toBe('Crédit suggéré : 3 × 10 − 1 × 5 = 25')
    expect(document.body.textContent.match(/25 pts/g)).toHaveLength(1)
    expect(document.body.textContent).not.toContain('3 × 10')
    expect(document.body.textContent).not.toMatch(/passe · illimitées|\/ passe/)
    expect(screen.getByTestId('picto-rafale').className).toContain(`picto-rafale-${variant}`)
  })
  it('mot en cours en grand sur une ligne « Mot : … » ; aucune ligne « Pool · catégorie · mots tirés »', () => {
    mount(gs({ PICTO_FOUND_COUNT: 0, PICTO_FOUND_WORDS: [], PICTO_PASS_COUNT: 0 }))
    expect(screen.getByTestId('picto-rafale-word').textContent).toBe('Maison')
    expect(document.body.textContent).toContain('Mot :')
    expect(document.body.textContent).not.toMatch(/Mot 1/)
    if (variant === 'admin') expect(document.body.textContent).not.toMatch(/mots tirés|Pool Picto ·|disponibles/)
    expect(screen.queryByTestId('picto-rafale-nth')).toBeNull() // première présentation : rien
  })
  it('rechargement en cours de manche : les 3 mots trouvés (publics) reconstituent le fil ; le mot en cours est sur sa ligne', () => {
    mount(gs())
    const items = within(screen.getByTestId('picto-thread')).getAllByRole('listitem').map(li => [li.dataset.status, li.textContent])
    expect(items).toEqual([['found', '✅ Chat'], ['found', '✅ Soleil'], ['found', '✅ Vélo']])
    expect(screen.getByTestId('picto-rafale-word').textContent).toBe('Maison')
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
    expect(screen.getByTestId('picto-rafale-word').closest('.picto-revealed-chip')).not.toBeNull() // pastille distincte
    expect(document.body.textContent).toContain('Mot révélé :')
    expect(screen.getByTestId('picto-rafale-recap-help').textContent).toBe('→ crédit dans la colonne Équipes ⓘ')
    expect(screen.getByTestId('picto-rafale-recap-help').title).toMatch(/présélectionnée/)
    expect(screen.getByTestId('picto-rafale-credit').title).toContain('3 × 10 − 1 × 5 = 25') // formule : info-bulle seulement
    expect(document.body.textContent).not.toContain('3 × 10')
    expect(screen.queryByTestId('picto-rafale-found')).toBeNull()
    expect(screen.queryByTestId('picto-rafale-pass')).toBeNull()
  })
  it('révélation après fin de pool : aucun mot révélé (« — »)', () => {
    mount(gs({ phase: 'REVEALED', PICTO_REVEALED_WORD: '', PICTO_OUTCOME: 'POOL_END' }))
    expect(screen.queryByTestId('picto-rafale-word')).toBeNull()
    expect(screen.getByTestId('picto-rafale-recap').textContent).toContain('Fin de pool')
    expect(screen.getByTestId('picto-rafale-state').textContent).toBe('⛔ pool épuisé')
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
    // le mot en cours (Maison) est sur sa propre ligne, pas dans le fil ; aucun mot à venir
    // trouvés d'abord (ordre), puis passés regroupés
    expect(items.map(i => i[0])).toEqual(['found', 'found', 'passed'])
    expect(items.map(i => i[1])).toEqual(['✅ Chat', '✅ Girafe', '⏭ Soleil'])
    expect(screen.getByTestId('picto-rafale-word').textContent).toBe('Maison')
  })
  it('révélation : le mot en cours quitte le fil et devient le « mot révélé » (pastille)', () => {
    const { rerender } = admin(gs({ PICTO_FOUND_COUNT: 0, PICTO_FOUND_WORDS: [], PICTO_PASS_COUNT: 0, PICTO_CANVAS_EPOCH: 1 }), mkActions(), { WORD: 'Chat', EPOCH: 1 })
    step(rerender, gs({ phase: 'REVEALED', PICTO_FOUND_COUNT: 0, PICTO_FOUND_WORDS: [], PICTO_PASS_COUNT: 0, PICTO_CANVAS_EPOCH: 1, PICTO_REVEALED_WORD: 'Chat' }), { WORD: 'Chat', EPOCH: 1 })
    expect(screen.queryByTestId('picto-thread')).toBeNull() // fil vide : seul le mot révélé existait
    expect(screen.getByTestId('picto-rafale-word').closest('.picto-revealed-chip')).not.toBeNull()
  })
  it('nouvelle question ou relance (retour en préparation) : fil vide', () => {
    const { rerender } = admin(gs({ PICTO_FOUND_COUNT: 1, PICTO_FOUND_WORDS: ['Chat'], PICTO_PASS_COUNT: 0, PICTO_CANVAS_EPOCH: 1 }), mkActions(), { WORD: 'Soleil', EPOCH: 1 })
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
  it('manche en cours : le bandeau remplace l\'en-tête et la ligne de source (aucun doublon)', () => {
    admin(gs())
    const t = screen.getByTestId('picto-panel').textContent
    expect(t).toContain('🖍️ Rafale Picto')
    expect(t).not.toMatch(/manche en cours|mots tirés au fil/)
    expect(t.match(/Rafale Picto/g)).toHaveLength(1)
  })
  it('préparation : l\'en-tête et la ligne de source restent affichés', () => {
    admin(gs({ phase: 'READY' }), mkActions(), null)
    expect(screen.getByTestId('picto-panel').textContent).toContain('Rafale Picto — préparation')
  })
})

describe('Fin de pool : champ absent = END (picto.md §17.2)', () => {
  const { RAFALE_PICTO_POOL_END: _drop, ...Qabsent } = Q
  it('champ absent : pas de mention de rebouclage', () => {
    admin(gs({ question: Qabsent }))
    expect(screen.getByTestId('picto-thread').title).toBe('')
  })
  it('END explicite : pas de mention ; LOOP : mention', () => {
    const { unmount } = admin(gs({ question: { ...Q, RAFALE_PICTO_POOL_END: 'END' } }))
    expect(screen.getByTestId('picto-thread').title).toBe('')
    unmount()
    admin(gs({ question: { ...Q, RAFALE_PICTO_POOL_END: 'LOOP' } }))
    // texte du rebouclage en info-bulle, jamais en paragraphe
    expect(screen.getByTestId('picto-thread').title).toContain('rebouclage')
    expect(screen.getByTestId('picto-thread').textContent).not.toContain('rebouclage')
  })
})

describe('Fil des mots en ligne, couleur de l\'équipe (cycle 5)', () => {
  it('fil en rangée (liste flex wrap) avec le fond de la couleur de l\'équipe', () => {
    admin(gs({ PICTO_TEAM_COLOR: [0, 0, 255] }))
    const thread = screen.getByTestId('picto-thread')
    expect(thread.style.getPropertyValue('--thread-bg')).toBeTruthy()
    expect(thread.style.getPropertyValue('--thread-bg')).not.toBe('#475569')
    expect(thread.style.getPropertyValue('--thread-fg')).toBe('#ffffff') // contraste sur bleu
  })
  it('équipe claire : texte noir', () => {
    admin(gs({ PICTO_TEAM_COLOR: [250, 240, 100] }))
    expect(screen.getByTestId('picto-thread').style.getPropertyValue('--thread-fg')).toBe('#000000')
  })
  it('aucune couleur d\'équipe : fond neutre', () => {
    admin(gs({ PICTO_TEAM_COLOR: [] }))
    expect(screen.getByTestId('picto-thread').style.getPropertyValue('--thread-bg')).toBe('#475569')
  })
  it('le mot en cours est distinct (ligne « Mot : », en grand) ; même fil sur l\'anim', () => {
    anim(gs({ PICTO_TEAM_COLOR: [0, 0, 255] }))
    expect(screen.getByTestId('picto-rafale-word').className).toContain('picto-rafale-word')
    expect(screen.getByTestId('picto-rafale-word').textContent).toBe('Maison')
    expect(screen.getByTestId('picto-thread').style.getPropertyValue('--thread-bg')).toBeTruthy()
  })
  it('CSS : rangée flex wrap, pas de colonne', async () => {
    const fs = await import('node:fs')
    const css = fs.readFileSync('src/components/picto/PictoPanel.css', 'utf8')
    expect(css).toMatch(/\.picto-thread ol \{[^}]*flex-flow: row wrap/)
  })
})

describe('Maquette M9 A — regroupement des passés « ×n », bandeau, sans doublon (cycle 6)', () => {
  const step = (rerender, g, w) => rerender(<PictoPanel gameState={g} actions={mkActions()} teamList={teamList} pictoWord={w} customCategories={CATS} />)
  // 3 trouvés (Chat, Girafe, Zèbre), « Lion » passé 5 fois, « Tortue » 4 fois, « Lion » en cours (6ᵉ présentation)
  const play = () => {
    const seq = [['Chat', 'f'], ['Lion', 'p'], ['Tortue', 'p'], ['Lion', 'p'], ['Girafe', 'f'], ['Lion', 'p'], ['Tortue', 'p'], ['Lion', 'p'], ['Zèbre', 'f'], ['Lion', 'p'], ['Tortue', 'p'], ['Tortue', 'p']]
    let found = 0, pass = 0, epoch = 1
    const base = (g) => gs({ PICTO_FOUND_COUNT: found, PICTO_PASS_COUNT: pass, PICTO_FOUND_WORDS: [], PICTO_CANVAS_EPOCH: epoch, ...g })
    const r = admin(base(), mkActions(), { WORD: seq[0][0], EPOCH: 1 })
    for (let i = 1; i <= seq.length; i += 1) {
      if (seq[i - 1][1] === 'f') found += 1; else pass += 1
      epoch += 1
      step(r.rerender, base(), { WORD: i < seq.length ? seq[i][0] : 'Lion', EPOCH: epoch })
    }
    return r
  }
  it('passés dédoublonnés « ×n », trouvés un à un dans l\'ordre, mot en cours hors du fil', () => {
    play()
    const items = within(screen.getByTestId('picto-thread')).getAllByRole('listitem').map(li => li.textContent)
    expect(items).toEqual(['✅ Chat', '✅ Girafe', '✅ Zèbre', '⏭ Lion ×5', '⏭ Tortue ×4'])
    expect(screen.getByTestId('picto-rafale-word').textContent).toBe('Lion')
  })
  it('« (6ᵉ présentation) » : nombre de fois où le mot en cours a été présenté, lui compris', () => {
    play()
    expect(screen.getByTestId('picto-rafale-nth').textContent).toBe('(6ᵉ présentation)')
  })
  it('bandeau d\'une ligne EN TÊTE du bloc : avant le mot, les boutons et le fil', () => {
    admin(gs())
    const root = screen.getByTestId('picto-rafale')
    expect(root.firstElementChild).toBe(screen.getByTestId('picto-rafale-stats'))
    const pos = (el) => root.compareDocumentPosition(el)
    const banner = screen.getByTestId('picto-rafale-stats')
    ;['picto-rafale-word', 'picto-rafale-pass', 'picto-thread'].forEach(id => {
      expect(banner.compareDocumentPosition(screen.getByTestId(id)) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    })
    expect(pos(banner)).toBeTruthy()
  })
  it('un seul montant de crédit visible, quel que soit l\'état', () => {
    for (const g of [gs(), gs({ phase: 'REVEALED', PICTO_REVEALED_WORD: 'Maison', PICTO_OUTCOME: 'TIMEOUT' }), gs({ phase: 'STOPPED', PICTO_OUTCOME: 'POOL_END' })]) {
      const { unmount } = admin(g)
      expect(document.body.textContent.match(/25 pts/g)).toHaveLength(1)
      unmount()
    }
  })
  it('coût de passe nul : aucune mention « −0 pts » ; sur le bouton comme dans l\'info-bulle', () => {
    admin(gs({ question: { ...Q, RAFALE_PICTO_PASS_COST: 0 } }))
    expect(document.body.textContent).not.toMatch(/−0|-0 pts/)
    expect(screen.getByTestId('picto-rafale-pass').title).not.toMatch(/−0|0 pts/)
  })
  it('pool épuisé, 0 point : état, fil des passés, « rien n\'est coché », sans mot révélé', () => {
    const base = gs({ phase: 'REVEALED', PICTO_OUTCOME: 'POOL_END', PICTO_REVEALED_WORD: '', PICTO_ROUND_POINTS: 0, PICTO_FOUND_COUNT: 0, PICTO_FOUND_WORDS: [], PICTO_PASS_COUNT: 4, PICTO_POOL_AVAILABLE: 0, question: { ...Q, RAFALE_PICTO_POOL_END: 'END' } })
    admin(base)
    expect(screen.getByTestId('picto-rafale-state').textContent).toBe('⛔ pool épuisé')
    expect(screen.getByTestId('picto-rafale-stats').textContent).toContain('0 restant')
    expect(screen.getByTestId('picto-rafale-recap').textContent).toContain("crédit nul : rien n'est coché")
    expect(screen.getByTestId('picto-rafale-credit').textContent).toContain('0 pts')
  })
  it('libellés d\'état du bandeau', () => {
    const labels = [[gs(), 'en cours'], [gs({ phase: 'PAUSED' }), '⏸ pause'], [gs({ phase: 'STOPPED', PICTO_OUTCOME: 'TIMEOUT' }), '⏰ temps écoulé'], [gs({ phase: 'STOPPED', PICTO_OUTCOME: 'STOPPED' }), '⏹ arrêtée']]
    for (const [g, l] of labels) {
      const { unmount } = admin(g)
      expect(screen.getByTestId('picto-rafale-state').textContent).toBe(l)
      unmount()
    }
  })
  it('anim : même bandeau, fil regroupé et gros boutons ; Passer/Trouvé inchangés', () => {
    const a = mkActions()
    anim(gs({ PICTO_FOUND_COUNT: 0, PICTO_FOUND_WORDS: [], PICTO_PASS_COUNT: 0 }), a)
    expect(screen.getByTestId('picto-rafale').className).toContain('picto-rafale-anim')
    fireEvent.click(screen.getByTestId('picto-rafale-found'))
    expect(a.sent).toEqual([['PICTO_FOUND', { EPOCH: 5 }]])
  })
})
