/**
 * Tests for utils/questionTypeMeta.js — table unique des types de question
 * (#183, A-T1 — filet de non-régression du refactor A-F2).
 *
 * A-F2 fusionne trois tables locales qui divergeaient silencieusement
 * (`QuestionCard.jsx` TYPE_LABELS, `AIGenerateModal.jsx` TYPES,
 * `QuestionsPage.jsx` boutons codés en dur) dans `questionTypeMeta.js`,
 * source unique. Ce fichier vérifie :
 *   1. Le registre exporte les 5 types connus, avec icon+label+color.
 *   2. getQuestionTypeMeta() : repli SPEEDY sur type absent/vide (conservé,
 *      §3 du contrat) ; PLUS de repli silencieux sur SPEEDY pour un type
 *      renseigné mais inconnu (critère d'acceptance #183 — signalement
 *      explicite à la place).
 *   3. Unicité de la table : QuestionCard.jsx et AIGenerateModal.jsx ne
 *      portent plus de copie locale des libellés/icônes de type — scan de
 *      source, même technique que AnimMotionActions.test.jsx (fs.readFileSync
 *      + assertion sur l'absence d'un motif, pas de duplication de logique).
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import {
  QUESTION_TYPES,
  QUESTION_TYPE_META,
  GENERABLE_TYPES,
  getQuestionTypeMeta,
} from './questionTypeMeta'

const HERE = path.dirname(fileURLToPath(import.meta.url))
const QUESTION_CARD_PATH = path.join(HERE, '..', 'components', 'QuestionCard.jsx')
const AI_GENERATE_MODAL_PATH = path.join(HERE, '..', 'components', 'AIGenerateModal.jsx')

const KNOWN_TYPES = ['SPEEDY', 'QCM', 'MEMORY', 'MEMOTION', 'ARDOISE']

// ---------------------------------------------------------------------------
// 1. Le registre — 5 types connus, forme attendue.
// ---------------------------------------------------------------------------

describe('questionTypeMeta — registre QUESTION_TYPES / QUESTION_TYPE_META', () => {
  it('QUESTION_TYPES contient exactement les 5 types connus, une fois chacun', () => {
    const keys = QUESTION_TYPES.map(t => t.key)
    expect(keys).toHaveLength(5)
    expect(new Set(keys).size).toBe(5)
    KNOWN_TYPES.forEach(type => expect(keys).toContain(type))
  })

  it.each(KNOWN_TYPES)('QUESTION_TYPES[%s] porte un label et une icône non vides', (type) => {
    const entry = QUESTION_TYPES.find(t => t.key === type)
    expect(entry).toBeDefined()
    expect(entry.label).toBeTruthy()
    expect(entry.icon).toBeTruthy()
  })

  it('QUESTION_TYPE_META est dérivée de QUESTION_TYPES (même 5 clés, même contenu)', () => {
    expect(Object.keys(QUESTION_TYPE_META).sort()).toEqual([...KNOWN_TYPES].sort())
    KNOWN_TYPES.forEach(type => {
      const fromArray = QUESTION_TYPES.find(t => t.key === type)
      expect(QUESTION_TYPE_META[type]).toEqual(fromArray)
    })
  })
})

// ---------------------------------------------------------------------------
// `nestable` (#184/B-F4) — miroir JS de `TypeDescriptor.NestableInMotionCard`
// (registre Go, `internal/game/question_types.go`, contrat §7). Ajouté par
// B-T1 (#184) : c'est le point de rendez-vous Go/JS le plus exposé aux
// dérives silencieuses (une des deux tables oubliée lors d'un futur type
// nestable, ex. #186/#187) — un test qui épingle les 5 valeurs attendues
// des deux côtés le détecte immédiatement plutôt qu'à l'usage.
// ---------------------------------------------------------------------------

describe('questionTypeMeta — nestable (#184/B-F4, miroir de NestableInMotionCard)', () => {
  // Valeurs attendues = table du contrat §7 / registre Go
  // (internal/game/question_types.go) au 2026-08-24 : SPEEDY et QCM
  // nestables depuis v7.0.0 ; MEMORY rejoint en v7.1.0 (#187). ARDOISE reste
  // définitivement non nestable — #186 fermée « not planned » le 2026-08-24
  // (contrat §7.2). MEMOTION jamais (profondeur plafonnée à 1, contrat §1).
  const EXPECTED_NESTABLE = {
    SPEEDY: true,
    QCM: true,
    MEMORY: true,
    MEMOTION: false,
    ARDOISE: false,
  }

  it.each(KNOWN_TYPES)('QUESTION_TYPES[%s] porte un champ `nestable` booléen', (type) => {
    const entry = QUESTION_TYPES.find(t => t.key === type)
    expect(typeof entry.nestable).toBe('boolean')
  })

  it.each(Object.entries(EXPECTED_NESTABLE))('nestable(%s) == %s (miroir du registre Go)', (type, expected) => {
    const entry = QUESTION_TYPES.find(t => t.key === type)
    expect(entry.nestable).toBe(expected)
  })
})

// ---------------------------------------------------------------------------
// GENERABLE_TYPES (#196, v7.1.0) — export séparé pour la modale de
// génération IA, miroir JS de `generableQuestionTypes`
// (internal/server/ai_generator.go). Contrat ai-generation.md §3ter :
// invariant central testé ici — QUESTION_TYPES ne doit JAMAIS gagner le
// pseudo-type MEMOTION_PLUS, quel que soit ce qui est ajouté à
// GENERABLE_TYPES. C'est le critère de fait explicitement demandé par le
// CDP au dispatch de #196.
// ---------------------------------------------------------------------------

describe('GENERABLE_TYPES — pseudo-type MEMOTION_PLUS, séparé de QUESTION_TYPES (#196)', () => {
  it('QUESTION_TYPES contient TOUJOURS exactement 5 entrées — MEMOTION_PLUS ne doit JAMAIS y fuiter', () => {
    expect(QUESTION_TYPES).toHaveLength(5)
    expect(QUESTION_TYPES.some(t => t.key === 'MEMOTION_PLUS')).toBe(false)
  })

  it('GENERABLE_TYPES contient exactement 6 entrées : les 5 types réels + MEMOTION_PLUS', () => {
    const keys = GENERABLE_TYPES.map(t => t.key)
    expect(keys).toHaveLength(6)
    expect(new Set(keys).size).toBe(6)
    KNOWN_TYPES.forEach(type => expect(keys).toContain(type))
    expect(keys).toContain('MEMOTION_PLUS')
  })

  it('GENERABLE_TYPES contient bien les 5 types réels de QUESTION_TYPES, inchangés (même objets)', () => {
    KNOWN_TYPES.forEach(type => {
      const fromQuestionTypes = QUESTION_TYPES.find(t => t.key === type)
      const fromGenerable = GENERABLE_TYPES.find(t => t.key === type)
      expect(fromGenerable).toEqual(fromQuestionTypes)
    })
  })

  it('MEMOTION_PLUS est affiché "MEMOTION+" (contrat ai-generation.md §3ter)', () => {
    const entry = GENERABLE_TYPES.find(t => t.key === 'MEMOTION_PLUS')
    expect(entry).toBeDefined()
    expect(entry.label).toBe('MEMOTION+')
    expect(entry.icon).toBeTruthy()
    expect(entry.color).toBeTruthy()
  })

  it('MEMOTION_PLUS est inséré juste après MEMOTION, avant ARDOISE (ordre de l\'exemple de payload du contrat §3)', () => {
    const keys = GENERABLE_TYPES.map(t => t.key)
    const memotionIdx = keys.indexOf('MEMOTION')
    const memotionPlusIdx = keys.indexOf('MEMOTION_PLUS')
    const ardoiseIdx = keys.indexOf('ARDOISE')
    expect(memotionPlusIdx).toBe(memotionIdx + 1)
    expect(memotionPlusIdx).toBeLessThan(ardoiseIdx)
  })

  it('MEMOTION_PLUS n\'a pas de champ `nestable` — le pseudo-type n\'est pas imbricable, il n\'existe pas hors génération', () => {
    const entry = GENERABLE_TYPES.find(t => t.key === 'MEMOTION_PLUS')
    expect(entry.nestable).toBeUndefined()
  })

  it('QUESTION_TYPE_META reste dérivée EXCLUSIVEMENT de QUESTION_TYPES — MEMOTION_PLUS n\'y apparaît jamais', () => {
    expect(QUESTION_TYPE_META.MEMOTION_PLUS).toBeUndefined()
  })

  it('getQuestionTypeMeta("MEMOTION_PLUS") ne retourne pas la meta MEMOTION_PLUS de GENERABLE_TYPES (le pseudo-type est inconnu du registre réel)', () => {
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
    const meta = getQuestionTypeMeta('MEMOTION_PLUS')
    expect(meta.key).toBeNull() // UNKNOWN_TYPE_META — jamais confondu avec un vrai type
    errorSpy.mockRestore()
  })
})

// ---------------------------------------------------------------------------
// 2. getQuestionTypeMeta() — repli SPEEDY (type absent) vs signalement
//    explicite (type renseigné mais inconnu, #183).
// ---------------------------------------------------------------------------

describe('getQuestionTypeMeta — repli SPEEDY (type absent) préservé', () => {
  it.each([undefined, null, ''])('type=%p → repli sur la meta SPEEDY', (type) => {
    expect(getQuestionTypeMeta(type)).toEqual(QUESTION_TYPE_META.SPEEDY)
  })

  it.each(KNOWN_TYPES)('type=%s → sa propre meta (pas de repli)', (type) => {
    expect(getQuestionTypeMeta(type)).toEqual(QUESTION_TYPE_META[type])
  })
})

describe('getQuestionTypeMeta — type renseigné mais inconnu : PLUS de repli silencieux sur SPEEDY (#183)', () => {
  let errorSpy

  beforeEach(() => {
    errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  afterEach(() => {
    errorSpy.mockRestore()
  })

  it('un type inconnu (ex: "FOO") ne renvoie PAS la meta SPEEDY', () => {
    const meta = getQuestionTypeMeta('FOO')
    expect(meta).not.toEqual(QUESTION_TYPE_META.SPEEDY)
    expect(meta.label).not.toBe(QUESTION_TYPE_META.SPEEDY.label)
    expect(meta.icon).not.toBe(QUESTION_TYPE_META.SPEEDY.icon)
  })

  it('un type inconnu renvoie un objet meta distinct, toujours { icon, label } exploitable (pas de crash consommateur)', () => {
    const meta = getQuestionTypeMeta('FOO')
    expect(typeof meta.label).toBe('string')
    expect(meta.label.length).toBeGreaterThan(0)
    expect(typeof meta.icon).toBe('string')
    expect(meta.icon.length).toBeGreaterThan(0)
  })

  it('un type inconnu déclenche un signalement explicite (console.error) — pas un échec silencieux', () => {
    getQuestionTypeMeta('FOO')
    expect(errorSpy).toHaveBeenCalled()
  })

  it('deux types inconnus différents ne sont pas confondus avec un type connu quelconque', () => {
    const metaFoo = getQuestionTypeMeta('FOO')
    KNOWN_TYPES.forEach(type => {
      expect(metaFoo).not.toEqual(QUESTION_TYPE_META[type])
    })
  })
})

// ---------------------------------------------------------------------------
// 3. Unicité de la table côté JS (#183 critère d'acceptance) — QuestionCard.jsx
//    et AIGenerateModal.jsx ne portent plus de copie locale des libellés de
//    type. Scan de source (comme AnimMotionActions.test.jsx pour la palette
//    CSS) : on vérifie l'ABSENCE d'une table locale, pas un détail de rendu.
// ---------------------------------------------------------------------------

describe('Unicité de la table JS — QuestionCard.jsx ne porte plus de copie locale (#183/A-F2)', () => {
  const source = fs.readFileSync(QUESTION_CARD_PATH, 'utf-8')

  it('importe getQuestionTypeMeta (ou QUESTION_TYPES/QUESTION_TYPE_META) depuis utils/questionTypeMeta', () => {
    expect(source).toMatch(/from ['"]\.\.\/utils\/questionTypeMeta['"]/)
  })

  it("ne redéfinit plus de table locale TYPE_LABELS (l'ancienne copie #183 devait éliminer)", () => {
    expect(source).not.toMatch(/\bTYPE_LABELS\s*=/)
  })

  it("ne contient plus les 5 libellés de type en dur dans un même littéral objet (signature de l'ancienne table locale)", () => {
    // L'ancienne table locale portait les 5 paires clé/libellé consécutives.
    // Une seule survivance légitime des chaînes de type est l'usage ponctuel
    // (ex: classes CSS `type-${...}`), jamais les 5 réunies en bloc.
    const oldTableShape = /SPEEDY[^}]*QCM[^}]*MEMORY[^}]*MEMOTION[^}]*ARDOISE/s
    expect(source).not.toMatch(oldTableShape)
  })
})

describe('Unicité de la table JS — AIGenerateModal.jsx ne porte plus de copie locale (#183/A-F2)', () => {
  const source = fs.readFileSync(AI_GENERATE_MODAL_PATH, 'utf-8')

  // #196 (v7.1.0) — depuis l'ajout du pseudo-type MEMOTION_PLUS, la modale de
  // génération importe GENERABLE_TYPES (5 types réels + MEMOTION_PLUS),
  // JAMAIS QUESTION_TYPES directement : c'est le point exact que le contrat
  // ai-generation.md §3ter rend normatif (« Ne JAMAIS ajouter MEMOTION_PLUS
  // à QUESTION_TYPES »). Une régression qui réintroduirait
  // `import { QUESTION_TYPES }` ferait disparaître MEMOTION_PLUS de la
  // modale silencieusement — ce test la détecte.
  it('importe GENERABLE_TYPES (pas QUESTION_TYPES) depuis utils/questionTypeMeta (#196)', () => {
    expect(source).toMatch(/from ['"]\.\.\/utils\/questionTypeMeta['"]/)
    expect(source).toMatch(/import\s*\{\s*GENERABLE_TYPES\s*\}\s*from ['"]\.\.\/utils\/questionTypeMeta['"]/)
  })

  it("ne redéfinit plus les 5 types (clé/libellé/couleur) en dur dans un littéral local", () => {
    const oldTableShape = /key:\s*['"]SPEEDY['"][^]*key:\s*['"]QCM['"][^]*key:\s*['"]MEMORY['"][^]*key:\s*['"]MEMOTION['"][^]*key:\s*['"]ARDOISE['"]/
    expect(source).not.toMatch(oldTableShape)
  })
})
