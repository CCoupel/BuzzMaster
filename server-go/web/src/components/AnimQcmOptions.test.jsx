import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import AnimQcmOptions from './AnimQcmOptions'

// ---------------------------------------------------------------------------
// AnimQcmOptions — propositions QCM en zone contexte de /anim (#163, F3).
//
// Plan de référence : _work/reports/plan-20260814-101626.md (tâche F3) et
// maquette https://claude.ai/code/artifact/76c34d5c-74ce-4dcd-ad0d-3b102988b7af
// (état 2 « QCM en cours », état 3 « Reveal », tableau « Règles d'affichage
// par phase »). Décision GATE 2 D3 : les propositions QCM vivent dans la
// zone contexte, visibles dès que la question est chargée — aucune garde de
// phase sur les propositions elles-mêmes (seules l'invalidation et la bonne
// réponse dépendent des données/de la phase).
//
// Contrat de props (voir AnimQcmOptions.jsx) : answers (QCM_ANSWERS, hôte
// question OU carte MEMOTION active — objet {RED,GREEN,YELLOW,BLUE} ->
// libellé), correct (QCM_CORRECT), invalidated (état d'indices de l'hôte
// courant, via getTypeState), revealed (hostContext.revealed). #185/C-F1 :
// plus de prop `type` — le dispatch par type est la responsabilité de
// l'appelant, pas de ce composant (voir describe "garde pas d'answers" plus
// bas).
//
// Classes : `.anim-qcm-option` par carte, modificateurs `invalidated` et
// `correct` (non préfixés, cf. AnimQcmOptions.css), `.anim-qcm-option-mark`
// pour la coche visible uniquement sur la bonne réponse révélée.
//
// Mapping couleur -> lettre/couleur : QCM_COLORS (constants/colors.js),
// seule source, jamais réécrite ici (risque R2 du plan).
// ---------------------------------------------------------------------------

const ANSWERS = {
  RED: 'Sydney',
  GREEN: 'Canberra',
  YELLOW: 'Melbourne',
  BLUE: 'Perth',
}

describe('AnimQcmOptions — rendu des 4 propositions', () => {
  it('rend les 4 propositions, lettres A/B/C/D dans l\'ordre RED/GREEN/YELLOW/BLUE, avec leur libellé', () => {
    const { container } = render(
      <AnimQcmOptions answers={ANSWERS} correct={null} invalidated={[]} revealed={false} />
    )
    const letters = Array.from(container.querySelectorAll('.anim-qcm-option-letter')).map(el => el.textContent)
    expect(letters).toEqual(['A', 'B', 'C', 'D'])
    expect(screen.getByText('Sydney')).toBeInTheDocument()
    expect(screen.getByText('Canberra')).toBeInTheDocument()
    expect(screen.getByText('Melbourne')).toBeInTheDocument()
    expect(screen.getByText('Perth')).toBeInTheDocument()
  })

  it('colore chaque pastille lettre avec la couleur QCM_COLORS correspondante', () => {
    const { container } = render(
      <AnimQcmOptions answers={ANSWERS} correct={null} invalidated={[]} revealed={false} />
    )
    const letters = container.querySelectorAll('.anim-qcm-option-letter')
    expect(letters[0].style.backgroundColor).toBe('rgb(239, 68, 68)') // RED #ef4444
    expect(letters[1].style.backgroundColor).toBe('rgb(34, 197, 94)') // GREEN #22c55e
    expect(letters[2].style.backgroundColor).toBe('rgb(234, 179, 8)') // YELLOW #eab308
    expect(letters[3].style.backgroundColor).toBe('rgb(59, 130, 246)') // BLUE #3b82f6
  })
})

