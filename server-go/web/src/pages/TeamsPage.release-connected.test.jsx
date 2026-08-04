import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import TeamsPage from './TeamsPage'
import { useGame } from '../hooks/GameContext'

// ---------------------------------------------------------------------------
// Tests : TeamsPage — avertissements "Réinscription" sur un VJoueur connecté (#134, T1.2/T1.3)
//
// Avant #134, "Réinscription" ne coupait jamais un joueur encore connecté
// (autorisation différée #122 seulement) — aucun avertissement n'était
// nécessaire. #134 lui donne l'effet attendu : éviction immédiate d'un
// joueur CONNECTED (score/équipe conservés côté serveur). Ce fichier
// verrouille les DEUX avertissements côté client (fichier:ligne
// TeamsPage.jsx, ReclaimConfirmModal, option releaseOption) — le libellé du
// bouton et l'action émise (RELEASE_BUMPER_NAME) restent inchangés dans
// tous les cas, c'est le SERVEUR qui décide du comportement selon l'état de
// connexion (contrat seat-release.md), pas le client.
// ---------------------------------------------------------------------------

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
}))

vi.mock('framer-motion', () => {
  const makeEl = (tag) => ({ children, initial, animate, exit, transition, whileHover, whileTap, ...props }) => {
    const Tag = tag
    return <Tag {...props}>{children}</Tag>
  }
  return {
    motion: { div: makeEl('div'), button: makeEl('button'), span: makeEl('span') },
    AnimatePresence: ({ children }) => children,
  }
})

vi.mock('../components/Button', () => ({
  default: ({ children, onClick, disabled, variant, size, fullWidth, ...rest }) => (
    <button onClick={onClick} disabled={disabled} {...rest}>{children}</button>
  ),
}))

vi.mock('../components/Card', () => ({
  default: ({ children, className, padding, variant, ...rest }) => (
    <div className={className} {...rest}>{children}</div>
  ),
}))

vi.mock('../components/TeamCard', () => ({
  OtaModal: () => null,
}))

vi.mock('./TeamsPage.css', () => ({}))

const MAC = 'AA:BB:CC:DD:EE:01'

const makeGameMock = (connected, enrollmentActive) => ({
  teams: { Rouges: { COLOR: [255, 0, 0], SCORE: 0 } },
  bumpers: {
    [MAC]: { NAME: 'Alice', IS_VIRTUAL: true, TEAM: 'Rouges', CONNECTED: connected },
  },
  gameState: { virtualPlayerCount: 1, enrollmentActive },
  updateConfig: vi.fn(),
  deleteBumper: vi.fn(),
  releaseBumperName: vi.fn(),
  showQRCode: vi.fn(),
  hideQRCode: vi.fn(),
  setVirtualPlayerLimit: vi.fn(),
})

const openModal = () => {
  fireEvent.click(screen.getByTitle('Supprimer le joueur'))
}

describe('TeamsPage — avertissements "Réinscription" selon CONNECTED × enrollmentActive (#134)', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('CONNECTED=true, enrollmentActive=true : avertissement "connectée" présent, "Inscriptions fermées" absent', () => {
    const mock = makeGameMock(true, true)
    useGame.mockReturnValue(mock)

    render(<TeamsPage />)
    openModal()

    expect(screen.getByText(/Alice est connectée : elle sera renvoyée à l'inscription tout de suite\./)).toBeTruthy()
    expect(screen.queryByText(/ne pourra pas se réinscrire tant qu'elles le sont/)).toBeNull()
  })

  it('CONNECTED=true, enrollmentActive=false : les DEUX avertissements sont présents', () => {
    const mock = makeGameMock(true, false)
    useGame.mockReturnValue(mock)

    render(<TeamsPage />)
    openModal()

    expect(screen.getByText(/Alice est connectée : elle sera renvoyée à l'inscription tout de suite\./)).toBeTruthy()
    expect(screen.getByText(/Inscriptions fermées : Alice ne pourra pas se réinscrire tant qu'elles le sont\./)).toBeTruthy()
  })

  it('CONNECTED=false, enrollmentActive=true : aucun des deux avertissements de Réinscription (comportement #122 inchangé)', () => {
    const mock = makeGameMock(false, true)
    useGame.mockReturnValue(mock)

    render(<TeamsPage />)
    openModal()

    expect(screen.queryByText(/est connectée : elle sera renvoyée/)).toBeNull()
    expect(screen.queryByText(/ne pourra pas se réinscrire tant qu'elles le sont/)).toBeNull()
  })

  it('CONNECTED=false, enrollmentActive=false : aucun des deux avertissements de Réinscription (comportement #122 inchangé)', () => {
    const mock = makeGameMock(false, false)
    useGame.mockReturnValue(mock)

    render(<TeamsPage />)
    openModal()

    expect(screen.queryByText(/est connectée : elle sera renvoyée/)).toBeNull()
    expect(screen.queryByText(/ne pourra pas se réinscrire tant qu'elles le sont/)).toBeNull()
  })

  it('libellé du bouton et sous-titre inchangés, que le joueur soit connecté ou non (CA8)', () => {
    useGame.mockReturnValue(makeGameMock(true, true))
    const { unmount } = render(<TeamsPage />)
    openModal()
    expect(screen.getByRole('button', { name: 'Réinscription' })).toBeTruthy()
    expect(screen.getByText('Retrouve son score et son équipe')).toBeTruthy()
    unmount()

    useGame.mockReturnValue(makeGameMock(false, true))
    render(<TeamsPage />)
    openModal()
    expect(screen.getByRole('button', { name: 'Réinscription' })).toBeTruthy()
    expect(screen.getByText('Retrouve son score et son équipe')).toBeTruthy()
  })

  it('action émise identique (releaseBumperName, jamais deleteBumper) que le joueur soit connecté ou non', () => {
    const connectedMock = makeGameMock(true, true)
    useGame.mockReturnValue(connectedMock)
    const { unmount } = render(<TeamsPage />)
    openModal()
    fireEvent.click(screen.getByRole('button', { name: 'Réinscription' }))
    expect(connectedMock.releaseBumperName).toHaveBeenCalledWith(MAC)
    expect(connectedMock.deleteBumper).not.toHaveBeenCalled()
    unmount()

    const disconnectedMock = makeGameMock(false, true)
    useGame.mockReturnValue(disconnectedMock)
    render(<TeamsPage />)
    openModal()
    fireEvent.click(screen.getByRole('button', { name: 'Réinscription' }))
    expect(disconnectedMock.releaseBumperName).toHaveBeenCalledWith(MAC)
    expect(disconnectedMock.deleteBumper).not.toHaveBeenCalled()
  })
})
