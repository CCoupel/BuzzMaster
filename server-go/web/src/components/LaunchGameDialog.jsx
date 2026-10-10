import { useEffect, useMemo, useRef, useState } from 'react'
import Button from './Button'
import './LaunchGameDialog.css'

// #243 (v11.2) — bouton « LANCER LA PARTIE » + popup de purge à 4 cases
// (maquette validée admin_new_game__purge_popup_details). Partagé par la page
// de jeu et le Backstage. Rien n'est effacé sans confirmation ; Scores et
// Questions cochées par défaut ; « Joueurs des tablettes » coche ET verrouille
// « Scores » (le serveur refuse PURGE_PLAYERS sans PURGE_SCORES).

const QUESTION_RUNNING_PHASES = ['STARTED', 'PAUSED', 'COUNTDOWN']
const PLAYED_STATUSES = ['STARTED', 'PAUSED', 'STOPPED', 'REVEALED']

const REFUSAL_MESSAGES = {
  PLAYERS_REQUIRE_SCORES: 'Lancement refusé : supprimer les joueurs des tablettes exige de remettre les scores à zéro.',
}

/** Compteurs affichés dans la popup, dérivés de l'état reçu (aucun calcul serveur). */
export function getLaunchCounts(teams, bumpers, questions) {
  const bumperList = Object.values(bumpers || {})
  const vplayers = bumperList.filter((b) => b && b.IS_VPLAYER)
  const questionList = Object.values(questions || {})
  return {
    teams: Object.keys(teams || {}).length,
    players: bumperList.length,
    vplayers: vplayers.length,
    vplayersConnected: vplayers.filter((b) => b.CONNECTED === true).length,
    played: questionList.filter((q) => PLAYED_STATUSES.includes(q?.STATUS)).length,
    questions: questionList.length,
  }
}

function Option({ id, checked, locked, danger, onChange, title, count, description, lockNote, inputRef }) {
  return (
    <label
      className={`lg-opt${danger ? ' danger' : ''}${checked ? ' checked' : ''}${locked ? ' locked' : ''}`}
      htmlFor={id}
    >
      <input id={id} type="checkbox" checked={checked} disabled={locked} onChange={onChange} ref={inputRef} />
      <div>
        <div className="lg-t">{title} <span className="lg-count">{count}</span></div>
        <div className="lg-d">{description}</div>
        {locked && lockNote && <div className="lg-lock-note">{lockNote}</div>}
      </div>
    </label>
  )
}

/**
 * @param {Object} props
 * @param {(options: {purgeScores: boolean, purgePlayers: boolean, purgeTeams: boolean, resetQuestions: boolean}) => void} props.onConfirm
 * @param {() => void} props.onClose
 * @param {ReturnType<typeof getLaunchCounts>} props.counts
 * @param {string} [props.phase] - phase courante (avertissement « question en cours »)
 * @param {boolean} props.sent - le NEW_GAME est parti : confirmation désactivée (pas de double envoi)
 * @param {string|null} [props.refusedReason] - NEW_GAME_REFUSED reçu : message, la popup reste ouverte
 */