describe('AnimQcmOptions — proposition invalidée (indice, #157/#162)', () => {
  it('marque une proposition invalidée (classe "invalidated") sans la retirer du rendu', () => {
    const { container } = render(
      <AnimQcmOptions answers={ANSWERS} correct={null} invalidated={['YELLOW']} revealed={false} />
    )
    const options = container.querySelectorAll('.anim-qcm-option')
    expect(options[2].classList.contains('invalidated')).toBe(true)
    expect(options[0].classList.contains('invalidated')).toBe(false)
    expect(options[1].classList.contains('invalidated')).toBe(false)
    expect(options[3].classList.contains('invalidated')).toBe(false)
    // Grisée != retirée : le libellé reste dans le DOM (parité TV, PlayerDisplay.jsx).
    expect(screen.getByText('Melbourne')).toBeInTheDocument()
  })

  it('marque plusieurs propositions invalidées simultanément', () => {
    const { container } = render(
      <AnimQcmOptions answers={ANSWERS} correct={null} invalidated={['RED', 'BLUE']} revealed={false} />
    )
    const options = container.querySelectorAll('.anim-qcm-option')
    expect(options[0].classList.contains('invalidated')).toBe(true)
    expect(options[3].classList.contains('invalidated')).toBe(true)
    expect(options[1].classList.contains('invalidated')).toBe(false)
    expect(options[2].classList.contains('invalidated')).toBe(false)
  })

  it('ne marque aucune proposition invalidée quand la liste est vide', () => {
    const { container } = render(
      <AnimQcmOptions answers={ANSWERS} correct={null} invalidated={[]} revealed={false} />
    )
    expect(container.querySelectorAll('.anim-qcm-option.invalidated').length).toBe(0)
  })

  it('ne marque aucune proposition invalidée quand la prop est absente (undefined)', () => {
    const { container } = render(
      <AnimQcmOptions answers={ANSWERS} correct={null} revealed={false} />
    )
    expect(container.querySelectorAll('.anim-qcm-option.invalidated').length).toBe(0)
  })
})

describe('AnimQcmOptions — bonne réponse, garde de phase REVEALED', () => {
  it('ne marque PAS la bonne réponse hors REVEALED, même si QCM_CORRECT est connue (le payload la porte dès le chargement — R3 du plan)', () => {
    const { container } = render(
      <AnimQcmOptions answers={ANSWERS} correct="GREEN" invalidated={[]} revealed={false} />
    )
    expect(container.querySelectorAll('.anim-qcm-option.correct').length).toBe(0)
    expect(container.querySelector('.anim-qcm-option-mark')).toBeNull()
  })

  it('marque uniquement la bonne réponse une fois revealed=true (liseré + coche)', () => {
    const { container } = render(
      <AnimQcmOptions answers={ANSWERS} correct="GREEN" invalidated={[]} revealed={true} />
    )
    const options = container.querySelectorAll('.anim-qcm-option')
    expect(options[1].classList.contains('correct')).toBe(true)
    expect(options[0].classList.contains('correct')).toBe(false)
    expect(options[2].classList.contains('correct')).toBe(false)
    expect(options[3].classList.contains('correct')).toBe(false)
    expect(options[1].querySelector('.anim-qcm-option-mark')).not.toBeNull()
  })

  it('une proposition invalidée peut aussi être la bonne réponse en REVEALED (cas limite) — les deux marqueurs coexistent', () => {
    const { container } = render(
      <AnimQcmOptions answers={ANSWERS} correct="YELLOW" invalidated={['YELLOW']} revealed={true} />
    )
    const options = container.querySelectorAll('.anim-qcm-option')
    expect(options[2].classList.contains('invalidated')).toBe(true)
    expect(options[2].classList.contains('correct')).toBe(true)
  })
})

// #185/C-F1 — auto-garde relâchée : ce composant ne reçoit plus de prop
// `type` et ne se garde plus lui-même sur une valeur de type (le dispatch
// par type appartient désormais entièrement à l'appelant, host-aware depuis
// #184/B-F3 — AnimConductPanel/AnimMotionCard). La seule garde restante est
// `!answers`, suffisante en pratique : QCM_ANSWERS n'est jamais peuplé pour
// un autre type, question ou carte (contrat question-types.md §2/§7).
describe('AnimQcmOptions — garde "pas d\'answers"', () => {
  it('rend les propositions même sans prop `type` (dispatch délégué à l\'appelant, #185)', () => {
    const { container } = render(
      <AnimQcmOptions answers={ANSWERS} correct={null} invalidated={[]} revealed={false} />
    )
    expect(container.querySelectorAll('.anim-qcm-option').length).toBe(4)
  })

  it('ne rend rien quand answers est absent (undefined)', () => {
    const { container } = render(
      <AnimQcmOptions answers={undefined} correct={null} invalidated={[]} revealed={false} />
    )
    expect(container.firstChild).toBeNull()
  })

  it('ne rend rien quand answers est null', () => {
    const { container } = render(
      <AnimQcmOptions answers={null} correct={null} invalidated={[]} revealed={false} />
    )
    expect(container.firstChild).toBeNull()
  })
})
