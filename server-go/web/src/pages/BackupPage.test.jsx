import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import BackupPage from './BackupPage'
import { GameProvider } from '../hooks/GameContext'

// Mock GameContext
vi.mock('../hooks/GameContext', () => ({
  useGame: () => ({
    sendMessage: vi.fn(),
  }),
  GameProvider: ({ children }) => children,
}))

const renderBackupPage = () => {
  return render(
    <GameProvider>
      <BackupPage />
    </GameProvider>
  )
}

describe('BackupPage', () => {
  beforeEach(() => {
    // Clear all mocks before each test
    vi.clearAllMocks()
    // Mock window.confirm
    global.confirm = vi.fn(() => true)
    // Mock fetch
    global.fetch = vi.fn()
  })

  describe('Rendering', () => {
    it('should render page title and subtitle', () => {
      renderBackupPage()
      expect(screen.getByText('Sauvegarde et Restauration')).toBeInTheDocument()
      expect(screen.getByText('Gestion des sauvegardes et reinitialisations')).toBeInTheDocument()
    })

    it('should render three sections: Backup, Restore, Reset', () => {
      renderBackupPage()
      expect(screen.getByText('Sauvegarde')).toBeInTheDocument()
      expect(screen.getByText('Restauration')).toBeInTheDocument()
      expect(screen.getByText('Reinitialisation')).toBeInTheDocument()
    })

    it('should render all backup checkboxes', () => {
      // Les labels "Questions", "Equipes", etc. apparaissent deux fois (backup + reset)
      // On vérifie leur présence via getAllByLabelText (plusieurs résultats attendus)
      renderBackupPage()
      expect(screen.getAllByLabelText('Questions').length).toBeGreaterThanOrEqual(1)
      expect(screen.getAllByLabelText('Equipes').length).toBeGreaterThanOrEqual(1)
      expect(screen.getAllByLabelText('Joueurs').length).toBeGreaterThanOrEqual(1)
      expect(screen.getAllByLabelText('Historique').length).toBeGreaterThanOrEqual(1)
      expect(screen.getAllByLabelText('Médias & Catégories').length).toBeGreaterThanOrEqual(1)
      // #152 — case dédiée game-config.json, plus rattachée à "Historique"
      // #207 — libellé renommé « Réglages de jeu » (clé `ambiance` et paramètre
      // `ambiance=true` inchangés : les archives existantes restent restaurables)
      expect(screen.getAllByLabelText('Réglages de jeu').length).toBeGreaterThanOrEqual(1)
      expect(screen.queryByLabelText('Configuration Ambiance')).toBeNull()
    })

    it('should render all reset checkboxes', () => {
      renderBackupPage()
      const labels = screen.getAllByText('Questions')
      expect(labels.length).toBeGreaterThan(0)
    })

    it('should render action buttons', () => {
      renderBackupPage()
      const saveButtons = screen.getAllByText('Sauvegarder')
      expect(saveButtons.length).toBeGreaterThan(0)
      expect(screen.getByText('Selectionner un fichier')).toBeInTheDocument()
      expect(screen.getByText('Reinitialiser')).toBeInTheDocument()
    })
  })

  describe('Backup Options State', () => {
    it('should have all backup options checked by default', () => {
      renderBackupPage()
      // #152 — 7 cases par section depuis l'ajout de "Questions RAFALE"
      // (questions, teams, bumpers, history, medias, ambiance, rafale)
      const backupCheckboxes = screen.getAllByRole('checkbox').slice(0, 7)
      backupCheckboxes.forEach(checkbox => {
        expect(checkbox).toBeChecked()
      })
    })

    it('should toggle backup checkbox state', () => {
      renderBackupPage()
      const questionCheckbox = screen.getAllByRole('checkbox')[0]
      expect(questionCheckbox).toBeChecked()
      fireEvent.click(questionCheckbox)
      expect(questionCheckbox).not.toBeChecked()
    })
  })

  describe('Reset Options State', () => {
    it('should have all reset options unchecked by default', () => {
      renderBackupPage()
      // #152 — 7 cases par section (voir commentaire ci-dessus)
      const allCheckboxes = screen.getAllByRole('checkbox')
      const resetCheckboxes = allCheckboxes.slice(7, 14)
      resetCheckboxes.forEach(checkbox => {
        expect(checkbox).not.toBeChecked()
      })
    })

    it('should toggle reset checkbox state', () => {
      renderBackupPage()
      const allCheckboxes = screen.getAllByRole('checkbox')
      const firstResetCheckbox = allCheckboxes[7]
      expect(firstResetCheckbox).not.toBeChecked()
      fireEvent.click(firstResetCheckbox)
      expect(firstResetCheckbox).toBeChecked()
    })
  })

  // #207 — le renommage « Configuration Ambiance » → « Réglages de jeu » ne
  // touche QUE le libellé visible : le paramètre `ambiance=true` de la
  // requête de sauvegarde est inchangé (une archive produite avant le
  // renommage doit continuer à se restaurer).
  describe('#207 — renommage « Réglages de jeu », paramètre de requête inchangé', () => {
    it('la sauvegarde envoie toujours ambiance=true', async () => {
      global.fetch = vi.fn().mockResolvedValue({ blob: async () => new Blob([]) })
      global.URL.createObjectURL = vi.fn(() => 'blob:x')
      global.URL.revokeObjectURL = vi.fn()
      renderBackupPage()

      fireEvent.click(screen.getByText('Sauvegarder'))

      await waitFor(() => expect(global.fetch).toHaveBeenCalledTimes(1))
      const url = global.fetch.mock.calls[0][0]
      expect(url).toContain('ambiance=true')
      expect(url).not.toContain('lighting')
    })

    it('la réinitialisation utilise toujours la clé ambiance', async () => {
      global.fetch = vi.fn().mockResolvedValue({ ok: true })
      global.confirm = vi.fn(() => true)
      const reload = vi.fn()
      Object.defineProperty(window, 'location', { value: { reload }, writable: true })
      renderBackupPage()

      const resetCheckbox = screen.getAllByLabelText('Réglages de jeu')[1]
      fireEvent.click(resetCheckbox)
      fireEvent.click(screen.getByText('Reinitialiser'))

      await waitFor(() => expect(global.fetch).toHaveBeenCalledTimes(1))
      expect(global.fetch.mock.calls[0][0]).toContain('ambiance=true')
      expect(global.confirm.mock.calls[0][0]).toContain('Réglages de jeu')
    })
  })

  describe('Section Descriptions', () => {
    it('should display backup section description', () => {
      renderBackupPage()
      expect(screen.getByText('Selectionnez les elements a sauvegarder et generez un fichier archive.')).toBeInTheDocument()
    })

    it('should display restore section description', () => {
      renderBackupPage()
      expect(screen.getByText('Restaurez vos donnees a partir d\'un fichier archive anterieur.')).toBeInTheDocument()
    })

    it('should display reset section description', () => {
      renderBackupPage()
      expect(screen.getByText('Reinitialiser selectivement les donnees du systeme.')).toBeInTheDocument()
    })
  })

  describe('File Input', () => {
    it('should have tar file input for restore', () => {
      // "Selectionner un fichier" est un <label> wrappant un <Button as="span"> + <input type="file" hidden>
      // On recherche l'input file directement via le conteneur de la section restore
      const { container } = renderBackupPage()
      const restoreSection = container.querySelector('.restore-section')
      const fileInput = restoreSection.querySelector('input[type="file"]')
      expect(fileInput).not.toBeNull()
      expect(fileInput).toHaveAttribute('accept', '.tar')
    })
  })

  describe('Responsive Classes', () => {
    it('should render with correct CSS classes', () => {
      const { container } = renderBackupPage()
      expect(container.querySelector('.backup-page')).toBeInTheDocument()
      expect(container.querySelector('.backup-layout')).toBeInTheDocument()
      expect(container.querySelector('.backup-section')).toBeInTheDocument()
      expect(container.querySelector('.restore-section')).toBeInTheDocument()
      expect(container.querySelector('.reset-section')).toBeInTheDocument()
    })
  })
})
