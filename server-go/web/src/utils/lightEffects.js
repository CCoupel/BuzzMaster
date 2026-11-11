// #223 (v11.7.0, LOT-2) — helpers PURS des sélecteurs « effet et vitesse lumière par moment et par zone » (révision 4).
// Contrats : contracts/light-effects.md §6 (réglages), §7.1 (catalogue par zone), §7.4 (essai), §2.5 (zones) ;
// contracts/ambiance-settings.md §10. Maquette : ambiance_settings_panel__light_effects_zones.

export const MOMENTS = [
  { key: 'chrono', label: 'Pendant le chronomètre', sub: 'Question chronométrée en cours' },
  { key: 'calm', label: 'Phases calmes', sub: 'Hors partie, pause, entracte' },
  { key: 'score', label: 'Quand une équipe marque', sub: 'Attribution de points' },
]

// Trois zones : Ambiance (la salle), Animateur, Public.
export const ZONES = Object.freeze(['ambiance', 'animateur', 'public'])
export const ZONE_LABEL = Object.freeze({ ambiance: 'Ambiance', animateur: 'Animateur', public: 'Public' })
export const ZONE_ICON = Object.freeze({ ambiance: '💡', animateur: '🎤', public: '👥' })
export const ZONE_ROLE = Object.freeze({ ambiance: 'salle', animateur: 'animateur', public: 'public' })

// Valeurs par défaut globales (§6) : `pulsation` pour ambiance.chrono (comportement v11.6.0), `""` partout ailleurs.
export const defaultEffect = (zone, moment) => (zone === 'ambiance' && moment === 'chrono' ? 'pulsation' : '')

export const EMPTY_GROUPS = Object.freeze({ ambiance: 0, animateur: 0, public: 0 })
export const EMPTY_CATALOG = Object.freeze({ groups: EMPTY_GROUPS, effects: [], errors: [], unresolved: [], dir: '' })

export const INHERIT = '__inherit__'
export const NONE = ''

// Chemins éditables (`light.*` du modèle de réglages) : `light.effects.<zone>.<moment>`, `light.speeds.<zone>.<moment>`.
export const effectsPath = (zone, moment) => `light.effects.${zone}.${moment}`

// Vitesse des effets : multiplicateur par zone et moment.
export const SPEEDS = Object.freeze([0.5, 0.75, 1, 1.5, 2])
export const DEFAULT_SPEED = 1
export const MIN_TICK_MS = 250
export const speedPath = (zone, moment) => `light.speeds.${zone}.${moment}`
export const formatSpeed = (v) => `×${String(v).replace('.', ',')}`
// Vitesse « plafonnée » : le tick (`tick_ms` du catalogue) ÷ vitesse tomberait sous 250 ms.
export const isSpeedCapped = (tickMs, speed) => Number.isFinite(tickMs) && tickMs > 0 && tickMs / speed < MIN_TICK_MS

// Tous les chemins éditables d'effet et de vitesse (parité avec ambianceSettings.SETTINGS_FIELD_PATHS).
export const effectFieldPaths = () => ZONES.flatMap(z => MOMENTS.flatMap(m => [effectsPath(z, m.key), speedPath(z, m.key)]))

// Chemin `lighting.*` de config.json correspondant (§6, table de correspondance).
export const configKeyFor = (field) => {
  const m = /^light\.(effects|speeds)\.(ambiance|animateur|public)\.(chrono|calm|score)$/.exec(field)
  return m ? { section: m[1], zone: m[2], moment: m[3] } : null
}

// Corps `lighting` de POST /config.json pour un changement d'effet ou de vitesse : `{effects:{<zone>:{<moment>:v}}}`.
export const configPatchFor = (key, value) => ({ [key.section]: { [key.zone]: { [key.moment]: value } } })

const isObj = (v) => v && typeof v === 'object' && !Array.isArray(v)
const count = (v) => (Number.isFinite(v) ? v : 0)

