/**
 * #308 (v12.1.0) puis #316/#317 (v12.2.0) — case BONUS : spécification des écrans (contracts/roue.md §17 ;
 * maquettes docs/mockup/v12.2.0/…bonus_on_value). Un BONUS est une MARQUE (`BONUS: true`) posée sur une case `value` :
 * visuel 🎁 seul, montant dans le bandeau « BONUS ! case 800 », arrêt = sous-phase LETTER (plus de relance de la roue).
 * Les écrans n'affichent que ce que dit le serveur (ROUE_WHEEL.CURRENT, ROUE_LAST_EVENT.KIND = BONUS).
 */
import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest'
import { render, cleanup, fireEvent, screen, waitFor, act } from '@testing-library/react'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, join } from 'node:path'
import RoueTv from './RoueTv'
import RouePanel from './RouePanel'
import { AnimRoueView } from './AnimRoue'
import RoueEditor from './RoueEditor'
import FortuneWheel, { buildFortuneSegments } from './FortuneWheel'
import { roueAnnouncement, roueGates, segmentAria, segmentLabel, currentSegment } from './roueState'

afterEach(() => cleanup())
const here = dirname(fileURLToPath(import.meta.url))

const SEGS = [
  { ID: 's0', KIND: 'value', VALUE: 100, BONUS: false }, { ID: 's1', KIND: 'bankrupt', VALUE: 0, BONUS: false },
  { ID: 's2', KIND: 'value', VALUE: 800, BONUS: true }, { ID: 's3', KIND: 'lose_turn', VALUE: 0, BONUS: false },
]
const BONUS_EVENT = { SEQ: 7, KIND: 'BONUS', TEAM: 'Rouges', LETTER: '', COUNT: 0, VALUE: 800, GAIN: 0 }
const NO_EVENT = { SEQ: 0, KIND: '', TEAM: '', LETTER: '', COUNT: 0, VALUE: 0, GAIN: 0 }

/** État serveur juste après un arrêt sur une case marquée BONUS : sous-phase LETTER (étape 4), la roue n'est pas relancée. */
const gs = (over = {}) => ({
  phase: 'STARTED',
  question: { TYPE: 'ROUE', ROUE_MODE: 'TANT_QUE_JE_GAGNE', ROUE_PHRASE: 'BONJOUR LE MONDE', ROUE_PRESENTATION: 'Salutation' },
  ROUE_SUBPHASE: 'LETTER', ROUE_STEP: 4, ROUE_TEAMS: ['Rouges', 'Bleus'], ROUE_CURRENT_TEAM: 'Rouges', ROUE_CURRENT_TEAM_COLOR: [239, 68, 68],
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
  it('la roue conserve l\'ordre et les ID du serveur ; la case marquée reste une case à points (marque, pas un type)', () => {
    const segs = buildFortuneSegments(SEGS)
    expect(segs.map(s => s.id)).toEqual(['s0', 's1', 's2', 's3'])
    expect(segs[2].kind).toBe('value')
    expect(segs[2].mark).toBe('bonus')
    expect(segs[2].label).toBe('800') // montant conservé (retour à la normale, aria) ; Wheel masque l'affichage
    expect(segs[2].color).toBeTruthy() // couleur de palette conservée pour le fondu
    expect(segs[0].mark).toBeUndefined()
    expect(segs[1].kind).toBe('special')
  })

  it('la palette compte les cases marquées comme les autres (couleurs inchangées par la pose des BONUS)', () => {
    const plain = buildFortuneSegments([100, 200, 300].map((v, i) => ({ ID: `a${i}`, KIND: 'value', VALUE: v, BONUS: false })))
    const marked = buildFortuneSegments([100, 200, 300].map((v, i) => ({ ID: `a${i}`, KIND: 'value', VALUE: v, BONUS: i === 1 })))
    expect(marked.map(s => s.color)).toEqual(plain.map(s => s.color))
  })

  it('une valeur 0 n\'est jamais confondue avec une case BONUS', () => {
    const [seg] = buildFortuneSegments([{ ID: 'a', KIND: 'value', VALUE: 0, BONUS: false }])
    expect(seg.kind).toBe('value')
    expect(seg.mark).toBeUndefined()
  })

  it('libellé de la case : « 🎁 » seul (ni montant ni « BONUS ») ; libellé accessible « 800, BONUS »', () => {
    expect(segmentLabel(currentSegment(gs()))).toBe('🎁')
    expect(segmentAria(currentSegment(gs()))).toBe('800, BONUS')
    expect(segmentLabel({ KIND: 'value', VALUE: 800, BONUS: false })).toBe('800') // après retrait éphémère
    expect(segmentAria({ KIND: 'value', VALUE: 800, BONUS: false })).toBe('800')
  })

  it('FortuneWheel.css : jamais de scroll (overflow auto / scroll interdits)', () => {
    expect(readFileSync(join(here, 'FortuneWheel.css'), 'utf8')).not.toMatch(/overflow(-x|-y)?\s*:\s*(auto|scroll)/)
  })
})

