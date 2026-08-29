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
//   2. RAFALE affiche un CategoryBadge PAR entrée de RAFALE_CATEGORIES
//      (multi, contrat rafale.md §3.3) au lieu du badge CATEGORY simple
//      (toujours vide pour ce type — configuration de manche, pas
//      d'énoncé propre).
//
// Composant SANS AUCUN test au moment où ce fichier est écrit — première
// couverture, périmètre : les 2 points du bugfix + non-régression sur les
// 5 autres types (SPEEDY/QCM/MEMORY/MEMOTION/ARDOISE), catégorie simple
// toujours affichée à l'identique après le retrait de la liste CSS.
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
// RAFALE — catégories multiples (RAFALE_CATEGORIES, bugfix a161bbab)
// ---------------------------------------------------------------------------

describe('QuestionCard — RAFALE : catégories multiples (RAFALE_CATEGORIES, bugfix a161bbab)', () => {
  it('RAFALE_CATEGORIES absent/vide : aucun badge catégorie rendu (pas de conteneur vide non plus)', () => {
    const { container } = render(<QuestionCard question={baseQuestion({ TYPE: 'RAFALE', RAFALE_CATEGORIES: [] })} />)
    expect(container.querySelector('.qcard-rafale-categories')).toBeNull()
  })

  it('RAFALE_CATEGORIES avec 1 entrée : exactement 1 CategoryBadge affiché', () => {
    const { container } = render(<QuestionCard question={baseQuestion({ TYPE: 'RAFALE', RAFALE_CATEGORIES: ['HISTORY'] })} />)
    const wrap = container.querySelector('.qcard-rafale-categories')
    expect(wrap).not.toBeNull()
    expect(screen.getByTitle('Histoire')).toBeInTheDocument()
    expect(wrap.querySelectorAll('.category-badge').length).toBe(1)
  })

  it('RAFALE_CATEGORIES avec plusieurs entrées : un badge PAR catégorie, dans l\'ordre', () => {
    const { container } = render(<QuestionCard question={baseQuestion({ TYPE: 'RAFALE', RAFALE_CATEGORIES: ['HISTORY', 'SCIENCE', 'SPORTS'] })} />)
    const wrap = container.querySelector('.qcard-rafale-categories')
    expect(wrap.querySelectorAll('.category-badge').length).toBe(3)
    expect(screen.getByTitle('Histoire')).toBeInTheDocument()
    expect(screen.getByTitle('Sciences & Nature')).toBeInTheDocument()
    expect(screen.getByTitle('Sports & Loisirs')).toBeInTheDocument()
  })

  it('RAFALE avec CATEGORY (simple, non pertinent pour ce type) EN PLUS de RAFALE_CATEGORIES : CATEGORY est ignoré, seul RAFALE_CATEGORIES pilote l\'affichage', () => {
    const { container } = render(<QuestionCard question={baseQuestion({
      TYPE: 'RAFALE', CATEGORY: 'ANIMALS', RAFALE_CATEGORIES: ['HISTORY'],
    })} />)
    // Un seul badge (HISTORY, via RAFALE_CATEGORIES) — jamais un 2e pour
    // CATEGORY=ANIMALS, qui n'a aucun sens pour ce type (contrat §3.3).
    expect(container.querySelectorAll('.category-badge').length).toBe(1)
    expect(screen.getByTitle('Histoire')).toBeInTheDocument()
    expect(screen.queryByTitle('Animaux')).not.toBeInTheDocument()
  })
})

// ---------------------------------------------------------------------------
// Non-régression — catégorie simple (autres types), inchangée après le
// retrait de la liste CSS de couleur.
// ---------------------------------------------------------------------------

describe('QuestionCard — non-régression : catégorie simple pour les autres types', () => {
  it.each(['SPEEDY', 'QCM', 'MEMORY', 'MEMOTION', 'ARDOISE'])('%s avec CATEGORY défini : un seul CategoryBadge simple (pas de conteneur multi RAFALE)', (type) => {
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
