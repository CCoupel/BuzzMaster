// #210 (v11.8.0, LOT-2) — opérations PURES sur le document d'effet de l'éditeur (forme JSON d'échange).
// Contrat : contracts/light-effects.md §11.1 (document, ordre canonique), §11.2 (`grid.effect`), §11.8 (groupes de la grille,
// variables), §12 (`g*`, 24 variables, 40 actions). Maquette : docs/mockup/v11.8.0/ui/ambiance_page__effect_editor.html.
//
// AUCUNE analyse de YAML, AUCUN calcul de trame, AUCUNE évaluation d'expression ici : le serveur (route `draft`) est le seul
// analyseur, le seul validateur et le seul interprète. Ce module ne fait que construire et modifier le document.
//
// Document : { id, name, description, moments[], min_groups, tick_ms, loop,
//              variables[{ name, kind: 'progression'|'list', start, step | items[] }],
//              pre, steps[][], last, post }   — `pre`/`last`/`post` : tableau d'actions ou `null` (= pas absent).
// Action : { target, color, intensity } | { target, off: true }.
// Toutes les fonctions rendent un NOUVEAU document (le document reçu n'est jamais modifié).

export const MAX_VARIABLES = 24
export const MAX_ACTIONS = 40
export const MAX_STEPS = 200
export const STAR = 'g*'
export const MAX_NUMBERED = 11 // « Ajouter un groupe » : g1…g11 + g* = 12 lignes (§11.8)
export const MIN_NUMBERED_ROWS = 5 // 6 lignes au départ : g1…g5 + g*

// Colonnes d'un pas : `pre`, `last`, `post` ou l'indice d'un pas de `steps`.
export const COLUMN_PRE = 'pre'
export const COLUMN_LAST = 'last'
export const COLUMN_POST = 'post'

const NAMED_COLORS = ['theme', 'equipe', 'urgence', 'scene']
export const isNamedColor = (c) => NAMED_COLORS.includes(c)
export const isHexColor = (c) => typeof c === 'string' && /^#[0-9a-fA-F]{6}$/.test(c)

const clone = (v) => (v === undefined ? v : JSON.parse(JSON.stringify(v)))

// ---- cibles -----------------------------------------------------------------------------------------------------------
const NUMBERED = /^g([1-9]|1[0-2])$/
const VARIABLE = /^g[A-Z][A-Za-z0-9]*$/
export const isNumberedTarget = (t) => NUMBERED.test(t)
export const numberOfTarget = (t) => (NUMBERED.test(t) ? Number(t.slice(1)) : 0)
export const isVariableTarget = (t) => VARIABLE.test(t)

// Rang d'écriture d'une action (§11.1) : `g0`, puis `g1`…`g12`, puis `g*`, puis les variables dans l'ordre de `variables`.
// Une cible inconnue (voisin `gA+1`, lu dans un fichier) se range juste avant sa variable ; sinon à la fin.
function rankOf(target, varNames) {
  if (target === 'g0') return [0, 0, 0]
  if (NUMBERED.test(target)) return [1, numberOfTarget(target), 0]
  if (target === STAR) return [2, 0, 0]
  const vi = varNames.indexOf(target)
  if (vi >= 0) return [3, vi, 0]
  const m = /^(g[A-Z][A-Za-z0-9]*)([+-])(\d+)$/.exec(target)
  if (m && varNames.includes(m[1])) return [3, varNames.indexOf(m[1]), -1000 + (m[2] === '-' ? -1 : 1) * Number(m[3])]
  return [4, 0, 0]
}
const cmpRank = (a, b) => a[0] - b[0] || a[1] - b[1] || a[2] - b[2]

// Tri d'un tableau d'actions dans l'ordre canonique ; stable (même cible : ordre d'origine).
export function sortActions(actions, variables = []) {
  const names = variables.map(v => v.name)
  return actions
    .map((a, i) => ({ a, i, r: rankOf(a.target, names) }))
    .sort((x, y) => cmpRank(x.r, y.r) || x.i - y.i)
    .map(x => x.a)
}

