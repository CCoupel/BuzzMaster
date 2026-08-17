import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import AnimMotionGrid from './AnimMotionGrid'
import { getMotionGridCols, getMotionCardCoord } from '../utils/motionGrid'

// ---------------------------------------------------------------------------
// AnimMotionGrid — grille MEMOTION tactile de /anim (#160/F4, T3 — test
// central de correspondance positionnelle + volet états de carte).
//
// Plan : _work/reports/plan-20260817-160500.md §F4. Calquée sur
// AnimMemoryGrid (conventions `.anim-motion-*`, cibles tactiles ≥62px,
// AUCUNE logique de jeu côté client) mais PAS une réutilisation de code :
// modèle de données différent (MOTION_CARDS vs MEMORY_PAIRS), formule de
// colonnes différente (utils/motionGrid.js, PAS memoryGrid.js), états de
// carte différents (UNPLAYED/DONE, pas de "retournée"/"trouvée").
//
// Contrat de props (§F4) : question, phase, subphase, cardStates, cardTeams,
// currentTeam, selectedId, teams, onSelect.
// ---------------------------------------------------------------------------

const cssPath = path.join(path.dirname(fileURLToPath(import.meta.url)), 'AnimMotionGrid.css')

function makeCards(n, overrides = {}) {
  return Array.from({ length: n }, (_, i) => ({
    ID: `c${i + 1}`,
    RECTO_THEME: `Thème ${i + 1}`,
    RECTO_IMAGE: '',
    DIFFICULTY: (i % 3) + 1,
    ...overrides,
  }))
}

function makeQuestion(n, overrides = {}) {
  return { ID: '1', MOTION_CARDS: makeCards(n), ...overrides }
}

function baseProps(overrides = {}) {
  return {
    question: makeQuestion(6), // 6 cartes -> 3 colonnes (getMotionGridCols)
    phase: 'STARTED',
    subphase: 'GRID',
    cardStates: {},
    cardTeams: {},
    currentTeam: null,
    selectedId: null,
    teams: {},
    onSelect: vi.fn(),
    ...overrides,
  }
}

describe('AnimMotionGrid — absence si pas de question MEMOTION', () => {
  it('ne rend rien si question est null', () => {
    const { container } = render(<AnimMotionGrid {...baseProps({ question: null })} />)
    expect(container.firstChild).toBeNull()
  })

  it('ne rend rien si MOTION_CARDS est vide/absente', () => {
    const { container } = render(<AnimMotionGrid {...baseProps({ question: { ID: '1', MOTION_CARDS: [] } })} />)
    expect(container.firstChild).toBeNull()
  })
})

// ---------------------------------------------------------------------------
// Quatre états de carte (plan §F4) :
//   - UNPLAYED + subphase GRID + phase STARTED -> cliquable (dégradé violet)
//   - UNPLAYED hors GRID (dont MEMORIZE) -> éteinte, non cliquable
//   - DONE avec gagnant -> couleur d'équipe, non cliquable
//   - DONE sans gagnant -> gris neutre, "–", non cliquable
// ---------------------------------------------------------------------------

