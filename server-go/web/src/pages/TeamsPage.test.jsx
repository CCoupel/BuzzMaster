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
//
// [CHANGED — batch1 badge 4 états] Le badge ad-hoc conditionné sur
// `bumper.CONNECTED === false` est remplacé par <ConnectionBadge state=
// {bumper.CONN_STATE} /> (titre "Déconnecté" / "Déconnecté — message(s)
// perdu(s)" / "Reconnecté"). Voir plan
// _work/reports/planner-20260725-105503-final.md §1/§6. Tests migrés vers
// CONN_STATE — même couverture fonctionnelle, nouveau contrat de champ.
// ---------------------------------------------------------------------------

describe('TeamsPage — badge ⚠ buzzer déconnecté', () => {

  // Test 1 : badge présent pour un buzzer physique déconnecté (non assigné)
  it('badge visible quand CONN_STATE=orange sur buzzer physique non assigné', () => {
    useGame.mockReturnValue(makeGameMock({
      'AA:BB:CC:DD:EE:01': { NAME: 'Buzzer1', CONN_STATE: 'orange' },
    }))

    render(<TeamsPage />)

    // Le badge est implémenté en SVG (pas de texte), on vérifie sa présence via le title
    const badge = screen.getByTitle('Déconnecté')
    expect(badge).toBeInTheDocument()
  })

  // Test 2 : badge absent quand CONN_STATE=""
  it('badge absent quand CONN_STATE=""', () => {
    useGame.mockReturnValue(makeGameMock({
      'AA:BB:CC:DD:EE:02': { NAME: 'Buzzer2', CONN_STATE: '' },
    }))

    render(<TeamsPage />)

    expect(screen.queryByTitle('Déconnecté')).toBeNull()
  })

  // Test 3 (mis à jour #109 — commit a4716ff) : le badge est maintenant VISIBLE
  // même si IS_VIRTUAL=true. Avant #109, l'icône de déconnexion excluait volontairement
  // les bumpers IS_VIRTUAL, masquant à tort la déconnexion des VJoueurs — c'était le bug.
  // Le fix retire cette exclusion : seul CONN_STATE fait foi, comme pour un buzzer physique.
  // Voir plan _work/reports/plan-20260711-160927.md (tâche 10) et commit a4716ff.
  it('badge visible même quand IS_VIRTUAL=true si CONN_STATE=orange (#109)', () => {
    useGame.mockReturnValue(makeGameMock({
      'AA:BB:CC:DD:EE:03': { NAME: 'Virtual1', CONN_STATE: 'orange', IS_VIRTUAL: true },
    }))

    render(<TeamsPage />)

    expect(screen.getByTitle('Déconnecté')).toBeInTheDocument()
  })

  // Test 4 (mis à jour #109 — commit a4716ff) : le badge est maintenant VISIBLE
  // même si IS_VPLAYER=true. C'est le coeur du bug #109 : un VJoueur déconnecté doit
  // afficher la même icône qu'un buzzer physique déconnecté.
  it('badge visible même quand IS_VPLAYER=true si CONN_STATE=orange (#109)', () => {
    useGame.mockReturnValue(makeGameMock({
      'AA:BB:CC:DD:EE:04': { NAME: 'VPlayer1', CONN_STATE: 'orange', IS_VPLAYER: true },
    }))

    render(<TeamsPage />)

    expect(screen.getByTitle('Déconnecté')).toBeInTheDocument()
  })

  // Test 5 : badge présent dans la section équipe (buzzer assigné à une équipe)
  it('badge visible dans la section équipe pour buzzer assigné déconnecté', () => {
    useGame.mockReturnValue(makeGameMock(
      {
        'AA:BB:CC:DD:EE:05': { NAME: 'Buzzer5', TEAM: 'red', CONN_STATE: 'orange' },
      },
      {
        red: { NAME: 'Équipe Rouge', COLOR: [255, 0, 0] },
      }
    ))

    render(<TeamsPage />)

    // Le badge est implémenté en SVG (pas de texte), on vérifie sa présence via le title
    const badge = screen.getByTitle('Déconnecté')
    expect(badge).toBeInTheDocument()
  })

  // Test 6 (mis à jour #109 — commit a4716ff) : tous les bumpers déconnectés
  // affichent désormais le badge, quel que soit leur type (plus d'exclusion
  // IS_VIRTUAL/IS_VPLAYER sur ce badge précis).
  it('badge sur tous les bumpers déconnectés quel que soit leur type (plusieurs buzzers, #109)', () => {
    useGame.mockReturnValue(makeGameMock({
      'AA:BB:CC:DD:EE:01': { NAME: 'Connected',     CONN_STATE: '' },
      'AA:BB:CC:DD:EE:02': { NAME: 'Disconnected',  CONN_STATE: 'orange' },
      'AA:BB:CC:DD:EE:03': { NAME: 'Virtual',       CONN_STATE: 'orange', IS_VIRTUAL: true },
      'AA:BB:CC:DD:EE:04': { NAME: 'VPlayer',       CONN_STATE: 'orange', IS_VPLAYER: true },
    }))

    render(<TeamsPage />)

    // Trois badges attendus : Disconnected + Virtual + VPlayer (tous CONN_STATE=orange)
    const badges = screen.getAllByTitle('Déconnecté')
    expect(badges).toHaveLength(3)
  })

  it('badge absent quand CONN_STATE est undefined (firmware/bumper pré-migration)', () => {
    useGame.mockReturnValue(makeGameMock({
      'AA:BB:CC:DD:EE:FF': { NAME: 'OldFirmware' }, // CONN_STATE absent
    }))
    render(<TeamsPage />)
    expect(screen.queryByTitle('Déconnecté')).toBeNull()
  })
})

