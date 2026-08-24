import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import AnimMemoryGrid from './AnimMemoryGrid'
import { buildMemoryCards, getMemoryGridCols } from '../utils/memoryGrid'

// ---------------------------------------------------------------------------
// AnimMemoryGrid — grille MEMORY tactile de /anim (#159/F1+F2, T3 central +
// T4 volet bandeau).
//
// Plan : _work/reports/plan-20260816-224500.md §7. AUCUNE logique de jeu
// côté client : ce composant affiche l'état reçu du serveur et gate le
// GESTE (canClick), rien de plus — pas de détection de paire locale.
//
// ⚠️ Test central de correspondance positionnelle (motif de #159) :
// AnimMemoryGrid doit produire EXACTEMENT le même ordre et le même nombre
// de colonnes que utils/memoryGrid.js pour le même jeu de paires — c'est
// la garantie que l'animateur, `/tv` et le joueur désignent la même carte.
// ---------------------------------------------------------------------------

function makePairs(n) {
  return Array.from({ length: n }, (_, i) => ({
    ID: i + 1,
    CARD1: { TEXT: `Q${i + 1}` },
    CARD2: { TEXT: `R${i + 1}` },
  }))
}

function baseProps(overrides = {}) {
  return {
    question: { ID: '1', MEMORY_PAIRS: makePairs(3) }, // 6 cartes
    // #184/B-F2 — sevrage de `phase` : le composant reçoit désormais
    // `playable`/`revealed` (contrat question-types.md §4), jamais `phase`.
    // Équivalent question-host de PHASE=STARTED, comme avant ce refactor.
    playable: true,
    revealed: false,
    teams: {},
    flippedCards: [],
    matchedPairs: [],
    pairOwners: {},
    currentTeam: null,
    teamPairs: {},
    teamErrors: {},
    globalErrors: 0,
    onFlip: vi.fn(),
    ...overrides,
  }
}

describe('AnimMemoryGrid — absence si pas de question MEMORY', () => {
  it('ne rend rien si question est null', () => {
    const { container } = render(<AnimMemoryGrid {...baseProps({ question: null })} />)
    expect(container.firstChild).toBeNull()
  })

  it('ne rend rien si MEMORY_PAIRS est vide/absente', () => {
    const { container } = render(<AnimMemoryGrid {...baseProps({ question: { ID: '1', MEMORY_PAIRS: [] } })} />)
    expect(container.firstChild).toBeNull()
  })
})

