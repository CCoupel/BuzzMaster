import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, within, act, cleanup } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Link } from 'react-router-dom'
import App from '../App'

// ---------------------------------------------------------------------------
// #267 (v11.2.2) — alerte admin « le serveur n'a pas de réseau ».
// Contrat : contracts/game-state.md §NETWORK. Maquette validée :
// docs/mockup/v11.2.2/ui/admin_network_overlay__no_network.html (3 états).
//
//   NETWORK.ONLY_LOCALHOST = true  → overlay plein écran, role="alertdialog"
//   « Continuer quand même » / Échap → bandeau rouge persistant « Perte détectée il y a N s » + « Détails »
//   toute NOUVELLE transition false → true rouvre l'overlay
//   true → false → overlay et bandeau retirés, toast « Réseau rétabli — <IP> » 6 s
//   uniquement sur /admin/* ; jamais sur /, /player, /tv, /anim ; aucun conseil par OS (Q3)
//
// Montage VÉRIFIÉ À TRAVERS App.jsx (vrai routeur, vraie Navbar-coque remplacée par un repère) :
// pages et GameProvider remplacés ; NetworkLostOverlay / NetworkWarningBanner sont les VRAIS composants.
// Sélecteurs : rôles ARIA et texte visible uniquement (aucune classe CSS).
// ---------------------------------------------------------------------------

const store = vi.hoisted(() => ({ current: null }))

vi.mock('../App.css', () => ({}))

vi.mock('../hooks/GameContext', () => ({
  GameProvider: ({ children }) => <>{children}</>,
  useGame: () => store.current,
  useOptionalGame: () => store.current,
}))

// Navbar factice : un lien et un bouton FOCALISABLES hors de l'overlay (test du piège de focus)
// + de vrais <Link> pour naviguer entre pages admin.
vi.mock('../components/Navbar', () => ({
  default: () => (
    <nav aria-label="navigation test">
      <Link to="/admin/teams">Équipes</Link>
      <Link to="/admin/quiz">Questions</Link>
      <Link to="/tv">Télévision</Link>
      <button type="button">Action de la barre</button>
    </nav>
  ),
}))
vi.mock('../components/LaunchGameHost', () => ({
  default: ({ children }) => <>{children}</>,
  LaunchLockBanner: () => null,
}))
vi.mock('../components/RegieMessageBar', () => ({ default: () => null }))

// Pages : un repère par page (vi.mock est hissé avant les imports, ce que vi.doMock ne ferait pas).
vi.mock('../pages/GamePage', () => ({ default: () => <div data-testid="page-GamePage" /> }))
vi.mock('../pages/ScoresPage', () => ({ default: () => <div data-testid="page-ScoresPage" /> }))
vi.mock('../pages/TeamsPage', () => ({ default: () => <div data-testid="page-TeamsPage" /> }))
vi.mock('../pages/QuestionsPage', () => ({ default: () => <div data-testid="page-QuestionsPage" /> }))
vi.mock('../pages/BackstagePage', () => ({ default: () => <div data-testid="page-BackstagePage" /> }))
vi.mock('../pages/HistoryPage', () => ({ default: () => <div data-testid="page-HistoryPage" /> }))
vi.mock('../pages/CategoryPalmaresPage', () => ({ default: () => <div data-testid="page-CategoryPalmaresPage" /> }))
vi.mock('../pages/LogsPage', () => ({ default: () => <div data-testid="page-LogsPage" /> }))
vi.mock('../pages/ConfigPage', () => ({ default: () => <div data-testid="page-ConfigPage" /> }))
vi.mock('../pages/BackupPage', () => ({ default: () => <div data-testid="page-BackupPage" /> }))
vi.mock('../pages/AmbiancePage', () => ({ default: () => <div data-testid="page-AmbiancePage" /> }))
vi.mock('../pages/UpdatePage', () => ({ default: () => <div data-testid="page-UpdatePage" /> }))
vi.mock('../pages/PlayerDisplay', () => ({ default: () => <div data-testid="page-PlayerDisplay" /> }))
vi.mock('../pages/EnrollPage', () => ({ default: () => <div data-testid="page-EnrollPage" /> }))
vi.mock('../pages/VPlayerPage', () => ({ default: () => <div data-testid="page-VPlayerPage" /> }))
vi.mock('../pages/AnimPage', () => ({ default: () => <div data-testid="page-AnimPage" /> }))

const NOW = new Date('2026-10-14T10:00:00Z').getTime()

