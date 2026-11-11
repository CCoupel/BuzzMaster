// #223 (v11.7.0, LOT-2) — helpers PURS des sélecteurs « effet lumière par moment ».
// Contrats : contracts/light-effects.md §6 (réglages), §7.1 (catalogue), §2.5 (groupe animateur) ;
// contracts/ambiance-settings.md §10. Maquette : ambiance_settings_panel__light_effects.

export const MOMENTS = [
  { key: 'chrono', label: 'Pendant le chronomètre', sub: 'Question chronométrée en cours' },
  { key: 'calm', label: 'Phases calmes', sub: 'Hors partie, pause, entracte' },
  { key: 'score', label: 'Quand une équipe marque', sub: 'Attribution de points' },
]

// Valeurs par défaut globales (§6) : reproduisent le comportement v11.6.0.
export const DEFAULT_EFFECTS = Object.freeze({ chrono: 'pulsation', calm: '', score: '' })
// Groupes animateur (révision du GATE LOT-1) : une entrée par `id` de groupe, blanc fixe aux trois moments par défaut.
export const DEFAULT_HOST_EFFECTS = Object.freeze({ chrono: '', calm: '', score: '' })

export const EMPTY_CATALOG = Object.freeze({ groups: 0, effects: [], errors: [], unresolved: [], dir: '' })

export const INHERIT = '__inherit__'
export const NONE = ''

// Chemin éditable d'un effet : salle `light.effects.<moment>` ; groupe animateur `light.host_effects.<id>.<moment>`
// (clé = `id` stable du groupe, contracts/light-effects.md §6).
export const effectsPath = (host, moment) => (host
  ? `light.host_effects.${host}.${moment}`
  : `light.effects.${moment}`)

// Chemin `lighting.*` de config.json correspondant (§6, table de correspondance).
export const configKeyFor = (field) => {
  const room = /^light\.effects\.(chrono|calm|score)$/.exec(field)
  if (room) return { section: 'effects', moment: room[1] }
  const host = /^light\.host_effects\.([a-z0-9]{1,12})\.(chrono|calm|score)$/.exec(field)
  return host ? { section: 'host_effects', id: host[1], moment: host[2] } : null
}

// Corps `lighting` de POST /config.json pour un changement d'effet : salle `{effects:{m:v}}`,
// groupe animateur `{host_effects:{<id>:{m:v}}}`.
export const configPatchFor = (key, value) => (key.section === 'effects'
  ? { effects: { [key.moment]: value } }
  : { host_effects: { [key.id]: { [key.moment]: value } } })

const isObj = (v) => v && typeof v === 'object' && !Array.isArray(v)

export function normalizeCatalog(data) {
  if (!isObj(data)) return EMPTY_CATALOG
  return {
    groups: Number.isFinite(data.groups) ? data.groups : 0,
    effects: Array.isArray(data.effects) ? data.effects.filter(e => isObj(e) && typeof e.id === 'string') : [],
    errors: Array.isArray(data.errors) ? data.errors : [],
    unresolved: Array.isArray(data.unresolved) ? data.unresolved : [],
    dir: typeof data.dir === 'string' ? data.dir : '',
  }
}

export const effectLabel = (e) => (e.origin === 'native' ? e.name : `${e.name} (personnalisé)`)

const byName = (a, b) => a.name.localeCompare(b.name, 'fr', { sensitivity: 'base' })

// Raison d'inapplicabilité d'un effet pour la salle ou pour le groupe animateur (un seul groupe).
export function inapplicableReason(e, { host, groups }) {
  if (host) {
    // `host_applicable` (§7.1, §4.6) fait foi ; repli sur min_groups pour un catalogue qui ne le porte pas.
    const ok = typeof e.host_applicable === 'boolean' ? e.host_applicable : (e.min_groups || 0) <= 1
    return ok ? '' : 'demande plusieurs groupes'
  }
  // Salle : `target: all` toujours applicable ; `target: groups` si N >= min_groups (le serveur le calcule dans `applicable`).
  const ok = typeof e.applicable === 'boolean' ? e.applicable : (e.target === 'all' || groups >= (e.min_groups || 0))
  if (ok) return ''
  if (e.reason) return e.reason
  return `demande ${e.min_groups} groupe${e.min_groups > 1 ? 's' : ''}, ${groups} configuré${groups > 1 ? 's' : ''}`
}

