import { describe, it, expect } from 'vitest'
import { motionGestures } from './motionRules'

// ---------------------------------------------------------------------------
// motionRules — #160/F3, T2. Source UNIQUE des gestes de L2 en MEMOTION, sur
// le modèle philosophique de phaseRules.js : le composant consommateur
// (AnimMotionActions, #160/F6) ne réécrit JAMAIS une condition localement,
// il rend ce que la règle lui donne. Matrice dérivée de la maquette
// docs/mockups/anim-memotion-160.html §"Ce que la maquette engage" et de
// chaque section "Gestes disponibles" (une par sous-phase).
//
// Contrat de `motionGestures(subphase, ctx)` — DÉFINI ICI en TDD (le plan
// fixe la signature et les cinq sous-phases, pas le détail des champs) :
//   ctx = { timerRunning, currentTeam, currentTeamColor, selectedCardId, cardPoints }
//   -> [{ key, label, subLabel, state, action, payload }]
//
// `action` est le NOM D'ACTION WEBSOCKET LITTÉRAL (contracts/websocket-actions.md,
// §"Animateur (Interface conduite)" + les `.emits` de la maquette) plutôt
// qu'un identifiant interne inventé — c'est la forme la moins ambiguë pour
// qu'AnimMotionActions.jsx (#160/F6, T5) sache quel émetteur useWebSocket.js
// (#160/F2 : flipMotionCard/stopMotionTimer/revealMotionCard/doneMotionCard)
// appeler, et elle documente elle-même sa correspondance au contrat.
// `payload` est le MSG exact tel que décrit par la maquette (ex. MEMOTION_DONE
// { CARD_ID, WINNER_TEAM }) — CARD_ID vient TOUJOURS de `selectedCardId`, y
// compris pour "annuler"/"sans vainqueur" (R7 du plan : le moteur se fie à
// sa propre sélection, mais CARD_ID doit être renseigné).
// ---------------------------------------------------------------------------

function baseCtx(overrides = {}) {
  return {
    timerRunning: false,
    currentTeam: 'Les Bleus',
    currentTeamColor: [37, 99, 235],
    selectedCardId: 'c7',
    cardPoints: 3,
    ...overrides,
  }
}

function byKey(gestures, key) {
  return gestures.find(g => g.key === key)
}

describe('motionGestures — MEMORIZE : aucun geste (état d\'attente, bascule automatique du moteur)', () => {
  it('retourne un tableau vide, quel que soit le contexte', () => {
    expect(motionGestures('MEMORIZE', baseCtx())).toEqual([])
    expect(motionGestures('MEMORIZE', baseCtx({ timerRunning: true }))).toEqual([])
  })
})

describe('motionGestures — GRID : aucun geste (le geste est la carte elle-même, rendue par AnimMotionGrid en L3)', () => {
  it('retourne un tableau vide, quel que soit le contexte', () => {
    expect(motionGestures('GRID', baseCtx())).toEqual([])
  })
})

describe('motionGestures — SELECTED : DÉMARRER (go) · ANNULER (optional)', () => {
  const gestures = motionGestures('SELECTED', baseCtx())

  it('exactement deux gestes, dans cet ordre', () => {
    expect(gestures).toHaveLength(2)
    expect(gestures.map(g => g.key)).toEqual(['start', 'cancel'])
  })

  it('DÉMARRER : state "go", émet MEMOTION_FLIP {} (retourne la carte, démarre le chrono)', () => {
    const g = byKey(gestures, 'start')
    expect(g.label).toBe('DÉMARRER')
    expect(g.state).toBe('go')
    expect(g.action).toBe('MEMOTION_FLIP')
    expect(g.payload).toEqual({})
  })

  it('ANNULER : state "optional", émet MEMOTION_DONE { CARD_ID: selectedCardId, WINNER_TEAM: "" }', () => {
    const g = byKey(gestures, 'cancel')
    expect(g.label).toBe('ANNULER')
    expect(g.state).toBe('optional')
    expect(g.action).toBe('MEMOTION_DONE')
    expect(g.payload).toEqual({ CARD_ID: 'c7', WINNER_TEAM: '' })
  })

  it('ANNULER porte CARD_ID = selectedCardId même si vide (le moteur se fie à sa propre sélection, R7 du plan)', () => {
    const g = byKey(motionGestures('SELECTED', baseCtx({ selectedCardId: '' })), 'cancel')
    expect(g.payload).toEqual({ CARD_ID: '', WINNER_TEAM: '' })
  })
})

