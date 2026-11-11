// #225 (v11.7.0, LOT-1) — fonctions pures de la page « Groupes » de l'onglet Lumière.
// Contrat : contracts/light-effects.md §2.1 (configuration), §2.5 (groupes animateur).
// Maquette : docs/mockup/v11.7.0/ui/ambiance_page__light_groups_numbered.html (révision 3 : plus de nom, plus d'équipes à proximité).
//
// Modèle d'édition (« brouillon ») : { groups: [{id, lights}], hosts: [{id, lights}] }
//   - `groups` : groupes de salle, dans l'ordre de parcours ; le numéro est le rang (1-based), jamais saisi.
//   - `hosts`  : groupes animateur (4 au plus), ordre propre (🎤1, 🎤2…).
//   - Aucun nom : un groupe est son numéro (« Groupe 1 », « Groupe animateur 1 »), contracts/light-effects.md §2.1 (révision 3).
//   - `id`     : identifiant STABLE attribué par le serveur (vide tant que le groupe n'est pas enregistré) ;
//                renvoyé tel quel à l'enregistrement — il sert de clé aux effets des groupes animateur.
// Charge utile `lighting.groups` : groupes de salle puis groupes animateur (`role: "animateur"`).
// Toutes les fonctions retournent un nouveau brouillon (immuable).

export const MAX_ROOM_GROUPS = 12
export const MAX_HOST_GROUPS = 4
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
    // `name` et `near_teams` d'un config.json antérieur sont ignorés (supprimés à la révision 3).
    const entry = { id: asId(g.id), lights: asStrings(g.lights) }
    if (g.role === ROLE_HOST) draft.hosts.push(entry)
    else draft.groups.push(entry)
  }
  return draft
}

// Brouillon -> `lighting.groups` (remplacement complet du tableau). `id` omis tant qu'il est vide (le serveur l'attribue).
export function draftToPayload(draft) {
  const entry = (g, role) => (g.id ? { id: g.id, role, lights: [...g.lights] } : { role, lights: [...g.lights] })
  return [...draft.groups.map(g => entry(g, ROLE_SALLE)), ...draft.hosts.map(g => entry(g, ROLE_HOST))]
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

export function addGroup(draft) {
  if (draft.groups.length >= MAX_ROOM_GROUPS) return draft
  return { ...draft, groups: [...draft.groups, { id: '', lights: [] }] }
}

export function removeGroup(draft, index) {
  if (index < 0 || index >= draft.groups.length) return draft
  return { ...draft, groups: draft.groups.filter((_, i) => i !== index) }
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
  return { ...draft, hosts: [...draft.hosts, { id: '', lights: [] }] }
}

export function removeHost(draft, index) {
  if (index < 0 || index >= draft.hosts.length) return draft
  return { ...draft, hosts: draft.hosts.filter((_, i) => i !== index) }
}

export function moveHost(draft, from, to) {
  const n = draft.hosts.length
  if (from === to || from < 0 || from >= n || to < 0 || to >= n) return draft
  const next = [...draft.hosts]
  const [item] = next.splice(from, 1)
  next.splice(to, 0, item)
  return { ...draft, hosts: next }
}

// Erreurs de validation (messages lisibles, alignés sur le refus 400 du serveur) : plafonds seulement,
// un groupe n'ayant plus de nom. Retourne { global: [..] }.
export function validateDraft(draft) {
  const errors = { global: [] }
  if (draft.groups.length > MAX_ROOM_GROUPS) errors.global.push(`${MAX_ROOM_GROUPS} groupes de salle au plus.`)
  if (draft.hosts.length > MAX_HOST_GROUPS) errors.global.push(`${MAX_HOST_GROUPS} groupes animateur au plus.`)
  return errors
}

export const isDraftValid = (draft) => validateDraft(draft).global.length === 0

// Libellés : un groupe est son rang.
export const groupName = (index) => `Groupe ${index + 1}`
export const hostName = (index) => `Groupe animateur ${index + 1}`

// L'ordre des groupes déjà enregistrés a-t-il changé ? (les numéros g1, g2… désignés par les fichiers d'effets
// changent alors de groupe) — compare la suite des `id` communs au brouillon et à l'enregistré.
export function orderChanged(draft, saved) {
  const seq = (d) => d.groups.map(g => g.id).filter(Boolean)
  const before = seq(saved)
  const common = new Set(before)
  const after = seq(draft).filter(id => common.has(id))
  const kept = before.filter(id => new Set(after).has(id))
  return after.some((id, i) => id !== kept[i])
}

// Libellé d'une ampoule vue depuis « Pont et ampoules » :
// {kind:'salle', n} | {kind:'host', rank} | null.
export function groupLabelForLight(groups, lightName) {
  const draft = draftFromConfig(groups)
  const loc = locateLight(draft, lightName)
  if (loc.kind === 'group') return { kind: 'salle', n: loc.index + 1 }
  if (loc.kind === 'host') return { kind: 'host', rank: loc.index + 1 }
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
