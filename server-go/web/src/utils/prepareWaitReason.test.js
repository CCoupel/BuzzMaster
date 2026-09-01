import { describe, it, expect } from 'vitest'
import { isTeamReady, participantsConform, prepareWaitReason } from './prepareWaitReason'

// ---------------------------------------------------------------------------
// prepareWaitReason.js — miroir client-side, en LECTURE SEULE, du prédicat
// serveur `participantsConform` (#172/B1, engine.go) et de la condition de
// sortie de PREPARE (#172/B2, main.go:1456). Ce fichier ne décide rien —
// couvert ici en isolation pure (pas de rendu), table de règles identique à
// celle vérifiée côté moteur par TestParticipantsConform_Rules (rapport QA
// #172, `_work/reports/qa-20260817-145412.md` §2).
// ---------------------------------------------------------------------------

describe('isTeamReady', () => {
  it('READY === true → prêt', () => {
    expect(isTeamReady({ READY: true })).toBe(true)
  })

  it('READY === "TRUE" (string, tolérance déjà utilisée GamePage.jsx:1015) → prêt', () => {
    expect(isTeamReady({ READY: 'TRUE' })).toBe(true)
  })

  it('READY === false → non prêt', () => {
    expect(isTeamReady({ READY: false })).toBe(false)
  })

  it('READY absent (undefined) → non prêt', () => {
    expect(isTeamReady({})).toBe(false)
  })

  it('team null/undefined → non prêt, ne lève pas', () => {
    expect(isTeamReady(null)).toBe(false)
    expect(isTeamReady(undefined)).toBe(false)
  })

  it('valeur READY inattendue ("false" string, 1, etc.) → non prêt', () => {
    expect(isTeamReady({ READY: 'false' })).toBe(false)
    expect(isTeamReady({ READY: 1 })).toBe(false)
  })
})

describe('participantsConform — table B1 (miroir engine.go)', () => {
  describe('MEMORY SOLO (MEMORY_MODE vide ou "SOLO")', () => {
    it.each([
      [0, false],
      [1, true],
      [2, false],
      [3, false],
    ])('MEMORY_MODE vide, %i équipe(s) → conforme=%s', (count, expected) => {
      const participating = Array.from({ length: count }, (_, i) => `T${i}`)
      expect(participantsConform({ TYPE: 'MEMORY' }, participating)).toBe(expected)
    })

    it.each([
      [0, false],
      [1, true],
      [2, false],
    ])('MEMORY_MODE explicite "SOLO", %i équipe(s) → conforme=%s', (count, expected) => {
      const participating = Array.from({ length: count }, (_, i) => `T${i}`)
      expect(participantsConform({ TYPE: 'MEMORY', MEMORY_MODE: 'SOLO' }, participating)).toBe(expected)
    })
  })

  describe('MEMORY multi (CHACUN_SON_TOUR / TANT_QUE_JE_GAGNE)', () => {
    it.each(['CHACUN_SON_TOUR', 'TANT_QUE_JE_GAGNE'])(
      'mode %s : 0 équipe → non conforme, 1 → non conforme, 2 → conforme, 3 → conforme',
      (mode) => {
        expect(participantsConform({ TYPE: 'MEMORY', MEMORY_MODE: mode }, [])).toBe(false)
        expect(participantsConform({ TYPE: 'MEMORY', MEMORY_MODE: mode }, ['A'])).toBe(false)
        expect(participantsConform({ TYPE: 'MEMORY', MEMORY_MODE: mode }, ['A', 'B'])).toBe(true)
        expect(participantsConform({ TYPE: 'MEMORY', MEMORY_MODE: mode }, ['A', 'B', 'C'])).toBe(true)
      }
    )
  })

  describe('MEMOTION (au moins une équipe, quel que soit MOTION_MODE — pas de branche dédiée)', () => {
    it.each([
      [0, false],
      [1, true],
      [2, true],
    ])('%i équipe(s) → conforme=%s', (count, expected) => {
      const participating = Array.from({ length: count }, (_, i) => `T${i}`)
      expect(participantsConform({ TYPE: 'MEMOTION' }, participating)).toBe(expected)
    })
  })

  describe('RAFALE (v8.0.0, #201, miroir engine.go participantsConform SHA e2917395/d3c6fb20 — durci pour être symétrique à MEMORY, remplace la règle #199 SHA 7b8659d5)', () => {
    it.each([
      [0, false],
      [1, true],
      [2, false],
      [3, false],
    ])('RAFALE_MODE vide (défaut SOLO), %i équipe(s) → conforme=%s', (count, expected) => {
      const participating = Array.from({ length: count }, (_, i) => `T${i}`)
      expect(participantsConform({ TYPE: 'RAFALE' }, participating)).toBe(expected)
    })

    it.each([
      [0, false],
      [1, true],
      [2, false],
    ])('RAFALE_MODE explicite "SOLO", %i équipe(s) → conforme=%s', (count, expected) => {
      const participating = Array.from({ length: count }, (_, i) => `T${i}`)
      expect(participantsConform({ TYPE: 'RAFALE', RAFALE_MODE: 'SOLO' }, participating)).toBe(expected)
    })

    it.each(['CHACUN_SON_TOUR', 'TANT_QUE_JE_GAGNE', 'MAILLON_FAIBLE'])(
      'mode multi %s : 0 équipe → non conforme, 1 → non conforme, 2 → conforme, 3 → conforme (désormais aligné sur MEMORY multi)',
      (mode) => {
        expect(participantsConform({ TYPE: 'RAFALE', RAFALE_MODE: mode }, [])).toBe(false)
        expect(participantsConform({ TYPE: 'RAFALE', RAFALE_MODE: mode }, ['A'])).toBe(false)
        expect(participantsConform({ TYPE: 'RAFALE', RAFALE_MODE: mode }, ['A', 'B'])).toBe(true)
        expect(participantsConform({ TYPE: 'RAFALE', RAFALE_MODE: mode }, ['A', 'B', 'C'])).toBe(true)
      }
    )
  })

  describe('SPEEDY / QCM / ARDOISE / type inconnu — permissif (déjà couvert par AreAllTeamsReady)', () => {
    it.each(['SPEEDY', 'QCM', 'ARDOISE', 'INCONNU', undefined])(
      'type %s : toujours conforme, même avec 0 équipe',
      (type) => {
        expect(participantsConform({ TYPE: type }, [])).toBe(true)
      }
    )

    it('question null → conforme (permissif)', () => {
      expect(participantsConform(null, [])).toBe(true)
    })
  })

  it('participating undefined → traité comme liste vide, pas de crash', () => {
    expect(participantsConform({ TYPE: 'MEMORY' }, undefined)).toBe(false)
  })
})

