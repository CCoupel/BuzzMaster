/**
 * #144 (v12.0.0, LOT-1.2) — interface animateur ROUE : spécification (contracts/roue.md §5, §5.2, §8.2 ; maquette M7).
 * L'animateur commande la manche avec 7 boutons d'étape — jamais ROUE_SET_TEAMS / ROUE_PASS / ROUE_REVEAL.
 * Complète AnimRoue.144.test.jsx (développeur).
 */
import { describe, it, expect, afterEach, vi } from 'vitest'
import { render, cleanup, fireEvent, screen } from '@testing-library/react'
import { AnimRoueView } from './AnimRoue'

afterEach(() => cleanup())

const gs = (over = {}) => ({
  phase: 'STARTED',
  question: { TYPE: 'ROUE', ROUE_MODE: 'TANT_QUE_JE_GAGNE', ROUE_PHRASE: 'BONJOUR LE MONDE', ROUE_PRESENTATION: 'Salutation' },
  ROUE_SUBPHASE: 'TURN', ROUE_STEP: 1, ROUE_TEAMS: ['Rouges', 'Bleus'], ROUE_CURRENT_TEAM: 'Rouges', ROUE_CURRENT_TEAM_COLOR: [239, 68, 68],
  ROUE_NEXT_TEAM: 'Bleus', ROUE_NEXT_DEFAULT: '', ROUE_TEAM_BANK: { Rouges: 300, Bleus: 50 }, ROUE_TURN_GAIN: 0,
  ROUE_BOARD: { COLS: 14, ROWS: ['_______ __'] }, ROUE_LETTERS: [], ROUE_SHOW_LETTERS: true, ROUE_CONSONANTS_LEFT: true,
  ROUE_WHEEL: { LAYOUT: 'strip', SPIN_PERIOD_MS: 2500, SPIN_MAX_MS: 10000, FLIP_INTERVAL_MS: 600, CURRENT: -1, STOP: null,
    SEGMENTS: [{ ID: 's0', KIND: 'value', VALUE: 500 }, { ID: 's1', KIND: 'bankrupt', VALUE: 0 }, { ID: 's2', KIND: 'lose_turn', VALUE: 0 }] },
  ROUE_LAST_EVENT: { SEQ: 0, KIND: '', TEAM: '', LETTER: '', COUNT: 0, VALUE: 0, GAIN: 0 }, ROUE_WINNER: '', ROUE_AWARDS: {},
  ...over,
})
const teams = { Rouges: { COLOR: [239, 68, 68] }, Bleus: { COLOR: [59, 130, 246] } }
const actionsMock = () => ({ setTeams: vi.fn(), spin: vi.fn(), stop: vi.fn(), letter: vi.fn(), flip: vi.fn(), answer: vi.fn(), next: vi.fn(), pass: vi.fn(), reveal: vi.fn() })
const view = (over, extra = {}) => { const a = actionsMock(); const r = render(<AnimRoueView gameState={gs(over)} teams={teams} actions={a} {...extra} />); return { a, ...r } }
const stepBtns = (c) => Array.from(c.querySelectorAll('.anim-roue-steps button'))
const byText = (c, re) => stepBtns(c).find(b => re.test(b.textContent))
const enabled = (c) => stepBtns(c).filter(b => !b.disabled).map(b => b.textContent.replace(/\s+/g, ' ').trim())

