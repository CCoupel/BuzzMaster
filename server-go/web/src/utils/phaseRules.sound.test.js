import { describe, it, expect } from 'vitest'
import {
  showSoundRow,
  soundReplayButtonState,
  soundPauseResumeButtonState,
  soundPauseResumeCommand,
  soundStopButtonState,
} from './phaseRules'

// ---------------------------------------------------------------------------
// phaseRules — son de la question (v11.1, #219), tâche 14 du plan
// _work/reports/plan-20260922-103848.md §7. Contrat websocket-actions.md
// §QUESTION_SOUND, maquette docs/mockups/question-sound-219.html §02.
//
// Dispatché en correctif de revue (finding MAJEUR du sous-reviewer
// frontend, _work/reports/code-review-frontend-20260922-150000.md) — le
// plan initial ne listait aucun test JS pour ce lot. Patron direct :
// phaseRules.entracte.test.js (même fichier source, même style
// fonctions-pures/it.each).
//
// Cinq fonctions, toutes pures — paramètres phase/soundState/question reçus
// en argument, aucun état interne (R4, dérive Go/JS déjà matérialisée 2×
// sur v7.0.0).
// ---------------------------------------------------------------------------

describe('showSoundRow — visible SEULEMENT si phase STARTED/PAUSED ET question.SOUND non vide', () => {
  it.each([
    ['STARTED', true],
    ['PAUSED', true],
    ['STOPPED', false],
    ['READY', false],
    ['COUNTDOWN', false],
    ['REVEALED', false],
    ['PREPARE', false],
    ['NEW_GAME', false],
  ])('phase %s, question avec SOUND → visible=%s', (phase, expected) => {
    expect(showSoundRow(phase, { SOUND: '/question/1/sound_1234.wav' })).toBe(expected)
  })

  it('phase STARTED mais question.SOUND vide/absent → jamais visible', () => {
    expect(showSoundRow('STARTED', { SOUND: '' })).toBe(false)
    expect(showSoundRow('STARTED', {})).toBe(false)
  })

  it('phase STARTED, question null/undefined → jamais visible (pas de crash)', () => {
    expect(showSoundRow('STARTED', null)).toBe(false)
    expect(showSoundRow('STARTED', undefined)).toBe(false)
  })

  it('phase PAUSED, question avec SOUND → visible (maquette §02, écran PAUSED)', () => {
    expect(showSoundRow('PAUSED', { SOUND: '/x.wav' })).toBe(true)
  })
})

describe('soundReplayButtonState — toujours actionnable dès que la rangée est visible', () => {
  it("renvoie 'optional' inconditionnellement (maquette §02 : actif dans les 4 états illustrés)", () => {
    expect(soundReplayButtonState()).toBe('optional')
    expect(soundReplayButtonState('IDLE')).toBe('optional')
    expect(soundReplayButtonState('PLAYING')).toBe('optional')
    expect(soundReplayButtonState('PAUSED')).toBe('optional')
  })
})

describe('soundPauseResumeButtonState — actionnable seulement pendant PLAYING/PAUSED', () => {
  it.each([
    ['PLAYING', 'optional'],
    ['PAUSED', 'optional'],
    ['IDLE', 'off'],
    [undefined, 'off'],
    ['', 'off'],
    ['UNE_VALEUR_FUTURE_INCONNUE', 'off'],
  ])('soundState=%s → %s', (soundState, expected) => {
    expect(soundPauseResumeButtonState(soundState)).toBe(expected)
  })
})

describe('soundPauseResumeCommand — un seul bouton bascule Pause ↔ Reprendre (maquette §02)', () => {
  it("soundState='PAUSED' → commande 'RESUME'", () => {
    expect(soundPauseResumeCommand('PAUSED')).toBe('RESUME')
  })

  it("tout autre état (PLAYING, IDLE, absent) → commande 'PAUSE'", () => {
    expect(soundPauseResumeCommand('PLAYING')).toBe('PAUSE')
    expect(soundPauseResumeCommand('IDLE')).toBe('PAUSE')
    expect(soundPauseResumeCommand(undefined)).toBe('PAUSE')
  })
})

describe('soundStopButtonState — actionnable seulement pendant PLAYING/PAUSED', () => {
  it.each([
    ['PLAYING', 'optional'],
    ['PAUSED', 'optional'],
    ['IDLE', 'off'],
    [undefined, 'off'],
  ])('soundState=%s → %s', (soundState, expected) => {
    expect(soundStopButtonState(soundState)).toBe(expected)
  })
})

// ---------------------------------------------------------------------------
// Cohérence croisée — Pause↔Resume et Stop partagent la même garde
// d'activation (maquette §02 : les trois gestes sont actionnables ensemble,
// jamais l'un sans l'autre pendant PLAYING/PAUSED).
// ---------------------------------------------------------------------------

describe('Pause↔Resume et Stop restent synchronisés (même garde PLAYING/PAUSED)', () => {
  it.each(['PLAYING', 'PAUSED', 'IDLE', undefined])('soundState=%s : pauseResume et stop rendent le même état', (soundState) => {
    expect(soundPauseResumeButtonState(soundState)).toBe(soundStopButtonState(soundState))
  })
})
