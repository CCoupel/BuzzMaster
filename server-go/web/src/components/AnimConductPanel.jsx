import {
  startButtonState,
  pauseButtonState,
  continueButtonState,
  stopButtonState,
  revealButtonState,
} from '../utils/phaseRules'
import { getQuestionTypeMeta } from '../utils/questionTypeMeta'
import { getMotionCardPoints, computeStarsProrataPoints } from '../utils/motionGrid'
import AnimNextButton from './AnimNextButton'
import AnimQcmOptions from './AnimQcmOptions'
import AnimMemoryGrid from './AnimMemoryGrid'
import AnimMotionGrid from './AnimMotionGrid'
import AnimMotionCard from './AnimMotionCard'
import AnimMotionActions from './AnimMotionActions'
import AnimExplanationNote from './AnimExplanationNote'
import AnimRafaleActions from './AnimRafaleActions'
import AnimRafaleQuestion from './AnimRafaleQuestion'
import './AnimConductPanel.css'

// L1 — cinq emplacements FIXES (#166/F5) : la liste ne varie jamais dans
// son nombre d'entrées, seul l'état de chaque bouton varie par phase.
function buildL1(phase, question, handlers) {
  return [
    { key: 'start', label: 'LANCER', state: startButtonState(phase), onClick: handlers.onStart },
    { key: 'pause', label: 'PAUSE', state: pauseButtonState(phase), onClick: handlers.onPause },
    { key: 'continue', label: 'CONTINUER', state: continueButtonState(phase), onClick: handlers.onContinue },
    { key: 'stop', label: 'STOP', state: stopButtonState(phase), onClick: handlers.onStop },
    { key: 'reveal', label: 'RÉPONSE', state: revealButtonState(phase, question), onClick: handlers.onReveal },
  ]
}

// Libellé secondaire (#166/E3) — texte de présentation attaché à un état
// DÉJÀ dérivé de phaseRules (pas une condition d'activation réécrite,
// juste le texte humain qui va avec). Matrice complète vérifiée phase par
// phase contre _work/mockups/anim-conduct-permanent-166.html (les 3 écrans
// illustrés — READY, STARTED, REVEALED — donnent "attendu"/"optionnel"/
// "arrête"/"en cours"/"après arrêt"/"déjà révélée" ; les phases non
// illustrées (ENROLL, COUNTDOWN, PAUSED, NEW_GAME, PREPARE, STOPPED non
// jouée) reprennent le repli générique "indispo." faute de texte donné —
// à confirmer en revue si un libellé plus spécifique est attendu).
// waitReason (#172/C2) — motif d'attente PREPARE (buzzers, ou sélection non
// conforme, cf. utils/prepareWaitReason.js), calculé par AnimPage.jsx et
// passé en prop. Vient remplacer le repli générique "indispo." pour le
// bouton LANCER en PREPARE — même emplacement, même style, aucun nouveau
// badge ni CSS (#166).
function buttonSubLabel(key, state, phase, waitReason) {
  if (state === 'go') return key === 'continue' ? 'reprise' : 'attendu'
  if (state === 'optional') return 'optionnel'
  if (state === 'danger') return 'arrête'
  // state === 'off'
  if (key === 'start' && phase === 'PREPARE' && waitReason) return waitReason
  if (key === 'start' && (phase === 'STARTED' || phase === 'PAUSED')) return 'en cours'
  if (key === 'reveal' && (phase === 'STARTED' || phase === 'PAUSED')) return 'après arrêt'
  if (key === 'reveal' && phase === 'REVEALED') return 'déjà révélée'
  return 'indispo.'
}

