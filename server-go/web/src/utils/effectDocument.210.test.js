import { describe, it, expect } from 'vitest'
import {
  setCell, getCell, addStep, duplicateStep, moveStep, removeStep, insertStepBefore,
  addVariable, removeVariable, moveVariable, setVariableKind, nextVariableName, parseItems,
  addGroup, removeGroup, initialNumberedRows, highestNumbered, canAddGroup, starRowUsed, gridRows,
  newEffectDocument, copyDocument, copyId, toggleMoment, sortActions, isCanonical, locateError, sameDocument,
  MAX_VARIABLES, MAX_STEPS,
} from './effectDocument'

// ---------------------------------------------------------------------------
// #210 (v11.8.0, LOT-2, F2.1) — opérations pures sur le document d'effet. Source : contracts/light-effects.md §11.1 (ordre canonique),
// §11.2 (grid.effect), §11.8 (groupes de la grille, variables, pas), §12.1 (limites). Aucune analyse de YAML ni calcul de trame ici.
// ---------------------------------------------------------------------------

const A = (target, color = 'theme', intensity = 100) => ({ target, color, intensity })
const doc = (over = {}) => ({
  id: 'e', name: 'E', description: '', moments: ['chrono'], min_groups: 0, tick_ms: 1000, loop: true,
  variables: [], pre: null, steps: [[A('g0')]], last: null, post: null, ...over,
})
const targets = (list) => list.map(a => a.target)

describe('ordre canonique des actions (§11.1)', () => {
  it('g0, puis g1…g12 par numéro, puis g*, puis les variables dans l\'ordre de variables', () => {
    const vars = [{ name: 'gB' }, { name: 'gA' }]
    const sorted = sortActions([A('gA'), A('g*'), A('g10'), A('g2'), A('gB'), A('g0'), A('g1')], vars)
    expect(targets(sorted)).toEqual(['g0', 'g1', 'g2', 'g10', 'g*', 'gB', 'gA'])
  })

  it('un voisin se range juste avant sa variable, par décalage croissant', () => {
    const sorted = sortActions([A('gA'), A('gA+1'), A('gA-1'), A('g0')], [{ name: 'gA' }])
    expect(targets(sorted)).toEqual(['g0', 'gA-1', 'gA+1', 'gA'])
  })

  it('setCell écrit dans l\'ordre canonique quelle que soit la case cliquée', () => {
    let d = doc({ variables: [{ name: 'gA', kind: 'progression', start: '1', step: '+1' }], steps: [[]] })
    d = setCell(d, 0, 'gA', { color: 'urgence', intensity: 100 })
    d = setCell(d, 0, 'g3', { color: '#FF0000', intensity: 50 })
    d = setCell(d, 0, 'g0', { color: 'theme', intensity: 100 })
    d = setCell(d, 0, 'g*', { color: 'scene', intensity: 80 })
    expect(targets(d.steps[0])).toEqual(['g0', 'g3', 'g*', 'gA'])
    expect(isCanonical(d)).toBe(true)
  })

  it('isCanonical détecte un pas dans un autre ordre (custom_order)', () => {
    expect(isCanonical(doc({ steps: [[A('g2'), A('g1')]] }))).toBe(false)
  })
})

