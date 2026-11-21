/**
 * #308 (v12.1.0, LOT-1.1) — case BONUS : spécification des écrans (contracts/roue.md §15.2 ; maquettes
 * docs/mockup/v12.1.0/ui/{tv_roue_started,admin_game_roue,anim_roue,admin_question_editor_roue}__vowels_bonus.html).
 * Les écrans n'affichent que ce que dit le serveur (ROUE_WHEEL.CURRENT, ROUE_LAST_EVENT.KIND = BONUS).
 */
import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest'
import { render, cleanup, fireEvent, screen, waitFor } from '@testing-library/react'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, join } from 'node:path'
import RoueTv from './RoueTv'
import RouePanel from './RouePanel'
import { AnimRoueView } from './AnimRoue'
import RoueEditor from './RoueEditor'
import { buildFortuneSegments } from './FortuneWheel'
import { roueAnnouncement, roueGates, segmentLabel, currentSegment } from './roueState'

afterEach(() => cleanup())
const here = dirname(fileURLToPath(import.meta.url))

const SEGS = [
  { ID: 's0', KIND: 'value', VALUE: 100 }, { ID: 's1', KIND: 'bankrupt', VALUE: 0 },
  { ID: 's2', KIND: 'bonus', VALUE: 0 }, { ID: 's3', KIND: 'lose_turn', VALUE: 0 },
]
const BONUS_EVENT = { SEQ: 7, KIND: 'BONUS', TEAM: 'Rouges', LETTER: '', COUNT: 0, VALUE: 0, GAIN: 0 }
const NO_EVENT = { SEQ: 0, KIND: '', TEAM: '', LETTER: '', COUNT: 0, VALUE: 0, GAIN: 0 }

/** État serveur juste après un arrêt sur BONUS : nouveau tour (TURN, étape 1), case BONUS sous le pointeur. */
const gs = (over = {}) => ({
  phase: 'STARTED',
  question: { TYPE: 'ROUE', ROUE_MODE: 'TANT_QUE_JE_GAGNE', ROUE_PHRASE: 'BONJOUR LE MONDE', ROUE_PRESENTATION: 'Salutation' },
  ROUE_SUBPHASE: 'TURN', ROUE_STEP: 1, ROUE_TEAMS: ['Rouges', 'Bleus'], ROUE_CURRENT_TEAM: 'Rouges', ROUE_CURRENT_TEAM_COLOR: [239, 68, 68],
  ROUE_NEXT_TEAM: '', ROUE_NEXT_DEFAULT: '', ROUE_TEAM_BANK: { Rouges: 500, Bleus: 300 }, ROUE_TURN_GAIN: 0,
  ROUE_BOARD: { COLS: 14, ROWS: ['_______ __'] }, ROUE_LETTERS: [], ROUE_SHOW_LETTERS: true, ROUE_CONSONANTS_LEFT: true,
  ROUE_WHEEL: { LAYOUT: 'strip', SPIN_PERIOD_MS: 2500, SPIN_MAX_MS: 10000, FLIP_INTERVAL_MS: 600, CURRENT: 2, STOP: null, SEGMENTS: SEGS },
  ROUE_LAST_EVENT: BONUS_EVENT, ROUE_WINNER: '', ROUE_AWARDS: {},
  ...over,
})
const teams = { Rouges: { COLOR: [239, 68, 68] }, Bleus: { COLOR: [59, 130, 246] } }
const actionsMock = () => ({ setTeams: vi.fn(), spin: vi.fn(), stop: vi.fn(), letter: vi.fn(), flip: vi.fn(), answer: vi.fn(), next: vi.fn(), pass: vi.fn(), reveal: vi.fn() })
const text = (parts) => parts.map(p => p.strong ?? p.text).join('')

// ---- la case ------------------------------------------------------------------------------------------------------