// Vrai quand au moins un pas du document n'est pas dans l'ordre canonique (à signaler quand le fichier le dit : `custom_order`).
export function isCanonical(doc) {
  const names = doc.variables.map(v => v.name)
  const ok = (arr) => !arr || arr.every((a, i) => i === 0 || cmpRank(rankOf(arr[i - 1].target, names), rankOf(a.target, names)) <= 0)
  return ok(doc.pre) && doc.steps.every(ok) && ok(doc.last) && ok(doc.post)
}

// ---- colonnes ---------------------------------------------------------------------------------------------------------
const getColumn = (doc, col) => (col === COLUMN_PRE ? doc.pre : col === COLUMN_LAST ? doc.last : col === COLUMN_POST ? doc.post : doc.steps[col])

function withColumn(doc, col, actions) {
  const next = { ...doc }
  if (col === COLUMN_PRE || col === COLUMN_LAST || col === COLUMN_POST) next[col] = actions && actions.length ? actions : null
  else next.steps = doc.steps.map((s, i) => (i === col ? (actions || []) : s))
  return next
}

// Action d'une case (colonne × cible), ou `undefined` si la case est vide.
export function getCell(doc, col, target) {
  return (getColumn(doc, col) || []).find(a => a.target === target)
}

export const brushAction = (target, brush) => {
  if (brush.color === 'off') return { target, off: true }
  return { target, color: brush.color, intensity: Math.max(0, Math.min(100, Math.round(Number(brush.intensity)))) }
}

// Écrit (`action` définie) ou vide (`action` nulle) une case. Les actions restent dans l'ordre canonique. Vider la dernière
// action de `pre` / `last` / `post` rend le pas absent (`null`) ; un pas de `steps` vidé reste un tableau vide (erreur du serveur,
// que la grille place sur la colonne : « un pas compte de 1 à 40 actions »).
export function setCell(doc, col, target, action) {
  const without = (getColumn(doc, col) || []).filter(a => a.target !== target)
  const list = action ? [...without, { ...action, target }] : without
  return withColumn(doc, col, sortActions(list, doc.variables))
}

// ---- pas --------------------------------------------------------------------------------------------------------------
export const addStep = (doc) => (doc.steps.length >= MAX_STEPS ? doc : { ...doc, steps: [...doc.steps, []] })
export function duplicateStep(doc, i) {
  if (doc.steps.length >= MAX_STEPS || !doc.steps[i]) return doc
  const steps = [...doc.steps]
  steps.splice(i + 1, 0, clone(doc.steps[i]))
  return { ...doc, steps }
}
export function insertStepBefore(doc, i) {
  if (doc.steps.length >= MAX_STEPS) return doc
  const steps = [...doc.steps]
  steps.splice(i, 0, [])
  return { ...doc, steps }
}
export function moveStep(doc, i, delta) {
  const j = i + delta
  if (j < 0 || j >= doc.steps.length || !doc.steps[i]) return doc
  const steps = [...doc.steps]
  ;[steps[i], steps[j]] = [steps[j], steps[i]]
  return { ...doc, steps }
}
export function removeStep(doc, i) {
  if (doc.steps.length <= 1 || !doc.steps[i]) return doc
  return { ...doc, steps: doc.steps.filter((_, k) => k !== i) }
}