describe('cases : écrire, vider, ne pas modifier l\'original', () => {
  it('une case vide = aucune action ; écrire puis vider rend le pas vide', () => {
    const d0 = doc({ steps: [[]] })
    expect(getCell(d0, 0, 'g1')).toBeUndefined()
    const d1 = setCell(d0, 0, 'g1', { color: '#112233', intensity: 40 })
    expect(getCell(d1, 0, 'g1')).toEqual({ target: 'g1', color: '#112233', intensity: 40 })
    expect(setCell(d1, 0, 'g1', null).steps[0]).toEqual([])
    expect(d0.steps[0]).toEqual([]) // document reçu jamais modifié
  })

  it('« éteint » s\'écrit { target, off: true } sans color ni intensity', () => {
    const d = setCell(doc({ steps: [[]] }), 0, 'g2', { off: true })
    expect(getCell(d, 0, 'g2')).toEqual({ target: 'g2', off: true })
  })

  it('PRE / DERNIER / POST : absents (null) tant qu\'aucune action ; vider la dernière rend le pas absent', () => {
    let d = setCell(doc(), 'pre', 'g0', { color: 'theme', intensity: 100 })
    expect(d.pre).toHaveLength(1)
    d = setCell(d, 'pre', 'g0', null)
    expect(d.pre).toBeNull()
    expect(setCell(doc(), 'last', 'g1', { off: true }).last).toEqual([{ target: 'g1', off: true }])
    expect(setCell(doc(), 'post', 'g1', { off: true }).post).toEqual([{ target: 'g1', off: true }])
  })
})

describe('pas : ajouter, dupliquer, déplacer, supprimer (§11.8)', () => {
  const d3 = doc({ steps: [[A('g1')], [A('g2')], [A('g3')]] })

  it('ajouter un pas à la fin, vide', () => {
    expect(addStep(d3).steps).toHaveLength(4)
    expect(addStep(d3).steps[3]).toEqual([])
  })

  it('dupliquer insère une copie profonde juste après', () => {
    const d = duplicateStep(d3, 1)
    expect(d.steps.map(s => s[0].target)).toEqual(['g1', 'g2', 'g2', 'g3'])
    d.steps[2][0].intensity = 5
    expect(d.steps[1][0].intensity).toBe(100)
  })

  it('déplacer à gauche / à droite ; sans effet aux bords', () => {
    expect(moveStep(d3, 1, -1).steps.map(s => s[0].target)).toEqual(['g2', 'g1', 'g3'])
    expect(moveStep(d3, 1, +1).steps.map(s => s[0].target)).toEqual(['g1', 'g3', 'g2'])
    expect(moveStep(d3, 0, -1)).toBe(d3)
    expect(moveStep(d3, 2, +1)).toBe(d3)
  })

  it('supprimer un pas ; le dernier pas ne se supprime pas (1 à 200 pas)', () => {
    expect(removeStep(d3, 1).steps.map(s => s[0].target)).toEqual(['g1', 'g3'])
    const one = doc()
    expect(removeStep(one, 0)).toBe(one)
  })

  it('insérer avant', () => {
    expect(insertStepBefore(d3, 0).steps).toHaveLength(4)
    expect(insertStepBefore(d3, 0).steps[0]).toEqual([])
  })

  it('200 pas au plus', () => {
    const full = doc({ steps: Array.from({ length: MAX_STEPS }, () => [A('g1')]) })
    expect(addStep(full)).toBe(full)
    expect(duplicateStep(full, 0)).toBe(full)
  })
})

