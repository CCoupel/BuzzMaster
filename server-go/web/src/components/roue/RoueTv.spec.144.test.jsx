/**
 * #144 (v12.0.0, LOT-1.2) — écran TV / tablette ROUE : spécification (contracts/roue.md §4, §5, §9 ; CLAUDE.md
 * « Contrainte Affichage TV » ; maquettes M1, M5). Complète RoueTv.144.test.jsx (développeur).
 */
import { describe, it, expect, afterEach, vi } from 'vitest'
import { render, cleanup, fireEvent } from '@testing-library/react'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, join } from 'node:path'
import RoueTv from './RoueTv'
import RoueGameBoard from './RoueGameBoard'
import { roueAnnouncement, makeRoueActions, roueGates } from './roueState'

afterEach(() => cleanup())
const here = dirname(fileURLToPath(import.meta.url))

const gs = (over = {}) => ({
  phase: 'STARTED',
  question: { TYPE: 'ROUE', ROUE_MODE: 'TANT_QUE_JE_GAGNE' },
  ROUE_SUBPHASE: 'TURN', ROUE_TEAMS: ['Rouges', 'Bleus'], ROUE_CURRENT_TEAM: 'Rouges', ROUE_CURRENT_TEAM_COLOR: [239, 68, 68],
  ROUE_TEAM_BANK: { Rouges: 0, Bleus: 0 }, ROUE_CONSONANTS_LEFT: true, ROUE_SHOW_LETTERS: true,
  ROUE_BOARD: { COLS: 14, ROWS: ['______ __', '_____'] }, ROUE_LETTERS: [],
  ROUE_WHEEL: { LAYOUT: 'strip', SPIN_PERIOD_MS: 2500, SPIN_MAX_MS: 10000, FLIP_INTERVAL_MS: 600, CURRENT: -1, STOP: null,
    SEGMENTS: [{ ID: 's0', KIND: 'value', VALUE: 100 }, { ID: 's1', KIND: 'bankrupt', VALUE: 0 }, { ID: 's2', KIND: 'lose_turn', VALUE: 0 }] },
  ROUE_LAST_EVENT: { SEQ: 0, KIND: '', TEAM: '', LETTER: '', COUNT: 0, VALUE: 0, GAIN: 0 },
  ROUE_WINNER: '', ROUE_AWARDS: {}, ROUE_TURN_GAIN: 0,
  ...over,
})
const teams = { Rouges: { COLOR: [239, 68, 68] }, Bleus: { COLOR: [59, 130, 246] } }
// Pleine vitesse (montée terminée, ROUE_WHEEL.FULL_SPEED du serveur — roue.md §4, §5.1) ; sans cela, la roue monte en vitesse.
const full = (over = {}) => ({ ROUE_SUBPHASE: 'SPINNING', ROUE_WHEEL: { ...gs().ROUE_WHEEL, SPIN_UP_MS: 10000, FULL_SPEED: true }, ...over })
const climb = (over = {}) => ({ ROUE_SUBPHASE: 'SPINNING', ROUE_WHEEL: { ...gs().ROUE_WHEEL, SPIN_UP_MS: 10000, FULL_SPEED: false }, ...over })
const text = (parts) => parts.map(p => p.strong ?? p.text).join('')

describe('RoueTv — confidentialité (§9)', () => {
  it('TV : la phrase n\'est jamais rendue, même si elle figurait dans le payload', () => {
    const { container } = render(<RoueTv gameState={gs({ question: { TYPE: 'ROUE', ROUE_PHRASE: 'COMBIEN POUR CE CHIEN' } })} teams={teams} />)
    expect(container.textContent).not.toMatch(/COMBIEN|CHIEN/)
  })

  it('TV : le plateau affiche exactement ROUE_BOARD (cases cachées sans lettre)', () => {
    const { container } = render(<RoueTv gameState={gs()} teams={teams} />)
    const letters = Array.from(container.querySelectorAll('.rgb-cell.shown')).map(e => e.textContent)
    expect(letters).toEqual([])
    expect(container.querySelectorAll('.rgb-cell.tile').length).toBe(13)
  })
})

