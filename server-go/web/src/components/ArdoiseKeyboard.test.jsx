import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import ArdoiseKeyboard from './ArdoiseKeyboard'

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

vi.mock('./ArdoiseKeyboard.css', () => ({}))

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/**
 * AZERTY layout : 3 rows (10 + 10 + 8 keys including SPACE and BACKSPACE)
 * Total unique visible keys: A-Z (26) + SPACE (1) + BACKSPACE (1) = 28 buttons
 */
const AZERTY_KEY_COUNT = 28

/**
 * NUMPAD layout : 4 rows (3 + 3 + 3 + 3 keys including '.' and BACKSPACE)
 * Keys: 7,8,9 / 4,5,6 / 1,2,3 / .,0,BACKSPACE = 12 buttons
 */
const NUMPAD_KEY_COUNT = 12

const renderKeyboard = (props = {}) =>
  render(
    <ArdoiseKeyboard
      keyboardType={props.keyboardType ?? 'AZERTY'}
      value={props.value ?? ''}
      onChange={props.onChange ?? vi.fn()}
      disabled={props.disabled ?? false}
    />
  )

// ---------------------------------------------------------------------------
// Tests : ArdoiseKeyboard (#88)
// ---------------------------------------------------------------------------

describe('ArdoiseKeyboard — layout AZERTY', () => {

  it('affiche le bon nombre de touches pour AZERTY', () => {
    renderKeyboard({ keyboardType: 'AZERTY' })

    // All key buttons (excluding the clear ✕ button)
    const keyButtons = screen.getAllByRole('button').filter(
      btn => btn.classList.contains('ardoise-key')
    )
    expect(keyButtons).toHaveLength(AZERTY_KEY_COUNT)
  })

  it('contient la classe CSS "azerty" sur le container', () => {
    const { container } = renderKeyboard({ keyboardType: 'AZERTY' })
    expect(container.querySelector('.ardoise-keyboard.azerty')).not.toBeNull()
  })

  it('affiche la touche BACKSPACE avec le label ⌫', () => {
    renderKeyboard({ keyboardType: 'AZERTY' })
    expect(screen.getByText('⌫')).toBeInTheDocument()
  })

  it('affiche la touche SPACE avec le label ␣', () => {
    renderKeyboard({ keyboardType: 'AZERTY' })
    expect(screen.getByText('␣')).toBeInTheDocument()
  })
})

describe('ArdoiseKeyboard — layout NUMPAD', () => {

  it('affiche le bon nombre de touches pour NUMPAD', () => {
    renderKeyboard({ keyboardType: 'NUMPAD' })

    const keyButtons = screen.getAllByRole('button').filter(
      btn => btn.classList.contains('ardoise-key')
    )
    expect(keyButtons).toHaveLength(NUMPAD_KEY_COUNT)
  })

  it('contient la classe CSS "numpad" sur le container', () => {
    const { container } = renderKeyboard({ keyboardType: 'NUMPAD' })
    expect(container.querySelector('.ardoise-keyboard.numpad')).not.toBeNull()
  })

  it('affiche les chiffres 0-9 et le point décimal', () => {
    renderKeyboard({ keyboardType: 'NUMPAD' })
    ;['0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '.'].forEach(digit => {
      expect(screen.getByText(digit)).toBeInTheDocument()
    })
  })
})

