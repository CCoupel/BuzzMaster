/**
 * usePictoThread — fil des mots d'une manche Rafale Picto (admin et anim, picto.md §17.5 / maquettes M6 et M7).
 * Construit côté client à partir de `PICTO_WORD` (mot visible seulement par admin, anim et dessinateur) et des
 * compteurs du GameState ; remis à zéro à chaque changement de question ou de manche (retour en préparation).
 */
import { useRef } from 'react'
import { advanceThread, closeThread } from './pictoRafaleState'

export function usePictoThread(gs, pictoWord) {
  const ref = useRef({ qid: null, thread: [], seen: { found: 0, passes: 0 } })
  const st = ref.current
  const found = gs?.PICTO_FOUND_COUNT || 0
  const passes = gs?.PICTO_PASS_COUNT || 0
  const qid = gs?.question?.ID ?? null
  const phase = gs?.phase

  // Nouvelle question, retour en préparation (relance) ou compteurs repartis à zéro : fil vide
  if (st.qid !== qid || phase === 'PREPARE' || phase === 'READY' || found < st.seen.found || passes < st.seen.passes) {
    st.qid = qid
    st.thread = []
    st.seen = { found: 0, passes: 0 }
  }

  if (phase === 'STARTED' || phase === 'PAUSED' || phase === 'STOPPED') {
    const word = pictoWord && pictoWord.EPOCH === gs.PICTO_CANVAS_EPOCH ? (pictoWord.WORD || '') : ''
    if (word) {
      const next = advanceThread(st.thread, { word, epoch: gs.PICTO_CANVAS_EPOCH, found, passes }, st.seen)
      st.thread = next.thread
      st.seen = next.seen
    }
    return st.thread
  }
  if (phase === 'REVEALED') {
    const closed = closeThread(st.thread, { found }, st.seen)
    return closed
  }
  return st.thread
}
