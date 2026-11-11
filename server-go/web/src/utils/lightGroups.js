// #225 (v11.7.0, LOT-1) — fonctions pures de la page « Groupes » de l'onglet Lumière.
// Contrat : contracts/light-effects.md §2.1 (configuration), §2.4 (essai), §2.5 (les trois zones, révision 4).
// Maquette : docs/mockup/v11.7.0/ui/ambiance_page__light_groups_zones.html.
//
// Modèle d'édition (« brouillon ») : { ambiance: [{id, lights}], animateur: [{id, lights}], public: [{id, lights}] }
//   - Trois zones ; dans chacune, les groupes sont numérotés g1, g2… d'après leur rang (vides compris), 12 au plus.
//   - Aucun nom : un groupe est son numéro (« Groupe 1 »).
//   - `id` : identifiant stable attribué par le serveur (vide tant que le groupe n'est pas enregistré), renvoyé tel quel.
// Charge utile `lighting.groups` : zone ambiance (`role: "salle"`), puis animateur, puis public.
// Toutes les fonctions retournent un nouveau brouillon (immuable).

export const ZONES = Object.freeze(['ambiance', 'animateur', 'public'])
export const ZONE_ROLE = Object.freeze({ ambiance: 'salle', animateur: 'animateur', public: 'public' })
export const ZONE_LABEL = Object.freeze({ ambiance: 'Ambiance', animateur: 'Animateur', public: 'Public' })
export const ZONE_ICON = Object.freeze({ ambiance: '💡', animateur: '🎤', public: '👥' })
export const MAX_GROUPS_PER_ZONE = 12

export const EMPTY_DRAFT = Object.freeze({ ambiance: [], animateur: [], public: [] })

const asStrings = (v) => (Array.isArray(v) ? v.filter(x => typeof x === 'string') : [])
const asId = (v) => (typeof v === 'string' ? v : '')
const zoneOfRole = (role) => (role === 'animateur' ? 'animateur' : role === 'public' ? 'public' : 'ambiance')

// Configuration enregistrée (`lighting.groups`) -> brouillon. Tolérant : entrée absente, non tableau, `name` /
// `near_teams` d'un config.json antérieur (ignorés).
export function draftFromConfig(groups) {
  const draft = { ambiance: [], animateur: [], public: [] }
  if (!Array.isArray(groups)) return draft
  for (const g of groups) {
    if (!g || typeof g !== 'object') continue
    draft[zoneOfRole(g.role)].push({ id: asId(g.id), lights: asStrings(g.lights) })
  }
  return draft
}

// Brouillon -> `lighting.groups` (remplacement complet). `id` omis tant qu'il est vide (le serveur l'attribue).
export function draftToPayload(draft) {
  return ZONES.flatMap(zone => draft[zone].map(g => {
    const entry = { role: ZONE_ROLE[zone], lights: [...g.lights] }
    return g.id ? { id: g.id, ...entry } : entry
  }))
}

export const draftsEqual = (a, b) => JSON.stringify(draftToPayload(a)) === JSON.stringify(draftToPayload(b))
export const groupCount = (draft) => ZONES.reduce((n, z) => n + draft[z].length, 0)

const mapZones = (draft, fn) => Object.fromEntries(ZONES.map(z => [z, fn(draft[z], z)]))

// Retire du brouillon les ampoules qui ne sont plus de rôle général (ou n'existent plus).
export function pruneToGeneral(draft, generalNames) {
  const ok = new Set(generalNames)
  return mapZones(draft, gs => gs.map(g => ({ ...g, lights: g.lights.filter(n => ok.has(n)) })))
}

// Ampoules générales rangées dans aucun groupe (donc de la zone ambiance, hors groupe), dans l'ordre de `generalNames`.
export function unassignedLights(draft, generalNames) {
  const used = new Set(ZONES.flatMap(z => draft[z].flatMap(g => g.lights)))
  return generalNames.filter(n => !used.has(n))
}

// Où se trouve une ampoule : {kind:'group', zone, index} | {kind:'pool'}.
export function locateLight(draft, name) {
  for (const zone of ZONES) {
    const index = draft[zone].findIndex(g => g.lights.includes(name))
    if (index >= 0) return { kind: 'group', zone, index }
  }
  return { kind: 'pool' }
}