// ---- variables --------------------------------------------------------------------------------------------------------
// Première lettre libre de gA à gZ (24 variables au plus) ; '' quand il n'y en a plus.
export function nextVariableName(doc) {
  const used = new Set(doc.variables.map(v => v.name))
  for (const c of 'ABCDEFGHIJKLMNOPQRSTUVWXYZ') if (!used.has(`g${c}`)) return `g${c}`
  return ''
}
export function addVariable(doc) {
  const name = nextVariableName(doc)
  if (!name || doc.variables.length >= MAX_VARIABLES) return doc
  return { ...doc, variables: [...doc.variables, { name, kind: 'progression', start: '1', step: '+1' }] }
}
export function updateVariable(doc, i, patch) {
  if (!doc.variables[i]) return doc
  const v = { ...doc.variables[i], ...patch }
  if (v.kind === 'list') { delete v.start; delete v.step; v.items = v.items || ['g1', 'g4', 'g2'] } else { delete v.items; v.start = v.start ?? '1'; v.step = v.step === undefined ? '+1' : v.step }
  return { ...doc, variables: doc.variables.map((x, k) => (k === i ? v : x)) }
}
// Passage d'une variable de « Suite de groupes » à « Avance régulière » et inversement, en gardant ce qui peut l'être.
export function setVariableKind(doc, i, kind) {
  const v = doc.variables[i]
  if (!v || v.kind === kind) return doc
  return updateVariable(doc, i, kind === 'list' ? { kind, items: ['g1', 'g4', 'g2'] } : { kind, start: '1', step: '+1' })
}
// « g1, g4, g3 » → ['g1', 'g4', 'g3'] (numéros de 1 à 12 ; le reste est ignoré — le serveur valide la liste).
export const parseItems = (text) => (String(text).match(/\d+/g) || []).map(Number).filter(n => n >= 1 && n <= 12).map(n => `g${n}`)
export const itemsText = (items) => (items || []).join(', ')
export function moveVariable(doc, i, delta) {
  const j = i + delta
  if (j < 0 || j >= doc.variables.length) return doc
  const variables = [...doc.variables]
  ;[variables[i], variables[j]] = [variables[j], variables[i]]
  const next = { ...doc, variables }
  return resortAll(next) // l'ordre des variables décide de l'ordre d'écriture
}
export function removeVariable(doc, i) {
  const v = doc.variables[i]
  if (!v) return doc
  const drop = (arr) => (arr ? arr.filter(a => a.target !== v.name && !a.target.startsWith(v.name + '+') && !a.target.startsWith(v.name + '-')) : arr)
  const nz = (arr) => (arr && arr.length ? arr : null)
  return {
    ...doc,
    variables: doc.variables.filter((_, k) => k !== i),
    pre: nz(drop(doc.pre)), last: nz(drop(doc.last)), post: nz(drop(doc.post)),
    steps: doc.steps.map(drop),
  }
}
function resortAll(doc) {
  const s = (arr) => (arr ? sortActions(arr, doc.variables) : arr)
  return { ...doc, pre: s(doc.pre), last: s(doc.last), post: s(doc.post), steps: doc.steps.map(s) }
}

// ---- groupes de la grille (§11.8, révision 7) --------------------------------------------------------------------------
const allActionLists = (doc) => [doc.pre, ...doc.steps, doc.last, doc.post]

// Plus grand numéro `g<k>` écrit dans le document (0 si aucun).
export function highestNumbered(doc) {
  let max = 0
  for (const list of allActionLists(doc)) for (const a of list || []) max = Math.max(max, numberOfTarget(a.target))
  return max
}
// Lignes numérotées affichées à l'ouverture : max(5, plus grand g<k> utilisé) ; la ligne g* s'y ajoute toujours.
export const initialNumberedRows = (doc) => Math.max(MIN_NUMBERED_ROWS, highestNumbered(doc))

export const canAddGroup = (numbered) => numbered < MAX_NUMBERED
export const canRemoveGroup = (numbered) => numbered >= 1

// La ligne g* porte-t-elle une action ? (confirmation avant « Retirer un groupe »)
export const starRowUsed = (doc) => allActionLists(doc).some(list => (list || []).some(a => a.target === STAR))

function renameTargets(doc, from, to) {
  const rn = (arr) => (arr ? arr.map(a => (a.target === from ? { ...a, target: to } : a)) : arr)
  return resortAll({ ...doc, pre: rn(doc.pre), last: rn(doc.last), post: rn(doc.post), steps: doc.steps.map(rn) })
}

// « Ajouter un groupe » : les actions de g* deviennent g<numbered+1> (la ligne garde son contenu) ; une nouvelle ligne g* est vide.
export function addGroup(doc, numbered) {
  return canAddGroup(numbered) ? renameTargets(doc, STAR, `g${numbered + 1}`) : doc
}
// « Retirer un groupe » : les actions de g* sont supprimées, puis celles de g<numbered> deviennent g*.
export function removeGroup(doc, numbered) {
  if (!canRemoveGroup(numbered)) return doc
  const drop = (arr) => (arr ? arr.filter(a => a.target !== STAR) : arr)
  const nz = (arr) => (arr && arr.length ? arr : null)
  const cleaned = { ...doc, pre: nz(drop(doc.pre)), last: nz(drop(doc.last)), post: nz(drop(doc.post)), steps: doc.steps.map(drop) }
  return renameTargets(cleaned, `g${numbered}`, STAR)
}