describe('AnimRoue — un seul bouton d\'étape actif à la fois', () => {
  it('TURN : seul « Lancer la roue » est actif', () => {
    const { container } = view()
    expect(enabled(container)).toEqual([expect.stringMatching(/Lancer/)])
  })
  it('SPINNING : seul « Forcer l\'arrêt » est actif', () => {
    const { container } = view({ ROUE_SUBPHASE: 'SPINNING', ROUE_STEP: 2 })
    expect(enabled(container)).toEqual([expect.stringMatching(/Forcer/)])
  })
  it('STOPPING : aucun bouton actif, « La roue ralentit… » affiché', () => {
    const { container } = view({ ROUE_SUBPHASE: 'STOPPING', ROUE_STEP: 3 })
    expect(enabled(container)).toEqual([])
    expect(container.textContent).toMatch(/ralentit/)
  })
  it('LETTER sans lettre choisie : « Valider la consonne » inactif', () => {
    const { container } = view({ ROUE_SUBPHASE: 'LETTER', ROUE_STEP: 4 })
    expect(enabled(container)).toEqual([])
  })
  it('LIT : seul « Retourner les cases » est actif', () => {
    const { container } = view({ ROUE_SUBPHASE: 'LIT', ROUE_STEP: 5 })
    expect(enabled(container)).toEqual([expect.stringMatching(/Retourner/)])
  })
  it('REVEALING : aucun bouton actif', () => {
    const { container } = view({ ROUE_SUBPHASE: 'REVEALING', ROUE_STEP: 5 })
    expect(enabled(container)).toEqual([])
  })
  it('ANSWER : « Réponse validée » + les deux boutons d\'étape 7', () => {
    const { container } = view({ ROUE_SUBPHASE: 'ANSWER', ROUE_STEP: 6 })
    const on = enabled(container)
    expect(on).toHaveLength(3)
    expect(on.some(t => /Réponse\s*validée/.test(t))).toBe(true)
    expect(on.some(t => /Tour suivant/.test(t))).toBe(true)
    expect(on.some(t => /Équipe suivante/.test(t))).toBe(true)
  })
  it('TURN_END : les deux boutons d\'étape 7 seulement', () => {
    const { container } = view({ ROUE_SUBPHASE: 'TURN_END', ROUE_STEP: 7 })
    expect(enabled(container)).toHaveLength(2)
  })
})

describe('AnimRoue — gestes', () => {
  it('Lancer → spin ; Forcer l\'arrêt → stop ; Retourner → flip ; Réponse validée → answer', () => {
    let r = view(); fireEvent.click(screen.getByRole('button', { name: /Lancer/ })); expect(r.a.spin).toHaveBeenCalledTimes(1); cleanup()
    r = view({ ROUE_SUBPHASE: 'SPINNING', ROUE_STEP: 2 }); fireEvent.click(screen.getByRole('button', { name: /Forcer/ })); expect(r.a.stop).toHaveBeenCalledTimes(1); cleanup()
    r = view({ ROUE_SUBPHASE: 'LIT', ROUE_STEP: 5 }); fireEvent.click(screen.getByRole('button', { name: /Retourner/ })); expect(r.a.flip).toHaveBeenCalledTimes(1); cleanup()
    r = view({ ROUE_SUBPHASE: 'ANSWER', ROUE_STEP: 6 }); fireEvent.click(byText(r.container, /Réponse\s*validée/)); expect(r.a.answer).toHaveBeenCalledTimes(1)
  })

  it('saisie : choisir une consonne puis « Valider la lettre X » envoie ROUE_LETTER', () => {
    const { a, container } = view({ ROUE_SUBPHASE: 'LETTER', ROUE_STEP: 4 })
    fireEvent.click(container.querySelector('.roue-kbd button.roue-key:not([disabled])'))
    const validate = stepBtns(container).find(b => /Valider la\s*lettre/.test(b.textContent))
    expect(validate).toBeTruthy()
    fireEvent.click(validate)
    expect(a.letter).toHaveBeenCalledTimes(1)
    expect(a.letter.mock.calls[0][0]).toMatch(/^[B-Z]$/)
  })

  it('étape 7 : « Tour suivant » → next(SAME), « Équipe suivante » → next(NEXT) ; noms d\'équipes affichés', () => {
    const { a } = view({ ROUE_SUBPHASE: 'TURN_END', ROUE_STEP: 7, ROUE_NEXT_DEFAULT: 'NEXT' })
    const same = screen.getByRole('button', { name: /Tour suivant/ }); const next = screen.getByRole('button', { name: /Équipe suivante/ })
    expect(same.textContent).toMatch(/Rouges/); expect(next.textContent).toMatch(/Bleus/)
    fireEvent.click(same); expect(a.next).toHaveBeenLastCalledWith('SAME')
    fireEvent.click(next); expect(a.next).toHaveBeenLastCalledWith('NEXT')
  })

  it.each([['SAME', /Tour suivant/], ['NEXT', /Équipe suivante/]])('conseillé %s : bouton plein (cur), l\'autre secondaire (alt) mais actif', (rec, re) => {
    const { container } = view({ ROUE_SUBPHASE: 'TURN_END', ROUE_STEP: 7, ROUE_NEXT_DEFAULT: rec })
    const btns = stepBtns(container).filter(b => !b.disabled)
    const recBtn = btns.find(b => re.test(b.textContent)); const other = btns.find(b => !re.test(b.textContent))
    expect(recBtn.className).toMatch(/\bcur\b/); expect(other.className).toMatch(/\balt\b/)
    expect(other.disabled).toBe(false)
  })
})

