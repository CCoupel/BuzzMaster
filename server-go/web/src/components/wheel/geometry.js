// Géométrie pure de la roue générique (contracts/wheel-component.md §3/§4).
// Aucune dépendance DOM : testée seule (geometry.test.js).

/** Nombre de répétitions du ruban en layout `strip` (≥ 2 + ⌈amplitudeMax/n⌉ + 1, amplitude ≤ 4n). */
export const REP = 7
/** Amplitude maximale d'un mouvement, en multiples de n (contrat §4). */
export const MAX_AMPLITUDE_N = 4

/** Position continue (en « segments ») d'un index entier. */
export function indexToOffset(index, n) {
  if (!(n > 0)) return 0
  return offsetToIndex(index, n)
}

/** Ramène toute position continue dans [0, n) (modulo sûr pour les négatifs et les très grandes valeurs). */
export function offsetToIndex(offset, n) {
  if (!(n > 0) || !Number.isFinite(offset)) return 0
  const r = ((offset % n) + n) % n
  return r >= n ? 0 : r
}

/** Distance vers l'avant (sens de lecture, index croissant) de `from` à `to`, dans [0, n). */
export function forwardDistance(from, to, n) {
  return offsetToIndex(to - from, n)
}

/** Angle (degrés) du centre du secteur `index` en layout `arc` : -90° (sommet) + index × 360/n. */
export function angleFor(index, n) {
  return -90 + (index * 360) / n
}

/** Ease-out cubique, identique à la bascule courte CSS (cubic-bezier(.215,.61,.355,1) ≈ easeOutCubic). */
export function easeOutCubic(p) {
  const c = Math.min(1, Math.max(0, p))
  return 1 - Math.pow(1 - c, 3)
}

/**
 * Position à l'instant `t` (horloge serveur, ms) d'un mouvement long {from, to, turns, durationMs, startedAt}.
 * Réservé v12.0 ; `n` (optionnel) convertit `turns` en segments.
 */
export function positionAt(t, motion, n = 0) {
  const { from, to, turns = 0, durationMs, startedAt } = motion
  const p = durationMs > 0 ? (t - startedAt) / durationMs : 1
  const total = (to - from) + turns * n
  return from + total * easeOutCubic(p)
}

// ---------------------------------------------------------------------------
// Mouvement long v12.0.0 (contracts/wheel-component.md §5, révisé) — sans horloge serveur.
// Fonctions pures : la position est TOUJOURS renvoyée normalisée dans [0, n).
// ---------------------------------------------------------------------------

/** Nombre maximal de tours complets ajoutés à un freinage : amplitude totale ≤ MAX_AMPLITUDE_N × n (§4). */
export const MAX_LAND_TURNS = MAX_AMPLITUDE_N - 1

/** `spin` : position après `elapsedMs` de défilement continu (un tour en `periodMs`), depuis `from`. */
export function spinPosition(from, elapsedMs, periodMs, n) {
  if (!(n > 0)) return 0
  if (!(periodMs > 0)) return offsetToIndex(from, n)
  return offsetToIndex(from + (Math.max(0, elapsedMs) / periodMs) * n, n)
}

/**
 * `land` : amplitude totale (en segments) d'un freinage de `from` jusqu'à l'index `to` — distance vers l'avant
 * + `minTurns` tours complets (plafonné pour rester ≤ 4n). Toujours dans [0, 4n].
 */
export function landTotal(from, to, n, minTurns = 1) {
  if (!(n > 0)) return 0
  const turns = Math.min(MAX_LAND_TURNS, Math.max(0, Math.floor(Number.isFinite(minTurns) ? minTurns : 0)))
  return forwardDistance(from, to, n) + turns * n
}

/** `land` : position normalisée à l'avancement `p` ∈ [0, 1] (ease-out cubique) ; `p ≥ 1` ⇒ exactement `to`. */
export function landPosition(from, to, total, p, n) {
  if (!(n > 0)) return 0
  if (p >= 1) return offsetToIndex(to, n)
  return offsetToIndex(from + total * easeOutCubic(p), n)
}

// ---------------------------------------------------------------------------
// ROUE (#144) : lancement avec montée en vitesse, puis freinage à vitesse continue (correctifs 12.0.0.3 / 12.0.0.4).
// Vitesses en segments/ms. Sans `accelMs` / `periodMs`, `spin` et `land` gardent leurs profils d'origine.
// ---------------------------------------------------------------------------