describe('Case BONUS — roue et libellés', () => {
  it('la roue conserve l\'ordre et les ID du serveur et rend une case BONUS distincte d\'une valeur et d\'une case spéciale', () => {
    const segs = buildFortuneSegments(SEGS)
    expect(segs.map(s => s.id)).toEqual(['s0', 's1', 's2', 's3'])
    expect(segs[2].label).toMatch(/BONUS/)
    expect(segs[2].kind).not.toBe('value')
    expect(segs[2].color).not.toBe(segs[1].color) // dorée, pas la couleur des cases spéciales
    expect(segs[1].kind).toBe('special')
    expect(segs[0].kind).toBe('value')
  })

  it('une valeur 0 n\'est jamais confondue avec une case BONUS', () => {
    expect(buildFortuneSegments([{ ID: 'a', KIND: 'value', VALUE: 0 }])[0].kind).toBe('value')
  })

  it('libellé de la case : « 🎁 BONUS » ; case courante lue sur ROUE_WHEEL.CURRENT', () => {
    expect(segmentLabel({ KIND: 'bonus', VALUE: 0 })).toBe('🎁 BONUS')
    expect(segmentLabel(currentSegment(gs()))).toBe('🎁 BONUS')
  })

  it('FortuneWheel.css : jamais de scroll (overflow auto / scroll interdits)', () => {
    expect(readFileSync(join(here, 'FortuneWheel.css'), 'utf8')).not.toMatch(/overflow(-x|-y)?\s*:\s*(auto|scroll)/)
  })
})

// ---- annonce et portes --------------------------------------------------------------------------------------------

describe('Après un BONUS — annonce et portes', () => {
  it('l\'annonce dit BONUS et que l\'équipe rejoue ; aucune invitation à proposer une consonne', () => {
    const t = text(roueAnnouncement(gs()))
    expect(t).toMatch(/BONUS/)
    expect(t).toMatch(/rejou/i)
    expect(t).not.toMatch(/consonne/i)
  })

  it('tablette de l\'équipe concernée : « vous rejouez »', () => {
    expect(text(roueAnnouncement(gs(), { mine: true }))).toMatch(/BONUS.*vous rejouez/)
  })

  it('l\'annonce BONUS disparaît dès que l\'événement change (geste suivant)', () => {
    const t = text(roueAnnouncement(gs({ ROUE_SUBPHASE: 'SPINNING', ROUE_LAST_EVENT: { ...BONUS_EVENT, SEQ: 8, KIND: 'SPIN' } })))
    expect(t).not.toMatch(/BONUS/)
  })

  it('portes : on peut lancer la roue ; ni lettre, ni dévoilement de lettre, ni fin de tour', () => {
    const g = roueGates(gs())
    expect(g.canSpin).toBe(true)
    expect(g.canLetter).toBeFalsy()
    expect(g.canFlip).toBeFalsy()
    expect(g.canNext).toBeFalsy()
  })
})

// ---- TV et tablette -----------------------------------------------------------------------------------------------

