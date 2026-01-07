import { NavLink } from 'react-router-dom'
import { motion } from 'framer-motion'
import './Navbar.css'

export default function Navbar({ connectionStatus = 'disconnected' }) {
  const navItems = [
    { to: '/', label: 'Jeu', icon: '🎮' },
    { to: '/scoreboard', label: 'Scores', icon: '🏆' },
    { to: '/quiz', label: 'Questions', icon: '❓' },
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
