/**
 * PictoTv — écran TV (statique) et tablettes joueur d'une manche PICTO (v13.0.0, #192/#193, maquettes M3 et M4).
 *
 * TV : structure commune à tous les types (`game-content-zones`) — chronomètre commun (`Timer`, `zone-timer`), indice dans
 * le bandeau haut (`zone-question` : catégorie et « ✏️ Nom dessine pour Équipe »), feuille carrée centrée dans la zone média
 * (`zone-media`), mot dans la zone réponse (`zone-answers`) seulement en REVEALED. Écran statique : `overflow: hidden`,
 * unités viewport. Le mot n'entre jamais ici avant la révélation (contrat §10) ; la TV ne lit que `PICTO_REVEALED_WORD`.
 * Tablettes (`isVPlayer`) : le dessinateur a le mot en bandeau et la feuille de dessin (sans aucun outil) ; son équipe
 * « devine » à l'oral ; les autres attendent leur tour. Les tablettes ne reçoivent jamais le dessin (c'est la TV qui le montre).
 */
import { useEffect } from 'react'
import Timer from '../Timer'
import PictoCanvasView from './PictoCanvasView'
import PictoCanvasInput from './PictoCanvasInput'
import { errorText, pictoShownWord } from './pictoState'
import { foundLabel, passUsedLabel } from './pictoRafaleState'
import { useRafaleFlash } from './useRafaleFlash'
import { isRafalePicto } from '../../utils/pictoFamily'
import './PictoCanvas.css'
import './PictoTv.css'

// « le dessin d'Alice » / « de Bruno » (élision devant une voyelle ou un h)
const deName = (n) => (/^[aeiouyhàâäéèêëîïôöùûü]/i.test(n || '') ? `d'${n}` : `de ${n || '—'}`)

