import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import { MemoryRouter, Link, useLocation, useNavigate, useNavigationType } from 'react-router-dom'
import App from './App'

// ---------------------------------------------------------------------------
// #268 — montée de react-router(-dom) 6.30.6 (avis GHSA : open redirect / XSS).
//
// Test de NON-RÉGRESSION du routage de App.jsx : après le bump, CHAQUE route
// doit rendre le même composant qu'avant, les redirections doivent rester des
// `Navigate replace`, et l'endpoint WebSocket choisi par App doit rester celui
// du contrat (contracts/websocket-endpoints.md) :
//
//   /admin*  → /ws/admin      /anim → /ws/anim      /tv → /ws/tv
//   tout le reste (/, /player, inconnu) → /ws/player
//
// Les pages et composants lourds sont remplacés par des repères (data-testid) :
// on teste le ROUTEUR, pas les pages (couvertes par leurs propres fichiers).
// Aucune route 404 n'existe dans App.jsx : une URL inconnue ne rend AUCUNE page
// (comportement actuel, figé ici pour détecter un changement involontaire).
// ---------------------------------------------------------------------------

vi.mock('./App.css', () => ({}))

// GameProvider : trace l'endpoint reçu, sans ouvrir de WebSocket.
vi.mock('./hooks/GameContext', () => ({
  GameProvider: ({ children, endpoint }) => (
    <div data-testid="provider" data-endpoint={endpoint}>{children}</div>
  ),
  useGame: () => ({
    status: 'connected',
    clientCounts: {},
    version: '0.0.0-test',
    bumpers: {},
  }),
}))

// Navbar factice : de vrais <Link> + useNavigate pour tester la navigation
// (clic, retour arrière) avec le routeur réel.
vi.mock('./components/Navbar', () => ({
  default: function NavbarStub() {
    const navigate = useNavigate()
    return (
      <nav data-testid="navbar">
        <Link to="/admin/teams" data-testid="link-teams">Équipes</Link>
        <Link to="/admin/quiz" data-testid="link-quiz">Quiz</Link>
        <Link to="/admin" data-testid="link-game">Jeu</Link>
        <button data-testid="nav-back" onClick={() => navigate(-1)}>retour</button>
      </nav>
    )
  },
}))

vi.mock('./components/LaunchGameHost', () => ({
  default: ({ children }) => <div data-testid="launch-host">{children}</div>,
  LaunchLockBanner: () => <div data-testid="lock-banner" />,
}))
vi.mock('./components/RegieMessageBar', () => ({
  default: () => <div data-testid="regie-bar" />,
}))

// Pages — un repère par page.
vi.mock('./pages/GamePage', () => ({ default: () => <div data-testid="page-game" /> }))
vi.mock('./pages/ScoresPage', () => ({ default: () => <div data-testid="page-scores" /> }))
vi.mock('./pages/TeamsPage', () => ({ default: () => <div data-testid="page-teams" /> }))
vi.mock('./pages/BackstagePage', () => ({ default: () => <div data-testid="page-backstage" /> }))
vi.mock('./pages/HistoryPage', () => ({ default: () => <div data-testid="page-history" /> }))
vi.mock('./pages/CategoryPalmaresPage', () => ({ default: () => <div data-testid="page-palmares" /> }))
vi.mock('./pages/LogsPage', () => ({ default: () => <div data-testid="page-logs" /> }))
vi.mock('./pages/ConfigPage', () => ({ default: () => <div data-testid="page-settings" /> }))
vi.mock('./pages/BackupPage', () => ({ default: () => <div data-testid="page-backup" /> }))
vi.mock('./pages/AmbiancePage', () => ({ default: () => <div data-testid="page-ambiance" /> }))
vi.mock('./pages/UpdatePage', () => ({ default: () => <div data-testid="page-updates" /> }))
vi.mock('./pages/PlayerDisplay', () => ({ default: () => <div data-testid="page-tv" /> }))
vi.mock('./pages/EnrollPage', () => ({ default: () => <div data-testid="page-enroll" /> }))
vi.mock('./pages/VPlayerPage', () => ({ default: () => <div data-testid="page-vplayer" /> }))
vi.mock('./pages/AnimPage', () => ({ default: () => <div data-testid="page-anim" /> }))
// QuestionsPage : expose la cible de la redirection /admin/rafale (query + type de navigation).
vi.mock('./pages/QuestionsPage', () => ({
  default: function QuestionsPageStub() {
    const location = useLocation()
    const navType = useNavigationType()
    return (
      <div
        data-testid="page-quiz"
        data-search={location.search}
        data-pathname={location.pathname}
        data-nav-type={navType}
      />
    )
  },
}))

