import { NavLink, useLocation } from 'react-router-dom'
import { motion } from 'framer-motion'
import './Navbar.css'

export default function Navbar({ connectionStatus = 'disconnected', clientCounts = { admin: 0, tv: 0 }, serverVersion = '' }) {
  const location = useLocation()

  // Zone Jeu: pages principales du jeu
  const gameItems = [
    { to: '/admin', label: 'Jeu', icon: '🎮' },
    { to: '/admin/scoreboard', label: 'Scores', icon: '🏆' },
    { to: '/admin/palmares', label: 'Palmarès', icon: '🏅' },
    { to: '/admin/history', label: 'Historique', icon: '📜' },
  ]

  // Zone Config: configuration et gestion
  const configItems = [
    { to: '/admin/teams', label: 'Joueurs', icon: '👥' },
    { to: '/admin/quiz', label: 'Questions', icon: '❓' },
    { to: '/admin/settings', label: 'Config', icon: '⚙️' },
  ]

  // Check if current path matches (works for both /admin and /anim prefixes)
  const isActiveRoute = (to) => {
    const adminPath = to
    const animPath = to.replace('/admin', '/anim')
    return location.pathname === adminPath || location.pathname === animPath
  }

  const renderNavLink = (item) => (
    <NavLink
      key={item.to}
      to={item.to}
      className={() => `nav-link ${isActiveRoute(item.to) ? 'active' : ''}`}
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
