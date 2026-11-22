/**
 * #316 (v12.2.0, LOT-1) — marque BONUS sur une case à points : spécification des écrans écrite depuis les contrats
 * (contracts/wheel-component.md §9, contracts/roue.md §17.4 / §17.5 ; maquette tv_roue_started__bonus_on_value).
 *
 * Décisions utilisateur vérifiées ici : case DORÉE + RUBAN, logo 🎁 SEUL (ni montant ni mot « BONUS » à l'écran), valeur sur le
 * bandeau, `aria-label` « 800, BONUS » (montant annoncé aux lecteurs d'écran), retrait par un fondu SANS recréer la roue.
 * Les écrans n'affichent que ce que dit le serveur : aucune règle de placement côté navigateur.
 * Fichier NEUF : les tests v12.1 (RoueBonus.308.test.jsx …) appartiennent à dev-frontend.
 */
import { describe, it, expect, afterEach, vi } from 'vitest'
import { render, cleanup, act } from '@testing-library/react'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, join } from 'node:path'
import FortuneWheel, { buildFortuneSegments } from './FortuneWheel'
import RoueTv from './RoueTv'
import { roueAnnouncement, segmentAria, segmentLabel } from './roueState'

afterEach(() => cleanup())
const here = dirname(fileURLToPath(import.meta.url))
const wheelCss = readFileSync(join(here, '..', 'wheel', 'Wheel.css'), 'utf8')

const seg = (i, kind, value = 0, bonus = false) => ({ ID: `s${i}`, KIND: kind, VALUE: value, BONUS: bonus })
/** 8 cases : 3 cases à points marquables, une BANQUEROUTE, un PERDU. La case 2 (800) est la case jouée. */
const segs = (marked = [2]) => [
  seg(0, 'value', 1000), seg(1, 'bankrupt'), seg(2, 'value', 800, marked.includes(2)), seg(3, 'value', 100),
  seg(4, 'lose_turn'), seg(5, 'value', 900, marked.includes(5)), seg(6, 'value', 300), seg(7, 'value', 700, marked.includes(7)),
]
const wheel = (marked = [2], over = {}) => ({ LAYOUT: 'strip', SPIN_PERIOD_MS: 2500, SPIN_MAX_MS: 10000, CURRENT: 2, STOP: null, SEGMENTS: segs(marked), ...over })

const cells = (c) => Array.from(c.querySelectorAll('.wheel-cell'))
/** Une seule répétition du ruban suffit : les cellules d'un même ID sont des copies. */
const cellOf = (c, id) => cells(c).find((el) => el.getAttribute('data-id') === id) || null

describe('buildFortuneSegments — correspondance contrat → Wheel (wheel-component §9)', () => {
  it('KIND value + BONUS true → { kind:"value", label:montant, mark:"bonus" } ; BONUS false / absent → pas de mark', () => {
    const out = buildFortuneSegments([seg(0, 'value', 800, true), seg(1, 'value', 100, false), { ID: 's2', KIND: 'value', VALUE: 50 }])
    expect(out[0]).toMatchObject({ id: 's0', kind: 'value', label: '800', mark: 'bonus' })
    expect(out[1].mark).toBeFalsy()
    expect(out[2].mark).toBeFalsy()
    expect(out.every((s) => s.kind === 'value')).toBe(true)
  })

  it('une marque n\'est jamais portée par une case BANQUEROUTE / PERDU (le serveur ne le fait pas ; le front ne l\'invente pas)', () => {
    const out = buildFortuneSegments([seg(0, 'bankrupt'), seg(1, 'lose_turn')])
    expect(out.map((s) => s.mark)).toEqual([undefined, undefined])
    expect(out.map((s) => s.kind)).toEqual(['special', 'special'])
  })

  it('la couleur de palette est celle de la case non marquée (couleurs inchangées par la pose des BONUS)', () => {
    const plain = buildFortuneSegments(segs([]))
    const marked = buildFortuneSegments(segs([2, 5, 7]))
    expect(marked.map((s) => s.color)).toEqual(plain.map((s) => s.color))
    expect(marked.map((s) => s.label)).toEqual(plain.map((s) => s.label)) // le montant reste calculé (retour à la normale)
  })

  it('libellés : « 🎁 » seul pour une case marquée, jamais le montant ni « BONUS » ; « 800 » après retrait ; aria « 800, BONUS »', () => {
    const on = { KIND: 'value', VALUE: 800, BONUS: true }
    const off = { KIND: 'value', VALUE: 800, BONUS: false }
    expect(segmentLabel(on)).toBe('🎁')
    expect(segmentLabel(on)).not.toMatch(/800|BONUS/)
    expect(segmentAria(on)).toBe('800, BONUS')
    expect(segmentLabel(off)).toBe('800')
    expect(segmentAria(off)).toBe('800')
    expect(segmentLabel({ KIND: 'value', VALUE: 800 })).toBe('800') // BONUS absent = non marquée
  })
})