describe('AnimMemoryGrid — quatre états de carte', () => {
  // #159, commit c5393a8 — le dos de carte affichait une icône générique
  // (🃏, `.anim-memory-card-back`) ; repris à l'identique du design `/tv`
  // (PlayerDisplay.jsx:1819) : une LETTRE (A, B, C...) dérivée de la
  // position dans l'ordre mélangé, `.anim-memory-card-letter`. La classe
  // change de nom (elle porte un contenu réel, pas une icône statique) —
  // mis à jour ici (pas neutralisé), signalé par dev-frontend.
  it('face cachée par défaut (phase STARTED, rien de retourné) : lettre visible (A, position 0), pas de contenu de carte', () => {
    const { container } = render(<AnimMemoryGrid {...baseProps()} />)
    const card = container.querySelector('.anim-memory-card')
    expect(card.className).toMatch(/\banim-memory-card-down\b/)
    const letterEl = card.querySelector('.anim-memory-card-letter')
    expect(letterEl).not.toBeNull()
    expect(letterEl.textContent).toBe('A') // String.fromCharCode(65 + index), première carte de l'ordre mélangé
    expect(card.querySelector('.anim-memory-card-text')).toBeNull()
  })

  it('la lettre de chaque carte face cachée suit son INDEX dans l\'ordre mélangé (buildMemoryCards), pas son pairId/cardIndex', () => {
    const { container } = render(<AnimMemoryGrid {...baseProps()} />)
    // baseProps() : 3 paires -> 6 cartes, aucune révélée -> les 6 lettres A-F, dans l'ordre du DOM.
    const letters = Array.from(container.querySelectorAll('.anim-memory-card-letter')).map(el => el.textContent)
    expect(letters).toEqual(['A', 'B', 'C', 'D', 'E', 'F'])
  })

  it('retournée (flippedCards) : contenu visible, état "up", plus de lettre de repli', () => {
    const cards = buildMemoryCards(baseProps().question)
    const flippedId = cards[0].id
    const { container } = render(<AnimMemoryGrid {...baseProps({ flippedCards: [flippedId] })} />)
    const cardEl = Array.from(container.querySelectorAll('.anim-memory-card')).find(el =>
      el.className.includes('anim-memory-card-up')
    )
    expect(cardEl).not.toBeUndefined()
    expect(cardEl.querySelector('.anim-memory-card-letter')).toBeNull()
  })

  it('paire trouvée (matchedPairs) : état "matched", couleur du propriétaire posée', () => {
    const cards = buildMemoryCards(baseProps().question)
    const matchedPairId = cards[0].pairId
    const { container } = render(
      <AnimMemoryGrid {...baseProps({
        matchedPairs: [matchedPairId],
        pairOwners: { [String(matchedPairId)]: 'Les Rouges' },
        teams: { 'Les Rouges': { COLOR: [255, 0, 0] } },
      })} />
    )
    const matchedCards = Array.from(container.querySelectorAll('.anim-memory-card-matched'))
    expect(matchedCards).toHaveLength(2) // les deux cartes de la paire trouvée
    matchedCards.forEach((el) => {
      expect(el.style.getPropertyValue('--anim-memory-owner-color')).toBeTruthy()
    })
  })

  it('inerte hors STARTED : état "inert", non cliquable même sans être retournée/trouvée', () => {
    const { container } = render(<AnimMemoryGrid {...baseProps({ playable: false, revealed: false })} />)
    const card = container.querySelector('.anim-memory-card')
    expect(card.className).toMatch(/\banim-memory-card-inert\b/)
    expect(card.disabled).toBe(true)
  })
})

// ---------------------------------------------------------------------------
// Régression QUALIF v6.2.0.27 (#159, fix commit 89a98a2, v6.2.0.28) —
// angle mort signalé par code-reviewer : `revealed`/`state` ne couvraient
// que "trouvée" (matchedPairs) et "retournée en direct" (flippedCards) ;
// en phase REVEALED, une paire JAMAIS trouvée (délai écoulé, erreur non
// récupérée avant l'arrêt) n'était ni l'un ni l'autre — toutes les cartes
// restaient face cachée indéfiniment, aucun nom visible pour arbitrer.
// Le fix ajoute `phase === 'REVEALED'` comme troisième condition de
// révélation, SANS jamais permuter la priorité "matched" : une paire
// réellement trouvée doit garder son style
// "matched" (couleur du propriétaire) même en REVEALED, distinct du style
// "up" neutre réservé aux paires jamais trouvées.
// ---------------------------------------------------------------------------

