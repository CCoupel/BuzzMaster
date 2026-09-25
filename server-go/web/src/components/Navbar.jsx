import { useState, useRef, useEffect, useMemo } from 'react'
import { NavLink, useLocation, useNavigate } from 'react-router-dom'
import BrandLogo from './BrandLogo'
import { useUpdates } from '../hooks/useUpdates'
import { useLightingStatus } from '../hooks/useLightingStatus'
import { lightingStateTitle, normalizeLightingState } from '../utils/lightingState'
import LightingBulbIcon from './LightingBulbIcon'
import LightingModePanel from './LightingModePanel'
import { useSoundStatus } from '../hooks/useSoundStatus'
import { soundStateTitle } from '../utils/soundState'
import SoundSpeakerIcon from './SoundSpeakerIcon'
import useElementHeightVar from '../hooks/useElementHeightVar'
import { useGame } from '../hooks/GameContext'
import { canToggleEntracte } from '../utils/phaseRules'
import Button from './Button'
import NavGroupMenu from './NavGroupMenu'
import useMediaQuery from '../hooks/useMediaQuery'
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
  // Un seul menu ouvert à la fois (#238) : 'logo' | 'prep' | 'interface' |
  // 'lighting' | 'counts' | null.
  const [openMenu, setOpenMenu] = useState(null)
  const isMenuOpen = openMenu === 'logo'
  const setIsMenuOpen = (v) => setOpenMenu(v ? 'logo' : null)
  // ≥ 1500 px : Préparation en ligne + bouton Interface ; sinon menu unique.
  const inlineGroups = useMediaQuery('(min-width: 1500px)', true)
  // < 810 px : les 5 compteurs se replient dans un badge 👥.
  const compactCounts = useMediaQuery('(max-width: 809px)', false)
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
  // #234 — même mécanique, jumelle (hooks/useSoundStatus.js, livré en #230) :
  // sondage 30 s + événement de réveil indépendant, pour le second glyphe de
  // l'entrée « Ambiance » ci-dessous. Aucun second polling de l'éclairage
  // introduit — deux hooks, deux intervalles indépendants, comme prévu par
  // le handoff (§4, "Second sondage réseau").
  const { status: soundStatus } = useSoundStatus()
  // #208 — point d'accès UNIQUE aux commandes ON/AUTO/OFF/Flash (retour
  // utilisateur QUALIF v10.0.0.13, 2026-09-07) : d'abord un simple bandeau
  // d'avertissement renvoyant vers GamePage (SHA 110dfff3), le panneau
  // LUI-MÊME y a ensuite vécu un instant (SHA 30ff5abe) avant d'être retiré
  // le même jour — décision finale : les commandes ne vivent QUE dans la
  // Navbar, jamais sur un écran particulier, pour rester à portée de main
  // depuis TOUTE page admin. Ce bouton est donc désormais visible dès que
  // l'éclairage est configuré (pas seulement quand un mode est engagé) et
  // ouvre un popover contenant LightingModePanel tel quel (composant
  // inchangé, seul son point de montage a bougé).
  // Réutilise l'instance useLightingStatus() déjà interrogée ici pour
  // l'ampoule du menu Ambiance — mode/flash sont déjà dans la même réponse
  // (contrat lighting.md §10.1), aucun second polling introduit.
  const lightingConfigured = normalizeLightingState(lightingStatus.state) !== 'disabled'
  // Repli défensif : useLightingStatus() applique ce défaut lui-même
  // (EMPTY_LIGHTING_STATUS), mais un mock de test ou un serveur antérieur
  // au Batch 2 peut renvoyer un statut sans `mode` — jamais planter dessus.
  const lightingMode = lightingStatus.mode || 'AUTO'
  const lightingPopoverOpen = openMenu === 'lighting'
  const setLightingPopoverOpen = (fn) =>
    setOpenMenu(cur => ((typeof fn === 'function' ? fn(cur === 'lighting') : fn) ? 'lighting' : null))
  const lightingButtonRef = useRef(null)
  const lightingPopoverRef = useRef(null)

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

  const counterItems = [
    { key: 'admin', cls: 'admin', icon: 'A', value: clientCounts.admin, title: 'Interfaces admin', name: 'Admin' },
    { key: 'tv', cls: 'tv', icon: 'TV', value: clientCounts.tv, title: 'Ecrans TV/joueurs', name: 'TV/joueurs' },
    { key: 'anim', cls: 'anim', icon: '🎤', value: clientCounts.anim, title: 'Interfaces animateur', name: 'Animateur' },
    {
      key: 'vplayer', cls: `vplayer severity-${vjoueurCounts.severity}`, icon: '📱',
      value: `${vjoueurCounts.connected}/${vjoueurCounts.participants}`,
      title: `VJoueurs connectés/participants : ${vjoueurCounts.connected}/${vjoueurCounts.participants}`, name: 'VJoueurs',
    },
    {
      key: 'buzzer', cls: `buzzer severity-${buzzerCounts.severity}`, icon: '🎮',
      value: `${buzzerCounts.connected}/${buzzerCounts.participants}`,
      title: `Buzzers connectés/participants : ${buzzerCounts.connected}/${buzzerCounts.participants}`, name: 'Buzzers',
    },
  ]
  // Badge 👥 (< 810 px) : joueurs = VJoueurs + Buzzers, sévérité la plus grave.
  const countsBadge = {
    connected: vjoueurCounts.connected + buzzerCounts.connected,
    participants: vjoueurCounts.participants + buzzerCounts.participants,
    severity: aggregateSeverity([vjoueurCounts.severity, buzzerCounts.severity]),
  }

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

  // Fermeture du popover Éclairage au clic extérieur — même patron que le
  // menu abeille ci-dessus, refs et état dédiés (deux popovers indépendants
  // peuvent en théorie être ouverts en même temps, chacun se ferme sur son
  // propre clic extérieur).
  useEffect(() => {
    function handleClickOutsideLighting(event) {
      if (lightingPopoverRef.current && !lightingPopoverRef.current.contains(event.target) &&
          lightingButtonRef.current && !lightingButtonRef.current.contains(event.target)) {
        setLightingPopoverOpen(false)
      }
    }

    if (lightingPopoverOpen) {
      document.addEventListener('mousedown', handleClickOutsideLighting)
      return () => {
        document.removeEventListener('mousedown', handleClickOutsideLighting)
      }
    }
  }, [lightingPopoverOpen])

  // Échap ferme le menu ouvert ; clic extérieur pour les menus #238.
  useEffect(() => {
    if (!openMenu) return undefined
    const onKey = (e) => { if (e.key === 'Escape') setOpenMenu(null) }
    const onDown = (e) => {
      if (['prep', 'interface', 'counts'].includes(openMenu) &&
          !e.target.closest?.(`[data-navmenu="${openMenu}"]`)) {
        setOpenMenu(null)
      }
    }
    document.addEventListener('keydown', onKey)
    document.addEventListener('mousedown', onDown)
    return () => {
      document.removeEventListener('keydown', onKey)
      document.removeEventListener('mousedown', onDown)
    }
  }, [openMenu])

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

  // Groupe Préparation (#238) : préparation de la partie
  const configItems = [
    { path: 'teams', label: 'Joueurs', icon: '👥' },
    { path: 'quiz', label: 'Quiz', icon: '❓' },
    // Backstage (#215, milestone v9.0.0) — préparation de la partie (Quiz
    // méta/Entracte/Fonds d'écran), extraite de la page Quiz. L'entrée
    // "Rafale" est retirée : le réservoir devient un onglet de la page Quiz
    // (App.jsx adminRoutes, /admin/rafale conservée en redirection).
    { path: 'backstage', label: 'Backstage', icon: '🎭' },
  ]

  // Groupe Interface (#238) : affichage TV, joueurs et animateur (nouvel onglet)
  const tvItems = [
    { path: '/tv', label: 'TV', icon: '📺', absolute: true },
    { path: '/player', label: 'Joueur', icon: '📱', absolute: true },
    { path: '/anim', label: 'Animateur', icon: '🎤', absolute: true },
  ]

  // Menu items dans le menu déroulant
  const menuItems = [
    { path: 'settings', label: 'Réglages', icon: '⚙️' },
    // #207 — juste après Réglages. L'icône est un ÉLÉMENT React (SVG en ligne,
    // 3 glyphes distincts selon l'état), pas un emoji : ni la couleur ni la
    // forme d'un emoji ne sont pilotables. `title` dit l'état en toutes
    // lettres pour les lecteurs d'écran (le SVG est aria-hidden).
    //
    // #234 — la page /admin/ambiance a deux onglets (Lumière, Son) depuis
    // #230 : cette entrée de menu porte donc désormais DEUX glyphes côte à
    // côte, l'ampoule Hue (inchangée) et le haut-parleur (nouveau). `icon`
    // accepte du JSX arbitraire (voir les trois sites qui rendent
    // `{item.icon}` dans ce fichier) : un fragment se propage sans aucune
    // autre modification, quel que soit celui qui affiche cette entrée.
    // Aucun popover pour le son (contrairement à l'ampoule) : c'est une
    // pastille d'état, pas une commande — cliquer l'entrée mène à la page,
    // comme aujourd'hui.
    {
      path: 'ambiance',
      label: 'Ambiance',
      icon: (
        <span className="ambiance-menu-icons">
          <LightingBulbIcon state={lightingStatus.state} />
          <SoundSpeakerIcon active={soundStatus.active} />
        </span>
      ),
      title: `${lightingStateTitle(lightingStatus.state)} · ${soundStateTitle(soundStatus.active)}`,
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
        title={item.label}
        aria-label={item.label}
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
            <BrandLogo />
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
          title={!canEntracteToggle ? "Désactivé pendant une question en cours" : (entracteActive ? "FIN D'ENTRACTE" : 'ENTRACTE')}
          aria-label={entracteActive ? "FIN D'ENTRACTE" : 'ENTRACTE'}
        >
          <span className="entracte-icon" aria-hidden="true">{entracteActive ? '🎬' : '🍿'}</span>
          <span className="entracte-label">{entracteActive ? "FIN D'ENTRACTE" : 'ENTRACTE'}</span>
        </Button>

        {/* #208 — point d'accès complet aux commandes ON/AUTO/OFF/Flash,
            visible dès que l'éclairage est configuré (pas seulement un mode
            engagé) : c'est un CONTRÔLE, plus seulement un indicateur.
            Absent tant que l'éclairage n'est pas configuré — même ligne de
            conduite que l'entrée Ambiance du menu (aucune trace si aucun
            pont Hue). */}
        {lightingConfigured && (
          <div className="lighting-mode-container">
            <button
              ref={lightingButtonRef}
              type="button"
              className={`lighting-mode-nav-badge is-${lightingMode.toLowerCase()}`}
              title="Éclairage général — ouvrir les commandes"
              aria-haspopup="true"
              aria-expanded={lightingPopoverOpen}
              onClick={() => setLightingPopoverOpen(o => !o)}
              aria-label={lightingMode === 'AUTO' ? 'Éclairage' : `Mode ${lightingMode} engagé`}
            >
              <span aria-hidden="true">💡</span>{' '}
              <span className="lighting-label">{lightingMode === 'AUTO' ? 'Éclairage' : `Mode ${lightingMode} engagé`}</span>
            </button>
            {lightingPopoverOpen && (
              <div ref={lightingPopoverRef} className="lighting-mode-popover">
                <LightingModePanel />
              </div>
            )}
          </div>
        )}
      </div>

      <div className="navbar-links">
        <div className="nav-group nav-group-game">
          <span className="nav-group-label">Jeu</span>
          <div className="nav-group-items">
            {gameItems.map(renderNavLink)}
          </div>
        </div>
        <NavGroupMenu
          mode={inlineGroups ? 'inline' : 'single'}
          prepItems={configItems}
          interfaceItems={tvItems}
          renderNavLink={renderNavLink}
          getFullPath={getFullPath}
          isActiveRoute={isActiveRoute}
          pathname={location.pathname}
          openMenu={openMenu}
          setOpenMenu={setOpenMenu}
        />
      </div>

      <div className="navbar-status">
        {compactCounts ? (
          <div className="counts-badge-wrapper" data-navmenu="counts"
            onMouseEnter={() => setOpenMenu('counts')}
            onMouseLeave={() => setOpenMenu(cur => (cur === 'counts' ? null : cur))}>
            <button
              type="button"
              className={`counts-badge severity-${countsBadge.severity}`}
              title="Compteurs de connexions"
              aria-label={`Compteurs de connexions : ${countsBadge.connected}/${countsBadge.participants}`}
              aria-haspopup="true"
              aria-expanded={openMenu === 'counts'}
              onClick={() => setOpenMenu(cur => (cur === 'counts' ? null : 'counts'))}
            >
              <span aria-hidden="true">👥</span> {countsBadge.connected}/{countsBadge.participants}
            </button>
            {openMenu === 'counts' && (
              <div className="navbar-menu-dropdown counts-dropdown">
                {counterItems.map(c => (
                  <span key={c.key} className={`client-count ${c.cls}`} title={c.title}>
                    <span className="count-icon">{c.icon}</span>
                    <span className="count-value">{c.value}</span>
                    <span className="count-name">{c.name}</span>
                  </span>
                ))}
              </div>
            )}
          </div>
        ) : (
          <div className="client-counts">
            {counterItems.map(c => (
              <span key={c.key} className={`client-count ${c.cls}`} title={c.title}>
                <span className="count-icon">{c.icon}</span>
                <span className="count-value">{c.value}</span>
              </span>
            ))}
          </div>
        )}
        <div
          className={`connection-status ${connectionStatus}`}
          title={connectionStatus === 'connected' ? 'Connecté' : connectionStatus === 'connecting' ? 'Connexion...' : 'Déconnecté'}
        >
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
