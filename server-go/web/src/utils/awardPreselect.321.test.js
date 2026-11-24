import { describe, it, expect } from 'vitest'
import { preselectAwardKeys } from './awardPreselect'
import { fold } from './fold'

// #321 (v13.1.0, LOT-1) — contrat contracts/award-preselect.md §1-§3 : table de TOUS les types de la §2,
// cas limites (déjà créditée, montant ≤ 0, aucun participant, niveau joueur/équipe, équipe supprimée), repli ARDOISE.
// Signature du contrat : preselectAwardKeys({ question, gameState, bumpers, awardLevel, awarded, amountOf }) → string[]
// `awarded` = tableau de clés déjà créditées ; `amountOf` = (clé) → montant par défaut.

const BUMPERS = {
  m1: { NAME: 'J1', TEAM: 'Bleus', TIME: 3000, ANSWER_COLOR: 'GREEN' },
  m2: { NAME: 'J2', TEAM: 'Bleus', TIME: 0, ANSWER_COLOR: '' },
  m3: { NAME: 'J3', TEAM: 'Rouges', TIME: 1500, ANSWER_COLOR: 'RED' },
  m4: { NAME: 'J4', TEAM: 'Verts', TIME: 2000, ANSWER_COLOR: 'GREEN' },
}
const call = (over = {}) => preselectAwardKeys({
  question: { TYPE: 'QCM' }, gameState: {}, bumpers: BUMPERS, awardLevel: 'TEAM', awarded: [], amountOf: () => 5, ...over,
})
const sorted = (a) => [...a].sort()

