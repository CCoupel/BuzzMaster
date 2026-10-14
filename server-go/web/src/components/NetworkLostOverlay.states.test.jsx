import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, act, within } from '@testing-library/react'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'
import NetworkLostOverlay from './NetworkLostOverlay'
import { useGame } from '../hooks/GameContext'

// #267 (v11.2.2, LOT-3) — alerte admin « le serveur n'a pas de réseau ».
// Maquette VALIDÉE admin_network_overlay__no_network : 3 états (overlay / bandeau replié / toast de retour).
// Décisions : Q1 interfaces virtuelles ignorées (listées dans l'overlay), Q2 « Continuer quand même » replie
// en bandeau et l'overlay REVIENT à chaque nouvelle perte, Q3 texte unique (aucun conseil par plateforme).
// Contrat : contracts/game-state.md §NETWORK. Le montage sur les seules pages /admin/* est dans
// App.network.267.test.jsx.

vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn() }))
vi.mock('./NetworkLostOverlay.css', () => ({}))

const here = dirname(fileURLToPath(import.meta.url))
const css = (f) => readFileSync(resolve(here, f), 'utf8')

const NOW = 1_800_000_000_000
const WIRED = { ONLY_LOCALHOST: false, IPS: ['192.168.1.84'], IGNORED_INTERFACES: [], CHECKED_AT: NOW }
const LOST = {
  ONLY_LOCALHOST: true,
  IPS: [],
  IGNORED_INTERFACES: [{ NAME: 'vEthernet (WSL)', IP: '172.21.0.1' }],
  CHECKED_AT: NOW - 12_000,
}
const mk = (network) => ({ gameState: { NETWORK: network } })

const DIALOG_NAME = "Le serveur n'a pas de réseau"

function setup(network) {
  useGame.mockReturnValue(mk(network))
  const utils = render(<NetworkLostOverlay />)
  const update = (next) => {
    useGame.mockReturnValue(mk(next))
    utils.rerender(<NetworkLostOverlay />)
  }
  return { ...utils, update }
}

beforeEach(() => {
  vi.useFakeTimers()
  vi.setSystemTime(NOW)
})

afterEach(() => {
  vi.useRealTimers()
  vi.clearAllMocks()
})

describe('NetworkLostOverlay — réseau présent (état 0)', () => {
  it('ne rend ni overlay, ni bandeau, ni toast quand ONLY_LOCALHOST est false', () => {
    setup(WIRED)
    expect(screen.queryByRole('alertdialog')).toBeNull()
    expect(screen.queryByRole('status')).toBeNull()
  })

  it('ne rend rien si NETWORK est absent (état partiel / serveur ancien)', () => {
    useGame.mockReturnValue({ gameState: {} })
    render(<NetworkLostOverlay />)
    expect(screen.queryByRole('alertdialog')).toBeNull()
    expect(screen.queryByRole('status')).toBeNull()
  })

  it('ne rend rien si gameState est indéfini', () => {
    useGame.mockReturnValue({})
    render(<NetworkLostOverlay />)
    expect(screen.queryByRole('alertdialog')).toBeNull()
  })
})

