import { describe, it, expect } from 'vitest'
import { pickAutoProvider } from './ConfigPage'

// Tests ciblés sur la fonction pure `pickAutoProvider` (bugfix/config-api-key-help,
// fix revue de code cycle 2 — bug critique : l'auto-sélection au montage écrasait
// silencieusement un choix Groq explicite dès que Claude avait aussi une clé).
//
// Volontairement isolé de ConfigPage.test.jsx / ConfigPage.ai.test.jsx (aucun render,
// aucun fetch) : ces derniers restent bloqués dans l'environnement WSL de dev (cf.
// handoff dev-frontend), ce test doit lui s'exécuter indépendamment de ce blocage.
describe('pickAutoProvider', () => {
  it('respecte une préférence Groq déjà enregistrée même si Claude a aussi une clé (bug critique cycle 1)', () => {
    // Repro exacte du bug : les deux clés existent, l'utilisateur a explicitement
    // choisi Groq (persisté côté serveur) → un remontage ne doit PAS revenir à Claude.
    expect(pickAutoProvider(true, true, 'groq')).toBe('groq')
  })

  it('respecte une préférence Claude déjà enregistrée quand les deux clés existent', () => {
    expect(pickAutoProvider(true, true, 'anthropic')).toBe('anthropic')
  })

  it('auto-sélectionne Claude en priorité quand la sélection courante est invalide et les deux clés existent', () => {
    // ex: défaut backend "anthropic" alors que Claude n'a en réalité pas de clé
    // (claudeConfigured=false) puis une clé Claude est ajoutée : current='anthropic'
    // était déjà invalide (pas de clé), donc l'auto-pick se déclenche et choisit Claude.
    expect(pickAutoProvider(true, true, 'invalid-or-unset')).toBe('anthropic')
  })

  it('auto-sélectionne Groq quand seule la clé Groq est disponible et la sélection courante est invalide', () => {
    expect(pickAutoProvider(false, true, 'anthropic')).toBe('groq')
  })

  it('ne bascule pas sur Claude si Groq est actif et reste valide (Groq configuré)', () => {
    expect(pickAutoProvider(false, true, 'groq')).toBe('groq')
  })

  it('conserve la sélection courante quand aucune clé n\'est disponible (aucune sélection valide possible)', () => {
    expect(pickAutoProvider(false, false, 'anthropic')).toBe('anthropic')
    expect(pickAutoProvider(false, false, 'groq')).toBe('groq')
  })

  it('bascule sur Groq quand la clé du fournisseur actuellement sélectionné (Claude) est supprimée', () => {
    // handleClearAiKey : claudeConfigured devient false après suppression.
    expect(pickAutoProvider(false, true, 'anthropic')).toBe('groq')
  })

  it('bascule sur Claude quand la clé du fournisseur actuellement sélectionné (Groq) est supprimée', () => {
    // handleClearGroqKey : groqConfigured devient false après suppression.
    expect(pickAutoProvider(true, false, 'groq')).toBe('anthropic')
  })

  it('ne bascule pas si on supprime la clé du fournisseur NON sélectionné', () => {
    // Claude sélectionné et valide ; on supprime la clé Groq (non active) → pas de changement.
    expect(pickAutoProvider(true, false, 'anthropic')).toBe('anthropic')
  })
})
