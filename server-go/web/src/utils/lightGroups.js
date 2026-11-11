// #225 (v11.7.0, LOT-1) — fonctions pures de la page « Groupes » de l'onglet
// Lumière. Contrat : contracts/light-effects.md §2.1 (configuration), §2.5
// (groupe animateur). Maquette : docs/mockup/v11.7.0/ui/ambiance_page__light_groups.html.
//
// Modèle d'édition (« brouillon ») : { groups: [{name, lights, near_teams}], host: null | {name, lights} }
//   - `groups` : groupes de salle, dans l'ordre de parcours ; le numéro est le rang (1-based),
//     jamais saisi.
//   - `host`   : le groupe animateur (un seul, sans numéro, sans équipes à proximité).
// Charge utile `lighting.groups` : groupes de salle puis, s'il existe, le groupe animateur
// (`role: "animateur"`). Toutes les fonctions retournent un nouveau brouillon (immuable).

export const MAX_GROUPS = 12
export const MAX_NAME_LENGTH = 40
export const ROLE_SALLE = 'salle'
export const ROLE_HOST = 'animateur'

export const EMPTY_DRAFT = Object.freeze({ groups: [], host: null })

const asStrings = (v) => (Array.isArray(v) ? v.filter(x => typeof x === 'string') : [])

// Configuration enregistrée (`lighting.groups`) -> brouillon. Tolérant : entrée absente,
// non tableau, ou plusieurs groupes animateur (seul le premier est gardé).
export function draftFromConfig(groups) {
  const draft = { groups: [], host: null }
  if (!Array.isArray(groups)) return draft
  for (const g of groups) {
    if (!g || typeof g !== 'object') continue
    const name = typeof g.name === 'string' ? g.name : ''
    if (g.role === ROLE_HOST) {
      if (!draft.host) draft.host = { name, lights: asStrings(g.lights) }
    } else {
      draft.groups.push({ name, lights: asStrings(g.lights), near_teams: asStrings(g.near_teams) })
    }
  }
  return draft
}

// Brouillon -> `lighting.groups` (remplacement complet du tableau).
export function draftToPayload(draft) {
  const out = draft.groups.map(g => ({
    name: g.name.trim(),
    role: ROLE_SALLE,
    lights: [...g.lights],
    near_teams: [...g.near_teams],
  }))
  if (draft.host) {
    out.push({ name: draft.host.name.trim(), role: ROLE_HOST, lights: [...draft.host.lights], near_teams: [] })
  }
  return out
}

export const draftsEqual = (a, b) => JSON.stringify(draftToPayload(a)) === JSON.stringify(draftToPayload(b))

export const groupCount = (draft) => draft.groups.length + (draft.host ? 1 : 0)

// Retire du brouillon les ampoules qui ne sont plus de rôle général (ou n'existent plus).
export function pruneToGeneral(draft, generalNames) {
  const ok = new Set(generalNames)
  const keep = (arr) => arr.filter(n => ok.has(n))
  return {
    groups: draft.groups.map(g => ({ ...g, lights: keep(g.lights) })),
    host: draft.host ? { ...draft.host, lights: keep(draft.host.lights) } : null,
  }
}

// Ampoules générales rangées dans aucun groupe, dans l'ordre de `generalNames`.
export function unassignedLights(draft, generalNames) {
  const used = new Set([...draft.groups.flatMap(g => g.lights), ...(draft.host ? draft.host.lights : [])])
  return generalNames.filter(n => !used.has(n))
}

// Où se trouve une ampoule : {kind:'group', index} | {kind:'host'} | {kind:'pool'}.
export function locateLight(draft, name) {
  const index = draft.groups.findIndex(g => g.lights.includes(name))
  if (index >= 0) return { kind: 'group', index }
  if (draft.host && draft.host.lights.includes(name)) return { kind: 'host' }
  return { kind: 'pool' }
}

function withoutLight(draft, name) {
  return {
    groups: draft.groups.map(g => (g.lights.includes(name) ? { ...g, lights: g.lights.filter(n => n !== name) } : g)),
    host: draft.host && draft.host.lights.includes(name)
      ? { ...draft.host, lights: draft.host.lights.filter(n => n !== name) }
      : draft.host,
  }
}

// Déplace une ampoule vers `target` : {kind:'pool'} | {kind:'group', index} | {kind:'host'}.
// Une ampoule n'est jamais que dans un groupe. Cible inconnue : brouillon inchangé.
export function moveLight(draft, name, target) {
  const base = withoutLight(draft, name)
  if (target.kind === 'pool') return base
  if (target.kind === 'host') {
    if (!base.host) return draft
    return { ...base, host: { ...base.host, lights: [...base.host.lights, name] } }
  }
  if (target.kind === 'group') {
    if (target.index < 0 || target.index >= base.groups.length) return draft
    return {
      ...base,
      groups: base.groups.map((g, i) => (i === target.index ? { ...g, lights: [...g.lights, name] } : g)),
    }
  }
  return draft
}

// Nom proposé par défaut : « Groupe 1 », « Groupe 2 »… (premier libre, casse ignorée).
export function defaultGroupName(draft) {
  const used = new Set([...draft.groups.map(g => g.name), draft.host ? draft.host.name : ''].map(n => n.trim().toLowerCase()))
  let i = draft.groups.length + 1
  while (used.has(`groupe ${i}`)) i += 1
  return `Groupe ${i}`
}

export function addGroup(draft) {
  if (groupCount(draft) >= MAX_GROUPS) return draft
  return { ...draft, groups: [...draft.groups, { name: defaultGroupName(draft), lights: [], near_teams: [] }] }
}

