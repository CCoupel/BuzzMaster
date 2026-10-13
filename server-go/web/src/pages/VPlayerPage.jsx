import { useState, useEffect, useRef, useCallback } from 'react'
import { useAckedSend } from '../hooks/useAckedSend'
import { useNavigate } from 'react-router-dom'
import { useGame } from '../hooks/GameContext'
import { AnimatePresence, motion } from 'framer-motion'
import PlayerDisplay from './PlayerDisplay'
import EntractePanel from '../components/EntractePanel'
import ArdoiseKeyboard from '../components/ArdoiseKeyboard'
import NoSleep from 'nosleep.js'
import { REJECTION_MESSAGES, DEFAULT_REJECTION_MESSAGE, REDIRECT_MESSAGES, DEFAULT_REDIRECT_MESSAGE } from '../utils/playerConnectMessages'
import { fnv1a } from '../utils/fnv1a'
import { resolveHostContext } from '../utils/hostContext'
import { getTypeState } from '../utils/typeState'
import {
  clearOwnVPlayerSession, getStoredVPlayerId, isStaleVPlayerSession,
  saveArdoiseDraft, loadArdoiseDraft, clearArdoiseDraft,
} from '../utils/vplayerSession'
import './VPlayerPage.css'
import '../styles/entracte.css'

// QCM answer colors mapping
const ANSWER_COLORS = {
  RED: '#ef4444',
  GREEN: '#22c55e',
  YELLOW: '#eab308',
  BLUE: '#3b82f6',
}

// Fix R1 (suite, #109) : délai avant redirection automatique après un rejet
// de reconnexion — laisse le temps de lire le message d'erreur.
const RECONNECT_ERROR_REDIRECT_DELAY_MS = 3000

// #120 — filet de sécurité : si le bumper n'apparaît jamais (un point de
// suppression aurait été oublié côté serveur), ne pas laisser le joueur
// bloqué indéfiniment sur l'état d'attente (F5). 10 s est très largement
// au-delà de la fenêtre normale d'enrôlement (résolue en un aller-retour de
// broadcast, de l'ordre de la centaine de ms) : ce délai ne se déclenche
// jamais pendant un enrôlement légitime.
const SESSION_EXPIRED_SAFETY_NET_MS = 10000

// #246 — cadence maximale des ARDOISE_INPUT pendant la frappe (leading + trailing).
const ARDOISE_THROTTLE_MS = 200

// #246 — plafond de renvois d'ARDOISE_FINAL sans accusé (1 renvoi / s).
const FINAL_MAX_SENDS = 30

