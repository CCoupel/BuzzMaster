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

  describe('MEMOTION (v8.0.0, #201 suivi, miroir engine.go participantsConform SHA d3c6fb20 — durci pour être symétrique à MEMORY, remplace "au moins une équipe" sans distinction SOLO/multi)', () => {
    it.each([
      [0, false],
      [1, true],
      [2, false],
      [3, false],
    ])('MOTION_MODE vide (défaut SOLO), %i équipe(s) → conforme=%s', (count, expected) => {
      const participating = Array.from({ length: count }, (_, i) => `T${i}`)
      expect(participantsConform({ TYPE: 'MEMOTION' }, participating)).toBe(expected)
    })

    it.each([
      [0, false],
      [1, true],
      [2, false],
    ])('MOTION_MODE explicite "SOLO", %i équipe(s) → conforme=%s', (count, expected) => {
      const participating = Array.from({ length: count }, (_, i) => `T${i}`)
      expect(participantsConform({ TYPE: 'MEMOTION', MOTION_MODE: 'SOLO' }, participating)).toBe(expected)
    })

    it.each(['CHACUN_SON_TOUR', 'TANT_QUE_JE_GAGNE'])(
      'mode multi %s : 0 équipe → non conforme, 1 → non conforme, 2 → conforme, 3 → conforme',
      (mode) => {
        expect(participantsConform({ TYPE: 'MEMOTION', MOTION_MODE: mode }, [])).toBe(false)
        expect(participantsConform({ TYPE: 'MEMOTION', MOTION_MODE: mode }, ['A'])).toBe(false)
        expect(participantsConform({ TYPE: 'MEMOTION', MOTION_MODE: mode }, ['A', 'B'])).toBe(true)
        expect(participantsConform({ TYPE: 'MEMOTION', MOTION_MODE: mode }, ['A', 'B', 'C'])).toBe(true)
      }
    )
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

    it('MEMOTION SOLO (défaut), aucune équipe sélectionnée → "sélectionnez une équipe" (complet) / "1 équipe" (court) — #201 suivi, avant #201 "sélectionnez au moins une équipe"', () => {
      const activeTeams = [readyTeam('A')]
      const gameState = { MEMOTION_PARTICIPATING_TEAMS: [] }
      const question = { TYPE: 'MEMOTION' }
      expect(prepareWaitReason('PREPARE', question, activeTeams, gameState)).toBe('sélectionnez une équipe')
      expect(prepareWaitReason('PREPARE', question, activeTeams, gameState, { short: true })).toBe('1 équipe')
    })

    it('MEMOTION multi (CHACUN_SON_TOUR), une seule équipe sélectionnée → "sélectionnez au moins deux équipes" (complet) / "2 équipes" (court) — #201 suivi, avant #201 une seule équipe suffisait', () => {
      const activeTeams = [readyTeam('A'), readyTeam('B')]
      const gameState = { MEMOTION_PARTICIPATING_TEAMS: ['A'] }
      const question = { TYPE: 'MEMOTION', MOTION_MODE: 'CHACUN_SON_TOUR' }
      expect(prepareWaitReason('PREPARE', question, activeTeams, gameState)).toBe('sélectionnez au moins deux équipes')
      expect(prepareWaitReason('PREPARE', question, activeTeams, gameState, { short: true })).toBe('2 équipes')
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
        'sélectionnez une équipe'
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

// ---------------------------------------------------------------------------
// Addendum média indisponible (v11.1, #219/#236/#237, plan
// _work/reports/plan-20260923-101500.md rév. 8 §6 tâche 13, contrat
// sound.md §10.8) — la branche `GAME.QUESTION_SOUND_UNAVAILABLE`, dernière
// avant le repli permissif de `prepareWaitReason`. Dispatché en correctif
// de revue (le dispatch initial du CDP ne listait pas explicitement ce
// fichier, mais le plan §6 tâche 13 le demande : "prepareWaitReason.test.js
// (3 motifs × 2 formulations)").
// ---------------------------------------------------------------------------

describe('prepareWaitReason — addendum média indisponible (v11.1, #219/#236/#237)', () => {
  const readyTeam = (name) => ({ name, READY: true })
  // Buzzers déjà prêts et participants conformes (question SPEEDY, jamais
  // de garde participants) — pour isoler la SEULE branche son.
  const activeTeams = [readyTeam('A')]
  const question = { TYPE: 'SPEEDY' }

  it.each([
    ['DISABLED', 'son désactivé', 'Les sons sont désactivés dans la configuration. Les réactiver demande un redémarrage du serveur.'],
    ['OUTPUT', 'pas de sortie audio', "Aucune sortie audio n'a pu être ouverte au démarrage du serveur. Brancher l'enceinte puis redémarrer le serveur."],
    ['FILE', 'son introuvable', 'Le fichier son de cette question est introuvable ou invalide sur le serveur.'],
  ])('QUESTION_SOUND_UNAVAILABLE=%s → libellé court=%j, long=%j', (unavailable, wantShort, wantLong) => {
    const gameState = { QUESTION_SOUND_UNAVAILABLE: unavailable }
    expect(prepareWaitReason('PREPARE', question, activeTeams, gameState)).toBe(wantLong)
    expect(prepareWaitReason('PREPARE', question, activeTeams, gameState, { short: true })).toBe(wantShort)
  })

  it('chaque motif NOMME LE REMÈDE réel dans le libellé long (jamais une étiquette générique)', () => {
    const gameState = { QUESTION_SOUND_UNAVAILABLE: 'DISABLED' }
    expect(prepareWaitReason('PREPARE', question, activeTeams, gameState)).toMatch(/redémarrage du serveur/)

    expect(
      prepareWaitReason('PREPARE', question, activeTeams, { QUESTION_SOUND_UNAVAILABLE: 'OUTPUT' })
    ).toMatch(/brancher l'enceinte/i)
  })

  it('QUESTION_SOUND_UNAVAILABLE="" (disponible) → null, rien à expliquer', () => {
    expect(prepareWaitReason('PREPARE', question, activeTeams, { QUESTION_SOUND_UNAVAILABLE: '' })).toBeNull()
  })

  it('QUESTION_SOUND_UNAVAILABLE absent (clé manquante) → null, même comportement que ""', () => {
    expect(prepareWaitReason('PREPARE', question, activeTeams, {})).toBeNull()
  })

  it('motif inconnu (valeur future non prévue par la table) → null, permissif par défaut (jamais de crash)', () => {
    expect(
      prepareWaitReason('PREPARE', question, activeTeams, { QUESTION_SOUND_UNAVAILABLE: 'UNE_VALEUR_FUTURE_INCONNUE' })
    ).toBeNull()
  })

  it('la branche participants garde PRIORITÉ sur la branche son (ordre normatif : buzzers, puis participants, puis son)', () => {
    // MEMORY SOLO non conforme ET son indisponible en même temps — le motif
    // participants doit rester affiché, jamais masqué par le motif son.
    const memoryQuestion = { TYPE: 'MEMORY' }
    const gameState = { MEMORY_PARTICIPATING_TEAMS: [], QUESTION_SOUND_UNAVAILABLE: 'DISABLED' }
    expect(prepareWaitReason('PREPARE', memoryQuestion, activeTeams, gameState)).toBe('sélectionnez une équipe')
  })

  it('aucune garde de type : une question sans SOUND mais avec QUESTION_SOUND_UNAVAILABLE renseigné (état incohérent hypothétique) affiche quand même le motif — le fichier ne réinvente pas la garde serveur (§10.8.4)', () => {
    // Documente le choix explicite du handoff dev-frontend : ce fichier ne
    // reproduit PAS la garde "Question.SOUND == ''" côté serveur — en
    // pratique le serveur ne pousse jamais QUESTION_SOUND_UNAVAILABLE pour
    // une question sans son (sortie immédiate de sa propre branche), donc
    // cet état ne se produit jamais réellement ; ce test verrouille que le
    // fichier fait confiance à l'état serveur plutôt que de le
    // re-vérifier lui-même (cohérent avec le principe R4 du projet).
    const gameState = { QUESTION_SOUND_UNAVAILABLE: 'FILE' }
    expect(prepareWaitReason('PREPARE', { TYPE: 'SPEEDY' }, activeTeams, gameState)).not.toBeNull()
  })

  it('SPEEDY/QCM/ARDOISE en PREPARE, son disponible → toujours null (non-régression, complète le test existant "toujours null")', () => {
    ;['SPEEDY', 'QCM', 'ARDOISE'].forEach((type) => {
      const reason = prepareWaitReason(
        'PREPARE',
        { TYPE: type },
        activeTeams,
        { MEMORY_PARTICIPATING_TEAMS: [], MEMOTION_PARTICIPATING_TEAMS: [], QUESTION_SOUND_UNAVAILABLE: '' }
      )
      expect(reason).toBeNull()
    })
  })
})
