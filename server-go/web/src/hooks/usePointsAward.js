import { useEffect, useRef, useState } from 'react'
import { planAward, lotOffsetMs, scheduleAward, pickArrivalSlots } from '../utils/pointsAwardLots'

// #265 — pilote l'animation d'attribution de points (TV : tous les lots ; tablette : le gain de SA
// équipe, au moment où son lot passe à la TV). Calendrier = module pur pointsAwardLots (même calcul des
// deux côtés). Source : SCORE_CHANGE.GAINS (contrat game-state.md) lu UNE fois par nouveau SEQ ; repli par
// comparaison des scores (équipes triées par nom) pour les crédits automatiques et les anciens serveurs.
// Règles : DISPLAY=false → rien ; premier SEQ d'une connexion → référence sans animer ; changement de vue →
// file vidée ; tous les minuteurs sont nettoyés.

export const FLIGHT = 0.28 // part du lot passée en vol
export const STAGGER = 0.03 // décalage de départ entre deux fusées d'un lot
export const FADE_AT = 0.86 // début du fondu de sortie

const rnd = (a, b) => a + Math.random() * (b - a)
const clamp = (v, a, b) => Math.max(a, Math.min(b, v))

function prefersReducedMotion() {
  try {
    return !!window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
  } catch {
    return false
  }
}

const rgb = (c) => (Array.isArray(c) && c.length >= 3 ? `rgb(${c.slice(0, 3).join(',')})` : '#6366f1')

function scoresOf(teams) {
  const out = {}
  for (const [name, t] of Object.entries(teams || {})) out[name] = t?.SCORE || 0
  return out
}

// Repli : hausses de score observées, triées par nom (même ordre sur la TV et la tablette).
function diffGains(prev, scores) {
  const rises = Object.keys(scores)
    .filter((n) => prev[n] !== undefined && scores[n] > prev[n])
    .map((n) => ({ TEAM: n, POINTS: scores[n] - prev[n] }))
  // Pas de tri (#127 : aucun tri sur la rafale PREPARE→READY d'un VJoueur) tant qu'il n'y a pas 2 hausses.
  return rises.length > 1 ? rises.sort((a, b) => a.TEAM.localeCompare(b.TEAM)) : rises
}

/**
 * @returns {Array} lots affichés : { id, index, count, durationMs, own, cards: [{ key, team, points, color,
 *   stage: 'wait'|'flying'|'in'|'out', end:{x,y}, start:{x,y}, ctrl:{x,y} }] }
 */
