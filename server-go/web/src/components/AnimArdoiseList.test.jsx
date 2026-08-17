import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import AnimArdoiseList from './AnimArdoiseList'

// ---------------------------------------------------------------------------
// AnimArdoiseList — liste ARDOISE de la colonne équipes /anim (#158/F2, T3).
//
// Plan : _work/reports/plan-20260816-125123.md §6. Monte AnimCreditControl
// (#170) TEL QUEL pour le crédit et le refus (« 0 pt ») — ce composant ne
// réimplémente RIEN de la règle de verrouillage, il calcule seulement le
// montant "+N pts" (calcArdoiseDefaultPoints, mirror exact de /admin,
// GamePage.jsx) et décide QUAND monter AnimCreditControl :
// `showCredit = revealed || awarded` — même règle stricte que /admin pour
// ARDOISE (crédit gated sur REVEALED uniquement, contrairement à SPEEDY/QCM
// qui l'autorisent dès STOPPED), et une ligne déjà créditée reste affichée
// verrouillée même si `revealed` redevient faux entre-temps (persiste
// jusqu'au changement de question, awardedTeams ne se vide pas seul).
//
// Trois états de LIGNE (visibles une fois `revealed` vrai, ou pour une
// ligne déjà créditée) : a répondu/pas encore créditée (« à traiter ») ·
// créditée (« traitée », montant 0 compris) · sans réponse.
// ---------------------------------------------------------------------------

function makeEntry(teamName, { color = [255, 0, 0], answer = null } = {}) {
  return { team: { NAME: teamName, COLOR: color }, teamName, answer }
}

describe('AnimArdoiseList — avant REVEALED (et équipe non créditée)', () => {
  it("ne monte AUCUN AnimCreditControl tant que revealed est faux et qu'aucune équipe n'est créditée", () => {
    const entries = [
      makeEntry('Les Rouges', { answer: { TEXT: 'Réponse A', STARTED_AT: 12_000_000 } }),
      makeEntry('Les Bleus', { answer: null }),
    ]
    const { container } = render(
      <AnimArdoiseList
        entries={entries}
        question={{ POINTS: '5' }}
        gameTime={10_000_000}
        creditPoints={1}
        revealed={false}
        awardedTeams={{}}
        onCredit={() => {}}
      />
    )
    expect(container.querySelectorAll('.anim-credit-control')).toHaveLength(0)
  })

  it('affiche quand même le rang, le délai et le texte de la copie avant REVEALED (lecture en direct)', () => {
    const entries = [makeEntry('Les Rouges', { answer: { TEXT: 'Réponse A', STARTED_AT: 12_000_000 } })]
    render(
      <AnimArdoiseList
        entries={entries}
        question={{ POINTS: '5' }}
        gameTime={10_000_000}
        creditPoints={1}
        revealed={false}
        awardedTeams={{}}
        onCredit={() => {}}
      />
    )
    expect(screen.getByText('1')).toBeInTheDocument() // rang
    expect(screen.getByText('2.000 s')).toBeInTheDocument() // délai
    expect(screen.getByText('Réponse A')).toBeInTheDocument()
  })

  it("monte AnimCreditControl (verrouillé) pour une équipe déjà créditée MÊME si revealed est faux (persiste jusqu'au changement de question)", () => {
    const entries = [makeEntry('Les Rouges', { answer: { TEXT: 'Réponse A', STARTED_AT: 12_000_000 } })]
    const { container } = render(
      <AnimArdoiseList
        entries={entries}
        question={{ POINTS: '5' }}
        gameTime={10_000_000}
        creditPoints={1}
        revealed={false}
        awardedTeams={{ 'Les Rouges': { POINTS: 5, TIMESTAMP: 1000 } }}
        onCredit={() => {}}
      />
    )
    expect(container.querySelector('.anim-credit-control-locked')).not.toBeNull()
  })
})