describe('NetworkLostOverlay — perte de réseau (état 1, overlay)', () => {
  it('affiche un alertdialog modal nommé et décrit, en portail sur <body>', () => {
    const { container } = setup(LOST)
    const dialog = screen.getByRole('alertdialog', { name: DIALOG_NAME })
    expect(dialog).toHaveAttribute('aria-modal', 'true')
    expect(dialog).toHaveAccessibleDescription(/n'est joignable que depuis ce poste/)
    // portail : hors du conteneur de rendu, directement sous <body>
    expect(container.contains(dialog)).toBe(false)
    expect(document.body.contains(dialog)).toBe(true)
  })

  it('reprend les textes de la maquette (aucune adresse, conseils en texte unique)', () => {
    setup(LOST)
    const dialog = screen.getByRole('alertdialog')
    expect(within(dialog).getByText(/aucun buzzer, aucune tablette, ni la TV ni l'animateur/)).toBeInTheDocument()
    expect(within(dialog).getByText('Adresses détectées')).toBeInTheDocument()
    expect(within(dialog).getByText('aucune')).toBeInTheDocument()
    expect(within(dialog).getByText('127.0.0.1')).toBeInTheDocument()
    expect(within(dialog).getByText('Branchez le câble réseau ou reconnectez le WiFi de ce poste.')).toBeInTheDocument()
    expect(within(dialog).getByText("Sur un Raspberry Pi, vérifiez que le point d'accès est démarré.")).toBeInTheDocument()
    expect(within(dialog).getByText(/disparaîtra de lui-même/)).toBeInTheDocument()
    expect(within(dialog).getByRole('button', { name: 'Continuer quand même' })).toBeInTheDocument()
  })

  it('liste les interfaces ignorées (nom + adresse) quand il y en a', () => {
    setup(LOST)
    const dialog = screen.getByRole('alertdialog')
    expect(within(dialog).getByText('Interfaces ignorées')).toBeInTheDocument()
    expect(within(dialog).getByText('vEthernet (WSL)')).toBeInTheDocument()
    expect(within(dialog).getByText(/172\.21\.0\.1/)).toBeInTheDocument()
  })

  it("n'affiche pas la ligne « Interfaces ignorées » quand la liste est vide", () => {
    setup({ ...LOST, IGNORED_INTERFACES: [] })
    expect(screen.queryByText('Interfaces ignorées')).toBeNull()
  })

  it('affiche « Dernière vérification : il y a N s » calculé depuis CHECKED_AT', () => {
    setup(LOST)
    expect(within(screen.getByRole('alertdialog')).getByText(/il y a 12 s — contrôle automatique toutes les 30 s/)).toBeInTheDocument()
  })

  it("n'invente pas de durée quand CHECKED_AT vaut 0 (jamais vérifié)", () => {
    setup({ ...LOST, CHECKED_AT: 0 })
    const dialog = screen.getByRole('alertdialog')
    expect(within(dialog).getByText('contrôle automatique toutes les 30 s')).toBeInTheDocument()
    expect(within(dialog).queryByText(/il y a/)).toBeNull()
  })

  it('met à jour « il y a N s » chaque seconde (minuteurs factices)', () => {
    setup(LOST)
    expect(screen.getByText(/il y a 12 s/)).toBeInTheDocument()
    act(() => { vi.advanceTimersByTime(5000) })
    expect(screen.getByText(/il y a 17 s/)).toBeInTheDocument()
    act(() => { vi.advanceTimersByTime(60_000) })
    expect(screen.getByText(/il y a 1 min/)).toBeInTheDocument()
  })

  it('démarre avec le focus sur « Continuer quand même »', () => {
    setup(LOST)
    expect(screen.getByRole('button', { name: 'Continuer quand même' })).toHaveFocus()
  })
})

describe('NetworkLostOverlay — accessibilité (alertdialog)', () => {
  it('Tab et Maj+Tab gardent le focus dans le dialogue (focus piégé)', () => {
    setup(LOST)
    const btn = screen.getByRole('button', { name: 'Continuer quand même' })
    fireEvent.keyDown(btn, { key: 'Tab' })
    expect(btn).toHaveFocus()
    fireEvent.keyDown(btn, { key: 'Tab', shiftKey: true })
    expect(btn).toHaveFocus()
  })

  it('ramène le focus dans le dialogue si il a échappé (élément extérieur)', () => {
    const outside = document.createElement('button')
    document.body.appendChild(outside)
    setup(LOST)
    outside.focus()
    fireEvent.keyDown(outside, { key: 'Tab' })
    expect(screen.getByRole('button', { name: 'Continuer quand même' })).toHaveFocus()
    outside.remove()
  })

  it('Échap = « Continuer quand même » : replie en bandeau', () => {
    setup(LOST)
    fireEvent.keyDown(document.activeElement, { key: 'Escape' })
    expect(screen.queryByRole('alertdialog')).toBeNull()
    expect(screen.getByRole('status')).toHaveTextContent('Serveur sans réseau')
  })

  it("Échap est consommé (ne remonte pas à la popup « Lancer la partie » #253 ni au document)", () => {
    const docSpy = vi.fn()
    document.addEventListener('keydown', docSpy)
    setup(LOST)
    fireEvent.keyDown(document.activeElement, { key: 'Escape' })
    expect(docSpy).not.toHaveBeenCalled()
    document.removeEventListener('keydown', docSpy)
  })

  it("restitue le focus à l'élément précédent à la fermeture", () => {
    const opener = document.createElement('button')
    document.body.appendChild(opener)
    opener.focus()
    setup(LOST)
    expect(screen.getByRole('button', { name: 'Continuer quand même' })).toHaveFocus()
    fireEvent.click(screen.getByRole('button', { name: 'Continuer quand même' }))
    expect(opener).toHaveFocus()
    opener.remove()
  })
})

describe('NetworkLostOverlay — « Continuer quand même » (état 2, bandeau replié)', () => {
  it('replie en bandeau rouge persistant avec « Vérifié il y a N s » et « Détails »', () => {
    setup(LOST)
    fireEvent.click(screen.getByRole('button', { name: 'Continuer quand même' }))
    expect(screen.queryByRole('alertdialog')).toBeNull()
    const banner = screen.getByRole('status')
    expect(banner).toHaveClass('network-warning-banner')
    expect(banner).toHaveTextContent(
      "Serveur sans réseau — les buzzers, les tablettes, la TV et l'animateur ne peuvent pas se connecter."
    )
    expect(within(banner).getByText('Vérifié il y a 12 s')).toBeInTheDocument()
    expect(within(banner).getByRole('button', { name: 'Détails' })).toBeInTheDocument()
  })

  it('le bandeau persiste (aucune disparition automatique) tant que le réseau manque', () => {
    setup(LOST)
    fireEvent.click(screen.getByRole('button', { name: 'Continuer quand même' }))
    act(() => { vi.advanceTimersByTime(10 * 60_000) })
    expect(screen.getByRole('status')).toHaveClass('network-warning-banner')
    expect(screen.queryByRole('alertdialog')).toBeNull()
  })

  it('« Détails » rouvre l\'overlay et retire le bandeau', () => {
    setup(LOST)
    fireEvent.click(screen.getByRole('button', { name: 'Continuer quand même' }))
    fireEvent.click(screen.getByRole('button', { name: 'Détails' }))
    expect(screen.getByRole('alertdialog', { name: DIALOG_NAME })).toBeInTheDocument()
    expect(screen.queryByRole('status')).toBeNull()
  })

  it("une mise à jour qui garde ONLY_LOCALHOST=true ne rouvre PAS l'overlay replié", () => {
    const { update } = setup(LOST)
    fireEvent.click(screen.getByRole('button', { name: 'Continuer quand même' }))
    update({ ...LOST, CHECKED_AT: NOW })
    expect(screen.queryByRole('alertdialog')).toBeNull()
    expect(screen.getByRole('status')).toHaveClass('network-warning-banner')
  })
})

describe('NetworkLostOverlay — transitions (réouverture, retour du réseau)', () => {
  it('une NOUVELLE perte (false → true) rouvre l\'overlay même si il avait été replié', () => {
    const { update } = setup(LOST)
    fireEvent.click(screen.getByRole('button', { name: 'Continuer quand même' }))
    expect(screen.queryByRole('alertdialog')).toBeNull()

    update(WIRED) // retour du réseau
    expect(screen.queryByRole('status', { name: /sans réseau/ })).toBeNull()

    update(LOST) // nouvelle perte
    expect(screen.getByRole('alertdialog', { name: DIALOG_NAME })).toBeInTheDocument()
    expect(screen.queryByText(/Vérifié il y a/)).toBeNull()
  })

  it('une perte survenant en cours de session ouvre l\'overlay (false → true)', () => {
    const { update } = setup(WIRED)
    expect(screen.queryByRole('alertdialog')).toBeNull()
    update(LOST)
    expect(screen.getByRole('alertdialog', { name: DIALOG_NAME })).toBeInTheDocument()
  })

  it('retour du réseau (true → false) depuis l\'overlay : tout disparaît, toast avec l\'IP', () => {
    const { update } = setup(LOST)
    update(WIRED)
    expect(screen.queryByRole('alertdialog')).toBeNull()
    expect(screen.queryByText(/Serveur sans réseau/)).toBeNull()
    const toast = screen.getByRole('status')
    expect(toast).toHaveClass('network-toast')
    expect(toast).toHaveTextContent('Réseau rétabli — 192.168.1.84')
  })

  it('retour du réseau depuis le bandeau replié : bandeau retiré, toast affiché', () => {
    const { update } = setup(LOST)
    fireEvent.click(screen.getByRole('button', { name: 'Continuer quand même' }))
    update(WIRED)
    expect(document.querySelector('.network-warning-banner')).toBeNull()
    expect(screen.getByRole('status')).toHaveTextContent('Réseau rétabli — 192.168.1.84')
  })

  it('le toast de retour disparaît au bout de 6 s', () => {
    const { update } = setup(LOST)
    update(WIRED)
    act(() => { vi.advanceTimersByTime(5900) })
    expect(screen.getByRole('status')).toHaveTextContent('Réseau rétabli')
    act(() => { vi.advanceTimersByTime(200) })
    expect(screen.queryByRole('status')).toBeNull()
  })

  it('retour sans IP connue : toast sans adresse', () => {
    const { update } = setup(LOST)
    update({ ...WIRED, IPS: [] })
    expect(screen.getByRole('status').textContent).toBe('Réseau rétabli')
  })

  it("aucun toast au montage quand le réseau est présent (pas de transition)", () => {
    setup(WIRED)
    expect(screen.queryByRole('status')).toBeNull()
  })

  it("une perte pendant l'affichage du toast de retour le retire et rouvre l'overlay", () => {
    const { update } = setup(LOST)
    update(WIRED)
    expect(screen.getByRole('status')).toHaveClass('network-toast')
    update(LOST)
    expect(document.querySelector('.network-toast')).toBeNull()
    expect(screen.getByRole('alertdialog')).toBeInTheDocument()
  })
})

describe('NetworkLostOverlay — empilement avec la popup « Lancer la partie » (#253, R1)', () => {
  it("l'overlay réseau est au-dessus de la popup (.lg-backdrop) et du toast", () => {
    const z = (src, selector) => {
      const m = src.match(new RegExp(`${selector.replace('.', '\\.')}\\s*\\{[^}]*z-index:\\s*(\\d+)`))
      return m ? Number(m[1]) : NaN
    }
    const net = css('./NetworkLostOverlay.css')
    const zOverlay = z(net, '.network-overlay')
    const zPopup = z(css('./LaunchGameDialog.css'), '.lg-backdrop')
    const zToast = z(net, '.network-toast')
    expect(zOverlay).toBeGreaterThan(zPopup)
    expect(zOverlay).toBeGreaterThan(zToast)
  })

  it("avec une popup ouverte, Échap ferme l'overlay réseau sans fermer la popup", () => {
    const popupEscape = vi.fn()
    useGame.mockReturnValue(mk(LOST))
    render(
      <>
        {/* simulacre de la popup #253 : handler React onKeyDown sur son dialogue */}
        <div role="dialog" aria-label="Lancer la partie" onKeyDown={(e) => e.key === 'Escape' && popupEscape()}>
          <button>Annuler</button>
        </div>
        <NetworkLostOverlay />
      </>
    )
    expect(screen.getByRole('alertdialog')).toBeInTheDocument()
    fireEvent.keyDown(document.activeElement, { key: 'Escape' })
    expect(screen.queryByRole('alertdialog')).toBeNull()
    expect(popupEscape).not.toHaveBeenCalled()
    expect(screen.getByRole('dialog', { name: 'Lancer la partie' })).toBeInTheDocument()
  })
})
