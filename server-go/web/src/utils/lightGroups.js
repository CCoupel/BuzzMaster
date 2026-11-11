// #225 (v11.7.0, LOT-1) — fonctions pures de la page « Groupes » de l'onglet Lumière.
// Contrat : contracts/light-effects.md §2.1 (configuration), §2.5 (groupes animateur).
// Maquette : docs/mockup/v11.7.0/ui/ambiance_page__light_groups_multi_host.html (révision du GATE LOT-1).
//
// Modèle d'édition (« brouillon ») : { groups: [{id, name, lights, near_teams}], hosts: [{id, name, lights}] }
//   - `groups` : groupes de salle, dans l'ordre de parcours ; le numéro est le rang (1-based), jamais saisi.
//   - `hosts`  : groupes animateur (4 au plus), ordre propre (🎤1, 🎤2…), sans équipes à proximité.
//   - `id`     : identifiant STABLE attribué par le serveur (vide tant que le groupe n'est pas enregistré) ;
//                renvoyé tel quel à l'enregistrement — il sert de clé aux effets des groupes animateur.
// Charge utile `lighting.groups` : groupes de salle puis groupes animateur (`role: "animateur"`).
// Toutes les fonctions retournent un nouveau brouillon (immuable).

export const MAX_ROOM_GROUPS = 12
export const MAX_HOST_GROUPS = 4
export const MAX_NAME_LENGTH = 40
export const ROLE_SALLE = 'salle'
export const ROLE_HOST = 'animateur'

export const EMPTY_DRAFT = Object.freeze({ groups: [], hosts: [] })

const asStrings = (v) => (Array.isArray(v) ? v.filter(x => typeof x === 'string') : [])
const asId = (v) => (typeof v === 'string' ? v : '')

// Configuration enregistrée (`lighting.groups`) -> brouillon. Tolérant : entrée absente ou non tableau.
export function draftFromConfig(groups) {
  const draft = { groups: [], hosts: [] }
  if (!Array.isArray(groups)) return draft
  for (const g of groups) {
    if (!g || typeof g !== 'object') continue
    const name = typeof g.name === 'string' ? g.name : ''
    if (g.role === ROLE_HOST) draft.hosts.push({ id: asId(g.id), name, lights: asStrings(g.lights) })
    else draft.groups.push({ id: asId(g.id), name, lights: asStrings(g.lights), near_teams: asStrings(g.near_teams) })
  }
  return draft
}

// Brouillon -> `lighting.groups` (remplacement complet du tableau). `id` omis tant qu'il est vide (le serveur l'attribue).
export function draftToPayload(draft) {
  const withId = (g, rest) => (g.id ? { id: g.id, ...rest } : rest)
  return [
    ...draft.groups.map(g => withId(g, { name: g.name.trim(), role: ROLE_SALLE, lights: [...g.lights], near_teams: [...g.near_teams] })),
    ...draft.hosts.map(g => withId(g, { name: g.name.trim(), role: ROLE_HOST, lights: [...g.lights], near_teams: [] })),
  ]
}

export const draftsEqual = (a, b) => JSON.stringify(draftToPayload(a)) === JSON.stringify(draftToPayload(b))

export const groupCount = (draft) => draft.groups.length + draft.hosts.length

// Retire du brouillon les ampoules qui ne sont plus de rôle général (ou n'existent plus).
export function pruneToGeneral(draft, generalNames) {
  const ok = new Set(generalNames)
  const keep = (arr) => arr.filter(n => ok.has(n))
  return {
    groups: draft.groups.map(g => ({ ...g, lights: keep(g.lights) })),
    hosts: draft.hosts.map(g => ({ ...g, lights: keep(g.lights) })),
  }
}

// Ampoules générales rangées dans aucun groupe, dans l'ordre de `generalNames`.
export function unassignedLights(draft, generalNames) {
  const used = new Set([...draft.groups.flatMap(g => g.lights), ...draft.hosts.flatMap(g => g.lights)])
  return generalNames.filter(n => !used.has(n))
}

// Où se trouve une ampoule : {kind:'group', index} | {kind:'host', index} | {kind:'pool'}.
export function locateLight(draft, name) {
  const gi = draft.groups.findIndex(g => g.lights.includes(name))
  if (gi >= 0) return { kind: 'group', index: gi }
  const hi = draft.hosts.findIndex(g => g.lights.includes(name))
  if (hi >= 0) return { kind: 'host', index: hi }
  return { kind: 'pool' }
}

function withoutLight(draft, name) {
  const strip = (g) => (g.lights.includes(name) ? { ...g, lights: g.lights.filter(n => n !== name) } : g)
  return { groups: draft.groups.map(strip), hosts: draft.hosts.map(strip) }
}

// Déplace une ampoule vers `target` : {kind:'pool'} | {kind:'group', index} | {kind:'host', index}.
// Une ampoule n'est jamais que dans un groupe. Cible inconnue : brouillon inchangé.
export function moveLight(draft, name, target) {
  const base = withoutLight(draft, name)
  if (target.kind === 'pool') return base
  const key = target.kind === 'group' ? 'groups' : target.kind === 'host' ? 'hosts' : null
  if (!key || target.index < 0 || target.index >= base[key].length) return draft
  return { ...base, [key]: base[key].map((g, i) => (i === target.index ? { ...g, lights: [...g.lights, name] } : g)) }
}

const taken = (draft) => new Set([...draft.groups, ...draft.hosts].map(g => g.name.trim().toLowerCase()))