describe('Roue — marque BONUS : rendu et fondu (wheel-component §9)', () => {
  const wheelOf = (bonus) => ({ LAYOUT: 'strip', SPIN_PERIOD_MS: 2500, SPIN_MAX_MS: 10000, CURRENT: 2, STOP: null,
    SEGMENTS: SEGS.map(x => (x.ID === 's2' ? { ...x, BONUS: bonus } : x)) })

  it('case marquée : calque doré, 🎁 ; aria-label « Segment : 800, BONUS » (montant annoncé, pas affiché)', () => {
    const { container } = render(<FortuneWheel wheel={wheelOf(true)} subphase="LETTER" />)
    const marks = container.querySelectorAll('[data-testid="wheel-mark"]')
    expect(marks.length).toBeGreaterThan(0)
    marks.forEach(m => { expect(m.classList.contains('on')).toBe(true); expect(m.textContent).toBe('🎁') })
    expect(container.querySelector('.wheel-cell.marked')).toBeTruthy()
    expect(container.querySelector('[role="img"]').getAttribute('aria-label')).toBe('Segment : 800, BONUS')
  })

  it('arc : même marque (calque doré + 🎁 dans le secteur)', () => {
    const w = { ...wheelOf(true), LAYOUT: 'arc' }
    const { container } = render(<FortuneWheel wheel={w} subphase="LETTER" />)
    const mark = container.querySelector('g[data-testid="wheel-mark"]')
    expect(mark.getAttribute('class')).toMatch(/on/)
    expect(mark.textContent).toBe('🎁')
  })

  it('retrait (éphémère) : la marque passe hors « on » (fondu CSS), le MÊME nœud reste monté — roue non recréée', () => {
    const { container, rerender } = render(<FortuneWheel wheel={wheelOf(true)} subphase="LETTER" />)
    const track = container.querySelector('.wheel-track')
    const mark = container.querySelector('[data-testid="wheel-mark"]')
    rerender(<FortuneWheel wheel={wheelOf(false)} subphase="ANSWER" />)
    expect(container.querySelector('.wheel-track')).toBe(track)
    expect(container.querySelector('[data-testid="wheel-mark"]')).toBe(mark)
    expect(mark.classList.contains('on')).toBe(false)
    expect(container.querySelector('.wheel-cell.marked')).toBeNull()
    expect(container.querySelector('[role="img"]').getAttribute('aria-label')).toBe('Segment : 800')
  })

  it('une case jamais marquée n\'a pas de calque', () => {
    const { container } = render(<FortuneWheel wheel={wheelOf(false)} subphase="LETTER" />)
    expect(container.querySelector('[data-testid="wheel-mark"]')).toBeNull()
  })

  it('Wheel.css : fondu de 600 ms et prefers-reduced-motion respecté', () => {
    const css = readFileSync(join(here, '..', 'wheel', 'Wheel.css'), 'utf8')
    expect(css).toMatch(/\.wheel-mark\s*\{[^}]*transition:\s*opacity 600ms/)
    expect(css).toMatch(/prefers-reduced-motion[\s\S]*\.wheel-mark/)
  })
})

