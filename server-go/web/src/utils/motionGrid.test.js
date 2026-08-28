import { describe, it, expect } from 'vitest'
import {
  getMotionGridCols,
  getMotionGridRows,
  getMotionCardCoord,
  getMotionCardPoints,
  isMotionSecretMode,
  computeStarsProrataPoints,
} from './motionGrid'

// ---------------------------------------------------------------------------
// motionGrid — #160/F0, T1. Extraction PURE de PlayerDisplay.jsx:1985-2001
// (formule de colonnes, coordonnées A1/B2/..., barème de points par
// difficulté, détection du mode Secret) et GamePage.jsx:814-821 (copie
// IDENTIQUE du barème de points côté régie) — écrite en TDD directement
// contre l'algorithme lu dans le code existant (plan §F0), pas une
// supposition. Consommée par PlayerDisplay.jsx (#160/F1, refactor pur) ET
// AnimMotionGrid.jsx (#160/F4) : seule source de vérité pour la disposition
// et les points, pour que `/anim` et `/tv` désignent la même carte à la
// même coordonnée et annoncent le même barème (risque R1 du plan, leçon R9
// de #159).
//
// ⚠️ TEST PIVOT (exigé par le plan, §"Tests Requis" T1) : la formule de
// colonnes MEMOTION est DIFFÉRENTE de celle de MEMORY (`memoryGrid.js` :
// 2/3/4/5/6, palier à 16). Ne jamais aligner ces bornes sur memoryGrid.test.js
// par réflexe de copier-coller — elles sont volontairement distinctes
// (source : PlayerDisplay.jsx:2000, un seul palier à 12, plafond à 5
// colonnes qui ne bouge plus au-delà).
// ---------------------------------------------------------------------------

describe('getMotionGridCols — formule fixe MEMOTION, DIFFÉRENTE de memoryGrid.js (test pivot)', () => {
  it.each([
    [1, 2], [2, 2], [3, 2], [4, 2],   // ≤4 -> 2
    [5, 3], [6, 3],                  // ≤6 -> 3
    [7, 4], [10, 4], [12, 4],        // ≤12 -> 4 (palier différent de MEMORY, qui va jusqu'à 16)
    [13, 5], [16, 5], [20, 5],       // >12 -> 5
    [24, 5], [30, 5],                // >20 : PLAFOND à 5, contrairement à MEMORY (palier à 6 au-delà de 20)
  ])('%i cartes -> %i colonnes', (count, expectedCols) => {
    expect(getMotionGridCols(count)).toBe(expectedCols)
  })

  it('0 carte -> ne plante pas (repli sur 2, même borne que ≤4)', () => {
    expect(getMotionGridCols(0)).toBe(2)
  })
})

describe('getMotionGridRows — rangées déduites (ceil), même mécanique que memoryGrid', () => {
  it.each([
    [4, 2, 2],   // 4 cartes, 2 colonnes -> 2 rangées pile
    [6, 3, 2],   // 6 cartes, 3 colonnes -> 2 rangées pile
    [12, 4, 3],  // 12 cartes, 4 colonnes -> 3 rangées pile
    [20, 5, 4],  // 20 cartes, 5 colonnes -> 4 rangées pile
    [5, 3, 2],   // 5 cartes, 3 colonnes -> ceil(5/3) = 2
    [7, 4, 2],   // 7 cartes, 4 colonnes -> ceil(7/4) = 2
  ])('%i cartes, %i colonnes -> %i rangées', (count, cols, expectedRows) => {
    expect(getMotionGridRows(count, cols)).toBe(expectedRows)
  })

  it('cohérence bout-en-bout : getMotionGridRows(n, getMotionGridCols(n)) pour chaque borne', () => {
    ;[4, 6, 12, 20, 24].forEach((count) => {
      const cols = getMotionGridCols(count)
      const rows = getMotionGridRows(count, cols)
      expect(rows).toBe(Math.ceil(count / cols))
    })
  })
})

// ---------------------------------------------------------------------------
// getMotionCardCoord — coordonnées "A1"/"B2"/... — extraction verbatim de
// PlayerDisplay.jsx:1996 :
//   `${String.fromCharCode(65 + Math.floor(idx / cols))}${(idx % cols) + 1}`
// Mode Secret : c'est LA seule information transmise au joueur pour
// désigner une carte — toute divergence avec `/tv` rend une coordonnée
// annoncée incompréhensible (risque R1).
// ---------------------------------------------------------------------------

