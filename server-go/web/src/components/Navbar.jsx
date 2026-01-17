import { NavLink } from 'react-router-dom'
import { motion } from 'framer-motion'
import './Navbar.css'

export default function Navbar({ connectionStatus = 'disconnected', clientCounts = { admin: 0, tv: 0 }, serverVersion = '' }) {
  const navItems = [
    { to: '/', label: 'Jeu', icon: '🎮' },
    { to: '/scoreboard', label: 'Scores', icon: '🏆' },
    { to: '/teams', label: 'Equipes', icon: '👥' },
    { to: '/quiz', label: 'Questions', icon: '❓' },
    { to: '/history-page', label: 'Historique', icon: '📜' },
    { to: '/palmares', label: 'Palmares', icon: '🏅' },
    { to: '/settings', label: 'Config', icon: '⚙️' },
  ]

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
        <div className="version-badges">
          <span className="version-badge server" title="Version serveur Go">
            <span className="version-icon">🖥️</span>
            <span className="version-value">{serverVersion || '...'}</span>
          </span>
          <span className="version-badge web" title="Version interface web React">
            <span className="version-icon">🌐</span>
            <span className="version-value">2.3.0</span>
          </span>
        </div>
      </div>

      <div className="navbar-links">
        {navItems.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            className={({ isActive }) =>
              `nav-link ${isActive ? 'active' : ''}`
            }
          >
            <span className="nav-icon">{item.icon}</span>
            <span className="nav-label">{item.label}</span>
          </NavLink>
        ))}
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
