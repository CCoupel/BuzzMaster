import { useState, useRef, useEffect, useMemo } from 'react'
import { NavLink, useLocation, useNavigate } from 'react-router-dom'
import { motion } from 'framer-motion'
import { useUpdates } from '../hooks/useUpdates'
import './Navbar.css'

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
  const [isMenuOpen, setIsMenuOpen] = useState(false)
  const menuRef = useRef(null)
  const buttonRef = useRef(null)
  const { updateInfo, checkForUpdates } = useUpdates()

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
    { path: 'backup', label: 'Backup/Restaure', icon: '💾' },
    { path: 'updates', label: 'Mises à jour', icon: '🔄', badge: updateInfo?.update_available },
    { path: 'logs', label: 'Logs', icon: '📋' },
  ]

  // Build full path with current prefix
  const getFullPath = (path) => path ? `${currentPrefix}/${path}` : currentPrefix

  // Check if current path matches
  const isActiveRoute = (path) => {
    const fullPath = getFullPath(path)
    return location.pathname === fullPath
  }

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

  return (
    <nav className="navbar">
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
                <NavLink
                  key={item.path}
                  to={getFullPath(item.path)}
                  className={() => `menu-item ${isActiveRoute(item.path) ? 'active' : ''}`}
                  onClick={() => setIsMenuOpen(false)}
                >
                  <span className="menu-icon">{item.icon}</span>
                  <span className="menu-label">{item.label}</span>
                  {item.badge && <span className="update-badge">!</span>}
                </NavLink>
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