describe('preselectAwardKeys — table par type (§2)', () => {
  it('PICTO : l’équipe qui dessine si PICTO_OUTCOME = FOUND', () => {
    const q = { TYPE: 'PICTO' }
    expect(call({ question: q, gameState: { PICTO_TEAM: 'Bleus', PICTO_OUTCOME: 'FOUND' } })).toEqual(['Bleus'])
  })
  it.each(['', 'TIMEOUT', 'STOPPED', 'NO_WORD'])('PICTO : rien si PICTO_OUTCOME = "%s"', (o) => {
    expect(call({ question: { TYPE: 'PICTO' }, gameState: { PICTO_TEAM: 'Bleus', PICTO_OUTCOME: o } })).toEqual([])
  })
  it('PICTO : rien si aucune équipe choisie', () => {
    expect(call({ question: { TYPE: 'PICTO' }, gameState: { PICTO_TEAM: '', PICTO_OUTCOME: 'FOUND' } })).toEqual([])
  })
  it('RAFALE_PICTO : PICTO_TEAM si PICTO_ROUND_POINTS > 0', () => {
    expect(call({ question: { TYPE: 'RAFALE_PICTO' }, gameState: { PICTO_TEAM: 'Rouges', PICTO_ROUND_POINTS: 4 } })).toEqual(['Rouges'])
  })
  it.each([0, -1, undefined])('RAFALE_PICTO : rien si PICTO_ROUND_POINTS = %s', (p) => {
    expect(call({ question: { TYPE: 'RAFALE_PICTO' }, gameState: { PICTO_TEAM: 'Rouges', PICTO_ROUND_POINTS: p } })).toEqual([])
  })

  it('MEMORY SOLO : l’équipe participante', () => {
    const q = { TYPE: 'MEMORY', MEMORY_MODE: 'SOLO' }
    expect(call({ question: q, gameState: { MEMORY_PARTICIPATING_TEAMS: ['Verts'] } })).toEqual(['Verts'])
  })
  it('MEMORY multi-équipes : seulement les équipes de score > 0', () => {
    const q = { TYPE: 'MEMORY', MEMORY_MODE: 'TEAMS', MEMORY_PAIRS: [{}, {}, {}], MEMORY_CONFIG: { POINTS_PER_PAIR: 10 } }
    const gs = { MEMORY_PARTICIPATING_TEAMS: ['Bleus', 'Rouges', 'Verts'], MEMORY_TEAM_PAIRS: { Bleus: 2, Rouges: 0, Verts: 1 }, MEMORY_TEAM_ERRORS: { Verts: 5 } }
    // Verts : 1 paire (10) − 5 erreurs × pénalité (0 par défaut) = 10 > 0 ; Rouges : 0
    expect(sorted(call({ question: q, gameState: gs }))).toEqual(['Bleus', 'Verts'])
  })
  it('MEMORY multi-équipes : score ramené à 0 par les erreurs → pas coché', () => {
    const q = { TYPE: 'MEMORY', MEMORY_MODE: 'TEAMS', MEMORY_PAIRS: [{}, {}], MEMORY_CONFIG: { POINTS_PER_PAIR: 10, ERROR_PENALTY: 10 } }
    const gs = { MEMORY_PARTICIPATING_TEAMS: ['Bleus'], MEMORY_TEAM_PAIRS: { Bleus: 1 }, MEMORY_TEAM_ERRORS: { Bleus: 3 } }
    expect(call({ question: q, gameState: gs })).toEqual([])
  })

  it('RAFALE SOLO : l’équipe participante si son compteur retenu > 0', () => {
    const q = { TYPE: 'RAFALE', RAFALE_MODE: 'SOLO' }
    expect(call({ question: q, gameState: { RAFALE_PARTICIPATING_TEAMS: ['Bleus'], RAFALE_TEAM_COUNTERS: { Bleus: 3 } } })).toEqual(['Bleus'])
    expect(call({ question: q, gameState: { RAFALE_PARTICIPATING_TEAMS: ['Bleus'], RAFALE_TEAM_COUNTERS: { Bleus: 0 } } })).toEqual([])
  })
  it('RAFALE multi-équipes : compteurs > 0 seulement', () => {
    const q = { TYPE: 'RAFALE', RAFALE_MODE: 'COURSE' }
    const gs = { RAFALE_PARTICIPATING_TEAMS: ['Bleus', 'Rouges', 'Verts'], RAFALE_TEAM_COUNTERS: { Bleus: 2, Rouges: 0, Verts: 1 } }
    expect(sorted(call({ question: q, gameState: gs }))).toEqual(['Bleus', 'Verts'])
  })
  it('RAFALE MAILLON_FAIBLE : le compteur retenu est RAFALE_TEAM_BEST (pas le compteur courant retombé à 0)', () => {
    const q = { TYPE: 'RAFALE', RAFALE_MODE: 'MAILLON_FAIBLE' }
    const gs = { RAFALE_PARTICIPATING_TEAMS: ['Bleus', 'Rouges'], RAFALE_TEAM_COUNTERS: { Bleus: 0, Rouges: 0 }, RAFALE_TEAM_BEST: { Bleus: 4, Rouges: 0 } }
    expect(call({ question: q, gameState: gs })).toEqual(['Bleus'])
  })

  it('QCM niveau équipe : les équipes ayant donné QCM_CORRECT', () => {
    const q = { TYPE: 'QCM', QCM_CORRECT: 'GREEN' }
    expect(sorted(call({ question: q }))).toEqual(['Bleus', 'Verts'])
  })
  it('QCM niveau joueur : les joueurs ayant donné QCM_CORRECT (clé = MAC)', () => {
    const q = { TYPE: 'QCM', QCM_CORRECT: 'GREEN' }
    expect(sorted(call({ question: q, awardLevel: 'PLAYER' }))).toEqual(['m1', 'm4'])
  })
  it('QCM : aucune bonne réponse donnée → rien', () => {
    expect(call({ question: { TYPE: 'QCM', QCM_CORRECT: 'BLUE' } })).toEqual([])
  })
  it('QCM : couleur assignée sans réponse (TIME = 0) ≠ réponse (#255)', () => {
    const b = { m1: { NAME: 'J1', TEAM: 'Bleus', TIME: 0, ANSWER_COLOR: 'GREEN' } }
    expect(call({ question: { TYPE: 'QCM', QCM_CORRECT: 'GREEN' }, bumpers: b })).toEqual([])
  })

  it('SPEEDY niveau joueur : le bumper de plus petit TIME > 0', () => {
    expect(call({ question: { TYPE: 'SPEEDY' }, awardLevel: 'PLAYER' })).toEqual(['m3'])
  })
  it('SPEEDY niveau équipe : l’équipe du premier buzzeur', () => {
    expect(call({ question: { TYPE: 'SPEEDY' }, awardLevel: 'TEAM' })).toEqual(['Rouges'])
  })
  it('SPEEDY : aucun buzz → rien ; TIME = 0 ignoré', () => {
    const b = { m1: { NAME: 'J1', TEAM: 'Bleus', TIME: 0 }, m2: { NAME: 'J2', TEAM: 'Rouges', TIME: 0 } }
    expect(call({ question: { TYPE: 'SPEEDY' }, bumpers: b })).toEqual([])
  })

  it('ARDOISE : équipes dont la réponse repliée = ANSWER repliée', () => {
    const q = { TYPE: 'ARDOISE', ANSWER: 'Éléphant' }
    const gs = { ARDOISE_ANSWERS: { Bleus: { TEXT: 'elephant' }, Rouges: { TEXT: ' ÉLÉPHANT ' }, Verts: { TEXT: 'girafe' } } }
    expect(sorted(call({ question: q, gameState: gs }))).toEqual(['Bleus', 'Rouges'])
  })
  it('ARDOISE : « Le Louvre » ≠ « Louvre »', () => {
    const q = { TYPE: 'ARDOISE', ANSWER: 'Louvre' }
    expect(call({ question: q, gameState: { ARDOISE_ANSWERS: { Bleus: { TEXT: 'Le Louvre' } } } })).toEqual([])
  })
  it('ARDOISE : ANSWER vide → rien (même avec une réponse vide)', () => {
    expect(call({ question: { TYPE: 'ARDOISE', ANSWER: '' }, gameState: { ARDOISE_ANSWERS: { Bleus: { TEXT: '' } } } })).toEqual([])
  })

  it.each(['ROUE', 'ENTRACTE'])('%s : rien (crédit automatique / aucun point)', (type) => {
    expect(call({ question: { TYPE: type }, gameState: { PICTO_TEAM: 'Bleus', PICTO_OUTCOME: 'FOUND' } })).toEqual([])
  })
  it('question absente ou type inconnu : rien, sans exception', () => {
    expect(call({ question: null })).toEqual([])
    expect(call({ question: { TYPE: 'INCONNU' } })).toEqual([])
  })
})

