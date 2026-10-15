import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import AnimMotionCard from './AnimMotionCard'
import AnimMotionGrid from './AnimMotionGrid'

// ---------------------------------------------------------------------------
// AnimMotionCard — face SELECTED : le montant suit la règle POINTS_RULE de la
// carte (#190, LOT-5, v11.2.2, contrat question-types.md §6.2 « Parité
// d'affichage »), via la fonction JS unique miroir de
// `motionCardPointsForOutcome` (utils/motionGrid.js). Maquette §3 :
//   Étoiles (défaut)  → « 3 pts »              — inchangé
//   Montant fixe 10   → « 10 pts »             — valeur de la règle, pas 1 pt
//   Par unité 2       → « 2 pts / bonne rép. » (RAFALE) · « 2 pts / paire » (MEMORY)
//
// Les NR du comportement par défaut vivent dans AnimMotionCard.test.jsx (non
// modifié). Décision utilisateur (GATE 2) : les GRILLES anim / TV restent en ★
// seules — un test de non-régression le verrouille ci-dessous.
// ---------------------------------------------------------------------------

function selectedCard(overrides = {}) {
  return {
    ID: 'c1',
    RECTO_THEME: 'Question bonus',
    DIFFICULTY: 1,
    ...overrides,
  }
}

function renderSelected(card, motionConfig = null) {
  return render(<AnimMotionCard playable={false} revealed={false} card={card} motionConfig={motionConfig} />)
}