describe('AnimMotionGrid — états de carte', () => {
  it('UNPLAYED, subphase GRID, phase STARTED : cliquable, émet onSelect(card.ID)', () => {
    const onSelect = vi.fn()
    const { container } = render(<AnimMotionGrid {...baseProps({ onSelect })} />)
    const card = container.querySelector('.anim-motion-card')
    expect(card.disabled).toBe(false)
    card.click()
    expect(onSelect).toHaveBeenCalledWith('c1')
  })

  it('UNPLAYED, subphase MEMORIZE : éteinte, non cliquable (mémorisation en cours)', () => {
    const onSelect = vi.fn()
    const { container } = render(<AnimMotionGrid {...baseProps({ subphase: 'MEMORIZE', onSelect })} />)
    const card = container.querySelector('.anim-motion-card')
    expect(card.disabled).toBe(true)
    expect(card.className).toMatch(/\banim-motion-card-inert\b/)
    card.click()
    expect(onSelect).not.toHaveBeenCalled()
  })

  it('UNPLAYED, subphase SELECTED/QUESTION/REVEAL : non cliquable (le geste n\'est plus la grille)', () => {
    ;['SELECTED', 'QUESTION', 'REVEAL'].forEach((subphase) => {
      const { container, unmount } = render(<AnimMotionGrid {...baseProps({ subphase })} />)
      const card = container.querySelector('.anim-motion-card')
      expect(card.disabled, `subphase ${subphase}`).toBe(true)
      unmount()
    })
  })

  it('UNPLAYED, subphase GRID, mais phase !== STARTED (ex. PAUSED) : non cliquable', () => {
    const onSelect = vi.fn()
    const { container } = render(<AnimMotionGrid {...baseProps({ phase: 'PAUSED', onSelect })} />)
    const card = container.querySelector('.anim-motion-card')
    expect(card.disabled).toBe(true)
    card.click()
    expect(onSelect).not.toHaveBeenCalled()
  })

  // ⚠️ Convention de classes/style pour l'état DONE — pas encore stabilisée
  // côté dev-frontend au moment de l'écriture de ce test (plusieurs
  // variantes observées en cours d'implémentation : `.anim-motion-card-title`
  // vs `-theme`, `--anim-motion-winner-color` vs `-team-color`, mention
  // "PERSONNE –" inline vs classe `.anim-motion-card-nowinner` dédiée).
  // Ces DEUX tests fixent le contrat COMPORTEMENTAL exigé par le plan §F4
  // ("DONE → couleur de l'équipe de cardTeams[ID], nom d'équipe en pied,
  // non cliquable ; gris neutre + '–' si aucun gagnant") sur la variante la
  // plus récente observée : `.anim-motion-card-title` + custom property
  // `--anim-motion-winner-color` + mention textuelle inline (pas de classe
  // `-nowinner` séparée). Si dev-frontend a convergé sur une autre variante
  // au moment de la review, ajuster CES DEUX sélecteurs uniquement — le
  // reste du fichier (correspondance positionnelle, contenu, HUD, cibles
  // tactiles) est stable et ne dépend pas de cette convention.
  it('DONE avec gagnant : couleur d\'équipe posée, non cliquable, nom d\'équipe affiché', () => {
    const onSelect = vi.fn()
    const { container } = render(
      <AnimMotionGrid {...baseProps({
        cardStates: { c1: 'DONE' },
        cardTeams: { c1: 'Les Rouges' },
        teams: { 'Les Rouges': { COLOR: [255, 0, 0] } },
        onSelect,
      })} />
    )
    const card = container.querySelector('.anim-motion-card')
    expect(card.className).toMatch(/\banim-motion-card-done\b/)
    expect(card.disabled).toBe(true)
    expect(card.style.getPropertyValue('--anim-motion-winner-color')).toBeTruthy()
    expect(card.querySelector('.anim-motion-card-team').textContent).toContain('Les Rouges')
    card.click()
    expect(onSelect).not.toHaveBeenCalled()
  })

  it('DONE sans gagnant ("sans vainqueur") : gris neutre (repli de couleur), mention "–", non cliquable', () => {
    const { container } = render(
      <AnimMotionGrid {...baseProps({ cardStates: { c1: 'DONE' }, cardTeams: {} })} />
    )
    const card = container.querySelector('.anim-motion-card')
    expect(card.className).toMatch(/\banim-motion-card-done\b/)
    expect(card.disabled).toBe(true)
    expect(card.style.getPropertyValue('--anim-motion-winner-color')).toBeTruthy() // repli neutre, pas une couleur d'équipe
    expect(card.querySelector('.anim-motion-card-team').textContent).toContain('–')
  })

  it('une grille mixte (jouées + non jouées) applique le bon état à CHAQUE carte indépendamment', () => {
    const { container } = render(
      <AnimMotionGrid {...baseProps({
        question: makeQuestion(3),
        cardStates: { c1: 'DONE', c2: 'UNPLAYED' },
        cardTeams: { c1: 'Les Rouges' },
        teams: { 'Les Rouges': { COLOR: [255, 0, 0] } },
      })} />
    )
    const cards = container.querySelectorAll('.anim-motion-card')
    expect(cards[0].className).toMatch(/\banim-motion-card-done\b/)
    expect(cards[1].disabled).toBe(false) // c2 UNPLAYED, GRID+STARTED -> cliquable
    expect(cards[2].disabled).toBe(false) // c3 UNPLAYED (absent de cardStates -> UNPLAYED par défaut)
  })
})