const LOST = {
  ONLY_LOCALHOST: true,
  IPS: [],
  IGNORED_INTERFACES: [{ NAME: 'vEthernet (WSL)', IP: '172.21.0.1' }],
  CHECKED_AT: NOW - 12000,
}
const OK = {
  ONLY_LOCALHOST: false,
  IPS: ['192.168.1.84'],
  IGNORED_INTERFACES: [],
  CHECKED_AT: NOW,
}

function setNetwork(network) {
  store.current = {
    status: 'connected',
    clientCounts: {},
    version: '0.0.0-test',
    bumpers: {},
    teams: {},
    gameState: { phase: 'STOPPED', NETWORK: network, NETWORK_ONLY_LOCALHOST: network.ONLY_LOCALHOST },
  }
}

function tree(path) {
  return (
    <MemoryRouter initialEntries={[path]}>
      <App />
    </MemoryRouter>
  )
}

const dialog = () => screen.queryByRole('alertdialog')
const banner = () => screen.queryByText(/Serveur sans réseau/)
const restoredToast = () => screen.queryByText(/Réseau rétabli/)

// react-router 7 : navigate() est enveloppé dans startTransition → le rendu de la nouvelle route
// passe par le scheduler (timers factices ici) ; on le vide avant de lire le DOM.
const flushTransition = () => act(async () => { await vi.advanceTimersByTimeAsync(50) })

