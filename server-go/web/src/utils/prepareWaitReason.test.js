import { describe, it, expect } from 'vitest'
import { isTeamReady, participantsConform, prepareWaitReasons } from './prepareWaitReason'

// ---------------------------------------------------------------------------
// prepareWaitReason.js — miroir client-side, en LECTURE SEULE, du prédicat
// serveur `participantsConform` (#172/B1, engine.go) et de la condition de
// sortie de PREPARE (#172/B2, main.go:1456). Ce fichier ne décide rien —
// couvert ici en isolation pure (pas de rendu), table de règles identique à
// celle vérifiée côté moteur par TestParticipantsConform_Rules (rapport QA
// #172, `_work/reports/qa-20260817-145412.md` §2).
//
// v11.1 (retour QUALIF v11.1.0.5, #219/#236/#237) — `prepareWaitReason`
// (singulier, un seul motif prioritaire) est devenue `prepareWaitReasons`
// (pluriel, TOUS les motifs actifs simultanément, dans un tableau). Les
// tests ci-dessous sont adaptés en conséquence (`.toEqual([...])` au lieu de
// `.toBe(...)`/`.toBeNull()`), et une section dédiée couvre explicitement
// les combinaisons à plusieurs motifs actifs en même temps.
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

describe('prepareWaitReasons — orchestration (phase, buzzers, conformité)', () => {
  const readyTeam = (name) => ({ name, READY: true })
  const notReadyTeam = (name) => ({ name, READY: false })

  it('hors PREPARE → toujours [], quelle que soit la situation', () => {
    ;['NEW_GAME', 'ENROLL', 'READY', 'COUNTDOWN', 'STARTED', 'PAUSED', 'STOPPED', 'REVEALED'].forEach((phase) => {
      expect(
        prepareWaitReasons(phase, { TYPE: 'MEMORY' }, [notReadyTeam('A')], { MEMORY_PARTICIPATING_TEAMS: [] })
      ).toEqual([])
    })
  })

  it('un buzzer actif non prêt sur 2 → ["Buzzers en attente : 1/2"] (libellé complet, compte réel — retour QUALIF v11.1.0.6)', () => {
    // Sélection déjà conforme (MEMORY SOLO, 1 équipe) : seul le motif
    // buzzers est actif ici, seul dans le tableau.
    const reasons = prepareWaitReasons(
      'PREPARE',
      { TYPE: 'MEMORY' },
      [readyTeam('A'), notReadyTeam('B')],
      { MEMORY_PARTICIPATING_TEAMS: ['A'] }
    )
    expect(reasons).toEqual(['Buzzers en attente : 1/2'])
  })

  it('aucun buzzer prêt sur 1 → ["buzzers 0/1"] (libellé court, opts.short)', () => {
    // Sélection déjà conforme (MEMORY SOLO, 1 équipe) : isole la SEULE
    // branche buzzers (le cas "plusieurs motifs à la fois" est couvert
    // explicitement plus bas, section dédiée).
    const reasons = prepareWaitReasons(
      'PREPARE',
      { TYPE: 'MEMORY' },
      [notReadyTeam('A')],
      { MEMORY_PARTICIPATING_TEAMS: ['A'] },
      { short: true }
    )
    expect(reasons).toEqual(['buzzers 0/1'])
  })

  describe('buzzers tous prêts, sélection non conforme → motif mode-spécifique', () => {
    it('MEMORY SOLO, aucune équipe sélectionnée → ["sélectionnez une équipe"] (complet) / ["1 équipe"] (court)', () => {
      const activeTeams = [readyTeam('A'), readyTeam('B')]
      const gameState = { MEMORY_PARTICIPATING_TEAMS: [] }
      expect(prepareWaitReasons('PREPARE', { TYPE: 'MEMORY' }, activeTeams, gameState)).toEqual(['sélectionnez une équipe'])
      expect(prepareWaitReasons('PREPARE', { TYPE: 'MEMORY' }, activeTeams, gameState, { short: true })).toEqual(['1 équipe'])
    })

    it('MEMORY multi, une seule équipe sélectionnée → ["sélectionnez au moins deux équipes"] (complet) / ["2 équipes"] (court)', () => {
      const activeTeams = [readyTeam('A'), readyTeam('B')]
      const gameState = { MEMORY_PARTICIPATING_TEAMS: ['A'] }
      const question = { TYPE: 'MEMORY', MEMORY_MODE: 'CHACUN_SON_TOUR' }
      expect(prepareWaitReasons('PREPARE', question, activeTeams, gameState)).toEqual(['sélectionnez au moins deux équipes'])
      expect(prepareWaitReasons('PREPARE', question, activeTeams, gameState, { short: true })).toEqual(['2 équipes'])
    })

    it('MEMOTION SOLO (défaut), aucune équipe sélectionnée → ["sélectionnez une équipe"] (complet) / ["1 équipe"] (court) — #201 suivi, avant #201 "sélectionnez au moins une équipe"', () => {
      const activeTeams = [readyTeam('A')]
      const gameState = { MEMOTION_PARTICIPATING_TEAMS: [] }
      const question = { TYPE: 'MEMOTION' }
      expect(prepareWaitReasons('PREPARE', question, activeTeams, gameState)).toEqual(['sélectionnez une équipe'])
      expect(prepareWaitReasons('PREPARE', question, activeTeams, gameState, { short: true })).toEqual(['1 équipe'])
    })

    it('MEMOTION multi (CHACUN_SON_TOUR), une seule équipe sélectionnée → ["sélectionnez au moins deux équipes"] (complet) / ["2 équipes"] (court) — #201 suivi, avant #201 une seule équipe suffisait', () => {
      const activeTeams = [readyTeam('A'), readyTeam('B')]
      const gameState = { MEMOTION_PARTICIPATING_TEAMS: ['A'] }
      const question = { TYPE: 'MEMOTION', MOTION_MODE: 'CHACUN_SON_TOUR' }
      expect(prepareWaitReasons('PREPARE', question, activeTeams, gameState)).toEqual(['sélectionnez au moins deux équipes'])
      expect(prepareWaitReasons('PREPARE', question, activeTeams, gameState, { short: true })).toEqual(['2 équipes'])
    })

    it('RAFALE SOLO, aucune équipe sélectionnée → ["sélectionnez une équipe"] (complet) / ["1 équipe"] (court) — #201, avant #201 toujours conforme donc []', () => {
      const activeTeams = [readyTeam('A'), readyTeam('B')]
      const gameState = { RAFALE_PARTICIPATING_TEAMS: [] }
      const question = { TYPE: 'RAFALE' }
      expect(prepareWaitReasons('PREPARE', question, activeTeams, gameState)).toEqual(['sélectionnez une équipe'])
      expect(prepareWaitReasons('PREPARE', question, activeTeams, gameState, { short: true })).toEqual(['1 équipe'])
    })

    it('RAFALE multi (CHACUN_SON_TOUR), une seule équipe sélectionnée → ["sélectionnez au moins deux équipes participantes"] (complet) / ["2 équipes"] (court) — #201, avant #201 une seule équipe suffisait', () => {
      const activeTeams = [readyTeam('A'), readyTeam('B')]
      const gameState = { RAFALE_PARTICIPATING_TEAMS: ['A'] }
      const question = { TYPE: 'RAFALE', RAFALE_MODE: 'CHACUN_SON_TOUR' }
      expect(prepareWaitReasons('PREPARE', question, activeTeams, gameState)).toEqual(['sélectionnez au moins deux équipes participantes'])
      expect(prepareWaitReasons('PREPARE', question, activeTeams, gameState, { short: true })).toEqual(['2 équipes'])
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
      expect(prepareWaitReasons('PREPARE', { TYPE: 'RAFALE', RAFALE_MODE: 'MAILLON_FAIBLE' }, activeTeams, gameState)).toEqual([
        'sélectionnez au moins deux équipes participantes',
      ])
    })

    it('MEMOTION lit MEMOTION_PARTICIPATING_TEAMS, pas MEMORY_PARTICIPATING_TEAMS', () => {
      // Piège : si le mauvais champ était lu, cette sélection MEMORY à 3
      // équipes masquerait à tort le motif MEMOTION (liste vide).
      const activeTeams = [readyTeam('A')]
      const gameState = {
        MEMORY_PARTICIPATING_TEAMS: ['A', 'B', 'C'],
        MEMOTION_PARTICIPATING_TEAMS: [],
      }
      expect(prepareWaitReasons('PREPARE', { TYPE: 'MEMOTION' }, activeTeams, gameState)).toEqual(['sélectionnez une équipe'])
    })
  })

  it('buzzers tous prêts et sélection conforme → [] (rien à expliquer)', () => {
    const activeTeams = [readyTeam('A'), readyTeam('B')]
    const gameState = { MEMORY_PARTICIPATING_TEAMS: ['A'] }
    expect(prepareWaitReasons('PREPARE', { TYPE: 'MEMORY' }, activeTeams, gameState)).toEqual([])
  })

  it('SPEEDY/QCM/ARDOISE en PREPARE, buzzers prêts → toujours [] (non-régression, aucun changement de comportement #172)', () => {
    ;['SPEEDY', 'QCM', 'ARDOISE'].forEach((type) => {
      const reasons = prepareWaitReasons(
        'PREPARE',
        { TYPE: type },
        [readyTeam('A')],
        { MEMORY_PARTICIPATING_TEAMS: [], MEMOTION_PARTICIPATING_TEAMS: [] }
      )
      expect(reasons).toEqual([])
    })
  })

  it('activeTeams vide (aucune équipe active) → pas de "Buzzers en attente", retombe sur la conformité', () => {
    const reasons = prepareWaitReasons('PREPARE', { TYPE: 'MEMORY' }, [], { MEMORY_PARTICIPATING_TEAMS: [] })
    expect(reasons).toEqual(['sélectionnez une équipe'])
  })

  it('question null en PREPARE → permissif, []', () => {
    expect(prepareWaitReasons('PREPARE', null, [readyTeam('A')], {})).toEqual([])
  })
})