describe('variables (§11.8, §12.1)', () => {
  it('le nom proposé est la première lettre libre de gA à gZ ; 24 au plus', () => {
    let d = doc()
    expect(nextVariableName(d)).toBe('gA')
    d = addVariable(d)
    d = addVariable(d)
    expect(d.variables.map(v => v.name)).toEqual(['gA', 'gB'])
    expect(nextVariableName(d)).toBe('gC')
    for (let i = 0; i < 30; i++) d = addVariable(d)
    expect(d.variables).toHaveLength(MAX_VARIABLES)
  })

  it('une variable ajoutée est une avance régulière par défaut (départ et pas en texte d\'expression)', () => {
    const v = addVariable(doc()).variables[0]
    expect(v.kind).toBe('progression')
    expect(typeof v.start).toBe('string')
    expect(typeof v.step).toBe('string')
  })

  it('suppression : la ligne disparaît avec ses actions (cible et voisins), les autres restent', () => {
    let d = doc({
      variables: [{ name: 'gA', kind: 'progression', start: '1', step: '+1' }, { name: 'gB', kind: 'progression', start: '2', step: null }],
      steps: [[A('g0'), A('gA+1'), A('gA'), A('gB')]], pre: [A('gA')],
    })
    d = removeVariable(d, 0)
    expect(d.variables.map(v => v.name)).toEqual(['gB'])
    expect(targets(d.steps[0])).toEqual(['g0', 'gB'])
    expect(d.pre).toBeNull()
  })

  it('réordonner change l\'ordre d\'écriture des actions (la dernière déclarée gagne)', () => {
    let d = doc({
      variables: [{ name: 'gA', kind: 'progression', start: '1', step: '+1' }, { name: 'gB', kind: 'progression', start: '2', step: '+1' }],
      steps: [[A('gA'), A('gB')]],
    })
    d = moveVariable(d, 0, +1)
    expect(d.variables.map(v => v.name)).toEqual(['gB', 'gA'])
    expect(targets(d.steps[0])).toEqual(['gB', 'gA'])
    expect(moveVariable(d, 0, -1)).toBe(d)
  })

  it('« Suite de groupes » ⇄ « Avance régulière » garde un état cohérent', () => {
    const d0 = addVariable(doc())
    const dl = setVariableKind(d0, 0, 'list')
    expect(dl.variables[0].kind).toBe('list')
    expect(Array.isArray(dl.variables[0].items)).toBe(true)
    expect(dl.variables[0].start).toBeUndefined()
    const dp = setVariableKind(dl, 0, 'progression')
    expect(dp.variables[0].items).toBeUndefined()
    expect(typeof dp.variables[0].start).toBe('string')
  })

  it('la liste de groupes se lit comme « g1, g4, g3 » : numéros de 1 à 12 seulement', () => {
    expect(parseItems('g1, g4 , 3')).toEqual(['g1', 'g4', 'g3'])
    expect(parseItems('0 13 7')).toEqual(['g7'])
    expect(parseItems('')).toEqual([])
  })
})

describe('groupes de la grille — dernière ligne = g* (§11.8, révision 7)', () => {
  it('ouverture : max(5, plus grand g<k> utilisé) lignes numérotées, plus g*', () => {
    expect(initialNumberedRows(doc())).toBe(5)
    expect(initialNumberedRows(doc({ steps: [[A('g8')]] }))).toBe(8)
    expect(highestNumbered(doc({ steps: [[A('g3'), A('g*')]] }))).toBe(3)
  })

  it('un effet qui écrit g12 (natif degrade) : 12 lignes numérotées, « Ajouter un groupe » indisponible', () => {
    const d = doc({ steps: [[A('g12')]] })
    expect(initialNumberedRows(d)).toBe(12)
    expect(canAddGroup(12)).toBe(false)
  })

  it('lignes affichées : variables en haut, puis g0, g1…g<n>, g* en dernier', () => {
    const d = doc({ variables: [{ name: 'gA', kind: 'progression', start: '1', step: '+1' }] })
    expect(gridRows(d, 5).map(r => r.target)).toEqual(['gA', 'g0', 'g1', 'g2', 'g3', 'g4', 'g5', 'g*'])
    expect(gridRows(doc(), 6).at(-1).target).toBe('g*')
  })

  it('Ajouter un groupe : les actions de g* deviennent g<n+1>, la nouvelle ligne g* est vide', () => {
    const d = doc({ steps: [[A('g0'), A('g5'), A('g*', 'urgence', 70)], [A('g*', '#00FF00', 30)]], pre: [A('g*', 'scene', 10)] })
    const r = addGroup(d, 5)
    expect(r.steps[0].map(a => a.target)).toEqual(['g0', 'g5', 'g6'])
    expect(r.steps[0][2]).toEqual({ target: 'g6', color: 'urgence', intensity: 70 })
    expect(r.steps[1][0].target).toBe('g6')
    expect(r.pre[0].target).toBe('g6')
    expect(starRowUsed(r)).toBe(false)
  })

  it('Ajouter : jusqu\'à 12 lignes en tout (11 numérotées + g*) ; au-delà sans effet', () => {
    const d = doc({ steps: [[A('g*')]] })
    expect(canAddGroup(10)).toBe(true)
    expect(addGroup(d, 11)).toBe(d)
  })

  it('Retirer un groupe : g* supprimée, puis g<n> devient g*', () => {
    const d = doc({ steps: [[A('g0'), A('g4', 'scene', 20), A('g5', '#FF0000', 90), A('g*', 'urgence', 70)]] })
    const r = removeGroup(d, 5)
    expect(r.steps[0].map(a => a.target)).toEqual(['g0', 'g4', 'g*'])
    expect(r.steps[0][2]).toEqual({ target: 'g*', color: '#FF0000', intensity: 90 })
  })

  it('confirmation avant Retirer seulement si la ligne g* n\'est pas vide', () => {
    expect(starRowUsed(doc({ steps: [[A('g0')]] }))).toBe(false)
    expect(starRowUsed(doc({ steps: [[A('g0')], [A('g*')]] }))).toBe(true)
    expect(starRowUsed(doc({ post: [A('g*')] }))).toBe(true)
  })

  it('Retirer jusqu\'à une seule ligne numérotée ; ajouter puis retirer est neutre quand g* est vide', () => {
    const d = doc({ steps: [[A('g0'), A('g1')]] })
    expect(removeGroup(addGroup(d, 5), 6).steps).toEqual(d.steps)
  })

  it('vider un pas en retirant son seul contenu laisse null pour pre / last / post', () => {
    const d = doc({ post: [A('g*')], steps: [[A('g0')]] })
    expect(removeGroup(d, 3).post).toBeNull()
  })
})