export function LaunchGameDialog({ onConfirm, onClose, counts, phase, sent, refusedReason }) {
  const [scores, setScores] = useState(true)
  const [questions, setQuestions] = useState(true)
  const [players, setPlayers] = useState(false)
  const [teams, setTeams] = useState(false)
  const firstRef = useRef(null)
  const dialogRef = useRef(null)

  const scoresChecked = players || scores // D2 : joueurs supprimés => scores obligatoirement remis à zéro
  const destructive = useMemo(() => [
    players && `les ${counts.vplayers} joueurs des tablettes`,
    teams && `les ${counts.teams} équipes`,
  ].filter(Boolean), [players, teams, counts.vplayers, counts.teams])
  const kept = [
    !scoresChecked && 'scores',
    !questions && 'questions déjà jouées',
    !players && 'joueurs des tablettes',
    !teams && 'équipes',
  ].filter(Boolean)
  const nothing = !(scoresChecked || questions || players || teams)
  const running = QUESTION_RUNNING_PHASES.includes(phase)

  useEffect(() => { firstRef.current?.focus() }, [])

  const onKeyDown = (e) => {
    if (e.key === 'Escape') {
      if (!sent) { e.stopPropagation(); onClose() }
      return
    }
    if (e.key === 'Tab' && dialogRef.current) { // piège de focus dans la popup
      const f = [...dialogRef.current.querySelectorAll('input:not(:disabled), button:not(:disabled)')]
      if (f.length === 0) return
      const first = f[0]
      const last = f[f.length - 1]
      if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last.focus() }
      else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus() }
    }
  }

  const confirm = () => {
    if (sent) return
    onConfirm({
      purgeScores: scoresChecked,
      purgePlayers: players,
      purgeTeams: teams,
      resetQuestions: questions,
    })
  }

  return (
    <div className="lg-backdrop" onMouseDown={(e) => { if (e.target === e.currentTarget && !sent) onClose() }}>
      <div
        className="lg-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="lg-title"
        aria-describedby="lg-sub"
        ref={dialogRef}
        onKeyDown={onKeyDown}
      >
        <h3 id="lg-title">Lancer la partie</h3>
        <p className="lg-sub" id="lg-sub">
          Choisissez ce qui doit être effacé ou remis à zéro avant de commencer. Ce qui n'est pas coché est conservé.
        </p>

        <Option
          id="lg-scores" inputRef={firstRef}
          checked={scoresChecked} locked={players}
          onChange={(e) => setScores(e.target.checked)}
          title="Scores" count={`(${counts.teams} équipes, ${counts.players} joueurs)`}
          description="Remet tous les scores d'équipe et de joueur à 0 et efface l'historique des points."
          lockNote="🔒 Obligatoire quand les joueurs des tablettes sont supprimés : leurs points ne peuvent pas être conservés sans eux."
        />
        <Option
          id="lg-questions" checked={questions}
          onChange={(e) => setQuestions(e.target.checked)}
          title="Questions" count={`(${counts.played} jouées sur ${counts.questions})`}
          description="Remet toutes les questions « à jouer ». Décoché : les questions déjà jouées restent marquées jouées."
        />
        <Option
          id="lg-players" danger checked={players}
          onChange={(e) => setPlayers(e.target.checked)}
          title="Joueurs des tablettes" count={`(${counts.vplayersConnected} connectés)`}
          description="Supprime tous les joueurs inscrits sur tablette : ils devront se réinscrire (comportement de l'ancien bouton « Nouvelle partie »). Les buzzers physiques ne sont pas concernés."
        />
        <Option
          id="lg-teams" danger checked={teams}
          onChange={(e) => setTeams(e.target.checked)}
          title="Équipes" count={`(${counts.teams})`}
          description={<>Supprime toutes les équipes. Les joueurs conservés (buzzers et tablettes) restent inscrits mais <b>sans équipe</b> : il faudra recréer les équipes et y affecter les joueurs.</>}
        />

        {destructive.length > 0 && (
          <div className="lg-warn" role="alert">⚠ Irréversible : {destructive.join(' et ')} seront supprimés.</div>
        )}
        {kept.length > 0 && <div className="lg-keep">Conservé : {kept.join(', ')}.</div>}
        <div className="lg-keep">Dans tous les cas : la question en cours s'arrête.</div>
        {running && <div className="lg-busy">⚠ Une question est en cours : elle sera interrompue.</div>}
        {refusedReason && (
          <div className="lg-warn" role="alert">
            {REFUSAL_MESSAGES[refusedReason] || 'Lancement refusé par le serveur.'}
          </div>
        )}

        <div className="lg-actions">
          <button type="button" className="lg-btn lg-btn-secondary" onClick={onClose} disabled={sent}>Annuler</button>
          <button
            type="button"
            className={`lg-btn ${destructive.length ? 'lg-btn-danger' : 'lg-btn-primary'}`}
            onClick={confirm}
            disabled={sent}
          >
            {nothing ? 'Lancer sans rien effacer' : 'Lancer la partie'}
          </button>
        </div>
      </div>
    </div>
  )
}

/**
 * Bouton « LANCER LA PARTIE » + popup. `onLaunch(options)` envoie NEW_GAME.
 * La popup se ferme quand la phase passe à NEW_GAME après l'envoi ; un refus
 * serveur (`refused`) la laisse ouverte avec le message.
 */
export default function LaunchGameButton({
  onLaunch, teams, bumpers, questions, phase, refused, onClearRefused,
  variant = 'fun', size = 'sm', className = '',
}) {
  const [open, setOpen] = useState(false)
  const [sent, setSent] = useState(false)
  const openerRef = useRef(null)
  const counts = useMemo(() => getLaunchCounts(teams, bumpers, questions), [teams, bumpers, questions])

  const close = () => {
    setOpen(false)
    setSent(false)
    onClearRefused?.()
    openerRef.current?.focus?.()
  }

  // Refus serveur : la popup reste ouverte, on peut corriger et relancer.
  useEffect(() => {
    if (refused && open) setSent(false)
  }, [refused, open])

  // Lancement accepté (phase NEW_GAME) : fermeture.
  useEffect(() => {
    if (open && sent && phase === 'NEW_GAME') close()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [phase, sent, open])

  return (
    <>
      <Button
        variant={variant}
        size={size}
        className={className}
        onClick={(e) => { openerRef.current = e.currentTarget; onClearRefused?.(); setOpen(true) }}
        title="Préparer une nouvelle partie : choisir ce qu'il faut effacer"
      >
        LANCER LA PARTIE
      </Button>
      {open && (
        <LaunchGameDialog
          counts={counts}
          phase={phase}
          sent={sent}
          refusedReason={refused?.reason ?? null}
          onClose={close}
          onConfirm={(opts) => { onClearRefused?.(); setSent(true); onLaunch(opts) }}
        />
      )}
    </>
  )
}