export function removeGroup(draft, index) {
  if (index < 0 || index >= draft.groups.length) return draft
  return { ...draft, groups: draft.groups.filter((_, i) => i !== index) }
}

export function renameGroup(draft, index, name) {
  return { ...draft, groups: draft.groups.map((g, i) => (i === index ? { ...g, name } : g)) }
}

// Change le rang d'un groupe de salle (les numéros se recalculent seuls).
export function moveGroup(draft, from, to) {
  const n = draft.groups.length
  if (from === to || from < 0 || from >= n || to < 0 || to >= n) return draft
  const next = [...draft.groups]
  const [item] = next.splice(from, 1)
  next.splice(to, 0, item)
  return { ...draft, groups: next }
}

// Équipes à proximité : une équipe n'est que dans un groupe au plus (l'ajouter ailleurs l'en retire).
export function addNearTeam(draft, index, team) {
  if (!team || index < 0 || index >= draft.groups.length) return draft
  return {
    ...draft,
    groups: draft.groups.map((g, i) => {
      const without = g.near_teams.filter(t => t !== team)
      return i === index ? { ...g, near_teams: [...without, team] } : { ...g, near_teams: without }
    }),
  }
}

export function removeNearTeam(draft, index, team) {
  return { ...draft, groups: draft.groups.map((g, i) => (i === index ? { ...g, near_teams: g.near_teams.filter(t => t !== team) } : g)) }
}

// Équipes proposées dans le menu d'un groupe : celles qui ne sont dans aucun groupe.
export const availableTeams = (draft, teamNames) => {
  const used = new Set(draft.groups.flatMap(g => g.near_teams))
  return teamNames.filter(t => !used.has(t))
}

// Retire des « équipes à proximité » les équipes qui n'existent plus.
export function pruneTeams(draft, teamNames) {
  const ok = new Set(teamNames)
  return { ...draft, groups: draft.groups.map(g => ({ ...g, near_teams: g.near_teams.filter(t => ok.has(t)) })) }
}

export function addHost(draft) {
  if (draft.host || groupCount(draft) >= MAX_GROUPS) return draft
  return { ...draft, host: { name: draft.groups.some(g => g.name.trim().toLowerCase() === 'animateur') ? 'Pupitre' : 'Animateur', lights: [] } }
}

export const removeHost = (draft) => ({ ...draft, host: null })

export const renameHost = (draft, name) => (draft.host ? { ...draft, host: { ...draft.host, name } } : draft)

// « Créer un groupe par ampoule » : un groupe de salle par ampoule générale, dans l'ordre fourni,
// nommé d'après l'ampoule. Ne touche ni au groupe animateur ni aux groupes existants : ajoute
// seulement les ampoules encore hors groupe (plafonné à MAX_GROUPS).
export function createGroupPerLight(draft, generalNames) {
  let next = draft
  for (const name of unassignedLights(draft, generalNames)) {
    if (groupCount(next) >= MAX_GROUPS) break
    const taken = new Set([...next.groups.map(g => g.name), next.host ? next.host.name : ''].map(n => n.trim().toLowerCase()))
    const label = taken.has(name.toLowerCase()) ? `${name} (groupe)` : name
    next = { ...next, groups: [...next.groups, { name: label, lights: [name], near_teams: [] }] }
  }
  return next
}

// Erreurs de validation (messages lisibles, alignés sur le refus 400 du serveur).
// Retourne { global: [..], byGroup: { [clé]: msg } } ; clé = indice de salle ou 'host'.
export function validateDraft(draft) {
  const errors = { global: [], byGroup: {} }
  if (groupCount(draft) > MAX_GROUPS) errors.global.push(`${MAX_GROUPS} groupes au plus.`)
  const seen = new Map()
  const check = (key, name) => {
    const n = name.trim()
    if (n.length < 1) { errors.byGroup[key] = 'Le nom est obligatoire.'; return }
    if (n.length > MAX_NAME_LENGTH) { errors.byGroup[key] = `${MAX_NAME_LENGTH} caractères au plus.`; return }
    const low = n.toLowerCase()
    if (seen.has(low)) { errors.byGroup[key] = 'Ce nom est déjà utilisé par un autre groupe.'; return }
    seen.set(low, key)
  }
  draft.groups.forEach((g, i) => check(i, g.name))
  if (draft.host) check('host', draft.host.name)
  return errors
}

export const isDraftValid = (draft) => {
  const e = validateDraft(draft)
  return e.global.length === 0 && Object.keys(e.byGroup).length === 0
}

// Libellé d'une ampoule vue depuis « Pont et ampoules » : {kind:'salle', n, name} | {kind:'host', name} | null.
export function groupLabelForLight(groups, lightName) {
  const draft = draftFromConfig(groups)
  const loc = locateLight(draft, lightName)
  if (loc.kind === 'group') return { kind: 'salle', n: loc.index + 1, name: draft.groups[loc.index].name }
  if (loc.kind === 'host') return { kind: 'host', name: draft.host.name }
  return null
}

// Message d'avertissement de la page de tête quand un changement de rôle retire une ampoule de son groupe.
export const teamRoleWarning = (names) =>
  names.length === 0 ? '' : `${names.join(', ')} ${names.length > 1 ? 'seront retirées' : 'sera retirée'} de leur groupe à l'enregistrement.`

// Résumé sous le tableau : ampoules hors groupe et groupes vides.
export function summarize(draft, generalNames) {
  const free = unassignedLights(draft, generalNames).length
  const empty = draft.groups.filter(g => g.lights.length === 0).length
  const active = draft.groups.length - empty
  return { free, empty, active }
}