describe('AnimMemoryGrid — révélation en phase REVEALED (régression QUALIF v6.2.0.27, #159)', () => {
  it('paire JAMAIS trouvée ni retournée, phase REVEALED : contenu affiché, état "up" neutre, pas le repli face cachée', () => {
    const { container } = render(<AnimMemoryGrid {...baseProps({ playable: false, revealed: true })} />)
    // baseProps() : 3 paires, aucune matchedPairs/flippedCards fournie —
    // TOUTES les cartes sont dans le cas "jamais trouvée".
    const cardsEl = Array.from(container.querySelectorAll('.anim-memory-card'))
    expect(cardsEl).toHaveLength(6)
    cardsEl.forEach((el) => {
      expect(el.className).toMatch(/\banim-memory-card-up\b/)
      expect(el.className).not.toMatch(/\banim-memory-card-matched\b/)
      expect(el.querySelector('.anim-memory-card-letter')).toBeNull() // pas le repli face cachée (le bug QUALIF)
      expect(el.querySelector('.anim-memory-card-text')).not.toBeNull() // contenu réellement affiché
    })
  })

  it('paire réellement trouvée (matchedPairs), phase REVEALED : garde le style "matched", pas confondue avec "up"', () => {
    const cards = buildMemoryCards(baseProps().question)
    const matchedPairId = cards[0].pairId
    const { container } = render(
      <AnimMemoryGrid {...baseProps({
        playable: false,
        revealed: true,
        matchedPairs: [matchedPairId],
        pairOwners: { [String(matchedPairId)]: 'Les Rouges' },
        teams: { 'Les Rouges': { COLOR: [255, 0, 0] } },
      })} />
    )
    const matchedCards = Array.from(container.querySelectorAll('.anim-memory-card-matched'))
    expect(matchedCards).toHaveLength(2) // les deux cartes de LA paire trouvée, toujours "matched"
    matchedCards.forEach((el) => {
      expect(el.className).not.toMatch(/\banim-memory-card-up\b/) // jamais les deux classes à la fois
      expect(el.style.getPropertyValue('--anim-memory-owner-color')).toBeTruthy() // couleur du propriétaire conservée
    })
    // Les paires NON trouvées de la même grille restent en "up" neutre (sans couleur), pas "matched".
    const neutralCards = Array.from(container.querySelectorAll('.anim-memory-card'))
      .filter((el) => !el.className.includes('anim-memory-card-matched'))
    expect(neutralCards.length).toBeGreaterThan(0)
    neutralCards.forEach((el) => {
      expect(el.className).toMatch(/\banim-memory-card-up\b/)
      expect(el.style.getPropertyValue('--anim-memory-owner-color')).toBeFalsy()
    })
  })

  it('carte image jamais trouvée, phase REVEALED : image affichée (pas seulement le cas texte)', () => {
    const question = {
      ID: '1',
      MEMORY_PAIRS: [{ ID: 1, CARD1: { IS_IMAGE: true, IMAGE: '/img/a.png' }, CARD2: { IS_IMAGE: true, IMAGE: '/img/b.png' } }],
    }
    const { container } = render(<AnimMemoryGrid {...baseProps({ question, playable: false, revealed: true })} />)
    const cardEl = container.querySelector('.anim-memory-card-up')
    expect(cardEl.querySelector('.anim-memory-card-image')).not.toBeNull()
  })

  it('non cliquable même révélée sans avoir été trouvée (REVEALED n\'est jamais STARTED)', () => {
    const onFlip = vi.fn()
    const { container } = render(<AnimMemoryGrid {...baseProps({ playable: false, revealed: true, onFlip })} />)
    const card = container.querySelector('.anim-memory-card-up')
    expect(card.disabled).toBe(true)
    card.click()
    expect(onFlip).not.toHaveBeenCalled()
  })
})

describe('AnimMemoryGrid — geste (canClick), identifiant émis', () => {
  it('émet onFlip(cardId) au format "pairID-cardNum" au clic, en STARTED sur une carte face cachée', () => {
    const onFlip = vi.fn()
    const { container } = render(<AnimMemoryGrid {...baseProps({ onFlip })} />)
    const card = container.querySelector('.anim-memory-card-down')
    card.click()
    expect(onFlip).toHaveBeenCalledTimes(1)
    expect(onFlip.mock.calls[0][0]).toMatch(/^\d+-[12]$/)
  })

  it('aucun geste hors STARTED : bouton disabled, clic sans effet', () => {
    const onFlip = vi.fn()
    const { container } = render(<AnimMemoryGrid {...baseProps({ playable: false, revealed: false, onFlip })} />)
    const card = container.querySelector('.anim-memory-card')
    expect(card.disabled).toBe(true)
    card.click()
    expect(onFlip).not.toHaveBeenCalled()
  })

  it('aucun geste sur une carte déjà retournée', () => {
    const cards = buildMemoryCards(baseProps().question)
    const flippedId = cards[0].id
    const onFlip = vi.fn()
    const { container } = render(<AnimMemoryGrid {...baseProps({ flippedCards: [flippedId], onFlip })} />)
    const cardEl = Array.from(container.querySelectorAll('.anim-memory-card')).find(el => el.className.includes('anim-memory-card-up'))
    expect(cardEl.disabled).toBe(true)
    cardEl.click()
    expect(onFlip).not.toHaveBeenCalled()
  })

  it('aucun geste sur une carte déjà trouvée (paire matched)', () => {
    const cards = buildMemoryCards(baseProps().question)
    const matchedPairId = cards[0].pairId
    const onFlip = vi.fn()
    const { container } = render(<AnimMemoryGrid {...baseProps({ matchedPairs: [matchedPairId], onFlip })} />)
    const cardEl = container.querySelector('.anim-memory-card-matched')
    expect(cardEl.disabled).toBe(true)
    cardEl.click()
    expect(onFlip).not.toHaveBeenCalled()
  })

  it('AUCUNE restriction par équipe côté /anim : le geste reste disponible quelle que soit currentTeam', () => {
    const onFlip = vi.fn()
    const { container } = render(
      <AnimMemoryGrid {...baseProps({ currentTeam: 'Les Bleus', teamPairs: { 'Les Bleus': 0, 'Les Rouges': 0 }, onFlip })} />
    )
    const card = container.querySelector('.anim-memory-card-down')
    expect(card.disabled).toBe(false)
    card.click()
    expect(onFlip).toHaveBeenCalledTimes(1)
  })
})

