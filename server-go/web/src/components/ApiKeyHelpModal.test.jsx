import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import ApiKeyHelpModal from './ApiKeyHelpModal'

// Tests dérivés de bugfix/config-api-key-help (handoff
// _work/handoff/dev-frontend-20260808-164252.md, tâche 1, commit 38164db).
// Composant purement statique (aucun appel réseau) — pas de mock fetch requis,
// ce fichier n'est donc pas concerné par le blocage vitest/WSL documenté sur
// ConfigPage.test.jsx / ConfigPage.ai.test.jsx.

describe('ApiKeyHelpModal', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('renders nothing for an unknown provider', () => {
    const { container } = render(<ApiKeyHelpModal provider="unknown" onClose={() => {}} />)
    expect(container).toBeEmptyDOMElement()
  })

  it('renders nothing when provider is null', () => {
    const { container } = render(<ApiKeyHelpModal provider={null} onClose={() => {}} />)
    expect(container).toBeEmptyDOMElement()
  })

  describe('provider="anthropic"', () => {
    it('renders the Claude/Anthropic title, "Payant" badge and signup/keys links', () => {
      render(<ApiKeyHelpModal provider="anthropic" onClose={() => {}} />)

      expect(screen.getByRole('heading', { name: 'Obtenir une clé API Claude (Anthropic)' })).toBeInTheDocument()
      expect(screen.getByText('Payant')).toBeInTheDocument()

      const signupLink = screen.getByRole('link', { name: /Ouvrir console\.anthropic\.com ↗/ })
      expect(signupLink).toHaveAttribute('href', 'https://console.anthropic.com')
      expect(signupLink).toHaveAttribute('target', '_blank')
      expect(signupLink).toHaveAttribute('rel', 'noopener noreferrer')

      const keysLink = screen.getByRole('link', { name: /Ouvrir console\.anthropic\.com\/settings\/keys ↗/ })
      expect(keysLink).toHaveAttribute('href', 'https://console.anthropic.com/settings/keys')

      // Avertissement moyen de paiement — spécifique à Anthropic (contrairement à Groq)
      expect(screen.getByText(/moyen de paiement/)).toBeInTheDocument()
    })

    it('has an accessible dialog role wired to its title via aria-labelledby', () => {
      render(<ApiKeyHelpModal provider="anthropic" onClose={() => {}} />)
      const dialog = screen.getByRole('dialog')
      expect(dialog).toHaveAttribute('aria-modal', 'true')
      const title = screen.getByRole('heading', { name: 'Obtenir une clé API Claude (Anthropic)' })
      expect(dialog).toHaveAttribute('aria-labelledby', title.id)
    })
  })

  describe('provider="groq"', () => {
    it('renders the Groq title, "Gratuit — recommandé" badge and signup/keys links', () => {
      render(<ApiKeyHelpModal provider="groq" onClose={() => {}} />)

      expect(screen.getByRole('heading', { name: 'Obtenir une clé API Groq' })).toBeInTheDocument()
      expect(screen.getByText('Gratuit — recommandé')).toBeInTheDocument()

      const signupLink = screen.getByRole('link', { name: /Ouvrir console\.groq\.com ↗/ })
      expect(signupLink).toHaveAttribute('href', 'https://console.groq.com')

      const keysLink = screen.getByRole('link', { name: /Ouvrir console\.groq\.com\/keys ↗/ })
      expect(keysLink).toHaveAttribute('href', 'https://console.groq.com/keys')

      // Pas de mention de moyen de paiement pour Groq (tier gratuit)
      expect(screen.getByText(/Aucune carte bancaire requise/)).toBeInTheDocument()
    })
  })

  it('calls onClose when the "×" close button is clicked', () => {
    const onClose = vi.fn()
    render(<ApiKeyHelpModal provider="groq" onClose={onClose} />)
    fireEvent.click(screen.getByRole('button', { name: 'Fermer' }))
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('calls onClose when the overlay (outside the modal box) is clicked', () => {
    const onClose = vi.fn()
    const { container } = render(<ApiKeyHelpModal provider="groq" onClose={onClose} />)
    fireEvent.click(container.querySelector('.apikey-help-overlay'))
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('does NOT call onClose when clicking inside the modal box', () => {
    const onClose = vi.fn()
    render(<ApiKeyHelpModal provider="groq" onClose={onClose} />)
    fireEvent.click(screen.getByRole('dialog'))
    expect(onClose).not.toHaveBeenCalled()
  })

  it('calls onClose when Escape is pressed', () => {
    const onClose = vi.fn()
    render(<ApiKeyHelpModal provider="anthropic" onClose={onClose} />)
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('does not call onClose for a key other than Escape', () => {
    const onClose = vi.fn()
    render(<ApiKeyHelpModal provider="anthropic" onClose={onClose} />)
    fireEvent.keyDown(document, { key: 'Enter' })
    expect(onClose).not.toHaveBeenCalled()
  })

  it('removes the Escape keydown listener on unmount (no stale handler firing after unmount)', () => {
    const onClose = vi.fn()
    const { unmount } = render(<ApiKeyHelpModal provider="anthropic" onClose={onClose} />)
    unmount()
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(onClose).not.toHaveBeenCalled()
  })

  it('always presents the 3-step guidance ("Créer un compte", "Générer une clé API", "Coller la clé dans BuzzControl")', () => {
    render(<ApiKeyHelpModal provider="anthropic" onClose={() => {}} />)
    expect(screen.getByText('Créer un compte')).toBeInTheDocument()
    expect(screen.getByText('Générer une clé API')).toBeInTheDocument()
    expect(screen.getByText('Coller la clé dans BuzzControl')).toBeInTheDocument()
  })
})
