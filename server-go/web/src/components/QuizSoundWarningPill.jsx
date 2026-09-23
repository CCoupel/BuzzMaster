import { useMemo } from 'react'
import { useGame } from '../hooks/GameContext'
import { useSoundStatus } from '../hooks/useSoundStatus'
import './QuizSoundWarningPill.css'

/**
 * QuizSoundWarningPill — puce d'alerte globale (v11.1, addendum média
 * indisponible, #219/#236/#237, plan `_work/reports/plan-20260923-101500.md`
 * §3bis, maquette `docs/mockups/question-sound-219.html` §07). Patron
 * `RafalePoolAlert` (un composant, un besoin, deux surfaces) — montée sans
 * props sur `QuestionsPage.jsx` (où on **agit** : retirer les sons) et
 * `GamePage.jsx` (où on **joue** : le savoir avant de sélectionner une
 * question).
 *
 * État **entièrement dérivé**, aucun appel serveur nouveau : `useGame()`
 * expose déjà `questions` (diffusion `QUESTIONS`, admin seul — donc jamais
 * montée sur `/anim`, qui ne les reçoit pas), `useSoundStatus()` interroge
 * déjà `GET /api/sound/status` (montage + 30 s + évènement
 * `buzzcontrol:sound-changed`). Aucun état local propre à ce composant : il
 * s'efface de lui-même dès que l'une des deux conditions cesse d'être vraie.
 *
 * ⚠️ Ne regarde QUE « le quiz a des sons » × « l'audio est actif » — jamais
 * un fichier son individuel manquant/corrompu, déjà couvert **question par
 * question** par la gate de lancement (`prepareWaitReason.js`, motif
 * `FILE`). Les deux signaux sont complémentaires, pas redondants (plan
 * §3bis.2).
 *
 * ⚠️ Composant **distinct** de la pastille de menu "Ambiance"
 * (`SoundSpeakerIcon.jsx`/`utils/soundState.js`) — celle-ci reste à deux
 * formes seulement (décision #234), jamais touchée ici (plan §3bis.3).
 *
 * @param {Object} [props]
 * @param {string} [props.className]
 */
export default function QuizSoundWarningPill({ className = '' }) {
  const { questions } = useGame()
  const { status } = useSoundStatus()

  const soundQuestionCount = useMemo(
    () => Object.values(questions || {}).filter(q => q.SOUND).length,
    [questions],
  )

  if (soundQuestionCount === 0 || status.active) return null

  const plural = soundQuestionCount > 1
  return (
    <div className={`quiz-sound-warning-pill ${className}`}>
      <span className="quiz-sound-warning-pill-icon">!</span>
      <span>
        <b>
          Ce quiz contient {soundQuestionCount} question{plural ? 's' : ''} sonore{plural ? 's' : ''},
          mais l'audio n'est pas disponible.
        </b>{' '}
        Elle{plural ? 's' : ''} ne pourr{plural ? 'ont' : 'a'} pas être lancée{plural ? 's' : ''}.
        Vérifier la configuration ou brancher l'enceinte, puis <b>redémarrer le serveur</b>.
      </span>
    </div>
  )
}
