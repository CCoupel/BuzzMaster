/**
 * #147 (v12.1.0, LOT-1.2) — achat de voyelles : spécification des écrans (contracts/roue.md §15.1 ; maquettes
 * docs/mockup/v12.1.0/ui/{admin_game_roue,anim_roue,tv_roue_started,admin_question_editor_roue}__vowels_bonus.html).
 * L'interface n'affiche que ce que dit le serveur (champs GameState ROUE_BUY_VOWELS … ROUE_STOP_VOWEL) ; le serveur reste seul juge.
 */
import { describe, it, expect, afterEach, vi } from 'vitest'
import { render, cleanup, fireEvent, screen } from '@testing-library/react'
import RouePanel from './RouePanel'
import RoueTv from './RoueTv'
import RoueEditor from './RoueEditor'
import { AnimRoueView } from './AnimRoue'
import { makeRoueActions, roueGates, roueAnnouncement } from './roueState'

afterEach(() => cleanup())

const SEGS = [{ ID: 's0', KIND: 'value', VALUE: 500 }, { ID: 's1', KIND: 'bankrupt', VALUE: 0 }, { ID: 's2', KIND: 'value', VALUE: 800, BONUS: true }]
const NO_EVENT = { SEQ: 0, KIND: '', TEAM: '', LETTER: '', COUNT: 0, VALUE: 0, GAIN: 0 }

/** Début de tour (TURN) d'une manche aux réglages par défaut d'une question v12.0 : Avant de tourner, Plusieurs, 250. */
const gs = (over = {}) => ({
  phase: 'STARTED',
  question: { TYPE: 'ROUE', ROUE_MODE: 'TANT_QUE_JE_GAGNE', ROUE_PHRASE: 'TOTO TATA', ROUE_PRESENTATION: 'Salutation' },
  ROUE_SUBPHASE: 'TURN', ROUE_STEP: 1, ROUE_TEAMS: ['Rouges', 'Bleus'], ROUE_CURRENT_TEAM: 'Rouges', ROUE_CURRENT_TEAM_COLOR: [239, 68, 68],
  ROUE_NEXT_TEAM: '', ROUE_NEXT_DEFAULT: '', ROUE_TEAM_BANK: { Rouges: 1000, Bleus: 300 }, ROUE_TURN_GAIN: 0,
  ROUE_BOARD: { COLS: 14, ROWS: ['____ ____'] }, ROUE_LETTERS: [], ROUE_SHOW_LETTERS: true, ROUE_CONSONANTS_LEFT: true,
  ROUE_BUY_VOWELS: true, ROUE_VOWEL_TIMING: 'BEFORE', ROUE_VOWELS_BEFORE: 'MANY', ROUE_VOWELS_AFTER: 'MANY',
  ROUE_VOWEL_PRICE: 250, ROUE_VOWEL_GAIN: 'NONE', ROUE_VOWELS_LEFT: true, ROUE_TURN_VOWEL: false, ROUE_STOP_VOWEL: false,
  ROUE_WHEEL: { LAYOUT: 'strip', SPIN_PERIOD_MS: 2500, SPIN_MAX_MS: 10000, FLIP_INTERVAL_MS: 600, CURRENT: 0, STOP: null, SEGMENTS: SEGS },
  ROUE_LAST_EVENT: NO_EVENT, ROUE_WINNER: '', ROUE_AWARDS: {},
  ...over,
})
const teams = { Rouges: { COLOR: [239, 68, 68] }, Bleus: { COLOR: [59, 130, 246] } }
const actionsMock = () => ({ setTeams: vi.fn(), spin: vi.fn(), stop: vi.fn(), letter: vi.fn(), flip: vi.fn(), answer: vi.fn(), next: vi.fn(), pass: vi.fn(), reveal: vi.fn(), buyVowel: vi.fn() })
const btnLike = (re) => screen.queryAllByRole('button').filter(b => re.test(b.textContent))

// ---- action et portes ---------------------------------------------------------------------------------------------