describe('RoueTv — statique (CLAUDE.md « Contrainte Affichage TV »)', () => {
  it.each(['RoueTv.css', 'RoueGameBoard.css', 'FortuneWheel.css'])('%s : jamais de scroll (overflow auto / scroll interdits)', (f) => {
    const css = readFileSync(join(here, f), 'utf8')
    expect(css).not.toMatch(/overflow(-x|-y)?\s*:\s*(auto|scroll)/)
  })

  it('RoueTv.css : le conteneur TV est en overflow hidden et en unités viewport', () => {
    const css = readFileSync(join(here, 'RoueTv.css'), 'utf8')
    expect(css).toMatch(/\.roue-tv\s*\{[^}]*overflow:\s*hidden/)
    expect(css).toMatch(/\d+(\.\d+)?vh/)
  })
})

describe('RoueTv — équipes et cagnottes', () => {
  it('une carte par équipe participante, dans l\'ordre de rotation, avec sa cagnotte', () => {
    const { container } = render(<RoueTv gameState={gs({ ROUE_TEAMS: ['Bleus', 'Rouges'], ROUE_TEAM_BANK: { Rouges: 1300, Bleus: 7 } })} teams={teams} />)
    expect(Array.from(container.querySelectorAll('.roue-bank .n')).map(e => e.textContent)).toEqual(['Bleus', 'Rouges'])
    expect(container.querySelectorAll('.roue-bank')[1].querySelector('.v').textContent).toMatch(/1\s?300/)
  })

  it('seule l\'équipe qui a la main est mise en avant', () => {
    const { container } = render(<RoueTv gameState={gs()} teams={teams} />)
    expect(container.querySelectorAll('.roue-bank.active')).toHaveLength(1)
    expect(container.querySelector('.roue-bank.active .n').textContent).toBe('Rouges')
  })

  it('résolution : plus d\'équipe active ; le gagnant affiche son crédit', () => {
    const { container } = render(<RoueTv gameState={gs({ ROUE_SUBPHASE: 'SOLVED', ROUE_WINNER: 'Bleus', ROUE_AWARDS: { Bleus: 1500 } })} teams={teams} />)
    expect(container.querySelector('.roue-bank.active')).toBeNull()
    expect(container.querySelector('.roue-bank.win .v').textContent).toMatch(/\+1\s?500/)
  })

  it('une cagnotte absente s\'affiche 0, jamais NaN / undefined', () => {
    const { container } = render(<RoueTv gameState={gs({ ROUE_TEAM_BANK: {} })} teams={teams} />)
    expect(container.textContent).not.toMatch(/NaN|undefined/)
  })
})

describe('RoueTv — roue', () => {
  it.each([['strip', 'roue-wheel-strip'], ['arc', 'roue-wheel-arc']])('forme %s → classe %s', (layout, cls) => {
    const { container } = render(<RoueTv gameState={gs({ ROUE_WHEEL: { ...gs().ROUE_WHEEL, LAYOUT: layout } })} teams={teams} />)
    expect(container.querySelector(`.${cls}`)).not.toBeNull()
  })

  it('la roue reste affichée pendant toute la manche, y compris résolue', () => {
    for (const sub of ['TURN', 'SPINNING', 'STOPPING', 'LETTER', 'LIT', 'REVEALING', 'ANSWER', 'TURN_END', 'SOLVED']) {
      const { container, unmount } = render(<RoueTv gameState={gs({ ROUE_SUBPHASE: sub, ROUE_WHEEL: { ...gs().ROUE_WHEEL, STOP: sub === 'STOPPING' ? { TO: 1, DURATION_MS: 4000 } : null } })} teams={teams} />)
      expect(container.querySelector('[data-testid="fortune-wheel"]'), sub).not.toBeNull()
      unmount()
    }
  })

  it('lettres proposées : lettres absentes barrées, ligne masquée quand ROUE_SHOW_LETTERS=false', () => {
    const letters = [{ LETTER: 'T', TEAM: 'Rouges', HIT: true }, { LETTER: 'Z', TEAM: 'Bleus', HIT: false }]
    const on = render(<RoueTv gameState={gs({ ROUE_LETTERS: letters })} teams={teams} />)
    expect(on.container.querySelectorAll('.roue-letters b')).toHaveLength(2)
    expect(on.container.querySelector('.roue-letters b.miss').textContent).toBe('Z')
    cleanup()
    const off = render(<RoueTv gameState={gs({ ROUE_LETTERS: letters, ROUE_SHOW_LETTERS: false })} teams={teams} />)
    expect(off.container.querySelectorAll('.roue-letters b')).toHaveLength(0)
  })
})