describe('Wheel — case marquée : dorée + ruban, 🎁 seul, montant masqué', () => {
  it('bandeau : une seule marque active (calque doré + ruban + 🎁) pour la case marquée, aucune ailleurs', () => {
    const { container } = render(<FortuneWheel wheel={wheel([2])} subphase="LETTER" />)
    const layers = Array.from(container.querySelectorAll('[data-testid="wheel-mark"]'))
    expect(layers.length).toBeGreaterThan(0)
    layers.forEach((m) => {
      expect(m.classList.contains('on')).toBe(true)
      expect(m.textContent).toBe('🎁') // le logo, rien d'autre : ni montant, ni « BONUS »
    })
    // Ruban cadeau : bande dorée décorative SANS texte.
    const ribbons = container.querySelectorAll('.wheel-ribbon')
    expect(ribbons.length).toBeGreaterThan(0)
    ribbons.forEach((r) => expect(r.textContent).toBe(''))
    // Chaque répétition du ruban de cellules porte la marque sur la même case seulement.
    const marked = Array.from(container.querySelectorAll('.wheel-cell.marked'))
    expect(marked.length).toBe(layers.length)
    marked.forEach((cell) => expect(cell.querySelector('.wheel-mark.on')).toBeTruthy())
  })

  it('le montant n\'est pas AFFICHÉ sur la case marquée : masqué par la feuille de style (l\'élément reste, pour le retour à la normale)', () => {
    expect(wheelCss).toMatch(/\.wheel-cell\.marked\s+\.wheel-lab[^{]*\{[^}]*opacity:\s*0/)
    expect(wheelCss).toMatch(/\.wheel-arc\s+\.wheel-sector\.marked\s+\.wheel-lab-text[^{]*\{[^}]*opacity:\s*0/)
  })

  it('aucun texte « BONUS » visible dans la case marquée (le mot n\'existe que dans l\'aria-label de la roue)', () => {
    const { container } = render(<FortuneWheel wheel={wheel([2])} subphase="LETTER" />)
    container.querySelectorAll('.wheel-cell.marked').forEach((cell) => {
      expect(cell.textContent).not.toMatch(/BONUS/i)
    })
    expect(container.querySelector('[role="img"]').getAttribute('aria-label')).toBe('Segment : 800, BONUS')
  })

  it('arc : même rendu (secteur doré, 🎁 seul) ; le montant du secteur est masqué par la feuille de style', () => {
    const { container } = render(<FortuneWheel wheel={wheel([2], { LAYOUT: 'arc' })} subphase="LETTER" />)
    const mark = container.querySelector('g[data-testid="wheel-mark"]')
    expect(mark).toBeTruthy()
    expect(mark.getAttribute('class')).toMatch(/\bon\b/)
    expect(mark.textContent).toBe('🎁')
    expect(container.querySelector('.wheel-sector.marked')).toBeTruthy()
    expect(container.querySelectorAll('.wheel-sector.marked').length).toBe(1)
  })

  it('plusieurs marques : une seule cellule par case marquée (trois cases), les autres restent normales avec leur montant', () => {
    const { container } = render(<FortuneWheel wheel={wheel([2, 5, 7], { CURRENT: 0 })} subphase="TURN" />)
    const onceMarked = new Set(Array.from(container.querySelectorAll('.wheel-cell.marked')).map((c) => c.textContent))
    // textContent d'une case marquée = « 🎁 » + son libellé masqué ; trois cases distinctes
    expect(onceMarked.size).toBe(3)
    const plain = Array.from(container.querySelectorAll('.wheel-cell:not(.marked)'))
    expect(plain.some((c) => c.textContent.includes('1000'))).toBe(true)
    expect(plain.some((c) => c.textContent.includes('100'))).toBe(true)
  })

  it('sans aucune marque : aucun calque, aria sans « BONUS »', () => {
    const { container } = render(<FortuneWheel wheel={wheel([])} subphase="LETTER" />)
    expect(container.querySelector('[data-testid="wheel-mark"]')).toBeNull()
    expect(container.querySelector('[role="img"]').getAttribute('aria-label')).toBe('Segment : 800')
  })

  it('une case spéciale courante n\'a jamais d\'aria « BONUS »', () => {
    const { container } = render(<FortuneWheel wheel={wheel([2], { CURRENT: 1 })} subphase="TURN_END" />)
    expect(container.querySelector('[role="img"]').getAttribute('aria-label')).not.toMatch(/BONUS/)
  })
})

describe('Retrait de la marque (BONUS éphémère) — fondu, la roue n\'est jamais recréée', () => {
  it('critical — la marque passe hors « on » : MÊME ruban, MÊME calque monté, même case courante, même position', () => {
    const { container, rerender } = render(<FortuneWheel wheel={wheel([2])} subphase="LETTER" />)
    const root = container.querySelector('.wheel')
    const track = container.querySelector('.wheel-track')
    const layer = container.querySelector('[data-testid="wheel-mark"]')
    const cellsBefore = cells(container)
    const pos = root.getAttribute('data-pos')
    const current = container.querySelector('.wheel-cell[data-current="true"]')

    // Éphémère : le serveur republie SEGMENTS (nouveau tableau, mêmes ID) avec BONUS = false sur la case jouée.
    rerender(<FortuneWheel wheel={wheel([], { SEGMENTS: segs([]).map((s) => ({ ...s })) })} subphase="LIT" />)

    expect(container.querySelector('.wheel')).toBe(root) // pas de nouvelle roue
    expect(container.querySelector('.wheel-track')).toBe(track)
    expect(container.querySelector('[data-testid="wheel-mark"]')).toBe(layer) // le calque reste monté : c'est lui qui fond
    expect(layer.classList.contains('on')).toBe(false)
    expect(root.getAttribute('data-pos')).toBe(pos) // aucun saut de CURRENT
    expect(root.getAttribute('data-animate')).toBe('false') // aucun mouvement relancé
    expect(container.querySelector('.wheel-cell[data-current="true"]')).toBe(current)
    expect(cells(container).every((el, i) => el === cellsBefore[i])).toBe(true) // mêmes cellules (pas de remontage)
    expect(container.querySelector('.wheel-cell.marked')).toBeNull()
  })

  it('le fondu est une transition CSS d\'environ 600 ms ; sans animation si l\'utilisateur la refuse (prefers-reduced-motion)', () => {
    expect(wheelCss).toMatch(/\.wheel-mark\s*\{[^}]*transition:\s*opacity 600ms/)
    expect(wheelCss).toMatch(/\.wheel-cell \.wheel-lab[^{]*\{[^}]*transition:\s*opacity 600ms/) // le montant réapparaît par le même fondu
    expect(wheelCss).toMatch(/prefers-reduced-motion[\s\S]*transition:\s*none/)
  })

  it('après le retrait : la case reprend sa couleur de palette et son montant ; aria « Segment : 800 »', () => {
    const colorOfCase = (c) => {
      const el = Array.from(c.querySelectorAll('.wheel-cell')).find((e) => e.textContent.includes('800'))
      return el ? el.style.background : null
    }
    const never = render(<FortuneWheel wheel={wheel([])} subphase="LETTER" />)
    const expectedColor = colorOfCase(never.container)
    never.unmount()
    const { container, rerender } = render(<FortuneWheel wheel={wheel([2])} subphase="LETTER" />)
    expect(colorOfCase(container)).toBe(expectedColor) // même sous la marque, la couleur de palette est calculée (fondu vers elle)
    rerender(<FortuneWheel wheel={wheel([])} subphase="ANSWER" />)
    expect(colorOfCase(container)).toBe(expectedColor)
    expect(container.querySelector('[role="img"]').getAttribute('aria-label')).toBe('Segment : 800')
    expect(Array.from(container.querySelectorAll('.wheel-cell')).some((c) => c.textContent.includes('800'))).toBe(true)
  })

  it('seule la case jouée perd sa marque : les autres marques restent actives', () => {
    const { container, rerender } = render(<FortuneWheel wheel={wheel([2, 5, 7])} subphase="LETTER" />)
    const before = container.querySelectorAll('.wheel-cell.marked').length
    rerender(<FortuneWheel wheel={wheel([5, 7])} subphase="LIT" />)
    const after = container.querySelectorAll('.wheel-cell.marked').length
    expect(after).toBeLessThan(before)
    expect(after).toBeGreaterThan(0)
    const onLayers = Array.from(container.querySelectorAll('[data-testid="wheel-mark"]')).filter((m) => m.classList.contains('on'))
    expect(onLayers.length).toBe(after)
  })

  it('permanent : la marque ne bouge pas quand le serveur republie le même état', () => {
    const { container, rerender } = render(<FortuneWheel wheel={wheel([2])} subphase="LETTER" />)
    const layer = container.querySelector('[data-testid="wheel-mark"]')
    rerender(<FortuneWheel wheel={wheel([2])} subphase="LIT" />)
    expect(container.querySelector('[data-testid="wheel-mark"]')).toBe(layer)
    expect(layer.classList.contains('on')).toBe(true)
    expect(container.querySelector('[role="img"]').getAttribute('aria-label')).toBe('Segment : 800, BONUS')
  })

  it('la clé de mouvement ne dépend pas de SEGMENTS : republier les cases pendant la montée ne relance pas l\'animation', () => {
    const mk = (marked) => wheel(marked, { CURRENT: -1, FULL_SPEED: true, SPIN_UP_MS: 5000 })
    const { container, rerender } = render(<FortuneWheel wheel={mk([2])} subphase="SPINNING" />)
    const mover = container.querySelector('.wheel-track')
    const style = mover.getAttribute('style')
    rerender(<FortuneWheel wheel={mk([])} subphase="SPINNING" />)
    expect(container.querySelector('.wheel-track')).toBe(mover)
    expect(mover.getAttribute('style')).toBe(style)
  })
})

// ---- annonce -----------------------------------------------------------------------------------------------------

const text = (parts) => parts.map((p) => p.strong ?? p.text).join('')
const BONUS_EVENT = { SEQ: 7, KIND: 'BONUS', TEAM: 'Rouges', LETTER: '', COUNT: 0, VALUE: 800, GAIN: 0 }
const gs = (over = {}) => ({
  phase: 'STARTED',
  question: { TYPE: 'ROUE', ROUE_MODE: 'TANT_QUE_JE_GAGNE', ROUE_PHRASE: 'BONJOUR LE MONDE', ROUE_PRESENTATION: 'Salutation' },
  ROUE_SUBPHASE: 'LETTER', ROUE_STEP: 4, ROUE_TEAMS: ['Rouges', 'Bleus'], ROUE_CURRENT_TEAM: 'Rouges', ROUE_CURRENT_TEAM_COLOR: [239, 68, 68],
  ROUE_NEXT_TEAM: '', ROUE_NEXT_DEFAULT: '', ROUE_TEAM_BANK: { Rouges: 500, Bleus: 300 }, ROUE_TURN_GAIN: 0,
  ROUE_BOARD: { COLS: 14, ROWS: ['_______ __'] }, ROUE_LETTERS: [], ROUE_SHOW_LETTERS: true, ROUE_CONSONANTS_LEFT: true,
  ROUE_BONUS_MODE: 'EPHEMERAL',
  ROUE_WHEEL: wheel([2]), ROUE_LAST_EVENT: BONUS_EVENT, ROUE_WINNER: '', ROUE_AWARDS: {},
  ...over,
})
const teams = { Rouges: { COLOR: [239, 68, 68] }, Bleus: { COLOR: [59, 130, 246] } }

describe('Annonce et bandeau après l\'arrêt sur une case marquée', () => {
  it('annonce : « 🎁 BONUS — case 800 : proposez une consonne » ; plus de « rejouent » (la roue n\'est pas relancée)', () => {
    const t = text(roueAnnouncement(gs()))
    expect(t).toMatch(/🎁/)
    expect(t).toMatch(/BONUS.*case 800.*proposez une consonne/)
    expect(t).not.toMatch(/rejou/i)
  })

  it('l\'annonce porte la valeur de la CASE (VALUE de l\'événement), pas 0', () => {
    expect(text(roueAnnouncement(gs({ ROUE_LAST_EVENT: { ...BONUS_EVENT, VALUE: 500 } })))).toMatch(/case 500/)
  })

  it('TV : bandeau « BONUS ! case 800 » avec la valeur ; l\'arc / bandeau de la roue porte 🎁 seul', () => {
    const { container } = render(<RoueTv gameState={gs()} teams={teams} />)
    const banner = container.querySelector('[data-testid="roue-bonus"]')
    expect(banner).toBeTruthy()
    expect(banner.textContent).toMatch(/BONUS ! case 800/)
    container.querySelectorAll('.wheel-cell.marked [data-testid="wheel-mark"]').forEach((m) => expect(m.textContent).toBe('🎁'))
  })

  it('tablette joueur : même rendu que la TV (bandeau et case marquée)', () => {
    const { container } = render(<RoueTv gameState={gs()} teams={teams} isVPlayer teamName="Rouges" />)
    expect(container.textContent).toMatch(/BONUS ! case 800/)
    expect(container.querySelector('.wheel-cell.marked')).toBeTruthy()
  })

  it('permanent : le bandeau est retiré tout de suite au choix de la lettre (pas de fondu, la marque reste)', () => {
    vi.useFakeTimers()
    try {
      const { container, rerender } = render(<RoueTv gameState={gs({ ROUE_BONUS_MODE: 'PERMANENT' })} teams={teams} />)
      expect(container.querySelector('[data-testid="roue-bonus"]')).toBeTruthy()
      rerender(<RoueTv gameState={gs({ ROUE_BONUS_MODE: 'PERMANENT', ROUE_SUBPHASE: 'LIT', ROUE_LAST_EVENT: { ...BONUS_EVENT, SEQ: 8, KIND: 'LETTER_HIT', LETTER: 'R', COUNT: 1, VALUE: 800, GAIN: 800 } })} teams={teams} />)
      act(() => { vi.advanceTimersByTime(50) })
      expect(container.querySelector('[data-testid="roue-bonus"]')).toBeNull()
      expect(container.querySelector('.wheel-cell.marked')).toBeTruthy() // permanent : la marque reste
    } finally { vi.useRealTimers() }
  })

  it('éphémère : le bandeau s\'efface par le même fondu que la case (monté ~600 ms) puis disparaît', () => {
    vi.useFakeTimers()
    try {
      const { container, rerender } = render(<RoueTv gameState={gs()} teams={teams} />)
      rerender(<RoueTv gameState={gs({ ROUE_SUBPHASE: 'LIT', ROUE_WHEEL: wheel([]), ROUE_LAST_EVENT: { ...BONUS_EVENT, SEQ: 8, KIND: 'LETTER_HIT', LETTER: 'R', COUNT: 1, VALUE: 800, GAIN: 800 } })} teams={teams} />)
      expect(container.querySelector('[data-testid="roue-bonus"]')).toBeTruthy() // encore monté pendant le fondu
      expect(container.querySelector('.wheel-cell.marked')).toBeNull() // la case est redevenue normale
      act(() => { vi.advanceTimersByTime(900) })
      expect(container.querySelector('[data-testid="roue-bonus"]')).toBeNull()
    } finally { vi.useRealTimers() }
  })

  it('la phrase reste secrète pendant un BONUS (TV et tablette)', () => {
    const tv = render(<RoueTv gameState={gs()} teams={teams} />)
    expect(tv.container.textContent).not.toMatch(/BONJOUR|MONDE/)
    tv.unmount()
    const tablet = render(<RoueTv gameState={gs()} teams={teams} isVPlayer teamName="Rouges" />)
    expect(tablet.container.textContent).not.toMatch(/BONJOUR|MONDE/)
  })
})