describe('Action ROUE_BUY_VOWEL et portes (roueGates)', () => {
  it('buyVowel envoie ROUE_BUY_VOWEL { LETTER }', () => {
    const send = vi.fn()
    makeRoueActions(send).buyVowel('E')
    expect(send).toHaveBeenCalledWith('ROUE_BUY_VOWEL', { LETTER: 'E' })
  })

  it('critical — question v12.0 (réglages par défaut) : achat avant de tourner ouvert au TURN, pas après l\'arrêt', () => {
    const g = roueGates(gs())
    expect(g.canBuyBefore).toBe(true)
    expect(g.canBuyAfter).toBe(false)
  })

  const table = [
    // [nom, gs, canBuyBefore, canBuyAfter]
    ['achat désactivé', gs({ ROUE_BUY_VOWELS: false }), false, false],
    ['AFTER au TURN', gs({ ROUE_VOWEL_TIMING: 'AFTER' }), false, false],
    ['BOTH au TURN', gs({ ROUE_VOWEL_TIMING: 'BOTH' }), true, false],
    ['cagnotte < prix', gs({ ROUE_TEAM_BANK: { Rouges: 100, Bleus: 0 } }), false, false],
    ['cagnotte = prix', gs({ ROUE_TEAM_BANK: { Rouges: 250, Bleus: 0 } }), true, false],
    ['plus de voyelle', gs({ ROUE_VOWELS_LEFT: false }), false, false],
    ['ONE avant, déjà une voyelle', gs({ ROUE_VOWELS_BEFORE: 'ONE', ROUE_TURN_VOWEL: true }), false, false],
    ['MANY avant, déjà une voyelle', gs({ ROUE_TURN_VOWEL: true }), true, false],
    ['ONE avant, aucune voyelle', gs({ ROUE_VOWELS_BEFORE: 'ONE' }), true, false],
    ['LETTER avec BEFORE', gs({ ROUE_SUBPHASE: 'LETTER', ROUE_STEP: 4 }), false, false],
    ['LETTER avec AFTER', gs({ ROUE_SUBPHASE: 'LETTER', ROUE_STEP: 4, ROUE_VOWEL_TIMING: 'AFTER' }), false, true],
    ['LETTER avec BOTH', gs({ ROUE_SUBPHASE: 'LETTER', ROUE_STEP: 4, ROUE_VOWEL_TIMING: 'BOTH' }), false, true],
    ['LETTER AFTER, cagnotte < prix', gs({ ROUE_SUBPHASE: 'LETTER', ROUE_STEP: 4, ROUE_VOWEL_TIMING: 'AFTER', ROUE_TEAM_BANK: { Rouges: 10, Bleus: 0 } }), false, false],
    ['ANSWER après une consonne (BOTH)', gs({ ROUE_SUBPHASE: 'ANSWER', ROUE_STEP: 6, ROUE_VOWEL_TIMING: 'BOTH' }), false, false],
    ['ANSWER après voyelle, AFTER + MANY', gs({ ROUE_SUBPHASE: 'ANSWER', ROUE_STEP: 6, ROUE_VOWEL_TIMING: 'AFTER', ROUE_STOP_VOWEL: true }), false, true],
    ['ANSWER après voyelle, AFTER + ONE', gs({ ROUE_SUBPHASE: 'ANSWER', ROUE_STEP: 6, ROUE_VOWEL_TIMING: 'AFTER', ROUE_VOWELS_AFTER: 'ONE', ROUE_STOP_VOWEL: true }), false, false],
    ['SPINNING', gs({ ROUE_SUBPHASE: 'SPINNING', ROUE_STEP: 2 }), false, false],
    ['TURN_END', gs({ ROUE_SUBPHASE: 'TURN_END', ROUE_STEP: 7 }), false, false],
  ]
  it.each(table)('%s', (_name, state, before, after) => {
    const g = roueGates(state)
    expect(!!g.canBuyBefore).toBe(before)
    expect(!!g.canBuyAfter).toBe(after)
  })

  it('« Réponse validée » ouverte au TURN seulement après une voyelle présente (ROUE_TURN_VOWEL)', () => {
    expect(roueGates(gs()).canAnswer).toBe(false)
    expect(roueGates(gs({ ROUE_TURN_VOWEL: true })).canAnswer).toBe(true)
  })

  it('« Retourner les cases » actif en VOWEL_LIT comme en LIT', () => {
    expect(roueGates(gs({ ROUE_SUBPHASE: 'VOWEL_LIT' })).canFlip).toBe(true)
    expect(roueGates(gs({ ROUE_SUBPHASE: 'LIT' })).canFlip).toBe(true)
    expect(roueGates(gs({ ROUE_SUBPHASE: 'VOWEL_REVEALING' })).canFlip).toBe(false)
  })

  it('sans consonne restante : la roue reste lançable seulement si un achat après l\'arrêt est possible', () => {
    const none = { ROUE_CONSONANTS_LEFT: false }
    expect(roueGates(gs({ ...none, ROUE_VOWEL_TIMING: 'AFTER' })).canSpin).toBe(true)
    expect(roueGates(gs({ ...none, ROUE_VOWEL_TIMING: 'BOTH' })).canSpin).toBe(true)
    expect(roueGates(gs({ ...none })).canSpin).toBe(false) // BEFORE : comme v12.0
    expect(roueGates(gs({ ...none, ROUE_VOWEL_TIMING: 'AFTER', ROUE_TEAM_BANK: { Rouges: 10, Bleus: 0 } })).canSpin).toBe(false)
    expect(roueGates(gs({ ...none, ROUE_VOWEL_TIMING: 'AFTER', ROUE_BUY_VOWELS: false })).canSpin).toBe(false)
  })

  it('un GameState sans les champs voyelle (serveur v12.0) : aucun achat, comportement v12.0', () => {
    const old = gs()
    for (const k of ['ROUE_BUY_VOWELS', 'ROUE_VOWEL_TIMING', 'ROUE_VOWELS_BEFORE', 'ROUE_VOWELS_AFTER', 'ROUE_VOWEL_PRICE', 'ROUE_VOWEL_GAIN', 'ROUE_VOWELS_LEFT', 'ROUE_TURN_VOWEL', 'ROUE_STOP_VOWEL']) delete old[k]
    const g = roueGates(old)
    expect(g.canBuyBefore).toBe(false)
    expect(g.canBuyAfter).toBe(false)
    expect(g.canSpin).toBe(true)
  })
})