const ALL_PAGE_IDS = [
  'page-game', 'page-scores', 'page-teams', 'page-quiz', 'page-backstage',
  'page-history', 'page-palmares', 'page-settings', 'page-ambiance',
  'page-backup', 'page-updates', 'page-logs', 'page-tv', 'page-enroll',
  'page-vplayer', 'page-anim',
]

function renderAt(initialEntries, initialIndex) {
  return render(
    <MemoryRouter initialEntries={Array.isArray(initialEntries) ? initialEntries : [initialEntries]} initialIndex={initialIndex}>
      <App />
    </MemoryRouter>
  )
}

function renderedPageIds() {
  return ALL_PAGE_IDS.filter((id) => screen.queryByTestId(id))
}

describe('#268 — App.jsx : chaque route rend le bon composant', () => {
  afterEach(cleanup)

  const ROUTES = [
    ['/', 'page-enroll'],
    ['/player', 'page-vplayer'],
    ['/tv', 'page-tv'],
    ['/anim', 'page-anim'],
    ['/admin', 'page-game'],
    ['/admin/scoreboard', 'page-scores'],
    ['/admin/teams', 'page-teams'],
    ['/admin/quiz', 'page-quiz'],
    ['/admin/backstage', 'page-backstage'],
    ['/admin/history', 'page-history'],
    ['/admin/palmares', 'page-palmares'],
    ['/admin/settings', 'page-settings'],
    ['/admin/ambiance', 'page-ambiance'],
    ['/admin/backup', 'page-backup'],
    ['/admin/updates', 'page-updates'],
    ['/admin/logs', 'page-logs'],
  ]

  it.each(ROUTES)('%s rend %s, et rien d\'autre', (path, expectedId) => {
    renderAt(path)
    expect(renderedPageIds()).toEqual([expectedId])
  })

  it('/admin/ (slash final) rend la page de jeu comme /admin', () => {
    renderAt('/admin/')
    expect(renderedPageIds()).toEqual(['page-game'])
  })

  it('une query string ne change pas la route (/admin/quiz?tab=rafale)', () => {
    renderAt('/admin/quiz?tab=rafale')
    expect(renderedPageIds()).toEqual(['page-quiz'])
    expect(screen.getByTestId('page-quiz').dataset.search).toBe('?tab=rafale')
  })

  it('/admin/logs est servi par LogsPage (et non par une route générique /admin/:x)', () => {
    renderAt('/admin/logs')
    expect(screen.getByTestId('page-logs')).toBeInTheDocument()
  })
})

describe('#268 — redirection /admin/rafale (Navigate replace)', () => {
  afterEach(cleanup)

  it('/admin/rafale redirige vers /admin/quiz?tab=rafale', () => {
    renderAt('/admin/rafale')
    const quiz = screen.getByTestId('page-quiz')
    expect(quiz.dataset.pathname).toBe('/admin/quiz')
    expect(quiz.dataset.search).toBe('?tab=rafale')
    expect(renderedPageIds()).toEqual(['page-quiz'])
  })

  it('la redirection REMPLACE l\'entrée d\'historique (type de navigation REPLACE)', () => {
    renderAt('/admin/rafale')
    expect(screen.getByTestId('page-quiz').dataset.navType).toBe('REPLACE')
  })

  it('le retour arrière après la redirection ne ramène pas sur /admin/rafale (pas de boucle)', () => {
    // historique : /admin/teams → /admin/rafale (→ redirigée, remplacée par /admin/quiz?tab=rafale)
    renderAt(['/admin/teams', '/admin/rafale'], 1)
    expect(screen.getByTestId('page-quiz')).toBeInTheDocument()
    fireEvent.click(screen.getByTestId('nav-back'))
    expect(renderedPageIds()).toEqual(['page-teams'])
  })

  it('/admin/rafale reste une route admin : Navbar et bandeau régie présents', () => {
    renderAt('/admin/rafale')
    expect(screen.getByTestId('navbar')).toBeInTheDocument()
    expect(screen.getByTestId('regie-bar')).toBeInTheDocument()
  })
})