export function usePointsAward({ scoreChange, teams, enabled, ownTeam, isVPlayer }) {
  const [lots, setLots] = useState([])
  const seqRef = useRef(null) // null = aucun état de référence reçu
  const prevScoresRef = useRef(null)
  const busyRef = useRef(0)
  const timersRef = useRef(new Set())
  const idRef = useRef(0)

  const later = (fn, ms) => {
    const t = setTimeout(() => { timersRef.current.delete(t); fn() }, Math.max(0, ms))
    timersRef.current.add(t)
  }
  const clearAll = () => {
    timersRef.current.forEach(clearTimeout)
    timersRef.current.clear()
    busyRef.current = 0
    setLots((l) => (l.length ? [] : l))
  }

  const setCard = (lotId, key, patch) => setLots((ls) => ls.map((l) => (l.id !== lotId ? l
    : { ...l, cards: l.cards.map((c) => (c.key === key ? { ...c, ...patch } : c)) })))

  // Joue un lot (cartes d'un lot de la TV, ou la seule carte de la tablette) après `delayMs`.
  const playLot = (lot, count, delayMs, own) => {
    const reduced = prefersReducedMotion()
    const id = ++idRef.current
    const L = lot.durationMs
    const slots = own ? [{ x: 50, y: 50 }] : pickArrivalSlots(lot.entries.length)
    const cards = lot.entries.map((e, i) => {
      const end = slots[i]
      const start = own ? { x: rnd(10, 90), y: 100 } : { x: rnd(5, 95), y: 100 }
      const ctrl = own
        ? { x: clamp(50 + rnd(-25, 25), 5, 95), y: rnd(20, 40) }
        : { x: clamp((start.x + end.x) / 2 + rnd(-20, 20), 2, 98), y: clamp(Math.min(end.y, start.y) - rnd(5, 15), 2, 98) }
      return { key: `${id}-${e.TEAM}`, team: e.TEAM, points: e.POINTS, color: rgb(teams?.[e.TEAM]?.COLOR), stage: 'wait', end, start, ctrl, reduced }
    })
    later(() => {
      setLots((ls) => [...ls, { id, index: lot.index, count, durationMs: L, own, reduced, cards }])
      cards.forEach((c, i) => {
        const t0 = i * L * STAGGER
        if (reduced) {
          later(() => setCard(id, c.key, { stage: 'in' }), t0)
        } else {
          later(() => setCard(id, c.key, { stage: 'flying' }), t0)
          later(() => setCard(id, c.key, { stage: 'in' }), t0 + L * FLIGHT)
        }
      })
      later(() => setLots((ls) => ls.map((l) => (l.id !== id ? l : { ...l, cards: l.cards.map((c) => ({ ...c, stage: 'out' })) }))), L * FADE_AT)
      later(() => setLots((ls) => ls.filter((l) => l.id !== id)), L)
    }, delayMs)
  }

  useEffect(() => {
    const seq = scoreChange?.SEQ ?? 0
    const scores = scoresOf(teams)
    const prev = prevScoresRef.current
    prevScoresRef.current = scores
    const first = seqRef.current === null
    const isNewSeq = !first && seq !== seqRef.current
    seqRef.current = seq
    if (first || !prev || !enabled) return // premier état d'une connexion : référence sans animer

    let gains
    if (isNewSeq) {
      if (scoreChange?.DISPLAY === false) {
        // Correction cachée : aucune animation pour les équipes listées, qui n'entre pas dans la file. Une
        // autre équipe dont le score monte dans le même UPDATE (rare) reste animée comme un crédit automatique.
        const hidden = new Set(scoreChange.TEAMS || [])
        gains = diffGains(prev, scores).filter((g) => !hidden.has(g.TEAM))
      } else gains = Array.isArray(scoreChange?.GAINS)
        ? scoreChange.GAINS.filter((g) => g && teams?.[g.TEAM])
        : diffGains(prev, scores) // ancien serveur sans GAINS
    } else {
      gains = diffGains(prev, scores) // crédit automatique (pas de nouveau SEQ)
    }

    const plan = planAward(gains)
    const sched = scheduleAward(busyRef.current, Date.now(), plan)
    busyRef.current = sched.busyUntilMs
    if (plan.lots.length === 0) return

    if (isVPlayer) {
      // Tablette : seulement SA propre équipe, au début de son lot à la TV (la file avance quand même).
      if (!ownTeam) return
      const off = lotOffsetMs(plan, ownTeam)
      if (off === null) return
      const lot = plan.lots.find((l) => l.entries.some((e) => e.TEAM === ownTeam))
      playLot({ ...lot, entries: lot.entries.filter((e) => e.TEAM === ownTeam) }, plan.lots.length, sched.delayMs + off, true)
    } else {
      plan.lots.forEach((lot) => playLot(lot, plan.lots.length, sched.delayMs + lot.startMs, false))
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [scoreChange, teams])

  // Changement de vue : la file est vidée.
  useEffect(() => { if (!enabled) clearAll() }, [enabled]) // eslint-disable-line react-hooks/exhaustive-deps
  // Démontage : plus aucun minuteur.
  useEffect(() => () => { timersRef.current.forEach(clearTimeout); timersRef.current.clear() }, [])

  return lots
}