describe('Annonces TV / tablette', () => {
  const text = (parts) => parts.map(p => p.strong ?? p.text).join('')
  // Événements réels du moteur : VOWEL_HIT { VALUE = prix, GAIN = gain à venir } puis FLIP { COUNT, VALUE = 0, GAIN = gain crédité } (roue.go).
  const hit = (gain) => ({ ...NO_EVENT, SEQ: 4, KIND: 'VOWEL_HIT', TEAM: 'Rouges', LETTER: 'O', COUNT: 2, VALUE: 250, GAIN: gain })
  const flip = (gain, letter = 'O', count = 2) => ({ ...NO_EVENT, SEQ: 5, KIND: 'FLIP', TEAM: 'Rouges', LETTER: letter, COUNT: count, VALUE: 0, GAIN: gain })
  const rev = (ev, sub = 'VOWEL_REVEALING', over = {}) => text(roueAnnouncement(gs({ ROUE_SUBPHASE: sub, ROUE_LAST_EVENT: ev, ...over })))

  it('VOWEL_LIT annonce la lettre et les cases éclairées', () => {
    expect(text(roueAnnouncement(gs({ ROUE_SUBPHASE: 'VOWEL_LIT', ROUE_LAST_EVENT: hit(500) })))).toMatch(/O.*2 cases/)
  })

  it('FLIP réel (VALUE 0) d\'une voyelle avec gain : « O : 2 cases = +500 », jamais « × 0 »', () => {
    const t = rev(flip(500))
    expect(t).toMatch(/O : 2 cases = \+500/)
    expect(t).not.toMatch(/×/)
  })

  it('FLIP réel d\'une voyelle sans gain (NONE) : aucun « × 0 » ni « +0 »', () => {
    const t = rev(flip(0))
    expect(t).toMatch(/O/)
    expect(t).toMatch(/2 cases/)
    expect(t).not.toMatch(/× 0|\+\s?0/)
  })

  it('même annonce après l\'arrêt (sous-phase REVEALING, lettre voyelle)', () => {
    expect(rev(flip(250), 'REVEALING')).toMatch(/O : 2 cases = \+250/)
    expect(rev(flip(0), 'REVEALING')).not.toMatch(/× 0|\+\s?0/)
  })

  it('FLIP réel d\'une consonne : valeur de la case × cases = gain du tour en attente, jamais « × 0 = +0 »', () => {
    const t = rev(flip(0, 'T', 3), 'REVEALING', { ROUE_TURN_GAIN: 1500, ROUE_WHEEL: { ...gs().ROUE_WHEEL, CURRENT: 0 } })
    expect(t).toMatch(/T : 3 × 500 = \+1.?500/)
    expect(t).not.toMatch(/× 0|= \+0/)
  })
})

