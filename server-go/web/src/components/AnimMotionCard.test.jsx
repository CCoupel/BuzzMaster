import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import AnimMotionCard from './AnimMotionCard'

// ---------------------------------------------------------------------------
// AnimMotionCard — carte MEMOTION au premier plan de /anim (#160/F5, T4),
// montée en L3 à la place de la grille pendant SELECTED/QUESTION/REVEAL.
// Trois faces (plan §F5) :
//   - SELECTED -> RECTO_THEME + RECTO_IMAGE + étoiles + points (getMotionCardPoints)
//   - QUESTION -> QUESTION_TEXT + QUESTION_IMAGE
//   - REVEAL   -> rappel QUESTION_TEXT (atténué) + ANSWER_IMAGE + ANSWER_TEXT en évidence
// Pas d'animation de flip (le spectacle est sur /tv, la tablette est un
// outil de conduite, cf. plan §F5).
//
// Contrat de props RÉEL (AnimMotionCard.jsx) : { subphase, card, motionConfig }.
// motionConfig (question.MOTION_CONFIG) est transmis TEL QUEL — le composant
// dérive lui-même les points via utils/motionGrid.js (getMotionCardPoints),
// jamais un montant précalculé par l'appelant (même philosophie que #160/F0).
// ---------------------------------------------------------------------------

function baseCard(overrides = {}) {
  return {
    ID: 'c1',
    RECTO_THEME: 'Histoire',
    RECTO_IMAGE: '/img/recto.png',
    DIFFICULTY: 2,
    QUESTION_TEXT: 'En quelle année le mur de Berlin est-il tombé ?',
    QUESTION_IMAGE: '/img/question.png',
    ANSWER_TEXT: '1989',
    ANSWER_IMAGE: '/img/answer.png',
    ...overrides,
  }
}

describe('AnimMotionCard — face SELECTED', () => {
  it('affiche RECTO_THEME, RECTO_IMAGE, les étoiles (DIFFICULTY) et les points (getMotionCardPoints, repli 1/3/5)', () => {
    const { container } = render(<AnimMotionCard subphase="SELECTED" card={baseCard({ DIFFICULTY: 2 })} motionConfig={null} />)
    expect(container.querySelector('.anim-motion-card-focus-recto')).not.toBeNull()
    expect(screen.getByText('Histoire')).toBeInTheDocument()
    const img = container.querySelector('img')
    expect(img).not.toBeNull()
    expect(img.getAttribute('src')).toBe('/img/recto.png')
    expect(container.querySelector('.anim-motion-card-focus-stars').textContent).toBe('★★')
    expect(container.querySelector('.anim-motion-card-focus-points').textContent).toBe('3 pts') // DIFFICULTY 2, repli -> 3 pts
  })

  it('applique motionConfig (barème personnalisé) via getMotionCardPoints, pas le repli 1/3/5', () => {
    const { container } = render(
      <AnimMotionCard
        subphase="SELECTED"
        card={baseCard({ DIFFICULTY: 2 })}
        motionConfig={{ POINTS_1_STAR: 2, POINTS_2_STAR: 8, POINTS_3_STAR: 15 }}
      />
    )
    expect(container.querySelector('.anim-motion-card-focus-points').textContent).toBe('8 pts')
  })

  it('accorde "pt" (singulier) pour 0 ou 1 point, "pts" (pluriel) au-delà', () => {
    const { container: c0 } = render(
      <AnimMotionCard subphase="SELECTED" card={baseCard({ DIFFICULTY: 1 })} motionConfig={{ POINTS_1_STAR: 0 }} />
    )
    expect(c0.querySelector('.anim-motion-card-focus-points').textContent).toBe('0 pt')
    const { container: c1 } = render(
      <AnimMotionCard subphase="SELECTED" card={baseCard({ DIFFICULTY: 1 })} motionConfig={null} />
    )
    expect(c1.querySelector('.anim-motion-card-focus-points').textContent).toBe('1 pt')
  })

  it('QUESTION_TEXT/ANSWER_TEXT ne sont PAS affichés en SELECTED (face thème uniquement)', () => {
    render(<AnimMotionCard subphase="SELECTED" card={baseCard()} motionConfig={null} />)
    expect(screen.queryByText(/mur de Berlin/)).not.toBeInTheDocument()
    expect(screen.queryByText('1989')).not.toBeInTheDocument()
  })
})