describe('AnimRoue — droits (jamais ROUE_SET_TEAMS / ROUE_PASS / ROUE_REVEAL)', () => {
  it('aucun bouton « Passer la main », « Dévoiler » ni choix d\'équipes dans aucune sous-phase', () => {
    for (const sub of ['TURN', 'SPINNING', 'STOPPING', 'LETTER', 'LIT', 'REVEALING', 'ANSWER', 'TURN_END', 'SOLVED']) {
      const { container, unmount } = view({ ROUE_SUBPHASE: sub, ROUE_STEP: 1 })
      expect(container.textContent, sub).not.toMatch(/Passer la main|Dévoiler|Équipes participantes/)
      unmount()
    }
  })
  it('un clic n\'appelle jamais setTeams / pass / reveal', () => {
    const { a, container } = view({ ROUE_SUBPHASE: 'ANSWER', ROUE_STEP: 6 })
    stepBtns(container).forEach(b => !b.disabled && fireEvent.click(b))
    expect(a.setTeams).not.toHaveBeenCalled(); expect(a.pass).not.toHaveBeenCalled(); expect(a.reveal).not.toHaveBeenCalled()
  })
})

describe('AnimRoue — SOLO et fin de manche', () => {
  it('SOLO : pas de bouton « Équipe suivante »', () => {
    view({ question: { ...gs().question, ROUE_MODE: 'SOLO' }, ROUE_TEAMS: ['Rouges'], ROUE_SUBPHASE: 'TURN_END', ROUE_STEP: 7, ROUE_NEXT_TEAM: '' })
    expect(screen.queryByRole('button', { name: /Équipe suivante/ })).toBeNull()
    expect(screen.getByRole('button', { name: /Tour suivant/ }).disabled).toBe(false)
  })
  it('plus de consonne : réponse validée et fin de tour depuis TURN, lancer inactif', () => {
    const { container } = view({ ROUE_CONSONANTS_LEFT: false })
    const on = enabled(container)
    expect(on.some(t => /Lancer/.test(t))).toBe(false)
    expect(on.some(t => /Réponse\s*validée/.test(t))).toBe(true)
  })
  it('SOLVED : aucun bouton d\'étape actif', () => {
    const { container } = view({ ROUE_SUBPHASE: 'SOLVED', ROUE_STEP: 0, ROUE_WINNER: 'Rouges' })
    expect(enabled(container)).toEqual([])
  })
  it('PAUSED : les boutons d\'étape ne sont pas proposés', () => {
    const { container } = view({ phase: 'PAUSED' })
    expect(container.querySelector('.anim-roue-steps')).toBeNull()
  })
})