// ---- admin ------------------------------------------------------------------------------------------------------

describe('RouePanel (admin) — achat de voyelles', () => {
  it('étape 1 : « Lancer la roue » et « Acheter une voyelle — 250 » ; achat actif', () => {
    render(<RouePanel gameState={gs()} actions={actionsMock()} />)
    expect(btnLike(/Lancer la roue/).length).toBe(1)
    const buy = btnLike(/Acheter une voyelle/)
    expect(buy.length).toBe(1)
    expect(buy[0].textContent).toMatch(/250/)
    expect(buy[0].disabled).toBe(false)
  })

  it('étape 1 : choisir une voyelle puis « Acheter le O — 250 » envoie ROUE_BUY_VOWEL O', () => {
    const a = actionsMock()
    render(<RouePanel gameState={gs()} actions={a} />)
    fireEvent.click(btnLike(/Acheter une voyelle/)[0])
    fireEvent.click(screen.getByRole('button', { name: 'O' }))
    fireEvent.click(screen.getByRole('button', { name: /Acheter le O/ }))
    expect(a.buyVowel).toHaveBeenCalledTimes(1)
    expect(a.buyVowel).toHaveBeenCalledWith('O')
    expect(a.spin).not.toHaveBeenCalled()
  })

  it('étape 1 : bouton grisé si la cagnotte est inférieure au prix', () => {
    render(<RouePanel gameState={gs({ ROUE_TEAM_BANK: { Rouges: 100, Bleus: 0 } })} actions={actionsMock()} />)
    const buy = btnLike(/Acheter une voyelle/)[0]
    expect(buy.disabled).toBe(true)
  })

  it('étape 1 : « Plus de voyelle à trouver » grisé quand toutes les voyelles sont dites', () => {
    render(<RouePanel gameState={gs({ ROUE_VOWELS_LEFT: false })} actions={actionsMock()} />)
    const b = btnLike(/Plus de voyelle à trouver/)[0]
    expect(b).toBeTruthy()
    expect(b.disabled).toBe(true)
  })

  it('étape 1 avec « 1 voyelle » : après une voyelle, l\'achat est grisé (1 voyelle par tour)', () => {
    render(<RouePanel gameState={gs({ ROUE_VOWELS_BEFORE: 'ONE', ROUE_TURN_VOWEL: true })} actions={actionsMock()} />)
    const b = btnLike(/1 voyelle par tour/)[0]
    expect(b).toBeTruthy()
    expect(b.disabled).toBe(true)
  })

  it('étape 1 après une voyelle présente : « Réponse validée » proposée et envoie ROUE_ANSWER', () => {
    const a = actionsMock()
    render(<RouePanel gameState={gs({ ROUE_TURN_VOWEL: true })} actions={a} />)
    fireEvent.click(btnLike(/Réponse validée/)[0])
    expect(a.answer).toHaveBeenCalledTimes(1)
  })

  it('étape 1 sans voyelle achetée : pas de « Réponse validée »', () => {
    render(<RouePanel gameState={gs()} actions={actionsMock()} />)
    expect(btnLike(/Réponse validée/).length).toBe(0)
  })

  it('achat désactivé ou moment AFTER : aucun bouton « Acheter une voyelle » à l\'étape 1', () => {
    const { unmount } = render(<RouePanel gameState={gs({ ROUE_BUY_VOWELS: false })} actions={actionsMock()} />)
    expect(btnLike(/Acheter une voyelle/).length).toBe(0)
    unmount()
    render(<RouePanel gameState={gs({ ROUE_VOWEL_TIMING: 'AFTER' })} actions={actionsMock()} />)
    expect(btnLike(/Acheter une voyelle/).length).toBe(0)
  })

  it('VOWEL_LIT : « Retourner les cases » envoie ROUE_FLIP ; VOWEL_REVEALING : aucun geste', () => {
    const a = actionsMock()
    const ev = { ...NO_EVENT, SEQ: 4, KIND: 'VOWEL_HIT', LETTER: 'O', COUNT: 2, VALUE: 250, GAIN: 0 }
    const { unmount } = render(<RouePanel gameState={gs({ ROUE_SUBPHASE: 'VOWEL_LIT', ROUE_LAST_EVENT: ev, ROUE_LETTERS: [{ LETTER: 'O', HIT: true }] })} actions={a} />)
    fireEvent.click(btnLike(/Retourner/)[0])
    expect(a.flip).toHaveBeenCalledTimes(1)
    unmount()
    render(<RouePanel gameState={gs({ ROUE_SUBPHASE: 'VOWEL_REVEALING', ROUE_LAST_EVENT: { ...ev, KIND: 'FLIP', VALUE: 0, GAIN: 0 }, ROUE_LETTERS: [{ LETTER: 'O', HIT: true }] })} actions={a} />)
    expect(btnLike(/Retourner/).filter(b => !b.disabled).length).toBe(0)
  })

  it('étape 4 avec AFTER : le titre propose « Consonne ou voyelle » ; avec BEFORE : « Sélection de la consonne »', () => {
    const letter = { ROUE_SUBPHASE: 'LETTER', ROUE_STEP: 4 }
    const { container, unmount } = render(<RouePanel gameState={gs({ ...letter, ROUE_VOWEL_TIMING: 'AFTER' })} actions={actionsMock()} />)
    expect(container.textContent).toMatch(/Consonne ou voyelle/)
    unmount()
    const r = render(<RouePanel gameState={gs(letter)} actions={actionsMock()} />)
    expect(r.container.textContent).toMatch(/Sélection de la consonne/)
    expect(r.container.textContent).not.toMatch(/Consonne ou voyelle/)
  })

  it('étape 4 AFTER : choisir une voyelle envoie ROUE_BUY_VOWEL (jamais ROUE_LETTER)', () => {
    const a = actionsMock()
    render(<RouePanel gameState={gs({ ROUE_SUBPHASE: 'LETTER', ROUE_STEP: 4, ROUE_VOWEL_TIMING: 'AFTER' })} actions={a} />)
    fireEvent.click(screen.getByRole('button', { name: 'A' }))
    const validate = btnLike(/Acheter le A|Valider la lettre A/)[0]
    expect(validate).toBeTruthy()
    fireEvent.click(validate)
    expect(a.buyVowel).toHaveBeenCalledWith('A')
    expect(a.letter).not.toHaveBeenCalled()
  })

  it('étape 6 après voyelle (AFTER + Plusieurs) : « ou une autre voyelle » ; avec « 1 » : grisé « déjà achetée »', () => {
    const answer = { ROUE_SUBPHASE: 'ANSWER', ROUE_STEP: 6, ROUE_VOWEL_TIMING: 'AFTER', ROUE_STOP_VOWEL: true, ROUE_LETTERS: [{ LETTER: 'O', HIT: true }] }
    const many = render(<RouePanel gameState={gs(answer)} actions={actionsMock()} />)
    expect(many.container.textContent).toMatch(/autre voyelle/)
    many.unmount()
    const one = render(<RouePanel gameState={gs({ ...answer, ROUE_VOWELS_AFTER: 'ONE' })} actions={actionsMock()} />)
    const b = btnLike(/1 voyelle par arrêt/)[0]
    expect(b).toBeTruthy()
    expect(b.disabled).toBe(true)
    expect(one.container.textContent).not.toMatch(/ou une autre voyelle/)
  })

  it('étape 6 après une consonne : aucune proposition d\'acheter une voyelle', () => {
    const { container } = render(<RouePanel gameState={gs({ ROUE_SUBPHASE: 'ANSWER', ROUE_STEP: 6, ROUE_VOWEL_TIMING: 'BOTH', ROUE_LETTERS: [{ LETTER: 'T', HIT: true }] })} actions={actionsMock()} />)
    expect(container.textContent).not.toMatch(/autre voyelle/)
  })

  it('voyelles déjà proposées : touches grisées dans le clavier d\'achat', () => {
    render(<RouePanel gameState={gs({ ROUE_LETTERS: [{ LETTER: 'O', HIT: true }, { LETTER: 'E', HIT: false }] })} actions={actionsMock()} />)
    fireEvent.click(btnLike(/Acheter une voyelle/)[0])
    expect(screen.getByRole('button', { name: 'O' }).disabled).toBe(true)
    expect(screen.getByRole('button', { name: 'E' }).disabled).toBe(true)
    expect(screen.getByRole('button', { name: 'A' }).disabled).toBe(false)
  })

  it('voyelle absente : la fin de tour dit « payée, tour perdu »', () => {
    const { container } = render(<RouePanel gameState={gs({
      ROUE_SUBPHASE: 'TURN_END', ROUE_STEP: 7, ROUE_LETTERS: [{ LETTER: 'E', HIT: false }],
      ROUE_LAST_EVENT: { ...NO_EVENT, SEQ: 5, KIND: 'VOWEL_MISS', TEAM: 'Rouges', LETTER: 'E', VALUE: 250 },
    })} actions={actionsMock()} />)
    expect(container.textContent).toMatch(/E.*absente/)
  })
})

