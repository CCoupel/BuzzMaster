import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import AnimCreditControl from './AnimCreditControl'

// ---------------------------------------------------------------------------
// AnimCreditControl — composant de crédit UNIQUE de /anim (#170/F2, T5,
// TEST CENTRAL DU LOT).
//
// Plan : _work/reports/plan-20260816-125123.md §5.3 T5, risque R1 (§5.6).
// Contrat de props : `team` (title uniquement), `amount` (montant "+N pts",
// fourni par l'appelant, jamais recalculé ici), `awarded`
// (`awardedTeams[team]` — `undefined` = pas encore crédité, `{POINTS,
// TIMESTAMP}` sinon, POINTS 0 compris), `onCredit(points)`.
//
// ⚠️ R1 — LE PIÈGE DU LOT : le verrou est la PRÉSENCE de `awarded`, jamais
// la valeur de `awarded.POINTS`. Un `if (awarded?.POINTS)` classique
// déverrouillerait silencieusement toute ligne refusée (0 pt) — c'est très
// exactement le cas de test qui suit immédiatement chaque assertion de
// verrouillage ci-dessous : `awarded: { POINTS: 0, ... }`.
// ---------------------------------------------------------------------------

describe('AnimCreditControl — état libre (awarded absent)', () => {
  it('propose les deux gestes "+N pts" et "0 pt" quand amount est fourni', () => {
    render(<AnimCreditControl team="Les Rouges" amount={10} awarded={undefined} onCredit={() => {}} />)
    expect(screen.getByText('+10 pts')).toBeInTheDocument()
    expect(screen.getByText('0 pt')).toBeInTheDocument()
  })

  it('ne propose QUE "0 pt" quand amount est null (ligne sans réponse, ex. futur ARDOISE)', () => {
    render(<AnimCreditControl team="Les Rouges" amount={null} awarded={undefined} onCredit={() => {}} />)
    expect(screen.queryByText(/^\+.*pts$/)).not.toBeInTheDocument()
    expect(screen.getByText('0 pt')).toBeInTheDocument()
  })

  it('"+N pts" appelle onCredit(amount)', () => {
    const onCredit = vi.fn()
    render(<AnimCreditControl team="Les Rouges" amount={10} awarded={undefined} onCredit={onCredit} />)
    screen.getByText('+10 pts').click()
    expect(onCredit).toHaveBeenCalledWith(10)
  })

  it('"0 pt" appelle onCredit(0)', () => {
    const onCredit = vi.fn()
    render(<AnimCreditControl team="Les Rouges" amount={10} awarded={undefined} onCredit={onCredit} />)
    screen.getByText('0 pt').click()
    expect(onCredit).toHaveBeenCalledWith(0)
  })

  it('pas de verrouillage avant réception du payload (awarded === undefined, PAS awarded === null)', () => {
    const { container } = render(<AnimCreditControl team="Les Rouges" amount={10} awarded={undefined} onCredit={() => {}} />)
    expect(container.querySelector('.anim-credit-control-locked')).toBeNull()
  })
})

describe('AnimCreditControl — état verrouillé (awarded présent) — R1, le piège du lot', () => {
  it('verrouillé quand awarded est présent avec un montant positif', () => {
    const { container } = render(
      <AnimCreditControl team="Les Rouges" amount={10} awarded={{ POINTS: 10, TIMESTAMP: 1000 }} onCredit={() => {}} />
    )
    expect(container.querySelector('.anim-credit-control-locked')).not.toBeNull()
    expect(screen.getByText('+10 pts')).toBeInTheDocument()
  })

  // ⚠️ LE cas de test que le plan nomme explicitement (R1) : un refus à 0
  // point DOIT verrouiller exactement comme un crédit positif.
  it('verrouillé MÊME à montant nul (awarded.POINTS === 0) — le refus verrouille comme un crédit', () => {
    const { container } = render(
      <AnimCreditControl team="Les Rouges" amount={10} awarded={{ POINTS: 0, TIMESTAMP: 1000 }} onCredit={() => {}} />
    )
    expect(container.querySelector('.anim-credit-control-locked')).not.toBeNull()
  })

  it('affiche le montant réel crédité (awarded.POINTS), PAS le montant local "amount" — une autre tablette a pu créditer un montant différent', () => {
    render(<AnimCreditControl team="Les Rouges" amount={10} awarded={{ POINTS: 7, TIMESTAMP: 1000 }} onCredit={() => {}} />)
    expect(screen.getByText('+7 pts')).toBeInTheDocument()
    expect(screen.queryByText('+10 pts')).not.toBeInTheDocument()
  })

  it('affiche "0 pt" (pas "+0 pts") pour un refus verrouillé — origine du geste lisible dans le libellé', () => {
    render(<AnimCreditControl team="Les Rouges" amount={10} awarded={{ POINTS: 0, TIMESTAMP: 1000 }} onCredit={() => {}} />)
    expect(screen.getByText('0 pt')).toBeInTheDocument()
    expect(screen.queryByText('+0 pts')).not.toBeInTheDocument()
  })

  it('aucun bouton "+N pts" ni "0 pt" cliquable en état verrouillé (pas d\'élément <button>)', () => {
    const { container } = render(
      <AnimCreditControl team="Les Rouges" amount={10} awarded={{ POINTS: 10, TIMESTAMP: 1000 }} onCredit={() => {}} />
    )
    expect(container.querySelectorAll('button')).toHaveLength(0)
  })

  it('aucun geste émis en état verrouillé : onCredit jamais appelé, même en cliquant sur l\'élément racine', () => {
    const onCredit = vi.fn()
    const { container } = render(
      <AnimCreditControl team="Les Rouges" amount={10} awarded={{ POINTS: 10, TIMESTAMP: 1000 }} onCredit={onCredit} />
    )
    container.querySelector('.anim-credit-control').click()
    expect(onCredit).not.toHaveBeenCalled()
  })

  it('verrouillé même avec amount=null (ligne sans réponse déjà refusée)', () => {
    const { container } = render(
      <AnimCreditControl team="Les Rouges" amount={null} awarded={{ POINTS: 0, TIMESTAMP: 1000 }} onCredit={() => {}} />
    )
    expect(container.querySelector('.anim-credit-control-locked')).not.toBeNull()
    expect(screen.getByText('0 pt')).toBeInTheDocument()
  })
})
