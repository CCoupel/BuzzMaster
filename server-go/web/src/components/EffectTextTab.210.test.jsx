import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import EffectTextTab from './EffectTextTab'

// #210 (v11.8.0, LOT-2, F2.2) — onglet « Texte ». Source : contracts/light-effects.md §11.8 (deux onglets, une seule source ;
// avertissement « commentaires »), maquette effect_editor §03. Le navigateur n'analyse pas le YAML : erreurs = réponse du serveur.

const mount = (props = {}) => {
  const onChange = vi.fn()
  render(<EffectTextTab text={'id: mon-effet\n'} onChange={onChange} {...props} />)
  return onChange
}

describe('onglet Texte', () => {
  it('le champ montre le fichier tel quel ; la saisie est remontée', () => {
    const onChange = mount()
    const area = screen.getByLabelText("Fichier de l'effet (YAML)")
    expect(area).toHaveValue('id: mon-effet\n')
    fireEvent.change(area, { target: { value: 'id: autre\n' } })
    expect(onChange).toHaveBeenCalledWith('id: autre\n')
  })

  it('lecture seule : le champ ne se modifie pas, aucun avertissement de commentaires', () => {
    mount({ readOnly: true, hasComments: true })
    expect(screen.getByLabelText("Fichier de l'effet (YAML)")).toHaveAttribute('readonly')
    expect(screen.queryByText(/contient des commentaires/)).toBeNull()
  })

  it('erreurs du serveur : « Ligne N » + cause, annoncées (role=alert)', () => {
    mount({ errors: [{ line: 12, message: 'la variable « gB » n\'est pas déclarée dans variables' }, { line: 0, message: 'général' }] })
    const alert = screen.getByRole('alert')
    expect(alert).toHaveTextContent('Ligne 12')
    expect(alert).toHaveTextContent('« gB »')
    expect(alert).toHaveTextContent('général')
    expect(screen.queryByText('Le texte est valide.')).toBeNull()
  })

  it('texte valide : confirmation ; en attente de réponse : « Vérification du texte… »', () => {
    const { unmount } = render(<EffectTextTab text="x" onChange={vi.fn()} />)
    expect(screen.getByText('Le texte est valide.')).toBeInTheDocument()
    unmount()
    render(<EffectTextTab text="x" onChange={vi.fn()} pending />)
    expect(screen.getByRole('status')).toHaveTextContent(/Vérification/)
  })

  it('avertissement quand le fichier contient des commentaires : conservés depuis l\'onglet Texte, perdus par la grille', () => {
    mount({ hasComments: true })
    const w = screen.getByText(/contient des commentaires/).parentElement
    expect(w).toHaveTextContent(/conservés tant que vous enregistrez depuis l'onglet Texte/)
    expect(w).toHaveTextContent(/commentaires sont alors perdus/)
  })

  it('sans commentaire : pas d\'avertissement', () => {
    mount({ hasComments: false })
    expect(screen.queryByText(/contient des commentaires/)).toBeNull()
  })

  it('l\'aide rappelle que les voisins restent permis dans le texte', () => {
    mount()
    expect(screen.getByText(/y compris les voisins/)).toBeInTheDocument()
  })
})
