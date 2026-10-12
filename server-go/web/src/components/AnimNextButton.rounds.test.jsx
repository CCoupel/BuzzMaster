import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import AnimNextButton from './AnimNextButton'

// #245 (lot V3-3) — bouton « suivant » de l'anim : entracte proposée, changement de manche, fin d'entracte (maquette (g‴)).
const base = { ID: '7', TYPE: 'QCM', QUESTION: 'Fleuve ?', POINTS: '3', TIME: '30', ROUND_ID: 'r1', ROUND_NAME: 'Finale', NEW_ROUND: false }

describe('AnimNextButton — manches et entractes (#245)', () => {
  it('question ordinaire : « À suivre » inchangé', () => {
    const { container } = render(<AnimNextButton phase="REVEALED" question={{ ID: '1' }} nextQuestion={base} onSelectNext={() => {}} />)
    expect(screen.getByText('Question suivante')).toBeTruthy()
    expect(container.querySelector('.anim-next-btn-newround')).toBeNull()
    expect(container.querySelector('.anim-next-btn-entracte')).toBeNull()
  })

  it('Critical — NEW_ROUND : « Manche suivante : <nom> » (orange)', () => {
    const { container } = render(<AnimNextButton phase="REVEALED" question={{ ID: '1' }} nextQuestion={{ ...base, NEW_ROUND: true }} onSelectNext={() => {}} />)
    expect(screen.getByText('Manche suivante : Finale')).toBeTruthy()
    expect(container.querySelector('.anim-next-btn-newround')).not.toBeNull()
  })

  it('NEW_ROUND vers « Sans manche » : libellé « Sans manche »', () => {
    render(<AnimNextButton phase="REVEALED" question={{ ID: '1' }} nextQuestion={{ ...base, NEW_ROUND: true, ROUND_ID: '', ROUND_NAME: '' }} onSelectNext={() => {}} />)
    expect(screen.getByText('Manche suivante : Sans manche')).toBeTruthy()
  })

  it('Critical — une ENTRACTE est proposée : « Lancer l’entracte » (violet), sélection par READY', () => {
    const onSelectNext = vi.fn()
    const { container } = render(<AnimNextButton phase="REVEALED" question={{ ID: '1' }} nextQuestion={{ ...base, ID: '86', TYPE: 'ENTRACTE', QUESTION: 'Début de la manche Finale', NEW_ROUND: true }} onSelectNext={onSelectNext} />)
    expect(screen.getByText("Lancer l'entracte")).toBeTruthy()
    expect(screen.getByText('🎬 Début de la manche Finale')).toBeTruthy()
    expect(container.querySelector('.anim-next-btn-entracte')).not.toBeNull()
    fireEvent.click(screen.getByRole('button'))
    expect(onSelectNext).toHaveBeenCalledWith('86')
  })

  it('Critical — pendant l’entracte : « Terminer l’entracte » remplace le bouton et appelle onEndEntracte (jamais d’ACTIVE:true)', () => {
    const onEndEntracte = vi.fn()
    const onSelectNext = vi.fn()
    render(<AnimNextButton phase="STARTED" question={{ ID: '86', QUESTION: 'Début de la manche Finale' }} nextQuestion={base} onSelectNext={onSelectNext} entracteActive onEndEntracte={onEndEntracte} />)
    const btn = screen.getByRole('button', { name: /Terminer l'entracte/ })
    expect(btn.disabled).toBe(false)
    fireEvent.click(btn)
    expect(onEndEntracte).toHaveBeenCalledTimes(1)
    expect(onSelectNext).not.toHaveBeenCalled()
    expect(screen.queryByText('Question suivante')).toBeNull()
  })

  it('bouton inerte (phase en cours) : aucune variante colorée', () => {
    const { container } = render(<AnimNextButton phase="STARTED" question={{ ID: '1' }} nextQuestion={{ ...base, NEW_ROUND: true }} onSelectNext={() => {}} />)
    expect(container.querySelector('.anim-next-btn-newround')).toBeNull()
  })
})