describe('ArdoiseKeyboard — frappe et affichage texte', () => {

  it('appelle onChange avec le texte mis à jour quand une touche est pressée', () => {
    const onChange = vi.fn()
    renderKeyboard({ keyboardType: 'AZERTY', value: 'HEL', onChange })

    // Press 'L'
    fireEvent.click(screen.getByText('L'))
    expect(onChange).toHaveBeenCalledWith('HELL')
  })

  it('appelle onChange avec texte + espace quand SPACE est pressée', () => {
    const onChange = vi.fn()
    renderKeyboard({ keyboardType: 'AZERTY', value: 'HELLO', onChange })

    fireEvent.click(screen.getByText('␣'))
    expect(onChange).toHaveBeenCalledWith('HELLO ')
  })

  it('BACKSPACE appelle onChange en supprimant le dernier caractère', () => {
    const onChange = vi.fn()
    renderKeyboard({ keyboardType: 'AZERTY', value: 'HELLO', onChange })

    fireEvent.click(screen.getByText('⌫'))
    expect(onChange).toHaveBeenCalledWith('HELL')
  })

  it('BACKSPACE sur texte vide appelle onChange avec une chaîne vide', () => {
    const onChange = vi.fn()
    renderKeyboard({ keyboardType: 'AZERTY', value: '', onChange })

    fireEvent.click(screen.getByText('⌫'))
    expect(onChange).toHaveBeenCalledWith('')
  })

  it('affiche la valeur courante dans la zone de texte', () => {
    renderKeyboard({ keyboardType: 'AZERTY', value: 'BUZZ' })

    expect(screen.getByText('BUZZ')).toBeInTheDocument()
  })

  it('affiche le placeholder quand la valeur est vide', () => {
    renderKeyboard({ keyboardType: 'AZERTY', value: '' })

    expect(screen.getByText('Votre réponse…')).toBeInTheDocument()
  })

  it('affiche le bouton Effacer tout quand valeur non vide et non disabled', () => {
    renderKeyboard({ keyboardType: 'AZERTY', value: 'BUZZ', disabled: false })

    expect(screen.getByTitle('Effacer tout')).toBeInTheDocument()
  })

  it('masque le bouton Effacer tout quand valeur vide', () => {
    renderKeyboard({ keyboardType: 'AZERTY', value: '', disabled: false })

    expect(screen.queryByTitle('Effacer tout')).toBeNull()
  })

  it('le bouton Effacer tout appelle onChange avec chaîne vide', () => {
    const onChange = vi.fn()
    renderKeyboard({ keyboardType: 'AZERTY', value: 'BUZZ', onChange })

    fireEvent.click(screen.getByTitle('Effacer tout'))
    expect(onChange).toHaveBeenCalledWith('')
  })
})

describe('ArdoiseKeyboard — prop disabled', () => {

  it('toutes les touches ardoise ont l\'attribut disabled quand disabled=true', () => {
    renderKeyboard({ keyboardType: 'AZERTY', value: '', disabled: true })

    const keyButtons = screen.getAllByRole('button').filter(
      btn => btn.classList.contains('ardoise-key')
    )
    keyButtons.forEach(btn => {
      expect(btn).toBeDisabled()
    })
  })

  it('n\'appelle pas onChange quand une touche est pressée avec disabled=true', () => {
    const onChange = vi.fn()
    renderKeyboard({ keyboardType: 'AZERTY', value: 'A', onChange, disabled: true })

    fireEvent.click(screen.getByText('B'))
    expect(onChange).not.toHaveBeenCalled()
  })

  it('affiche l\'overlay quand disabled=true', () => {
    const { container } = renderKeyboard({ keyboardType: 'AZERTY', value: '', disabled: true })

    expect(container.querySelector('.ardoise-keyboard-overlay')).not.toBeNull()
  })

  it('overlay affiche "✓ Réponse envoyée" quand disabled=true et valeur non vide', () => {
    renderKeyboard({ keyboardType: 'AZERTY', value: 'HELLO', disabled: true })

    expect(screen.getByText('✓ Réponse envoyée')).toBeInTheDocument()
  })

  it('overlay affiche "⏳ En attente…" quand disabled=true et valeur vide', () => {
    renderKeyboard({ keyboardType: 'AZERTY', value: '', disabled: true })

    expect(screen.getByText('⏳ En attente…')).toBeInTheDocument()
  })

  it('masque le bouton Effacer tout quand disabled=true même si valeur non vide', () => {
    renderKeyboard({ keyboardType: 'AZERTY', value: 'BUZZ', disabled: true })

    expect(screen.queryByTitle('Effacer tout')).toBeNull()
  })

  it('n\'affiche pas l\'overlay quand disabled=false', () => {
    const { container } = renderKeyboard({ keyboardType: 'AZERTY', value: '', disabled: false })

    expect(container.querySelector('.ardoise-keyboard-overlay')).toBeNull()
  })

  it('contient la classe CSS "disabled" sur le container quand disabled=true', () => {
    const { container } = renderKeyboard({ keyboardType: 'AZERTY', value: '', disabled: true })

    expect(container.querySelector('.ardoise-keyboard.disabled')).not.toBeNull()
  })
})