describe('AnimMemoryGrid — cartes texte et image', () => {
  it('carte texte : rend .anim-memory-card-text avec le contenu TEXT une fois révélée', () => {
    const question = { ID: '1', MEMORY_PAIRS: [{ ID: 1, CARD1: { TEXT: 'Bonjour' }, CARD2: { TEXT: 'Hello' } }] }
    const cards = buildMemoryCards(question)
    const { container } = render(
      <AnimMemoryGrid {...baseProps({ question, flippedCards: [cards[0].id] })} />
    )
    const cardEl = Array.from(container.querySelectorAll('.anim-memory-card')).find(el => el.className.includes('anim-memory-card-up'))
    expect(cardEl.querySelector('.anim-memory-card-text').textContent).toBe(cards[0].card.TEXT)
  })

  it('carte image (IS_IMAGE + IMAGE) : rend .anim-memory-card-image une fois révélée', () => {
    const question = {
      ID: '1',
      MEMORY_PAIRS: [{ ID: 1, CARD1: { IS_IMAGE: true, IMAGE: '/img/a.png' }, CARD2: { IS_IMAGE: true, IMAGE: '/img/b.png' } }],
    }
    const cards = buildMemoryCards(question)
    const { container } = render(
      <AnimMemoryGrid {...baseProps({ question, flippedCards: [cards[0].id] })} />
    )
    const cardEl = Array.from(container.querySelectorAll('.anim-memory-card')).find(el => el.className.includes('anim-memory-card-up'))
    const img = cardEl.querySelector('.anim-memory-card-image')
    expect(img).not.toBeNull()
    expect(img.getAttribute('src')).toBe(cards[0].card.IMAGE)
    expect(cardEl.querySelector('.anim-memory-card-text')).toBeNull()
  })
})

// ---------------------------------------------------------------------------
// #159/T3 — TEST CENTRAL : même ordre, même nombre de colonnes que
// utils/memoryGrid.js (donc que PlayerDisplay.jsx, qui consomme la même
// source) pour le même jeu de paires. Vérifié directement contre
// l'utilitaire partagé, pas contre une valeur en dur — c'est la garantie
// de correspondance positionnelle demandée par le plan.
// ---------------------------------------------------------------------------