describe('motionGestures — QUESTION : STOP CHRONO · RÉVÉLER · SANS VAINQUEUR', () => {
  it('exactement trois gestes, dans cet ordre, quel que soit l\'état du chrono', () => {
    const running = motionGestures('QUESTION', baseCtx({ timerRunning: true }))
    const stopped = motionGestures('QUESTION', baseCtx({ timerRunning: false }))
    expect(running.map(g => g.key)).toEqual(['stopTimer', 'reveal', 'noWinner'])
    expect(stopped.map(g => g.key)).toEqual(['stopTimer', 'reveal', 'noWinner'])
  })

  it('chrono EN COURS : STOP CHRONO "optional", RÉVÉLER "off", SANS VAINQUEUR "optional"', () => {
    const gestures = motionGestures('QUESTION', baseCtx({ timerRunning: true }))
    expect(byKey(gestures, 'stopTimer').state).toBe('optional')
    expect(byKey(gestures, 'reveal').state).toBe('off')
    expect(byKey(gestures, 'noWinner').state).toBe('optional')
  })

  it('chrono À ZÉRO : STOP CHRONO "off" (rien à arrêter), RÉVÉLER devient "go", SANS VAINQUEUR reste "optional"', () => {
    const gestures = motionGestures('QUESTION', baseCtx({ timerRunning: false }))
    expect(byKey(gestures, 'stopTimer').state).toBe('off')
    expect(byKey(gestures, 'reveal').state).toBe('go')
    expect(byKey(gestures, 'noWinner').state).toBe('optional')
  })

  it('STOP CHRONO émet MEMOTION_STOP_TIMER {}', () => {
    const g = byKey(motionGestures('QUESTION', baseCtx({ timerRunning: true })), 'stopTimer')
    expect(g.label).toBe('STOP CHRONO')
    expect(g.action).toBe('MEMOTION_STOP_TIMER')
    expect(g.payload).toEqual({})
  })

  it('RÉVÉLER émet MEMOTION_REVEAL {}', () => {
    const g = byKey(motionGestures('QUESTION', baseCtx({ timerRunning: false })), 'reveal')
    expect(g.label).toBe('RÉVÉLER')
    expect(g.action).toBe('MEMOTION_REVEAL')
    expect(g.payload).toEqual({})
  })

  it('SANS VAINQUEUR émet MEMOTION_DONE { CARD_ID: selectedCardId, WINNER_TEAM: "" } (clôt sans révéler)', () => {
    const g = byKey(motionGestures('QUESTION', baseCtx()), 'noWinner')
    expect(g.label).toBe('SANS VAINQUEUR')
    expect(g.action).toBe('MEMOTION_DONE')
    expect(g.payload).toEqual({ CARD_ID: 'c7', WINNER_TEAM: '' })
  })

  // Le champ `action` d'un geste "off" reste renseigné (donnée pure) — c'est
  // au CONSOMMATEUR (AnimMotionActions, T5) de ne jamais l'invoquer pour un
  // bouton désactivé. motionRules ne détient aucun callback : elle ne peut
  // structurellement "émettre" rien elle-même, elle ne fait QUE décrire.
  it('un geste "off" reste une donnée pure — motionGestures n\'exécute jamais de callback (aucun n\'est passé en ctx)', () => {
    const gestures = motionGestures('QUESTION', baseCtx({ timerRunning: true }))
    const revealOff = byKey(gestures, 'reveal')
    expect(revealOff.state).toBe('off')
    expect(revealOff.action).toBe('MEMOTION_REVEAL') // présent mais jamais déclenché par cette fonction
  })
})