// ---------------------------------------------------------------------------
// Describe : icône de déconnexion pour les VJoueurs — couverture dédiée (#109)
// Complète la describe ci-dessus avec des scénarios explicitement centrés sur
// les champs bruts CONN_STATE/IS_VPLAYER, y compris en section équipe assignée.
// ---------------------------------------------------------------------------

describe('TeamsPage — icône de déconnexion pour les VJoueurs (#109)', () => {

  it('badge visible pour un VJoueur assigné à une équipe et déconnecté (IS_VPLAYER + CONN_STATE=orange)', () => {
    useGame.mockReturnValue(makeGameMock(
      {
        vjoueur_alice: { NAME: 'Alice', TEAM: 'red', CONN_STATE: 'orange', IS_VPLAYER: true, IS_VIRTUAL: true },
      },
      {
        red: { NAME: 'Équipe Rouge', COLOR: [255, 0, 0] },
      }
    ))

    render(<TeamsPage />)

    expect(screen.getByTitle('Déconnecté')).toBeInTheDocument()
  })

  it('badge absent pour un VJoueur connecté (IS_VPLAYER=true, CONN_STATE="")', () => {
    useGame.mockReturnValue(makeGameMock({
      vjoueur_bob: { NAME: 'Bob', CONN_STATE: '', IS_VPLAYER: true, IS_VIRTUAL: true },
    }))

    render(<TeamsPage />)

    expect(screen.queryByTitle('Déconnecté')).toBeNull()
  })

  it('badge "message perdu" visible pour un VJoueur en CONN_STATE=red (nouvel état #109 batch1)', () => {
    useGame.mockReturnValue(makeGameMock({
      vjoueur_dana: { NAME: 'Dana', CONN_STATE: 'red', IS_VPLAYER: true, IS_VIRTUAL: true },
    }))

    render(<TeamsPage />)

    expect(screen.getByTitle('Déconnecté — message(s) perdu(s)')).toBeInTheDocument()
  })

  it('badge "Reconnecté" visible pour un VJoueur en CONN_STATE=green (nouvel état #109 batch1)', () => {
    useGame.mockReturnValue(makeGameMock({
      vjoueur_evan: { NAME: 'Evan', CONN_STATE: 'green', IS_VPLAYER: true, IS_VIRTUAL: true },
    }))

    render(<TeamsPage />)

    expect(screen.getByTitle('Reconnecté')).toBeInTheDocument()
  })

  it('non-régression : badge toujours présent pour un buzzer physique déconnecté après le fix #109', () => {
    useGame.mockReturnValue(makeGameMock({
      'AA:BB:CC:DD:EE:20': { NAME: 'Physique', CONN_STATE: 'orange' },
    }))

    render(<TeamsPage />)

    expect(screen.getByTitle('Déconnecté')).toBeInTheDocument()
  })

  it('non-régression : badge ACK_PENDING reste exclu pour un VJoueur (comportement inchangé, hors scope #109)', () => {
    useGame.mockReturnValue(makeGameMock({
      vjoueur_carla: { NAME: 'Carla', ACK_PENDING: true, IS_VPLAYER: true, IS_VIRTUAL: true },
    }))

    render(<TeamsPage />)

    expect(screen.queryByTitle('En attente de confirmation')).toBeNull()
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
