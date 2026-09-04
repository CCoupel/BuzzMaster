import { useState, useRef, useEffect, useMemo } from 'react'
import { NavLink, useLocation, useNavigate } from 'react-router-dom'
import { motion } from 'framer-motion'
import { useUpdates } from '../hooks/useUpdates'
import { useLightingStatus } from '../hooks/useLightingStatus'
import { lightingStateTitle } from '../utils/lightingState'
import LightingBulbIcon from './LightingBulbIcon'
import useElementHeightVar from '../hooks/useElementHeightVar'
import { useGame } from '../hooks/GameContext'
import { canToggleEntracte } from '../utils/phaseRules'
import Button from './Button'
import './Navbar.css'
import '../styles/entracte.css'

// Sévérité agrégée d'un groupe de participants (vjoueur/buzzer) à partir de
// leurs CONN_STATE respectifs. Priorité stricte : red > orange > neutre.
// "green" (reconnecté récent) compte comme connecté, n'assombrit pas le chip.
function aggregateSeverity(connStates) {
  if (connStates.some(s => s === 'red')) return 'red'
  if (connStates.some(s => s === 'orange')) return 'orange'
  return 'neutral'
}

// Calcule connectés/participants pour un type de bumper donné.
// Participant = TEAM non vide. Connecté (parmi les participants) = CONN_STATE ∈ {"", "green"}.
function computeParticipantCounts(bumpers, isType) {
  const connStates = []
  let participants = 0
  let connected = 0
  Object.values(bumpers || {}).forEach(b => {
    if (!isType(b)) return
    if (!b.TEAM) return
    participants += 1
    const state = b.CONN_STATE || ''
    connStates.push(state)
    if (state === '' || state === 'green') connected += 1
  })
  return { connected, participants, severity: aggregateSeverity(connStates) }
}

