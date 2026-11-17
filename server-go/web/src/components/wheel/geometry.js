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
// Vitesses en segments/ms. Sans `rampMs` / `periodMs`, `spin` et `land` gardent leurs profils d'origine.
// ---------------------------------------------------------------------------

/** Vitesse (segments/ms) du `spin` à l'instant `elapsedMs` : montée linéaire 0 → n/periodMs sur `rampMs`, puis croisière. */
export function spinSpeed(elapsedMs, periodMs, n, rampMs = 0) {
  if (!(n > 0) || !(periodMs > 0)) return 0
  const v0 = n / periodMs
  if (!(rampMs > 0)) return v0
  return v0 * Math.min(1, Math.max(0, elapsedMs) / rampMs)
}

/** `spin` avec montée : position après `elapsedMs` depuis l'arrêt (accélération constante sur `rampMs`, puis croisière). */
export function spinRampPosition(from, elapsedMs, periodMs, n, rampMs) {
  if (!(n > 0)) return 0
  if (!(periodMs > 0)) return offsetToIndex(from, n)
  if (!(rampMs > 0)) return spinPosition(from, elapsedMs, periodMs, n)
  const v0 = n / periodMs
  const t = Math.max(0, elapsedMs)
  const dist = t < rampMs ? (v0 * t * t) / (2 * rampMs) : v0 * (t - rampMs / 2)
  return offsetToIndex(from + dist, n)
}

/** Durée minimale (ms) de la décélération — en deçà elle serait perçue comme un arrêt sec. */
export const BRAKE_MIN_MS = 500

const EPS = 1e-9

/**
 * Plan de freinage continu depuis la vitesse courante `vStart` (segments/ms, 0 ≤ vStart ≤ v0 = n/periodMs ; défaut v0) :
 * `{ D, vs, a, t1, vp, tc, tb }` = montée à l'accélération `a` de vs à vp (t1), croisière à vp (tc), décélération constante
 * jusqu'à l'arrêt (tb) ; D = amplitude totale (segments), toujours ≡ distance vers `to` modulo n. `null` si la fenêtre
 * `durationMs` est trop courte (l'appelant retombe alors sur l'ancien profil).
 * Ordre : (A) croisière à vs puis décélération, atterrissage exact à durationMs (tb ≥ BRAKE_MIN_MS) ; (A') si la roue
 * monte encore (`rampMs`), poursuite de la montée jusqu'à vp ≤ v0 puis décélération, exact à durationMs ; (B) décélération
 * pure, arrêt avant durationMs. 0 à MAX_LAND_TURNS tours ajoutés (A/A' : premier k valide).
 */
export function brakePlan(from, to, n, durationMs, periodMs, vStart, rampMs = 0) {
  if (!(n > 0) || !(durationMs > 0) || !(periodMs > 0)) return null
  const v0 = n / periodMs
  const vs = Number.isFinite(vStart) ? Math.min(v0, Math.max(0, vStart)) : v0
  const a = rampMs > 0 ? v0 / rampMs : 0
  const d = forwardDistance(from, to, n)
  const cands = []
  for (let k = 0; k <= MAX_LAND_TURNS; k++) {
    const D = d + k * n
    if (D >= 1) cands.push(D) // moins d'un segment : pas de freinage perceptible, on ajoute un tour
  }
  if (vs > EPS) {
    for (const D of cands) {
      // A : croisière à vs puis décélération
      const tb = 2 * (durationMs - D / vs)
      if (tb >= BRAKE_MIN_MS - 1e-6 && tb <= (2 * D) / vs + EPS) return { D, vs, a, t1: 0, vp: vs, tc: durationMs - tb, tb }
      // A' : poursuite de la montée (vs < v0, a > 0) jusqu'à vp, puis décélération ; vp = (D + vs²/2a) / (dur/2 + vs/2a)
      if (a > 0 && vs < v0 - EPS) {
        const vp = (D + (vs * vs) / (2 * a)) / (durationMs / 2 + vs / (2 * a))
        const t1 = (vp - vs) / a
        const tb2 = durationMs - t1
        if (vp >= vs - EPS && vp <= v0 + EPS && tb2 >= BRAKE_MIN_MS - 1e-6) return { D, vs, a, t1, vp, tc: 0, tb: tb2 }
      }
    }
  } else if (a > 0) {
    // vitesse nulle (arrêt dès le lancement) : montée puis décélération (A' avec vs = 0)
    for (const D of cands) {
      const vp = (2 * D) / durationMs
      const t1 = vp / a
      const tb2 = durationMs - t1
      if (vp <= v0 + EPS && tb2 >= BRAKE_MIN_MS - 1e-6) return { D, vs: 0, a, t1, vp, tc: 0, tb: tb2 }
    }
  }
  // B : décélération constante pure (vitesse continue), arrêt avant la fin de la fenêtre.
  if (vs > EPS) {
    let best = null
    for (const D of cands) if ((2 * D) / vs <= durationMs) best = D
    if (best != null) return { D: best, vs, a, t1: 0, vp: vs, tc: 0, tb: (2 * best) / vs }
  }
  return null
}

/** Position normalisée à l'instant `t` (ms depuis le début du freinage) selon un `brakePlan` ; `to` exact à la fin. */
export function brakePosition(plan, from, to, t, n) {
  const { vs, a, t1, vp, tc, tb } = plan
  if (t >= t1 + tc + tb) return offsetToIndex(to, n)
  const tt = Math.max(0, t)
  let dist
  if (tt <= t1) dist = vs * tt + (a * tt * tt) / 2
  else {
    const p1 = vs * t1 + (a * t1 * t1) / 2
    if (tt <= t1 + tc) dist = p1 + vp * (tt - t1)
    else {
      const u = (tt - t1 - tc) / tb
      dist = p1 + vp * tc + vp * tb * (u - (u * u) / 2)
    }
  }
  return offsetToIndex(from + dist, n)
}