/**
 * AnimConductPanel — zone de conduite de la page animateur (`/anim`, zone
 * B), réécrite en cinq lignes PERMANENTES (#166/F5, plan
 * `_work/reports/plan-20260815-144925.md`), puis REMAPPÉE #171/F3 (plan
 * `_work/reports/plan-20260816-192400.md`) :
 *
 * Renversement de principe par rapport à #156/#165 : le composant ne
 * choisit plus QUELS boutons rendre selon la phase (branches PREPARE /
 * isPlaying / canReveal / idle) — il rend TOUJOURS les mêmes cinq
 * emplacements de L1 et calcule l'ÉTAT de chacun. Ordre fixe (#171) :
 *   L1 (LANCER · PAUSE · CONTINUER · STOP · RÉPONSE, 5 emplacements, inchangé)
 *   → bloc central `.anim-conduct-mid` (flex:1, min-height:0,
 *     overflow-y:auto — SEUL endroit qui peut défiler, jamais la page) :
 *     L2 (gestes propres au mode, ex-L3 #166) → L3 (grille QCM ou emplacement
 *     réservé, ex-L2 #166) → L4 (note d'explication #168, inchangé)
 *   → À suivre (AnimNextButton, #166/F4 — déplacé en DERNIER, ancré en bas
 *     du panneau car dernier enfant d'un flex-column, #171/F3)
 *
 * États d'un bouton L1 : 'go' (vert, attendu) / 'optional' (bleu,
 * facultatif) / 'danger' (rouge, destructif) / 'off' (gris, NON cliquable,
 * n'émet AUCUNE action — un libellé secondaire explique pourquoi). Source
 * de vérité unique : `utils/phaseRules.js`, dérivé lui-même de
 * `engine.go:585-593` et `GamePage.jsx:373-378`. Aucune condition
 * d'activation n'est réécrite ici (point de revue R1-a) — seul le texte de
 * présentation (`buttonSubLabel`) est local.
 *
 * @param {Object} props
 * @param {string} props.phase - gameState.phase (chrome de conduite L1/L5
 *   uniquement — LANCER/PAUSE/CONTINUER/STOP/RÉPONSE/"à suivre" restent des
 *   commandes de l'hôte question, hors du périmètre hostContext ; #184/B-F2
 *   ne touche PAS cet usage)
 * @param {Object|null} props.question - gameState.question
 * @param {string[]} [props.qcmInvalidated] - état d'indices QCM de l'hôte
 *   COURANT (#185/C-F1) : `getTypeState(gameState, hostContext).qcmInvalidated`,
 *   calculé par `AnimPage.jsx` — question ou carte MEMOTION active selon
 *   l'hôte, jamais `gameState.qcmInvalidated` directement depuis ce composant
 * @param {boolean} props.revealed - phase === 'REVEALED' (phaseRules.isRevealed)
 *   — hôte QUESTION uniquement, inchangé par #184 : consommé par L4
 *   (AnimExplanationNote) et la branche QCM (AnimQcmOptions), toutes deux
 *   propres à l'hôte question en v7.0.0 (QCM en carte n'est pas encore câblé,
 *   #185/Phase 3)
 * @param {import('../utils/hostContext').HostContext} [props.hostContext] -
 *   contexte d'hôte normalisé (#184/B-F2/B-F3, `utils/hostContext.js`,
 *   calculé UNE FOIS par `AnimPage.jsx`) — de l'hôte COURANT : question si
 *   aucune carte MEMOTION active, sinon carte active. `hostContext.playable`/
 *   `.revealed` sont transmis tels quels à `AnimMemoryGrid`/`AnimMotionCard`
 *   (L3), qui ne reçoivent donc jamais `phase`/`subphase` directement. Repli
 *   question quand aucune carte MEMOTION n'est active — valeurs strictement
 *   identiques à l'ancien `phase === 'STARTED'`/`'REVEALED'` dans ce cas,
 *   donc neutre pour l'hôte question.
 * @param {{ID?: string}|null} props.nextQuestion - dernier NEXT_QUESTION reçu
 * @param {() => void} props.onStart
 * @param {() => void} props.onPause
 * @param {() => void} props.onContinue
 * @param {() => void} props.onStop
 * @param {() => void} props.onReveal
 * @param {(questionId: string) => void} props.onSelectNext
 * @param {Object} [props.teams] - teams (useGame()) — #159/F3, couleur du propriétaire MEMORY
 * @param {Object} [props.memory] - état MEMORY groupé (#159/F3) : {flippedCards, matchedPairs,
 *   pairOwners, currentTeam, teamPairs, teamErrors, errors} — passthrough vers AnimMemoryGrid,
 *   rien n'est recalculé ici
 * @param {(cardId: string) => void} [props.onFlipMemoryCard] - flipMemoryCard (useGame())
 * @param {Object} [props.motion] - état MEMOTION groupé (#160/F7) : {subphase, timerRunning,
 *   cardStates, cardTeams, currentTeam, currentTeamColor, selectedId, participatingTeams} —
 *   passthrough vers AnimMotionGrid/AnimMotionCard/AnimMotionActions, rien n'est recalculé ici
 * @param {(cardId: string) => void} [props.onSelectMotionCard] - selectMotionCard (useGame())
 * @param {() => void} [props.onFlipMotionCard] - flipMotionCard (useGame())
 * @param {() => void} [props.onStopMotionTimer] - stopMotionTimer (useGame())
 * @param {() => void} [props.onRevealMotionCard] - revealMotionCard (useGame())
 * @param {(cardId: string, winnerTeam: string) => void} [props.onDoneMotionCard] - doneMotionCard (useGame())
 * @param {string|null} [props.waitReason] - #172/C2 : motif d'attente PREPARE
 *   (utils/prepareWaitReason.js, calculé par AnimPage.jsx) — remplace le
 *   repli générique "indispo." du bouton LANCER quand renseigné
 * @param {boolean} [props.rafaleDisabled] - RAFALE (contrat rafale.md §5.1,
 *   milestone v8.0.0) : désactive les deux boutons L2 (RAFALE_VALIDATE/
 *   RAFALE_INVALIDATE) quand true — ex. hors sous-phase QUESTION. Câblage
 *   réel (AnimPage.jsx, `RAFALE_SUBPHASE`) en Phase 2 (#107) ; `false` par
 *   défaut ici (socle #197/#198, données non encore diffusées).
 * @param {() => void} [props.onRafaleValidate] - émet RAFALE_VALIDATE
 * @param {() => void} [props.onRafaleInvalidate] - émet RAFALE_INVALIDATE
 * @param {Object} [props.rafale] - données déjà résolues par AnimPage.jsx pour
 *   `AnimRafaleQuestion` (L3, #198 : question+réponse déplacées depuis
 *   `.anim-zone-context` vers cette zone centrale) — `current`, `teamName`,
 *   `teamColorCss`, `answerValue`, `catMeta`, `askedCount` (voir
 *   AnimRafaleQuestion.jsx pour le détail de chaque champ). `null`/absent
 *   avant que `AnimPage.jsx` ne les câble (jamais le cas en pratique dès
 *   qu'`isRafale`, mais évite un crash si le composant est monté isolément).
 * @param {Object} [props.cardRafale] - RAFALE en carte MEMOTION (#217, contrat
 *   rafale.md §14) — mêmes champs que `rafale` ci-dessus MOINS `teamName`/
 *   `teamColorCss` (mode SOLO forcé, l'équipe est déjà affichée en L2 par
 *   `AnimMotionActions`) et MOINS `next`/`nextCatMeta` (pas de pré-tirage en
 *   carte, §14.2) — `showNext` vaut toujours `false`. `null` hors carte RAFALE.
 * @param {boolean} [props.cardRafaleDisabled] - désactive VALIDE/INVALIDE
 *   pour la carte RAFALE active (hors sous-cycle QUESTION, §14.3)
 * @param {() => void} [props.onCardRafaleValidate] - émet RAFALE_VALIDATE
 *   scopé `MOTION_CARD_ID` (contrat §14.5)
 * @param {() => void} [props.onCardRafaleInvalidate] - émet RAFALE_INVALIDATE
 *   scopé `MOTION_CARD_ID` (contrat §14.5)
 */