describe('AnimMotionCard — face QUESTION', () => {
  it('affiche QUESTION_TEXT et QUESTION_IMAGE', () => {
    const { container } = render(<AnimMotionCard subphase="QUESTION" card={baseCard()} motionConfig={null} />)
    expect(container.querySelector('.anim-motion-card-focus-verso')).not.toBeNull()
    expect(screen.getByText('En quelle année le mur de Berlin est-il tombé ?')).toBeInTheDocument()
    const img = container.querySelector('img')
    expect(img.getAttribute('src')).toBe('/img/question.png')
  })

  it('RECTO_THEME, étoiles/points et ANSWER_TEXT ne sont PAS affichés en QUESTION', () => {
    const { container } = render(<AnimMotionCard subphase="QUESTION" card={baseCard()} motionConfig={null} />)
    expect(screen.queryByText('Histoire')).not.toBeInTheDocument()
    expect(container.querySelector('.anim-motion-card-focus-points')).toBeNull()
    expect(screen.queryByText('1989')).not.toBeInTheDocument()
  })
})

describe('AnimMotionCard — face REVEAL', () => {
  it('affiche le rappel de QUESTION_TEXT, ANSWER_IMAGE et ANSWER_TEXT en évidence', () => {
    const { container } = render(<AnimMotionCard subphase="REVEAL" card={baseCard()} motionConfig={null} />)
    expect(container.querySelector('.anim-motion-card-focus-verso')).not.toBeNull()
    const recall = container.querySelector('.anim-motion-card-focus-question')
    expect(recall).not.toBeNull()
    expect(recall.textContent).toBe('En quelle année le mur de Berlin est-il tombé ?')
    const answer = container.querySelector('.anim-motion-card-focus-answer')
    expect(answer).not.toBeNull()
    expect(answer.textContent).toBe('1989')
    const img = container.querySelector('img')
    expect(img.getAttribute('src')).toBe('/img/answer.png')
  })

  it('le rappel de question et la réponse portent des classes DISTINCTES (atténué vs en évidence)', () => {
    const { container } = render(<AnimMotionCard subphase="REVEAL" card={baseCard()} motionConfig={null} />)
    const recall = container.querySelector('.anim-motion-card-focus-question')
    const answer = container.querySelector('.anim-motion-card-focus-answer')
    expect(recall.className).not.toBe(answer.className)
  })

  it('RECTO_THEME et les étoiles/points ne sont PAS affichés en REVEAL', () => {
    const { container } = render(<AnimMotionCard subphase="REVEAL" card={baseCard()} motionConfig={null} />)
    expect(screen.queryByText('Histoire')).not.toBeInTheDocument()
    expect(container.querySelector('.anim-motion-card-focus-points')).toBeNull()
  })
})

// ---------------------------------------------------------------------------
// Repli sans image — une carte peut ne porter aucune image sur une face
// (RECTO_IMAGE/QUESTION_IMAGE/ANSWER_IMAGE absente ou vide) : le composant
// ne doit ni planter ni rendre de <img> orpheline (src vide), mais garder le
// reste du contenu (thème/texte) visible.
// ---------------------------------------------------------------------------

describe('AnimMotionCard — repli sans image', () => {
  it('SELECTED sans RECTO_IMAGE : pas de <img>, thème et points toujours affichés', () => {
    const { container } = render(
      <AnimMotionCard subphase="SELECTED" card={baseCard({ RECTO_IMAGE: '' })} motionConfig={null} />
    )
    expect(container.querySelector('img')).toBeNull()
    expect(screen.getByText('Histoire')).toBeInTheDocument()
    expect(container.querySelector('.anim-motion-card-focus-points')).not.toBeNull()
  })

  it('QUESTION sans QUESTION_IMAGE : pas de <img>, QUESTION_TEXT toujours affiché', () => {
    const { container } = render(
      <AnimMotionCard subphase="QUESTION" card={baseCard({ QUESTION_IMAGE: '' })} motionConfig={null} />
    )
    expect(container.querySelector('img')).toBeNull()
    expect(screen.getByText('En quelle année le mur de Berlin est-il tombé ?')).toBeInTheDocument()
  })

  it('REVEAL sans ANSWER_IMAGE : pas de <img>, ANSWER_TEXT toujours affiché', () => {
    const { container } = render(
      <AnimMotionCard subphase="REVEAL" card={baseCard({ ANSWER_IMAGE: '' })} motionConfig={null} />
    )
    expect(container.querySelector('img')).toBeNull()
    expect(screen.getByText('1989')).toBeInTheDocument()
  })

  it('card=null/undefined : ne plante pas, ne rend rien (garde défensive, même philosophie que AnimMemoryGrid)', () => {
    expect(() => render(<AnimMotionCard subphase="SELECTED" card={null} motionConfig={null} />)).not.toThrow()
    const { container } = render(<AnimMotionCard subphase="SELECTED" card={null} motionConfig={null} />)
    expect(container.firstChild).toBeNull()
  })
})

describe('AnimMotionCard — sous-phase inattendue', () => {
  it('subphase hors SELECTED/QUESTION/REVEAL (ex. GRID) : ne rend rien (ce composant ne remplace la grille qu\'à partir de SELECTED)', () => {
    const { container } = render(<AnimMotionCard subphase="GRID" card={baseCard()} motionConfig={null} />)
    expect(container.firstChild).toBeNull()
  })
})
