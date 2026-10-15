import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, within } from '@testing-library/react'
import QcmAnswersEditor from './QcmAnswersEditor'

// ---------------------------------------------------------------------------
// QcmAnswersEditor — prop `showHints` (#279, LOT-6, v11.2.2, contrat
// question-types.md §7.1 : « une carte QCM n'a pas d'indices »).
//
//  - `showHints` vaut true PAR DÉFAUT : l'hôte QUESTION QCM garde son bloc
//    « Activer les indices progressifs » (+ seuils / pénalités) strictement
//    inchangé (NR #254).
//  - `showHints={false}` (carte MEMOTION) : le bloc d'indices n'est plus rendu,
//    même si `values` porte encore des indices résiduels ; réponses et
//    désignation de la bonne réponse restent intactes.
//
// Cible rôles / libellés, jamais les classes CSS.
// ---------------------------------------------------------------------------

const ANSWERS = { RED: 'Paris', GREEN: 'Lyon', YELLOW: 'Nice', BLUE: 'Metz' }

const HINTS_ON = {
  qcmHintsEnabled: true,
  qcmHintThreshold1: 0.25,
  qcmHintThreshold2: 0.125,
  qcmPenalty1: 0.67,
  qcmPenalty2: 0.33,
}

function renderEditor({ values, showHints, onFieldChange = vi.fn(), onAnswerChange = vi.fn() } = {}) {
  const props = {
    values: { qcmAnswers: ANSWERS, qcmCorrect: 'RED', ...values },
    onFieldChange,
    onAnswerChange,
  }
  if (showHints !== undefined) props.showHints = showHints
  const utils = render(<QcmAnswersEditor {...props} />)
  return { ...utils, onFieldChange, onAnswerChange }
}

const hintsCheckbox = () => screen.queryByRole('checkbox', { name: /Activer les indices progressifs/ })

describe('QcmAnswersEditor — showHints par défaut (hôte question QCM, inchangé)', () => {
  it('sans la prop : la case « Activer les indices progressifs » est présente', () => {
    renderEditor({ values: HINTS_ON })
    expect(hintsCheckbox()).toBeInTheDocument()
  })

  it('showHints={true} explicite : identique au défaut', () => {
    renderEditor({ values: HINTS_ON, showHints: true })
    expect(hintsCheckbox()).toBeInTheDocument()
  })

  it('indices désactivés : « Pas d\'indices automatiques », aucun réglage', () => {
    renderEditor({ values: { ...HINTS_ON, qcmHintsEnabled: false } })
    expect(hintsCheckbox()).not.toBeChecked()
    expect(screen.getByText('Pas d\'indices automatiques')).toBeInTheDocument()
    expect(screen.queryByText('Indice 1 :')).not.toBeInTheDocument()
  })

  it('indices activés : seuils et pénalités des deux indices affichés (25 % → 67 %, 12,5 % arrondi → 33 %)', () => {
    renderEditor({ values: HINTS_ON })
    expect(hintsCheckbox()).toBeChecked()
    expect(screen.getByText('Indice 1 :')).toBeInTheDocument()
    expect(screen.getByText('Indice 2 :')).toBeInTheDocument()
    expect(screen.getAllByRole('spinbutton')).toHaveLength(4)
  })

  it('cocher la case appelle onFieldChange(\'qcmHintsEnabled\', true)', () => {
    const { onFieldChange } = renderEditor({ values: { ...HINTS_ON, qcmHintsEnabled: false } })
    fireEvent.click(hintsCheckbox())
    expect(onFieldChange).toHaveBeenCalledWith('qcmHintsEnabled', true)
  })
})

describe('QcmAnswersEditor — showHints={false} (carte MEMOTION, #279)', () => {
  it('la case « Activer les indices progressifs » n\'est pas rendue', () => {
    renderEditor({ values: { ...HINTS_ON, qcmHintsEnabled: false }, showHints: false })
    expect(hintsCheckbox()).not.toBeInTheDocument()
  })

  it('aucun texte d\'indice : ni « Pas d\'indices automatiques » ni « Indice 1 / 2 »', () => {
    renderEditor({ values: { ...HINTS_ON, qcmHintsEnabled: false }, showHints: false })
    expect(screen.queryByText('Pas d\'indices automatiques')).not.toBeInTheDocument()
    expect(screen.queryByText(/Indice 1/)).not.toBeInTheDocument()
    expect(screen.queryByText(/Indice 2/)).not.toBeInTheDocument()
  })

  it('même si values porte des indices ACTIVÉS résiduels (carte héritée) : aucun bloc, aucun réglage', () => {
    renderEditor({ values: HINTS_ON, showHints: false })
    expect(hintsCheckbox()).not.toBeInTheDocument()
    expect(screen.queryByText('Indice 1 :')).not.toBeInTheDocument()
    expect(screen.queryAllByRole('spinbutton')).toHaveLength(0)
    expect(screen.queryByText(/penalites/i)).not.toBeInTheDocument()
  })

  it('values sans AUCUN champ d\'indice (forme d\'une carte après LOT-6) : se rend sans erreur', () => {
    renderEditor({ values: { qcmAnswers: ANSWERS, qcmCorrect: '' }, showHints: false })
    expect(screen.getByText('Reponses QCM *')).toBeInTheDocument()
  })

  it('les 4 réponses restent éditables et appellent onAnswerChange(couleur, texte)', () => {
    const { onAnswerChange } = renderEditor({ showHints: false })
    expect(screen.getAllByRole('textbox')).toHaveLength(4)
    fireEvent.change(screen.getByPlaceholderText('Reponse B...'), { target: { value: 'Marseille' } })
    expect(onAnswerChange).toHaveBeenCalledWith('GREEN', 'Marseille')
  })

  it('la désignation de la bonne réponse reste possible : onFieldChange(\'qcmCorrect\', couleur)', () => {
    const { onFieldChange } = renderEditor({ values: { qcmCorrect: '' }, showHints: false })
    fireEvent.click(screen.getAllByTitle('Marquer comme bonne reponse')[2])
    expect(onFieldChange).toHaveBeenCalledWith('qcmCorrect', 'YELLOW')
  })

  it('la bonne réponse désignée est marquée (✓) et l\'aide « Cliquez sur ○ » disparaît', () => {
    renderEditor({ values: { qcmCorrect: 'GREEN' }, showHints: false })
    expect(screen.getAllByTitle('Marquer comme bonne reponse')[1]).toHaveTextContent('✓')
    expect(screen.queryByText(/Cliquez sur ○/)).not.toBeInTheDocument()
  })

  it('aucune bonne réponse désignée : l\'aide « Cliquez sur ○ … » reste affichée', () => {
    renderEditor({ values: { qcmCorrect: '' }, showHints: false })
    expect(screen.getByText(/Cliquez sur ○/)).toBeInTheDocument()
  })

  it('le sous-éditeur ne rend aucune case à cocher du tout en mode carte', () => {
    const { container } = renderEditor({ values: HINTS_ON, showHints: false })
    expect(within(container).queryAllByRole('checkbox')).toHaveLength(0)
  })
})