const mmss = (t) => {
  const s = Math.max(0, Math.round(Number(t) || 0))
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`
}

/** Indice du bandeau haut : catégorie + « ✏️ Alice dessine pour Équipe ». */
function Hint({ gs, catMeta, small = false }) {
  return (
    <div className={`picto-hint${small ? ' small' : ''}`} data-testid="picto-hint">
      {catMeta && <span className="picto-hint-cat">{catMeta.imageURL ? null : `${catMeta.icon} `}{catMeta.label}</span>}
      {gs.PICTO_DRAWER_NAME && (
        <span className="picto-hint-who">✏️ <b>{gs.PICTO_DRAWER_NAME}</b> dessine pour <b>{gs.PICTO_TEAM || '—'}</b></span>
      )}
    </div>
  )
}

/** Écran TV : zones communes (M3) ; Rafale Picto : grand compteur à gauche de la feuille, mot trouvé 1 s dessus (M4). */
export function PictoTvScreen({ gameState: gs, catMeta, store }) {
  const revealed = gs.phase === 'REVEALED'
  const rafale = isRafalePicto(gs.question?.TYPE)
  const { foundWord, passMark } = useRafaleFlash(gs, store, rafale)
  const foundCount = gs.PICTO_FOUND_COUNT || 0
  const passCount = gs.PICTO_PASS_COUNT || 0
  const foundWords = Array.isArray(gs.PICTO_FOUND_WORDS) ? gs.PICTO_FOUND_WORDS : []
  return (
    <div className={`game-content-zones picto-tv${rafale ? ' picto-tv-rafale' : ''}`} data-testid="picto-tv">
      <div className="zone-timer">
        <Timer currentTime={gs.timer} totalTime={gs.totalTime} phase={gs.phase} size="xl" showPhase={false} />
      </div>
      <div className="zone-question"><Hint gs={gs} catMeta={catMeta} /></div>
      <div className="zone-media picto-media">
        {rafale && (
          <div className="picto-counter" data-testid="picto-counter">
            <span className="picto-counter-n">{foundCount}</span>
            <span className="picto-counter-lbl">{foundCount > 1 ? 'mots trouvés' : 'mot trouvé'}</span>
            {passCount > 0 && <span className="picto-counter-pass" data-testid="picto-counter-pass">⏭ {passUsedLabel(passCount)}</span>}
          </div>
        )}
        <div className="picto-sheet picto-sheet-fit picto-sheet-rel">
          {store && <PictoCanvasView store={store} />}
          {foundWord && <div className="picto-found-flash" data-testid="picto-found-flash">✅ {foundWord}</div>}
          {passMark && <div className="picto-pass-mark" data-testid="picto-pass-mark" aria-hidden="true">⏭</div>}
        </div>
      </div>
      <div className="zone-answers">
        {revealed && (gs.PICTO_REVEALED_WORD || (rafale && foundWords.length > 0)) && (
          <div className="answer-container">
            {gs.PICTO_REVEALED_WORD && <p className="answer-text" data-testid="picto-revealed-word">{gs.PICTO_REVEALED_WORD}</p>}
            {rafale && foundWords.length > 0 && (
              <p className="picto-tv-found" data-testid="picto-tv-found">
                {foundLabel(foundCount)} : {foundWords.map((w, i) => <span key={`${w}-${i}`} className="picto-tv-found-word">✅ {w}</span>)}
              </p>
            )}
          </div>
        )}
      </div>
    </div>
  )
}

/** Tablette du dessinateur : préparation (« C'est toi qui dessines ! ») puis mot en bandeau et feuille (M4). */
export function PictoDrawerScreen({ gameState: gs, pictoWord, store, sendMessage, error, landscape = false, catMeta = null }) {
  const rafale = isRafalePicto(gs.question?.TYPE)
  const prep = ['PREPARE', 'READY', 'COUNTDOWN'].includes(gs.phase)
  const word = pictoShownWord(gs, pictoWord)
  const locked = gs.phase !== 'STARTED'
  const full = error?.CODE === 'CANVAS_FULL'
  const revealed = gs.phase === 'REVEALED'
  // Feuille verrouillée : pause, temps écoulé / arrêt ; en révélation, la feuille finale reste affichée sans voile
  const lockLabel = gs.phase === 'PAUSED' ? '⏸ Jeu en pause'
    : gs.phase === 'STOPPED' ? (gs.PICTO_OUTCOME === 'TIMEOUT' ? '⏰ Temps écoulé' : '⏹ Manche arrêtée')
      : ''

  // Rechargement de la page / reconnexion : on redemande la feuille (réponse vide hors manche, contrat §8.5)
  useEffect(() => { if (!prep) sendMessage?.('PICTO_CANVAS_SYNC', {}) }, []) // eslint-disable-line react-hooks/exhaustive-deps

  if (prep) {
    return (
      <div className="picto-tab center" data-testid="picto-drawer-prep">
        <div className="picto-tab-emoji" aria-hidden="true">✏️</div>
        <div className="picto-tab-title">C'est toi qui dessines !</div>
        <div className="picto-tab-sub">Le mot s'affichera au lancement de la manche.</div>
        {gs.phase === 'COUNTDOWN' && (
          <div className="countdown-number picto-tab-countdown" data-testid="picto-drawer-countdown" aria-live="polite">
            {gs.countdownTime > 0 ? gs.countdownTime : 'GO!'}
          </div>
        )}
      </div>
    )
  }
  return (
    <div className={`picto-tab drawer${landscape ? ' land' : ''}`} data-testid="picto-drawer">
      <div className="picto-tab-banner">
        <span>
          {revealed ? 'Mot révélé' : `Fais deviner · ⏱ ${mmss(gs.timer)}`}
          {rafale && !revealed && <b className="picto-tab-found" data-testid="picto-drawer-found"> · ✅ {gs.PICTO_FOUND_COUNT || 0} mot{(gs.PICTO_FOUND_COUNT || 0) > 1 ? 's' : ''}</b>}
        </span>
        {rafale && !revealed && catMeta && <span className="picto-tab-cat" data-testid="picto-drawer-cat">Ton mot · {catMeta.icon ? `${catMeta.icon} ` : ''}{catMeta.label}</span>}
        <div className="picto-tab-word" data-testid="picto-drawer-word">{word || '—'}</div>
      </div>
      <div className="picto-tab-sheet">
        <PictoCanvasInput
          store={store}
          epoch={gs.PICTO_CANVAS_EPOCH}
          locked={locked}
          lockLabel={lockLabel}
          sendMessage={sendMessage}
        />
      </div>
      <p className="picto-tab-foot">{full ? errorText(error) : 'Dessine avec le doigt. Aucun outil : trait noir.'}</p>
    </div>
  )
}

/** Autres tablettes : équipe qui dessine (devine à l'oral), autres équipes (attendent), révélation. */
export function PictoGuesserScreen({ gameState: gs, teamName }) {
  const mine = !!teamName && teamName === gs.PICTO_TEAM
  const rafale = isRafalePicto(gs.question?.TYPE)
  const foundCount = gs.PICTO_FOUND_COUNT || 0
  const passCount = gs.PICTO_PASS_COUNT || 0
  const foundWords = Array.isArray(gs.PICTO_FOUND_WORDS) ? gs.PICTO_FOUND_WORDS : []
  if (gs.phase === 'REVEALED') {
    return (
      <div className="picto-tab center" data-testid="picto-guesser-revealed">
        <div className="picto-tab-sub">Mot révélé</div>
        {gs.PICTO_REVEALED_WORD ? <div className="picto-tab-word big">{gs.PICTO_REVEALED_WORD}</div> : null}
        {rafale && foundWords.length > 0 && (
          <ul className="picto-tab-found-list" data-testid="picto-guesser-found-list">
            {foundWords.map((w, i) => <li key={`${w}-${i}`}>✅ {w}</li>)}
          </ul>
        )}
        {rafale && <div className="picto-tab-sub" data-testid="picto-guesser-recap">{foundLabel(foundCount)} · {passUsedLabel(passCount)}</div>}
      </div>
    )
  }
  return (
    <div className="picto-tab center" data-testid="picto-guesser">
      <div className="picto-tab-emoji" aria-hidden="true">{mine ? '👀' : '✏️'}</div>
      <div className="picto-tab-title">{mine ? 'Devinez !' : `${gs.PICTO_TEAM || "L'équipe"} devine le dessin ${deName(gs.PICTO_DRAWER_NAME)}`}</div>
      <div className="picto-tab-sub">{mine ? `${gs.PICTO_DRAWER_NAME || '—'} dessine — regardez la TV` : (rafale ? '' : 'Attendez votre tour.')}</div>
      {rafale && (
        <div className="picto-tab-sub" data-testid="picto-guesser-counts">
          ✅ {foundLabel(foundCount)}
          {mine && passCount > 0 && <><br />⏭ {passCount} passe{passCount > 1 ? 's' : ''} utilisée{passCount > 1 ? 's' : ''}</>}
        </div>
      )}
      <div className="picto-tab-timer">{mmss(gs.timer)}</div>
    </div>
  )
}

export default function PictoTv({ gameState, catMeta, isVPlayer = false, playerId = null, teamName = null, pictoWord = null, store = null, sendMessage, error = null }) {
  if (!isVPlayer) return <PictoTvScreen gameState={gameState} catMeta={catMeta} store={store} />
  if (playerId && gameState.PICTO_DRAWER && playerId === gameState.PICTO_DRAWER) {
    return <PictoDrawerScreen gameState={gameState} pictoWord={pictoWord} store={store} sendMessage={sendMessage} error={error} catMeta={catMeta} />
  }
  return <PictoGuesserScreen gameState={gameState} teamName={teamName} />
}
