import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, within } from '@testing-library/react'
import QuizMetaForm from './QuizMetaForm'

// #243 (v11.2) — second point d'entrée de « LANCER LA PARTIE » : le Backstage (QuizMetaForm).
// Même popup, mêmes défauts, même payload à quatre booléens.
const gameState = { phase: 'REVEALED', quizName: 'Q', quizTheme: '', quizNotes: '', quizPopulations: [], quizDifficulties: [], quizLanguage: 'Français', quizObjectives: '', quizHiddenFields: [] }
const ctx = { teams: { A: {}, B: {} }, bumpers: { x: { IS_VPLAYER: true, CONNECTED: true }, y: { IS_VPLAYER: false } }, questions: { 1: { ID: '1', STATUS: 'REVEALED' }, 2: { ID: '2', STATUS: 'AVAILABLE' } }, refused: null }

describe('QuizMetaForm — LANCER LA PARTIE au Backstage (#243)', () => {
  it('Critical — le bouton ouvre la même popup (Scores + Questions cochées) ; la confirmation envoie les quatre booléens', () => {
    const onNewGame = vi.fn()
    render(<QuizMetaForm gameState={gameState} sendMessage={vi.fn()} onNewGame={onNewGame} launchContext={ctx} />)

    fireEvent.click(screen.getByText('LANCER LA PARTIE'))
    const dlg = screen.getByRole('dialog')
    expect(within(dlg).getByRole('checkbox', { name: /^Scores/ }).checked).toBe(true)
    expect(within(dlg).getByRole('checkbox', { name: /^Questions/ }).checked).toBe(true)
    expect(within(dlg).getByRole('checkbox', { name: /^Joueurs des tablettes/ }).checked).toBe(false)
    expect(within(dlg).getByRole('checkbox', { name: /^Équipes/ }).checked).toBe(false)
    expect(onNewGame).not.toHaveBeenCalled()

    fireEvent.click(within(dlg).getByRole('button', { name: 'Lancer la partie' }))
    expect(onNewGame).toHaveBeenCalledWith({ purgeScores: true, purgePlayers: false, purgeTeams: false, resetQuestions: true })
  })

  it('Annuler n’envoie rien ; pas de bouton sans onNewGame ; plus de « NOUVELLE PARTIE »', () => {
    const onNewGame = vi.fn()
    const { rerender } = render(<QuizMetaForm gameState={gameState} sendMessage={vi.fn()} onNewGame={onNewGame} launchContext={ctx} />)
    fireEvent.click(screen.getByText('LANCER LA PARTIE'))
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Annuler' }))
    expect(onNewGame).not.toHaveBeenCalled()
    expect(screen.queryByText('NOUVELLE PARTIE')).toBeNull()

    rerender(<QuizMetaForm gameState={gameState} sendMessage={vi.fn()} />)
    expect(screen.queryByText('LANCER LA PARTIE')).toBeNull()
  })

  it('joueurs cochés ⇒ Scores verrouillée et cochée, payload purgeScores true', () => {
    const onNewGame = vi.fn()
    render(<QuizMetaForm gameState={gameState} sendMessage={vi.fn()} onNewGame={onNewGame} launchContext={ctx} />)
    fireEvent.click(screen.getByText('LANCER LA PARTIE'))
    const dlg = screen.getByRole('dialog')
    fireEvent.click(within(dlg).getByRole('checkbox', { name: /^Joueurs des tablettes/ }))
    const scores = within(dlg).getByRole('checkbox', { name: /^Scores/ })
    expect(scores.checked).toBe(true)
    expect(scores.disabled).toBe(true)
    fireEvent.click(within(dlg).getByRole('button', { name: 'Lancer la partie' }))
    expect(onNewGame).toHaveBeenCalledWith({ purgeScores: true, purgePlayers: true, purgeTeams: false, resetQuestions: true })
  })
})