describe('roueAnnouncement — une annonce par sous-phase', () => {
  const seg = (kind, value = 0) => ({ ROUE_WHEEL: { ...gs().ROUE_WHEEL, CURRENT: 0, SEGMENTS: [{ ID: 's0', KIND: kind, VALUE: value }] } })
  it('TURN, SPINNING, STOPPING, ANSWER', () => {
    expect(text(roueAnnouncement(gs()))).toMatch(/ROUGES.*jouer/i)
    expect(text(roueAnnouncement(gs(full())))).toMatch(/buzzez/i)
    expect(text(roueAnnouncement(gs({ ROUE_SUBPHASE: 'STOPPING' })))).toMatch(/ralentit/i)
    expect(text(roueAnnouncement(gs({ ROUE_SUBPHASE: 'ANSWER' })))).toMatch(/réponse/i)
  })
  it('TURN sans consonne : invite à proposer la phrase', () => {
    expect(text(roueAnnouncement(gs({ ROUE_CONSONANTS_LEFT: false })))).toMatch(/plus de consonne/i)
  })
  it('LETTER : annonce la valeur de la case', () => {
    expect(text(roueAnnouncement(gs({ ROUE_SUBPHASE: 'LETTER', ...seg('value', 500) })))).toMatch(/500/)
  })
  it('LIT / REVEALING : lettre, nombre de cases, gain', () => {
    const ev = { SEQ: 4, KIND: 'LETTER_HIT', TEAM: 'Rouges', LETTER: 'N', COUNT: 2, VALUE: 500, GAIN: 1000 }
    expect(text(roueAnnouncement(gs({ ROUE_SUBPHASE: 'LIT', ROUE_LAST_EVENT: ev })))).toMatch(/N.*2 cases/)
    expect(text(roueAnnouncement(gs({ ROUE_SUBPHASE: 'REVEALING', ROUE_LAST_EVENT: ev })))).toMatch(/2 × 500/)
  })
  it('TURN_END : BANQUEROUTE, TOUR PERDU, lettre absente, gain', () => {
    expect(text(roueAnnouncement(gs({ ROUE_SUBPHASE: 'TURN_END', ...seg('bankrupt') })))).toMatch(/BANQUEROUTE/)
    expect(text(roueAnnouncement(gs({ ROUE_SUBPHASE: 'TURN_END', ...seg('lose_turn') })))).toMatch(/TOUR PERDU/)
    expect(text(roueAnnouncement(gs({ ROUE_SUBPHASE: 'TURN_END', ...seg('value', 100), ROUE_LETTERS: [{ LETTER: 'Z', TEAM: 'Rouges', HIT: false }] })))).toMatch(/Z.*absente/)
  })
  it('SOLVED : gagnant, ou « Phrase dévoilée » sans gagnant', () => {
    expect(text(roueAnnouncement(gs({ ROUE_SUBPHASE: 'SOLVED', ROUE_WINNER: 'Bleus', ROUE_AWARDS: { Bleus: 900 } })))).toMatch(/BLEUS.*900/)
    expect(text(roueAnnouncement(gs({ ROUE_SUBPHASE: 'SOLVED' })))).toMatch(/dévoilée/i)
  })
  it('sous-phase inconnue ou vide : aucune annonce, pas d\'exception', () => {
    expect(roueAnnouncement(gs({ ROUE_SUBPHASE: '' }))).toEqual([])
    expect(roueAnnouncement(gs({ ROUE_SUBPHASE: 'ZZZ' }))).toEqual([])
    expect(roueAnnouncement(undefined)).toEqual([])
  })
  it('tablette : la formulation dépend de « mine »', () => {
    expect(text(roueAnnouncement(gs(full()), { mine: true }))).toMatch(/arrêtez/i)
    expect(text(roueAnnouncement(gs(full()), { mine: false }))).toMatch(/vont arrêter/i)
  })
})