// ---- annonce et portes --------------------------------------------------------------------------------------------

describe('Après un BONUS — annonce et portes', () => {
  it('l\'annonce dit BONUS, la valeur de la case et invite à proposer une consonne ; plus de « rejouent »', () => {
    const t = text(roueAnnouncement(gs()))
    expect(t).toMatch(/BONUS.*case 800.*consonne/)
    expect(t).not.toMatch(/rejou/i)
  })

  it('tablette de l\'équipe concernée : même annonce', () => {
    expect(text(roueAnnouncement(gs(), { mine: true }))).toMatch(/BONUS.*case 800/)
  })

  it('l\'annonce BONUS disparaît dès que l\'événement change (geste suivant)', () => {
    const t = text(roueAnnouncement(gs({ ROUE_SUBPHASE: 'LIT', ROUE_LAST_EVENT: { ...BONUS_EVENT, SEQ: 8, KIND: 'FLIP', LETTER: 'R', COUNT: 1 } })))
    expect(t).not.toMatch(/BONUS/)
  })

  it('portes : on propose une lettre ; la roue n\'est pas relançable (pas de nouveau tour)', () => {
    const g = roueGates(gs())
    expect(g.canLetter).toBe(true)
    expect(g.canSpin).toBe(false)
    expect(g.canNext).toBeFalsy()
  })
})

// ---- TV et tablette -----------------------------------------------------------------------------------------------