describe('#267 — overlay « serveur sans réseau »', () => {
  let user

  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(NOW)
    user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    setNetwork(LOST)
  })
  afterEach(() => {
    cleanup()
    vi.useRealTimers()
  })

  // ── Présence / absence selon la route ─────────────────────────────────────────────────────────

  describe('montage par App.jsx : toutes les pages /admin/*, jamais ailleurs', () => {
    it.each([
      '/admin', '/admin/scoreboard', '/admin/teams', '/admin/quiz', '/admin/backstage', '/admin/history',
      '/admin/palmares', '/admin/settings', '/admin/ambiance', '/admin/backup', '/admin/updates', '/admin/logs',
    ])('Critical — %s : overlay présent quand NETWORK.ONLY_LOCALHOST = true', (path) => {
      render(tree(path))
      expect(dialog()).toBeInTheDocument()
    })

    it('Critical — la redirection /admin/rafale mène aussi à une page admin avec overlay', () => {
      render(tree('/admin/rafale'))
      expect(dialog()).toBeInTheDocument()
    })

    it.each(['/', '/player', '/tv', '/anim'])(
      'Critical — %s : aucun overlay, aucun bandeau, aucun toast, même réseau perdu',
      (path) => {
        render(tree(path))
        expect(dialog()).toBeNull()
        expect(banner()).toBeNull()
        expect(screen.queryByText(/n.a pas de réseau/)).toBeNull()
        expect(restoredToast()).toBeNull()
      }
    )

    it('Critical — réseau présent : rien sur /admin', () => {
      setNetwork(OK)
      render(tree('/admin'))
      expect(dialog()).toBeNull()
      expect(banner()).toBeNull()
      expect(restoredToast()).toBeNull()
    })

    it('un admin qui se connecte PENDANT la panne voit l\'overlay dès le premier rendu (HELLO), sans toast', () => {
      render(tree('/admin'))
      expect(dialog()).toBeInTheDocument()
      expect(restoredToast()).toBeNull()
    })

    it('quitter /admin vers /tv retire l\'alerte ; elle ne fuit pas sur l\'affichage TV', async () => {
      render(tree('/admin'))
      expect(dialog()).toBeInTheDocument()
      await user.keyboard('{Escape}') // replie pour atteindre la barre
      expect(banner()).toBeInTheDocument()
      await user.click(screen.getByRole('link', { name: 'Télévision' }))
      await flushTransition()
      expect(screen.queryByRole('navigation', { name: 'navigation test' })).toBeNull() // plus de coque admin
      expect(banner()).toBeNull()
      expect(dialog()).toBeNull()
    })
  })

  // ── État 1 : overlay — contenu, accessibilité, libellés de la maquette ───────────────────────

  describe('état 1 — overlay plein écran', () => {
    it('Critical — role="alertdialog", modal, nommé « Le serveur n\'a pas de réseau », décrit par le texte d\'explication', () => {
      render(tree('/admin'))
      const d = screen.getByRole('alertdialog', { name: /Le serveur n.a pas de réseau/ })
      expect(d).toHaveAttribute('aria-modal', 'true')
      expect(d).toHaveAccessibleDescription(/joignable que depuis ce poste/)
    })

    it('est rendu dans un portail sur <body>, hors de la coque de l\'application', () => {
      const { container } = render(tree('/admin'))
      expect(container.contains(dialog())).toBe(false)
      expect(document.body.contains(dialog())).toBe(true)
    })

    it('libellés de la maquette : explication, adresses, interface ignorée, perte détectée', () => {
      render(tree('/admin'))
      const d = dialog()
      expect(within(d).getByText(/BuzzControl n.est joignable que depuis ce poste/)).toBeInTheDocument()
      expect(within(d).getByText(/aucun buzzer, aucune tablette, ni la TV ni l.animateur/)).toBeInTheDocument()
      expect(within(d).getByText('Adresses détectées')).toBeInTheDocument()
      expect(within(d).getByText('aucune')).toBeInTheDocument()
      expect(within(d).getByText('127.0.0.1')).toBeInTheDocument()
      expect(within(d).getByText('Interfaces ignorées')).toBeInTheDocument()
      expect(within(d).getByText('vEthernet (WSL)')).toBeInTheDocument()
      expect(within(d).getByText(/172\.21\.0\.1/)).toBeInTheDocument()
      expect(within(d).getByText(/virtuelle — non joignable par les appareils/)).toBeInTheDocument()
      expect(within(d).getByText('Perte détectée')).toBeInTheDocument()
      expect(within(d).getByText(/il y a 12 s/)).toBeInTheDocument()
      expect(within(d).getByText(/contrôle automatique toutes les 30 s/)).toBeInTheDocument()
    })

    it('conseils génériques de la maquette ; l\'écran annonce sa disparition automatique', () => {
      render(tree('/admin'))
      const d = dialog()
      expect(within(d).getByText(/Branchez le câble réseau ou reconnectez le WiFi de ce poste/)).toBeInTheDocument()
      expect(within(d).getByText(/Cet écran disparaîtra de lui-même dès qu.une adresse réseau sera détectée/)).toBeInTheDocument()
      expect(within(d).getByText(/l.alerte restera visible en bandeau rouge jusqu.au retour du réseau/)).toBeInTheDocument()
    })

    it('Q3 — aucun conseil par système d\'exploitation (texte unique)', () => {
      render(tree('/admin'))
      const text = dialog().textContent
      expect(text).not.toMatch(/Windows|Linux|macOS|netsh|ipconfig|ifconfig|nmcli|sudo/i)
    })

    it('sans interface ignorée, l\'overlay reste complet (pas d\'erreur, bouton présent)', () => {
      setNetwork({ ...LOST, IGNORED_INTERFACES: [] })
      render(tree('/admin'))
      expect(dialog()).toBeInTheDocument()
      expect(within(dialog()).getByRole('button', { name: 'Continuer quand même' })).toBeInTheDocument()
    })

    it('le seul bouton d\'action est « Continuer quand même »', () => {
      render(tree('/admin'))
      expect(within(dialog()).getAllByRole('button').map((b) => b.textContent.trim())).toEqual(['Continuer quand même'])
    })
  })

  // ── Accessibilité : focus ─────────────────────────────────────────────────────────────────────

  describe('focus piégé et clavier', () => {
    it('à l\'ouverture, le focus entre dans le dialogue', () => {
      render(tree('/admin'))
      expect(dialog().contains(document.activeElement)).toBe(true)
    })

    it.each([
      ['Tab', '{Tab}'],
      ['Maj+Tab', '{Shift>}{Tab}{/Shift}'],
    ])('%s ne fait jamais sortir le focus du dialogue (la barre de navigation reste inatteignable)', async (_name, keys) => {
      render(tree('/admin'))
      for (let i = 0; i < 4; i++) {
        await user.keyboard(keys)
        expect(dialog().contains(document.activeElement)).toBe(true)
      }
      expect(screen.getByRole('button', { name: 'Action de la barre' })).not.toHaveFocus()
    })

    it('Critical — Échap = « Continuer quand même » : overlay fermé, bandeau affiché', async () => {
      render(tree('/admin'))
      await user.keyboard('{Escape}')
      expect(dialog()).toBeNull()
      expect(banner()).toBeInTheDocument()
    })

    it('Entrée sur « Continuer quand même » (focus initial) replie aussi l\'alerte', async () => {
      render(tree('/admin'))
      expect(within(dialog()).getByRole('button', { name: 'Continuer quand même' })).toHaveFocus()
      await user.keyboard('{Enter}')
      expect(dialog()).toBeNull()
      expect(banner()).toBeInTheDocument()
    })

    it('à la fermeture, le focus est rendu à un élément de la page (plus dans le dialogue supprimé)', async () => {
      render(tree('/admin'))
      await user.keyboard('{Escape}')
      expect(document.body.contains(document.activeElement)).toBe(true)
      expect(document.activeElement).not.toBeNull()
    })
  })

  // ── État 2 : bandeau replié ───────────────────────────────────────────────────────────────────

  describe('état 2 — bandeau rouge persistant', () => {
    async function collapse() {
      render(tree('/admin'))
      await user.click(screen.getByRole('button', { name: 'Continuer quand même' }))
    }

    it('Critical — « Continuer quand même » : plus d\'overlay, bandeau « Serveur sans réseau » avec « Perte détectée il y a N s » et « Détails »', async () => {
      await collapse()
      expect(dialog()).toBeNull()
      const status = screen.getByRole('status')
      expect(within(status).getByText(/Serveur sans réseau/)).toBeInTheDocument()
      expect(within(status).getByText(/les buzzers, les tablettes, la TV et l.animateur ne peuvent pas se connecter/)).toBeInTheDocument()
      expect(within(status).getByText('Perte détectée il y a 12 s')).toBeInTheDocument()
      expect(within(status).getByRole('button', { name: 'Détails' })).toBeInTheDocument()
    })

    it('« Perte détectée il y a N s » avance avec l\'horloge, depuis CHECKED_AT', async () => {
      await collapse()
      expect(screen.getByText('Perte détectée il y a 12 s')).toBeInTheDocument()
      act(() => { vi.advanceTimersByTime(5000) })
      expect(screen.getByText('Perte détectée il y a 17 s')).toBeInTheDocument()
      act(() => { vi.advanceTimersByTime(20000) })
      expect(screen.getByText('Perte détectée il y a 37 s')).toBeInTheDocument()
    })

    it('« Perte détectée il y a N s » repart de la nouvelle valeur quand le serveur actualise CHECKED_AT', async () => {
      const { rerender } = render(tree('/admin'))
      await user.click(screen.getByRole('button', { name: 'Continuer quand même' }))
      act(() => { vi.advanceTimersByTime(30000) })
      expect(screen.getByText('Perte détectée il y a 42 s')).toBeInTheDocument()
      setNetwork({ ...LOST, CHECKED_AT: NOW + 30000 - 3000 })
      rerender(tree('/admin'))
      expect(screen.getByText('Perte détectée il y a 3 s')).toBeInTheDocument()
    })

    it('Critical — « Détails » rouvre l\'overlay', async () => {
      await collapse()
      await user.click(screen.getByRole('button', { name: 'Détails' }))
      expect(dialog()).toBeInTheDocument()
    })

    it('après « Détails », Échap replie de nouveau (le cycle peut se répéter)', async () => {
      await collapse()
      await user.click(screen.getByRole('button', { name: 'Détails' }))
      await user.keyboard('{Escape}')
      expect(dialog()).toBeNull()
      expect(banner()).toBeInTheDocument()
    })

    it('le bandeau est PERSISTANT : il suit la navigation entre pages admin, sans rouvrir l\'overlay', async () => {
      await collapse()
      await user.click(screen.getByRole('link', { name: 'Équipes' }))
      await flushTransition()
      expect(screen.getByTestId('page-TeamsPage')).toBeInTheDocument()
      expect(banner()).toBeInTheDocument()
      expect(dialog()).toBeNull()
      await user.click(screen.getByRole('link', { name: 'Questions' }))
      await flushTransition()
      expect(screen.getByTestId('page-QuestionsPage')).toBeInTheDocument()
      expect(banner()).toBeInTheDocument()
      expect(dialog()).toBeNull()
    })

    it('une actualisation du même état perdu (nouveau CHECKED_AT, mêmes champs) ne rouvre pas l\'overlay', async () => {
      const { rerender } = render(tree('/admin'))
      await user.click(screen.getByRole('button', { name: 'Continuer quand même' }))
      setNetwork({ ...LOST, CHECKED_AT: NOW - 1000 })
      rerender(tree('/admin'))
      expect(dialog()).toBeNull()
      expect(banner()).toBeInTheDocument()
    })

    it('un changement d\'interfaces ignorées pendant la panne ne rouvre pas l\'overlay', async () => {
      const { rerender } = render(tree('/admin'))
      await user.click(screen.getByRole('button', { name: 'Continuer quand même' }))
      setNetwork({ ...LOST, IGNORED_INTERFACES: [] })
      rerender(tree('/admin'))
      expect(dialog()).toBeNull()
      expect(banner()).toBeInTheDocument()
    })
  })

  // ── Rouverture sur nouvelle perte ─────────────────────────────────────────────────────────────

  describe('nouvelle perte — l\'overlay revient toujours', () => {
    it('Critical — true → (replié) → false → true : l\'overlay se rouvre, plus de bandeau tant qu\'il est ouvert', async () => {
      const { rerender } = render(tree('/admin'))
      await user.keyboard('{Escape}')
      expect(banner()).toBeInTheDocument()

      setNetwork(OK)
      rerender(tree('/admin'))
      expect(dialog()).toBeNull()
      expect(banner()).toBeNull()

      setNetwork({ ...LOST, CHECKED_AT: NOW })
      rerender(tree('/admin'))
      expect(dialog()).toBeInTheDocument()
    })

    it('trois cycles successifs perte / retour : à chaque perte l\'overlay réapparaît, à chaque retour tout disparaît', async () => {
      const { rerender } = render(tree('/admin'))
      for (let cycle = 0; cycle < 3; cycle++) {
        expect(dialog()).toBeInTheDocument()
        await user.keyboard('{Escape}')
        expect(dialog()).toBeNull()
        expect(banner()).toBeInTheDocument()

        setNetwork(OK)
        rerender(tree('/admin'))
        expect(dialog()).toBeNull()
        expect(banner()).toBeNull()

        setNetwork({ ...LOST, CHECKED_AT: NOW })
        rerender(tree('/admin'))
      }
      expect(dialog()).toBeInTheDocument()
    })

    it('réseau présent au montage puis perdu : l\'overlay apparaît à la transition false → true', () => {
      setNetwork(OK)
      const { rerender } = render(tree('/admin'))
      expect(dialog()).toBeNull()
      setNetwork(LOST)
      rerender(tree('/admin'))
      expect(dialog()).toBeInTheDocument()
    })
  })

  // ── État 3 : retour du réseau ─────────────────────────────────────────────────────────────────

  describe('état 3 — réseau rétabli', () => {
    it('Critical — true → false depuis l\'overlay ouvert : overlay retiré, toast « Réseau rétabli — <IP> »', () => {
      const { rerender } = render(tree('/admin'))
      expect(dialog()).toBeInTheDocument()
      setNetwork(OK)
      rerender(tree('/admin'))
      expect(dialog()).toBeNull()
      expect(banner()).toBeNull()
      const toast = screen.getByRole('status')
      expect(toast).toHaveTextContent('Réseau rétabli — 192.168.1.84')
    })

    it('true → false depuis le bandeau replié : bandeau retiré, toast affiché', async () => {
      const { rerender } = render(tree('/admin'))
      await user.keyboard('{Escape}')
      expect(banner()).toBeInTheDocument()
      setNetwork(OK)
      rerender(tree('/admin'))
      expect(banner()).toBeNull()
      expect(screen.getByText(/Réseau rétabli — 192\.168\.1\.84/)).toBeInTheDocument()
    })

    it('le toast disparaît seul après 6 s (pas avant)', () => {
      const { rerender } = render(tree('/admin'))
      setNetwork(OK)
      rerender(tree('/admin'))
      expect(restoredToast()).toBeInTheDocument()
      act(() => { vi.advanceTimersByTime(5900) })
      expect(restoredToast()).toBeInTheDocument()
      act(() => { vi.advanceTimersByTime(200) })
      expect(restoredToast()).toBeNull()
    })

    it('le toast n\'apparaît jamais au démarrage avec un réseau déjà présent', () => {
      setNetwork(OK)
      render(tree('/admin'))
      expect(restoredToast()).toBeNull()
    })

    it('une nouvelle perte pendant que le toast est affiché le retire et rouvre l\'overlay', () => {
      const { rerender } = render(tree('/admin'))
      setNetwork(OK)
      rerender(tree('/admin'))
      expect(restoredToast()).toBeInTheDocument()
      setNetwork({ ...LOST, CHECKED_AT: NOW })
      rerender(tree('/admin'))
      expect(dialog()).toBeInTheDocument()
      expect(restoredToast()).toBeNull()
    })

    it('un changement d\'adresse sans perte (IPS seul) n\'affiche ni overlay, ni bandeau, ni toast', () => {
      setNetwork(OK)
      const { rerender } = render(tree('/admin'))
      setNetwork({ ...OK, IPS: ['192.168.1.99'], CHECKED_AT: NOW + 30000 })
      rerender(tree('/admin'))
      expect(dialog()).toBeNull()
      expect(banner()).toBeNull()
      expect(restoredToast()).toBeNull()
    })
  })
})