describe('getMotionCardCoord — coordonnées aux quatre coins d\'une grille 4 colonnes × 3 rangées (12 cartes)', () => {
  const cols = 4
  it.each([
    [0, 'A1'],   // coin haut-gauche
    [3, 'A4'],   // coin haut-droit
    [8, 'C1'],   // coin bas-gauche (idx 8 = rangée 2 (0-based) -> lettre C)
    [11, 'C4'],  // coin bas-droit
    [4, 'B1'],   // début de la 2e rangée
    [7, 'B4'],   // fin de la 2e rangée
  ])('index %i (cols=4) -> coordonnée %s', (idx, expected) => {
    expect(getMotionCardCoord(idx, cols)).toBe(expected)
  })

  it('grille à 2 colonnes : lettres qui progressent une par rangée', () => {
    expect(getMotionCardCoord(0, 2)).toBe('A1')
    expect(getMotionCardCoord(1, 2)).toBe('A2')
    expect(getMotionCardCoord(2, 2)).toBe('B1')
    expect(getMotionCardCoord(3, 2)).toBe('B2')
  })
})

// ---------------------------------------------------------------------------
// getMotionCardPoints — barème par étoile (DIFFICULTY 1/2/3), avec
// MOTION_CONFIG (POINTS_1_STAR/POINTS_2_STAR/POINTS_3_STAR) et repli 1/3/5
// sans config — DEUX copies identiques dans le code actuel
// (PlayerDisplay.jsx:1985-1992 ET GamePage.jsx:814-821) : cette extraction
// doit matcher les DEUX à l'identique (sinon la régie et l'animateur
// annoncent des points différents pour la même carte).
// ---------------------------------------------------------------------------

describe('getMotionCardPoints — barème avec MOTION_CONFIG', () => {
  it.each([
    [1, { POINTS_1_STAR: 2, POINTS_2_STAR: 6, POINTS_3_STAR: 10 }, 2],
    [2, { POINTS_1_STAR: 2, POINTS_2_STAR: 6, POINTS_3_STAR: 10 }, 6],
    [3, { POINTS_1_STAR: 2, POINTS_2_STAR: 6, POINTS_3_STAR: 10 }, 10],
  ])('difficulté %i avec config personnalisée -> %i pts', (difficulty, motionConfig, expected) => {
    expect(getMotionCardPoints(difficulty, motionConfig)).toBe(expected)
  })

  it('config PARTIELLE (un seul champ renseigné) : repli 1/3/5 SEULEMENT pour les champs absents', () => {
    expect(getMotionCardPoints(1, { POINTS_1_STAR: 7 })).toBe(7)
    expect(getMotionCardPoints(2, { POINTS_1_STAR: 7 })).toBe(3) // POINTS_2_STAR absent -> repli 3
    expect(getMotionCardPoints(3, { POINTS_1_STAR: 7 })).toBe(5) // POINTS_3_STAR absent -> repli 5
  })

  // Nullish coalescing (??), PAS ||  — un barème à 0 point est une valeur
  // valide (ex. carte "gratuite"), distincte de "absent". `||` la
  // confondrait avec `undefined` et retomberait à tort sur le repli 1/3/5.
  it('POINTS_n_STAR à 0 est une valeur VALIDE (0 pt), pas confondue avec "absent" (?? vs ||)', () => {
    expect(getMotionCardPoints(1, { POINTS_1_STAR: 0 })).toBe(0)
    expect(getMotionCardPoints(2, { POINTS_2_STAR: 0 })).toBe(0)
    expect(getMotionCardPoints(3, { POINTS_3_STAR: 0 })).toBe(0)
  })

  it('difficulté hors 1/2/3 (0, undefined) avec config présente : repli générique 1', () => {
    expect(getMotionCardPoints(0, { POINTS_1_STAR: 2, POINTS_2_STAR: 6, POINTS_3_STAR: 10 })).toBe(1)
    expect(getMotionCardPoints(undefined, { POINTS_1_STAR: 2 })).toBe(1)
  })
})

describe('getMotionCardPoints — repli SANS MOTION_CONFIG (question sans barème personnalisé)', () => {
  it.each([
    [1, undefined, 1],
    [2, undefined, 3],
    [3, undefined, 5],
    [1, null, 1],
    [2, null, 3],
    [3, null, 5],
  ])('difficulté %i, config %s -> repli %i pts', (difficulty, motionConfig, expected) => {
    expect(getMotionCardPoints(difficulty, motionConfig)).toBe(expected)
  })

  it('difficulté hors 1/2/3 sans config : repli générique 1', () => {
    expect(getMotionCardPoints(0, undefined)).toBe(1)
    expect(getMotionCardPoints(undefined, undefined)).toBe(1)
  })
})

