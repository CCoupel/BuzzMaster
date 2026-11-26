/**
 * useRafaleFlash — TV Rafale Picto (picto.md §17.5, maquette M4) : à l'instant « Trouvé », le dessin reste affiché
 * `PICTO_FOUND_FLASH_MS` avec le mot trouvé en surimpression (traits de la nouvelle époque tamponnés par le store),
 * puis la feuille du mot suivant apparaît. À une passe, la feuille se vide aussitôt avec un repère ⏭ discret.
 * Ni « Trouvé par » ni points : l'animation commune ne joue qu'au crédit.
 *
 * @returns {{ foundWord: string, passMark: boolean }}
 */
import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { PICTO_FOUND_FLASH_MS } from './pictoRafaleState'

export function useRafaleFlash(gs, store, enabled) {
  const [foundWord, setFoundWord] = useState('')
  const [passMark, setPassMark] = useState(false)
  const seen = useRef({ qid: null, found: 0, passes: 0 })
  const timers = useRef({ found: null, pass: null })
  const found = gs?.PICTO_FOUND_COUNT || 0
  const passes = gs?.PICTO_PASS_COUNT || 0
  const qid = gs?.question?.ID ?? null
  const words = gs?.PICTO_FOUND_WORDS

  // Tenue de l'ancienne feuille au changement d'époque (store) — armée seulement en Rafale Picto
  useEffect(() => {
    if (!store?.armHold) return undefined
    store.armHold(enabled ? PICTO_FOUND_FLASH_MS : 0)
    return () => store.armHold(0)
  }, [store, enabled])

  // Passe : la feuille se vide tout de suite (avant la peinture), repère ⏭ une seconde
  useLayoutEffect(() => {
    if (!enabled) return
    if (seen.current.qid !== qid || passes < seen.current.passes) { seen.current = { qid, found, passes }; return }
    if (passes > seen.current.passes) {
      store?.releaseHold?.()
      setPassMark(true)
      clearTimeout(timers.current.pass)
      timers.current.pass = setTimeout(() => setPassMark(false), PICTO_FOUND_FLASH_MS)
    }
    seen.current.passes = passes
  }, [enabled, passes, qid, store]) // eslint-disable-line react-hooks/exhaustive-deps

  // Trouvé : le dernier mot trouvé s'affiche une seconde sur le dessin
  useEffect(() => {
    if (!enabled) return
    if (seen.current.qid !== qid || found < seen.current.found) { seen.current = { ...seen.current, qid, found }; setFoundWord(''); return }
    if (found > seen.current.found) {
      const w = Array.isArray(words) && words.length ? words[words.length - 1] : ''
      if (w) {
        setFoundWord(w)
        clearTimeout(timers.current.found)
        timers.current.found = setTimeout(() => setFoundWord(''), PICTO_FOUND_FLASH_MS)
      }
    }
    seen.current.found = found
  }, [enabled, found, qid, words])

  useEffect(() => () => { clearTimeout(timers.current.found); clearTimeout(timers.current.pass) }, [])

  return enabled ? { foundWord, passMark } : { foundWord: '', passMark: false }
}