describe('AnimMotionCard (SELECTED) — le montant suit POINTS_RULE (#190)', () => {
  it('Montant fixe 10 sur une carte ★ : « 10 pts », pas le barème étoiles (1 pt)', () => {
    const { container } = renderSelected(selectedCard({ TYPE: 'QCM', DIFFICULTY: 1, POINTS_RULE: { MODE: 'FIXED', VALUE: 10 } }))
    expect(container).toHaveTextContent(/10 pts/)
    expect(container).not.toHaveTextContent(/\b1 pt\b/)
    expect(container).toHaveTextContent('★')
  })

  it('Montant fixe sur une carte ★★★ : la valeur de la règle (4), pas les 5 pts du barème étoiles', () => {
    const { container } = renderSelected(selectedCard({ TYPE: 'SPEEDY', DIFFICULTY: 3, POINTS_RULE: { MODE: 'FIXED', VALUE: 4 } }))
    expect(container).toHaveTextContent(/4 pts/)
    expect(container).not.toHaveTextContent(/5 pts/)
  })

  it('Montant fixe ignore MOTION_CONFIG (barème de la manche)', () => {
    const { container } = renderSelected(
      selectedCard({ TYPE: 'QCM', DIFFICULTY: 2, POINTS_RULE: { MODE: 'FIXED', VALUE: 10 } }),
      { POINTS_1_STAR: 2, POINTS_2_STAR: 8, POINTS_3_STAR: 15 },
    )
    expect(container).toHaveTextContent(/10 pts/)
    expect(container).not.toHaveTextContent(/8 pts/)
  })

  it('Par unité 2 sur une carte RAFALE : « 2 pts / bonne rép. »', () => {
    const { container } = renderSelected(selectedCard({ TYPE: 'RAFALE', DIFFICULTY: 2, POINTS_RULE: { MODE: 'PER_UNIT', VALUE: 2 } }))
    expect(container).toHaveTextContent(/2 pts \/ bonne rép\./)
  })

  it('Par unité 2 sur une carte MEMORY : « 2 pts / paire »', () => {
    const { container } = renderSelected(selectedCard({ TYPE: 'MEMORY', DIFFICULTY: 2, POINTS_RULE: { MODE: 'PER_UNIT', VALUE: 2 } }))
    expect(container).toHaveTextContent(/2 pts \/ paire/)
  })

  it('Étoiles pleines (STARS) explicite sur MEMORY : valeur étoiles pleine (★★★ = 5 pts)', () => {
    const { container } = renderSelected(selectedCard({ TYPE: 'MEMORY', DIFFICULTY: 3, POINTS_RULE: { MODE: 'STARS' } }))
    expect(container).toHaveTextContent(/5 pts/)
  })

  it('NR — sans POINTS_RULE (SPEEDY ★★) : « 3 pts », inchangé', () => {
    const { container } = renderSelected(selectedCard({ DIFFICULTY: 2 }))
    expect(container).toHaveTextContent(/3 pts/)
    expect(container).not.toHaveTextContent(/pts \//)
  })

  it('NR — carte MEMORY par défaut (prorata) en SELECTED : valeur étoiles maximale (★★★ = 5 pts), pas 0 ni un prorata calculé', () => {
    // La carte vient d'être choisie : aucune paire trouvée, elle reste intégralement gagnable.
    const { container } = renderSelected(selectedCard({ TYPE: 'MEMORY', DIFFICULTY: 3 }))
    expect(container).toHaveTextContent(/5 pts/)
    expect(container).not.toHaveTextContent(/\b0 pt/)
  })

  it('NR — carte RAFALE par défaut (prorata) en SELECTED : valeur étoiles (★★ = 3 pts)', () => {
    const { container } = renderSelected(selectedCard({ TYPE: 'RAFALE', DIFFICULTY: 2 }))
    expect(container).toHaveTextContent(/3 pts/)
  })

  it('le thème de la carte reste affiché quelle que soit la règle', () => {
    renderSelected(selectedCard({ TYPE: 'QCM', POINTS_RULE: { MODE: 'FIXED', VALUE: 10 } }))
    expect(screen.getByText('Question bonus')).toBeInTheDocument()
  })
})

describe('AnimMotionGrid — NR : la grille anim reste en ★ seules, aucun montant par carte (décision GATE 2, #190)', () => {
  it('une carte à Montant fixe 10 et une carte Par unité 2 : seules les étoiles sont affichées, aucun montant', () => {
    const question = {
      ID: '1',
      TYPE: 'MEMOTION',
      MOTION_CARDS: [
        { ID: 'c1', RECTO_THEME: 'Thème 1', DIFFICULTY: 1, TYPE: 'QCM', POINTS_RULE: { MODE: 'FIXED', VALUE: 10 } },
        { ID: 'c2', RECTO_THEME: 'Thème 2', DIFFICULTY: 2, TYPE: 'RAFALE', POINTS_RULE: { MODE: 'PER_UNIT', VALUE: 2 } },
        { ID: 'c3', RECTO_THEME: 'Thème 3', DIFFICULTY: 3 },
        { ID: 'c4', RECTO_THEME: 'Thème 4', DIFFICULTY: 1 },
      ],
    }
    const { container } = render(
      <AnimMotionGrid
        question={question}
        phase="STARTED"
        subphase="GRID"
        cardStates={{}}
        cardTeams={{}}
        currentTeam={null}
        selectedId={null}
        teams={{}}
        onSelect={vi.fn()}
      />
    )
    expect(container).toHaveTextContent('★★★')
    expect(container).not.toHaveTextContent(/\bpts?\b/)
    expect(container).not.toHaveTextContent(/\b10\b/)
  })
})

// ---------------------------------------------------------------------------
// Préfixe « jusqu'à » du prorata (#190, m3 de la revue, maquette
// admin_question_editor__memotion_card_points_rule §3 : « jusqu'à 5 pts ») :
// avant le jeu, le montant d'une carte en STARS_PRORATA est un MAXIMUM, pas un
// gain garanti. Les autres règles n'ont pas de préfixe.
// ---------------------------------------------------------------------------

describe('AnimMotionCard (SELECTED) — préfixe « jusqu\'à » du prorata (#190, m-B)', () => {
  it('carte MEMORY par défaut (prorata, ★★★) : le montant commence par « jusqu\'à » (« jusqu\'à 5 pts »)', () => {
    renderSelected(selectedCard({ TYPE: 'MEMORY', DIFFICULTY: 3 }))
    expect(screen.getByText(/jusqu'à 5 pts/).textContent.startsWith('jusqu\'à ')).toBe(true)
  })

  it('carte RAFALE par défaut (prorata, ★★) : « jusqu\'à 3 pts »', () => {
    renderSelected(selectedCard({ TYPE: 'RAFALE', DIFFICULTY: 2 }))
    expect(screen.getByText(/jusqu'à 3 pts/).textContent.startsWith('jusqu\'à ')).toBe(true)
  })

  it('STARS_PRORATA explicite (MEMORY ★★★) : même préfixe que le défaut', () => {
    renderSelected(selectedCard({ TYPE: 'MEMORY', DIFFICULTY: 3, POINTS_RULE: { MODE: 'STARS_PRORATA' } }))
    expect(screen.getByText(/jusqu'à 5 pts/)).toBeInTheDocument()
  })

  it('STARS explicite (tout-ou-rien, MEMORY ★★★) : « 5 pts » SANS préfixe', () => {
    const { container } = renderSelected(selectedCard({ TYPE: 'MEMORY', DIFFICULTY: 3, POINTS_RULE: { MODE: 'STARS' } }))
    expect(container).toHaveTextContent(/5 pts/)
    expect(container).not.toHaveTextContent(/jusqu'à/)
  })

  it.each([
    ['SPEEDY par défaut', { TYPE: 'SPEEDY', DIFFICULTY: 2 }],
    ['QCM Montant fixe', { TYPE: 'QCM', POINTS_RULE: { MODE: 'FIXED', VALUE: 10 } }],
    ['RAFALE Par unité', { TYPE: 'RAFALE', POINTS_RULE: { MODE: 'PER_UNIT', VALUE: 2 } }],
  ])('%s : aucun préfixe « jusqu\'à »', (_label, card) => {
    const { container } = renderSelected(selectedCard(card))
    expect(container).not.toHaveTextContent(/jusqu'à/)
  })
})
