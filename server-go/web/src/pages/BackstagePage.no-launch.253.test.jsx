import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import BackstagePage from './BackstagePage'
import { useGame } from '../hooks/GameContext'

// #253 (v11.2) — le bouton « LANCER LA PARTIE » est aussi retiré de la page Backstage (décision de l'utilisateur :
// une seule entrée, dans le menu Préparation de la barre de navigation).

vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn(), GameProvider: ({ children }) => children }))
vi.mock('../components/Button', () => ({ default: ({ children, onClick, disabled, type, loading, ...rest }) => (<button onClick={onClick} disabled={disabled} type={type || 'button'} {...rest}>{children}</button>) }))
vi.mock('../components/Card', () => ({
  default: ({ children, className, padding, variant, ...rest }) => <div className={className} {...rest}>{children}</div>,
  CardHeader: ({ children }) => <div>{children}</div>, CardBody: ({ children }) => <div>{children}</div>,
}))
vi.mock('./BackstagePage.css', () => ({}))
vi.mock('./ConfigPage.css', () => ({}))

describe('BackstagePage — plus de « Lancer la partie » (#253)', () => {
  it('Critical — aucun bouton ni popup de lancement sur le Backstage', () => {
    useGame.mockReturnValue({
      gameState: { phase: 'STOPPED', entracte: false, entracteConfig: { TITLE: 'ENTRACTE', SUBTITLE: '', IMAGE_IS_CUSTOM: false, PANEL_SIZE: 65, ANIM_PERIOD: 10, ANIM_INTENSITY: 20 }, quizName: 'Q', quizPopulations: [], quizDifficulties: [], quizHiddenFields: [] },
      sendMessage: vi.fn(), teams: {}, bumpers: {}, questions: {}, newGame: vi.fn(),
    })
    render(<BackstagePage />)
    expect(screen.queryByText(/LANCER LA PARTIE/i)).toBeNull()
    expect(screen.queryByText('NOUVELLE PARTIE')).toBeNull()
    expect(screen.queryByRole('dialog')).toBeNull()
  })
})