describe('AnimArdoiseList — en REVEALED, trois états de ligne', () => {
  it('« à traiter » : a répondu, pas encore créditée -> AnimCreditControl ouvert avec le montant calculé', () => {
    const entries = [makeEntry('Les Rouges', { answer: { TEXT: 'Réponse A', STARTED_AT: 12_000_000 } })]
    render(
      <AnimArdoiseList
        entries={entries}
        question={{ POINTS: '5' }}
        gameTime={10_000_000}
        creditPoints={1}
        revealed={true}
        awardedTeams={{}}
        onCredit={() => {}}
      />
    )
    expect(screen.getByText('+5 pts')).toBeInTheDocument()
    expect(screen.getByText('0 pt')).toBeInTheDocument()
  })

  it('« traitée » : équipe créditée -> AnimCreditControl verrouillé, montant du serveur affiché (0 compris)', () => {
    const entries = [makeEntry('Les Rouges', { answer: { TEXT: 'Réponse A', STARTED_AT: 12_000_000 } })]
    const { container } = render(
      <AnimArdoiseList
        entries={entries}
        question={{ POINTS: '5' }}
        gameTime={10_000_000}
        creditPoints={1}
        revealed={true}
        awardedTeams={{ 'Les Rouges': { POINTS: 0, TIMESTAMP: 1000 } }}
        onCredit={() => {}}
      />
    )
    expect(container.querySelector('.anim-credit-control-locked')).not.toBeNull()
    expect(screen.getByText('0 pt')).toBeInTheDocument()
    expect(container.querySelectorAll('.anim-credit-control-btn')).toHaveLength(0)
  })

  it('« sans réponse » : pas de rang ni délai, seul "0 pt" est proposé (amount=null)', () => {
    const entries = [makeEntry('Les Verts', { answer: null })]
    const { container } = render(
      <AnimArdoiseList
        entries={entries}
        question={{ POINTS: '5' }}
        gameTime={10_000_000}
        creditPoints={1}
        revealed={true}
        awardedTeams={{}}
        onCredit={() => {}}
      />
    )
    expect(container.querySelector('.anim-ardoise-rank')).toBeNull()
    expect(container.querySelector('.anim-ardoise-delay')).toBeNull()
    expect(screen.getByText('0 pt')).toBeInTheDocument()
    expect(screen.queryByText(/^\+\d+ pts$/)).not.toBeInTheDocument()
  })

  it('« sans réponse » verrouillée (refus 0 pt déjà enregistré) : reste verrouillée, pas de bouton', () => {
    const entries = [makeEntry('Les Verts', { answer: null })]
    const { container } = render(
      <AnimArdoiseList
        entries={entries}
        question={{ POINTS: '5' }}
        gameTime={10_000_000}
        creditPoints={1}
        revealed={true}
        awardedTeams={{ 'Les Verts': { POINTS: 0, TIMESTAMP: 1000 } }}
        onCredit={() => {}}
      />
    )
    expect(container.querySelector('.anim-credit-control-locked')).not.toBeNull()
    expect(container.querySelectorAll('.anim-credit-control-btn')).toHaveLength(0)
  })
})

describe('AnimArdoiseList — AnimCreditControl monté tel quel, pas réimplémenté', () => {
  it('appelle onCredit(teamName, points) pour "+N pts" et "0 pt" — même chemin de crédit', () => {
    const onCredit = vi.fn()
    const entries = [makeEntry('Les Rouges', { answer: { TEXT: 'Réponse A', STARTED_AT: 12_000_000 } })]
    render(
      <AnimArdoiseList
        entries={entries}
        question={{ POINTS: '5' }}
        gameTime={10_000_000}
        creditPoints={1}
        revealed={true}
        awardedTeams={{}}
        onCredit={onCredit}
      />
    )
    screen.getByText('+5 pts').click()
    expect(onCredit).toHaveBeenCalledWith('Les Rouges', 5)

    screen.getByText('0 pt').click()
    expect(onCredit).toHaveBeenCalledWith('Les Rouges', 0)
  })

  it('le bouton "+N pts" est bien un descendant de .anim-credit-control (AnimCreditControl monté, pas un bouton "maison" au même niveau)', () => {
    const entries = [makeEntry('Les Rouges', { answer: { TEXT: 'Réponse A', STARTED_AT: 12_000_000 } })]
    render(
      <AnimArdoiseList
        entries={entries}
        question={{ POINTS: '5' }}
        gameTime={10_000_000}
        creditPoints={1}
        revealed={true}
        awardedTeams={{}}
        onCredit={() => {}}
      />
    )
    const btn = screen.getByText('+5 pts')
    expect(btn.closest('.anim-credit-control')).not.toBeNull()
    expect(btn.className).toMatch(/\banim-credit-control-btn-award\b/)
  })
})

describe('AnimArdoiseList — copie longue, non tronquée', () => {
  it("le texte complet d'une copie longue est présent dans le DOM (troncature = CSS seul, pas de coupure de contenu)", () => {
    const longText = 'Une réponse très longue qui dépasse largement la largeur habituelle attendue pour une ligne ARDOISE sur tablette, mot après mot, sans être coupée par le composant lui-même.'
    const entries = [makeEntry('Les Rouges', { answer: { TEXT: longText, STARTED_AT: 12_000_000 } })]
    render(
      <AnimArdoiseList
        entries={entries}
        question={{ POINTS: '5' }}
        gameTime={10_000_000}
        creditPoints={1}
        revealed={false}
        awardedTeams={{}}
        onCredit={() => {}}
      />
    )
    expect(screen.getByText(longText)).toBeInTheDocument()
  })
})

describe('AnimArdoiseList — ordre de rendu', () => {
  it('rend les lignes dans l\'ordre fourni par `entries` (déjà trié par sortedArdoiseEntries en amont)', () => {
    const entries = [
      makeEntry('Rapide', { answer: { TEXT: 'r', STARTED_AT: 11_000_000 } }),
      makeEntry('Lente', { answer: { TEXT: 'l', STARTED_AT: 15_000_000 } }),
      makeEntry('SansReponse', { answer: null }),
    ]
    const { container } = render(
      <AnimArdoiseList
        entries={entries}
        question={{ POINTS: '5' }}
        gameTime={10_000_000}
        creditPoints={1}
        revealed={false}
        awardedTeams={{}}
        onCredit={() => {}}
      />
    )
    const names = Array.from(container.querySelectorAll('.anim-ardoise-team-name')).map(el => el.textContent)
    expect(names).toEqual(['Rapide', 'Lente', 'SansReponse'])
  })
})