describe('prepareWaitReason — orchestration (phase, buzzers, conformité)', () => {
  const readyTeam = (name) => ({ name, READY: true })
  const notReadyTeam = (name) => ({ name, READY: false })

  it('hors PREPARE → toujours null, quelle que soit la situation', () => {
    ;['NEW_GAME', 'ENROLL', 'READY', 'COUNTDOWN', 'STARTED', 'PAUSED', 'STOPPED', 'REVEALED'].forEach((phase) => {
      expect(
        prepareWaitReason(phase, { TYPE: 'MEMORY' }, [notReadyTeam('A')], { MEMORY_PARTICIPATING_TEAMS: [] })
      ).toBeNull()
    })
  })

  it('un buzzer actif non prêt → "Buzzers en attente" (libellé complet), prioritaire sur la conformité', () => {
    // Sélection déjà conforme (MEMORY SOLO, 1 équipe) mais une équipe active
    // (non sélectionnée) n'a pas de buzzer prêt : le motif buzzers prime.
    const reason = prepareWaitReason(
      'PREPARE',
      { TYPE: 'MEMORY' },
      [readyTeam('A'), notReadyTeam('B')],
      { MEMORY_PARTICIPATING_TEAMS: ['A'] }
    )
    expect(reason).toBe('Buzzers en attente')
  })

  it('un buzzer actif non prêt → "buzzers" (libellé court, opts.short)', () => {
    const reason = prepareWaitReason(
      'PREPARE',
      { TYPE: 'MEMORY' },
      [notReadyTeam('A')],
      { MEMORY_PARTICIPATING_TEAMS: [] },
      { short: true }
    )
    expect(reason).toBe('buzzers')
  })

  describe('buzzers tous prêts, sélection non conforme → motif mode-spécifique', () => {
    it('MEMORY SOLO, aucune équipe sélectionnée → "sélectionnez une équipe" (complet) / "1 équipe" (court)', () => {
      const activeTeams = [readyTeam('A'), readyTeam('B')]
      const gameState = { MEMORY_PARTICIPATING_TEAMS: [] }
      expect(prepareWaitReason('PREPARE', { TYPE: 'MEMORY' }, activeTeams, gameState)).toBe('sélectionnez une équipe')
      expect(prepareWaitReason('PREPARE', { TYPE: 'MEMORY' }, activeTeams, gameState, { short: true })).toBe('1 équipe')
    })

    it('MEMORY multi, une seule équipe sélectionnée → "sélectionnez au moins deux équipes" (complet) / "2 équipes" (court)', () => {
      const activeTeams = [readyTeam('A'), readyTeam('B')]
      const gameState = { MEMORY_PARTICIPATING_TEAMS: ['A'] }
      const question = { TYPE: 'MEMORY', MEMORY_MODE: 'CHACUN_SON_TOUR' }
      expect(prepareWaitReason('PREPARE', question, activeTeams, gameState)).toBe('sélectionnez au moins deux équipes')
      expect(prepareWaitReason('PREPARE', question, activeTeams, gameState, { short: true })).toBe('2 équipes')
    })

    it('MEMOTION, aucune équipe sélectionnée → "sélectionnez au moins une équipe" (complet) / "1 équipe" (court)', () => {
      const activeTeams = [readyTeam('A')]
      const gameState = { MEMOTION_PARTICIPATING_TEAMS: [] }
      const question = { TYPE: 'MEMOTION' }
      expect(prepareWaitReason('PREPARE', question, activeTeams, gameState)).toBe('sélectionnez au moins une équipe')
      expect(prepareWaitReason('PREPARE', question, activeTeams, gameState, { short: true })).toBe('1 équipe')
    })

    it('RAFALE SOLO, aucune équipe sélectionnée → "sélectionnez une équipe" (complet) / "1 équipe" (court) — #201, avant #201 toujours conforme donc null', () => {
      const activeTeams = [readyTeam('A'), readyTeam('B')]
      const gameState = { RAFALE_PARTICIPATING_TEAMS: [] }
      const question = { TYPE: 'RAFALE' }
      expect(prepareWaitReason('PREPARE', question, activeTeams, gameState)).toBe('sélectionnez une équipe')
      expect(prepareWaitReason('PREPARE', question, activeTeams, gameState, { short: true })).toBe('1 équipe')
    })

    it('RAFALE multi (CHACUN_SON_TOUR), une seule équipe sélectionnée → "sélectionnez au moins deux équipes participantes" (complet) / "2 équipes" (court) — #201, avant #201 une seule équipe suffisait', () => {
      const activeTeams = [readyTeam('A'), readyTeam('B')]
      const gameState = { RAFALE_PARTICIPATING_TEAMS: ['A'] }
      const question = { TYPE: 'RAFALE', RAFALE_MODE: 'CHACUN_SON_TOUR' }
      expect(prepareWaitReason('PREPARE', question, activeTeams, gameState)).toBe('sélectionnez au moins deux équipes participantes')
      expect(prepareWaitReason('PREPARE', question, activeTeams, gameState, { short: true })).toBe('2 équipes')
    })

    it('RAFALE lit RAFALE_PARTICIPATING_TEAMS, pas MEMORY_PARTICIPATING_TEAMS', () => {
      // Même piège que MEMOTION ci-dessous : si le mauvais champ était lu,
      // cette sélection MEMORY à 3 équipes masquerait à tort le motif RAFALE
      // (liste RAFALE_PARTICIPATING_TEAMS vide).
      const activeTeams = [readyTeam('A')]
      const gameState = {
        MEMORY_PARTICIPATING_TEAMS: ['A', 'B', 'C'],
        RAFALE_PARTICIPATING_TEAMS: [],
      }
      expect(prepareWaitReason('PREPARE', { TYPE: 'RAFALE', RAFALE_MODE: 'MAILLON_FAIBLE' }, activeTeams, gameState)).toBe(
        'sélectionnez au moins deux équipes participantes'
      )
    })

    it('MEMOTION lit MEMOTION_PARTICIPATING_TEAMS, pas MEMORY_PARTICIPATING_TEAMS', () => {
      // Piège : si le mauvais champ était lu, cette sélection MEMORY à 3
      // équipes masquerait à tort le motif MEMOTION (liste vide).
      const activeTeams = [readyTeam('A')]
      const gameState = {
        MEMORY_PARTICIPATING_TEAMS: ['A', 'B', 'C'],
        MEMOTION_PARTICIPATING_TEAMS: [],
      }
      expect(prepareWaitReason('PREPARE', { TYPE: 'MEMOTION' }, activeTeams, gameState)).toBe(
        'sélectionnez au moins une équipe'
      )
    })
  })

  it('buzzers tous prêts et sélection conforme → null (rien à expliquer)', () => {
    const activeTeams = [readyTeam('A'), readyTeam('B')]
    const gameState = { MEMORY_PARTICIPATING_TEAMS: ['A'] }
    expect(prepareWaitReason('PREPARE', { TYPE: 'MEMORY' }, activeTeams, gameState)).toBeNull()
  })

  it('SPEEDY/QCM/ARDOISE en PREPARE, buzzers prêts → toujours null (non-régression, aucun changement de comportement #172)', () => {
    ;['SPEEDY', 'QCM', 'ARDOISE'].forEach((type) => {
      const reason = prepareWaitReason(
        'PREPARE',
        { TYPE: type },
        [readyTeam('A')],
        { MEMORY_PARTICIPATING_TEAMS: [], MEMOTION_PARTICIPATING_TEAMS: [] }
      )
      expect(reason).toBeNull()
    })
  })

  it('activeTeams vide (aucune équipe active) → pas de "Buzzers en attente", retombe sur la conformité', () => {
    const reason = prepareWaitReason('PREPARE', { TYPE: 'MEMORY' }, [], { MEMORY_PARTICIPATING_TEAMS: [] })
    expect(reason).toBe('sélectionnez une équipe')
  })

  it('question null en PREPARE → permissif, null', () => {
    expect(prepareWaitReason('PREPARE', null, [readyTeam('A')], {})).toBeNull()
  })
})