describe('motionGestures — REVEAL : équipe courante (go) · PERSONNE (optional)', () => {
  it('deux gestes quand currentTeam est renseignée, dans cet ordre', () => {
    const gestures = motionGestures('REVEAL', baseCtx())
    expect(gestures.map(g => g.key)).toEqual(['award', 'nobody'])
  })

  it('geste "award" : label = nom de l\'équipe courante, state "go", couleur transmise, sous-libellé "+N pts"', () => {
    const g = byKey(motionGestures('REVEAL', baseCtx({ currentTeam: 'Les Bleus', currentTeamColor: [37, 99, 235], cardPoints: 3 })), 'award')
    expect(g.label).toBe('Les Bleus')
    expect(g.state).toBe('go')
    expect(g.subLabel).toBe('+3 pts')
    expect(g.color).toEqual([37, 99, 235])
  })

  it('geste "award" émet MEMOTION_DONE { CARD_ID: selectedCardId, WINNER_TEAM: currentTeam }', () => {
    const g = byKey(motionGestures('REVEAL', baseCtx({ currentTeam: 'Les Bleus', selectedCardId: 'c9' })), 'award')
    expect(g.action).toBe('MEMOTION_DONE')
    expect(g.payload).toEqual({ CARD_ID: 'c9', WINNER_TEAM: 'Les Bleus' })
  })

  it('PERSONNE : label "PERSONNE", state "optional", sous-libellé "0 pt", émet MEMOTION_DONE WINNER_TEAM=""', () => {
    const g = byKey(motionGestures('REVEAL', baseCtx({ selectedCardId: 'c9' })), 'nobody')
    expect(g.label).toBe('PERSONNE')
    expect(g.state).toBe('optional')
    expect(g.subLabel).toBe('0 pt')
    expect(g.action).toBe('MEMOTION_DONE')
    expect(g.payload).toEqual({ CARD_ID: 'c9', WINNER_TEAM: '' })
  })

  // Règle du plan (§F3, dernier point) : mode SOLO, aucune équipe courante
  // -> aucun bouton d'attribution n'a de sens (il n'y a personne à qui
  // attribuer), SEUL "PERSONNE" reste proposé. Pas de bouton "générique"
  // pour une équipe absente.
  it('currentTeam vide (mode SOLO) : SEUL le geste "PERSONNE" est retourné', () => {
    const gestures = motionGestures('REVEAL', baseCtx({ currentTeam: '' }))
    expect(gestures).toHaveLength(1)
    expect(gestures[0].key).toBe('nobody')
    expect(gestures[0].label).toBe('PERSONNE')
  })

  it('currentTeam undefined (jamais renseignée) : même repli que currentTeam vide', () => {
    const gestures = motionGestures('REVEAL', baseCtx({ currentTeam: undefined }))
    expect(gestures).toHaveLength(1)
    expect(gestures[0].key).toBe('nobody')
  })

  // Aucune AUTRE équipe que la courante n'est jamais proposée (c'est la
  // règle du jeu tenue par l'interface, cf. maquette §REVEAL, dernier
  // paragraphe) — même avec un contexte contenant plusieurs équipes
  // possibles côté appelant, motionGestures n'a même pas connaissance
  // d'une liste d'équipes : le contrat ne l'expose pas, structurellement
  // impossible de proposer "une autre équipe".
  it('jamais plus de deux gestes en REVEAL (pas de bouton par équipe du roster)', () => {
    const gestures = motionGestures('REVEAL', baseCtx())
    expect(gestures.length).toBeLessThanOrEqual(2)
  })
})

describe('motionGestures — sous-phase inconnue/absente : repli défensif sur []', () => {
  it('sous-phase vide, null ou non reconnue -> tableau vide, ne plante pas', () => {
    expect(motionGestures('', baseCtx())).toEqual([])
    expect(() => motionGestures(null, baseCtx())).not.toThrow()
    expect(motionGestures(null, baseCtx())).toEqual([])
    expect(motionGestures('UNKNOWN_SUBPHASE', baseCtx())).toEqual([])
  })
})
