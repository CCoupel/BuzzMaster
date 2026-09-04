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

  it('next absent (défaut undefined) : la zone SUIVANTE n\'est pas rendue non plus (même repli que les autres props)', () => {
    const { container } = render(<AnimRafaleQuestion />)
    expect(container.querySelector('.rafale-anim-qcard-next')).not.toBeInTheDocument()
  })
})

// ---------------------------------------------------------------------------
// Zone « SUIVANTE » (#202, contrat rafale.md §13) — pré-tirage de la question
// suivante, transporté par `RAFALE_ANSWER.NEXT` (§13.3), jamais par
// `GameState`. Trois valeurs distinctes pour `next` (§13.5 + doc du
// composant) :
//   - objet {ID, QUESTION, CATEGORY, DIFFICULTY} → zone rendue avec l'énoncé
//   - null → fin de réservoir, message "Dernière question du réservoir"
//   - undefined (défaut) → garde anti-obsolescence côté appelant (AnimPage.jsx)
//     a jugé le NEXT périmé pour la question courante (ID ne correspondant
//     plus) OU pas encore reçu — la zone ne rend RIEN plutôt que d'afficher
//     une information potentiellement fausse (même discipline qu'answerValue).
// `showNext` (sous-phase QUESTION, §13.5 dernière ligne) masque la zone
// ENTIÈREMENT quel que soit `next`, ex. ROUND_END.
// ---------------------------------------------------------------------------

describe('AnimRafaleQuestion — zone SUIVANTE : next objet (#202, contrat §13.5)', () => {
  const next = { ID: 'r-017', QUESTION: 'Plus long fleuve d\'Europe ?', CATEGORY: 'GEOGRAPHY', DIFFICULTY: 2 }
  const nextCatMeta = { icon: '🌍', label: 'Geographie' }

  it('affiche le libellé "Suivante" et l\'énoncé de la question suivante', () => {
    const { container } = render(
      <AnimRafaleQuestion current={{ QUESTION: 'Q courante' }} next={next} nextCatMeta={nextCatMeta} showNext />
    )
    expect(screen.getByText('Suivante')).toBeInTheDocument()
    expect(screen.getByText('Plus long fleuve d\'Europe ?')).toBeInTheDocument()
    expect(container.querySelector('.rafale-anim-qcard-next')).toBeInTheDocument()
  })

  it('affiche catégorie et difficulté de la question suivante, résolues séparément de la question courante', () => {
    render(
      <AnimRafaleQuestion
        current={{ QUESTION: 'Q courante', DIFFICULTY: 1 }}
        catMeta={{ icon: '📜', label: 'Histoire' }}
        next={next}
        nextCatMeta={nextCatMeta}
        showNext
      />
    )
    expect(screen.getByText('Geographie')).toBeInTheDocument()
    expect(screen.getByText('🌍')).toBeInTheDocument()
    expect(screen.getByText('★★')).toBeInTheDocument() // next.DIFFICULTY=2
    // La question courante garde sa propre catégorie, non écrasée par celle de next.
    expect(screen.getByText('Histoire')).toBeInTheDocument()
  })

  it('l\'énoncé courant et l\'énoncé suivant vivent dans des blocs DOM distincts (hiérarchie visuelle, contrat §13.6)', () => {
    const { container } = render(
      <AnimRafaleQuestion current={{ QUESTION: 'Q courante' }} next={next} nextCatMeta={nextCatMeta} showNext />
    )
    expect(container.querySelector('.rafale-anim-qcard-text').textContent).toBe('Q courante')
    expect(container.querySelector('.rafale-anim-qcard-next-text').textContent).toBe('Plus long fleuve d\'Europe ?')
  })

  it('pas de classe "dernière question" quand une vraie question suivante est fournie', () => {
    const { container } = render(
      <AnimRafaleQuestion current={{ QUESTION: 'Q courante' }} next={next} nextCatMeta={nextCatMeta} showNext />
    )
    expect(container.querySelector('.rafale-anim-qcard-next-last')).not.toBeInTheDocument()
  })
})

describe('AnimRafaleQuestion — zone SUIVANTE : next === null, fin de réservoir (#202, contrat §13.5)', () => {
  it('affiche "Dernière question du réservoir" au lieu d\'un énoncé', () => {
    const { container } = render(<AnimRafaleQuestion current={{ QUESTION: 'Q courante' }} next={null} showNext />)
    expect(screen.getByText('Dernière question du réservoir')).toBeInTheDocument()
    expect(container.querySelector('.rafale-anim-qcard-next-last')).toBeInTheDocument()
  })

  it('n\'affiche aucun chip catégorie/difficulté (rien à préparer, pas de méta pour une question qui n\'existe pas)', () => {
    const { container } = render(
      <AnimRafaleQuestion current={{ QUESTION: 'Q courante' }} next={null} nextCatMeta={{ icon: '🌍', label: 'Geographie' }} showNext />
    )
    expect(container.querySelector('.rafale-anim-qcard-next-label').querySelectorAll('.anim-chip').length).toBe(0)
  })

  it('la question COURANTE reste affichée normalement (seule la zone SUIVANTE change)', () => {
    render(<AnimRafaleQuestion current={{ QUESTION: 'Q courante' }} next={null} showNext />)
    expect(screen.getByText('Q courante')).toBeInTheDocument()
  })
})

