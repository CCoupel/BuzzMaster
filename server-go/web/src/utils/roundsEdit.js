/**
 * roundsEdit — édition de la structure des manches (#245, v11.2). Fonctions PURES : chaque geste de l'éditeur
 * (déplacer une question ou une manche, créer, renommer, supprimer) produit une nouvelle structure COMPLÈTE, que
 * `toRoundsUpdatePayload` transforme en ROUNDS_UPDATE (contrat websocket-actions.md §ROUNDS_UPDATE : le serveur
 * refuse `INCOMPLETE` si une question existante n'est pas citée).
 *
 * Structure : { rounds: [{ id, name, questions: string[], entractes? }], unassigned: string[], deletedRounds: [{id, questions}] }
 * `id` absent = nouvelle manche (ID attribué par le serveur).
 */
import { groupQuestionsByRound } from './questionOrder'

export const UNASSIGNED_KEY = 'un'
export const MAX_ROUND_NAME = 60

/** Structure courante (IDs seulement) à partir des questions et de la liste des manches du serveur. */
export function structureFrom(questionsById, rounds) {
  const g = groupQuestionsByRound(questionsById, rounds)
  return {
    rounds: g.rounds.map(r => ({ id: r.ID, name: r.NAME, questions: r.questions.map(q => String(q.ID)) })),
    unassigned: g.unassigned.map(q => String(q.ID)),
    deletedRounds: [],
  }
}

/** Identifiant de groupe : UNASSIGNED_KEY ou l'ID (ou l'indice `new:<n>`) de la manche. */
function roundKey(r, i) { return r.id || `new:${i}` }
export { roundKey }

function clone(s) {
  return {
    rounds: s.rounds.map(r => ({ ...r, questions: [...r.questions] })),
    unassigned: [...s.unassigned],
    deletedRounds: [...(s.deletedRounds || [])],
  }
}

/** Retire une question de tous les groupes. */
function without(s, qid) {
  s.unassigned = s.unassigned.filter(x => x !== qid)
  s.rounds.forEach(r => { r.questions = r.questions.filter(x => x !== qid) })
}

/**
 * Déplace une question vers un groupe (`toKey` = UNASSIGNED_KEY ou clé de manche), avant `beforeId` si fourni
 * (sinon à la fin du groupe).
 */
export function moveQuestion(struct, qid, toKey, beforeId = null) {
  const s = clone(struct)
  qid = String(qid)
  without(s, qid)
  const list = toKey === UNASSIGNED_KEY ? s.unassigned : s.rounds.find((r, i) => roundKey(r, i) === toKey)?.questions
  if (!list) return struct
  const at = beforeId != null ? list.indexOf(String(beforeId)) : -1
  if (at >= 0) list.splice(at, 0, qid)
  else list.push(qid)
  return s
}

/** Déplace la manche d'indice `from` à la position `to` (0-based, dans la liste des manches). */
export function moveRound(struct, from, to) {
  if (from === to || from < 0 || to < 0 || from >= struct.rounds.length || to >= struct.rounds.length) return struct
  const s = clone(struct)
  const [r] = s.rounds.splice(from, 1)
  s.rounds.splice(to, 0, r)
  return s
}

/** Valide un nom de manche : 1 à 60 caractères après suppression des espaces de bord. */
export function cleanRoundName(name) {
  const n = String(name ?? '').trim()
  return n.length >= 1 && n.length <= MAX_ROUND_NAME ? n : null
}

export function renameRound(struct, id, name) {
  const n = cleanRoundName(name)
  if (!n) return struct
  const s = clone(struct)
  const r = s.rounds.find(x => x.id === id)
  if (!r) return struct
  r.name = n
  return s
}

/** Nouvelle manche en fin de liste ; `entractes` (défaut vrai) = créer « Début / Fin de la manche … » côté serveur. */
export function addRound(struct, name, entractes = true) {
  const n = cleanRoundName(name)
  if (!n) return struct
  const s = clone(struct)
  s.rounds.push({ id: undefined, name: n, questions: [], entractes: !!entractes })
  return s
}

/**
 * Supprime une manche avec le choix de la fenêtre : 'KEEP' (ses entrées passent à la fin de « Sans manche », dans
 * leur ordre) ou 'DELETE' (supprimées avec leurs médias). Les entrées d'une manche supprimée ne sont pas citées
 * (le serveur les retrouve par leur ROUND_ID).
 */
export function deleteRound(struct, id, choice) {
  const s = clone(struct)
  const r = s.rounds.find(x => x.id === id)
  if (!r) return struct
  s.rounds = s.rounds.filter(x => x !== r)
  s.deletedRounds.push({ id, questions: choice === 'DELETE' ? 'DELETE' : 'KEEP' })
  if (choice !== 'DELETE') s.unassigned = [...s.unassigned, ...r.questions]
  return s
}

/** Identifiants des entrées d'une manche supprimée avec DELETE (pour l'affichage optimiste). */
export function deletedQuestionIds(before, after) {
  const kept = new Set([...after.unassigned, ...after.rounds.flatMap(r => r.questions)])
  return [...before.unassigned, ...before.rounds.flatMap(r => r.questions)].filter(id => !kept.has(id))
}

/** Charge utile pour `roundsUpdate()` du hook. */
export function toRoundsUpdatePayload(struct) {
  return {
    rounds: struct.rounds.map(r => ({
      ...(r.id ? { id: r.id } : {}),
      name: r.name,
      questions: r.questions,
      ...(r.id ? {} : { entractes: r.entractes !== false }),
    })),
    unassigned: struct.unassigned,
    deletedRounds: struct.deletedRounds || [],
  }
}

/** Résumé des types d'un groupe replié : « 2 QCM · 1 SPEEDY ». */
export function typesSummary(questions) {
  const c = {}
  questions.forEach(q => { const t = q.TYPE || 'SPEEDY'; c[t] = (c[t] || 0) + 1 })
  return Object.entries(c).map(([t, n]) => `${n} ${t}`).join(' · ')
}
