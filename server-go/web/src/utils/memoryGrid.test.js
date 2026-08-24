import { describe, it, expect } from 'vitest'
import { buildMemoryCards, getMemoryGridCols, getMemoryGridRows } from './memoryGrid'

// ---------------------------------------------------------------------------
// memoryGrid — #159/F0, T0. Extraction PURE de PlayerDisplay.jsx:709-757
// (mélange Fisher-Yates ensemencé par question.ID + formule de colonnes),
// écrite en TDD directement contre l'algorithme lu dans le code existant —
// pas une supposition. Consommée par PlayerDisplay.jsx ET AnimMemoryGrid.jsx
// (#159/F1) : **seule** source de vérité pour l'ordre et la disposition,
// pour que /anim, /tv, la vue joueur et l'aperçu régie désignent
// littéralement la même carte à la même position (motif du plan — la
// correspondance positionnelle, huitième mutualisation de la série).
//
// ⚠️ Piège nommé au plan (R9) : une règle de disposition DUPLIQUÉE plutôt
// qu'extraite ne produirait ni erreur ni test rouge — seulement un
// animateur et un joueur qui ne parlent plus de la même carte. C'est
// pourquoi T0 fixe le contrat exact de cet utilitaire AVANT que
// PlayerDisplay.jsx et AnimMemoryGrid.jsx le consomment tous les deux.
// ---------------------------------------------------------------------------

function makePairs(n) {
  return Array.from({ length: n }, (_, i) => ({
    ID: i + 1,
    CARD1: { TEXT: `Q${i + 1}` },
    CARD2: { TEXT: `R${i + 1}` },
  }))
}

// buildMemoryCards prend la QUESTION entière (comme gameState.question),
// pas des paires/ID séparés — extraction verbatim de PlayerDisplay.jsx, qui
// lisait déjà `gameState.question?.MEMORY_PAIRS`/`?.ID` de cette façon.
function makeQuestion(n, id) {
  return { ID: id, MEMORY_PAIRS: makePairs(n) }
}

describe('buildMemoryCards — ordre déterministe (mélange ensemencé par l\'ID de question)', () => {
  it('deux appels avec la MÊME question produisent EXACTEMENT le même ordre', () => {
    const question = makeQuestion(6, '42') // 12 cartes
    const first = buildMemoryCards(question)
    const second = buildMemoryCards(question)
    expect(second.map(c => c.id)).toEqual(first.map(c => c.id))
  })

  it('deux identifiants de question DIFFÉRENTS produisent (presque toujours) un ordre différent', () => {
    const pairs = makePairs(8) // 16 cartes, assez pour que la probabilité de collision soit négligeable
    const a = buildMemoryCards({ ID: '1', MEMORY_PAIRS: pairs })
    const b = buildMemoryCards({ ID: '2', MEMORY_PAIRS: pairs })
    expect(a.map(c => c.id)).not.toEqual(b.map(c => c.id))
  })

  it('produit deux entrées par paire (id `${pairID}-1` et `${pairID}-2`), toutes présentes', () => {
    const cards = buildMemoryCards(makeQuestion(3, '7'))
    expect(cards).toHaveLength(6)
    const ids = cards.map(c => c.id).sort()
    expect(ids).toEqual(['1-1', '1-2', '2-1', '2-2', '3-1', '3-2'])
  })

  it('chaque carte porte pairId, card et cardIndex corrects (format "pairID-cardNum", contrat Go)', () => {
    const question = { ID: '1', MEMORY_PAIRS: [{ ID: 5, CARD1: { TEXT: 'Question' }, CARD2: { TEXT: 'Réponse' } }] }
    const cards = buildMemoryCards(question)
    const card1 = cards.find(c => c.id === '5-1')
    const card2 = cards.find(c => c.id === '5-2')
    expect(card1).toMatchObject({ pairId: 5, cardIndex: 1, card: { TEXT: 'Question' } })
    expect(card2).toMatchObject({ pairId: 5, cardIndex: 2, card: { TEXT: 'Réponse' } })
  })

  it('MEMORY_PAIRS vide/absente -> liste de cartes vide', () => {
    expect(buildMemoryCards({ ID: '1', MEMORY_PAIRS: [] })).toEqual([])
    expect(buildMemoryCards({ ID: '1' })).toEqual([])
  })

  it('question absente (null/undefined) ne plante pas -> liste vide', () => {
    expect(() => buildMemoryCards(null)).not.toThrow()
    expect(() => buildMemoryCards(undefined)).not.toThrow()
    expect(buildMemoryCards(null)).toEqual([])
  })

  it('question.ID absent ne plante pas (repli sur une graine par défaut)', () => {
    const question = { MEMORY_PAIRS: makePairs(2) }
    expect(() => buildMemoryCards(question)).not.toThrow()
    expect(buildMemoryCards(question)).toHaveLength(4)
  })

  // #187 — régression : avant la correction, `parseInt(id, 10) || 1` sur un
  // MotionCard.ID non numérique ("mc-...", "card_3") retombait sur la graine
  // 1 pour TOUTES les cartes -> mélange identique partout dans une manche
  // MEMOTION. `hashStringToSeed` doit dériver une graine par ID de carte,
  // numérique ou non.
  it('#187 — deux cartes MEMOTION MEMORY (ID non numérique) ont des mélanges distincts', () => {
    const pairs = makePairs(8) // 16 cartes, collision négligeable
    const cardA = buildMemoryCards({ ID: 'mc-1699999999999', MEMORY_PAIRS: pairs })
    const cardB = buildMemoryCards({ ID: 'card_3', MEMORY_PAIRS: pairs })
    expect(cardA.map(c => c.id)).not.toEqual(cardB.map(c => c.id))
  })

  it('#187 — un ID non numérique ne retombe plus silencieusement sur la graine 1 (repli parseInt)', () => {
    const pairs = makePairs(8)
    // Avant #187 : parseInt("mc-abc", 10) -> NaN -> repli `|| 1`, IDENTIQUE
    // au mélange d'une question numérique d'ID "1" (parseInt("1") === 1).
    // Après #187 : les deux graines sont dérivées par hachage de chaîne
    // complète -> plus aucune raison de coïncider.
    const numeric = buildMemoryCards({ ID: '1', MEMORY_PAIRS: pairs })
    const nonNumeric = buildMemoryCards({ ID: 'mc-abc', MEMORY_PAIRS: pairs })
    expect(nonNumeric.map(c => c.id)).not.toEqual(numeric.map(c => c.id))
  })

  it('#187 — même ID non numérique -> même mélange (déterminisme préservé)', () => {
    const question = { ID: 'card_7', MEMORY_PAIRS: makePairs(4) }
    const first = buildMemoryCards(question)
    const second = buildMemoryCards(question)
    expect(second.map(c => c.id)).toEqual(first.map(c => c.id))
  })
})