// ---------------------------------------------------------------------------
// Addendum média indisponible (v11.1, #219/#236/#237, plan
// _work/reports/plan-20260923-101500.md rév. 8 §6 tâche 13, contrat
// sound.md §10.8) — la branche `GAME.QUESTION_SOUND_UNAVAILABLE`, dernière
// branche évaluée par `prepareWaitReasons`.
// ---------------------------------------------------------------------------

describe('prepareWaitReasons — addendum média indisponible (v11.1, #219/#236/#237)', () => {
  const readyTeam = (name) => ({ name, READY: true })
  // Buzzers déjà prêts et participants conformes (question SPEEDY, jamais
  // de garde participants) — pour isoler la SEULE branche son.
  const activeTeams = [readyTeam('A')]
  const question = { TYPE: 'SPEEDY' }

  it.each([
    ['DISABLED', 'son désactivé', 'Son désactivé (redémarrage requis)'],
    ['OUTPUT', 'pas de sortie audio', 'Sortie audio indisponible (redémarrage requis)'],
    ['FILE', 'son introuvable', 'Fichier son illisible'],
  ])('QUESTION_SOUND_UNAVAILABLE=%s → libellé court=%j, long=%j', (unavailable, wantShort, wantLong) => {
    const gameState = { QUESTION_SOUND_UNAVAILABLE: unavailable }
    expect(prepareWaitReasons('PREPARE', question, activeTeams, gameState)).toEqual([wantLong])
    expect(prepareWaitReasons('PREPARE', question, activeTeams, gameState, { short: true })).toEqual([wantShort])
  })

  // Retour QUALIF v11.1.0.6 — les phrases longues d'origine ("Les sons sont
  // désactivés dans la configuration. Les réactiver demande un redémarrage
  // du serveur.") ont été raccourcies : le remède reste référencé, mais
  // brièvement ("(redémarrage requis)"), plus de phrase complète.
  it('chaque motif long référence le remède réel BRIÈVEMENT (jamais une étiquette générique, jamais une phrase complète)', () => {
    const gameState = { QUESTION_SOUND_UNAVAILABLE: 'DISABLED' }
    expect(prepareWaitReasons('PREPARE', question, activeTeams, gameState)[0]).toMatch(/redémarrage requis/)

    expect(
      prepareWaitReasons('PREPARE', question, activeTeams, { QUESTION_SOUND_UNAVAILABLE: 'OUTPUT' })[0]
    ).toMatch(/redémarrage requis/)
  })

  it('QUESTION_SOUND_UNAVAILABLE="" (disponible) → [], rien à expliquer', () => {
    expect(prepareWaitReasons('PREPARE', question, activeTeams, { QUESTION_SOUND_UNAVAILABLE: '' })).toEqual([])
  })

  it('QUESTION_SOUND_UNAVAILABLE absent (clé manquante) → [], même comportement que ""', () => {
    expect(prepareWaitReasons('PREPARE', question, activeTeams, {})).toEqual([])
  })

  it('motif inconnu (valeur future non prévue par la table) → [], permissif par défaut (jamais de crash)', () => {
    expect(
      prepareWaitReasons('PREPARE', question, activeTeams, { QUESTION_SOUND_UNAVAILABLE: 'UNE_VALEUR_FUTURE_INCONNUE' })
    ).toEqual([])
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
    expect(prepareWaitReasons('PREPARE', { TYPE: 'SPEEDY' }, activeTeams, gameState)).not.toEqual([])
  })

  it('SPEEDY/QCM/ARDOISE en PREPARE, son disponible → toujours [] (non-régression, complète le test existant "toujours []")', () => {
    ;['SPEEDY', 'QCM', 'ARDOISE'].forEach((type) => {
      const reasons = prepareWaitReasons(
        'PREPARE',
        { TYPE: type },
        activeTeams,
        { MEMORY_PARTICIPATING_TEAMS: [], MEMOTION_PARTICIPATING_TEAMS: [], QUESTION_SOUND_UNAVAILABLE: '' }
      )
      expect(reasons).toEqual([])
    })
  })
})