describe('RoueTv — tablette (M5)', () => {
  const TAP = { name: /Arrêter la roue/ }

  it('la roue est présente sur la tablette dans toutes les sous-phases, y compris SOLVED, pour les deux équipes', () => {
    for (const sub of ['TURN', 'SPINNING', 'STOPPING', 'LETTER', 'LIT', 'REVEALING', 'ANSWER', 'TURN_END', 'SOLVED']) {
      for (const team of ['Rouges', 'Bleus']) {
        const { container, unmount } = render(<RoueTv gameState={gs({ ROUE_SUBPHASE: sub, ROUE_WINNER: sub === 'SOLVED' ? 'Bleus' : '' })} teams={teams} isVPlayer teamName={team} onStop={() => {}} />)
        expect(container.querySelector('[data-testid="fortune-wheel"]'), `${sub}/${team}`).not.toBeNull()
        unmount()
      }
    }
  })

  it('un tap = un seul onStop, seulement équipe qui a la main + SPINNING ; jamais ailleurs', () => {
    for (const sub of ['TURN', 'STOPPING', 'LETTER', 'LIT', 'REVEALING', 'ANSWER', 'TURN_END', 'SOLVED']) {
      const onStop = vi.fn()
      const { container, queryByRole, unmount } = render(<RoueTv gameState={gs({ ROUE_SUBPHASE: sub })} teams={teams} isVPlayer teamName="Rouges" onStop={onStop} />)
      expect(queryByRole('button', TAP), sub).toBeNull()
      fireEvent.click(container.querySelector('[data-testid="fortune-wheel"]'))
      expect(onStop, sub).not.toHaveBeenCalled()
      unmount()
    }
    // autre équipe en SPINNING : pas de bouton, un clic sur la roue n'envoie rien
    const other = vi.fn()
    const o = render(<RoueTv gameState={gs(full())} teams={teams} isVPlayer teamName="Bleus" onStop={other} />)
    expect(o.queryByRole('button', TAP)).toBeNull()
    fireEvent.click(o.container.querySelector('[data-testid="fortune-wheel"]'))
    expect(other).not.toHaveBeenCalled()
    o.unmount()
    const onStop = vi.fn()
    const { getByRole } = render(<RoueTv gameState={gs(full())} teams={teams} isVPlayer teamName="Rouges" onStop={onStop} />)
    fireEvent.click(getByRole('button', TAP))
    expect(onStop).toHaveBeenCalledTimes(1)
  })

  it('plus aucun bouton STOP, dans aucune sous-phase', () => {
    for (const sub of ['TURN', 'SPINNING', 'STOPPING', 'LETTER', 'SOLVED']) {
      const { container, unmount } = render(<RoueTv gameState={gs({ ROUE_SUBPHASE: sub })} teams={teams} isVPlayer teamName="Rouges" onStop={() => {}} />)
      expect(container.querySelector('.roue-vp-stop'), sub).toBeNull()
      expect(Array.from(container.querySelectorAll('button')).some(b => /^\s*STOP/.test(b.textContent)), sub).toBe(false)
      unmount()
    }
  })

  it('SPINNING : un seul bouton (la roue), cadre à la couleur de l\'équipe, messages pour chaque équipe', () => {
    const mine = render(<RoueTv gameState={gs(full())} teams={teams} isVPlayer teamName="Rouges" onStop={() => {}} />)
    expect(mine.container.querySelectorAll('button').length).toBe(1)
    expect(mine.container.textContent).toMatch(/Touchez la roue pour l'arrêter/)
    mine.unmount()
    const other = render(<RoueTv gameState={gs(full())} teams={teams} isVPlayer teamName="Bleus" onStop={() => {}} />)
    expect(other.container.querySelectorAll('button').length).toBe(0)
    expect(other.container.textContent).toMatch(/Les Rouges vont arrêter la roue/)
    expect(other.container.textContent).not.toMatch(/Touchez la roue/)
  })

  it('tablette sans nom d\'équipe (non liée) : roue affichée mais jamais tapable', () => {
    const { container, queryByRole } = render(<RoueTv gameState={gs(full())} teams={teams} isVPlayer teamName={null} onStop={() => {}} />)
    expect(queryByRole('button', TAP)).toBeNull()
    expect(container.querySelector('[data-testid="fortune-wheel"]')).not.toBeNull()
  })

  it('onStop absent : jamais tapable (pas d\'erreur)', () => {
    const { queryByRole } = render(<RoueTv gameState={gs(full())} teams={teams} isVPlayer teamName="Rouges" />)
    expect(queryByRole('button', TAP)).toBeNull()
  })

  it('indice : catégorie + présentation (courte, 80 caractères, absente), jamais la phrase, en toutes sous-phases', () => {
    const long = 'P'.repeat(40) + ' ' + 'Q'.repeat(39)
    expect(long.length).toBe(80)
    for (const [pres, re] of [['Chanson de Line Renaud', /Chanson de Line Renaud/], [long, new RegExp(long)], [undefined, null]]) {
      for (const sub of ['TURN', 'SPINNING', 'SOLVED']) {
        const q = { TYPE: 'ROUE', ROUE_MODE: 'TANT_QUE_JE_GAGNE', ROUE_PHRASE: 'COMBIENPOURCECHIEN', ...(pres ? { ROUE_PRESENTATION: pres } : {}) }
        const { container, unmount } = render(<RoueTv gameState={gs({ question: q, ROUE_SUBPHASE: sub })} teams={teams} catMeta={{ icon: '🐾', label: 'Animaux' }} isVPlayer teamName="Rouges" onStop={() => {}} />)
        expect(container.textContent, sub).toMatch(/Animaux/)
        if (re) expect(container.textContent, sub).toMatch(re)
        expect(container.textContent, sub).not.toMatch(/COMBIENPOURCECHIEN/)
        unmount()
      }
    }
  })

  it('l\'indice n\'est plus dans la ligne du haut (équipe + cagnotte seulement)', () => {
    const { container } = render(<RoueTv gameState={gs({ question: { TYPE: 'ROUE', ROUE_PRESENTATION: 'Chanson de Line Renaud' } })} teams={teams} catMeta={{ icon: '🐾', label: 'Animaux' }} isVPlayer teamName="Rouges" />)
    const top = container.querySelector('.roue-vp-top')
    expect(top.textContent).not.toMatch(/Animaux|Chanson/)
    expect(container.textContent).toMatch(/Chanson de Line Renaud/)
  })

  it('tablette : pas de points de suspension sur l\'indice (CSS)', () => {
    const css = readFileSync(join(here, 'RoueTv.css'), 'utf8')
    const blocks = css.match(/\.roue-vp-(hint|pres|cat|clue)[^{]*\{[^}]*\}/g) || []
    for (const b of blocks) expect(b).not.toMatch(/text-overflow\s*:\s*ellipsis/)
  })

  it('MONTÉE (FULL_SPEED=false) : la roue est affichée mais jamais tapable, aucun BUTTON, quelle que soit l\'équipe', () => {
    for (const team of ['Rouges', 'Bleus']) {
      const onStop = vi.fn()
      const { container, queryByRole, unmount } = render(<RoueTv gameState={gs(climb())} teams={teams} isVPlayer teamName={team} onStop={onStop} />)
      expect(queryByRole('button', TAP), team).toBeNull()
      expect(container.querySelectorAll('button').length, team).toBe(0)
      fireEvent.click(container.querySelector('[data-testid="fortune-wheel"]'))
      expect(onStop, team).not.toHaveBeenCalled()
      expect(container.textContent, team).not.toMatch(/Touchez la roue/)
      unmount()
    }
  })

  it('MONTÉE : message « La roue prend de la vitesse », y compris pour l\'autre équipe', () => {
    for (const team of ['Rouges', 'Bleus']) {
      const { container, unmount } = render(<RoueTv gameState={gs(climb())} teams={teams} isVPlayer teamName={team} onStop={() => {}} />)
      expect(container.textContent, team).toMatch(/prend de la vitesse/i)
      unmount()
    }
  })

  it('la roue devient tapable à FULL_SPEED=true (UPDATE du serveur), pour l\'équipe qui a la main seulement', () => {
    const onStop = vi.fn()
    const { rerender, getByRole, queryByRole, container } = render(<RoueTv gameState={gs(climb())} teams={teams} isVPlayer teamName="Rouges" onStop={onStop} />)
    expect(queryByRole('button', TAP)).toBeNull()
    rerender(<RoueTv gameState={gs(full())} teams={teams} isVPlayer teamName="Rouges" onStop={onStop} />)
    expect(container.textContent).toMatch(/Touchez la roue pour l'arrêter/)
    fireEvent.click(getByRole('button', TAP))
    expect(onStop).toHaveBeenCalledTimes(1)
    rerender(<RoueTv gameState={gs(full())} teams={teams} isVPlayer teamName="Bleus" onStop={onStop} />)
    expect(queryByRole('button', TAP)).toBeNull()
    expect(container.textContent).toMatch(/Les Rouges vont arrêter la roue/)
  })

  it('montage direct en SPINNING à pleine vitesse (client connecté en cours de rotation) : tapable tout de suite', () => {
    const onStop = vi.fn()
    const { getByRole } = render(<RoueTv gameState={gs(full())} teams={teams} isVPlayer teamName="Rouges" onStop={onStop} />)
    fireEvent.click(getByRole('button', TAP))
    expect(onStop).toHaveBeenCalledTimes(1)
  })

  it('montage direct en SPINNING pendant la montée : pas tapable (FULL_SPEED fait foi)', () => {
    const { queryByRole } = render(<RoueTv gameState={gs(climb())} teams={teams} isVPlayer teamName="Rouges" onStop={() => {}} />)
    expect(queryByRole('button', TAP)).toBeNull()
  })

  it('FULL_SPEED absent (ancien serveur) : jamais tapable', () => {
    const w = { ...gs().ROUE_WHEEL }
    const { queryByRole } = render(<RoueTv gameState={gs({ ROUE_SUBPHASE: 'SPINNING', ROUE_WHEEL: w })} teams={teams} isVPlayer teamName="Rouges" onStop={() => {}} />)
    expect(queryByRole('button', TAP)).toBeNull()
  })

  it('TV (statique) : annonce de montée puis « buzzez » seulement à pleine vitesse', () => {
    expect(text(roueAnnouncement(gs(climb())))).toMatch(/vitesse/i)
    expect(text(roueAnnouncement(gs(climb())))).not.toMatch(/buzzez/i)
    expect(text(roueAnnouncement(gs(full())))).toMatch(/buzzez/i)
  })

  it('cagnotte de son équipe uniquement', () => {
    const { container } = render(<RoueTv gameState={gs({ ROUE_TEAM_BANK: { Rouges: 1200, Bleus: 55 } })} teams={teams} isVPlayer teamName="Bleus" />)
    expect(container.querySelector('.roue-vp-bank').textContent).toMatch(/55/)
    expect(container.querySelector('.roue-vp-bank').textContent).not.toMatch(/1\s?200/)
  })

  it('résolution : message de victoire pour son équipe, sinon le nom du gagnant, sinon « dévoilée »', () => {
    const win = render(<RoueTv gameState={gs({ ROUE_SUBPHASE: 'SOLVED', ROUE_WINNER: 'Bleus', ROUE_AWARDS: { Bleus: 800 } })} teams={teams} isVPlayer teamName="Bleus" />)
    expect(win.container.querySelector('.roue-vp-win').textContent).toMatch(/Votre équipe.*800/)
    cleanup()
    const lose = render(<RoueTv gameState={gs({ ROUE_SUBPHASE: 'SOLVED', ROUE_WINNER: 'Bleus' })} teams={teams} isVPlayer teamName="Rouges" />)
    expect(lose.container.querySelector('.roue-vp-win').textContent).toMatch(/Bleus/)
    cleanup()
    const none = render(<RoueTv gameState={gs({ ROUE_SUBPHASE: 'SOLVED' })} teams={teams} isVPlayer teamName="Rouges" />)
    expect(none.container.querySelector('.roue-vp-win').textContent).toMatch(/dévoilée/i)
  })
})

describe('RoueGameBoard — retournement', () => {
  const rows = (...r) => ({ COLS: 14, ROWS: r })
  it('les cases retournées le sont dans l\'ordre de lecture (rang croissant)', () => {
    const { container, rerender } = render(<RoueGameBoard board={rows('*_*', '_*')} flipIntervalMs={100} />)
    rerender(<RoueGameBoard board={rows('N_N', '_N')} flipIntervalMs={100} />)
    const ranks = Array.from(container.querySelectorAll('.rgb-cell.flip')).map(e => Number(e.style.getPropertyValue('--rgb-i')))
    expect(ranks).toEqual([0, 1, 2])
  })

  it('l\'intervalle de retournement vient du serveur (FLIP_INTERVAL_MS)', () => {
    const { container } = render(<RoueGameBoard board={rows('_')} flipIntervalMs={450} />)
    expect(container.querySelector('.rgb-board').style.getPropertyValue('--rgb-flip')).toBe('450ms')
  })

  it('plateau toujours à 4 lignes × 14 colonnes (56 cases)', () => {
    const { container } = render(<RoueGameBoard board={rows('AB')} />)
    expect(container.querySelectorAll('.rgb-cell')).toHaveLength(56)
  })

  it('plateau absent : rendu vide sans exception', () => {
    const { container } = render(<RoueGameBoard board={undefined} />)
    expect(container.querySelectorAll('.rgb-cell')).toHaveLength(56)
  })
})

describe('roueGates — portes d\'affichage (réplique du contrat §5/§8)', () => {
  const g = (sub, over = {}) => roueGates(gs({ ROUE_SUBPHASE: sub, ...over }))
  it('une sous-phase = une action principale', () => {
    expect(g('TURN')).toMatchObject({ canSpin: true, canStop: false, canLetter: false, canFlip: false, canAnswer: false, canNext: false })
    expect(g('SPINNING')).toMatchObject({ canSpin: false, canStop: true })
    expect(g('LETTER')).toMatchObject({ canLetter: true, canStop: false })
    expect(g('LIT')).toMatchObject({ canFlip: true })
    expect(g('ANSWER')).toMatchObject({ canAnswer: true, canNext: true, canSame: true })
    expect(g('TURN_END')).toMatchObject({ canNext: true, canAnswer: false })
  })
  it('plus de consonne : pas de lancer, mais réponse et fin de tour depuis TURN', () => {
    expect(g('TURN', { ROUE_CONSONANTS_LEFT: false })).toMatchObject({ canSpin: false, canAnswer: true, canNext: true, canPass: false })
  })
  it('SOLO : ni passe ni « Équipe suivante »', () => {
    const solo = { question: { TYPE: 'ROUE', ROUE_MODE: 'SOLO' } }
    expect(g('TURN_END', solo)).toMatchObject({ canNextTeam: false, canSame: true })
    expect(g('TURN', solo).canPass).toBe(false)
  })
  it('dévoiler : toute sous-phase non vide sauf SOLVED', () => {
    expect(g('SOLVED').canReveal).toBe(false)
    expect(g('').canReveal).toBe(false)
    expect(g('SPINNING').canReveal).toBe(true)
  })
})

describe('makeRoueActions — payloads du contrat §8.1', () => {
  it('chaque geste envoie la bonne action et le bon payload', () => {
    const send = vi.fn(); const a = makeRoueActions(send)
    a.setTeams(['red', 'blue']); a.spin(); a.stop(); a.letter('N'); a.flip(); a.answer(); a.next('NEXT'); a.next('SAME'); a.next(undefined); a.pass(); a.reveal()
    expect(send.mock.calls).toEqual([
      ['ROUE_SET_TEAMS', { TEAMS: ['red', 'blue'] }], ['ROUE_SPIN', {}], ['ROUE_STOP', {}], ['ROUE_LETTER', { LETTER: 'N' }],
      ['ROUE_FLIP', {}], ['ROUE_ANSWER', {}], ['ROUE_NEXT', { HAND: 'NEXT' }], ['ROUE_NEXT', { HAND: 'SAME' }],
      ['ROUE_NEXT', { HAND: 'SAME' }], ['ROUE_PASS', {}], ['ROUE_REVEAL', {}],
    ])
  })
})