// ---------------------------------------------------------------------------
// Contenu de carte — thème + étoiles en mode normal ; coordonnée SEULE, sans
// étoiles, en mode Secret (les étoiles trahiraient la difficulté de la
// carte, comportement identique à /tv, PlayerDisplay.jsx:2107-2113).
// ---------------------------------------------------------------------------

describe('AnimMotionGrid — contenu (thème/étoiles vs coordonnée, mode Secret)', () => {
  it('mode normal, subphase GRID : thème + étoiles (DIFFICULTY) affichés, pas de coordonnée', () => {
    const question = makeQuestion(1, { MOTION_CARDS: [{ ID: 'c1', RECTO_THEME: 'Cinéma', DIFFICULTY: 2 }] })
    const { container } = render(<AnimMotionGrid {...baseProps({ question })} />)
    expect(screen.getByText('Cinéma')).toBeInTheDocument()
    expect(container.querySelector('.anim-motion-card-stars').textContent).toBe('★★')
    expect(container.querySelector('.anim-motion-card-coord')).toBeNull()
  })

  it('mode Secret (MOTION_MEMORIZE_DURATION > 0), subphase GRID : coordonnée SEULE, ni thème ni étoiles', () => {
    const question = {
      ID: '1',
      MOTION_MEMORIZE_DURATION: 15,
      MOTION_CARDS: [{ ID: 'c1', RECTO_THEME: 'Cinéma', DIFFICULTY: 2 }],
    }
    const { container } = render(<AnimMotionGrid {...baseProps({ question })} />)
    expect(screen.queryByText('Cinéma')).not.toBeInTheDocument()
    expect(container.querySelector('.anim-motion-card-stars')).toBeNull()
    expect(container.querySelector('.anim-motion-card-coord')).not.toBeNull()
    expect(container.querySelector('.anim-motion-card-coord').textContent).toBe('A1')
  })

  // ⚠️ Parité STRICTE avec /tv (AC6, PlayerDisplay.jsx : condition
  // `isSecretMode && subphase === 'GRID'`, PAS `isSecretMode` seul) — c'est
  // le point même du mode Secret : les joueurs MÉMORISENT les thèmes
  // pendant MEMORIZE (mockup §MEMORIZE : "Cinéma ★, Sport ★★★..." bien
  // visibles), et c'est SEULEMENT au passage en GRID que le thème est
  // remplacé par la coordonnée. Si ce test est rouge, la garde de
  // AnimMotionGrid.jsx ne teste que `isSecretMode` sans `subphase === 'GRID'`
  // — divergence avec /tv à corriger côté dev-frontend (F4), pas un test à
  // assouplir.
  it('mode Secret, subphase MEMORIZE : thème + étoiles affichés (les joueurs mémorisent AVANT que la coordonnée ne remplace le thème, parité /tv AC6)', () => {
    const question = {
      ID: '1',
      MOTION_MEMORIZE_DURATION: 15,
      MOTION_CARDS: [{ ID: 'c1', RECTO_THEME: 'Cinéma', DIFFICULTY: 1 }],
    }
    const { container } = render(<AnimMotionGrid {...baseProps({ question, subphase: 'MEMORIZE' })} />)
    expect(screen.getByText('Cinéma')).toBeInTheDocument()
    expect(container.querySelector('.anim-motion-card-coord')).toBeNull()
  })
})

// ---------------------------------------------------------------------------
// #160/T3 — TEST CENTRAL : coordonnées et colonnes EXCLUSIVEMENT via
// utils/motionGrid.js, jamais recalculées localement (risque R1, motif
// identique à #159/T3 pour MEMORY).
// ---------------------------------------------------------------------------