describe('RoueTv — bandeau « 🎁 BONUS ! »', () => {
  it('TV : bandeau « BONUS ! » tant que ROUE_LAST_EVENT.KIND = BONUS', () => {
    const { container } = render(<RoueTv gameState={gs()} teams={teams} />)
    expect(container.textContent).toMatch(/BONUS !/)
  })

  it('TV : pas de bandeau sans événement BONUS (ni pour une autre case spéciale)', () => {
    const { container, rerender } = render(<RoueTv gameState={gs({ ROUE_LAST_EVENT: NO_EVENT })} teams={teams} />)
    expect(container.textContent).not.toMatch(/BONUS !/)
    rerender(<RoueTv gameState={gs({ ROUE_LAST_EVENT: { ...NO_EVENT, SEQ: 3, KIND: 'LOSE_TURN' } })} teams={teams} />)
    expect(container.textContent).not.toMatch(/BONUS !/)
  })

  it('tablette joueur : même bandeau', () => {
    const { container } = render(<RoueTv gameState={gs()} teams={teams} isVPlayer teamName="Rouges" />)
    expect(container.textContent).toMatch(/BONUS !/)
  })

  it('le bandeau n\'expose ni cagnotte modifiée ni gain : cagnottes inchangées à l\'écran', () => {
    const { container } = render(<RoueTv gameState={gs()} teams={teams} />)
    expect(container.textContent).not.toMatch(/\+\s?\d/) // aucun « +N » de gain
  })

  it('la phrase reste secrète pendant un BONUS (§9)', () => {
    const { container } = render(<RoueTv gameState={gs()} teams={teams} />)
    expect(container.textContent).not.toMatch(/BONJOUR|MONDE/)
  })

  it('TV statique : RoueTv.css sans scroll, conteneur en overflow hidden', () => {
    const css = readFileSync(join(here, 'RoueTv.css'), 'utf8')
    expect(css).not.toMatch(/overflow(-x|-y)?\s*:\s*(auto|scroll)/)
    expect(css).toMatch(/\.roue-tv\s*\{[^}]*overflow:\s*hidden/)
  })
})

// ---- admin et animateur -------------------------------------------------------------------------------------------

describe('RouePanel (admin) — après un BONUS', () => {
  it('bandeau BONUS, « Lancer la roue » actif, aucune consonne à proposer', () => {
    const a = actionsMock()
    render(<RouePanel gameState={gs()} actions={a} />)
    expect(screen.getByText(/BONUS !/)).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: /Lancer la roue/ }))
    expect(a.spin).toHaveBeenCalledTimes(1)
  })

  it('« Passer la main » reste disponible', () => {
    const a = actionsMock()
    render(<RouePanel gameState={gs()} actions={a} />)
    const pass = screen.queryByRole('button', { name: /Passer la main/ })
    expect(pass).toBeTruthy()
    expect(pass.disabled).toBe(false)
    fireEvent.click(pass)
    expect(a.pass).toHaveBeenCalledTimes(1)
  })

  it('pas de bandeau BONUS sans événement BONUS', () => {
    render(<RouePanel gameState={gs({ ROUE_LAST_EVENT: NO_EVENT })} actions={actionsMock()} />)
    expect(screen.queryByText(/BONUS !/)).toBeNull()
  })
})

describe('AnimRoue (animateur) — après un BONUS', () => {
  it('bandeau BONUS et seul « Lancer la roue » actif', () => {
    const { container } = render(<AnimRoueView gameState={gs()} teams={teams} actions={actionsMock()} />)
    expect(container.textContent).toMatch(/BONUS !/)
    const enabled = Array.from(container.querySelectorAll('.anim-roue-steps button')).filter(b => !b.disabled).map(b => b.textContent)
    expect(enabled).toEqual([expect.stringMatching(/Lancer/)])
  })

  it('sans événement BONUS : pas de bandeau', () => {
    const { container } = render(<AnimRoueView gameState={gs({ ROUE_LAST_EVENT: NO_EVENT })} teams={teams} actions={actionsMock()} />)
    expect(container.textContent).not.toMatch(/BONUS !/)
  })
})

// ---- éditeur ------------------------------------------------------------------------------------------------------