// ---- animateur --------------------------------------------------------------------------------------------------

describe('AnimRoue (animateur) — achat de voyelles', () => {
  const view = (over) => { const a = actionsMock(); const r = render(<AnimRoueView gameState={gs(over)} teams={teams} actions={a} />); return { a, ...r } }
  const buy = (c) => c.querySelector('.anim-roue-buy')

  it('étape 1 : bouton d\'achat visible et sans voyelle choisie, grisé', () => {
    const { container } = view()
    expect(buy(container)).toBeTruthy()
    expect(buy(container).disabled).toBe(true)
  })

  it('étape 1 : choisir O puis acheter envoie ROUE_BUY_VOWEL O', () => {
    const { container, a } = view()
    fireEvent.click(screen.getByRole('button', { name: /^O$/ }))
    expect(buy(container).disabled).toBe(false)
    expect(buy(container).textContent).toMatch(/Acheter le O.*250/)
    fireEvent.click(buy(container))
    expect(a.buyVowel).toHaveBeenCalledWith('O')
  })

  it('aucun bouton d\'achat quand l\'achat est désactivé', () => {
    const { container } = view({ ROUE_BUY_VOWELS: false })
    expect(buy(container)).toBeNull()
  })

  it('moment AFTER : pas d\'achat à l\'étape 1', () => {
    const { container } = view({ ROUE_VOWEL_TIMING: 'AFTER' })
    expect(buy(container)).toBeNull()
  })

  it('cagnotte insuffisante : bouton grisé et libellé « Cagnotte < 250 »', () => {
    const { container } = view({ ROUE_TEAM_BANK: { Rouges: 100, Bleus: 0 } })
    expect(buy(container).disabled).toBe(true)
    expect(buy(container).textContent).toMatch(/Cagnotte < 250/)
  })

  it('« Plus de voyelle » quand toutes les voyelles sont dites', () => {
    const { container } = view({ ROUE_VOWELS_LEFT: false })
    expect(buy(container).textContent).toMatch(/Plus de voyelle/)
    expect(buy(container).disabled).toBe(true)
  })

  it('étape 6 après une voyelle (AFTER + Plusieurs) : « Autre voyelle » ; avec « 1 » : « 1 voyelle à ce moment » grisé', () => {
    const answer = { ROUE_SUBPHASE: 'ANSWER', ROUE_STEP: 6, ROUE_VOWEL_TIMING: 'AFTER', ROUE_STOP_VOWEL: true, ROUE_LETTERS: [{ LETTER: 'O', HIT: true }] }
    const many = view(answer)
    expect(buy(many.container).textContent).toMatch(/Autre voyelle/)
    many.unmount()
    const one = view({ ...answer, ROUE_VOWELS_AFTER: 'ONE' })
    expect(buy(one.container).textContent).toMatch(/1 voyelle à ce moment/)
    expect(buy(one.container).disabled).toBe(true)
  })

  it('« Réponse validée » active à l\'étape 1 après une voyelle présente', () => {
    const { container } = view({ ROUE_TURN_VOWEL: true })
    const enabled = Array.from(container.querySelectorAll('.anim-roue-steps button')).filter(b => !b.disabled).map(b => b.textContent)
    expect(enabled.some(t => /Réponse/.test(t))).toBe(true)
    expect(enabled.some(t => /Lancer/.test(t))).toBe(true)
  })
})

