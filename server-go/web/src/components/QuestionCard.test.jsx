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
//   2. (bugfix suivant, 2026-08-29) RAFALE_CATEGORIES (multi) est RETIRÉ
//      côté backend — RAFALE réutilise désormais le champ CATEGORY simple,
//      exactement comme tous les autres types (contrat rafale.md §3.3). La
//      branche `isRafale` dédiée qui affichait un CategoryBadge par entrée
//      de RAFALE_CATEGORIES est supprimée : le chemin générique suffit,
//      testé ci-dessous en même temps que les 5 autres types.
//
// Composant SANS AUCUN test au moment où ce fichier est écrit — première
// couverture, périmètre : le point 1 du bugfix + non-régression sur les
// 6 types (SPEEDY/QCM/MEMORY/MEMOTION/ARDOISE/RAFALE), catégorie simple
// affichée à l'identique pour tous.
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
// Catégorie simple — TOUS les types, RAFALE compris depuis le bugfix
// 2026-08-29 (contrat rafale.md §3.3 : CATEGORY unique, RAFALE_CATEGORIES
// multi retiré). Un seul chemin générique, plus de branche isRafale dédiée.
// ---------------------------------------------------------------------------

describe('QuestionCard — catégorie simple pour tous les types (RAFALE inclus depuis le bugfix CATEGORY unique)', () => {
  it.each(['SPEEDY', 'QCM', 'MEMORY', 'MEMOTION', 'ARDOISE', 'RAFALE'])('%s avec CATEGORY défini : un seul CategoryBadge simple', (type) => {
    const { container } = render(<QuestionCard question={baseQuestion({ TYPE: type, CATEGORY: 'SCIENCE' })} />)
    expect(container.querySelector('.qcard-rafale-categories')).toBeNull()
    expect(screen.getByTitle('Sciences & Nature')).toBeInTheDocument()
    expect(container.querySelectorAll('.category-badge').length).toBe(1)
  })

  it.each(['SPEEDY', 'QCM', 'MEMORY', 'MEMOTION', 'ARDOISE', 'RAFALE'])('%s sans CATEGORY : aucun badge catégorie rendu', (type) => {
    const { container } = render(<QuestionCard question={baseQuestion({ TYPE: type, CATEGORY: undefined })} />)
    expect(container.querySelectorAll('.category-badge').length).toBe(0)
  })

  it('RAFALE : un éventuel RAFALE_CATEGORIES résiduel (ancien champ, retiré du contrat) n\'est jamais lu — CATEGORY seul pilote l\'affichage', () => {
    const { container } = render(<QuestionCard question={baseQuestion({
      TYPE: 'RAFALE', CATEGORY: 'HISTORY', RAFALE_CATEGORIES: ['ANIMALS', 'SPORTS'],
    })} />)
    expect(container.querySelectorAll('.category-badge').length).toBe(1)
    expect(screen.getByTitle('Histoire')).toBeInTheDocument()
    expect(screen.queryByTitle('Animaux')).not.toBeInTheDocument()
  })
})
