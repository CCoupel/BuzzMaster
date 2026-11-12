import { describe, it, expect } from 'vitest'
import {
  addGroup, addStep, addVariable, canAddGroup, copyDocument, copyId, duplicateStep, getCell, gridRows, highestNumbered, initialNumberedRows,
  isCanonical, locateError, moveStep, moveVariable, newEffectDocument, parseItems, parseTickSeconds, removeGroup, removeStep, removeVariable,
  setCell, setVariableKind, sortActions, starRowUsed, tickSecondsText, toggleMoment,
} from './effectDocument'

const A = (target, color = 'theme', intensity = 100) => ({ target, color, intensity })
const base = () => ({
  id: 'x', name: 'X', description: '', moments: ['chrono'], min_groups: 0, tick_ms: 1000, loop: true,
  variables: [{ name: 'gA', kind: 'progression', start: 'N/2', step: '+1' }, { name: 'gB', kind: 'progression', start: 'gA+1', step: null }],
  pre: null, steps: [[A('g0'), A('g*', 'scene', 60), A('gA', 'urgence'), A('gB')]], last: null, post: null,
})

describe('effectDocument — écriture dans l\'ordre canonique (§11.1)', () => {
  it('une case écrite se range : g0, g<n>, g*, puis les variables dans l\'ordre de variables', () => {
    let d = setCell(newEffectDocument({ id: 'n', name: 'N' }), 0, 'g3', A('g3', 'urgence', 50))
    d = setCell(d, 0, 'g*', A('g*'))
    d = setCell(d, 0, 'g1', A('g1'))
    expect(d.steps[0].map(a => a.target)).toEqual(['g0', 'g1', 'g3', 'g*'])
    const v = addVariable(addVariable(d))
    const w = setCell(setCell(v, 0, 'gB', A('gB')), 0, 'gA', A('gA'))
    expect(w.steps[0].map(a => a.target)).toEqual(['g0', 'g1', 'g3', 'g*', 'gA', 'gB'])
  })
  it('ne modifie pas le document reçu ; vider la dernière action de pre/last/post rend le pas absent (null)', () => {
    const d = base()
    const snap = JSON.stringify(d)
    const withPre = setCell(d, 'pre', 'g0', A('g0'))
    expect(JSON.stringify(d)).toBe(snap)
    expect(withPre.pre).toEqual([A('g0')])
    expect(setCell(withPre, 'pre', 'g0', null).pre).toBeNull()
  })
  it('vider une case de pas garde le pas (tableau) : le serveur signale « 1 à 40 actions »', () => {
    const d = setCell(newEffectDocument({ id: 'n', name: 'N' }), 0, 'g0', null)
    expect(d.steps).toEqual([[]])
  })
  it('effacer éteint : action { off: true } sans couleur', () => {
    const d = setCell(base(), 0, 'g1', { target: 'g1', off: true })
    expect(getCell(d, 0, 'g1')).toEqual({ target: 'g1', off: true })
  })
  it('un voisin d\'un fichier se range juste avant sa variable ; isCanonical détecte un ordre libre', () => {
    const vars = base().variables
    const s = sortActions([A('gA'), A('gA+1'), A('g0'), A('gA-1')], vars)
    expect(s.map(a => a.target)).toEqual(['g0', 'gA-1', 'gA+1', 'gA'])
    expect(isCanonical(base())).toBe(true)
    expect(isCanonical({ ...base(), steps: [[A('gA'), A('g0')]] })).toBe(false)
  })
})

describe('effectDocument — pas', () => {
  it('ajouter, dupliquer, déplacer, supprimer (jamais le dernier)', () => {
    let d = addStep(base())
    expect(d.steps).toHaveLength(2)
    d = duplicateStep(d, 0)
    expect(d.steps[1]).toEqual(d.steps[0])
    d = moveStep(d, 2, -1) // [p0, p0', vide] → [p0, vide, p0']
    expect(d.steps[1]).toEqual([])
    expect(d.steps[2]).toEqual(d.steps[0])
    expect(moveStep(d, 0, -1)).toBe(d)
    expect(removeStep({ ...base() }, 0).steps).toHaveLength(1)
  })
})

