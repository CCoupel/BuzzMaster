import { useEffect, useRef } from 'react'
import { NavLink } from 'react-router-dom'
import './NavGroupMenu.css'

// Ouverture au survol (délai de fermeture 150 ms) ET au clic/tap/clavier.
// L'état « quel menu est ouvert » vit dans Navbar (un seul menu à la fois).
export function useMenuTrigger(id, openMenu, setOpenMenu) {
  const timer = useRef(null)
  const byHover = useRef(false)
  const isOpen = openMenu === id

  useEffect(() => () => clearTimeout(timer.current), [])

  return {
    isOpen,
    wrapperProps: {
      'data-navmenu': id,
      onMouseEnter: () => {
        clearTimeout(timer.current)
        if (!isOpen) {
          byHover.current = true
          setOpenMenu(id)
        }
      },
      onMouseLeave: () => {
        clearTimeout(timer.current)
        timer.current = setTimeout(() => setOpenMenu(cur => (cur === id ? null : cur)), 150)
      },
    },
    onClick: () => {
      clearTimeout(timer.current)
      if (isOpen && !byHover.current) {
        setOpenMenu(null)
      } else {
        byHover.current = false
        setOpenMenu(id)
      }
    },
  }
}

function MenuEntry({ item, to, active, onNavigate }) {
  return (
    <NavLink
      to={to}
      className={() => `menu-item ${active ? 'active' : ''}`}
      onClick={onNavigate}
      {...(item.absolute ? { target: '_blank', rel: 'noopener' } : {})}
    >
      <span className="menu-icon">{item.icon}</span>
      <span className="menu-label">{item.label}</span>
      {item.absolute && <span className="menu-external" aria-hidden="true">↗</span>}
    </NavLink>
  )
}

function Dropdown({ children, label }) {
  return (
    <div className="navbar-menu-dropdown navgroup-dropdown" role="menu" aria-label={label}>
      {children}
    </div>
  )
}

/**
 * Groupe « Préparation » (Joueurs, Quiz, Backstage) + « Interface » (TV,
 * Joueur, Animateur, nouvel onglet).
 * - mode "inline"  (≥ 1685 px) : entrées Préparation en ligne + bouton « Interface ▾ » séparé ;
 * - mode "single"  (< 1685 px) : un seul bouton « 🛠️ Préparation ▾ », section INTERFACE dans la liste.
 */
export default function NavGroupMenu({
  mode, prepItems, interfaceItems, renderNavLink, getFullPath, isActiveRoute, pathname,
  openMenu, setOpenMenu,
}) {
  const prep = useMenuTrigger('prep', openMenu, setOpenMenu)
  const iface = useMenuTrigger('interface', openMenu, setOpenMenu)
  const close = () => setOpenMenu(null)
  const ifaceActive = interfaceItems.some(i => pathname === i.path)
  const prepActive = prepItems.some(i => isActiveRoute(i.path))

  const prepEntries = prepItems.map(i => (
    <MenuEntry key={i.path} item={i} to={getFullPath(i.path)} active={isActiveRoute(i.path)} onNavigate={close} />
  ))
  const ifaceEntries = interfaceItems.map(i => (
    <MenuEntry key={i.path} item={i} to={i.path} active={pathname === i.path} onNavigate={close} />
  ))

  if (mode === 'inline') {
    return (
      <div className="nav-group nav-group-prep">
        <div className="nav-group-items">
          {prepItems.map(renderNavLink)}
        </div>
        <div className="navgroup-wrapper" {...iface.wrapperProps}>
          <button
            type="button"
            className={`navgroup-button navgroup-interface ${ifaceActive ? 'active' : ''}`}
            title="Interface"
            aria-label="Interface"
            aria-haspopup="true"
            aria-expanded={iface.isOpen}
            onClick={iface.onClick}
          >
            <span className="nav-icon" aria-hidden="true">🖥️</span>
            <span className="navgroup-label">Interface</span>
            <span className="navgroup-caret" aria-hidden="true">▾</span>
          </button>
          {iface.isOpen && <Dropdown label="Interface">{ifaceEntries}</Dropdown>}
        </div>
      </div>
    )
  }

  return (
    <div className="nav-group nav-group-prep">
      <div className="navgroup-wrapper" {...prep.wrapperProps}>
        <button
          type="button"
          className={`navgroup-button ${prepActive || ifaceActive ? 'active' : ''}`}
          title="Préparation"
          aria-label="Préparation"
          aria-haspopup="true"
          aria-expanded={prep.isOpen}
          onClick={prep.onClick}
        >
          <span className="nav-icon" aria-hidden="true">🛠️</span>
          <span className="navgroup-caret" aria-hidden="true">▾</span>
        </button>
        {prep.isOpen && (
          <Dropdown label="Préparation">
            {prepEntries}
            <div className="menu-section-title">Interface</div>
            {ifaceEntries}
          </Dropdown>
        )}
      </div>
    </div>
  )
}