export default function AnimConductPanel({
  phase,
  question,
  qcmInvalidated,
  revealed,
  hostContext,
  nextQuestion,
  onStart,
  onPause,
  onContinue,
  onStop,
  onReveal,
  onSelectNext,
  teams,
  memory,
  cardMemory,
  onFlipMemoryCard,
  motion,
  onSelectMotionCard,
  onFlipMotionCard,
  onStopMotionTimer,
  onRevealMotionCard,
  onDoneMotionCard,
  waitReason = null,
  rafaleDisabled = false,
  onRafaleValidate,
  onRafaleInvalidate,
  rafale = null,
  cardRafale = null,
  cardRafaleDisabled = false,
  onCardRafaleValidate,
  onCardRafaleInvalidate,
}) {
  const l1 = buildL1(phase, question, { onStart, onPause, onContinue, onStop, onReveal })
  const isQcm = question?.TYPE === 'QCM'
  const isMemory = question?.TYPE === 'MEMORY'
  // #160/F7 — premier mode à occuper L2 (ex-vide, #166/E4). GRID/MEMORIZE
  // n'ont pas de carte sélectionnée : selectedMotionCard reste null, sans
  // effet (AnimMotionCard n'est de toute façon monté que hors GRID/MEMORIZE).
  const isMemotion = question?.TYPE === 'MEMOTION'
  // RAFALE (contrat rafale.md §2.1, milestone v8.0.0) — même patron que
  // MEMOTION : un QuestionType, pas une phase. Occupe L2 (AnimRafaleActions
  // — VALIDE/INVALIDE) ; L3 reste l'emplacement réservé générique, RAFALE
  // ne portant aucune grille propre à ce jour (§7).
  const isRafale = question?.TYPE === 'RAFALE'
  const motionCards = question?.MOTION_CARDS || []
  const selectedMotionCard = isMemotion
    ? motionCards.find(c => c.ID === motion?.selectedId) || null
    : null
  const selectedMotionCardStars = selectedMotionCard
    ? getMotionCardPoints(selectedMotionCard.DIFFICULTY || 1, question?.MOTION_CONFIG)
    : 0
  // #187 cycle 4 (F1) — gain affiché sur le bouton d'attribution (L2,
  // AnimMotionActions, sous-phase REVEAL) : pour une carte MEMORY en barème
  // STARS_PRORATA (le défaut — POINTS_RULE absent ou MODE explicite
  // "STARS_PRORATA", contrat §6.2/§6.3), le montant plein NE DOIT PAS être
  // affiché — le serveur, seule autorité, crédite le prorata des paires
  // trouvées. Une surcharge explicite STARS/FIXED/PER_UNIT sur la carte
  // reste au barème étoiles plein (tout-ou-rien, §6.2). Sans ce branchement,
  // l'animateur voit un montant différent de celui réellement attribué —
  // c'est le défaut corrigé par ce cycle (computeStarsProrataPoints existait
  // déjà, testée, mais n'était appelée nulle part).
  const isMemoryStarsProrata = selectedMotionCard?.TYPE === 'MEMORY'
    && (!selectedMotionCard.POINTS_RULE?.MODE || selectedMotionCard.POINTS_RULE.MODE === 'STARS_PRORATA')
  const motionCardPoints = isMemoryStarsProrata
    ? computeStarsProrataPoints(
        selectedMotionCardStars,
        cardMemory?.matchedPairs?.length || 0,
        selectedMotionCard.MEMORY_PAIRS?.length || 0,
      )
    : selectedMotionCardStars
  const modeLabel = question?.TYPE ? getQuestionTypeMeta(question.TYPE).label : null
  // #171/F3 — contenu L2 (gestes propres au mode), ex-L3 #166.
  const modeGestureText = modeLabel ? `Aucun geste propre au mode ${modeLabel}` : 'Aucun geste propre au mode'

  // #187 — dispatch POSITIF par type de carte en L3 (remplace la chaîne de
  // ternaires imbriquée introduite par #185 — esprit #183 : "un dispatch
  // positif par type, pas une chaîne de ternaires allongée"). Un type
  // nestable absent de cette table retombe sur AnimMotionCard — le même
  // défaut que SPEEDY avait déjà avant #187, jamais silencieusement une
  // branche voisine par accident.
  const motionCardRenderers = {
    QCM: (card) => (
      // #185/C-F1 — carte QCM : point de délégation posé en B-F3
      // (résolution du type de l'hôte courant). `invalidated` vient de
      // `qcmInvalidated` (résolu par l'appelant via getTypeState, hôte
      // carte quand une carte est active — question-types.md §5.3), jamais
      // recalculé ici.
      <AnimQcmOptions
        answers={card.QCM_ANSWERS}
        correct={card.QCM_CORRECT}
        invalidated={qcmInvalidated}
        revealed={hostContext?.revealed}
      />
    ),
    MEMORY: (card) => (
      // #187 — carte MEMORY : `AnimMemoryGrid` est déjà agnostique de
      // l'hôte (#184/B-F2, reçoit `playable`/`revealed`, jamais `phase`/
      // `subphase`) — montée ici SANS variante. Une seule équipe par carte
      // (contrat §6.3) : pas de `pairOwners`/`teamPairs`/`teamErrors`/
      // `currentTeam` (§5.4 — ces champs n'ont pas de sens en carte, une
      // carte MEMORY est jouée par l'équipe MEMOTION courante, déjà
      // affichée en L2 par `AnimMotionActions`).
      <AnimMemoryGrid
        question={card}
        playable={hostContext?.playable}
        revealed={hostContext?.revealed}
        teams={teams}
        flippedCards={cardMemory?.flippedCards}
        matchedPairs={cardMemory?.matchedPairs}
        globalErrors={cardMemory?.errors}
        onFlip={onFlipMemoryCard}
      />
    ),
    // #217 — carte RAFALE : mini-manche à plusieurs questions, pas une
    // question fixe (contrairement à QCM/MEMORY ci-dessus) — L3 porte donc
    // à la fois la question tirée EN COURS (`AnimRafaleQuestion`, même
    // composant que la manche classique, `cardRafale` déjà résolu par
    // `AnimPage.jsx`) ET les deux boutons qui avancent le sous-cycle
    // (`AnimRafaleActions`) : contrairement à QCM/MEMORY, RAFALE n'a pas de
    // geste propre en L2 pour une carte — cet emplacement reste occupé par
    // le cycle générique `AnimMotionActions` (STOP CHRONO/RÉVÉLER/SANS
    // VAINQUEUR, §14.5), donc VALIDE/INVALIDE n'ont nulle part d'autre où
    // vivre. `card` (le paramètre) n'est pas consommé ici : la carte réelle
    // à afficher est la question TIRÉE (`cardRafale.current`), pas la
    // définition statique `MOTION_CARDS[i]` (catégories/difficultés
    // choisies, jamais un énoncé).
    RAFALE: () => (
      <div className="anim-card-rafale">
        <AnimRafaleQuestion {...(cardRafale || {})} />
        <AnimRafaleActions
          disabled={cardRafaleDisabled}
          onValidate={onCardRafaleValidate}
          onInvalidate={onCardRafaleInvalidate}
        />
      </div>
    ),
  }

  return (
    <div className="anim-conduct">
      <div className="anim-conduct-l1">
        {l1.map(btn => (
          <button
            key={btn.key}
            className={`anim-conduct-btn anim-conduct-btn-${btn.state}`}
            disabled={btn.state === 'off'}
            onClick={btn.state === 'off' ? undefined : btn.onClick}
          >
            {btn.label}
            <span className="anim-conduct-btn-sub">{buttonSubLabel(btn.key, btn.state, phase, waitReason)}</span>
          </button>
        ))}
      </div>

      {/* #171/F3 — bloc central unique, seul endroit qui peut défiler
          (flex:1, min-height:0, overflow-y:auto — AnimConductPanel.css).
          L1 et "à suivre" restent visibles en permanence de part et
          d'autre : L1 au-dessus (statique), "à suivre" en dessous (dernier
          enfant, ancré en bas). Sans min-height:0 ici, une note L4 longue
          repousserait "à suivre" hors écran — c'est précisément le bug que
          cet ancrage doit empêcher (F7). */}
      <div className="anim-conduct-mid">
        {/* L2 — gestes propres au mode. #160/F7 : MEMOTION est le PREMIER
            mode à occuper cet emplacement (ex-vide, #166/E4) — QCM et
            MEMORY gardent leur repli générique inchangé (R5, point de
            revue). */}
        <div className="anim-conduct-l2">
          {isMemotion ? (
            <AnimMotionActions
              subphase={motion?.subphase}
              timerRunning={motion?.timerRunning}
              currentTeam={motion?.currentTeam}
              currentTeamColor={motion?.currentTeamColor}
              selectedCardId={motion?.selectedId}
              cardPoints={motionCardPoints}
              onFlipMotionCard={onFlipMotionCard}
              onStopMotionTimer={onStopMotionTimer}
              onRevealMotionCard={onRevealMotionCard}
              onDoneMotionCard={onDoneMotionCard}
            />
          ) : isRafale ? (
            <AnimRafaleActions
              disabled={rafaleDisabled}
              rafaleMode={question?.RAFALE_MODE}
              onValidate={onRafaleValidate}
              onInvalidate={onRafaleInvalidate}
            />
          ) : (
            <div className="anim-conduct-reserved">{modeGestureText}</div>
          )}
        </div>

        {/* L3 — grille QCM, grille MEMORY (#159/F3), grille/carte MEMOTION
            (#160/F7) ou emplacement réservé (#166/F9). */}
        <div className="anim-conduct-l3">
          {isQcm ? (
            <AnimQcmOptions
              answers={question.QCM_ANSWERS}
              correct={question.QCM_CORRECT}
              invalidated={qcmInvalidated}
              revealed={revealed}
            />
          ) : isMemory ? (
            <AnimMemoryGrid
              question={question}
              playable={hostContext?.playable}
              revealed={hostContext?.revealed}
              teams={teams}
              flippedCards={memory?.flippedCards}
              matchedPairs={memory?.matchedPairs}
              pairOwners={memory?.pairOwners}
              currentTeam={memory?.currentTeam}
              teamPairs={memory?.teamPairs}
              teamErrors={memory?.teamErrors}
              globalErrors={memory?.errors}
              onFlip={onFlipMemoryCard}
            />
          ) : isMemotion ? (
            (motion?.subphase === 'MEMORIZE' || motion?.subphase === 'GRID') ? (
              <AnimMotionGrid
                question={question}
                phase={phase}
                subphase={motion?.subphase}
                cardStates={motion?.cardStates}
                cardTeams={motion?.cardTeams}
                currentTeam={motion?.currentTeam}
                selectedId={motion?.selectedId}
                teams={teams}
                onSelect={onSelectMotionCard}
              />
            ) : selectedMotionCard && motionCardRenderers[selectedMotionCard.TYPE] ? (
              motionCardRenderers[selectedMotionCard.TYPE](selectedMotionCard)
            ) : (
              // Défaut (SPEEDY, ou tout type nestable sans rendu dédié) —
              // inchangé par #187.
              <AnimMotionCard
                playable={hostContext?.playable}
                revealed={hostContext?.revealed}
                card={selectedMotionCard}
                motionConfig={question?.MOTION_CONFIG}
              />
            )
          ) : isRafale ? (
            // RAFALE (#198, retour QUALIF 8.0.0.13) — question+réponse
            // affichées ICI (zone centrale), déplacées depuis
            // .anim-zone-context (le bandeau du haut) où elles vivaient au
            // cycle précédent. Câblage réel dans AnimPage.jsx.
            <AnimRafaleQuestion {...(rafale || {})} />
          ) : (
            <div className="anim-conduct-reserved">Aucun élément spécifique à cette question</div>
          )}
        </div>

        {/* L4 — note d'explication (#168, câblée). Consomme useHoldToPeek via
            AnimExplanationNote — même geste EXACT que la zone réponse
            (#166/F10, #169). question/revealed déjà disponibles dans les
            props de ce composant (#166). */}
        <div className="anim-conduct-l4">
          <AnimExplanationNote question={question} revealed={revealed} />
        </div>
      </div>

      {/* L5 — "à suivre" (#171/F3, déplacé de juste après L1 vers cette
          dernière position ; composant lui-même inchangé). */}
      <AnimNextButton phase={phase} question={question} nextQuestion={nextQuestion} onSelectNext={onSelectNext} />
    </div>
  )
}
