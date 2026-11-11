// #290 (LOT-4) — helpers PURS du modèle de réglages d'ambiance côté front
// (contracts/ambiance-settings.md §2–§3). Forme JSON structurée et positive :
// { sound: { muted, countdown: { enabled, loop }, cues: { <cue>: bool } }, light: { mode } }.

/** Chemins éditables hors cues — doivent rester alignés sur `Settings` (§2). */
export const SETTINGS_FIELD_PATHS = [
  'sound.muted',
  'sound.countdown.enabled',
  'sound.countdown.loop',
  'light.mode',
  // #223 (v11.7.0, LOT-2) — effet lumière par moment : salle (`effects`) et groupe animateur (`host_effects`).
  'light.effects.chrono',
  'light.effects.calm',
  'light.effects.score',
  // Vitesse des effets (révision 3) : salle ; les groupes animateur ont `light.host_speeds.<id>.<moment>` (hostSpeedPaths).
  'light.speeds.chrono',
  'light.speeds.calm',
  'light.speeds.score',
]
// Groupes animateur : chemins `light.host_effects.<id>.<moment>` (un par groupe), construits à la volée
// (hostEffectPaths) — la liste des `id` vit dans la configuration, pas ici.
export const hostEffectPaths = (ids) => ids.flatMap(id => ['chrono', 'calm', 'score'].map(m => `light.host_effects.${id}.${m}`))
export const hostSpeedPaths = (ids) => ids.flatMap(id => ['chrono', 'calm', 'score'].map(m => `light.host_speeds.${id}.${m}`))
export const CUE_PATH_PREFIX = 'sound.cues.'

const isObj = (v) => v && typeof v === 'object' && !Array.isArray(v)

/** Lecture d'un chemin pointé. */
export function getPath(obj, path) {
  return path.split('.').reduce((o, k) => (o == null ? undefined : o[k]), obj)
}

/**
 * Copie de `partial` où `path` vaut `value` ; `value === null` retire la clé (retour à « Hérité ») et
 * élague les objets devenus vides. `path === '*'` + null ⇒ `{}` (tout remettre à Hérité).
 */
export function setPartialPath(partial, path, value) {
  if (path === '*') return {}
  const keys = path.startsWith(CUE_PATH_PREFIX)
    ? ['sound', 'cues', path.slice(CUE_PATH_PREFIX.length)]   // le nom de cue contient des « - », jamais de « . »
    : path.split('.')
  const rec = (node, i) => {
    const copy = isObj(node) ? { ...node } : {}
    const k = keys[i]
    if (i === keys.length - 1) {
      if (value === null) delete copy[k]
      else copy[k] = value
    } else {
      const child = rec(copy[k], i + 1)
      if (Object.keys(child).length === 0) delete copy[k]
      else copy[k] = child
    }
    return copy
  }
  return rec(partial, 0)
}

/** Liste des champs surchargés : [{ path, value }] (cues comptées une à une). */
export function overriddenFields(partial) {
  const out = []
  if (!isObj(partial)) return out
  const s = partial.sound
  if (isObj(s)) {
    if (s.muted !== undefined) out.push({ path: 'sound.muted', value: s.muted })
    if (isObj(s.countdown)) {
      if (s.countdown.enabled !== undefined) out.push({ path: 'sound.countdown.enabled', value: s.countdown.enabled })
      if (s.countdown.loop !== undefined) out.push({ path: 'sound.countdown.loop', value: s.countdown.loop })
    }
    if (isObj(s.cues)) Object.entries(s.cues).forEach(([cue, v]) => out.push({ path: CUE_PATH_PREFIX + cue, value: v }))
  }
  if (isObj(partial.light) && partial.light.mode !== undefined) out.push({ path: 'light.mode', value: partial.light.mode })
  if (isObj(partial.light)) {
    const room = partial.light.effects
    if (isObj(room)) {
      for (const moment of ['chrono', 'calm', 'score']) {
        if (room[moment] !== undefined) out.push({ path: `light.effects.${moment}`, value: room[moment] })
      }
    }
    const speeds = partial.light.speeds
    if (isObj(speeds)) {
      for (const moment of ['chrono', 'calm', 'score']) {
        if (speeds[moment] !== undefined) out.push({ path: `light.speeds.${moment}`, value: speeds[moment] })
      }
    }
    // Un réglage compte pour chaque (groupe animateur, moment) surchargé, effet et vitesse séparément.
    for (const branch of ['host_effects', 'host_speeds']) {
      const hosts = partial.light[branch]
      if (!isObj(hosts)) continue
      for (const [id, byMoment] of Object.entries(hosts)) {
        if (!isObj(byMoment)) continue
        for (const moment of ['chrono', 'calm', 'score']) {
          if (byMoment[moment] !== undefined) out.push({ path: `light.${branch}.${id}.${moment}`, value: byMoment[moment] })
        }
      }
    }
  }
  return out
}

export const countOverrides = (partial) => overriddenFields(partial).length

/** Libellé court d'un champ surchargé (info-bulle du badge). */
export function describeOverride({ path, value }, cueLabel = (c) => c, hostLabel = (id) => id) {
  if (path === 'sound.muted') return `Son : ${value ? 'coupé' : 'ON'}`
  if (path === 'sound.countdown.enabled') return `Décompte : ${value ? 'actif' : 'inactif'}`
  if (path === 'sound.countdown.loop') return `Décompte : ${value ? 'en boucle' : 'une fois'}`
  if (path === 'light.mode') return `Lumière : ${value}`
  const eff = /^light\.(?:(effects|speeds)\.(chrono|calm|score)|(host_effects|host_speeds)\.([a-z0-9]{1,12})\.(chrono|calm|score))$/.exec(path)
  if (eff) {
    const branch = eff[1] || eff[3]
    const moment = { chrono: 'chronomètre', calm: 'phases calmes', score: 'équipe qui marque' }[eff[2] || eff[5]]
    const val = branch.endsWith('speeds') ? `×${String(value).replace('.', ',')}` : (value || 'aucun')
    if (branch === 'effects') return `Effet (${moment}) : ${val}`
    if (branch === 'speeds') return `Vitesse (${moment}) : ${val}`
    return `${hostLabel(eff[4])}${branch === 'host_speeds' ? ' — vitesse' : ''} (${moment}) : ${val}`
  }
  if (path.startsWith(CUE_PATH_PREFIX)) return `${cueLabel(path.slice(CUE_PATH_PREFIX.length))} : ${value ? 'actif' : 'inactif'}`
  return path
}

/** Nombre de champs imposés par la manche dans `AMBIANCE_EFFECTIVE.SOURCES`. */
export function countRoundSources(sources) {
  let n = 0
  const walk = (o) => {
    if (typeof o === 'string') { if (o === 'round') n += 1 } else if (isObj(o)) Object.values(o).forEach(walk)
  }
  walk(sources)
  return n
}

/** Cues dont la manche porte un fichier propre (`sound.files`, géré côté serveur — effet immédiat, D10). */
export function fileOverrides(partial) {
  const f = isObj(partial?.sound) ? partial.sound.files : null
  return isObj(f) ? Object.keys(f) : []
}