describe('RoueTv — bandeau « 🎁 BONUS ! »', () => {
  it('TV : bandeau « BONUS ! case 800 » tant que ROUE_LAST_EVENT.KIND = BONUS', () => {
    const { container } = render(<RoueTv gameState={gs()} teams={teams} />)
    expect(container.textContent).toMatch(/BONUS ! case 800/)
  })

  it('bandeau retiré au choix de la lettre (événement ≠ BONUS) ; en éphémère il reste monté 600 ms pour le fondu', () => {
    vi.useFakeTimers()
    try {
      const { container, rerender } = render(<RoueTv gameState={gs()} teams={teams} />)
      expect(container.querySelector('[data-testid="roue-bonus"]')).toBeTruthy()
      const after = gs({ ROUE_SUBPHASE: 'LIT', ROUE_LAST_EVENT: { ...BONUS_EVENT, SEQ: 8, KIND: 'FLIP', LETTER: 'R', VALUE: 0 } })
      rerender(<RoueTv gameState={after} teams={teams} />)
      const fading = container.querySelector('[data-testid="roue-bonus"]')
      expect(fading?.className).toMatch(/fading/)
      expect(fading.textContent).toMatch(/case 800/) // la valeur est mémorisée pendant le fondu
      act(() => { vi.advanceTimersByTime(700) })
      expect(container.querySelector('[data-testid="roue-bonus"]')).toBeNull()
    } finally { vi.useRealTimers() }
  })

  it('TV : pas de bandeau sans événement BONUS (ni pour une autre case spéciale)', () => {
    const { container, rerender } = render(<RoueTv gameState={gs({ ROUE_LAST_EVENT: NO_EVENT })} teams={teams} />)
    expect(container.textContent).not.toMatch(/BONUS !/)
    rerender(<RoueTv gameState={gs({ ROUE_LAST_EVENT: { ...NO_EVENT, SEQ: 3, KIND: 'LOSE_TURN' } })} teams={teams} />)
    expect(container.textContent).not.toMatch(/BONUS !/)
  })

  it('tablette joueur : même bandeau', () => {
    const { container } = render(<RoueTv gameState={gs()} teams={teams} isVPlayer teamName="Rouges" />)
    expect(container.textContent).toMatch(/BONUS ! case 800/)
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
  it('bandeau « BONUS ! case 800 — remettez le cadeau, puis proposez une consonne » ; la roue n\'est pas relançable', () => {
    render(<RouePanel gameState={gs()} actions={actionsMock()} />)
    const b = screen.getByTestId('roue-bonus')
    expect(b.textContent).toMatch(/BONUS ! case 800/)
    expect(b.textContent).toMatch(/remettez le cadeau/)
    expect(b.textContent).toMatch(/consonne/)
    expect(screen.queryByRole('button', { name: /Lancer la roue/ })).toBeNull()
  })

  it('la case du bandeau d\'étape : 🎁 seul avec aria-label « 800, BONUS »', () => {
    const { container } = render(<RouePanel gameState={gs()} actions={actionsMock()} />)
    const seg = container.querySelector('.roue-seg')
    expect(seg.textContent).toBe('🎁')
    expect(seg.getAttribute('aria-label')).toBe('800, BONUS')
  })

  it('pas de bandeau BONUS sans événement BONUS', () => {
    render(<RouePanel gameState={gs({ ROUE_LAST_EVENT: NO_EVENT })} actions={actionsMock()} />)
    expect(screen.queryByText(/BONUS !/)).toBeNull()
  })

  it('puce du mode : nombre de BONUS et mode (éphémère par défaut, permanent sinon)', () => {
    const q = { TYPE: 'ROUE', ROUE_MODE: 'TANT_QUE_JE_GAGNE', ROUE_PHRASE: 'X', ROUE_BONUS_COUNT: 2 }
    const { rerender } = render(<RouePanel gameState={gs({ question: q })} actions={actionsMock()} />)
    expect(screen.getByTestId('roue-bonus-chip').textContent).toMatch(/2.*une seule fois/)
    rerender(<RouePanel gameState={gs({ question: q, ROUE_BONUS_MODE: 'PERMANENT' })} actions={actionsMock()} />)
    expect(screen.getByTestId('roue-bonus-chip').textContent).toMatch(/permanent/)
  })
})

describe('AnimRoue (animateur) — après un BONUS', () => {
  it('bandeau « BONUS ! case 800 » ; case de la roue : 🎁 seul sur fond doré, aria-label « 800, BONUS »', () => {
    const { container } = render(<AnimRoueView gameState={gs()} teams={teams} actions={actionsMock()} />)
    expect(container.textContent).toMatch(/BONUS ! case 800/)
    const box = container.querySelector('.anim-roue-case')
    expect(box.className).toMatch(/gold/)
    expect(box.getAttribute('aria-label')).toBe('800, BONUS')
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
const WHEEL_OK = { COUNT: 12, step: 10, SEGMENTS: [{ ID: 's0', KIND: 'value', VALUE: 100 }, { ID: 's1', KIND: 'bankrupt', VALUE: 0 }, { ID: 's2', KIND: 'value', VALUE: 800, BONUS: true }] }
let calls
beforeEach(() => {
  calls = []
  global.fetch = vi.fn(async (url, opts) => {
    calls.push([url, opts])
    if (url === '/api/roue/preview') return { ok: true, status: 200, json: async () => PREVIEW_OK }
    if (url === '/game-config.json') return { ok: true, status: 200, json: async () => ({ wheel: { max_points: 100, step: 10, bankrupt_count: 1, lose_turn_count: 1, bonus_count: 0 } }) }
    if (url === '/api/roue/wheel-preview') return { ok: true, status: 200, json: async () => WHEEL_OK }
    return { ok: true, status: 200, json: async () => ({}) }
  })
})
const renderEditor = (values = {}) => {
  const onChange = vi.fn()
  const utils = render(<RoueEditor values={{ ...baseValues, ...values }} onChange={onChange} onValidityChange={vi.fn()} />)
  return { onChange, ...utils }
}
// Modèle §16 (#309) : le BONUS est le 5ᵉ paramètre de composition — `null` / absent = suit le global, nombre = personnalisé (0 compris), saisie 0–4.
const bonus = () => screen.getByTestId('roue-wparam-X')
const bonusInput = () => bonus().querySelector('input')
const bonusToggle = () => bonus().querySelector('button')
const loaded2 = async () => { await waitFor(() => expect(bonusInput()).toBeTruthy(), { timeout: 2000 }); await waitFor(() => expect(calls.some(c => c[0] === '/game-config.json')).toBe(true)) }
const loaded = async () => { await waitFor(() => expect(bonusInput().value).toBe('0'), { timeout: 2000 }); await waitFor(() => expect(calls.some(c => c[0] === '/game-config.json')).toBe(true)) }

describe('RoueEditor — cases BONUS (modèle #309 : suit le global, ou personnalisé 0–4)', () => {
  it('critical — question v12.0 (champ absent) : BONUS = global (0), lié, non modifiable', async () => {
    renderEditor()
    await loaded()
    expect(bonus().textContent).toMatch(/BONUS/)
    expect(bonus().textContent).toMatch(/global/)
    expect(bonus().textContent).not.toMatch(/personnalisé/)
    expect(bonusInput().disabled).toBe(true)
  })

  it('personnaliser (✏️) envoie le global comme départ ; saisie → onChange(roueBonusCount, n) ; 🔗 → null', async () => {
    const { onChange } = renderEditor()
    await loaded()
    fireEvent.click(bonusToggle())
    expect(onChange).toHaveBeenCalledWith('roueBonusCount', 0)
    cleanup()
    const r = renderEditor({ roueBonusCount: 2 })
    await loaded2()
    fireEvent.change(bonusInput(), { target: { value: '3' } })
    expect(r.onChange).toHaveBeenCalledWith('roueBonusCount', 3)
    fireEvent.click(bonusToggle())
    expect(r.onChange).toHaveBeenCalledWith('roueBonusCount', null)
  })

  it('borné à 0–4 : la saisie est plafonnée à 4 et ne descend pas sous 0 ; attributs min / max', async () => {
    const { onChange } = renderEditor({ roueBonusCount: 1 })
    await loaded2()
    expect(bonusInput().getAttribute('min')).toBe('0')
    expect(bonusInput().getAttribute('max')).toBe('4')
    fireEvent.change(bonusInput(), { target: { value: '9' } })
    expect(onChange).toHaveBeenCalledWith('roueBonusCount', 4)
    fireEvent.change(bonusInput(), { target: { value: '-3' } })
    expect(onChange).toHaveBeenCalledWith('roueBonusCount', 0)
  })

  it('l\'aperçu de la roue vient toujours du serveur et reçoit le BONUS effectif (global 0 si non personnalisé)', async () => {
    renderEditor()
    await waitFor(() => expect(calls.some(c => c[0] === '/api/roue/wheel-preview')).toBe(true), { timeout: 2000 })
    expect(JSON.parse(calls.find(c => c[0] === '/api/roue/wheel-preview')[1].body).bonus_count).toBe(0)
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
      if (url === '/game-config.json') return { ok: true, status: 200, json: async () => ({ wheel: { max_points: 63, step: 1, bankrupt_count: 0, lose_turn_count: 0, bonus_count: 0 } }) }
      return { ok: false, status: 400, json: async () => ({ CODE: 'ROUE_BONUS_COUNT_INVALID', MESSAGE: '' }) }
    })
    renderEditor({ roueBonusCount: 4 })
    await waitFor(() => expect(screen.getByRole('alert').textContent).toMatch(/BONUS|64/), { timeout: 2000 })
  })
})
