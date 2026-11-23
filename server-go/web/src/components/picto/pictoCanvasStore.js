/**
 * pictoCanvasStore — feuille PICTO côté client (TV, admin, anim, et dessinateur), hors état React.
 *
 * Les traits arrivent en rafale (≤ 1 trame / 60 ms) : ils ne passent jamais par `setState`. Le store garde
 * l'époque, le dernier `SEQ` et la liste ordonnée des traits ; les vues s'abonnent et redessinent sur
 * `requestAnimationFrame`. Règles (contrat picto.md §8.4/§8.5) :
 *  - une trame (ou une feuille) d'une AUTRE époque que `PICTO_CANVAS_EPOCH` est ignorée ;
 *  - un changement d'époque (UPDATE) efface la feuille ;
 *  - `SEQ ≠ dernier + 1` = trou : le store passe en attente de `PICTO_CANVAS` (les trames suivantes sont
 *    mises de côté puis rejouées) et `applyFrame` renvoie `'gap'` pour que l'appelant demande `PICTO_CANVAS_SYNC`.
 */

const SYNC_TIMEOUT_MS = 3000

export function createPictoCanvasStore() {
  let epoch = null // null = époque encore inconnue (aucun UPDATE reçu)
  let seq = 0
  let strokes = [] // [{ ID, PTS: [[x,y]…], END }]
  let version = 0
  let syncing = false
  let syncSince = 0
  let held = [] // trames reçues pendant l'attente d'une feuille
  const listeners = new Set()

  const notify = () => { version += 1; listeners.forEach(fn => fn()) }
  const clear = () => { seq = 0; strokes = []; held = []; syncing = false }

  /** Fusionne les traits d'une trame : même ID = suite du trait précédent. */
  const merge = (incoming) => {
    for (const s of incoming || []) {
      if (!s || !Array.isArray(s.PTS)) continue
      const cur = strokes.length ? strokes[strokes.length - 1] : null
      const existing = cur && cur.ID === s.ID ? cur : strokes.find(x => x.ID === s.ID)
      if (existing) {
        existing.PTS = existing.PTS.concat(s.PTS)
        existing.END = !!s.END
      } else {
        strokes.push({ ID: s.ID, PTS: s.PTS.slice(), END: !!s.END })
      }
    }
  }

  const replay = () => {
    const pending = held.filter(f => f.SEQ > seq).sort((a, b) => a.SEQ - b.SEQ)
    held = []
    for (const f of pending) {
      if (f.SEQ === seq + 1) { merge(f.STROKES); seq = f.SEQ } else if (f.SEQ > seq + 1) { held = pending.filter(x => x.SEQ > seq); syncing = true; break }
    }
  }

  return {
    subscribe(fn) { listeners.add(fn); return () => listeners.delete(fn) },
    getEpoch: () => epoch,
    getSeq: () => seq,
    getVersion: () => version,
    getStrokes: () => strokes,
    isSyncing: () => syncing,
    /** Plus grand ID de trait connu (le dessinateur continue la numérotation après une resynchronisation). */
    maxStrokeId: () => strokes.reduce((m, s) => Math.max(m, s.ID), 0),

    /** Époque issue d'un UPDATE (`PICTO_CANVAS_EPOCH`) : tout changement efface la feuille. */
    setEpoch(e) {
      if (!Number.isInteger(e) || e === epoch) return
      epoch = e
      clear()
      notify()
    },

    /** Trame relayée `PICTO_STROKES` → `'ok' | 'gap' | 'ignored'`. */
    applyFrame(msg) {
      if (!msg || msg.EPOCH !== epoch) return 'ignored'
      if (!Number.isInteger(msg.SEQ)) return 'ignored'
      if (syncing) {
        if (Date.now() - syncSince > SYNC_TIMEOUT_MS) { syncing = false; held = [] } else { held.push(msg); return 'ignored' }
      }
      if (msg.SEQ <= seq) return 'ignored'
      if (msg.SEQ !== seq + 1) { syncing = true; syncSince = Date.now(); held = [msg]; return 'gap' }
      merge(msg.STROKES)
      seq = msg.SEQ
      notify()
      return 'ok'
    },

    /** Feuille entière `PICTO_CANVAS` (réponse à `PICTO_CANVAS_SYNC`, ou envoi d'office). */
    applyCanvas(msg) {
      if (!msg || !Number.isInteger(msg.EPOCH)) return 'ignored'
      if (epoch === null) epoch = msg.EPOCH // rechargement : la feuille précède l'UPDATE
      else if (msg.EPOCH !== epoch) return 'ignored'
      strokes = []
      merge(msg.STROKES)
      seq = Number.isInteger(msg.SEQ) ? msg.SEQ : 0
      syncing = false
      replay()
      notify()
      return 'ok'
    },

    /** Trait local du dessinateur (jamais compté dans `SEQ`) : `end` ferme le trait. */
    addLocal(id, pts, end) {
      merge([{ ID: id, PTS: pts, END: !!end }])
      notify()
    },

    /** Marque la demande de resynchronisation comme envoyée (évite d'en émettre une par trame). */
    markSyncRequested() { syncing = true; syncSince = Date.now() },

    reset() { clear(); epoch = null; notify() },
  }
}