describe('fiche de l\'effet', () => {
  it('Nouvel effet : un pas (g0 au thème, 100 %), aucune variable, boucle, moment chrono', () => {
    const d = newEffectDocument({ id: 'nouveau', name: 'Nouveau' })
    expect(d.steps).toEqual([[{ target: 'g0', color: 'theme', intensity: 100 }]])
    expect(d.variables).toEqual([])
    expect(d.loop).toBe(true)
    expect(d.moments).toEqual(['chrono'])
    for (const k of ['pre', 'last', 'post']) expect(d[k]).toBeNull()
  })

  it('Dupliquer : copie profonde sous un autre identifiant et un autre nom', () => {
    const d = doc({ steps: [[A('g1')]] })
    const c = copyDocument(d, { id: 'copie', name: 'Copie' })
    c.steps[0][0].intensity = 1
    expect(c.id).toBe('copie')
    expect(d.steps[0][0].intensity).toBe(100)
    expect(d.id).toBe('e')
  })

  it('identifiant proposé : <id>-copie tronqué à 40', () => {
    expect(copyId('chenillard')).toBe('chenillard-copie')
    expect(copyId('a'.repeat(40))).toHaveLength(40)
  })

  it('moments gardés dans l\'ordre chrono, calm, score', () => {
    expect(toggleMoment(doc({ moments: ['score'] }), 'chrono').moments).toEqual(['chrono', 'score'])
    expect(toggleMoment(doc({ moments: ['chrono', 'calm'] }), 'chrono').moments).toEqual(['calm'])
  })

  it('sameDocument : modifications non enregistrées', () => {
    expect(sameDocument(doc(), doc())).toBe(true)
    expect(sameDocument(doc(), setCell(doc(), 0, 'g1', { off: true }))).toBe(false)
  })
})

describe('erreurs du serveur placées par path (§11.3)', () => {
  it.each([
    ['steps[2]', { kind: 'column', column: 2 }],
    ['pre', { kind: 'column', column: 'pre' }],
    ['variables[1].start', { kind: 'variable', index: 1, field: 'start' }],
    ['variables[0].step', { kind: 'variable', index: 0, field: 'step' }],
    ['variables[3]', { kind: 'variable', index: 3, field: '' }],
    ['tick_ms', { kind: 'field', field: 'tick_ms' }],
    ['', { kind: 'other' }],
    [undefined, { kind: 'other' }],
  ])('%s', (path, want) => expect(locateError(path)).toEqual(want))
})
