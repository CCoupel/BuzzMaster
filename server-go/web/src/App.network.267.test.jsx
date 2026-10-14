import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import App from './App'

// #267 (v11.2.2, LOT-3) — l'alerte « serveur sans réseau » est montée au niveau App :
// TOUTES les pages /admin/*, JAMAIS /tv, /anim, /player ni l'inscription (/). Même condition que la Navbar.
// Les pages sont des repères : on teste le montage, pas leur contenu.

vi.mock('./App.css', () => ({}))
vi.mock('./components/NetworkLostOverlay.css', () => ({}))

const network = vi.hoisted(() => ({
  value: { ONLY_LOCALHOST: true, IPS: [], IGNORED_INTERFACES: [], CHECKED_AT: 0 },
}))

vi.mock('./hooks/GameContext', () => ({
  GameProvider: ({ children }) => <>{children}</>,
  useGame: () => ({
    status: 'connected',
    clientCounts: {},
    version: '0.0.0-test',
    bumpers: {},
    gameState: { NETWORK: network.value },
  }),
}))

vi.mock('./components/Navbar', () => ({ default: () => <nav data-testid="navbar" /> }))
vi.mock('./components/LaunchGameHost', () => ({
  default: ({ children }) => <>{children}</>,
  LaunchLockBanner: () => null,
}))
vi.mock('./components/RegieMessageBar', () => ({ default: () => null }))

vi.mock('./pages/GamePage', () => ({ default: () => <div data-testid="page-game" /> }))
vi.mock('./pages/ScoresPage', () => ({ default: () => <div data-testid="page-scores" /> }))
vi.mock('./pages/TeamsPage', () => ({ default: () => <div data-testid="page-teams" /> }))
vi.mock('./pages/QuestionsPage', () => ({ default: () => <div data-testid="page-quiz" /> }))
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

const renderAt = (path) =>
  render(
    <MemoryRouter initialEntries={[path]}>
      <App />
    </MemoryRouter>
  )

const ADMIN_PATHS = [
  '/admin', '/admin/scoreboard', '/admin/teams', '/admin/quiz', '/admin/backstage', '/admin/history',
  '/admin/palmares', '/admin/settings', '/admin/ambiance', '/admin/backup', '/admin/updates', '/admin/logs',
]
const NON_ADMIN_PATHS = ['/', '/player', '/tv', '/anim']

beforeEach(() => {
  network.value = { ONLY_LOCALHOST: true, IPS: [], IGNORED_INTERFACES: [], CHECKED_AT: 0 }
})

describe('App — alerte réseau #267 : présence sur toutes les pages admin', () => {
  it.each(ADMIN_PATHS)('%s affiche l\'overlay « Le serveur n\'a pas de réseau »', (path) => {
    renderAt(path)
    expect(screen.getByRole('alertdialog', { name: "Le serveur n'a pas de réseau disponible" })).toBeInTheDocument()
  })

  it('/admin/rafale (redirection vers le quiz) affiche aussi l\'overlay', () => {
    renderAt('/admin/rafale')
    expect(screen.getByTestId('page-quiz')).toBeInTheDocument()
    expect(screen.getByRole('alertdialog')).toBeInTheDocument()
  })

  it.each(['/admin', '/admin/quiz', '/admin/settings'])('%s : « Continuer quand même » laisse un bandeau placé après la Navbar, avant le contenu', (path) => {
    renderAt(path)
    fireEvent.click(screen.getByRole('button', { name: 'Continuer quand même' }))
    expect(screen.queryByRole('alertdialog')).toBeNull()
    const banner = document.querySelector('.network-warning-banner')
    expect(banner).not.toBeNull()
    const navbar = screen.getByTestId('navbar')
    const main = document.querySelector('main')
    expect(navbar.compareDocumentPosition(banner) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(banner.compareDocumentPosition(main) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })
})

describe('App — alerte réseau #267 : absence hors admin', () => {
  it.each(NON_ADMIN_PATHS)('%s ne montre ni overlay ni bandeau même sans réseau', (path) => {
    renderAt(path)
    expect(screen.queryByRole('alertdialog')).toBeNull()
    expect(document.querySelector('.network-warning-banner')).toBeNull()
    expect(document.querySelector('.network-overlay')).toBeNull()
  })

  it('une page inconnue (aucune route) ne montre rien non plus', () => {
    renderAt('/nimporte-quoi')
    expect(screen.queryByRole('alertdialog')).toBeNull()
  })
})

describe('App — alerte réseau #267 : réseau présent', () => {
  it.each(ADMIN_PATHS)('%s : rien n\'est affiché quand le serveur a du réseau', (path) => {
    network.value = { ONLY_LOCALHOST: false, IPS: ['192.168.1.84'], IGNORED_INTERFACES: [], CHECKED_AT: 1 }
    renderAt(path)
    expect(screen.queryByRole('alertdialog')).toBeNull()
    expect(document.querySelector('.network-warning-banner')).toBeNull()
  })
})