describe('preselectAwardKeys — règles communes et cas limites (§1)', () => {
  const picto = { TYPE: 'PICTO' }
  const gs = { PICTO_TEAM: 'Bleus', PICTO_OUTCOME: 'FOUND' }
  it('jamais une unité déjà créditée', () => {
    expect(call({ question: picto, gameState: gs, awarded: ['Bleus'] })).toEqual([])
  })
  it('jamais une unité dont le montant par défaut est nul', () => {
    expect(call({ question: picto, gameState: gs, amountOf: () => 0 })).toEqual([])
  })
  it('jamais une unité dont le montant par défaut est une pénalité (< 0)', () => {
    expect(call({ question: picto, gameState: gs, amountOf: () => -1 })).toEqual([])
  })
  it('le montant est évalué par unité : seules les unités à montant > 0 restent', () => {
    const q = { TYPE: 'QCM', QCM_CORRECT: 'GREEN' }
    expect(call({ question: q, amountOf: (k) => (k === 'Verts' ? 0 : 5) })).toEqual(['Bleus'])
  })
  it('SPEEDY : le premier buzzeur déjà crédité → rien (pas de report sur le suivant)', () => {
    expect(call({ question: { TYPE: 'SPEEDY' }, awarded: ['Rouges'] })).toEqual([])
  })
  it('aucun participant → rien', () => {
    expect(call({ question: { TYPE: 'RAFALE', RAFALE_MODE: 'COURSE' }, gameState: { RAFALE_PARTICIPATING_TEAMS: [], RAFALE_TEAM_COUNTERS: {} } })).toEqual([])
    expect(call({ question: { TYPE: 'MEMORY', MEMORY_MODE: 'SOLO' }, gameState: { MEMORY_PARTICIPATING_TEAMS: [] } })).toEqual([])
  })
  it('bumpers vides → rien (QCM, SPEEDY)', () => {
    expect(call({ question: { TYPE: 'SPEEDY' }, bumpers: {} })).toEqual([])
    expect(call({ question: { TYPE: 'QCM', QCM_CORRECT: 'GREEN' }, bumpers: {} })).toEqual([])
  })
  it('équipe supprimée : une équipe absente des bumpers ne produit pas d’exception et pas de clé fantôme joueur', () => {
    const b = { m1: { NAME: 'J1', TEAM: 'Supprimée', TIME: 100, ANSWER_COLOR: 'GREEN' } }
    expect(() => call({ question: { TYPE: 'SPEEDY' }, bumpers: b, awardLevel: 'PLAYER' })).not.toThrow()
  })
  it('QCM : équipe créditée au niveau équipe → ses joueurs ne sont pas cochés au niveau joueur', () => {
    const q = { TYPE: 'QCM', QCM_CORRECT: 'GREEN' }
    expect(sorted(call({ question: q, awardLevel: 'PLAYER', awarded: ['m1'] }))).toEqual(['m4'])
  })
  it('fonction pure : n’altère pas ses entrées', () => {
    const b = JSON.stringify(BUMPERS)
    call({ question: { TYPE: 'SPEEDY' } })
    expect(JSON.stringify(BUMPERS)).toBe(b)
  })
})

describe('fold — repli de comparaison ARDOISE (§3)', () => {
  it('fold_accents', () => { expect(fold('Éléphant')).toBe('elephant'); expect(fold('çà et là')).toBe('ca et la') })
  it('fold_case', () => { expect(fold('ÉLÉPHANT')).toBe(fold('elephant')); expect(fold('PaRiS')).toBe('paris') })
  it('fold_spaces', () => { expect(fold('  Le   Louvre  ')).toBe('le louvre'); expect(fold('Le Louvre')).not.toBe(fold('Louvre')) })
  it('fold_empty', () => { expect(fold('')).toBe(''); expect(fold('   ')).toBe(''); expect(fold(undefined)).toBe(''); expect(fold(null)).toBe('') })
})

describe('preselectAwardKeys — niveau JOUEUR sans knownKeys (m2)', () => {
  it.each(['PICTO', 'RAFALE_PICTO', 'MEMORY', 'RAFALE'])('%s : aucune clé d’équipe renvoyée au niveau joueur', (TYPE) => {
    const keys = preselectAwardKeys({
      question: { TYPE, MEMORY_MODE: 'SOLO' },
      gameState: { PICTO_OUTCOME: 'FOUND', PICTO_TEAM: 'Bleus', PICTO_ROUND_POINTS: 2, MEMORY_PARTICIPATING_TEAMS: ['Bleus'], RAFALE_PARTICIPATING_TEAMS: ['Bleus'], RAFALE_TEAM_COUNTERS: { Bleus: 3 } },
      bumpers: {}, awardLevel: 'PLAYER', awarded: [], amountOf: () => 5,
    })
    expect(keys).toEqual([])
  })
})
