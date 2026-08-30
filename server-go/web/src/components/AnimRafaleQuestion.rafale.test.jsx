import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import AnimRafaleQuestion from './AnimRafaleQuestion'

// ---------------------------------------------------------------------------
// AnimRafaleQuestion — encart question+réponse RAFALE, zone centrale (L3) de
// /anim (milestone v8.0.0, #198, retour QUALIF 8.0.0.13). Composant
// PUREMENT présentationnel monté par AnimConductPanel.jsx en `<AnimRafaleQuestion
// {...(rafale || {})} />` — testé ici directement, sans mock, pour vérifier
// le CONTENU RÉELLEMENT RENDU (texte question/réponse visible dans le DOM),
// pas seulement que les props sont bien transmises (déjà couvert côté
// câblage par AnimPage.rafale.test.jsx, data-attributes sur AnimConductPanel
// mocké — ce fichier-ci comble le trou : AnimConductPanel.test.jsx ne
// mentionne RAFALE nulle part malgré le commentaire du commit 67e06294
// affirmant que "le rendu visuel final est couvert par AnimConductPanel.test.jsx").
// ---------------------------------------------------------------------------

describe('AnimRafaleQuestion — question et réponse affichées ENSEMBLE, sans masquage', () => {
  it('affiche le texte de la question', () => {
    render(<AnimRafaleQuestion current={{ QUESTION: 'Capitale de l\'Italie ?' }} />)
    expect(screen.getByText('Capitale de l\'Italie ?')).toBeInTheDocument()
  })

  it('affiche la réponse avec le libellé "Réponse attendue" quand answerValue est fourni', () => {
    render(<AnimRafaleQuestion current={{ QUESTION: 'Capitale de l\'Italie ?' }} answerValue="Rome" />)
    expect(screen.getByText('Réponse attendue')).toBeInTheDocument()
    expect(screen.getByText('Rome')).toBeInTheDocument()
  })

  it('answerValue absent (réponse pas encore reçue) : le bloc réponse n\'est PAS rendu, mais la question reste visible', () => {
    render(<AnimRafaleQuestion current={{ QUESTION: 'Capitale de l\'Italie ?' }} answerValue="" />)
    expect(screen.getByText('Capitale de l\'Italie ?')).toBeInTheDocument()
    expect(screen.queryByText('Réponse attendue')).not.toBeInTheDocument()
  })

  it('current.QUESTION absent : aucun paragraphe question rendu, ne plante pas', () => {
    const { container } = render(<AnimRafaleQuestion current={{}} />)
    expect(container.querySelector('.rafale-anim-qcard-text')).not.toBeInTheDocument()
  })
})

describe('AnimRafaleQuestion — méta (équipe, catégorie, difficulté, progression)', () => {
  it('teamName fourni (mode multi) : affiché comme chip équipe', () => {
    const { container } = render(<AnimRafaleQuestion current={{ QUESTION: 'Q' }} teamName="Équipe A" />)
    expect(screen.getByText('Équipe A')).toBeInTheDocument()
    expect(container.querySelector('.rafale-anim-qcard-team').textContent).toBe('Équipe A')
  })

  it('teamName absent (mode SOLO) : aucun chip équipe rendu', () => {
    const { container } = render(<AnimRafaleQuestion current={{ QUESTION: 'Q' }} teamName="" />)
    expect(container.querySelector('.rafale-anim-qcard-team')).not.toBeInTheDocument()
  })

  it('catMeta fourni : icône + libellé de catégorie affichés', () => {
    render(<AnimRafaleQuestion current={{ QUESTION: 'Q' }} catMeta={{ icon: '🌍', label: 'Geographie' }} />)
    expect(screen.getByText('Geographie')).toBeInTheDocument()
    expect(screen.getByText('🌍')).toBeInTheDocument()
  })

  it('catMeta null : aucun chip catégorie rendu', () => {
    render(<AnimRafaleQuestion current={{ QUESTION: 'Q' }} catMeta={null} />)
    expect(screen.queryByText('Geographie')).not.toBeInTheDocument()
  })

  it('DIFFICULTY > 0 : affiche autant d\'étoiles que la difficulté', () => {
    render(<AnimRafaleQuestion current={{ QUESTION: 'Q', DIFFICULTY: 3 }} />)
    expect(screen.getByText('★★★')).toBeInTheDocument()
  })

  it('DIFFICULTY absent ou 0 : aucun chip étoiles rendu (pas de "0 étoile" trompeur)', () => {
    const { container: c1 } = render(<AnimRafaleQuestion current={{ QUESTION: 'Q', DIFFICULTY: 0 }} />)
    expect(c1.querySelectorAll('.anim-chip').length).toBe(0)
    const { container: c2 } = render(<AnimRafaleQuestion current={{ QUESTION: 'Q' }} />)
    expect(c2.querySelectorAll('.anim-chip').length).toBe(0)
  })

  it('askedCount > 0 : affiche "question N"', () => {
    render(<AnimRafaleQuestion current={{ QUESTION: 'Q' }} askedCount={5} />)
    expect(screen.getByText('question 5')).toBeInTheDocument()
  })

  it('askedCount à 0 (défaut) : aucun chip progression rendu', () => {
    const { container } = render(<AnimRafaleQuestion current={{ QUESTION: 'Q' }} />)
    expect(screen.queryByText(/^question \d/)).not.toBeInTheDocument()
    expect(container.querySelectorAll('.anim-chip').length).toBe(0)
  })

  it('teamColorCss transmis en variable CSS --rafale-active-color (repli var(--error) par défaut)', () => {
    const { container: withColor } = render(<AnimRafaleQuestion current={{ QUESTION: 'Q' }} teamColorCss="rgb(99,102,241)" />)
    expect(withColor.querySelector('.rafale-anim-qcard').style.getPropertyValue('--rafale-active-color')).toBe('rgb(99,102,241)')

    const { container: withoutColor } = render(<AnimRafaleQuestion current={{ QUESTION: 'Q' }} />)
    expect(withoutColor.querySelector('.rafale-anim-qcard').style.getPropertyValue('--rafale-active-color')).toBe('var(--error)')
  })
})

describe('AnimRafaleQuestion — aucune prop (montage isolé, repli de AnimConductPanel.jsx §"rafale || {}")', () => {
  it('ne plante pas et ne rend aucun contenu optionnel', () => {
    const { container } = render(<AnimRafaleQuestion />)
    expect(container.querySelector('.rafale-anim-qcard')).toBeInTheDocument()
    expect(container.querySelector('.rafale-anim-qcard-text')).not.toBeInTheDocument()
    expect(container.querySelector('.rafale-anim-qcard-answer')).not.toBeInTheDocument()
    expect(container.querySelector('.rafale-anim-qcard-team')).not.toBeInTheDocument()
  })
})