/** Durée (ms) de la montée : pleine vitesse atteinte après `accelTurns` tours à accélération constante = 2·accelTurns·periodMs. */
export function accelDurationMs(periodMs, accelTurns) {
  return periodMs > 0 && accelTurns > 0 && Number.isFinite(accelTurns) ? 2 * accelTurns * periodMs : 0
}

/** Vitesse (segments/ms) du `spin` à l'instant `elapsedMs` : montée linéaire 0 → n/periodMs sur `accelTurns` tours, puis croisière. */
export function spinSpeed(elapsedMs, periodMs, n, accelTurns = 0) {
  if (!(n > 0) || !(periodMs > 0)) return 0
  const v0 = n / periodMs
  const accelMs = accelDurationMs(periodMs, accelTurns)
  if (!(accelMs > 0)) return v0
  return v0 * Math.min(1, Math.max(0, elapsedMs) / accelMs)
}

/** `spin` avec montée : position après `elapsedMs` depuis l'arrêt (accélération constante sur `accelTurns` tours, puis croisière). */
export function spinRampPosition(from, elapsedMs, periodMs, n, accelTurns) {
  if (!(n > 0)) return 0
  if (!(periodMs > 0)) return offsetToIndex(from, n)
  const accelMs = accelDurationMs(periodMs, accelTurns)
  if (!(accelMs > 0)) return spinPosition(from, elapsedMs, periodMs, n)
  const v0 = n / periodMs
  const t = Math.max(0, elapsedMs)
  const dist = t < accelMs ? (v0 * t * t) / (2 * accelMs) : v0 * (t - accelMs / 2)
  return offsetToIndex(from + dist, n)
}

/** Durée minimale (ms) de la décélération — en deçà elle serait perçue comme un arrêt sec. */
export const BRAKE_MIN_MS = 500

/**
 * Plan de freinage depuis la croisière `vc = n/periodMs` (le serveur n'accepte aucun arrêt avant la pleine vitesse,
 * roue.md §5.1 : jamais de freinage depuis la montée) : `{ D, vs, a, t1, vp, tc, tb }` = croisière à vc (tc) puis décélération
 * constante jusqu'à l'arrêt (tb) ; D = amplitude totale (segments), ≡ distance vers `to` modulo n ; `t1 = 0`, `a = 0`.
 * `null` si la fenêtre `durationMs` est trop courte pour la vitesse (repli de l'appelant sur le profil sans `periodMs`).
 * Cherche d'abord un atterrissage exact à `durationMs` (tc + tb = durationMs, tb ≥ BRAKE_MIN_MS) avec 0 à MAX_LAND_TURNS tours
 * ajoutés ; sinon une décélération constante pure qui s'arrête avant `durationMs`.
 */
export function brakePlan(from, to, n, durationMs, periodMs) {
  if (!(n > 0) || !(durationMs > 0) || !(periodMs > 0)) return null
  const vc = n / periodMs
  const d = forwardDistance(from, to, n)
  const cands = []
  for (let k = 0; k <= MAX_LAND_TURNS; k++) {
    const D = d + k * n
    if (D >= 1) cands.push(D) // moins d'un segment : pas de freinage perceptible, on ajoute un tour
  }
  // A : atterrissage exact à durationMs (croisière puis décélération), tb ≥ BRAKE_MIN_MS, tc ≥ 0.
  for (const D of cands) {
    const tb = 2 * (durationMs - D / vc)
    if (tb >= BRAKE_MIN_MS - 1e-6 && tb <= (2 * D) / vc + 1e-9) return { D, vs: vc, a: 0, t1: 0, vp: vc, tc: durationMs - tb, tb }
  }
  // B : décélération constante pure (vitesse continue), arrêt avant la fin de la fenêtre.
  let best = null
  for (const D of cands) if ((2 * D) / vc <= durationMs) best = D
  if (best != null) return { D: best, vs: vc, a: 0, t1: 0, vp: vc, tc: 0, tb: (2 * best) / vc }
  return null
}

/** Position normalisée à l'instant `t` (ms depuis le début du freinage) selon un `brakePlan` ; `to` exact à la fin. */
export function brakePosition(plan, from, to, t, n) {
  const { vp, tc, tb } = plan
  if (t >= tc + tb) return offsetToIndex(to, n)
  const tt = Math.max(0, t)
  if (tt <= tc) return offsetToIndex(from + vp * tt, n)
  const u = (tt - tc) / tb
  return offsetToIndex(from + vp * tc + vp * tb * (u - (u * u) / 2), n)
}