const baseValues = {
  rouePhrase: '', rouePresentation: '', roueMode: 'TANT_QUE_JE_GAGNE',
  roueGainsRule: 'WINNER_ONLY', roueWheelLayout: 'strip', roueShowLetters: true,
}
const PREVIEW_OK = { COLS: 14, MAX_ROWS: 4, ROWS: ['COMBIEN POUR'], CONSONANTS: 12, LETTERS: 17 }
const WHEEL_OK = { COUNT: 12, step: 10, SEGMENTS: [{ ID: 's0', KIND: 'value', VALUE: 100 }, { ID: 's1', KIND: 'bankrupt', VALUE: 0 }, { ID: 's2', KIND: 'bonus', VALUE: 0 }] }
let calls
beforeEach(() => {
  calls = []
  global.fetch = vi.fn(async (url, opts) => {
    calls.push([url, opts])
    if (url === '/api/roue/preview') return { ok: true, status: 200, json: async () => PREVIEW_OK }
    if (url === '/game-config.json') return { ok: true, status: 200, json: async () => ({ wheel: { max_points: 100, step: 10, bankrupt_count: 1, lose_turn_count: 1 } }) }
    if (url === '/api/roue/wheel-preview') return { ok: true, status: 200, json: async () => WHEEL_OK }
    return { ok: true, status: 200, json: async () => ({}) }
  })
})
const renderEditor = (values = {}) => {
  const onChange = vi.fn()
  const utils = render(<RoueEditor values={{ ...baseValues, ...values }} onChange={onChange} onValidityChange={vi.fn()} />)
  return { onChange, ...utils }
}
const plus = () => screen.getByRole('button', { name: /Plus de cases BONUS/ })
const minus = () => screen.getByRole('button', { name: /Moins de cases BONUS/ })

describe('RoueEditor — cases BONUS', () => {
  it('critical — question v12.0 (champ absent) : 0 BONUS, « moins » grisé', () => {
    renderEditor()
    expect(screen.getByText(/Cases BONUS/)).toBeTruthy()
    expect(document.body.textContent).toMatch(/Cases BONUS\s*(−|-)?\s*0/)
    expect(minus().disabled).toBe(true)
    expect(plus().disabled).toBe(false)
  })

  it('« plus » envoie onChange(roueBonusCount, n + 1) ; « moins » n - 1', () => {
    const { onChange } = renderEditor({ roueBonusCount: 2 })
    fireEvent.click(plus())
    expect(onChange).toHaveBeenCalledWith('roueBonusCount', 3)
    fireEvent.click(minus())
    expect(onChange).toHaveBeenCalledWith('roueBonusCount', 1)
  })

  it('borné à 0–4 : « plus » grisé à 4, « moins » grisé à 0', () => {
    renderEditor({ roueBonusCount: 4 })
    expect(plus().disabled).toBe(true)
    expect(minus().disabled).toBe(false)
  })

  it('aucun appel réseau d\'aperçu de la roue tant que le compteur est à 0 et intact', async () => {
    renderEditor()
    await new Promise(r => setTimeout(r, 450))
    expect(calls.filter(c => c[0] === '/api/roue/wheel-preview')).toHaveLength(0)
  })

  it('compteur > 0 : l\'aperçu de la roue vient du serveur (wheel-preview avec bonus_count), sans copie de la règle', async () => {
    renderEditor({ roueBonusCount: 2 })
    await waitFor(() => expect(calls.some(c => c[0] === '/api/roue/wheel-preview')).toBe(true), { timeout: 2000 })
    const [, opts] = calls.find(c => c[0] === '/api/roue/wheel-preview')
    expect(opts.method).toBe('POST')
    expect(JSON.parse(opts.body)).toMatchObject({ max_points: 100, step: 10, bankrupt_count: 1, lose_turn_count: 1, bonus_count: 2 })
  })

  it('refus du serveur (roue de plus de 64 cases) : message d\'erreur lisible, pas d\'exception', async () => {
    global.fetch = vi.fn(async (url) => {
      if (url === '/api/roue/preview') return { ok: true, status: 200, json: async () => PREVIEW_OK }
      if (url === '/game-config.json') return { ok: true, status: 200, json: async () => ({ wheel: { max_points: 63, step: 1, bankrupt_count: 0, lose_turn_count: 0 } }) }
      return { ok: false, status: 400, json: async () => ({ CODE: 'ROUE_BONUS_COUNT_INVALID', MESSAGE: '' }) }
    })
    renderEditor({ roueBonusCount: 4 })
    await waitFor(() => expect(screen.getByRole('alert').textContent).toMatch(/BONUS|64/), { timeout: 2000 })
  })
})