// ---- TV et tablette ---------------------------------------------------------------------------------------------

describe('RoueTv — voyelles', () => {
  it('les voyelles proposées figurent parmi les lettres affichées, absentes marquées ; sans les masquer', () => {
    const { container } = render(<RoueTv gameState={gs({ ROUE_LETTERS: [{ LETTER: 'O', HIT: true }, { LETTER: 'E', HIT: false }] })} teams={teams} />)
    const letters = Array.from(container.querySelectorAll('.roue-letters b'))
    expect(letters.map(b => b.textContent)).toEqual(expect.arrayContaining(['O', 'E']))
    expect(letters.find(b => b.textContent === 'E').className).toMatch(/miss/)
    expect(letters.find(b => b.textContent === 'O').className).not.toMatch(/miss/)
  })

  it('la phrase reste secrète pendant un achat (§9)', () => {
    const ev = { ...NO_EVENT, SEQ: 4, KIND: 'VOWEL_HIT', LETTER: 'O', COUNT: 2, VALUE: 250, GAIN: 0 }
    const { container } = render(<RoueTv gameState={gs({ ROUE_SUBPHASE: 'VOWEL_LIT', ROUE_LAST_EVENT: ev, ROUE_LETTERS: [{ LETTER: 'O', HIT: true }] })} teams={teams} />)
    expect(container.textContent).not.toMatch(/TOTO TATA/)
  })

  it('la cagnotte affichée est celle du serveur (débitée), aucun calcul local', () => {
    const { container } = render(<RoueTv gameState={gs({ ROUE_TEAM_BANK: { Rouges: 750, Bleus: 300 } })} teams={teams} />)
    expect(container.textContent).toMatch(/750/)
  })
})

