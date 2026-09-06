import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import QuestionCard from './QuestionCard'
import { QUESTION_TYPES, getQuestionTypeMeta } from '../utils/questionTypeMeta'

// ---------------------------------------------------------------------------
// QuestionCard — bugfix cohérence UI (v8.0.0, #16/#107, retour utilisateur
// QUALIF 8.0.0.3, SHA a161bbab) :
//   1. Couleur du badge de type désormais lue depuis typeMeta.color
//      (utils/questionTypeMeta.js, source UNIQUE) au lieu d'une liste CSS
//      séparée (.qcard-type-badge.type-*, supprimée) qui n'avait aucune
//      entrée RAFALE — badge sans fond ("blanc sur bleu clair").
//   2. (bugfix suivant, 2026-08-29) RAFALE_CATEGORIES (multi) était RETIRÉ
//      côté backend — RAFALE réutilisait le champ CATEGORY simple, comme
//      tous les autres types (contrat rafale.md §3.3 tel qu'il était alors).
//
// ⚠️ [CHANGED] #216 (milestone v9.0.0, 2026-09-04) — réouverture ASSUMÉE du
// point 2 ci-dessus (décision utilisateur explicite, voir
// contracts/rafale.md §3.3 et contracts/CHANGELOG.md [20260904]) :
// RAFALE_CATEGORIES et le nouveau RAFALE_DIFFICULTIES redeviennent multi,
// affichés en **chips multiples** (pas de retour au bug de #107 : la card
// reste générique, elle affiche simplement N chips au lieu d'une valeur
// scalaire quand le type le fournit — pas de branche `isRafale` dédiée).
// RAFALE est donc retiré des `it.each` génériques "une seule catégorie" ci-
// dessous — son comportement (désormais différent) a sa propre section plus
// bas, pas supprimé, déplacé et enrichi.
//
// Composant SANS AUCUN test au moment où ce fichier a été écrit — première
// couverture, périmètre : le point 1 du bugfix + non-régression sur les
// types à catégorie simple + (depuis #216) RAFALE à chips multiples.
// ---------------------------------------------------------------------------

function baseQuestion(overrides = {}) {
  return {
    ID: 'q1',
    QUESTION: 'Un énoncé de test',
    ANSWER: 'Une réponse',
    TYPE: 'SPEEDY',
    TIME: 30,
    POINTS: 10,
    POINTS_TARGET: 'PLAYER',
    ...overrides,
  }
}

// ---------------------------------------------------------------------------
// Badge de type — couleur unique (typeMeta.color), tous types confondus
// ---------------------------------------------------------------------------

describe('QuestionCard — badge de type : couleur unique (typeMeta.color, bugfix a161bbab)', () => {
  it.each(QUESTION_TYPES)('$key : le badge porte le libellé "$label" et la couleur de fond $color (questionTypeMeta.js)', ({ key, label, color }) => {
    const { container } = render(<QuestionCard question={baseQuestion({ TYPE: key })} />)
    const badge = container.querySelector('.qcard-type-badge')
    expect(badge).not.toBeNull()
    expect(badge.textContent).toBe(label)
    expect(badge.style.backgroundColor).toBeTruthy()
    // rgb(...) calculé par jsdom depuis le hex — comparaison via le style
    // recalculé de la MÊME couleur plutôt qu'une chaîne hex en dur, robuste
    // au format de sérialisation.
    const probe = document.createElement('div')
    probe.style.backgroundColor = color
    expect(badge.style.backgroundColor).toBe(probe.style.backgroundColor)
  })

  it('RAFALE spécifiquement : le badge a un fond NON VIDE (LE bug rapporté — "blanc sur bleu clair", texte sans fond)', () => {
    const { container } = render(<QuestionCard question={baseQuestion({ TYPE: 'RAFALE' })} />)
    const badge = container.querySelector('.qcard-type-badge')
    expect(badge.style.backgroundColor).not.toBe('')
  })

  it('aucune classe .type-* résiduelle sur le badge (liste CSS divergente supprimée, une seule source désormais)', () => {
    const { container } = render(<QuestionCard question={baseQuestion({ TYPE: 'RAFALE' })} />)
    const badge = container.querySelector('.qcard-type-badge')
    expect(badge.className.trim()).toBe('qcard-type-badge')
  })

  it('TYPE absent : repli sur SPEEDY (contrat questionTypeMeta.js — comportement documenté, pas une régression)', () => {
    const { container } = render(<QuestionCard question={baseQuestion({ TYPE: undefined })} />)
    const badge = container.querySelector('.qcard-type-badge')
    expect(badge.textContent).toBe('Speedy')
  })

  it('TYPE inconnu (absent du registre) : badge "Type inconnu" avec sa propre couleur — pas un repli silencieux sur Speedy', () => {
    const { container } = render(<QuestionCard question={baseQuestion({ TYPE: 'TOTALEMENT_INCONNU' })} />)
    const badge = container.querySelector('.qcard-type-badge')
    expect(badge.textContent).toBe('Type inconnu')
    const unknownMeta = getQuestionTypeMeta('TOTALEMENT_INCONNU')
    const probe = document.createElement('div')
    probe.style.backgroundColor = unknownMeta.color
    expect(badge.style.backgroundColor).toBe(probe.style.backgroundColor)
  })
})

