import { NavLink, useLocation } from 'react-router-dom'
import { motion } from 'framer-motion'
import './Navbar.css'

export default function Navbar({ connectionStatus = 'disconnected', clientCounts = { admin: 0, tv: 0, vplayer: 0 }, serverVersion = '' }) {
  const location = useLocation()

  // Detect current prefix from URL (default to /admin)
  const currentPrefix = location.pathname.startsWith('/anim') ? '/anim' : '/admin'

  // Zone Jeu: pages principales du jeu (use relative paths, prefix added dynamically)
  const gameItems = [
    { path: '', label: 'Jeu', icon: '🎮' },
    { path: 'scoreboard', label: 'Scores', icon: '🏆' },
    { path: 'palmares', label: 'Palmarès', icon: '🏅' },
    { path: 'history', label: 'Historique', icon: '📜' },
  ]

  // Zone Config: configuration et gestion
  const configItems = [
    { path: 'teams', label: 'Joueurs', icon: '👥' },
    { path: 'quiz', label: 'Questions', icon: '❓' },
    { path: 'logs', label: 'Logs', icon: '📋' },
    { path: 'settings', label: 'Config', icon: '⚙️' },
  ]

  // Build full path with current prefix
  const getFullPath = (path) => path ? `${currentPrefix}/${path}` : currentPrefix

  // Check if current path matches
  const isActiveRoute = (path) => {
    const fullPath = getFullPath(path)
    return location.pathname === fullPath
  }

  const renderNavLink = (item) => (
    <NavLink
      key={item.path}
      to={getFullPath(item.path)}
      className={() => `nav-link ${isActiveRoute(item.path) ? 'active' : ''}`}
    >
      <span className="nav-icon">{item.icon}</span>
      <span className="nav-label">{item.label}</span>
    </NavLink>
  )

  return (
    <nav className="navbar">
      <div className="navbar-brand">
        <motion.span
          className="brand-logo"
          animate={{ rotate: [0, 10, -10, 0] }}
          transition={{ duration: 2, repeat: Infinity, repeatDelay: 3 }}
        >
          🐝
        </motion.span>
        <span className="brand-text">BuzzControl</span>
        <span className="version-badge" title="Version BuzzControl">
          v{serverVersion || '...'}
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
          <span className="client-count vplayer" title="Joueurs virtuels">
            <span className="count-icon">📱</span>
            <span className="count-value">{clientCounts.vplayer}</span>
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