describe('#268 — URL inconnues : aucune route 404 dans App.jsx (comportement figé)', () => {
  let warnSpy
  beforeEach(() => { warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {}) })
  afterEach(() => { warnSpy.mockRestore(); cleanup() })

  it.each(['/inconnu', '/admin/inconnue', '/tv/extra', '/player/x', '/enroll'])(
    '%s ne rend aucune page et ne lève pas d\'erreur',
    (path) => {
      expect(() => renderAt(path)).not.toThrow()
      expect(renderedPageIds()).toEqual([])
    }
  )

  it('la coque (main) reste rendue sur une URL inconnue', () => {
    const { container } = renderAt('/inconnu')
    expect(container.querySelector('main.main-content')).not.toBeNull()
  })
})

describe('#268 — endpoint WebSocket choisi par App selon la route', () => {
  afterEach(cleanup)

  const ENDPOINTS = [
    ['/', '/ws/player'],
    ['/player', '/ws/player'],
    ['/tv', '/ws/tv'],
    ['/anim', '/ws/anim'],
    ['/admin', '/ws/admin'],
    ['/admin/teams', '/ws/admin'],
    ['/admin/logs', '/ws/admin'],
    ['/admin/rafale', '/ws/admin'],
    ['/inconnu', '/ws/player'],
  ]

  it.each(ENDPOINTS)('%s → %s', (path, endpoint) => {
    renderAt(path)
    expect(screen.getByTestId('provider').dataset.endpoint).toBe(endpoint)
  })
})

describe('#268 — coque : Navbar / bandeau régie / hôte de lancement uniquement sur /admin', () => {
  afterEach(cleanup)

  it.each(['/admin', '/admin/teams', '/admin/settings', '/admin/rafale'])(
    '%s : Navbar, bandeau de verrou, bandeau régie et hôte de lancement présents',
    (path) => {
      renderAt(path)
      expect(screen.getByTestId('navbar')).toBeInTheDocument()
      expect(screen.getByTestId('lock-banner')).toBeInTheDocument()
      expect(screen.getByTestId('regie-bar')).toBeInTheDocument()
      expect(screen.getByTestId('launch-host')).toBeInTheDocument()
    }
  )

  it.each(['/', '/player', '/tv', '/anim'])(
    '%s : aucun élément de régie, main en plein écran',
    (path) => {
      const { container } = renderAt(path)
      expect(screen.queryByTestId('navbar')).toBeNull()
      expect(screen.queryByTestId('lock-banner')).toBeNull()
      expect(screen.queryByTestId('regie-bar')).toBeNull()
      expect(screen.queryByTestId('launch-host')).toBeNull()
      expect(container.querySelector('main.main-content.fullscreen')).not.toBeNull()
    }
  )

  it('sur /admin, main n\'est pas en plein écran', () => {
    const { container } = renderAt('/admin')
    expect(container.querySelector('main.main-content')).not.toBeNull()
    expect(container.querySelector('main.main-content.fullscreen')).toBeNull()
  })
})

describe('#268 — navigation client (Link, retour arrière) avec le routeur monté', () => {
  afterEach(cleanup)

  it('un clic sur un lien de la Navbar change de page sans recharger, endpoint conservé', () => {
    renderAt('/admin')
    expect(renderedPageIds()).toEqual(['page-game'])

    fireEvent.click(screen.getByTestId('link-teams'))
    expect(renderedPageIds()).toEqual(['page-teams'])
    expect(screen.getByTestId('provider').dataset.endpoint).toBe('/ws/admin')

    fireEvent.click(screen.getByTestId('link-quiz'))
    expect(renderedPageIds()).toEqual(['page-quiz'])
  })

  it('le retour arrière revient à la page précédente, dans l\'ordre inverse', () => {
    renderAt('/admin')
    fireEvent.click(screen.getByTestId('link-teams'))
    fireEvent.click(screen.getByTestId('link-quiz'))
    expect(renderedPageIds()).toEqual(['page-quiz'])

    fireEvent.click(screen.getByTestId('nav-back'))
    expect(renderedPageIds()).toEqual(['page-teams'])
    fireEvent.click(screen.getByTestId('nav-back'))
    expect(renderedPageIds()).toEqual(['page-game'])
  })

  it('/admin/quiz → retour sur /admin est possible depuis une route profonde ouverte directement', () => {
    // Rafraîchissement sur route profonde = entrée unique à cet URL ; la route doit se résoudre seule.
    renderAt('/admin/palmares')
    expect(renderedPageIds()).toEqual(['page-palmares'])
    fireEvent.click(screen.getByTestId('link-game'))
    expect(renderedPageIds()).toEqual(['page-game'])
  })
})