// Nom proposé par défaut : « Groupe 1 », « Groupe 2 »… (premier libre, casse ignorée).
export function defaultGroupName(draft, prefix = 'Groupe', start = draft.groups.length + 1) {
  const used = taken(draft)
  let i = start
  while (used.has(`${prefix} ${i}`.toLowerCase())) i += 1
  return `${prefix} ${i}`
}

export function addGroup(draft) {
  if (draft.groups.length >= MAX_ROOM_GROUPS) return draft
  return { ...draft, groups: [...draft.groups, { id: '', name: defaultGroupName(draft), lights: [], near_teams: [] }] }
}

export function removeGroup(draft, index) {
  if (index < 0 || index >= draft.groups.length) return draft
  return { ...draft, groups: draft.groups.filter((_, i) => i !== index) }
}

export function renameGroup(draft, index, name) {
  return { ...draft, groups: draft.groups.map((g, i) => (i === index ? { ...g, name } : g)) }
}

// Change le rang d'un groupe de salle (les numéros se recalculent seuls ; l'`id` suit le groupe).
export function moveGroup(draft, from, to) {
  const n = draft.groups.length
  if (from === to || from < 0 || from >= n || to < 0 || to >= n) return draft
  const next = [...draft.groups]
  const [item] = next.splice(from, 1)
  next.splice(to, 0, item)
  return { ...draft, groups: next }
}

// Groupes animateur : création (4 au plus), nom, suppression, ordre propre.
export function addHost(draft) {
  if (draft.hosts.length >= MAX_HOST_GROUPS) return draft
  return { ...draft, hosts: [...draft.hosts, { id: '', name: defaultGroupName(draft, 'Animateur', draft.hosts.length + 1), lights: [] }] }
}

export function removeHost(draft, index) {
  if (index < 0 || index >= draft.hosts.length) return draft
  return { ...draft, hosts: draft.hosts.filter((_, i) => i !== index) }
}

export const renameHost = (draft, index, name) => ({ ...draft, hosts: draft.hosts.map((g, i) => (i === index ? { ...g, name } : g)) })

export function moveHost(draft, from, to) {
  const n = draft.hosts.length
  if (from === to || from < 0 || from >= n || to < 0 || to >= n) return draft
  const next = [...draft.hosts]
  const [item] = next.splice(from, 1)
  next.splice(to, 0, item)
  return { ...draft, hosts: next }
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

// Erreurs de validation (messages lisibles, alignés sur le refus 400 du serveur).
// Retourne { global: [..], byGroup: { [clé]: msg } } ; clé = indice de salle, ou `h<indice>` pour un groupe animateur.
export function validateDraft(draft) {
  const errors = { global: [], byGroup: {} }
  if (draft.groups.length > MAX_ROOM_GROUPS) errors.global.push(`${MAX_ROOM_GROUPS} groupes de salle au plus.`)
  if (draft.hosts.length > MAX_HOST_GROUPS) errors.global.push(`${MAX_HOST_GROUPS} groupes animateur au plus.`)
  const seen = new Set()
  const check = (key, name) => {
    const n = name.trim()
    if (n.length < 1) { errors.byGroup[key] = 'Le nom est obligatoire.'; return }
    if (n.length > MAX_NAME_LENGTH) { errors.byGroup[key] = `${MAX_NAME_LENGTH} caractères au plus.`; return }
    const low = n.toLowerCase()
    if (seen.has(low)) { errors.byGroup[key] = 'Ce nom est déjà utilisé par un autre groupe.'; return }
    seen.add(low)
  }
  draft.groups.forEach((g, i) => check(i, g.name))
  draft.hosts.forEach((g, i) => check(`h${i}`, g.name))
  return errors
}

export const isDraftValid = (draft) => {
  const e = validateDraft(draft)
  return e.global.length === 0 && Object.keys(e.byGroup).length === 0
}

// Libellé d'une ampoule vue depuis « Pont et ampoules » :
// {kind:'salle', n, name} | {kind:'host', rank, name} | null.
export function groupLabelForLight(groups, lightName) {
  const draft = draftFromConfig(groups)
  const loc = locateLight(draft, lightName)
  if (loc.kind === 'group') return { kind: 'salle', n: loc.index + 1, name: draft.groups[loc.index].name }
  if (loc.kind === 'host') return { kind: 'host', rank: loc.index + 1, name: draft.hosts[loc.index].name }
  return null
}

// Message d'avertissement de la page de tête quand un changement de rôle retire une ampoule de son groupe.
export const teamRoleWarning = (names) =>
  names.length === 0 ? '' : `${names.join(', ')} ${names.length > 1 ? 'seront retirées' : 'sera retirée'} de leur groupe à l'enregistrement.`

// Résumé sous le tableau : ampoules hors groupe et groupes de salle vides.
export function summarize(draft, generalNames) {
  const free = unassignedLights(draft, generalNames).length
  const empty = draft.groups.filter(g => g.lights.length === 0).length
  const active = draft.groups.length - empty
  return { free, empty, active }
}

// Pas de la séquence d'essai (contracts/light-effects.md §2.4) : 6 pas d'une seconde.
export const TEST_STEPS = ['éteint', 'blanc 50 %', 'blanc 100 %', 'rouge', 'vert', 'bleu']
export const testStepAt = (elapsedMs, stepMs = 1000) => Math.min(TEST_STEPS.length - 1, Math.max(0, Math.floor(elapsedMs / stepMs)))
