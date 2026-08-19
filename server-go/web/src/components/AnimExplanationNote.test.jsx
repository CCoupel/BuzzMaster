import { describe, it, expect } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import AnimExplanationNote from './AnimExplanationNote'

// ---------------------------------------------------------------------------
// AnimExplanationNote — note d'explication d'une question (`/anim`, L4 de
// AnimConductPanel, v6.4.x #168).
//
// Plan : _work/reports/plan-20260818-121500.md, tâche F7. Contrat :
// contracts/models.md §EXPLANATION. Même geste EXACT que AnimAnswerZone
// (#169) — mutualisé via useHoldToPeek (F6) — voir AnimAnswerZone.test.jsx
// pour la couverture exhaustive du geste lui-même (pointerdown/up/leave/
// cancel, réinitialisation au changement de question) : ce fichier se
// concentre sur les particularités propres à AnimExplanationNote (AC16-19).
// ---------------------------------------------------------------------------

describe('AnimExplanationNote — absence de note (AC17)', () => {
  it('question sans EXPLANATION : emplacement au repos, jamais un blanc', () => {
    render(<AnimExplanationNote question={{ ID: '1', TYPE: 'SPEEDY' }} revealed={false} />)
    expect(screen.getByText('Aucune note pour cette question')).toBeInTheDocument()
  })

  it('EXPLANATION uniquement composée d\'espaces : traité comme absente', () => {
    render(<AnimExplanationNote question={{ ID: '1', EXPLANATION: '   ' }} revealed={false} />)
    expect(screen.getByText('Aucune note pour cette question')).toBeInTheDocument()
  })

  it('question null (pas encore chargée) : emplacement au repos, toujours rendu', () => {
    const { container } = render(<AnimExplanationNote question={null} revealed={false} />)
    expect(screen.getByText('Aucune note pour cette question')).toBeInTheDocument()
    expect(container.querySelector('.anim-explanation-note.empty')).not.toBeNull()
  })

  it('emplacement au repos présent quel que soit revealed', () => {
    render(<AnimExplanationNote question={{ ID: '1' }} revealed={true} />)
    expect(screen.getByText('Aucune note pour cette question')).toBeInTheDocument()
  })
})

describe('AnimExplanationNote — note présente, avant REVEALED (AC16)', () => {
  const QUESTION = { ID: '1', EXPLANATION: 'Paris est la capitale depuis 508 (Clovis).' }

  it('floutée par défaut (classe "masked"), texte présent dans le DOM (pas un mécanisme de confidentialité)', () => {
    const { container } = render(<AnimExplanationNote question={QUESTION} revealed={false} />)
    const note = container.querySelector('.anim-explanation-note')
    expect(note.className).toMatch(/\bmasked\b/)
    expect(note.className).toMatch(/\banim-explanation-note-peekable\b/)
    expect(screen.getByText('Paris est la capitale depuis 508 (Clovis).')).toBeInTheDocument()
  })

  it('lisible tant qu\'un pointeur est maintenu (pointerdown) — classe "shown"', () => {
    const { container } = render(<AnimExplanationNote question={QUESTION} revealed={false} />)
    const note = container.querySelector('.anim-explanation-note')
    fireEvent.pointerDown(note)
    expect(note.className).toMatch(/\bshown\b/)
    expect(note.className).not.toMatch(/\bmasked\b/)
  })

  it('remasquée au relâchement (pointerup)', () => {
    const { container } = render(<AnimExplanationNote question={QUESTION} revealed={false} />)
    const note = container.querySelector('.anim-explanation-note')
    fireEvent.pointerDown(note)
    fireEvent.pointerUp(note)
    expect(note.className).toMatch(/\bmasked\b/)
  })

  it('le libellé change selon l\'état ("Note — maintenir pour lire" vs "Note")', () => {
    const { container } = render(<AnimExplanationNote question={QUESTION} revealed={false} />)
    expect(screen.getByText('Note — maintenir pour lire')).toBeInTheDocument()
    const note = container.querySelector('.anim-explanation-note')
    fireEvent.pointerDown(note)
    expect(screen.getByText('Note')).toBeInTheDocument()
    expect(screen.queryByText('Note — maintenir pour lire')).not.toBeInTheDocument()
  })
})

describe('AnimExplanationNote — REVEALED (AC16)', () => {
  const QUESTION = { ID: '1', EXPLANATION: 'Une note à lire.' }

  it('visible en permanence sans interaction, classe "shown"', () => {
    const { container } = render(<AnimExplanationNote question={QUESTION} revealed={true} />)
    const note = container.querySelector('.anim-explanation-note')
    expect(note.className).toMatch(/\bshown\b/)
    expect(screen.getByText('Une note à lire.')).toBeInTheDocument()
  })

  it('pas de classe "peekable" une fois REVEALED (plus d\'interaction nécessaire)', () => {
    const { container } = render(<AnimExplanationNote question={QUESTION} revealed={true} />)
    const note = container.querySelector('.anim-explanation-note')
    expect(note.className).not.toMatch(/\banim-explanation-note-peekable\b/)
  })

  it('pointerdown/pointerup n\'ont aucun effet une fois REVEALED (reste "shown")', () => {
    const { container } = render(<AnimExplanationNote question={QUESTION} revealed={true} />)
    const note = container.querySelector('.anim-explanation-note')
    fireEvent.pointerDown(note)
    fireEvent.pointerUp(note)
    expect(note.className).toMatch(/\bshown\b/)
    expect(note.className).not.toMatch(/\bmasked\b/)
  })
})

describe('AnimExplanationNote — remise à zéro au changement de question (mutualisée via useHoldToPeek, F6)', () => {
  it('un appui maintenu ne fuit pas sur la question suivante', () => {
    const q1 = { ID: '1', EXPLANATION: 'Note 1' }
    const q2 = { ID: '2', EXPLANATION: 'Note 2' }
    const { container, rerender } = render(<AnimExplanationNote question={q1} revealed={false} />)
    fireEvent.pointerDown(container.querySelector('.anim-explanation-note'))

    rerender(<AnimExplanationNote question={q2} revealed={false} />)
    const note2 = container.querySelector('.anim-explanation-note')
    expect(note2.className).toMatch(/\bmasked\b/)
    expect(note2.className).not.toMatch(/\bshown\b/)
  })
})