// Lignes de la grille, de haut en bas (§11.8) : variables (dans l'ordre de `variables`), puis g0, g1…g<numbered>, g*.
// Les cibles inconnues de la grille (voisins d'un document `effect` fidèle au fichier) sont ajoutées après les variables.
export function gridRows(doc, numbered) {
  const rows = doc.variables.map(v => ({ target: v.name, kind: 'variable', variable: v }))
  const known = new Set(['g0', STAR, ...doc.variables.map(v => v.name)])
  for (let k = 1; k <= Math.max(numbered, 12); k++) known.add(`g${k}`)
  const extra = []
  for (const list of allActionLists(doc)) for (const a of list || []) if (!known.has(a.target) && !extra.includes(a.target)) extra.push(a.target)
  for (const t of extra) rows.push({ target: t, kind: 'extra' })
  rows.push({ target: 'g0', kind: 'all' })
  for (let k = 1; k <= numbered; k++) rows.push({ target: `g${k}`, kind: 'group' })
  rows.push({ target: STAR, kind: 'last' })
  return rows
}

// ---- document neuf, copie, fiche ---------------------------------------------------------------------------------------
// « Nouvel effet » : un seul pas (g0 au thème, 100 %), sans variable (maquette §04).
export const newEffectDocument = ({ id, name }) => ({
  id, name, description: '', moments: ['chrono'], min_groups: 0, tick_ms: 1000, loop: true,
  variables: [], pre: null, steps: [[{ target: 'g0', color: 'theme', intensity: 100 }]], last: null, post: null,
})
// « Dupliquer » : copie profonde sous un autre identifiant et un autre nom.
export const copyDocument = (doc, { id, name }) => ({ ...clone(doc), id, name })
export const withFields = (doc, patch) => ({ ...doc, ...patch })
export const toggleMoment = (doc, moment) => {
  const has = doc.moments.includes(moment)
  const order = ['chrono', 'calm', 'score']
  const moments = has ? doc.moments.filter(m => m !== moment) : order.filter(m => m === moment || doc.moments.includes(m))
  return { ...doc, moments }
}
// Identifiant proposé pour une copie : `<id>-copie`, tronqué à 40 caractères.
export const copyId = (id) => `${id}-copie`.slice(0, 40)
// Durée d'un tick : secondes affichées avec une virgule ↔ `tick_ms`.
export const tickSecondsText = (ms) => String((Number(ms) || 0) / 1000).replace('.', ',')
export const parseTickSeconds = (text) => {
  const n = Number(String(text).trim().replace(',', '.'))
  return Number.isFinite(n) ? Math.round(n * 1000) : NaN
}

// Deux documents sont-ils identiques ? (modifications non enregistrées)
export const sameDocument = (a, b) => JSON.stringify(a) === JSON.stringify(b)

// ---- erreurs du serveur placées dans la grille -------------------------------------------------------------------------
// `path` d'une erreur de brouillon `effect` (§11.3) : « steps[2] », « variables[0].start », « pre », « tick_ms »…
// -> { kind: 'column', column } | { kind: 'variable', index, field } | { kind: 'field', field } | { kind: 'other' }
export function locateError(path) {
  if (typeof path !== 'string' || !path) return { kind: 'other' }
  let m = /^steps\[(\d+)\]/.exec(path)
  if (m) return { kind: 'column', column: Number(m[1]) }
  if (/^(pre|last|post)\b/.test(path)) return { kind: 'column', column: path.match(/^(pre|last|post)/)[1] }
  m = /^variables\[(\d+)\](?:\.(\w+))?/.exec(path)
  if (m) return { kind: 'variable', index: Number(m[1]), field: m[2] || '' }
  return { kind: 'field', field: path.split(/[.[]/)[0] }
}

// Étiquette d'une colonne.
export const columnLabel = (col) => (col === COLUMN_PRE ? 'PRE' : col === COLUMN_LAST ? 'DERNIER' : col === COLUMN_POST ? 'POST' : `pas ${col}`)
