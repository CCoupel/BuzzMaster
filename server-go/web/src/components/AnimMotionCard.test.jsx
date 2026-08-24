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
// Contrat de props RÉEL (AnimMotionCard.jsx) : { playable, revealed, card,
// motionConfig } — #184/B-F2 : subphase remplacée par le contexte d'hôte
// normalisé (utils/hostContext.js). Correspondance avec les 3 faces
// ci-dessus : SELECTED = playable:false, revealed:false (repli, seul cas
// atteint en pratique — AnimConductPanel ne monte jamais ce composant pour
// GRID/MEMORIZE) ; QUESTION = playable:true, revealed:false ;
// REVEAL = playable:false, revealed:true.
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
    const { container } = render(<AnimMotionCard playable={false} revealed={false} card={baseCard({ DIFFICULTY: 2 })} motionConfig={null} />)
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
        playable={false} revealed={false}
        card={baseCard({ DIFFICULTY: 2 })}
        motionConfig={{ POINTS_1_STAR: 2, POINTS_2_STAR: 8, POINTS_3_STAR: 15 }}
      />
    )
    expect(container.querySelector('.anim-motion-card-focus-points').textContent).toBe('8 pts')
  })

  it('accorde "pt" (singulier) pour 0 ou 1 point, "pts" (pluriel) au-delà', () => {
    const { container: c0 } = render(
      <AnimMotionCard playable={false} revealed={false} card={baseCard({ DIFFICULTY: 1 })} motionConfig={{ POINTS_1_STAR: 0 }} />
    )
    expect(c0.querySelector('.anim-motion-card-focus-points').textContent).toBe('0 pt')
    const { container: c1 } = render(
      <AnimMotionCard playable={false} revealed={false} card={baseCard({ DIFFICULTY: 1 })} motionConfig={null} />
    )
    expect(c1.querySelector('.anim-motion-card-focus-points').textContent).toBe('1 pt')
  })

  it('QUESTION_TEXT/ANSWER_TEXT ne sont PAS affichés en SELECTED (face thème uniquement)', () => {
    render(<AnimMotionCard playable={false} revealed={false} card={baseCard()} motionConfig={null} />)
    expect(screen.queryByText(/mur de Berlin/)).not.toBeInTheDocument()
    expect(screen.queryByText('1989')).not.toBeInTheDocument()
  })
})

describe('AnimMotionCard — face QUESTION', () => {
  it('affiche QUESTION_TEXT et QUESTION_IMAGE', () => {
    const { container } = render(<AnimMotionCard playable={true} revealed={false} card={baseCard()} motionConfig={null} />)
    expect(container.querySelector('.anim-motion-card-focus-verso')).not.toBeNull()
    expect(screen.getByText('En quelle année le mur de Berlin est-il tombé ?')).toBeInTheDocument()
    const img = container.querySelector('img')
    expect(img.getAttribute('src')).toBe('/img/question.png')
  })

  it('RECTO_THEME, étoiles/points et ANSWER_TEXT ne sont PAS affichés en QUESTION', () => {
    const { container } = render(<AnimMotionCard playable={true} revealed={false} card={baseCard()} motionConfig={null} />)
    expect(screen.queryByText('Histoire')).not.toBeInTheDocument()
    expect(container.querySelector('.anim-motion-card-focus-points')).toBeNull()
    expect(screen.queryByText('1989')).not.toBeInTheDocument()
  })
})

describe('AnimMotionCard — face REVEAL', () => {
  it('affiche le rappel de QUESTION_TEXT, ANSWER_IMAGE et ANSWER_TEXT en évidence', () => {
    const { container } = render(<AnimMotionCard playable={false} revealed={true} card={baseCard()} motionConfig={null} />)
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
    const { container } = render(<AnimMotionCard playable={false} revealed={true} card={baseCard()} motionConfig={null} />)
    const recall = container.querySelector('.anim-motion-card-focus-question')
    const answer = container.querySelector('.anim-motion-card-focus-answer')
    expect(recall.className).not.toBe(answer.className)
  })

  it('RECTO_THEME et les étoiles/points ne sont PAS affichés en REVEAL', () => {
    const { container } = render(<AnimMotionCard playable={false} revealed={true} card={baseCard()} motionConfig={null} />)
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
      <AnimMotionCard playable={false} revealed={false} card={baseCard({ RECTO_IMAGE: '' })} motionConfig={null} />
    )
    expect(container.querySelector('img')).toBeNull()
    expect(screen.getByText('Histoire')).toBeInTheDocument()
    expect(container.querySelector('.anim-motion-card-focus-points')).not.toBeNull()
  })

  it('QUESTION sans QUESTION_IMAGE : pas de <img>, QUESTION_TEXT toujours affiché', () => {
    const { container } = render(
      <AnimMotionCard playable={true} revealed={false} card={baseCard({ QUESTION_IMAGE: '' })} motionConfig={null} />
    )
    expect(container.querySelector('img')).toBeNull()
    expect(screen.getByText('En quelle année le mur de Berlin est-il tombé ?')).toBeInTheDocument()
  })

  it('REVEAL sans ANSWER_IMAGE : pas de <img>, ANSWER_TEXT toujours affiché', () => {
    const { container } = render(
      <AnimMotionCard playable={false} revealed={true} card={baseCard({ ANSWER_IMAGE: '' })} motionConfig={null} />
    )
    expect(container.querySelector('img')).toBeNull()
    expect(screen.getByText('1989')).toBeInTheDocument()
  })

  it('card=null/undefined : ne plante pas, ne rend rien (garde défensive, même philosophie que AnimMemoryGrid)', () => {
    expect(() => render(<AnimMotionCard playable={false} revealed={false} card={null} motionConfig={null} />)).not.toThrow()
    const { container } = render(<AnimMotionCard playable={false} revealed={false} card={null} motionConfig={null} />)
    expect(container.firstChild).toBeNull()
  })
})

// #184/B-F2 — l'ancienne garde "subphase hors SELECTED/QUESTION/REVEAL (ex.
// GRID) : ne rend rien" n'est plus exprimable avec le nouveau contrat de
// props : `playable`/`revealed` ne distinguent plus GRID de SELECTED (les
// deux valent playable:false, revealed:false — contrat question-types.md
// §4, ligne "Aucun hôte actif"). Ce n'est pas une perte de garantie réelle :
// AnimConductPanel ne monte JAMAIS ce composant pour GRID/MEMORIZE (ces
// sous-phases restent affichées par AnimMotionGrid, cf. son propre
// aiguillage) — le composant fait désormais confiance à son appelant, comme
// AnimQcmOptions/AnimAnswerZone le font déjà pour l'hôte courant. Le seul
// état "aucun hôte actif" qui atteint réellement ce composant en pratique
// est SELECTED, couvert par `describe('AnimMotionCard — face SELECTED')`
// ci-dessus.