describe('getMemoryGridCols — formule fixe sur le nombre de cartes, aux bornes exactes', () => {
  it.each([
    [1, 2], [2, 2], [3, 2], [4, 2],   // ≤4 -> 2
    [5, 3], [6, 3],                  // ≤6 -> 3
    [7, 4], [12, 4], [16, 4],        // ≤16 -> 4
    [17, 5], [20, 5],                // ≤20 -> 5
    [21, 6], [24, 6], [30, 6],       // >20 -> 6
  ])('%i cartes -> %i colonnes', (cardCount, expectedCols) => {
    expect(getMemoryGridCols(cardCount)).toBe(expectedCols)
  })

  it('0 carte -> ne plante pas (2 colonnes par le même repli que ≤4)', () => {
    expect(getMemoryGridCols(0)).toBe(2)
  })
})

describe('getMemoryGridRows — rangées déduites (ceil)', () => {
  it.each([
    [4, 2, 2],   // 4 cartes, 2 colonnes -> 2 rangées pile
    [6, 3, 2],   // 6 cartes, 3 colonnes -> 2 rangées pile
    [16, 4, 4],  // 16 cartes, 4 colonnes -> 4 rangées pile
    [20, 5, 4],  // 20 cartes, 5 colonnes -> 4 rangées pile
    [24, 6, 4],  // 24 cartes, 6 colonnes -> 4 rangées pile
    [5, 3, 2],   // 5 cartes, 3 colonnes -> ceil(5/3) = 2
    [7, 4, 2],   // 7 cartes, 4 colonnes -> ceil(7/4) = 2
  ])('%i cartes, %i colonnes -> %i rangées', (cardCount, cols, expectedRows) => {
    expect(getMemoryGridRows(cardCount, cols)).toBe(expectedRows)
  })

  it('cohérence bout-en-bout : getMemoryGridRows(n, getMemoryGridCols(n)) pour chaque borne', () => {
    ;[4, 6, 16, 20, 24].forEach((cardCount) => {
      const cols = getMemoryGridCols(cardCount)
      const rows = getMemoryGridRows(cardCount, cols)
      expect(rows).toBe(Math.ceil(cardCount / cols))
    })
  })
})
