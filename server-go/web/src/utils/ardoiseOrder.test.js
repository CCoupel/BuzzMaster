import { describe, it, expect } from 'vitest'
import { formatArdoiseDelay, sortArdoiseEntries } from './ardoiseOrder'

// ---------------------------------------------------------------------------
// ardoiseOrder — tri et délai ARDOISE (#158/F1), extraction PURE de
// GamePage.jsx (formatArdoiseDelay ~37-43, sortArdoiseEntries ~209-224),
// consommée désormais par /admin ET /anim (AnimArdoiseList, F2). Signatures
// et comportement inchangés — ce fichier fixe le contrat en TDD, avant que
// dev-frontend extraie, d'après la lecture directe du code GamePage.jsx
// existant (pas une supposition).
// ---------------------------------------------------------------------------

describe('formatArdoiseDelay', () => {
  it('formate le délai en secondes, 3 décimales, depuis STARTED_AT - gameTime (µs -> s)', () => {
    // STARTED_AT 2.5s après le départ de la question (gameTime)
    expect(formatArdoiseDelay({ STARTED_AT: 12_500_000 }, 10_000_000)).toBe('2.500 s')
  })

  it('retourne null si STARTED_AT est absent/0', () => {
    expect(formatArdoiseDelay({ STARTED_AT: 0 }, 10_000_000)).toBeNull()
    expect(formatArdoiseDelay({}, 10_000_000)).toBeNull()
  })

  it('retourne null si answer est absent (null/undefined)', () => {
    expect(formatArdoiseDelay(null, 10_000_000)).toBeNull()
    expect(formatArdoiseDelay(undefined, 10_000_000)).toBeNull()
  })

  it('retourne null si gameTime est absent (0, null, undefined) — pas de référence pour calculer un délai', () => {
    expect(formatArdoiseDelay({ STARTED_AT: 12_500_000 }, 0)).toBeNull()
    expect(formatArdoiseDelay({ STARTED_AT: 12_500_000 }, null)).toBeNull()
    expect(formatArdoiseDelay({ STARTED_AT: 12_500_000 }, undefined)).toBeNull()
  })

  it('retourne null si le délai est négatif (resync/rejeu, STARTED_AT antérieur à gameTime)', () => {
    expect(formatArdoiseDelay({ STARTED_AT: 5_000_000 }, 10_000_000)).toBeNull()
  })

  it('formate 0.000 s pour une frappe exactement au départ de la question', () => {
    expect(formatArdoiseDelay({ STARTED_AT: 10_000_000 }, 10_000_000)).toBe('0.000 s')
  })
})

describe('sortArdoiseEntries', () => {
  const team = (name, extra = {}) => ({ NAME: name, ...extra })

  it('trie les répondants par STARTED_AT croissant', () => {
    const teams = [team('Lente'), team('Rapide'), team('Moyenne')]
    const answers = {
      Lente: { STARTED_AT: 30_000_000, TEXT: 'l' },
      Rapide: { STARTED_AT: 10_000_000, TEXT: 'r' },
      Moyenne: { STARTED_AT: 20_000_000, TEXT: 'm' },
    }
    const entries = sortArdoiseEntries(teams, answers)
    expect(entries.map(e => e.teamName)).toEqual(['Rapide', 'Moyenne', 'Lente'])
  })

  it('replie sur SUBMITTED_AT quand STARTED_AT vaut 0 (réponses enregistrées avant le fix historique)', () => {
    const teams = [team('A'), team('B')]
    const answers = {
      A: { STARTED_AT: 0, SUBMITTED_AT: 20_000_000, TEXT: 'a' },
      B: { STARTED_AT: 0, SUBMITTED_AT: 10_000_000, TEXT: 'b' },
    }
    const entries = sortArdoiseEntries(teams, answers)
    expect(entries.map(e => e.teamName)).toEqual(['B', 'A'])
  })

  it('place les équipes sans réponse en fin de liste, dans l\'ordre de la liste équipe', () => {
    const teams = [team('SansReponse1'), team('Repond'), team('SansReponse2')]
    const answers = {
      Repond: { STARTED_AT: 10_000_000, TEXT: 'x' },
    }
    const entries = sortArdoiseEntries(teams, answers)
    expect(entries.map(e => e.teamName)).toEqual(['Repond', 'SansReponse1', 'SansReponse2'])
    expect(entries[1].answer).toBeNull()
    expect(entries[2].answer).toBeNull()
  })

  it('départage stable par ordre d\'équipe à égalité stricte de STARTED_AT', () => {
    const teams = [team('Premiere'), team('Seconde'), team('Troisieme')]
    const answers = {
      Premiere: { STARTED_AT: 10_000_000, TEXT: 'p' },
      Seconde: { STARTED_AT: 10_000_000, TEXT: 's' },
      Troisieme: { STARTED_AT: 10_000_000, TEXT: 't' },
    }
    const entries = sortArdoiseEntries(teams, answers)
    // Égalité totale -> ordre de la liste d'équipes conservé (idx croissant).
    expect(entries.map(e => e.teamName)).toEqual(['Premiere', 'Seconde', 'Troisieme'])
  })

  it('aucune réponse du tout : conserve l\'ordre de la liste équipe', () => {
    const teams = [team('A'), team('B'), team('C')]
    const entries = sortArdoiseEntries(teams, {})
    expect(entries.map(e => e.teamName)).toEqual(['A', 'B', 'C'])
    entries.forEach(e => expect(e.answer).toBeNull())
  })

  it('gère team.name (minuscule) comme repli si team.NAME est absent', () => {
    const teams = [{ name: 'Minuscule' }]
    const answers = { Minuscule: { STARTED_AT: 10_000_000, TEXT: 'x' } }
    const entries = sortArdoiseEntries(teams, answers)
    expect(entries[0].teamName).toBe('Minuscule')
    expect(entries[0].answer).not.toBeNull()
  })

  it('liste d\'équipes vide -> liste vide', () => {
    expect(sortArdoiseEntries([], {})).toEqual([])
  })

  it('ardoiseAnswers absent (undefined) -> toutes les équipes sans réponse', () => {
    const teams = [team('A'), team('B')]
    const entries = sortArdoiseEntries(teams, undefined)
    expect(entries.map(e => e.teamName)).toEqual(['A', 'B'])
    entries.forEach(e => expect(e.answer).toBeNull())
  })
})