describe('AnimRoue — informations affichées', () => {
  it('phrase : lettres trouvées à la couleur de l\'équipe qui les a proposées, non trouvées neutres', () => {
    const { container } = view({ ROUE_LETTERS: [{ LETTER: 'N', TEAM: 'Bleus', HIT: true }, { LETTER: 'Z', TEAM: 'Rouges', HIT: false }] })
    const sol = container.querySelector('.anim-roue-sol')
    expect(sol.textContent).toContain('BONJOUR')
    const n = Array.from(sol.querySelectorAll('span')).filter(e => e.textContent === 'N')
    expect(n.length).toBeGreaterThan(0)
    expect(n.every(e => e.className.includes('f') && e.style.getPropertyValue('--c').includes('59'))).toBe(true)
    const b = Array.from(sol.querySelectorAll('span')).find(e => e.textContent === 'B')
    expect(b.className).toBe('')
  })
  it('l\'animateur voit la phrase ; sans phrase dans le payload, pas de bloc', () => {
    expect(view().container.querySelector('.anim-roue-sol')).not.toBeNull()
    cleanup()
    expect(view({ question: { TYPE: 'ROUE', ROUE_MODE: 'SOLO' } }).container.querySelector('.anim-roue-sol')).toBeNull()
  })
  it('valeur de la case sous le pointeur, BANQUEROUTE / TOUR PERDU en toutes lettres', () => {
    const w = (i) => ({ ROUE_WHEEL: { ...gs().ROUE_WHEEL, CURRENT: i }, ROUE_SUBPHASE: 'LETTER', ROUE_STEP: 4 })
    expect(view(w(0)).container.querySelector('.anim-roue-case').textContent).toMatch(/500/); cleanup()
    expect(view(w(1)).container.querySelector('.anim-roue-case').textContent).toMatch(/BANQUEROUTE/); cleanup()
    expect(view(w(2)).container.querySelector('.anim-roue-case').textContent).toMatch(/TOUR PERDU/)
  })
  it('SPINNING : « la roue tourne » avec décompte', () => {
    expect(view({ ROUE_SUBPHASE: 'SPINNING', ROUE_STEP: 2 }).container.querySelector('.anim-roue-case').textContent).toMatch(/la roue tourne/)
  })
  it('cagnottes par équipe, l\'équipe qui a la main en avant', () => {
    const { container } = view()
    const rows = container.querySelectorAll('.anim-roue-team')
    expect(rows).toHaveLength(2)
    expect(container.querySelector('.anim-roue-team.active').textContent).toMatch(/Rouges/)
    expect(rows[1].textContent).toMatch(/50/)
  })
  it('gain du tour affiché en ANSWER et en TURN_END', () => {
    expect(view({ ROUE_SUBPHASE: 'ANSWER', ROUE_STEP: 6, ROUE_TURN_GAIN: 1000 }).container.querySelector('.anim-roue-gain').textContent).toMatch(/\+1\s?000/); cleanup()
    expect(view({ ROUE_SUBPHASE: 'TURN_END', ROUE_STEP: 7, ROUE_LAST_EVENT: { SEQ: 5, KIND: 'TURN_END', TEAM: 'Rouges', LETTER: '', COUNT: 0, VALUE: 0, GAIN: 200 } }).container.querySelector('.anim-roue-gain').textContent).toMatch(/\+200/)
  })
  it('lettre absente en fin de tour : « absente — aucun gain »', () => {
    const { container } = view({ ROUE_SUBPHASE: 'TURN_END', ROUE_STEP: 7, ROUE_LETTERS: [{ LETTER: 'Z', TEAM: 'Rouges', HIT: false }] })
    expect(container.querySelector('.anim-roue-gain').textContent).toMatch(/Z absente/)
  })
  it('refus serveur (WRONG_SUBPHASE) affiché à l\'animateur en double commande', () => {
    view({}, { error: { CODE: 'WRONG_SUBPHASE', MESSAGE: '', SEQ: 1 } })
    expect(screen.getByRole('alert').textContent).toMatch(/déjà prise en compte/)
  })
  it('aucun texte explicatif long (maquette M7) : pas de paragraphe d\'aide', () => {
    const { container } = view()
    expect(container.querySelectorAll('p.roue-hint, .roue-hint')).toHaveLength(0)
  })
})