export default function Navbar({ connectionStatus = 'disconnected', clientCounts = { admin: 0, tv: 0, vplayer: 0, anim: 0 }, serverVersion = '', bumpers = {} }) {
  const location = useLocation()
  const navigate = useNavigate()
  // ENTRACTE (#119, C2) — bouton déplacé ici depuis GamePage : visible sur
  // toutes les pages admin (Navbar montée pour toutes les routes admin,
  // App.jsx:49), pas seulement /admin. Élargissement de portée assumé par le
  // plan de correction — ENTRACTE_SET reste admin-only côté serveur (D6),
  // aucune conséquence de sécurité. Règle de phase inchangée, seul son point
  // d'usage déménage (utils/phaseRules.js).
  const { gameState, setEntracte } = useGame()
  const entracteActive = !!gameState.entracte
  const canEntracteToggle = canToggleEntracte(gameState.phase, entracteActive)
  const handleToggleEntracte = () => {
    if (!canEntracteToggle) return
    setEntracte(!entracteActive)
  }
  const [isMenuOpen, setIsMenuOpen] = useState(false)
  // #175 (F3) — "arrêt demandé" : passe à true après confirmation de
  // l'entrée Quitter. Sans cela, useWebSocket reconnecte toutes les ~5s
  // indéfiniment (RECONNECT_INTERVAL) et l'utilisateur reste devant une page
  // figée portant un badge "Déconnecté", sans lien de cause à effet avec son
  // clic — c'est le seul endroit du parcours qui peut expliquer ce qui vient
  // de se passer.
  const [shutdownRequested, setShutdownRequested] = useState(false)
  const menuRef = useRef(null)
  const buttonRef = useRef(null)
  const { updateInfo, checkForUpdates } = useUpdates()
  // #207 — ampoule d'état de l'entrée « Ambiance » : au montage, toutes les
  // 30 s et après chaque enregistrement (contrat hue-bridge.md §7.1). Un pont
  // peut devenir injoignable PENDANT une session — un appel au montage seul,
  // comme useUpdates, ne suffirait pas.
  const { status: lightingStatus } = useLightingStatus()

  // #179 (F3) — mesure la hauteur RÉELLE de la Navbar (jamais garantie par
  // son CSS, qui ne déclare aucune hauteur fixe) et la partage via
  // --navbar-h (App.css F4), consommée par --admin-chrome-h. Même hook que
  // RegieMessageBar (#177/#179, useElementHeightVar) : cleanup (disconnect +
  // remise à 0px) géré par le hook, nécessaire ici aussi puisque la Navbar
  // est démontée sur les routes plein écran (App.jsx, `{!hideNavbar && ...}`).
  const navRef = useRef(null)
  useElementHeightVar(navRef, '--navbar-h')

  // Compteurs participants (X/Y) — calculés côté client depuis `bumpers`
  // (porte TEAM + CONN_STATE), pas depuis CLIENTS (qui ignore la notion d'équipe).
  const vjoueurCounts = useMemo(
    () => computeParticipantCounts(bumpers, b => b.IS_VPLAYER === true),
    [bumpers]
  )
  const buzzerCounts = useMemo(
    () => computeParticipantCounts(bumpers, b => !b.IS_VIRTUAL && !b.IS_VPLAYER),
    [bumpers]
  )

  // Vérifier les mises à jour au montage
  useEffect(() => {
    checkForUpdates()
  }, [checkForUpdates])

  // Fermeture du menu au clic extérieur
  useEffect(() => {
    function handleClickOutside(event) {
      if (menuRef.current && !menuRef.current.contains(event.target) &&
          buttonRef.current && !buttonRef.current.contains(event.target)) {
        setIsMenuOpen(false)
      }
    }

    if (isMenuOpen) {
      document.addEventListener('mousedown', handleClickOutside)
      return () => {
        document.removeEventListener('mousedown', handleClickOutside)
      }
    }
  }, [isMenuOpen, menuRef, buttonRef])

  // #175 (F3) — si le serveur redémarre et que la reconnexion aboutit après
  // un "Quitter" (ex. relancé manuellement entre-temps), l'état "arrêté"
  // n'a plus lieu d'être : la page redevient utilisable normalement.
  useEffect(() => {
    if (shutdownRequested && connectionStatus === 'connected') {
      setShutdownRequested(false)
    }
  }, [shutdownRequested, connectionStatus])

  // Navbar only ever renders on /admin/* — /anim is its own page (AnimPage)
  // and never shows this navbar (App.jsx isAdminRoute), so the prefix is a
  // constant, not derived from the URL anymore (#155/F2, was an alias before).
  const currentPrefix = '/admin'

  // Zone Jeu: pages principales du jeu (use relative paths, prefix added dynamically)
  const gameItems = [
    { path: '', label: 'Jeu', icon: '🎮' },
    { path: 'scoreboard', label: 'Scores', icon: '🏆' },
    { path: 'palmares', label: 'Palmarès', icon: '🏅' },
    { path: 'history', label: 'Historique', icon: '📜' },
  ]

  // Zone Config: configuration et gestion (sans Config et Logs qui sont dans le menu)
  const configItems = [
    { path: 'teams', label: 'Joueurs', icon: '👥' },
    { path: 'quiz', label: 'Quiz', icon: '❓' },
    // RAFALE (milestone v8.0.0, #16/#197/#198) — réservoir de questions,
    // page dédiée (App.jsx adminRoutes).
    { path: 'rafale', label: 'Rafale', icon: '⚡' },
  ]

  // Zone TV: affichage TV, joueurs et animateur
  const tvItems = [
    { path: '/tv', label: 'TV', icon: '📺', absolute: true },
    { path: '/player', label: 'Joueur', icon: '📱', absolute: true },
    { path: '/anim', label: 'Animateur', icon: '🎤', absolute: true },
  ]

  // Menu items dans le menu déroulant
  const menuItems = [
    { path: 'settings', label: 'Config', icon: '⚙️' },
    // #207 — juste après Config. L'icône est un ÉLÉMENT React (SVG en ligne,
    // 3 glyphes distincts selon l'état), pas un emoji : ni la couleur ni la
    // forme d'un emoji ne sont pilotables. `title` dit l'état en toutes
    // lettres pour les lecteurs d'écran (le SVG est aria-hidden).
    {
      path: 'ambiance',
      label: 'Ambiance',
      icon: <LightingBulbIcon state={lightingStatus.state} />,
      title: lightingStateTitle(lightingStatus.state),
    },
    { path: 'backup', label: 'Backup/Restaure', icon: '💾' },
    { path: 'updates', label: 'Mises à jour', icon: '🔄', badge: updateInfo?.update_available },
    { path: 'logs', label: 'Logs', icon: '📋' },
    // #175 (F1) — seule entrée qui soit une ACTION et non une navigation :
    // pas de `path`, jamais de NavLink/href (un href serait préchargeable
    // par le navigateur — arrêt du serveur au simple survol du menu, AC6).
    { action: 'quit', label: 'Quitter', icon: '⏻', danger: true },
  ]

  // Build full path with current prefix
  const getFullPath = (path) => path ? `${currentPrefix}/${path}` : currentPrefix

  // Check if current path matches
  const isActiveRoute = (path) => {
    const fullPath = getFullPath(path)
    return location.pathname === fullPath
  }

  // #175 (F2) — pattern établi du projet pour les gestes destructifs
  // (window.confirm, BackupPage.jsx/ConfigPage.jsx/USBConfigModal.jsx) :
  // aucun composant de dialogue dédié, ce serait un doublon.
  const handleQuit = () => {
    const confirmed = window.confirm(
      'Arrêter le serveur ? Tous les participants seront déconnectés — TV, joueurs, animateur et cette page.'
    )
    // Le menu se referme dans tous les cas (AC4/AC7), confirmé ou annulé.
    setIsMenuOpen(false)
    if (!confirmed) return
    // La requête n'aboutit pas toujours : le serveur peut mourir avant
    // d'avoir fini d'écrire la réponse HTTP. Une erreur réseau ici EST le
    // succès attendu, jamais un échec à signaler (F2).
    fetch('/shutdown').catch(() => {})
    setShutdownRequested(true)
  }

  const menuActionHandlers = { quit: handleQuit }

  const renderNavLink = (item) => {
    const path = item.absolute ? item.path : getFullPath(item.path)
    const isActive = item.absolute ? location.pathname === item.path : isActiveRoute(item.path)
    // D4 (#155) — TV, Joueur et Animateur ouvrent désormais un nouvel onglet
    // (les 3 entrées `absolute`) : changement de comportement demandé
    // explicitement, pas une régression — voir plan §7 R8/R9.
    return (
      <NavLink
        key={item.path}
        to={path}
        className={() => `nav-link ${isActive ? 'active' : ''}`}
        {...(item.absolute ? { target: '_blank', rel: 'noopener' } : {})}
      >
        <span className="nav-icon">{item.icon}</span>
        <span className="nav-label">{item.label}</span>
      </NavLink>
    )
  }

  // #175 (F3, AC8) — état "arrêt demandé" : remplace la navbar entière par
  // un message explicite, plutôt que de laisser la page figée avec un
  // simple badge "Déconnecté" sans lien de cause à effet avec le clic.
  if (shutdownRequested) {
    return (
      <nav className="navbar navbar-shutdown">
        <div className="navbar-shutdown-message">
          <span className="navbar-shutdown-icon" aria-hidden="true">⏻</span>
          Serveur arrêté — cette page n'est plus active.
        </div>
      </nav>
    )
  }

  return (
    <nav className="navbar" ref={navRef}>
      <div className="navbar-brand">
        <div className="brand-logo-container">
          <button
            ref={buttonRef}
            className="brand-logo-button"
            onClick={() => setIsMenuOpen(!isMenuOpen)}
            title="Menu"
            aria-label="Menu de navigation"
          >
            <motion.span
              className="brand-logo"
              animate={{ rotate: [0, 10, -10, 0] }}
              transition={{ duration: 2, repeat: Infinity, repeatDelay: 3 }}
            >
              🐝
            </motion.span>
            <span className="menu-indicator">▼</span>
          </button>

          {/* Menu déroulant */}
          {isMenuOpen && (
            <div ref={menuRef} className="navbar-menu-dropdown">
              {menuItems.map((item) => (
                item.action ? (
                  // #175 (F1/AC6) — action, JAMAIS une navigation : <button>
                  // uniquement, aucun `to`/`href` (préchargeable par le
                  // navigateur, ce qui arrêterait le serveur au survol).
                  <button
                    key={item.action}
                    type="button"
                    className={`menu-item menu-item-action ${item.danger ? 'menu-item-danger' : ''}`}
                    onClick={menuActionHandlers[item.action]}
                  >
                    <span className="menu-icon">{item.icon}</span>
                    <span className="menu-label">{item.label}</span>
                  </button>
                ) : (
                  <NavLink
                    key={item.path}
                    to={getFullPath(item.path)}
                    className={() => `menu-item ${isActiveRoute(item.path) ? 'active' : ''}`}
                    onClick={() => setIsMenuOpen(false)}
                    title={item.title}
                  >
                    <span className="menu-icon">{item.icon}</span>
                    <span className="menu-label">{item.label}</span>
                    {item.badge && <span className="update-badge">!</span>}
                  </NavLink>
                )
              ))}
            </div>
          )}
        </div>

        <span className="brand-text">BuzzControl</span>
        <span
          className="version-badge version-badge-clickable"
          title={updateInfo?.update_available ? 'Mise à jour disponible — cliquer pour accéder' : 'Version BuzzControl — cliquer pour les mises à jour'}
          onClick={() => navigate(getFullPath('updates'))}
          role="button"
          tabIndex={0}
          onKeyDown={e => e.key === 'Enter' && navigate(getFullPath('updates'))}
        >
          v{serverVersion || '...'}
          {updateInfo?.update_available && (
            <span className="update-badge-version" title="Mise à jour disponible">!</span>
          )}
        </span>

        <Button
          variant={entracteActive ? 'danger' : 'warning'}
          size="sm"
          className={`entracte-toggle-btn${entracteActive ? ' active' : ''}`}
          onClick={handleToggleEntracte}
          disabled={!canEntracteToggle}
          title={!canEntracteToggle ? "Désactivé pendant une question en cours" : undefined}
        >
          {entracteActive ? "FIN D'ENTRACTE" : 'ENTRACTE'}
        </Button>
      </div>

      <div className="navbar-links">
        <div className="nav-group nav-group-game">
          <span className="nav-group-label">Jeu</span>
          <div className="nav-group-items">
            {gameItems.map(renderNavLink)}
          </div>
        </div>
        <div className="nav-group nav-group-config">
          <span className="nav-group-label">Config</span>
          <div className="nav-group-items">
            {configItems.map(renderNavLink)}
          </div>
        </div>
        <div className="nav-group nav-group-tv">
          <span className="nav-group-label">Pages</span>
          <div className="nav-group-items">
            {tvItems.map(renderNavLink)}
          </div>
        </div>
      </div>

      <div className="navbar-status">
        <div className="client-counts">
          <span className="client-count admin" title="Interfaces admin">
            <span className="count-icon">A</span>
            <span className="count-value">{clientCounts.admin}</span>
          </span>
          <span className="client-count tv" title="Ecrans TV/joueurs">
            <span className="count-icon">TV</span>
            <span className="count-value">{clientCounts.tv}</span>
          </span>
          <span className="client-count anim" title="Interfaces animateur">
            <span className="count-icon">🎤</span>
            <span className="count-value">{clientCounts.anim}</span>
          </span>
          <span
            className={`client-count vplayer severity-${vjoueurCounts.severity}`}
            title={`VJoueurs connectés/participants : ${vjoueurCounts.connected}/${vjoueurCounts.participants}`}
          >
            <span className="count-icon">📱</span>
            <span className="count-value">{vjoueurCounts.connected}/{vjoueurCounts.participants}</span>
          </span>
          <span
            className={`client-count buzzer severity-${buzzerCounts.severity}`}
            title={`Buzzers connectés/participants : ${buzzerCounts.connected}/${buzzerCounts.participants}`}
          >
            <span className="count-icon">🎮</span>
            <span className="count-value">{buzzerCounts.connected}/{buzzerCounts.participants}</span>
          </span>
        </div>
        <div className={`connection-status ${connectionStatus}`}>
          <span className="status-dot" />
          <span className="status-text">
            {connectionStatus === 'connected' ? 'Connecte' :
             connectionStatus === 'connecting' ? 'Connexion...' : 'Deconnecte'}
          </span>
        </div>
      </div>
    </nav>
  )
}
