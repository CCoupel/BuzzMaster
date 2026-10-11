import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import QuizMetaForm from './QuizMetaForm'

// #243 → #253 (v11.2) — « LANCER LA PARTIE » a quitté le Backstage : une seule entrée, dans le menu Préparation de
// la barre de navigation (changement CHANGED du contrat #253, confirmé par l'utilisateur). Le contenu de la popup
// est couvert par GamePage.launch-game.243.test.jsx et LaunchGameDialog.test.jsx.
const gameState = { phase: 'REVEALED', quizName: 'Q', quizTheme: '', quizNotes: '', quizPopulations: [], quizDifficulties: [], quizLanguage: 'Français', quizObjectives: '', quizHiddenFields: [] }

describe('QuizMetaForm — plus de bouton de lancement (#253)', () => {
  it('Critical — ni « LANCER LA PARTIE » ni « NOUVELLE PARTIE » dans le formulaire du Backstage', () => {
    render(<QuizMetaForm gameState={gameState} sendMessage={vi.fn()} />)
    expect(screen.queryByText(/LANCER LA PARTIE/i)).toBeNull()
    expect(screen.queryByText('NOUVELLE PARTIE')).toBeNull()
    expect(screen.queryByRole('dialog')).toBeNull()
  })
})
