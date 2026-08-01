import { useState, useEffect, useRef, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { useGame } from '../hooks/GameContext'
import PlayerDisplay from './PlayerDisplay'
import ArdoiseKeyboard from '../components/ArdoiseKeyboard'
import NoSleep from 'nosleep.js'
import { REJECTION_MESSAGES, DEFAULT_REJECTION_MESSAGE, REDIRECT_MESSAGES, DEFAULT_REDIRECT_MESSAGE } from '../utils/playerConnectMessages'
import { clearVPlayerSession } from '../utils/vplayerSession'
import './VPlayerPage.css'

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

export default function VPlayerPage() {
  const navigate = useNavigate()
  const {
    sendMessage, gameState, bumpers, teams, status,
    playerConnectStatus, clearPlayerConnectStatus,
    playerEvictedStatus, clearPlayerEvictedStatus,
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
  // #118 (F7) — appui buzzer mémorisé pendant une liaison morte : { questionId }
  // ou `null`. Un seul appui retenu (le premier), en mémoire uniquement (pas
  // de localStorage — un rechargement repart légitimement de zéro, et F1 est
  // précisément ce qui rend ce rechargement inutile).
  const pendingBuzzRef = useRef(null)
  const noSleepRef = useRef(null)
  // Ref to always have the latest bumper value in async callbacks (avoids stale closure)
  const bumperRef = useRef(null)

  // ARDOISE: local text state + throttle ref
  const [ardoiseText, setArdoiseText] = useState('')
  const ardoiseThrottleRef = useRef(null)
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

    if (!savedName || !savedSession) {
      // No session, redirect to enroll page
      navigate('/')
      return
    }

    setPlayerSession({
      name: savedName,
      sessionId: savedSession,
      // F2 (#120): identity by ID, the backend's sole source of truth since
      // #109 R1. `null` for a session created before this version — the name
      // fallback below is transitory, expected to fade out as old sessions expire.
      id: savedId || null,
    })
  }, [navigate])

  // Find bumper by ID in bumpers list (#120 F2) — falls back to name only
  // when the session predates ID-based identity (no vplayer_id stored yet).
  useEffect(() => {
    if (!playerSession || !bumpers) return

    let bumperId = null
    let bumperData = null
    if (playerSession.id) {
      if (bumpers[playerSession.id]) {
        bumperId = playerSession.id
        bumperData = bumpers[playerSession.id]
      }
      // Deliberately no name fallback once an ID is known: comparing by name
      // when a real ID is available is exactly the ambiguity #120 removes.
    } else {
      const found = Object.entries(bumpers).find(([_, b]) =>
        b.IS_VIRTUAL && b.NAME === playerSession.name
      )
      if (found) [bumperId, bumperData] = found
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
    // Fast-follow (#120 code-review [MAJEUR]) : effacer l'ID immédiatement, ne
    // pas attendre le délai de lecture de 3s (celui-ci ne doit retarder que le
    // bandeau/la navigation). Sans ça, si le minuteur de reconnexion hérité de
    // #109 (ci-dessous) atteint son échéance de 2s avant que la session ne
    // soit effacée, il retrouve `vplayer_id` encore présent et renvoie
    // PLAYER_CONNECT avec un ID déjà supprimé — le serveur le traite comme
    // absent et recrée un VJoueur fantôme sous le même nom.
    localStorage.removeItem('vplayer_id')
    setEvictedReason(playerEvictedStatus.reason ?? '')
    clearPlayerEvictedStatus()
  }, [playerEvictedStatus, clearPlayerEvictedStatus])

  // #120 — filet de sécurité (cf. risques du plan) : si le bumper ne se
  // matérialise jamais (chemin de suppression oublié côté serveur), ne pas
  // laisser le joueur bloqué indéfiniment sur l'état d'attente (F5). Purement
  // défensif : ne s'arme que 10 s après un état stable sans bumper, très
  // au-delà de la fenêtre d'enrôlement normale — n'interfère jamais avec F5,
  // dont le rendu reste inconditionnel et sans effet de bord.
  useEffect(() => {
    if (!playerSession || status !== 'connected') return
    if (bumper || reconnectError || evictedReason !== null) return

    const timeoutId = setTimeout(() => {
      console.log('[VPlayer] Safety net: bumper never appeared, redirecting (SESSION_EXPIRED)')
      setEvictedReason('SESSION_EXPIRED')
    }, SESSION_EXPIRED_SAFETY_NET_MS)

    return () => clearTimeout(timeoutId)
  }, [playerSession, status, bumper, reconnectError, evictedReason])

  // Auto-reconnect if session exists but bumper not found after initial state sync
  // Uses bumperRef (not bumper state) to read the latest value inside the timeout callback,
  // preventing the stale closure bug that caused PLAYER_CONNECT to be sent even when the
  // bumper was already found (which could create duplicate VJoueur entries on the server).
  useEffect(() => {
    if (!playerSession || status !== 'connected') return
    // If we already found the bumper, no need to reconnect
    if (bumperRef.current) return
    // Fast-follow (#120 code-review [MAJEUR]) : une éviction déjà armée ferme
    // définitivement toute tentative de reconnexion pour cette session — sans
    // ce garde, un `PLAYER_EVICTED` reçu avant l'échéance de 2s laissait ce
    // minuteur, hérité de #109 et indifférent à `evictedReason`, retenter un
    // PLAYER_CONNECT pendant que le client se redirigeait déjà vers
    // l'inscription. `evictedReason` étant dans les dépendances, ce garde
    // s'applique aussi bien à l'exécution initiale qu'à un passage à non-null
    // en cours de route : le nettoyage ci-dessous annule alors le minuteur
    // déjà programmé avant qu'il ne puisse se déclencher.
    if (evictedReason !== null) return
    // #118 (F3, R1) : même raisonnement pour reconnectError, ajouté par le
    // filet de sécurité voisin mais oublié ici — un rejet de reconnexion ne
    // doit pas non plus laisser ce minuteur se réarmer pendant les 3s de
    // lecture qui précèdent le renvoi.
    if (reconnectError) return

    const attemptReconnect = () => {
      // Check ref (latest value) rather than closure-captured bumper
      if (bumperRef.current) return
      // Fix R1 (#109) : renvoyer l'ID capturé au précédent PLAYER_CONNECTED
      // pour une reconnexion sans ambiguïté (lookup par ID côté backend,
      // plus par nom — évite qu'un nouvel enrôlement homonyme ne vole la
      // session). Omis si absent (jamais connecté depuis cet appareil).
      const storedId = localStorage.getItem('vplayer_id')
      console.log('[VPlayer] Reconnecting with name:', playerSession.name, 'id:', storedId)
      sendMessage('PLAYER_CONNECT', storedId
        ? { NAME: playerSession.name, ID: storedId }
        : { NAME: playerSession.name })
    }

    if (playerSession.id) {
      // #118 (F2, R4) : un ID est déjà connu — le serveur envoie l'état dès
      // le HELLO (sendStateToClient) et sait donc déjà que ce bumper existe.
      // Attendre n'apporte rien ici et ouvrait la fenêtre d'échec de #118 :
      // sur un lien instable, ces 2s pouvaient ne jamais s'écouler à
      // l'intérieur d'une seule fenêtre "connected" ininterrompue, et aucun
      // PLAYER_CONNECT n'était alors jamais émis. Émission immédiate, plus
      // une reprise à 2s en cas de message perdu (l'opération est idempotente
      // côté serveur — cas 1 de ReconnectOrCreateVirtualPlayer).
      attemptReconnect()
      const retryId = setTimeout(attemptReconnect, 2000)
      return () => clearTimeout(retryId)
    }

    // Premier enrôlement, aucun ID connu : comportement #109 inchangé — le
    // serveur n'a ici rien à réassocier immédiatement, l'attente de
    // synchronisation initiale garde son sens.
    const timeoutId = setTimeout(attemptReconnect, 2000)
    return () => clearTimeout(timeoutId)
  }, [playerSession, status, sendMessage, evictedReason, reconnectError])

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
      clearVPlayerSession()
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

    clearPlayerConnectStatus()
  }, [playerConnectStatus, clearPlayerConnectStatus])

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
    clearVPlayerSession()
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
    clearVPlayerSession()
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
    // Send PONG - ID in payload for web clients
    sendMessage('PONG', { ID: bumper.id })
  }, [gameState.phase, bumper, sendMessage])

  // #118 (F7, déclencheur a) — passage observé en PREPARE : la question
  // change, tout buzz mémorisé pour l'ancienne question est périmé. Vidage
  // SANS envoi. Ne couvre que le cas où ce client est resté en ligne pendant
  // le changement de question — voir le déclencheur (b) ci-dessous pour le
  // cas complémentaire (hors ligne pendant tout le changement).
  useEffect(() => {
    if (gameState.phase !== 'PREPARE') return
    if (pendingBuzzRef.current) {
      console.log('[VPlayer] Buzz mémorisé abandonné (nouvelle question observée en PREPARE)')
      pendingBuzzRef.current = null
    }
  }, [gameState.phase])

  // #118 (F7, déclencheur b — le point de vigilance du plan) — validation de
  // contexte au moment du vidage, déclenchée par la reconfirmation du bumper
  // (bumper.CONNECTED redevenu true, cf. F2). C'est la garde qui couvre un
  // client resté hors ligne pendant TOUT le changement de question : il n'a
  // alors jamais observé PREPARE, donc le déclencheur (a) ne peut pas
  // s'appliquer. Envoie le buzz mémorisé UNIQUEMENT si la question est
  // toujours celle mémorisée ET si la phase est toujours STARTED ; sinon,
  // abandon silencieux. Le buzz est consommé (pendingBuzzRef vidé) dans les
  // deux cas — jamais rejoué.
  useEffect(() => {
    const pending = pendingBuzzRef.current
    if (!pending) return
    if (!bumper?.id || bumper.CONNECTED !== true) return // reconnexion pas encore confirmée

    pendingBuzzRef.current = null

    const sameQuestion = pending.questionId !== null && pending.questionId === (gameState.question?.ID ?? null)
    const stillStarted = gameState.phase === 'STARTED'

    if (!sameQuestion || !stillStarted) {
      console.log('[VPlayer] Buzz mémorisé abandonné (contexte périmé après reconnexion : question ou phase différente)')
      return
    }

    // Chemin identique à un buzz normal : résolution par payload.ID côté
    // serveur, horodatage serveur fait foi — aucun horodatage client transmis.
    console.log('[VPlayer] Envoi du buzz mémorisé après reconnexion:', bumper.id)
    sendMessage('BUTTON', { ID: bumper.id, button: 'A' })
  }, [bumper, gameState.phase, gameState.question?.ID, sendMessage])

  // ARDOISE: reset text on question change
  useEffect(() => {
    setArdoiseText('')
    if (ardoiseThrottleRef.current) clearTimeout(ardoiseThrottleRef.current)
    ardoiseFirstSentRef.current = false
  }, [gameState.question?.ID])

  // ARDOISE: reset on PREPARE (covers replaying the same question + race conditions)
  useEffect(() => {
    if (gameState.phase !== 'PREPARE') return
    setArdoiseText('')
    if (ardoiseThrottleRef.current) clearTimeout(ardoiseThrottleRef.current)
    ardoiseFirstSentRef.current = false
  }, [gameState.phase])

  // ARDOISE #117: also rearm the immediate-first-send flag on entry into STARTED, so a
  // question restarted without passing through PREPARE still measures the real first key.
  useEffect(() => {
    if (gameState.phase !== 'STARTED') return
    ardoiseFirstSentRef.current = false
  }, [gameState.phase, gameState.question?.ID])

  // ARDOISE: forced flush on phase change from STARTED → anything else
  useEffect(() => {
    const currentPhase = gameState.phase
    const prev = prevPhaseRef.current
    prevPhaseRef.current = currentPhase
    if (prev === 'STARTED' && currentPhase !== 'STARTED' && ardoiseText) {
      // Cancel pending throttle and send immediately
      if (ardoiseThrottleRef.current) clearTimeout(ardoiseThrottleRef.current)
      sendMessage('ARDOISE_INPUT', { TEXT: ardoiseText, ID: bumper?.id })
    }
  }, [gameState.phase, ardoiseText, sendMessage, bumper])

  // ARDOISE: handle key input — update local state + debounced send (#117: except the
  // very first non-empty character for the current question, sent immediately so the
  // server timestamps STARTED_AT on the real first keystroke, not the first typing pause).
  const handleArdoiseChange = useCallback((text) => {
    setArdoiseText(text)
    if (ardoiseThrottleRef.current) clearTimeout(ardoiseThrottleRef.current)
    if (!ardoiseFirstSentRef.current && text !== '') {
      ardoiseFirstSentRef.current = true
      sendMessage('ARDOISE_INPUT', { TEXT: text, ID: bumper?.id })
      return
    }
    ardoiseThrottleRef.current = setTimeout(() => {
      sendMessage('ARDOISE_INPUT', { TEXT: text, ID: bumper?.id })
    }, 200)
  }, [sendMessage, bumper])

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

    // #118 (F7) — arbitrage utilisateur : le buzzer reste actif pendant une
    // coupure (jamais désactivé), mais l'appui ne peut pas atteindre le
    // serveur. Le mémoriser au lieu de le perdre — un seul appui retenu, le
    // premier de cette coupure (ProcessButtonPress côté serveur ne retient de
    // toute façon que le premier appui d'un bumper pour une question donnée).
    // Envoi différé et validation de contexte : voir les effets F7 ci-dessus.
    if (status !== 'connected') {
      if (!pendingBuzzRef.current) {
        pendingBuzzRef.current = { questionId: gameState.question?.ID ?? null }
        console.log('[VPlayer] Buzz mémorisé pendant la coupure (question', gameState.question?.ID, ')')
      }
      return
    }

    console.log('[VPlayer] Buzzing:', bumper.id)
    sendMessage('BUTTON', { ID: bumper.id, button: 'A' })
  }

  const handleQCMAnswer = (color) => {
    if (!bumper || !bumper.id) return
    if (gameState.phase !== 'STARTED') return
    if (gameState.question?.TYPE !== 'QCM') return

    console.log('[VPlayer] QCM answer:', color)

    // Send VPLAYER_QCM_ANSWER message - wait for server confirmation
    sendMessage('VPLAYER_QCM_ANSWER', { ID: bumper.id, ANSWER_COLOR: color })
  }

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

      <PlayerDisplay
        playerName={bumper?.NAME}
        playerNameColor={getPlayerNameColor()}
        teamName={team?.NAME}
        teamColor={getTeamColor()}
        isVPlayer={true}
        onMediaClick={handleBuzz}
        onQCMAnswer={handleQCMAnswer}
        vplayerHasBuzzed={hasBuzzed}
      />

      {/* ARDOISE keyboard overlay — shown for all ARDOISE phases, active only during STARTED */}
      {isArdoiseQuestion && (
        <div className="vplayer-ardoise-container">
          <ArdoiseKeyboard
            keyboardType={gameState.question?.ARDOISE_KEYBOARD_TYPE || 'AZERTY'}
            value={ardoiseText}
            onChange={handleArdoiseChange}
            disabled={gameState.phase !== 'STARTED'}
          />
        </div>
      )}
    </div>
  )
}
