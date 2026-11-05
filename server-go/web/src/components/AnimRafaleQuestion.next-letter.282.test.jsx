import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import AnimRafaleQuestion from './AnimRafaleQuestion'

// #282 (v11.4.0, LOT-2) — contracts/rafale.md §15.10 : zone « SUIVANTE » de /anim, « SUIVANTE (S) — … »,
// lettre fournie par next.LETTER (jamais normalisée côté frontend) ; vide ⇒ libellé « Suivante » v11.2.3.

const cur = { QUESTION: 'Fleuve de Paris ?' }

describe('AnimRafaleQuestion — lettre de la question suivante (#282, §15.10)', () => {
  it('next.LETTER = "S" : la zone Suivante affiche (S) et le libellé Suivante', () => {
    const { container } = render(
      <AnimRafaleQuestion current={cur} next={{ QUESTION: 'Fleuve de Rome ?', LETTER: 'S' }} />,
    )
    expect(screen.getByTestId('rafale-next-letter').textContent).toBe('(S)')
    const label = container.querySelector('.rafale-anim-qcard-next-label')
    expect(label.textContent).toMatch(/Suivante\s*\(S\)/)
    expect(screen.getByText('Fleuve de Rome ?')).toBeInTheDocument()
  })

  it('next.LETTER = "#" : affiche la lettre telle que fournie par le serveur', () => {
    render(<AnimRafaleQuestion current={cur} next={{ QUESTION: '42 ?', LETTER: '#' }} />)
    expect(screen.getByTestId('rafale-next-letter').textContent).toBe('(#)')
  })

  it('next.LETTER = "" : aucune lettre, libellé « Suivante » inchangé', () => {
    const { container } = render(<AnimRafaleQuestion current={cur} next={{ QUESTION: 'Q2', LETTER: '' }} />)
    expect(screen.queryByTestId('rafale-next-letter')).not.toBeInTheDocument()
    expect(container.querySelector('.rafale-anim-qcard-next-label').textContent).toContain('Suivante')
    expect(container.querySelector('.rafale-anim-qcard-next-label').textContent).not.toContain('(')
    expect(screen.getByText('Q2')).toBeInTheDocument()
  })

  it('next.LETTER absent : aucune lettre', () => {
    render(<AnimRafaleQuestion current={cur} next={{ QUESTION: 'Q2' }} />)
    expect(screen.queryByTestId('rafale-next-letter')).not.toBeInTheDocument()
  })

  it('next = null (dernière question) : pas de lettre, « Dernière question du réservoir »', () => {
    render(<AnimRafaleQuestion current={cur} next={null} />)
    expect(screen.queryByTestId('rafale-next-letter')).not.toBeInTheDocument()
    expect(screen.getByText('Dernière question du réservoir')).toBeInTheDocument()
  })
})