// ---------------------------------------------------------------------------
// Catégorie simple — types à catégorie scalaire (RAFALE exclu depuis #216,
// voir la section dédiée "chips multiples" plus bas : son comportement a
// changé, ce n'est plus une simple catégorie unique).
// ---------------------------------------------------------------------------

describe('QuestionCard — catégorie simple pour les types à catégorie scalaire', () => {
  it.each(['SPEEDY', 'QCM', 'MEMORY', 'MEMOTION', 'ARDOISE'])('%s avec CATEGORY défini : un seul CategoryBadge simple', (type) => {
    const { container } = render(<QuestionCard question={baseQuestion({ TYPE: type, CATEGORY: 'SCIENCE' })} />)
    expect(container.querySelector('.qcard-rafale-categories')).toBeNull()
    expect(screen.getByTitle('Sciences & Nature')).toBeInTheDocument()
    expect(container.querySelectorAll('.category-badge').length).toBe(1)
  })

  it.each(['SPEEDY', 'QCM', 'MEMORY', 'MEMOTION', 'ARDOISE'])('%s sans CATEGORY : aucun badge catégorie rendu', (type) => {
    const { container } = render(<QuestionCard question={baseQuestion({ TYPE: type, CATEGORY: undefined })} />)
    expect(container.querySelectorAll('.category-badge').length).toBe(0)
  })
})

// ---------------------------------------------------------------------------
// RAFALE — chips multiples catégories/difficultés (#216, milestone v9.0.0,
// réouverture ASSUMÉE du bugfix 2026-08-29 — voir contracts/rafale.md §3.3
// et le commentaire de fichier en tête). Remplace l'ancien test "RAFALE_CATEGORIES
// résiduel jamais lu", qui affirmait exactement le comportement inverse.
//
// Rétro-compatibilité (216-Q6, models.go EffectiveRafaleCategories/
// EffectiveRafaleDifficulties) : une question RAFALE créée avant #216 ne
// porte PAS RAFALE_CATEGORIES/RAFALE_DIFFICULTIES sur le fil JSON (champs
// omitempty, jamais migrés sur disque) — seuls CATEGORY et RAFALE_DIFFICULTY
// (mono) sont présents. La card doit donc répliquer côté client le même
// repli mono → liste-à-un-élément que l'API EffectiveRafale* côté serveur,
// sous peine d'afficher une card vide de chips pour une question ancienne
// pourtant valide.
// ---------------------------------------------------------------------------

describe('QuestionCard — RAFALE : chips multiples catégories/difficultés (#216)', () => {
  it('RAFALE_CATEGORIES à plusieurs entrées : un chip par catégorie, plus un seul CategoryBadge simple', () => {
    const { container } = render(<QuestionCard question={baseQuestion({
      TYPE: 'RAFALE', RAFALE_CATEGORIES: ['HISTORY', 'SCIENCE'], RAFALE_DIFFICULTIES: [1],
    })} />)
    expect(screen.getByTitle('Histoire')).toBeInTheDocument()
    expect(screen.getByTitle('Sciences & Nature')).toBeInTheDocument()
    expect(container.querySelectorAll('.category-badge').length).toBe(2)
  })

  it('RAFALE_DIFFICULTIES à plusieurs entrées : un chip par difficulté', () => {
    const { container } = render(<QuestionCard question={baseQuestion({
      TYPE: 'RAFALE', RAFALE_CATEGORIES: ['HISTORY'], RAFALE_DIFFICULTIES: [1, 2, 3],
    })} />)
    const difficultyChips = container.querySelectorAll('.qcard-rafale-diff-chip')
    expect(difficultyChips.length).toBe(3)
  })

  it('rétro-compatibilité — RAFALE_CATEGORIES absent, CATEGORY (mono) présent : repli sur un seul chip', () => {
    const { container } = render(<QuestionCard question={baseQuestion({
      TYPE: 'RAFALE', CATEGORY: 'HISTORY', RAFALE_CATEGORIES: undefined,
    })} />)
    expect(screen.getByTitle('Histoire')).toBeInTheDocument()
    expect(container.querySelectorAll('.category-badge').length).toBe(1)
  })

  it('rétro-compatibilité — RAFALE_DIFFICULTIES absent, RAFALE_DIFFICULTY (mono) présent : repli sur un seul chip', () => {
    const { container } = render(<QuestionCard question={baseQuestion({
      TYPE: 'RAFALE', RAFALE_CATEGORIES: ['HISTORY'], RAFALE_DIFFICULTIES: undefined, RAFALE_DIFFICULTY: 2,
    })} />)
    // Le repli doit rendre la difficulté mono (2) — même convention étoilée
    // que le panneau de configuration de manche (GamePage.jsx, '★'.repeat(n)).
    expect(container.textContent).toContain('★★')
  })

  it('ni CATEGORY ni RAFALE_CATEGORIES : aucun badge catégorie rendu (même règle que les autres types)', () => {
    const { container } = render(<QuestionCard question={baseQuestion({
      TYPE: 'RAFALE', CATEGORY: undefined, RAFALE_CATEGORIES: undefined,
    })} />)
    expect(container.querySelectorAll('.category-badge').length).toBe(0)
  })
})