describe('effectDocument — variables', () => {
  it('nom proposé : première lettre libre ; 24 au plus', () => {
    let d = base()
    d = addVariable(d)
    expect(d.variables.at(-1)).toMatchObject({ name: 'gC', kind: 'progression', start: '1', step: '+1' })
    for (let i = 0; i < 40; i++) d = addVariable(d)
    expect(d.variables).toHaveLength(24)
  })
  it('supprimer une variable retire ses actions partout ; l\'ordre se change et réordonne les actions', () => {
    const d = base()
    const r = removeVariable(d, 0)
    expect(r.variables.map(v => v.name)).toEqual(['gB'])
    expect(r.steps[0].map(a => a.target)).toEqual(['g0', 'g*', 'gB'])
    const m = moveVariable(d, 0, 1)
    expect(m.variables.map(v => v.name)).toEqual(['gB', 'gA'])
    expect(m.steps[0].map(a => a.target)).toEqual(['g0', 'g*', 'gB', 'gA'])
  })
  it('changer de type : liste ↔ avance, items numérotés', () => {
    const l = setVariableKind(base(), 0, 'list')
    expect(l.variables[0]).toMatchObject({ kind: 'list', items: ['g1', 'g4', 'g2'] })
    expect(l.variables[0].start).toBeUndefined()
    expect(setVariableKind(l, 0, 'progression').variables[0]).toMatchObject({ kind: 'progression', start: '1', step: '+1' })
    expect(parseItems('g1, g4 ; 13, g3')).toEqual(['g1', 'g4', 'g3'])
  })
})

describe('effectDocument — groupes de la grille (§11.8, révision 7)', () => {
  it('à l\'ouverture : max(5, plus grand g<k>) lignes numérotées + g*', () => {
    expect(initialNumberedRows(base())).toBe(5)
    const d = setCell(base(), 0, 'g12', A('g12'))
    expect(highestNumbered(d)).toBe(12)
    expect(initialNumberedRows(d)).toBe(12)
    expect(canAddGroup(12)).toBe(false)
    expect(gridRows(d, 12).filter(r => r.kind === 'group')).toHaveLength(12)
    expect(gridRows(d, 12).at(-1).target).toBe('g*')
  })
  it('lignes : variables en haut, g0, groupes, g* en dernier', () => {
    expect(gridRows(base(), 5).map(r => r.target)).toEqual(['gA', 'gB', 'g0', 'g1', 'g2', 'g3', 'g4', 'g5', 'g*'])
  })
  it('Ajouter : g* devient g<n+1> et garde son contenu', () => {
    const d = addGroup(base(), 5)
    expect(getCell(d, 0, 'g6')).toEqual(A('g6', 'scene', 60))
    expect(getCell(d, 0, 'g*')).toBeUndefined()
    expect(d.steps[0].map(a => a.target)).toEqual(['g0', 'g6', 'gA', 'gB'])
  })
  it('Retirer : g* supprimée avec son contenu, g<n> devient g*', () => {
    let d = setCell(base(), 0, 'g5', A('g5', 'urgence', 30))
    expect(starRowUsed(d)).toBe(true)
    d = removeGroup(d, 5)
    expect(getCell(d, 0, 'g*')).toEqual(A('g*', 'urgence', 30))
    expect(getCell(d, 0, 'g5')).toBeUndefined()
  })
})

describe('effectDocument — divers', () => {
  it('document neuf : un pas g0 thème 100, sans variable ; copie : autre id, copie profonde', () => {
    const n = newEffectDocument({ id: 'a', name: 'A' })
    expect(n.steps).toEqual([[{ target: 'g0', color: 'theme', intensity: 100 }]])
    expect(n.variables).toEqual([])
    const c = copyDocument(base(), { id: 'y', name: 'Y' })
    c.steps[0][0].intensity = 1
    expect(base().steps[0][0].intensity).toBe(100)
    expect(copyId('a'.repeat(40))).toHaveLength(40)
  })
  it('moments dans l\'ordre chrono, calm, score ; durée du tick en secondes à virgule', () => {
    expect(toggleMoment(toggleMoment(base(), 'score'), 'calm').moments).toEqual(['chrono', 'calm', 'score'])
    expect(tickSecondsText(500)).toBe('0,5')
    expect(parseTickSeconds('0,5')).toBe(500)
    expect(Number.isNaN(parseTickSeconds('abc'))).toBe(true)
  })
  it('chemins d\'erreur du serveur placés', () => {
    expect(locateError('steps[2]')).toEqual({ kind: 'column', column: 2 })
    expect(locateError('pre')).toEqual({ kind: 'column', column: 'pre' })
    expect(locateError('variables[1].start')).toEqual({ kind: 'variable', index: 1, field: 'start' })
    expect(locateError('tick_ms')).toEqual({ kind: 'field', field: 'tick_ms' })
    expect(locateError('')).toEqual({ kind: 'other' })
  })
})
