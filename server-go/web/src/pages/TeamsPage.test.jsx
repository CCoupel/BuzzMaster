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

// ---------------------------------------------------------------------------
// Describe : badge ⏱ ACK_PENDING dans TeamsPage (v3.8.0 #54)
// ---------------------------------------------------------------------------

describe('TeamsPage — badge ACK_PENDING (v3.8.0 #54)', () => {

  // Test 1 : badge présent pour un buzzer physique non assigné avec ACK_PENDING=true
  it('badge ACK_PENDING visible quand ACK_PENDING=true sur buzzer physique non assigné', () => {
    useGame.mockReturnValue(makeGameMock({
      'AA:BB:CC:DD:EE:10': { NAME: 'Buzzer10', ACK_PENDING: true },
    }))

    render(<TeamsPage />)

    const badge = screen.getByTitle('En attente de confirmation')
    expect(badge).toBeInTheDocument()
  })

  // Test 2 : badge absent quand ACK_PENDING=false
  it('badge ACK_PENDING absent quand ACK_PENDING=false', () => {
    useGame.mockReturnValue(makeGameMock({
      'AA:BB:CC:DD:EE:11': { NAME: 'Buzzer11', ACK_PENDING: false },
    }))

    render(<TeamsPage />)

    expect(screen.queryByTitle('En attente de confirmation')).toBeNull()
  })

  // Test 3 : badge absent quand ACK_PENDING absent (firmware pré-v3.8.0)
  it('badge ACK_PENDING absent quand ACK_PENDING est undefined (firmware pré-v3.8.0)', () => {
    useGame.mockReturnValue(makeGameMock({
      'AA:BB:CC:DD:EE:12': { NAME: 'OldFirmware' },
    }))

    render(<TeamsPage />)

    expect(screen.queryByTitle('En attente de confirmation')).toBeNull()
  })

  // Test 4 : badge absent pour buzzer IS_VIRTUAL même si ACK_PENDING=true
  it('badge ACK_PENDING absent quand IS_VIRTUAL=true même si ACK_PENDING=true', () => {
    useGame.mockReturnValue(makeGameMock({
      'AA:BB:CC:DD:EE:13': { NAME: 'Virtual1', ACK_PENDING: true, IS_VIRTUAL: true },
    }))

    render(<TeamsPage />)

    expect(screen.queryByTitle('En attente de confirmation')).toBeNull()
  })

  // Test 5 : badge absent pour IS_VPLAYER=true même si ACK_PENDING=true
  it('badge ACK_PENDING absent quand IS_VPLAYER=true même si ACK_PENDING=true', () => {
    useGame.mockReturnValue(makeGameMock({
      'AA:BB:CC:DD:EE:14': { NAME: 'VPlayer1', ACK_PENDING: true, IS_VPLAYER: true },
    }))

    render(<TeamsPage />)

    expect(screen.queryByTitle('En attente de confirmation')).toBeNull()
  })

  // Test 6 : badge présent dans la section équipe (buzzer assigné)
  it('badge ACK_PENDING visible dans la section équipe pour buzzer assigné', () => {
    useGame.mockReturnValue(makeGameMock(
      {
        'AA:BB:CC:DD:EE:15': { NAME: 'Buzzer15', TEAM: 'blue', ACK_PENDING: true },
      },
      {
        blue: { NAME: 'Équipe Bleue', COLOR: [99, 102, 241] },
      }
    ))

    render(<TeamsPage />)

    const badge = screen.getByTitle('En attente de confirmation')
    expect(badge).toBeInTheDocument()
  })

  // Test 7 : plusieurs buzzers — seul le buzzer pending a le badge
  it('badge ACK_PENDING uniquement sur les buzzers physiques en attente', () => {
    useGame.mockReturnValue(makeGameMock({
      'AA:BB:CC:DD:EE:01': { NAME: 'Normal',   ACK_PENDING: false },
      'AA:BB:CC:DD:EE:02': { NAME: 'Pending',  ACK_PENDING: true },
      'AA:BB:CC:DD:EE:03': { NAME: 'Virtual',  ACK_PENDING: true, IS_VIRTUAL: true },
      'AA:BB:CC:DD:EE:04': { NAME: 'VPlayer',  ACK_PENDING: true, IS_VPLAYER: true },
    }))

    render(<TeamsPage />)

    // Un seul badge (Pending uniquement — virtual et vplayer exclus)
    const badges = screen.getAllByTitle('En attente de confirmation')
    expect(badges).toHaveLength(1)
  })
})