describe('AnimMotionGrid — correspondance positionnelle avec utils/motionGrid.js (#160/T3, central)', () => {
  it('utilise EXACTEMENT le même nombre de colonnes que getMotionGridCols pour le même nombre de cartes', () => {
    const question = makeQuestion(20)
    const expectedCols = getMotionGridCols(20)
    const { container } = render(<AnimMotionGrid {...baseProps({ question })} />)
    const grid = container.querySelector('.anim-motion-grid')
    expect(grid.style.getPropertyValue('--anim-motion-cols')).toBe(String(expectedCols))
  })

  it.each([
    [4, 2], [6, 3], [12, 4], [20, 5], [24, 5], // reprend les bornes de motionGrid.test.js
  ])('%i cartes -> %i colonnes, identique à la formule partagée', (count, expectedCols) => {
    const question = makeQuestion(count)
    const { container } = render(<AnimMotionGrid {...baseProps({ question })} />)
    const grid = container.querySelector('.anim-motion-grid')
    expect(grid.style.getPropertyValue('--anim-motion-cols')).toBe(String(expectedCols))
  })

  it('en mode Secret, chaque coordonnée affichée correspond à getMotionCardCoord(index, cols) pour SON index', () => {
    const question = {
      ID: '1',
      MOTION_MEMORIZE_DURATION: 10,
      MOTION_CARDS: makeCards(6), // -> 3 colonnes
    }
    const cols = getMotionGridCols(6)
    const { container } = render(<AnimMotionGrid {...baseProps({ question })} />)
    const coords = Array.from(container.querySelectorAll('.anim-motion-card-coord')).map(el => el.textContent)
    const expected = question.MOTION_CARDS.map((_, idx) => getMotionCardCoord(idx, cols))
    expect(coords).toEqual(expected)
  })

  it('rend les cartes dans le MÊME ORDRE que question.MOTION_CARDS (pas de mélange local, contrairement à MEMORY)', () => {
    const question = makeQuestion(4)
    const { container } = render(<AnimMotionGrid {...baseProps({ question })} />)
    const themes = Array.from(container.querySelectorAll('.anim-motion-card-title')).map(el => el.textContent)
    expect(themes).toEqual(question.MOTION_CARDS.map(c => c.RECTO_THEME))
  })
})

// ---------------------------------------------------------------------------
// HUD — équipe au tour + cartes jouées/total (plan §F4).
// ---------------------------------------------------------------------------

describe('AnimMotionGrid — HUD (équipe au tour, compteur de cartes jouées)', () => {
  it('affiche "au tour de <équipe>" quand currentTeam est renseignée', () => {
    render(<AnimMotionGrid {...baseProps({ currentTeam: 'Les Bleus' })} />)
    expect(screen.getByText('au tour de')).toBeInTheDocument()
    expect(screen.getByText('Les Bleus')).toBeInTheDocument()
  })

  it('absent de "au tour de" quand currentTeam est vide (mode SOLO)', () => {
    render(<AnimMotionGrid {...baseProps({ currentTeam: '' })} />)
    expect(screen.queryByText('au tour de')).not.toBeInTheDocument()
  })

  it('compteur "jouées/total" reflète cardStates (DONE) sur le total de MOTION_CARDS', () => {
    const question = makeQuestion(4)
    render(<AnimMotionGrid {...baseProps({ question, cardStates: { c1: 'DONE', c2: 'DONE' } })} />)
    expect(screen.getByText('2/4')).toBeInTheDocument()
  })
})

// ---------------------------------------------------------------------------
// Cibles tactiles ≥62px (F9/T3) — vérification par lecture directe du texte
// source de AnimMotionGrid.css, jsdom n'appliquant jamais une feuille de
// style externe (même limite technique que GamePage.ardoise-grid.test.jsx).
// ---------------------------------------------------------------------------

describe('AnimMotionGrid.css — cibles tactiles ≥62px (#160/F9)', () => {
  it('la règle .anim-motion-card fixe min-height à 62px (ou plus), même convention que .anim-memory-card', () => {
    const css = fs.readFileSync(cssPath, 'utf-8')
    const match = css.match(/\.anim-motion-card\s*\{[^}]*min-height:\s*(\d+)px/)
    expect(match, 'aucune règle min-height trouvée pour .anim-motion-card dans AnimMotionGrid.css').not.toBeNull()
    expect(Number(match[1])).toBeGreaterThanOrEqual(62)
  })
})