// Catalogue (GET /api/lighting/effects, §7.1) : `groups` par zone, `applicable` par zone, `tick_ms`. Tolère un
// catalogue antérieur (`groups` numérique ⇒ zone ambiance ; `applicable` booléen ⇒ toutes les zones).
export function normalizeCatalog(data) {
  if (!isObj(data)) return EMPTY_CATALOG
  const g = data.groups
  const groups = isObj(g)
    ? { ambiance: count(g.ambiance), animateur: count(g.animateur), public: count(g.public) }
    : { ...EMPTY_GROUPS, ambiance: count(g) }
  return {
    groups,
    effects: Array.isArray(data.effects) ? data.effects.filter(e => isObj(e) && typeof e.id === 'string') : [],
    errors: Array.isArray(data.errors) ? data.errors : [],
    unresolved: Array.isArray(data.unresolved) ? data.unresolved : [],
    dir: typeof data.dir === 'string' ? data.dir : '',
  }
}

export const effectLabel = (e) => (e.origin === 'native' ? e.name : `${e.name} (personnalisé)`)

const byName = (a, b) => a.name.localeCompare(b.name, 'fr', { sensitivity: 'base' })

// Applicabilité d'un effet à une zone : table `applicable` par zone ; repli (catalogue sans table) : N ≥ min_groups.
function isApplicable(e, zone, groups) {
  if (isObj(e.applicable)) return e.applicable[zone] === true
  if (typeof e.applicable === 'boolean') return e.applicable
  return groups[zone] >= (e.min_groups || 0)
}

// Raison d'inapplicabilité d'un effet pour une zone, déduite de `min_groups` et de `groups` (le serveur ne la renvoie
// plus) : « demande 5 groupes, 2 dans cette zone ». '' = applicable.
export function inapplicableReason(e, zone, groups = EMPTY_GROUPS) {
  if (isApplicable(e, zone, groups)) return ''
  const n = groups[zone] ?? 0
  const min = e.min_groups || 0
  return `demande ${min} groupe${min > 1 ? 's' : ''}, ${n} dans cette zone`
}

const noneLabelOf = (zone) => (zone === 'animateur' ? 'Aucun effet (blanc fixe)' : 'Aucun effet')

// Liste d'un moment pour une zone : « Aucun effet » (« Aucun effet (blanc fixe) » pour la zone Animateur), puis les
// effets du catalogue qui déclarent ce moment — natifs d'abord, personnalisés ensuite par ordre alphabétique. Un effet
// inapplicable à la zone reste visible, grisé, avec sa raison. Une valeur sélectionnée absente du catalogue est ajoutée
// « (introuvable) ». `defaultValue` marque l'option « (par défaut) » (mode global, zones Ambiance et Public).
export function buildOptions(catalog, moment, { zone = 'ambiance', selected = null, defaultValue = null } = {}) {
  const cat = catalog || EMPTY_CATALOG
  const wanted = cat.effects.filter(e => Array.isArray(e.moments) && e.moments.includes(moment))
  const natives = wanted.filter(e => e.origin === 'native')
  const others = wanted.filter(e => e.origin !== 'native').sort(byName)
  const options = [{ value: NONE, label: noneLabelOf(zone), disabled: false, reason: '' }]
  for (const e of [...natives, ...others]) {
    const reason = inapplicableReason(e, zone, cat.groups)
    options.push({ value: e.id, label: effectLabel(e), disabled: !!reason, reason })
  }
  if (selected && selected !== INHERIT && !options.some(o => o.value === selected)) {
    options.push({ value: selected, label: `${selected} (introuvable)`, disabled: false, reason: '', missing: true })
  }
  const marks = defaultValue !== null && zone !== 'animateur'
  return options.map(o => {
    const label = marks && o.value === defaultValue ? `${o.label} (par défaut)` : o.label
    return { ...o, label, text: o.disabled ? `${label} — ${o.reason}` : label }
  })
}

export const optionLabel = (options, value) => options.find(o => o.value === value)?.label.replace(/ \(par défaut\)$/, '') ?? value

// Ce que la zone fait quand son effet n'est pas joué (introuvable ou inapplicable).
const FALLBACK = {
  ambiance: "aucun effet n'est joué",
  animateur: 'la zone reste en blanc fixe',
  public: "rien n'est envoyé aux lampes du public",
}

