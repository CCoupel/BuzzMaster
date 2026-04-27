import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import TeamsPage from './TeamsPage'

// ---------------------------------------------------------------------------
// Mocks
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
    motion: {
      div: makeEl('div'),
      button: makeEl('button'),
      span: makeEl('span'),
    },
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

// ---------------------------------------------------------------------------
// Helper : mock useGame minimal pour TeamsPage
// ---------------------------------------------------------------------------
import { useGame } from '../hooks/GameContext'

const makeGameMock = (bumpers = {}, teams = {}) => ({
  teams,
  bumpers,
  gameState: { virtualPlayerCount: 0, enrollmentActive: false },
  updateConfig: vi.fn(),
  showQRCode: vi.fn(),
  hideQRCode: vi.fn(),
  setVirtualPlayerLimit: vi.fn(),
})

// ---------------------------------------------------------------------------
// Describe : badge ⚠ buzzer déconnecté dans TeamsPage (v3.6.8)
// ---------------------------------------------------------------------------

describe('TeamsPage — badge ⚠ buzzer déconnecté', () => {

  // Test 1 : badge présent pour un buzzer physique déconnecté (non assigné)
  it('badge ⚠ visible quand CONNECTED=false sur buzzer physique non assigné', () => {
    useGame.mockReturnValue(makeGameMock({
      'AA:BB:CC:DD:EE:01': { NAME: 'Buzzer1', CONNECTED: false },
    }))

    render(<TeamsPage />)

    // Le badge est implémenté en SVG (pas de texte), on vérifie sa présence via le title
    const badge = screen.getByTitle('Buzzer déconnecté')
    expect(badge).toBeInTheDocument()
  })

  // Test 2 : badge absent quand CONNECTED=true
  it('badge ⚠ absent quand CONNECTED=true', () => {
    useGame.mockReturnValue(makeGameMock({
      'AA:BB:CC:DD:EE:02': { NAME: 'Buzzer2', CONNECTED: true },
    }))

    render(<TeamsPage />)

    expect(screen.queryByTitle('Buzzer déconnecté')).toBeNull()
  })

  // Test 3 : badge absent quand CONNECTED=false mais IS_VIRTUAL=true
  it('badge ⚠ absent quand IS_VIRTUAL=true même si CONNECTED=false', () => {
    useGame.mockReturnValue(makeGameMock({
      'AA:BB:CC:DD:EE:03': { NAME: 'Virtual1', CONNECTED: false, IS_VIRTUAL: true },
    }))

    render(<TeamsPage />)

    expect(screen.queryByTitle('Buzzer déconnecté')).toBeNull()
  })

  // Test 4 : badge absent quand CONNECTED=false mais IS_VPLAYER=true
  it('badge ⚠ absent quand IS_VPLAYER=true même si CONNECTED=false', () => {
    useGame.mockReturnValue(makeGameMock({
      'AA:BB:CC:DD:EE:04': { NAME: 'VPlayer1', CONNECTED: false, IS_VPLAYER: true },
    }))

    render(<TeamsPage />)

    expect(screen.queryByTitle('Buzzer déconnecté')).toBeNull()
  })

  // Test 5 : badge présent dans la section équipe (buzzer assigné à une équipe)
  it('badge ⚠ visible dans la section équipe pour buzzer assigné déconnecté', () => {
    useGame.mockReturnValue(makeGameMock(
      {
        'AA:BB:CC:DD:EE:05': { NAME: 'Buzzer5', TEAM: 'red', CONNECTED: false },
      },
      {
        red: { NAME: 'Équipe Rouge', COLOR: [255, 0, 0] },
      }
    ))

    render(<TeamsPage />)

    // Le badge est implémenté en SVG (pas de texte), on vérifie sa présence via le title
    const badge = screen.getByTitle('Buzzer déconnecté')
    expect(badge).toBeInTheDocument()
  })

  // Test 6 : plusieurs buzzers — seuls les déconnectés physiques ont le badge
  it('badge ⚠ uniquement sur les buzzers physiques déconnectés (plusieurs buzzers)', () => {
    useGame.mockReturnValue(makeGameMock({
      'AA:BB:CC:DD:EE:01': { NAME: 'Connected',     CONNECTED: true },
      'AA:BB:CC:DD:EE:02': { NAME: 'Disconnected',  CONNECTED: false },
      'AA:BB:CC:DD:EE:03': { NAME: 'Virtual',       CONNECTED: false, IS_VIRTUAL: true },
      'AA:BB:CC:DD:EE:04': { NAME: 'VPlayer',       CONNECTED: false, IS_VPLAYER: true },
    }))

    render(<TeamsPage />)

    // Un seul badge attendu (Disconnected uniquement)
    const badges = screen.getAllByTitle('Buzzer déconnecté')
    expect(badges).toHaveLength(1)
  })

  it('badge absent quand CONNECTED est undefined (firmware pré-v3.6.6)', () => {
    useGame.mockReturnValue(makeGameMock({
      'AA:BB:CC:DD:EE:FF': { NAME: 'OldFirmware' }, // CONNECTED absent
    }))
    render(<TeamsPage />)
    expect(screen.queryByTitle('Buzzer déconnecté')).toBeNull()
  })
})
