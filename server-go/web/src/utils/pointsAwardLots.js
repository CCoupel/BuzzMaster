// #265 — découpage et calendrier de l'animation d'attribution de points, MODULE PUR partagé par la TV
// (tous les lots) et la tablette (seulement le lot de sa propre équipe, calé sur la TV). Aucune horloge,
// aucun état, aucun DOM : mêmes entrées → mêmes lots, mêmes décalages sur tous les écrans.
// Références : maquettes docs/mockup/v11.2.1/ui/tv_points_award__multi_team.html et
// vplayer_points_award__own_team.html ; contrat contracts/game-state.md §SCORE_CHANGE.GAINS.

export const LOT_MAX = 6 // équipes par lot, au plus
export const NORMAL_LOT_MS = 3200 // durée d'un lot au rythme normal
export const ACCEL_FACTOR = 0.55 // lots accélérés : durée × 0,55
export const ACCEL_FROM_LOT = 4 // à partir du 4ᵉ lot (rang 1-based)

// Zone sûre des centres d'arrivée (en % de l'écran), découpée en 3 × 2 cases.
export const SAFE_ZONE = { x0: 8, x1: 92, y0: 12, y1: 78 }
const COLS = 3
const ROWS = 2
const JITTER_X = 1.5
const JITTER_Y = 5

function lotDurationMs(index) {
  return index + 1 >= ACCEL_FROM_LOT ? NORMAL_LOT_MS * ACCEL_FACTOR : NORMAL_LOT_MS
}

// Répartition « au plus juste » : k = ceil(n / 6) lots, les premiers lots reçoivent le reste.
function lotSizes(n) {
  const k = Math.max(1, Math.ceil(n / LOT_MAX))
  const base = Math.floor(n / k)
  const extra = n % k
  return Array.from({ length: k }, (_, i) => base + (i < extra ? 1 : 0))
}

/**
 * @param {{TEAM: string, POINTS: number}[]|null|undefined} gains SCORE_CHANGE.GAINS
 * @returns {{lots: {index: number, entries: {TEAM: string, POINTS: number}[], durationMs: number, startMs: number}[], totalMs: number}}
 * POINTS ≤ 0 exclus ; une équipe n'apparaît qu'une fois (la première entrée) ; ordre de `gains` conservé.
 */
export function planAward(gains) {
  const seen = new Set()
  const entries = []
  for (const g of Array.isArray(gains) ? gains : []) {
    if (!g || typeof g.TEAM !== 'string' || !(g.POINTS > 0) || seen.has(g.TEAM)) continue
    seen.add(g.TEAM)
    entries.push({ TEAM: g.TEAM, POINTS: g.POINTS })
  }
  if (entries.length === 0) return { lots: [], totalMs: 0 }
  const lots = []
  let pos = 0
  let startMs = 0
  lotSizes(entries.length).forEach((size, index) => {
    const durationMs = lotDurationMs(index)
    lots.push({ index, entries: entries.slice(pos, pos + size), durationMs, startMs })
    pos += size
    startMs += durationMs
  })
  return { lots, totalMs: startMs }
}

/** Début (ms, depuis le début de l'annonce) du lot contenant `team`, ou null si absente. */
export function lotOffsetMs(plan, team) {
  const lot = plan?.lots?.find((l) => l.entries.some((e) => e.TEAM === team))
  return lot ? lot.startMs : null
}

/**
 * File d'attente : une nouvelle annonce démarre à la fin de la précédente (sans chevauchement).
 * @returns {{startAtMs: number, delayMs: number, busyUntilMs: number}}
 */
export function scheduleAward(busyUntilMs, nowMs, plan) {
  const startAtMs = Math.max(busyUntilMs || 0, nowMs)
  if (!plan || plan.lots.length === 0) return { startAtMs, delayMs: startAtMs - nowMs, busyUntilMs: busyUntilMs || 0 }
  return { startAtMs, delayMs: startAtMs - nowMs, busyUntilMs: startAtMs + plan.totalMs }
}

/**
 * Centres d'arrivée (en %) de `count` cartes (≤ 6) : cases distinctes tirées au hasard dans la grille 3 × 2
 * de la zone sûre, léger décalage aléatoire (±1,5 % en x, ±5 % en y) — deux cartes ne se recouvrent jamais.
 */
export function pickArrivalSlots(count, rng = Math.random) {
  const w = (SAFE_ZONE.x1 - SAFE_ZONE.x0) / COLS
  const h = (SAFE_ZONE.y1 - SAFE_ZONE.y0) / ROWS
  const cells = []
  for (let r = 0; r < ROWS; r++) for (let c = 0; c < COLS; c++) cells.push({ x: SAFE_ZONE.x0 + w * (c + 0.5), y: SAFE_ZONE.y0 + h * (r + 0.5) })
  for (let i = cells.length - 1; i > 0; i--) {
    const j = Math.min(i, Math.floor(rng() * (i + 1)))
    ;[cells[i], cells[j]] = [cells[j], cells[i]]
  }
  return cells.slice(0, Math.max(0, Math.min(count, cells.length))).map((c) => ({
    x: c.x + (rng() * 2 - 1) * JITTER_X,
    y: c.y + (rng() * 2 - 1) * JITTER_Y,
  }))
}