// Liste d'un moment : « Aucun effet » (ou « Blanc fixe » pour le groupe animateur), puis les effets du
// catalogue qui déclarent ce moment — natifs d'abord, personnalisés ensuite par ordre alphabétique.
// Un effet inapplicable reste visible, grisé, avec sa raison. Une valeur sélectionnée absente du catalogue
// est ajoutée « (introuvable) ». `defaultValue` marque l'option « (actuel) » (mode global).
export function buildOptions(catalog, moment, { host = false, selected = null, defaultValue = null } = {}) {
  const cat = catalog || EMPTY_CATALOG
  const wanted = cat.effects.filter(e => Array.isArray(e.moments) && e.moments.includes(moment))
  const natives = wanted.filter(e => e.origin === 'native')
  const others = wanted.filter(e => e.origin !== 'native').sort(byName)
  const noneLabel = host ? 'Blanc fixe' : 'Aucun effet'
  const options = [{ value: NONE, label: noneLabel, disabled: false, reason: '' }]
  for (const e of [...natives, ...others]) {
    const reason = inapplicableReason(e, { host, groups: cat.groups })
    options.push({ value: e.id, label: effectLabel(e), disabled: !!reason, reason })
  }
  if (selected && selected !== INHERIT && !options.some(o => o.value === selected)) {
    options.push({ value: selected, label: `${selected} (introuvable)`, disabled: false, reason: '', missing: true })
  }
  return options.map(o => {
    const label = defaultValue !== null && o.value === defaultValue ? `${o.label} (actuel)` : o.label
    return { ...o, label, text: o.disabled ? `${label} — ${o.reason}` : label }
  })
}

export const optionLabel = (options, value) => options.find(o => o.value === value)?.label.replace(/ \(actuel\)$/, '') ?? value

// Avertissement affiché sous le sélecteur salle : effet inapplicable (repli sur la pulsation) ou introuvable.
export function effectWarning(catalog, moment, value, { host = false } = {}) {
  if (!value) return null
  const cat = catalog || EMPTY_CATALOG
  const e = cat.effects.find(x => x.id === value)
  if (!e || !Array.isArray(e.moments) || !e.moments.includes(moment)) {
    return { kind: 'missing', text: 'Le fichier de cet effet est absent ou invalide : aucun effet n\'est joué.' }
  }
  const reason = inapplicableReason(e, { host, groups: cat.groups })
  if (!reason) return null
  if (host) return { kind: 'inapplicable', text: 'Cet effet demande plusieurs groupes : le groupe animateur reste en blanc fixe.' }
  return {
    kind: 'inapplicable',
    text: `Cet effet demande ${e.min_groups} groupe${e.min_groups > 1 ? 's' : ''} et ${cat.groups} ${cat.groups > 1 ? 'sont configurés' : 'est configuré'} : ${moment === 'chrono' ? 'la pulsation est jouée à la place' : 'aucun effet n\'est joué'}.`,
  }
}

// Valeur globale effective d'un moment depuis la section `lighting` de config.json.
// `host` : `id` d'un groupe animateur (falsy = salle).
export function globalEffect(lighting, host, moment) {
  const src = host ? (isObj(lighting?.host_effects) ? lighting.host_effects[host] : undefined) : lighting?.effects
  const dflt = host ? DEFAULT_HOST_EFFECTS : DEFAULT_EFFECTS
  const v = isObj(src) ? src[moment] : undefined
  return typeof v === 'string' ? v : dflt[moment]
}

// Groupes animateur configurés, dans l'ordre de la page Groupes : [{id, name, rank, empty}] (rang k = A1, A2…).
// Un groupe sans `id` (jamais enregistré) est ignoré : ses effets ne sont réglables qu'après enregistrement.
export function findHostGroups(groups) {
  if (!Array.isArray(groups)) return []
  const out = []
  let rank = 0
  for (const g of groups) {
    if (!g || g.role !== 'animateur') continue
    rank += 1
    if (typeof g.id !== 'string' || !g.id) continue
    out.push({
      id: g.id,
      name: typeof g.name === 'string' ? g.name : '',
      rank,
      empty: !Array.isArray(g.lights) || g.lights.length === 0,
    })
  }
  return out
}