// Avertissement affiché sous un sélecteur : effet inapplicable à la zone ou introuvable.
export function effectWarning(catalog, moment, value, { zone = 'ambiance' } = {}) {
  if (!value) return null
  const cat = catalog || EMPTY_CATALOG
  const e = cat.effects.find(x => x.id === value)
  if (!e || !Array.isArray(e.moments) || !e.moments.includes(moment)) {
    return { kind: 'missing', text: `Le fichier de cet effet est absent ou invalide : ${FALLBACK[zone]}.` }
  }
  if (!inapplicableReason(e, zone, cat.groups)) return null
  const min = e.min_groups || 0
  const n = cat.groups[zone] ?? 0
  const fallback = zone === 'ambiance' && moment === 'chrono' ? 'la pulsation est jouée à la place' : FALLBACK[zone]
  return {
    kind: 'inapplicable',
    text: `Cet effet demande ${min} groupe${min > 1 ? 's' : ''} et la zone en a ${n} : ${fallback}.`,
  }
}

// Valeur globale effective (config.json `lighting.effects.<zone>.<moment>`). Lecture tolérante d'une forme antérieure
// à plat (`lighting.effects.<moment>`), lue comme celle de la zone ambiance (§6).
export function globalEffect(lighting, zone, moment) {
  const src = lighting?.effects
  let v = isObj(src) && isObj(src[zone]) ? src[zone][moment] : undefined
  if (v === undefined && zone === 'ambiance' && isObj(src)) v = src[moment]
  return typeof v === 'string' ? v : defaultEffect(zone, moment)
}

export function globalSpeed(lighting, zone, moment) {
  const src = lighting?.speeds
  let v = isObj(src) && isObj(src[zone]) ? src[zone][moment] : undefined
  if (v === undefined && zone === 'ambiance' && isObj(src)) v = src[moment]
  return SPEEDS.includes(v) ? v : DEFAULT_SPEED
}

// Nombre de groupes NON VIDES par zone d'après `lighting.groups` : { ambiance, animateur, public } (contrat §2.1 :
// N = groupes non vides ; une zone Animateur ou Public sans groupe non vide « n'existe pas » : pas de bloc d'effets).
export function zoneGroupCounts(groups) {
  const out = { ambiance: 0, animateur: 0, public: 0 }
  if (!Array.isArray(groups)) return out
  for (const g of groups) {
    if (!g || typeof g !== 'object' || !Array.isArray(g.lights) || g.lights.length === 0) continue
    out[g.role === 'animateur' ? 'animateur' : g.role === 'public' ? 'public' : 'ambiance'] += 1
  }
  return out
}

// Zones dont le bloc est affiché : Ambiance toujours, Animateur et Public si elles ont au moins un groupe.
export const visibleZones = (counts) => ZONES.filter(z => z === 'ambiance' || (counts?.[z] ?? 0) > 0)

// Une question est en cours : phase de jeu autre que STOPPED / NEW_GAME / ENROLL (§7.4, refus 409 `game`).
// Phase inconnue (état de jeu pas encore reçu) : on ne bloque pas, le serveur reste juge.
const IDLE_PHASES = ['STOPPED', 'NEW_GAME', 'ENROLL']
export const isQuestionRunning = (phase) => typeof phase === 'string' && phase !== '' && !IDLE_PHASES.includes(phase)

// Événement fenêtre émis quand un essai / test d'effet démarre : le dernier lancé remplace l'autre (bouton « Tester » des
// réglages ↔ « Essayer sur la salle » du catalogue). detail.source : 'panel' | 'catalogue'.
export const TRY_EVENT = 'buzzcontrol:effect-try'

// Durée de REPLI d'un test (§7.4) si la réponse ne porte pas `duration_ms` : le serveur fixe la durée (5 s pour un effet
// qui ne déclare que « score », 10 s sinon) et la renvoie ; l'interface ne l'envoie plus. Messages des refus 409 (`reason`).
export const TRY_SECONDS = 10
export const TRY_REFUSALS = {
  disabled: "L'éclairage d'ambiance est désactivé, ou le pont a refusé la clé.",
  game: 'Une question est en cours : le test est refusé.',
  not_applicable: "Cet effet n'est pas applicable à cette zone.",
  mode: "L'éclairage est forcé sur ON / OFF ou le Flash est actif : le test de la zone Ambiance est refusé.",
}