export default function VPlayerPage() {
  const navigate = useNavigate()
  const {
    sendMessage, gameState, bumpers, teams, status,
    playerConnectStatus, clearPlayerConnectStatus,
    playerEvictedStatus, clearPlayerEvictedStatus,
    sendClientTrace,
    ardoiseFinalAck,
    flipMemoryCard, flipMemoryCardAck,
  } = useGame()

  const [playerSession, setPlayerSession] = useState(null)
  const [bumper, setBumper] = useState(null)
  const [team, setTeam] = useState(null)
  const [isFullscreen, setIsFullscreen] = useState(false)
  const [showFullscreenHint, setShowFullscreenHint] = useState(false)
  // Fix R1 (suite, #109) : message affiché quand une tentative de
  // reconnexion (PLAYER_CONNECT avec ID) est rejetée par le serveur — ID
  // périmé (bumper supprimé par l'admin) dont le nom a été repris entre-
  // temps, ou NAME_TAKEN générique. Sans ça, écran vide sans explication.
  const [reconnectError, setReconnectError] = useState(null)
  // #120 — motif de renvoi vers l'inscription : `null` = pas de renvoi en
  // cours, une chaîne (éventuellement vide) = renvoi armé, message résolu via
  // REDIRECT_MESSAGES (repli DEFAULT_REDIRECT_MESSAGE si motif inconnu/absent
  // — jamais d'écran muet). Alimenté par PLAYER_EVICTED ou par le filet de
  // sécurité local ci-dessous.
  const [evictedReason, setEvictedReason] = useState(null)
  // #118 (F4) — bandeau de connexion sur l'écran de jeu : `null` = rien
  // (fonctionnement normal), `'lost'` = liaison perdue (orange, persiste tant
  // que `status !== 'connected'`), `'restored'` = liaison rétablie (vert,
  // effacé après 2s). Ne s'arme jamais sur la toute première connexion au
  // montage — seulement après un premier `'connected'` déjà observé, voir
  // `hasConnectedOnceRef` ci-dessous.
  const [connectionBanner, setConnectionBanner] = useState(null)
  const hasConnectedOnceRef = useRef(false)
  const wasDisconnectedRef = useRef(false)
  // #246 (F-ID) — identité de CET onglet. `ownIdRef` = ID de la session en
  // mémoire (jamais relu depuis le localStorage courant, qui appartient au
  // dernier onglet ayant écrit). `bound` = la connexion WebSocket courante est
  // liée à notre bumper (PLAYER_CONNECTED reçu sur CETTE connexion) ; tant que
  // faux, aucune action joueur n'est émise (le serveur la refuserait).
  const ownIdRef = useRef(null)
  const [bound, setBound] = useState(false)
  const boundRef = useRef(false)
  // Onglet périmé (un autre onglet porte une autre session) : plus aucune émission.
  const [stale, setStale] = useState(false)
  const staleRef = useRef(false)
  // Session reprise par une connexion plus récente (PLAYER_EVICTED DUPLICATE_SESSION).
  const [superseded, setSuperseded] = useState(false)
  const supersededRef = useRef(false)
  const staleTracedRef = useRef(false)
  staleRef.current = stale
  supersededRef.current = superseded
  const noSleepRef = useRef(null)
  // Ref to always have the latest bumper value in async callbacks (avoids stale closure)
  const bumperRef = useRef(null)

  // ARDOISE: local text state + throttle ref
  const [ardoiseText, setArdoiseText] = useState('')
  const ardoiseThrottleRef = useRef(null)
  // #246 — le joueur a-t-il saisi / un texte a-t-il été restauré / la page a-t-elle
  // vu le démarrage de la question dans CETTE montée ? Sans l'un des trois, un
  // final vide n'est jamais envoyé (il écraserait la réponse que le serveur détient).
  const typedRef = useRef(false)
  const restoredRef = useRef(false)
  const sawStartRef = useRef(false)
  const questionIdRef = useRef(null)
  const ardoiseLatestRef = useRef(null) // dernier état saisi non encore envoyé
  const ardoiseLastSentAtRef = useRef(0)
  // Annule l'envoi programmé et oublie l'état en attente (changement de question,
  // gel du clavier : le final prend le relais).
  const cancelArdoiseTimer = () => {
    if (ardoiseThrottleRef.current) clearTimeout(ardoiseThrottleRef.current)
    ardoiseThrottleRef.current = null
    ardoiseLatestRef.current = null
    ardoiseLastSentAtRef.current = 0
  }
  // #246 : dernier texte local, relu par le renvoi complet à la liaison.
  const ardoiseTextRef = useRef('')
  // #246 (F-LOG) — SEQ d'ARDOISE_INPUT : croissant par question et par
  // connexion (remis à 0 à la liaison et au changement de question).
  const ardoiseSeqRef = useRef(0)
  // #246 (F-STOP) — texte FINAL envoyé à la sortie de STARTED : { qid, text, hash,
  // acked }. `finalStatus` : 'sending' (renvoi toutes les 1 s jusqu'à l'accusé
  // valide) | 'received' | null. Jamais créé pour une page montée après le STOP
  // (rechargement) : envoyer un texte vide écraserait la réponse déjà saisie.
  const finalRef = useRef(null)
  const [finalStatus, setFinalStatus] = useState(null)
  ardoiseTextRef.current = ardoiseText
  const prevPhaseRef = useRef(null)
  // ARDOISE #117: true once the first non-empty ARDOISE_INPUT has been sent for the
  // current question, so the server can timestamp STARTED_AT on the real first
  // keystroke instead of the first post-debounce pause.
  const ardoiseFirstSentRef = useRef(false)

  // Load session from localStorage on mount
  useEffect(() => {
    const savedName = localStorage.getItem('vplayer_name')
    const savedSession = localStorage.getItem('vplayer_session')
    const savedId = localStorage.getItem('vplayer_id')

    // #246 : sans ID mémorisé, aucune liaison possible (plus de repli par
    // nom) — la session est inutilisable, retour à l'inscription.
    if (!savedName || !savedSession || !savedId) {
      if (savedName || savedSession) clearOwnVPlayerSession(null)
      navigate('/')
      return
    }

    ownIdRef.current = savedId
    setPlayerSession({
      name: savedName,
      sessionId: savedSession,
      id: savedId,
    })
  }, [navigate])

  // #246 (F-ID) — détection d'onglet périmé sur l'événement `storage` (un
  // autre onglet vient d'écrire une autre session dans le localStorage partagé).
  useEffect(() => {
    const onStorage = (e) => {
      if (e.key !== null && e.key !== 'vplayer_id') return
      if (isStaleVPlayerSession(ownIdRef.current)) setStale(true)
    }
    window.addEventListener('storage', onStorage)
    return () => window.removeEventListener('storage', onStorage)
  }, [])

  // Trace de diagnostic (une fois) quand l'onglet se déclare périmé — seule
  // émission autorisée d'un onglet périmé.
  useEffect(() => {
    if (!stale || status !== 'connected' || staleTracedRef.current) return
    staleTracedRef.current = true
    sendClientTrace?.('STALE_TAB', {
      DETAIL: { OWN_ID: ownIdRef.current, STORED_ID: getStoredVPlayerId() },
    })
  }, [stale, status, sendClientTrace])

  // #246 (F-LOG) — ARDOISE_INPUT numéroté (SEQ) + motif (REASON). SEQ n'avance
  // que si le message est réellement parti.
  const sendArdoise = useCallback((text, id, reason) => {
    if (!boundRef.current || staleRef.current || supersededRef.current) return false
    const seq = ardoiseSeqRef.current + 1
    const sent = sendMessage('ARDOISE_INPUT', { TEXT: text, ID: id, SEQ: seq, REASON: reason })
    if (sent) ardoiseSeqRef.current = seq
    return sent
  }, [sendMessage])

  // La liaison est propre à une connexion : toute perte de connexion la défait.
  useEffect(() => {
    if (status !== 'connected') {
      boundRef.current = false
      setBound(false)
    }
  }, [status])

  // Émission d'une action joueur : seulement si la connexion courante est
  // liée et que l'onglet est actif. Retourne false si non envoyé.
  const sendBound = useCallback((action, payload) => {
    if (!boundRef.current || staleRef.current || supersededRef.current) return false
    return sendMessage(action, payload)
  }, [sendMessage])

  // #248 — accusé de réception de la réponse QCM / du buzz SPEEDY / du retournement MEMORY-MEMOTION.
  const pendingColorRef = useRef(null) // couleur QCM choisie, mise en avant pendant l'attente
  // Lot 5b — retournement : un FLIP_ID neuf par geste (identique à chaque renvoi) ; l'accusé est celui qui
  // porte ce FLIP_ID. Repli sans accusé du serveur (ancien serveur) : la carte vue retournée dans l'état reçu.
  const flipIdRef = useRef(null)
  const flipCardRef = useRef(null)
  const qType = gameState.question?.TYPE
  const isFlipQuestion = qType === 'MEMORY' || qType === 'MEMOTION'
  const flipAck = flipMemoryCardAck && flipIdRef.current && flipMemoryCardAck.flipId === flipIdRef.current ? flipMemoryCardAck : null
  // Repli sans accusé : état MEMORY lu par l'accesseur unique (comme PlayerDisplay) ; la carte est vue
  // retournée, OU sa paire est trouvée (le moteur vide les retournées dès qu'une paire est trouvée).
  const flipMemoryState = isFlipQuestion ? getTypeState(gameState, resolveHostContext(gameState)).memory : null
  const flipPairId = flipCardRef.current ? String(flipCardRef.current).split('-')[0] : null
  const flipSeen = !!flipMemoryState && !!flipCardRef.current && (
    (flipMemoryState.flippedCards || []).includes(flipCardRef.current)
    || (flipPairId !== null && (flipMemoryState.matchedPairs || []).some((p) => String(p) === flipPairId)))
  const ackedSend = useAckedSend({
    sendBound,
    connStatus: status,
    bound,
    phase: gameState.phase,
    questionId: gameState.question?.ID ?? null,
    acked: isFlipQuestion ? (flipAck?.status === 'APPLIED' || flipSeen) : (!!bumper && bumper.TIME > 0),
    teamServed: isFlipQuestion ? false : (!!team && team.TIME > 0 && !(bumper && bumper.TIME > 0)),
    refused: isFlipQuestion && flipAck?.status === 'IGNORED' ? (flipAck.reason || 'IGNORED') : null,
  })

  // Find bumper by ID in bumpers list (#120 F2, #246 : sans repli par nom).
  useEffect(() => {
    if (!playerSession || !bumpers) return

    // #246 : identité par ID uniquement — jamais de repli par nom.
    let bumperId = null
    let bumperData = null
    if (playerSession.id && bumpers[playerSession.id]) {
      bumperId = playerSession.id
      bumperData = bumpers[playerSession.id]
    }

    if (bumperId && bumperData) {
      const newBumper = { id: bumperId, ...bumperData }
      setBumper(newBumper)
      // Fix R1 (verify, #109) : ne marquer "trouvé/déjà connecté" (ce qui
      // court-circuite le timer de reconnexion ci-dessous) que si le serveur
      // rapporte réellement CONNECTED=true pour ce bumper. Après un drop
      // réseau, le bumper reste dans `bumpers` (orange/red, jamais supprimé)
      // — dès que CE client se reconnecte au niveau WebSocket, la prochaine
      // UPDATE le fait matcher par nom AVANT que PLAYER_CONNECT n'ait jamais
      // été renvoyé. Sans cette garde, bumperRef.current devient truthy sur
      // la seule base du nom et le timer de reconnexion (ci-dessous) ne
      // renvoie PLAYER_CONNECT — le badge admin reste alors bloqué pour de
      // bon, car rien ne retouche plus jamais ce bumper côté serveur.
      bumperRef.current = bumperData.CONNECTED === true ? newBumper : null

      // Find team if assigned
      if (bumperData.TEAM && teams[bumperData.TEAM]) {
        setTeam(teams[bumperData.TEAM])
      } else {
        setTeam(null)
      }
    } else {
      // Bumper not found in current state — reset ref so reconnect logic can
      // fire. #120: this is NEVER, by itself, treated as an eviction — see
      // the PLAYER_EVICTED handling and F5 waiting state below.
      bumperRef.current = null
      // #123 (F2, cause A2) : symétrique au `bumperRef.current = null`
      // ci-dessus, mais manquait pour l'état React `bumper` (et `team`, qui
      // en dépend) — celui-ci restait donc figé à sa dernière valeur connue
      // pour toujours si le bumper disparaissait réellement du roster (
      // suppression admin, purge). Conséquences : l'écran de jeu continuait
      // d'afficher un joueur périmé (le symptôme rapporté), et le filet de
      // sécurité SESSION_EXPIRED de #118 — qui garde sur
      // `bumper || reconnectError || evictedReason !== null` — ne pouvait
      // plus jamais se déclencher puisque `bumper` restait éternellement
      // truthy. F5 (#120) affiche déjà l'état d'attente sur `!bumper`, donc
      // ce reset ne fait que réactiver un rendu déjà prévu pour ce cas.
      setBumper(null)
      setTeam(null)
    }
  }, [playerSession, bumpers, teams])

  // #120 (F1) — le serveur notifie explicitement l'éviction ; l'absence du
  // bumper dans un roster n'est plus jamais interprétée comme un renvoi (la
  // course qui causait #120 : le roster local est encore celui d'avant
  // l'inscription au moment où cette page monte, systématiquement, pendant
  // toute la fenêtre séparant PLAYER_CONNECTED de l'UPDATE qui suit).
  useEffect(() => {
    if (!playerEvictedStatus) return
    console.log('[VPlayer] PLAYER_EVICTED:', playerEvictedStatus.reason)
    // #246 : DUPLICATE_SESSION — la session reste valide (portée par la
    // connexion plus récente). On n'efface RIEN, on cesse d'émettre et on
    // propose « Reprendre ici ».
    if (playerEvictedStatus.reason === 'DUPLICATE_SESSION') {
      boundRef.current = false
      setBound(false)
      setSuperseded(true)
      sendClientTrace?.('DUPLICATE_SESSION', { DETAIL: { OWN_ID: ownIdRef.current } })
      clearPlayerEvictedStatus()
      return
    }
    // Fast-follow (#120 code-review [MAJEUR]) : effacer l'ID immédiatement, ne
    // pas attendre le délai de lecture de 3s (celui-ci ne doit retarder que le
    // bandeau/la navigation). Sans ça, si le minuteur de reconnexion hérité de
    // #109 (ci-dessous) atteint son échéance de 2s avant que la session ne
    // soit effacée, il retrouve `vplayer_id` encore présent et renvoie
    // PLAYER_CONNECT avec un ID déjà supprimé — le serveur le traite comme
    // absent et recrée un VJoueur fantôme sous le même nom.
    // #246 : seulement si l'ID stocké est le nôtre (le localStorage est partagé).
    if (getStoredVPlayerId() === ownIdRef.current) localStorage.removeItem('vplayer_id')
    setEvictedReason(playerEvictedStatus.reason ?? '')
    clearPlayerEvictedStatus()
  }, [playerEvictedStatus, clearPlayerEvictedStatus, sendClientTrace])

  // #120 — filet de sécurité (cf. risques du plan) : si le bumper ne se
  // matérialise jamais (chemin de suppression oublié côté serveur), ne pas
  // laisser le joueur bloqué indéfiniment sur l'état d'attente (F5). Purement
  // défensif : ne s'arme que 10 s après un état stable sans bumper, très
  // au-delà de la fenêtre d'enrôlement normale — n'interfère jamais avec F5,
  // dont le rendu reste inconditionnel et sans effet de bord.
  useEffect(() => {
    if (!playerSession || status !== 'connected') return
    if (bumper || reconnectError || evictedReason !== null) return
    // #246 : un onglet périmé ou évincé (DUPLICATE_SESSION) garde son écran
    // dédié tant que la situation dure — sa session n'est pas expirée.
    if (stale || superseded) return

    const timeoutId = setTimeout(() => {
      console.log('[VPlayer] Safety net: bumper never appeared, redirecting (SESSION_EXPIRED)')
      setEvictedReason('SESSION_EXPIRED')
    }, SESSION_EXPIRED_SAFETY_NET_MS)

    return () => clearTimeout(timeoutId)
  }, [playerSession, status, bumper, reconnectError, evictedReason, stale, superseded])

  // #246 (F-ID) — liaison de la connexion : PLAYER_CONNECT {NAME, ID} à CHAQUE
  // ouverture de WebSocket dès que l'ID est connu, même si le bumper est déjà
  // trouvé CONNECTED dans l'état reçu (avant #246 on s'en dispensait : la
  // connexion restait alors non liée et le serveur refuse ses actions).
  // L'ID vient de la session EN MÉMOIRE (`playerSession.id`), jamais du
  // localStorage courant, qui peut appartenir à un autre onglet. Rien n'est
  // envoyé par un onglet périmé ni par une session reprise ailleurs. Renvoi à
  // 2 s tant que la liaison n'est pas confirmée (opération idempotente).
  useEffect(() => {
    if (!playerSession || status !== 'connected') return
    if (evictedReason !== null || reconnectError || superseded) return
    if (stale) return
    if (isStaleVPlayerSession(playerSession.id)) {
      setStale(true)
      return
    }

    const attemptConnect = () => {
      if (boundRef.current || staleRef.current || supersededRef.current) return
      if (isStaleVPlayerSession(playerSession.id)) {
        setStale(true)
        return
      }
      console.log('[VPlayer] PLAYER_CONNECT name:', playerSession.name, 'id:', playerSession.id)
      sendMessage('PLAYER_CONNECT', { NAME: playerSession.name, ID: playerSession.id })
    }

    attemptConnect()
    const retryId = setTimeout(attemptConnect, 2000)
    return () => clearTimeout(retryId)
  }, [playerSession, status, sendMessage, evictedReason, reconnectError, stale, superseded])

  // Fix R1 (suite, #109) : consommer le résultat d'une tentative de
  // reconnexion. 'connected' ne nécessite rien ici (le bumper apparaîtra
  // via l'effet de matching par nom ci-dessus, alimenté par `bumpers`).
  // 'rejected' (ID périmé + nom repris, ou NAME_TAKEN) → afficher un
  // message et empêcher toute nouvelle tentative avec cet ID périmé.
  useEffect(() => {
    if (!playerConnectStatus) return

    if (playerConnectStatus.status === 'rejected') {
      console.log('[VPlayer] Reconnect rejected:', playerConnectStatus.reason)
      // L'ID stocké ne correspond plus à un bumper qui nous appartient — la
      // session locale entière est de toute façon invalidée par ce rejet
      // (#120 F4 : nettoyage mutualisé ; handleRejoinFromScratch la
      // re-effacerait de toute façon 3s plus tard, idempotent).
      clearOwnVPlayerSession(ownIdRef.current)
      // #118 (F3, R1) : playerSession (état React) n'était jamais remis à
      // null — seul localStorage l'était. La garde `!playerSession` du
      // minuteur de reconnexion restait donc inopérante : si `status`
      // basculait pendant les 3s de lecture qui suivent (un lien instable,
      // exactement le terrain de #118), l'effet pouvait se réarmer et
      // renvoyer PLAYER_CONNECT sans ID.
      setPlayerSession(null)
      // #123 (F3, cause B) : le motif est désormais celui REÇU DU SERVEUR,
      // jamais déduit côté client. #118 (F6) traduisait tout ENROLLMENT_CLOSED
      // en GAME_RESET en supposant « le plus souvent » une purge NEW_GAME —
      // mais une suppression individuelle produit le même ENROLLMENT_CLOSED
      // (ID introuvable, phase non ENROLL), et affichait alors à tort « une
      // nouvelle partie a commencé ». Avec le registre de motifs de #123
      // (B3, dev-backend), le serveur répond directement PLAYER_REMOVED ou
      // GAME_RESET quand la disparition est connue — le client n'a plus qu'à
      // relayer tel quel via le mécanisme de motifs de #120. ENROLLMENT_CLOSED
      // retrouve son sens littéral (inscriptions fermées) et reste affiché
      // via REJECTION_MESSAGES, inchangé (#109).
      if (playerConnectStatus.reason in REDIRECT_MESSAGES) {
        setEvictedReason(playerConnectStatus.reason)
      } else {
        setReconnectError(REJECTION_MESSAGES[playerConnectStatus.reason] || DEFAULT_REJECTION_MESSAGE)
      }
    }

    // #246 : PLAYER_CONNECTED de la connexion courante = liaison établie. Le
    // nom affiché est celui du serveur (le cas 1 ne renomme plus).
    if (playerConnectStatus.status === 'connected' && status === 'connected') {
      // SEQ repart à 1 seulement sur la transition non lié → lié (nouvelle
      // liaison) ; un second PLAYER_CONNECTED de la même connexion (renvoi à 2 s)
      // ne remet rien à zéro.
      if (!boundRef.current) ardoiseSeqRef.current = 0
      boundRef.current = true
      setBound(true)
      if (playerConnectStatus.name) {
        setPlayerSession((prev) => (prev && prev.name !== playerConnectStatus.name
          ? { ...prev, name: playerConnectStatus.name }
          : prev))
        if (!staleRef.current && getStoredVPlayerId() === ownIdRef.current) {
          localStorage.setItem('vplayer_name', playerConnectStatus.name)
        }
      }
    }

    clearPlayerConnectStatus()
  }, [playerConnectStatus, clearPlayerConnectStatus, status])

  // Fix R1 (suite) : après un rejet de reconnexion, repartir sur EnrollPage
  // avec un pseudo neuf — l'ancien bumper n'est plus à nous (supprimé ou
  // repris par quelqu'un d'autre), impossible de continuer sur cette session.
  //
  // Cible unique '/' : EnrollPage est la SEULE route d'enrôlement du routing
  // (vérifié — aucune page d'attente dédiée n'existe séparément) et gère déjà
  // en interne les deux sous-états via `gameState.enrollmentActive` (voir
  // EnrollPage.jsx `enrollmentOpen`) : formulaire standard si les inscriptions
  // sont ouvertes, écran "en attente de l'ouverture des inscriptions" sinon.
  // Naviguer vers '/' fait donc automatiquement atterrir l'utilisateur sur le
  // bon écran selon l'état d'enrôlement live — pas de route distincte à choisir.
  const handleRejoinFromScratch = useCallback(() => {
    clearOwnVPlayerSession(ownIdRef.current)
    setPlayerSession(null) // #118 (F3, R1) — cf. commentaire sur l'effet ci-dessus
    navigate('/')
  }, [navigate])

  // Fix R1 (suite) : la redirection doit être AUTOMATIQUE, pas seulement
  // disponible via un bouton — la session est de toute façon morte (ID
  // périmé), rien à faire d'autre ici. Un court délai laisse le temps de lire
  // le message ; le bouton reste disponible pour ne pas attendre.
  useEffect(() => {
    if (!reconnectError) return
    const timeoutId = setTimeout(handleRejoinFromScratch, RECONNECT_ERROR_REDIRECT_DELAY_MS)
    return () => clearTimeout(timeoutId)
  }, [reconnectError, handleRejoinFromScratch])

  // #120 (F1/F3) — renvoi vers l'inscription suite à PLAYER_EVICTED (ou au
  // filet de sécurité local) : efface la session et transmet le motif via
  // sessionStorage, lu et consommé une fois par EnrollPage au montage. Même
  // délai de lecture et même bouton de retour immédiat que le rejet de
  // reconnexion ci-dessus, pour rester homogène.
  const handleEvictedRedirect = useCallback(() => {
    clearOwnVPlayerSession(ownIdRef.current)
    setPlayerSession(null) // #118 (F3, R1) — cf. commentaire sur l'effet ci-dessus
    try {
      sessionStorage.setItem('vplayer_redirect_reason', evictedReason || '')
    } catch {
      // sessionStorage indisponible (navigation privée stricte, quota) — le
      // renvoi a toujours lieu, seul le bandeau de motif sur EnrollPage
      // n'apparaîtra pas.
    }
    navigate('/')
  }, [navigate, evictedReason])

  useEffect(() => {
    if (evictedReason === null) return
    const timeoutId = setTimeout(handleEvictedRedirect, RECONNECT_ERROR_REDIRECT_DELAY_MS)
    return () => clearTimeout(timeoutId)
  }, [evictedReason, handleEvictedRedirect])

  // #118 (F4) — bandeau de connexion : suit `status`, exposé par F1
  // (useWebSocket) qui passe désormais à 'disconnected' aussi bien sur une
  // vraie fermeture que sur la liaison morte détectée par sa surveillance
  // passive. Ignore la toute première connexion au montage (aucun bandeau en
  // fonctionnement normal) — seule une perte APRÈS un premier succès arme le
  // bandeau orange, suivie du bandeau vert 2s au retour.
  useEffect(() => {
    if (status === 'connected') {
      hasConnectedOnceRef.current = true
      if (wasDisconnectedRef.current) {
        wasDisconnectedRef.current = false
        setConnectionBanner('restored')
        const timeoutId = setTimeout(() => setConnectionBanner(null), 2000)
        return () => clearTimeout(timeoutId)
      }
    } else if (hasConnectedOnceRef.current) {
      wasDisconnectedRef.current = true
      setConnectionBanner('lost')
    }
  }, [status])

  // Auto-respond to PREPARE phase with PONG
  useEffect(() => {
    if (!bumper || !bumper.id) return
    if (gameState.phase !== 'PREPARE') return

    console.log('[VPlayer] Auto-sending PONG in PREPARE phase, bumper ID:', bumper.id)
    // Send PONG - ID in payload for web clients (#246 : seulement connexion liée)
    sendBound('PONG', { ID: bumper.id })
  }, [gameState.phase, bumper, sendBound, bound])

  // ARDOISE: reset text on question change
  useEffect(() => {
    setArdoiseText('')
    cancelArdoiseTimer()
    ardoiseFirstSentRef.current = false
    ardoiseSeqRef.current = 0 // #246 : SEQ repart à 1 pour chaque nouvelle question
  }, [gameState.question?.ID])

  // ARDOISE: reset on PREPARE (covers replaying the same question + race conditions)
  useEffect(() => {
    if (gameState.phase !== 'PREPARE') return
    setArdoiseText('')
    cancelArdoiseTimer()
    ardoiseFirstSentRef.current = false
    clearArdoiseDraft() // question rejouée : l'ancien texte conservé est périmé
  }, [gameState.phase])

  // ARDOISE #117: also rearm the immediate-first-send flag on entry into STARTED, so a
  // question restarted without passing through PREPARE still measures the real first key.
  useEffect(() => {
    if (gameState.phase !== 'STARTED') return
    ardoiseFirstSentRef.current = false
  }, [gameState.phase, gameState.question?.ID])

  // #246 (F-STOP) — émission du texte final (ARDOISE_FINAL) : texte complet au
  // moment du gel du clavier, hash FNV-1a UTF-8, QUESTION_ID courant. Renvoyé
  // tant que l'accusé n'est pas reçu (voir effets ci-dessous).
  const sendFinal = useCallback(() => {
    const f = finalRef.current
    if (!f || f.acked || (f.sends || 0) >= FINAL_MAX_SENDS) return false
    // Contrat §ARDOISE_FINAL « Règle de renvoi » : au plus 30 envois par liaison,
    // un envoi abandonné (WebSocket fermée / connexion non liée) compte aussi.
    f.sends = (f.sends || 0) + 1
    let sent = false
    if (boundRef.current && !staleRef.current && !supersededRef.current) {
      const seq = ardoiseSeqRef.current + 1
      sent = sendMessage('ARDOISE_FINAL', {
        ID: ownIdRef.current,
        QUESTION_ID: f.qid,
        TEXT: f.text,
        HASH: f.hash,
        SEQ: seq,
      })
      if (sent) ardoiseSeqRef.current = seq
    }
    if (f.sends >= FINAL_MAX_SENDS) {
      // Plafond atteint : arrêt (overlay « ⚠ »), trace de diagnostic. Une nouvelle
      // liaison de connexion relance un cycle de 30.
      setFinalStatus('stalled')
      sendClientTrace?.('ARDOISE_FINAL_GIVEUP', { DETAIL: { QUESTION_ID: f.qid, SENDS: f.sends } })
    }
    return sent
  }, [sendMessage, sendClientTrace])

  // Sortie de STARTED vers STOPPED/PAUSED (question ARDOISE) : gel du clavier
  // (disabled), débounce en attente annulé, texte final = dernier texte local.
  // Retour en STARTED (CONTINUE) : le cycle de réception repart de zéro.
  useEffect(() => {
    const currentPhase = gameState.phase
    const prev = prevPhaseRef.current
    prevPhaseRef.current = currentPhase
    if (gameState.question?.TYPE !== 'ARDOISE') return
    if (prev === 'STARTED' && (currentPhase === 'STOPPED' || currentPhase === 'PAUSED')) {
      cancelArdoiseTimer()
      const text = ardoiseTextRef.current
      // Page remontée sans texte restauré, sans saisie et sans avoir vu le départ
      // de la question : un final vide écraserait la réponse du serveur → rien.
      if (text === '' && !typedRef.current && !restoredRef.current && !sawStartRef.current) return
      finalRef.current = {
        qid: String(gameState.question?.ID ?? ''),
        text,
        hash: fnv1a(text),
        acked: false,
      }
      setFinalStatus('sending')
      sendFinal()
    } else if (currentPhase === 'STARTED') {
      if (prev && prev !== 'STARTED') sawStartRef.current = true
      finalRef.current = null
      setFinalStatus(null)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [gameState.phase])

  // Nouvelle question : plus de final en cours, indicateurs de montée remis à zéro.
  useEffect(() => {
    finalRef.current = null
    setFinalStatus(null)
    typedRef.current = false
    restoredRef.current = false
    sawStartRef.current = false
    // Passage vers un AUTRE ID de question non nul : le texte conservé pour
    // l'ancienne est périmé. A → null → A (reset transitoire à la reconnexion)
    // conserve le brouillon ; la fin de partie l'efface (effet NEW_GAME ci-dessous).
    const nextQid = gameState.question?.ID ?? null
    if (questionIdRef.current !== null && nextQid !== null && questionIdRef.current !== nextQid) {
      clearArdoiseDraft()
    }
    if (nextQid !== null) questionIdRef.current = nextQid
  }, [gameState.question?.ID])

  // Fin de partie explicite : plus aucun brouillon à conserver.
  useEffect(() => {
    if (gameState.phase === 'NEW_GAME') {
      clearArdoiseDraft()
      questionIdRef.current = null
    }
  }, [gameState.phase])

  // #246 — restauration du texte en cours après rechargement (même bumper, même
  // question). Déclaré APRÈS la remise à zéro de la saisie ci-dessus (même commit).
  useEffect(() => {
    const qid = gameState.question?.ID
    if (!playerSession?.id || qid == null || gameState.question?.TYPE !== 'ARDOISE') return
    const draft = loadArdoiseDraft(playerSession.id, qid)
    if (draft && !typedRef.current && ardoiseTextRef.current === '') {
      restoredRef.current = true
      setArdoiseText(draft)
    }
  }, [playerSession?.id, gameState.question?.ID, gameState.question?.TYPE])

  // Le final n'a de sens qu'en STOPPED/PAUSED pour SA question : dès qu'on en
  // sort (REVEAL forcé, question suivante...), on l'abandonne et on repasse à
  // un état neutre (aucun renvoi, aucun overlay d'envoi).
  useEffect(() => {
    if (finalStatus !== 'sending' && finalStatus !== 'stalled') return
    const f = finalRef.current
    const inStop = gameState.phase === 'STOPPED' || gameState.phase === 'PAUSED'
    if (!f || !inStop || String(gameState.question?.ID ?? '') !== f.qid) {
      finalRef.current = null
      setFinalStatus(null)
    }
  }, [finalStatus, gameState.phase, gameState.question?.ID])

  // Renvoi toutes les 1 s tant que l'accusé valide n'est pas reçu.
  useEffect(() => {
    if (finalStatus !== 'sending') return
    const id = setInterval(sendFinal, 1000)
    return () => clearInterval(id)
  }, [finalStatus, sendFinal])

  // Renvoi immédiat à la (re)liaison de la connexion et au retour au premier plan.
  useEffect(() => {
    if ((finalStatus === 'sending' || finalStatus === 'stalled') && bound && finalRef.current) {
      finalRef.current.sends = 0
      setFinalStatus('sending')
      sendFinal()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [bound])

  useEffect(() => {
    if (finalStatus !== 'sending' && finalStatus !== 'stalled') return
    const onVisible = () => {
      // Compte dans les 30 envois de la liaison (pas de remise à zéro ici).
      if (document.visibilityState === 'visible') sendFinal()
    }
    document.addEventListener('visibilitychange', onVisible)
    return () => document.removeEventListener('visibilitychange', onVisible)
  }, [finalStatus, sendFinal])

  // Accusé valide = même question ET même hash que le dernier final envoyé.
  useEffect(() => {
    if (!ardoiseFinalAck) return
    const f = finalRef.current
    if (!f || f.acked) return
    if (String(ardoiseFinalAck.questionId) === f.qid && ardoiseFinalAck.hash === f.hash) {
      f.acked = true
      setFinalStatus('received')
    }
  }, [ardoiseFinalAck])

  // #246 — renvoi du texte COMPLET dès que la connexion est (re)liée : les
  // frappes faites pendant la coupure/avant la liaison n'ont pas été émises.
  useEffect(() => {
    if (!bound || !bumper?.id) return
    if (gameState.phase !== 'STARTED' || gameState.question?.TYPE !== 'ARDOISE') return
    if (!ardoiseTextRef.current) return
    sendArdoise(ardoiseTextRef.current, bumper.id, 'resend')
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [bound])

  // ARDOISE: handle key input — update local state + debounced send (#117: except the
  // very first non-empty character for the current question, sent immediately so the
  // server timestamps STARTED_AT on the real first keystroke, not the first typing pause).
  const handleArdoiseChange = useCallback((text, reason = 'key') => {
    setArdoiseText(text)
    typedRef.current = true
    saveArdoiseDraft(ownIdRef.current, questionIdRef.current, text)
    ardoiseLatestRef.current = { text, reason }
    const id = () => ownIdRef.current ?? bumper?.id
    const flush = () => {
      ardoiseThrottleRef.current = null
      const latest = ardoiseLatestRef.current
      if (!latest) return
      ardoiseLatestRef.current = null
      ardoiseLastSentAtRef.current = Date.now()
      sendArdoise(latest.text, id(), latest.reason)
    }
    // Premier caractère non vide de la question : envoi immédiat (STARTED_AT).
    if (!ardoiseFirstSentRef.current && text !== '') {
      ardoiseFirstSentRef.current = true
      cancelArdoiseTimer()
      ardoiseLatestRef.current = { text, reason }
      flush()
      return
    }
    // #246 — throttle (leading + trailing) et non debounce : pendant une frappe
    // continue (jamais de pause ≥ 200 ms) le serveur reçoit un état au moins
    // toutes les 200 ms, et le DERNIER état est toujours envoyé. Un debounce
    // pur ne partait jamais et laissait admin/TV/anim vides pendant tout le chrono.
    if (ardoiseThrottleRef.current) return // un envoi est déjà programmé : il prendra le dernier état
    const wait = ARDOISE_THROTTLE_MS - (Date.now() - ardoiseLastSentAtRef.current)
    if (wait <= 0) flush()
    else ardoiseThrottleRef.current = setTimeout(flush, wait)
  }, [sendArdoise, bumper])

  // #246 — aucun envoi programmé ne doit survivre à la sortie de la page.
  useEffect(() => cancelArdoiseTimer, []) // eslint-disable-line react-hooks/exhaustive-deps

  // #246 — pull-to-refresh : le geste de tirage agit sur le défilement racine
  // (html/body), pas sur `.vplayer-page` seul. Restauré au démontage pour ne pas
  // toucher les autres pages. Aucun `beforeunload` (bloquerait les rechargements voulus).
  useEffect(() => {
    const root = document.documentElement
    const body = document.body
    const prevRoot = root.style.overscrollBehaviorY
    const prevBody = body.style.overscrollBehaviorY
    root.style.overscrollBehaviorY = 'none'
    body.style.overscrollBehaviorY = 'none'
    return () => {
      root.style.overscrollBehaviorY = prevRoot
      body.style.overscrollBehaviorY = prevBody
    }
  }, [])

  // NoSleep — requires user gesture to enable (called on fullscreen tap or first buzz)
  const activateNoSleep = useCallback(() => {
    if (!noSleepRef.current) noSleepRef.current = new NoSleep()
    if (!noSleepRef.current.isEnabled) noSleepRef.current.enable()
  }, [])

  useEffect(() => {
    return () => { if (noSleepRef.current?.isEnabled) noSleepRef.current.disable() }
  }, [])

  // Auto-fullscreen on mount; show hint if browser rejects (e.g. reconnect without user gesture)
  useEffect(() => {
    const el = document.documentElement
    const fn = el.requestFullscreen || el.webkitRequestFullscreen || el.mozRequestFullScreen
    if (fn) {
      fn.call(el).catch(() => setShowFullscreenHint(true))
    } else {
      setShowFullscreenHint(true)
    }
    const onChange = () => setIsFullscreen(!!(document.fullscreenElement || document.webkitFullscreenElement))
    document.addEventListener('fullscreenchange', onChange)
    document.addEventListener('webkitfullscreenchange', onChange)
    return () => {
      document.removeEventListener('fullscreenchange', onChange)
      document.removeEventListener('webkitfullscreenchange', onChange)
    }
  }, [])

  // Tap on fullscreen hint: enter fullscreen + activate NoSleep (user gesture)
  const handleFullscreenTap = useCallback(() => {
    setShowFullscreenHint(false)
    const el = document.documentElement
    const fn = el.requestFullscreen || el.webkitRequestFullscreen
    if (fn) fn.call(el).catch(() => {})
    activateNoSleep()
  }, [activateNoSleep])

  const handleBuzz = () => {
    if (!bumper || !bumper.id) return

    // ENTRACTE (#119) — le blocage réel est côté serveur (garde D6, seule
    // source de vérité) ; cette garde évite seulement l'envoi inutile et le
    // retour visuel trompeur (overlay "BUZZÉ !") pendant la pause. En pratique
    // l'entracte ne peut de toute façon coexister avec STARTED/PAUSED (D4),
    // donc le check de phase juste en dessous suffirait déjà — explicite ici
    // pour la clarté et la robustesse (voir plan #119, F5).
    if (gameState.entracte) return

    // Only allow buzz during STARTED or PAUSED phases
    if (gameState.phase !== 'STARTED' && gameState.phase !== 'PAUSED') return

    // Block buzz for MEMORY questions (admin controls the game)
    if (gameState.question?.TYPE === 'MEMORY') {
      console.log('[VPlayer] Buzz blocked for MEMORY question')
      return
    }

    // Block buzz for QCM questions (use QCM buttons instead)
    if (gameState.question?.TYPE === 'QCM') {
      console.log('[VPlayer] Buzz blocked for QCM question - use QCM buttons')
      return
    }

    // Block buzz for ARDOISE questions (use keyboard instead)
    if (gameState.question?.TYPE === 'ARDOISE') {
      console.log('[VPlayer] Buzz blocked for ARDOISE question - use keyboard')
      return
    }

    activateNoSleep()

    // #248 — le buzz attend l'accusé du serveur (TIME > 0 sur son bumper) et se
    // renvoie tant qu'il n'est pas venu ; pendant une coupure il est retenu et
    // envoyé à la reliaison (un seul appui par question, contexte revalidé).
    if (bumper.TIME > 0) return
    ackedSend.start('BUTTON', { ID: bumper.id, button: 'A' })
  }

  const handleQCMAnswer = (color) => {
    if (!bumper || !bumper.id) return
    if (gameState.phase !== 'STARTED') return
    if (gameState.question?.TYPE !== 'QCM') return

    if (bumper.TIME > 0) return

    // #248 — la réponse attend l'accusé du serveur (écho de son bumper, TIME > 0)
    // et se renvoie tant qu'il n'est pas venu ; un seul choix par question.
    if (ackedSend.start('VPLAYER_QCM_ANSWER', { ID: bumper.id, ANSWER_COLOR: color })) {
      pendingColorRef.current = color
    }
  }

  // #248 (lot 5b) — retournement d'une carte MEMORY / MEMOTION par la tablette. Seule l'équipe qui a la main
  // lance le parcours avec accusé (FLIP_ID, attente, renvoi) ; un tap hors tour part SANS FLIP_ID et reste
  // ignoré en silence par le serveur (décision du 2026-08-24). Un seul geste en attente à la fois.
  const handleFlipCard = (cardId, motionCardId) => {
    if (!bumper || !bumper.id) return
    const mainTeam = qType === 'MEMOTION' ? gameState.MEMOTION_CURRENT_TEAM : gameState.MEMORY_CURRENT_TEAM
    if (!team?.NAME || mainTeam !== team.NAME) {
      flipMemoryCard(cardId, motionCardId, bumper.id)
      return
    }
    if (ackedSend.status === 'sending') return
    const flipId = `f-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`
    const payload = { CARD_ID: cardId, ID: bumper.id, FLIP_ID: flipId }
    if (motionCardId) payload.MOTION_CARD_ID = motionCardId
    if (ackedSend.start('FLIP_MEMORY_CARD', payload, flipId)) {
      flipIdRef.current = flipId
      flipCardRef.current = cardId
    }
  }

  // #246 — « Reprendre ici » : cette connexion redevient la connexion liée
  // (l'autre onglet/appareil recevra à son tour DUPLICATE_SESSION).
  const handleResume = useCallback(() => {
    if (isStaleVPlayerSession(ownIdRef.current)) {
      setStale(true)
      return
    }
    setSuperseded(false)
    supersededRef.current = false
    if (playerSession) {
      sendMessage('PLAYER_CONNECT', { NAME: playerSession.name, ID: playerSession.id })
    }
  }, [playerSession, sendMessage])

  // Get player name color - always use team color when assigned (#112).
  // Fix #112 : bumper.ANSWER_COLOR n'est pas dédié à l'identité du VJoueur —
  // il est réassigné par le backend à chaque réponse QCM (ProcessButtonPress,
  // engine.go) et n'est jamais réinitialisé entre les questions. Le prioriser
  // faisait basculer le badge nom du VJoueur sur sa dernière couleur de
  // réponse QCM au lieu de la couleur de son équipe, en incohérence avec
  // l'admin et la TV qui affichent toujours team.COLOR. La couleur d'équipe
  // doit donc toujours primer ; ANSWER_COLOR ne sert plus que de repli pour
  // un VJoueur sans équipe assignée (mode solo).
  const getPlayerNameColor = () => {
    if (team?.COLOR) {
      return `rgb(${team.COLOR.join(',')})`
    }
    // Fallback for unassigned/solo VPlayers
    if (bumper?.ANSWER_COLOR && ANSWER_COLORS[bumper.ANSWER_COLOR]) {
      return ANSWER_COLORS[bumper.ANSWER_COLOR]
    }
    return null
  }

  // Get team color in rgb format
  const getTeamColor = () => {
    if (!team || !team.COLOR) return null
    return `rgb(${team.COLOR.join(',')})`
  }

  // Check if player has buzzed (TIME > 0)
  const hasBuzzed = bumper && bumper.TIME > 0

  // Check if current question is QCM
  const isQcmQuestion = gameState.question?.TYPE === 'QCM'

  // Check if current question is ARDOISE
  const isArdoiseQuestion = gameState.question?.TYPE === 'ARDOISE'

  // #246 (F-STOP) — message superposé quand le clavier est verrouillé, d'après
  // l'ACCUSÉ du serveur (maquette final_ack + DECISIONS.md : hors saisie, aucune
  // invitation à continuer ni à écrire — seulement l'état de jeu et le statut
  // de la réponse).
  let ardoiseOverlay = null
  if (isArdoiseQuestion && gameState.phase !== 'STARTED') {
    const paused = gameState.phase === 'PAUSED'
    const empty = ardoiseText === ''
    if (finalStatus === 'received') {
      ardoiseOverlay = paused
        ? { tone: 'pause', text: `⏸ Jeu en pause · ${empty ? '✓ reçu — aucune réponse écrite' : '✓ réponse reçue'}` }
        : { tone: 'ok', text: empty ? '✓ Reçu — aucune réponse écrite' : '✓ Réponse reçue' }
    } else if (finalStatus === 'stalled') {
      // Plafond de renvois atteint sans accusé : variante « ⚠ » de la maquette
      ardoiseOverlay = { tone: 'off', text: '⚠ Connexion perdue — votre réponse sera envoyée dès le retour du réseau' }
    } else if (finalStatus === 'sending') {
      if (status !== 'connected') {
        ardoiseOverlay = { tone: 'off', text: '⚠ Connexion perdue — votre réponse sera envoyée dès le retour du réseau' }
      } else if (paused) {
        ardoiseOverlay = { tone: 'pause', prefix: '⏸ Jeu en pause · ', spin: true, text: 'envoi de votre réponse…' }
      } else {
        ardoiseOverlay = { tone: 'sending', spin: true, text: 'Envoi de votre réponse…' }
      }
    } else if (paused) {
      ardoiseOverlay = { tone: 'pause', text: '⏸ Jeu en pause' }
    } else if (gameState.phase !== 'STOPPED' && gameState.phase !== 'REVEALED') {
      ardoiseOverlay = { tone: 'wait', text: '⏳ En attente…' }
    }
  }


  // #248 (maquette vplayer_qcm_answer__ack) — pastille d'état de la réponse QCM /
  // du buzz SPEEDY, même vocabulaire que l'ARDOISE (#246). « ✓ » seulement après
  // l'accusé du serveur ; jamais d'invitation à réessayer.
  let ackPill = null
  if (ackedSend.status !== 'idle' && !(isFlipQuestion && ackedSend.status === 'acked')) {
    // MEMORY/MEMOTION : pas de pastille « reçu » (maquette : la carte se retourne d'elle-même)
    const W = isFlipQuestion
      ? { lost: '⚠ Connexion perdue — la carte sera retournée dès le retour du réseau', failTx: ackedSend.failReason === 'NOT_TURN' ? '✕ Carte non retournée — ce n’est plus votre tour' : '✕ Carte non retournée', sending: 'Retournement de la carte…', resend: 'Renvoi du retournement…', paused: 'retournement en attente…' }
      : isQcmQuestion
      ? { lost: '⚠ Connexion perdue — votre réponse sera envoyée dès le retour du réseau', recv: '✓ Réponse reçue', failTx: '✕ Réponse non reçue — la question est terminée', team: 'ℹ Votre équipe a déjà répondu', sending: 'Envoi de votre réponse…', resend: 'Renvoi de votre réponse…', paused: 'envoi de votre réponse…' }
      : { lost: '⚠ Connexion perdue — votre buzz sera envoyé dès le retour du réseau', recv: '✓ Buzz reçu', failTx: '✕ Buzz non reçu — la question est terminée', team: 'ℹ Votre équipe a déjà buzzé', sending: 'Envoi de votre buzz…', resend: 'Renvoi de votre buzz…', paused: 'envoi de votre buzz…' }
    if (ackedSend.status === 'acked') ackPill = { tone: 'ok', text: W.recv }
    else if (ackedSend.status === 'served') ackPill = { tone: 'info', text: W.team }
    else if (ackedSend.status === 'failed') ackPill = { tone: 'fail', text: W.failTx }
    else if (status !== 'connected') ackPill = { tone: 'off', text: W.lost }
    else if (gameState.phase === 'PAUSED') ackPill = { tone: 'pause', prefix: '⏸ Jeu en pause · ', spin: true, text: W.paused }
    else ackPill = { tone: 'sending', spin: true, text: ackedSend.resending ? W.resend : W.sending }
  }

  // #214 — pause PROGRAMMÉE : image de fond depuis Question.MEDIA (pas
  // IMAGE_IS_CUSTOM, réservé au déclencheur manuel) — voir le commentaire
  // jumeau dans PlayerDisplay.jsx (même contrat, même logique).
  const entracteMediaUrl = (gameState.entracte && gameState.question?.TYPE === 'ENTRACTE')
    ? (gameState.question.MEDIA || null)
    : null

  // #118 (F3) : reconnectError/evictedReason sont désormais vérifiés AVANT
  // `!playerSession` — depuis #118, les deux chemins de nettoyage de session
  // (rejet de reconnexion, éviction) remettent playerSession à `null` pour
  // que la garde du minuteur de reconnexion redevienne effective. Vérifier
  // `!playerSession` en premier afficherait alors le "Chargement..." générique
  // à la place de l'écran de motif informatif au moment précis où celui-ci
  // doit apparaître.
  //
  // Fix R1 (suite, #109) : reconnexion rejetée — l'ancien bumper n'est plus
  // à nous, impossible de continuer sur cette session. Bloque l'écran avec
  // le message d'erreur, redirige automatiquement (voir handleRejoinFromScratch
  // + l'effet ci-dessus) vers '/' — EnrollPage affichera le formulaire ou
  // l'écran d'attente selon `gameState.enrollmentActive` en direct.
  if (reconnectError) {
    const redirectHint = gameState.enrollmentActive
      ? 'Redirection vers l’inscription…'
      : 'Redirection vers l’écran d’attente des inscriptions…'
    return (
      <div className="vplayer-page loading">
        <div className="vplayer-reconnect-error">
          <p className="vplayer-reconnect-error-text">{reconnectError}</p>
          <p className="vplayer-reconnect-error-hint">{redirectHint}</p>
          <button className="vplayer-reconnect-btn" onClick={handleRejoinFromScratch}>
            Rejoindre à nouveau
          </button>
        </div>
      </div>
    )
  }

  // #120 (F1/F3) — renvoi explicite (PLAYER_EVICTED ou filet de sécurité) :
  // même traitement que le rejet de reconnexion ci-dessus, motif résolu via
  // REDIRECT_MESSAGES (jamais d'écran muet — repli DEFAULT_REDIRECT_MESSAGE).
  if (evictedReason !== null) {
    const redirectHint = gameState.enrollmentActive
      ? 'Redirection vers l’inscription…'
      : 'Redirection vers l’écran d’attente des inscriptions…'
    return (
      <div className="vplayer-page loading">
        <div className="vplayer-reconnect-error">
          <p className="vplayer-reconnect-error-text">
            {REDIRECT_MESSAGES[evictedReason] || DEFAULT_REDIRECT_MESSAGE}
          </p>
          <p className="vplayer-reconnect-error-hint">{redirectHint}</p>
          <button className="vplayer-reconnect-btn" onClick={handleEvictedRedirect}>
            Rejoindre à nouveau
          </button>
        </div>
      </div>
    )
  }

  // #246 (A5.3) — onglet périmé : un autre onglet de cette tablette porte une
  // autre session. Aucune émission, aucune reconnexion, aucun effacement.
  if (stale) {
    return (
      <div className="vplayer-page loading">
        <div className="vplayer-session-screen" data-testid="vplayer-stale-tab">
          <div className="vplayer-session-ico" aria-hidden="true">🗂️</div>
          <h2>Cette page n’est plus active</h2>
          <p>
            Votre session de jeu est ouverte dans un <b>autre onglet</b> de cette
            tablette. Fermez cet onglet et utilisez l’autre.
          </p>
        </div>
      </div>
    )
  }

  // #246 (A5.6) — session reprise par une connexion plus récente.
  if (superseded) {
    return (
      <div className="vplayer-page loading">
        <div className="vplayer-session-screen" data-testid="vplayer-duplicate-session">
          <div className="vplayer-session-ico" aria-hidden="true">📱</div>
          <h2>Votre session est ouverte ailleurs</h2>
          <p>
            « {bumper?.NAME || playerSession?.name || ''} » vient d’être repris sur un autre
            onglet ou un autre appareil. Ici, plus rien n’est envoyé.
          </p>
          <button className="vplayer-reconnect-btn" onClick={handleResume}>
            Reprendre ici
          </button>
        </div>
      </div>
    )
  }

  // Show loading if no session yet (mount, or just cleared by one of the
  // redirect paths above — those are checked first, see #118 F3 note above).
  if (!playerSession) {
    return (
      <div className="vplayer-page loading">
        <div className="loading-spinner">Chargement...</div>
      </div>
    )
  }

  // #120 (F5) — état d'attente pendant la fenêtre entre PLAYER_CONNECTED et la
  // première UPDATE listant ce bumper. Purement visuel : ne déclenche ni
  // navigation ni effacement de session — c'est le filet de sécurité et
  // PLAYER_EVICTED ci-dessus qui en décident, jamais ce rendu. Remplace
  // l'écran de jeu incomplet auparavant rendu avec `bumper === null`.
  if (!bumper) {
    return (
      <div className="vplayer-page loading">
        <div className="vplayer-connecting">
          <span className="vplayer-connecting-spinner">⏳</span>
          <p>Connexion à la partie…</p>
        </div>
      </div>
    )
  }

  return (
    <div className="vplayer-page">
      {/* #118 (F4) — bandeau de connexion. Rien en fonctionnement normal ; le
          bouton buzzer reste actif dans les trois états (arbitrage
          utilisateur, cf. F7) — ce bandeau n'est jamais un `disabled`. */}
      {connectionBanner && (
        <div className={`vplayer-connection-banner ${connectionBanner}`} role="status">
          <span className="vplayer-connection-banner-dot" />
          <span>
            {connectionBanner === 'lost' ? 'Connexion perdue — reconnexion…' : 'Connexion rétablie'}
          </span>
        </div>
      )}

      {/* Fullscreen hint — shown when auto-fullscreen failed (reconnect without user gesture) */}
      {showFullscreenHint && (
        <div className="vplayer-fullscreen-hint" onClick={handleFullscreenTap}>
          <span className="vplayer-fullscreen-hint-icon">⛶</span>
          <span className="vplayer-fullscreen-hint-text">Appuyez pour le plein écran</span>
        </div>
      )}

      {/* Buzz confirmation overlay */}
      {hasBuzzed && (() => {
        const answerColor = isQcmQuestion && bumper.ANSWER_COLOR ? ANSWER_COLORS[bumper.ANSWER_COLOR] : null
        const answerLetter = isQcmQuestion && bumper.ANSWER_COLOR
          ? ['A', 'B', 'C', 'D'][['RED', 'GREEN', 'YELLOW', 'BLUE'].indexOf(bumper.ANSWER_COLOR)]
          : null
        const answerText = isQcmQuestion && bumper.ANSWER_COLOR && gameState.question?.QCM_ANSWERS
          ? gameState.question.QCM_ANSWERS[bumper.ANSWER_COLOR]
          : null
        return (
          <div
            className="vplayer-buzz-overlay"
            style={answerColor ? { '--buzz-color': answerColor } : undefined}
          >
            {(gameState.phase === 'STARTED' || gameState.phase === 'PAUSED' || gameState.phase === 'REVEALED') && (
              <>
                <div className="vplayer-buzz-checkmark">✓</div>
                <div className="vplayer-buzz-text">
                  {answerLetter && answerText ? `${answerLetter} : ${answerText}` : answerLetter ? `${answerLetter} !` : 'BUZZÉ !'}
                </div>
              </>
            )}
          </div>
        )
      })()}

      {/* ENTRACTE (#119) — le contenu de jeu existant (TV embarquée) reste
          visible mais estompé derrière le panneau. Wrapper toujours monté
          (seule la classe entracte-dim est conditionnelle) pour ne pas
          démonter/remonter PlayerDisplay au bascule. Le filtre de
          PlayerDisplay lui-même est désactivé quand isVPlayer (voir F4) :
          VPlayerPage porte son propre filtre ici, pas de double filtrage.
          --ep-transition (C3) depuis entracteConfig (diffusé, gelé pendant
          une pause active) — jamais entracteConfigSaved. */}
      <div
        className={`entracte-content${gameState.entracte ? ' entracte-dim' : ''}`}
        style={{ '--ep-transition': `${gameState.entracteConfig?.TRANSITION_MS ?? 2000}ms` }}
      >
        <PlayerDisplay
          playerName={bumper?.NAME}
          playerNameColor={getPlayerNameColor()}
          teamName={team?.NAME}
          teamColor={getTeamColor()}
          // 🔴 FIX bug QUALIF cycle 7 (#187) — playerId (bumper.id) manquait
          // totalement : sans lui, flipMemoryCard ne pouvait jamais faire
          // parvenir payload.ID au serveur, et la résolution de secours
          // (clientID) ne correspond PAS à la clé du bumper pour un VJoueur
          // (cmd/server/main.go, resolveFlipMemoryCardBumper) — tout flip
          // MEMORY depuis /player était donc ignoré, quelle que soit
          // l'équipe. Même motif que ID=bumper.id déjà envoyé par
          // ARDOISE_INPUT/VPLAYER_QCM_ANSWER/BUTTON ci-dessus.
          playerId={bumper?.id}
          isVPlayer={true}
          onMediaClick={gameState.entracte ? undefined : handleBuzz}
          onQCMAnswer={handleQCMAnswer}
          vplayerHasBuzzed={hasBuzzed}
          vplayerPending={ackedSend.status === 'sending' || ackedSend.status === 'failed'
            ? { status: ackedSend.status, color: pendingColorRef.current, cardId: isFlipQuestion ? flipCardRef.current : undefined } : null}
          onFlipCard={handleFlipCard}
        />
      </div>

      {ackPill && (
        <div className={`vplayer-ack-pill ov-${ackPill.tone}`} role="status" data-testid="vplayer-ack-pill">
          {ackPill.prefix}
          {ackPill.spin && <span className="vplayer-ack-spin">⏳</span>}
          {ackPill.spin ? ' ' : ''}
          {ackPill.text}
        </div>
      )}

      {/* ARDOISE keyboard overlay — shown for all ARDOISE phases, active only during STARTED.
          Kept OUTSIDE .entracte-content: position:fixed, même piège que le bandeau de
          connexion et l'overlay de buzz ci-dessus. En pratique ne peut pas être actif
          pendant un entracte (STARTED requis, exclu par la garde de phase D4). */}
      {isArdoiseQuestion && (
        <div className="vplayer-ardoise-container">
          <ArdoiseKeyboard
            keyboardType={gameState.question?.ARDOISE_KEYBOARD_TYPE || 'AZERTY'}
            value={ardoiseText}
            onChange={handleArdoiseChange}
            disabled={gameState.phase !== 'STARTED'}
            overlay={ardoiseOverlay}
          />
        </div>
      )}

      {/* ENTRACTE panel + cadenas — frères du contenu filtré, jamais enfants.
          Fondu groupé (#119, C3) : le cadenas apparaît/disparaît avec le
          panneau, jamais net sur un écran encore en train de s'estomper —
          deux enfants DIRECTS de la même AnimatePresence (chacun garde sa
          propre clé et sa propre animation d'exit ; un <div> englobant les
          deux empêcherait AnimatePresence de suivre leur démontage, seuls
          ses enfants directs sont interceptés). */}
      <AnimatePresence>
        {gameState.entracte && (
          <EntractePanel key="entracte-panel" config={gameState.entracteConfig} mediaUrl={entracteMediaUrl} />
        )}
        {gameState.entracte && (
          <motion.div
            key="entracte-lock"
            className="entracte-lock-wrap"
            aria-hidden="true"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: Math.max(0, Number(gameState.entracteConfig?.TRANSITION_MS ?? 2000)) / 1000 }}
          >
            <span className="entracte-lock">🔒</span>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  )
}