// Déplace une ampoule vers `target` : {kind:'pool'} | {kind:'group', zone, index}. Une ampoule n'est jamais que dans
// un groupe, d'une seule zone. Cible inconnue : brouillon inchangé.
export function moveLight(draft, name, target) {
  const base = mapZones(draft, gs => gs.map(g => (g.lights.includes(name) ? { ...g, lights: g.lights.filter(n => n !== name) } : g)))
  if (target.kind === 'pool') return base
  const list = base[target.zone]
  if (!list || target.index < 0 || target.index >= list.length) return draft
  return { ...base, [target.zone]: list.map((g, i) => (i === target.index ? { ...g, lights: [...g.lights, name] } : g)) }
}

// Création (12 groupes par zone), suppression et réordonnancement dans une zone (l'`id` suit le groupe).
export function addGroup(draft, zone) {
  if (!ZONES.includes(zone) || draft[zone].length >= MAX_GROUPS_PER_ZONE) return draft
  return { ...draft, [zone]: [...draft[zone], { id: '', lights: [] }] }
}

export function removeGroup(draft, zone, index) {
  if (!ZONES.includes(zone) || index < 0 || index >= draft[zone].length) return draft
  return { ...draft, [zone]: draft[zone].filter((_, i) => i !== index) }
}

export function moveGroup(draft, zone, from, to) {
  const list = draft[zone]
  if (!list || from === to || from < 0 || from >= list.length || to < 0 || to >= list.length) return draft
  const next = [...list]
  const [item] = next.splice(from, 1)
  next.splice(to, 0, item)
  return { ...draft, [zone]: next }
}

// Erreurs de validation (plafond par zone, alignées sur le refus 400 du serveur). Retourne { global: [..] }.
export function validateDraft(draft) {
  const errors = { global: [] }
  for (const zone of ZONES) {
    if (draft[zone].length > MAX_GROUPS_PER_ZONE) errors.global.push(`${MAX_GROUPS_PER_ZONE} groupes au plus dans la zone ${ZONE_LABEL[zone]}.`)
  }
  return errors
}

export const isDraftValid = (draft) => validateDraft(draft).global.length === 0

// Libellés : un groupe est son rang dans sa zone.
export const groupName = (index) => `Groupe ${index + 1}`

// Référence d'essai (POST /api/lighting/groups/test) : n (ambiance), "host<k>" (animateur), "public<k>" (public).
export const testRef = (zone, index) => (zone === 'ambiance' ? index + 1 : `${zone === 'animateur' ? 'host' : 'public'}${index + 1}`)

// L'ordre des groupes déjà enregistrés a-t-il changé dans une zone ? (les numéros g1, g2… désignés par les fichiers
// d'effets changent alors de groupe) — compare la suite des `id` communs au brouillon et à l'enregistré.
export function orderChanged(draft, saved) {
  return ZONES.some(zone => {
    const before = saved[zone].map(g => g.id).filter(Boolean)
    const common = new Set(before)
    const after = draft[zone].map(g => g.id).filter(id => common.has(id))
    const kept = before.filter(id => new Set(after).has(id))
    return after.some((id, i) => id !== kept[i])
  })
}

// Pastille d'une ampoule vue depuis « Pont et ampoules » : {zone, n} | null.
export function groupLabelForLight(groups, lightName) {
  const loc = locateLight(draftFromConfig(groups), lightName)
  return loc.kind === 'group' ? { zone: loc.zone, n: loc.index + 1 } : null
}
export const groupPillText = (label) => (label.zone === 'ambiance'
  ? `Ambiance · g${label.n}`
  : `${ZONE_ICON[label.zone]} ${ZONE_LABEL[label.zone]} · g${label.n}`)

// Message d'avertissement de la page de tête quand un changement de rôle retire une ampoule de son groupe.
export const teamRoleWarning = (names) =>
  names.length === 0 ? '' : `${names.join(', ')} ${names.length > 1 ? 'seront retirées' : 'sera retirée'} de leur groupe à l'enregistrement.`

// Résumé sous le tableau : ampoules hors groupe et groupes d'ambiance vides.
export function summarize(draft, generalNames) {
  const free = unassignedLights(draft, generalNames).length
  const empty = draft.ambiance.filter(g => g.lights.length === 0).length
  const active = draft.ambiance.length - empty
  return { free, empty, active }
}

// Pas de la séquence d'essai (contracts/light-effects.md §2.4) : 6 pas d'une seconde.
export const TEST_STEPS = ['éteint', 'blanc 50 %', 'blanc 100 %', 'rouge', 'vert', 'bleu']
export const testStepAt = (elapsedMs, stepMs = 1000) => Math.min(TEST_STEPS.length - 1, Math.max(0, Math.floor(elapsedMs / stepMs)))