// ---------------------------------------------------------------------------
// isMotionSecretMode — (question?.MOTION_MEMORIZE_DURATION || 0) > 0,
// extraction verbatim de PlayerDisplay.jsx:1995. Contrôle si la grille
// affiche les thèmes (mode normal) ou les coordonnées seules (mode Secret).
// ---------------------------------------------------------------------------

describe('isMotionSecretMode — détection du mode Secret', () => {
  it('MOTION_MEMORIZE_DURATION > 0 -> mode Secret actif', () => {
    expect(isMotionSecretMode({ MOTION_MEMORIZE_DURATION: 15 })).toBe(true)
  })

  it('MOTION_MEMORIZE_DURATION === 0 -> mode normal', () => {
    expect(isMotionSecretMode({ MOTION_MEMORIZE_DURATION: 0 })).toBe(false)
  })

  it('MOTION_MEMORIZE_DURATION absent -> mode normal (repli sur 0)', () => {
    expect(isMotionSecretMode({})).toBe(false)
  })

  it('question null/undefined -> ne plante pas, mode normal', () => {
    expect(() => isMotionSecretMode(null)).not.toThrow()
    expect(isMotionSecretMode(null)).toBe(false)
    expect(isMotionSecretMode(undefined)).toBe(false)
  })

  it('MOTION_MEMORIZE_DURATION négatif (donnée corrompue) -> traité comme mode normal (pas > 0)', () => {
    expect(isMotionSecretMode({ MOTION_MEMORIZE_DURATION: -5 })).toBe(false)
  })
})

// ---------------------------------------------------------------------------
// computeStarsProrataPoints — gain prévisionnel d'une carte MEMORY en barème
// STARS_PRORATA (#187, v7.1.0, contrat §6.2). Cas de test nommé À L'IDENTIQUE
// côté Go (coordination dev-backend/dev-frontend, #187) — dev-backend a
// retenu "5 points / 8 pairs" comme fragment canonique (voir
// internal/game/engine_points_rule_184_test.go,
// TestMotionCardPointsForOutcome_StarsProrata) : repris ici mot pour mot
// plutôt que le nom initialement proposé côté frontend
// ("STARS_PRORATA_5points_8pairs"), pour que la correspondance soit
// effective et pas seulement projetée. Attrape à lui seul l'erreur d'ordre
// des opérations (diviser avant de multiplier -> 0 point sur une carte à
// 5 pts/8 paires, quel que soit le nombre de paires trouvées).
// ---------------------------------------------------------------------------

describe('computeStarsProrataPoints — barème STARS_PRORATA (contrat §6.2)', () => {
  // Cas nommé À L'IDENTIQUE côté Go (fragment "5 points / 8 pairs") — NE PAS
  // renommer sans coordonner dev-backend.
  it('STARS_PRORATA — 5 points / 8 pairs', () => {
    // Carte à 5 points (barème étoiles), 8 paires au total.
    // Multiplier AVANT de diviser (contrat §6.2, normatif) :
    //   4/8 trouvées -> floor(5*4/8)  = floor(2.5) = 2
    //   8/8 trouvées -> floor(5*8/8)  = floor(5)   = 5 (valeur nominale exacte)
    // Diviser avant de multiplier donnerait floor(5/8)=0 -> 0 pt dans les DEUX cas.
    expect(computeStarsProrataPoints(5, 4, 8)).toBe(2)
    expect(computeStarsProrataPoints(5, 8, 8)).toBe(5)
  })

  it('grille complète -> toujours la valeur nominale exacte de la carte (aucune perte d\'arrondi cumulée)', () => {
    expect(computeStarsProrataPoints(10, 6, 6)).toBe(10)
    expect(computeStarsProrataPoints(3, 12, 12)).toBe(3)
  })

  it('unitsTotal <= 0 -> 0 point (garde division par zéro, contrat §6.2)', () => {
    expect(computeStarsProrataPoints(5, 0, 0)).toBe(0)
    expect(computeStarsProrataPoints(5, 3, -1)).toBe(0)
  })

  it('units = 0 -> 0 point, quel que soit unitsTotal', () => {
    expect(computeStarsProrataPoints(5, 0, 8)).toBe(0)
  })

  it('non-uniformité assumée par l\'arrondi entier : la dernière unité peut rapporter plus que les précédentes', () => {
    // 5 pts / 8 paires : la 4e paire ne rapporte rien (2 -> 2), la 5e fait
    // franchir un palier (2 -> 3) — documenté par le contrat comme
    // inhérent à l'arrondi, pas un bug à corriger.
    expect(computeStarsProrataPoints(5, 3, 8)).toBe(1)
    expect(computeStarsProrataPoints(5, 4, 8)).toBe(2)
    expect(computeStarsProrataPoints(5, 5, 8)).toBe(3)
  })
})