// ---- éditeur ----------------------------------------------------------------------------------------------------

const baseValues = { rouePhrase: '', rouePresentation: '', roueMode: 'TANT_QUE_JE_GAGNE', roueGainsRule: 'WINNER_ONLY', roueWheelLayout: 'strip', roueShowLetters: true }
const PREVIEW_OK = { COLS: 14, MAX_ROWS: 4, ROWS: ['COMBIEN POUR'], CONSONANTS: 12, LETTERS: 17 }
function renderEditor(values = {}) {
  global.fetch = vi.fn(async () => ({ ok: true, status: 200, json: async () => PREVIEW_OK }))
  const onChange = vi.fn()
  const utils = render(<RoueEditor values={{ ...baseValues, ...values }} onChange={onChange} onValidityChange={vi.fn()} />)
  return { onChange, ...utils }
}
const radio = (c, name, value) => c.querySelector(`input[name="${name}"][value="${value}"]`)

describe('RoueEditor — section Voyelles', () => {
  it('critical — question v12.0 rouverte : Avant de tourner, Plusieurs / Plusieurs, 250, Sans gain, achat activé', () => {
    const { container } = renderEditor()
    expect(screen.getByRole('switch', { name: /Autoriser l'achat de voyelles/i }).getAttribute('aria-checked')).toBe('true')
    expect(radio(container, 'roueVowelTiming', 'BEFORE').checked).toBe(true)
    expect(radio(container, 'roueVowelsBefore', 'MANY').checked).toBe(true)
    expect(radio(container, 'roueVowelsAfter', 'MANY').checked).toBe(true)
    expect(radio(container, 'roueVowelGain', 'NONE').checked).toBe(true)
    expect(container.querySelector('#roue-vowel-price').value).toBe('250')
  })

  it('3 moments, 2 nombres (× 2), 3 gains', () => {
    const { container } = renderEditor()
    expect(container.querySelectorAll('input[name="roueVowelTiming"]')).toHaveLength(3)
    expect(container.querySelectorAll('input[name="roueVowelsBefore"]')).toHaveLength(2)
    expect(container.querySelectorAll('input[name="roueVowelsAfter"]')).toHaveLength(2)
    expect(container.querySelectorAll('input[name="roueVowelGain"]')).toHaveLength(3)
    expect(Array.from(container.querySelectorAll('input[name="roueVowelTiming"]')).map(i => i.value)).toEqual(['BEFORE', 'AFTER', 'BOTH'])
    expect(Array.from(container.querySelectorAll('input[name="roueVowelGain"]')).map(i => i.value)).toEqual(['NONE', 'REFUND', 'PER_OCCURRENCE'])
  })

  it('libellés fixés : moments, nombres et gains', () => {
    const { container } = renderEditor()
    for (const label of ['Avant de tourner', "Après l'arrêt de la roue", 'Les deux', '1 voyelle', 'Plusieurs voyelles', 'Sans gain', 'Mise remboursée', 'Prix × nombre de lettres']) {
      expect(container.textContent).toContain(label)
    }
  })

  it('les choix envoient onChange(champ, valeur)', () => {
    const { container, onChange } = renderEditor()
    fireEvent.click(radio(container, 'roueVowelTiming', 'BOTH'))
    expect(onChange).toHaveBeenCalledWith('roueVowelTiming', 'BOTH')
    fireEvent.click(radio(container, 'roueVowelsBefore', 'ONE'))
    expect(onChange).toHaveBeenCalledWith('roueVowelsBefore', 'ONE')
    fireEvent.click(radio(container, 'roueVowelsAfter', 'ONE'))
    expect(onChange).toHaveBeenCalledWith('roueVowelsAfter', 'ONE')
    fireEvent.click(radio(container, 'roueVowelGain', 'REFUND'))
    expect(onChange).toHaveBeenCalledWith('roueVowelGain', 'REFUND')
    fireEvent.click(screen.getByRole('switch', { name: /Autoriser l'achat de voyelles/i }))
    expect(onChange).toHaveBeenCalledWith('roueBuyVowels', false)
  })

  it('le nombre qui ne sert pas au moment choisi est grisé : AFTER grise « avant », BEFORE grise « après », BOTH aucun', () => {
    const disabled = (c, name) => Array.from(c.querySelectorAll(`input[name="${name}"]`)).every(i => i.disabled)
    const enabled = (c, name) => Array.from(c.querySelectorAll(`input[name="${name}"]`)).every(i => !i.disabled)
    let r = renderEditor({ roueVowelTiming: 'AFTER' })
    expect(disabled(r.container, 'roueVowelsBefore')).toBe(true)
    expect(enabled(r.container, 'roueVowelsAfter')).toBe(true)
    r.unmount()
    r = renderEditor({ roueVowelTiming: 'BEFORE' })
    expect(enabled(r.container, 'roueVowelsBefore')).toBe(true)
    expect(disabled(r.container, 'roueVowelsAfter')).toBe(true)
    r.unmount()
    r = renderEditor({ roueVowelTiming: 'BOTH' })
    expect(enabled(r.container, 'roueVowelsBefore')).toBe(true)
    expect(enabled(r.container, 'roueVowelsAfter')).toBe(true)
  })

  it('achat désactivé : tous les réglages sont grisés (mais conservés)', () => {
    const { container } = renderEditor({ roueBuyVowels: false, roueVowelTiming: 'AFTER', roueVowelPrice: 500 })
    for (const name of ['roueVowelTiming', 'roueVowelsBefore', 'roueVowelsAfter', 'roueVowelGain']) {
      expect(Array.from(container.querySelectorAll(`input[name="${name}"]`)).every(i => i.disabled)).toBe(true)
    }
    expect(container.querySelector('#roue-vowel-price').disabled).toBe(true)
    expect(container.querySelector('#roue-vowel-price').value).toBe('500')
    expect(radio(container, 'roueVowelTiming', 'AFTER').checked).toBe(true)
  })

  it('prix : saisie envoyée, valeur invalide signalée (aria-invalid)', () => {
    const { container, onChange } = renderEditor()
    fireEvent.change(container.querySelector('#roue-vowel-price'), { target: { value: '400' } })
    expect(onChange).toHaveBeenCalledWith('roueVowelPrice', '400')
    for (const bad of [0, '', 1000001]) {
      const r = renderEditor({ roueVowelPrice: bad })
      expect(r.container.querySelector('#roue-vowel-price').getAttribute('aria-invalid')).toBe('true')
      r.unmount()
    }
  })

  it('exemple de gain : « Prix × nombre de lettres » illustré (3 E à 250 → +750)', () => {
    const { container } = renderEditor({ roueVowelGain: 'PER_OCCURRENCE' })
    expect(container.textContent).toMatch(/750/)
  })
})