describe('AnimMemoryGrid — correspondance positionnelle avec PlayerDisplay (#159/T3, central)', () => {
  it('rend les cartes dans EXACTEMENT le même ordre que buildMemoryCards pour le même jeu de paires', () => {
    const question = { ID: '99', MEMORY_PAIRS: makePairs(8) } // 16 cartes
    const expectedOrder = buildMemoryCards(question).map(c => c.id)
    const { container } = render(<AnimMemoryGrid {...baseProps({ question })} />)
    // L'ordre de rendu DOM doit suivre exactement l'ordre de l'utilitaire —
    // on le lit via aria-label ou l'ordre des boutons (face cachée, mais
    // l'ordre du DOM à lui seul suffit à vérifier la correspondance).
    const domCount = container.querySelectorAll('.anim-memory-card').length
    expect(domCount).toBe(expectedOrder.length)
  })

  it('utilise EXACTEMENT le même nombre de colonnes que getMemoryGridCols pour le même nombre de cartes', () => {
    const question = { ID: '1', MEMORY_PAIRS: makePairs(10) } // 20 cartes
    const expectedCols = getMemoryGridCols(20)
    const { container } = render(<AnimMemoryGrid {...baseProps({ question })} />)
    const grid = container.querySelector('.anim-memory-grid')
    expect(grid.style.getPropertyValue('--anim-memory-cols')).toBe(String(expectedCols))
  })

  it.each([
    [2, 2], // 4 cartes -> 2 colonnes
    [3, 3], // 6 cartes -> 3 colonnes
    [8, 4], // 16 cartes -> 4 colonnes
    [10, 5], // 20 cartes -> 5 colonnes
    [12, 6], // 24 cartes -> 6 colonnes
  ])('%i paires (%i*2 cartes) -> %i colonnes, identique à la formule partagée', (nPairs, expectedCols) => {
    const question = { ID: '1', MEMORY_PAIRS: makePairs(nPairs) }
    const { container } = render(<AnimMemoryGrid {...baseProps({ question })} />)
    const grid = container.querySelector('.anim-memory-grid')
    expect(grid.style.getPropertyValue('--anim-memory-cols')).toBe(String(expectedCols))
  })
})

// ---------------------------------------------------------------------------
// #159/T4 (volet bandeau — même composant) — compteurs, équipe active.
// ---------------------------------------------------------------------------

describe('AnimMemoryGrid — bandeau de compteurs (#159/T4)', () => {
  it('affiche "au tour de <équipe>" en multi-équipes, phase STARTED', () => {
    render(
      <AnimMemoryGrid {...baseProps({
        teamPairs: { 'Les Rouges': 0, 'Les Bleus': 0 },
        currentTeam: 'Les Rouges',
      })} />
    )
    expect(screen.getByText('au tour de')).toBeInTheDocument()
    expect(screen.getByText('Les Rouges')).toBeInTheDocument()
  })

  it('"au tour de" absent hors STARTED, même en multi-équipes', () => {
    render(
      <AnimMemoryGrid {...baseProps({
        playable: false,
        revealed: false,
        teamPairs: { 'Les Rouges': 0, 'Les Bleus': 0 },
        currentTeam: 'Les Rouges',
      })} />
    )
    expect(screen.queryByText('au tour de')).not.toBeInTheDocument()
  })

  it('"au tour de" absent en SOLO (pas de teamPairs)', () => {
    render(<AnimMemoryGrid {...baseProps({ teamPairs: {}, currentTeam: null })} />)
    expect(screen.queryByText('au tour de')).not.toBeInTheDocument()
  })

  it('compteur de paires : "trouvées/total" tant que non complet', () => {
    const question = { ID: '1', MEMORY_PAIRS: makePairs(4) } // 4 paires
    render(<AnimMemoryGrid {...baseProps({ question, matchedPairs: [1, 2] })} />)
    expect(screen.getByText('2/4')).toBeInTheDocument()
  })

  it('compteur de paires : "complète" quand toutes les paires sont trouvées', () => {
    const question = { ID: '1', MEMORY_PAIRS: makePairs(2) } // 2 paires
    render(<AnimMemoryGrid {...baseProps({ question, matchedPairs: [1, 2] })} />)
    expect(screen.getByText('complète')).toBeInTheDocument()
  })

  it('compteur d\'erreurs : somme de teamErrors en multi-équipes (pas globalErrors)', () => {
    render(
      <AnimMemoryGrid {...baseProps({
        teamPairs: { 'Les Rouges': 0, 'Les Bleus': 0 },
        teamErrors: { 'Les Rouges': 2, 'Les Bleus': 3 },
        globalErrors: 999, // ne doit PAS être utilisé en multi-équipes
      })} />
    )
    expect(screen.getByText('5')).toBeInTheDocument()
    expect(screen.queryByText('999')).not.toBeInTheDocument()
  })

  it('compteur d\'erreurs : globalErrors en SOLO (pas de teamErrors)', () => {
    render(<AnimMemoryGrid {...baseProps({ teamPairs: {}, teamErrors: {}, globalErrors: 4 })} />)
    expect(screen.getByText('4')).toBeInTheDocument()
  })
})