// ---------------------------------------------------------------------------
// Évolution UX (retour QUALIF v11.1.0.5, #219/#236/#237) — plusieurs motifs
// actifs EN MÊME TEMPS. Avant cette évolution, `prepareWaitReason`
// (singulier) ne retournait QUE le motif de plus haute priorité (buzzers >
// participants > son) : un utilisateur avec deux problèmes simultanés ne
// voyait que le premier, et découvrait le second seulement après avoir
// corrigé le premier. `prepareWaitReasons` accumule désormais TOUS les
// motifs actifs, dans l'ordre buzzers → participants → son.
//
// v11.1.0.6 — présentation revue une seconde fois : un motif par LIGNE
// (chaque appelant empile les éléments du tableau, jamais un `.join(' · ')`
// sur une seule ligne) et libellés plus concis (voir la table des motifs
// son ci-dessus, et le compte réel "x/y" pour les buzzers).
// ---------------------------------------------------------------------------

describe('prepareWaitReasons — plusieurs motifs actifs simultanément (retour QUALIF v11.1.0.5/.6)', () => {
  const readyTeam = (name) => ({ name, READY: true })
  const notReadyTeam = (name) => ({ name, READY: false })

  it('buzzers non prêts ET son indisponible en même temps (SPEEDY) → les DEUX motifs, dans cet ordre', () => {
    const reasons = prepareWaitReasons(
      'PREPARE',
      { TYPE: 'SPEEDY' },
      [notReadyTeam('A')],
      { QUESTION_SOUND_UNAVAILABLE: 'FILE' }
    )
    expect(reasons).toEqual(['Buzzers en attente : 0/1', 'Fichier son illisible'])
  })

  it('même situation, libellés courts (opts.short) → ["buzzers 0/1", "son introuvable"]', () => {
    const reasons = prepareWaitReasons(
      'PREPARE',
      { TYPE: 'SPEEDY' },
      [notReadyTeam('A')],
      { QUESTION_SOUND_UNAVAILABLE: 'FILE' },
      { short: true }
    )
    expect(reasons).toEqual(['buzzers 0/1', 'son introuvable'])
  })

  it('participants non conformes ET son indisponible en même temps (MEMORY) → les DEUX motifs, participants avant son', () => {
    // Buzzers déjà prêts pour isoler participants+son (pas de 3e motif).
    const reasons = prepareWaitReasons(
      'PREPARE',
      { TYPE: 'MEMORY' },
      [readyTeam('A')],
      { MEMORY_PARTICIPATING_TEAMS: [], QUESTION_SOUND_UNAVAILABLE: 'DISABLED' }
    )
    expect(reasons).toEqual(['sélectionnez une équipe', 'Son désactivé (redémarrage requis)'])
  })

  it('les TROIS motifs actifs en même temps (buzzers, participants, son) → les trois, dans l\'ordre normatif', () => {
    const reasons = prepareWaitReasons(
      'PREPARE',
      { TYPE: 'MEMORY' },
      [notReadyTeam('A')],
      { MEMORY_PARTICIPATING_TEAMS: [], QUESTION_SOUND_UNAVAILABLE: 'OUTPUT' }
    )
    expect(reasons).toEqual([
      'Buzzers en attente : 0/1',
      'sélectionnez une équipe',
      'Sortie audio indisponible (redémarrage requis)',
    ])
  })

  it('un seul motif actif (les autres résolus) → tableau à un seul élément (un appelant multi-ligne n\'affiche qu\'une ligne)', () => {
    const reasons = prepareWaitReasons(
      'PREPARE',
      { TYPE: 'MEMORY' },
      [readyTeam('A')],
      { MEMORY_PARTICIPATING_TEAMS: ['A'], QUESTION_SOUND_UNAVAILABLE: 'FILE' }
    )
    expect(reasons).toEqual(['Fichier son illisible'])
  })

  it('aucun motif actif → tableau vide (les appelants ne rendent alors aucune ligne)', () => {
    const reasons = prepareWaitReasons(
      'PREPARE',
      { TYPE: 'SPEEDY' },
      [readyTeam('A')],
      { QUESTION_SOUND_UNAVAILABLE: '' }
    )
    expect(reasons).toEqual([])
  })
})