describe('AnimRafaleQuestion — zone SUIVANTE : next === undefined, périmé ou pas encore reçu (#202)', () => {
  it('NEXT périmé (garde anti-obsolescence de l\'appelant, ID ne correspondant plus à la question courante) : la zone SUIVANTE n\'est PAS rendue', () => {
    // AnimPage.jsx transmet `undefined` (jamais `null`) quand
    // rafaleAnswer.ID !== RAFALE_CURRENT_QUESTION.ID — reproduit ici tel
    // quel, sans dépendre de la logique de dérivation d'AnimPage.jsx.
    const { container } = render(<AnimRafaleQuestion current={{ QUESTION: 'Q courante' }} next={undefined} showNext />)
    expect(container.querySelector('.rafale-anim-qcard-next')).not.toBeInTheDocument()
    expect(screen.queryByText('Suivante')).not.toBeInTheDocument()
    expect(screen.queryByText('Dernière question du réservoir')).not.toBeInTheDocument()
  })

  it('la question courante reste affichée même quand la zone SUIVANTE est absente', () => {
    render(<AnimRafaleQuestion current={{ QUESTION: 'Q courante' }} next={undefined} showNext />)
    expect(screen.getByText('Q courante')).toBeInTheDocument()
  })
})

describe('AnimRafaleQuestion — zone SUIVANTE : showNext (sous-phase QUESTION, #202, contrat §13.5 dernière ligne)', () => {
  const next = { ID: 'r-017', QUESTION: 'Plus long fleuve d\'Europe ?', CATEGORY: 'GEOGRAPHY', DIFFICULTY: 2 }

  it('showNext === false (ex. ROUND_END) : la zone SUIVANTE est entièrement masquée, MÊME avec un next valide', () => {
    const { container } = render(
      <AnimRafaleQuestion current={{ QUESTION: 'Q courante' }} next={next} showNext={false} />
    )
    expect(container.querySelector('.rafale-anim-qcard-next')).not.toBeInTheDocument()
    expect(screen.queryByText('Plus long fleuve d\'Europe ?')).not.toBeInTheDocument()
  })

  it('showNext === false : masque aussi le message "dernière question" (next === null)', () => {
    const { container } = render(<AnimRafaleQuestion current={{ QUESTION: 'Q courante' }} next={null} showNext={false} />)
    expect(container.querySelector('.rafale-anim-qcard-next')).not.toBeInTheDocument()
    expect(screen.queryByText('Dernière question du réservoir')).not.toBeInTheDocument()
  })

  it('showNext absent (repli true, rétrocompatibilité documentée) : la zone SUIVANTE est rendue si next est fourni', () => {
    const { container } = render(<AnimRafaleQuestion current={{ QUESTION: 'Q courante' }} next={next} />)
    expect(container.querySelector('.rafale-anim-qcard-next')).toBeInTheDocument()
  })
})

// ---------------------------------------------------------------------------
// current.POINTS — barème résolu de la question en cours (#216, milestone
// v9.0.0, Lot 2, contrats/rafale.md §4 : "la valeur résolue pour la question
// EN COURS est diffusée dans RAFALE_CURRENT_QUESTION.POINTS ... pour
// l'affichage TV + animateur"). NOUVEAU champ, valeur variable d'une
// question à l'autre selon sa difficulté (RAFALE_POINTS_BY_DIFFICULTY côté
// backend) — contrairement à DIFFICULTY (étoiles) déjà affiché ci-dessus,
// rien n'existait avant #216 pour ce champ : assertions volontairement
// tolérantes au format exact (le nombre de points doit être visible dans la
// zone méta, peu importe le gabarit textuel choisi par dev-frontend).
// ---------------------------------------------------------------------------

describe('AnimRafaleQuestion — current.POINTS : valeur en points de la question courante (#216)', () => {
  it('current.POINTS fourni : la valeur est visible dans la zone méta de la question', () => {
    const { container } = render(<AnimRafaleQuestion current={{ QUESTION: 'Q', DIFFICULTY: 2, POINTS: 15 }} />)
    const meta = container.querySelector('.rafale-anim-qcard-meta')
    expect(meta).not.toBeNull()
    expect(meta.textContent).toMatch(/15/)
  })

  it('current.POINTS change d\'une question à l\'autre (barème par difficulté, pas une valeur figée)', () => {
    const { container, rerender } = render(<AnimRafaleQuestion current={{ QUESTION: 'Q1', DIFFICULTY: 1, POINTS: 5 }} />)
    expect(container.querySelector('.rafale-anim-qcard-meta').textContent).toMatch(/5/)

    rerender(<AnimRafaleQuestion current={{ QUESTION: 'Q2', DIFFICULTY: 3, POINTS: 25 }} />)
    expect(container.querySelector('.rafale-anim-qcard-meta').textContent).toMatch(/25/)
  })

  it('current.POINTS absent/0 : ne plante pas, aucun chip "0" trompeur affiché', () => {
    const { container } = render(<AnimRafaleQuestion current={{ QUESTION: 'Q', DIFFICULTY: 1 }} />)
    expect(container.querySelector('.rafale-anim-qcard-meta')).not.toBeNull()
    expect(screen.queryByText('0')).not.toBeInTheDocument()
  })
})
