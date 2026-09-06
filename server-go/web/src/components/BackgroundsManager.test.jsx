/**
 * Tests — BackgroundsManager (#215, milestone v9.0.0).
 *
 * NOUVEAU composant, AUCUNE couverture préexistante : les deux zones
 * historiques de QuestionsPage.jsx qu'il remplace (Ambiance pendant le jeu,
 * Écran d'accueil Nouvelle Partie) — quasi identiques, l'une portant même
 * un commentaire indiquant qu'elle reproduisait l'autre — n'avaient jamais
 * eu de test dédié. Ce fichier verrouille le comportement du composant
 * mutualisé : les DEUX destinations (`background` / `new-game-backgrounds`)
 * doivent router vers leur endpoint respectif — c'est précisément le risque
 * qu'introduit la mutualisation (un bug de paramétrage ferait fuir les
 * requêtes d'une zone vers l'endpoint de l'autre).
 */
import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import BackgroundsManager from './BackgroundsManager'

vi.mock('./Button', () => ({
  default: ({ children, onClick, disabled, loading, as, ...rest }) => (
    <button onClick={onClick} disabled={disabled} {...rest}>{children}</button>
  ),
}))

vi.mock('./Card', () => ({
  default: ({ children, className, padding, variant, ...rest }) => (
    <div className={className} {...rest}>{children}</div>
  ),
  CardHeader: ({ children }) => <div className="card-header">{children}</div>,
  CardBody: ({ children }) => <div className="card-body">{children}</div>,
}))

const baseProps = {
  title: 'Ambiance — pendant le jeu',
  hint: 'Images affichees en boucle sur l\'ecran TV pendant le jeu.',
  emptyLabel: 'Aucune image de fond',
}

beforeEach(() => {
  vi.clearAllMocks()
  global.fetch = vi.fn().mockResolvedValue({ ok: true, text: async () => '' })
  vi.spyOn(window, 'confirm').mockReturnValue(true)
  // window.location.reload is called on successful upload — stub it so
  // jsdom doesn't throw "Not implemented: navigation".
  delete window.location
  window.location = { reload: vi.fn() }
})

afterEach(() => {
  vi.restoreAllMocks()
})

describe('BackgroundsManager — état vide', () => {
  it("affiche emptyLabel quand backgrounds est vide/absent", () => {
    render(<BackgroundsManager destination="background" backgrounds={[]} {...baseProps} />)
    expect(screen.getByText('Aucune image de fond')).toBeInTheDocument()
  })

  it('"Tout supprimer" est absent quand backgrounds est vide', () => {
    render(<BackgroundsManager destination="background" backgrounds={[]} {...baseProps} />)
    expect(screen.queryByText('Tout supprimer')).toBeNull()
  })
})

describe('BackgroundsManager — routage par destination (le risque de la mutualisation)', () => {
  it('destination="background" — upload POST vers /background', async () => {
    const { container } = render(<BackgroundsManager destination="background" backgrounds={[]} {...baseProps} />)
    const fileInput = container.querySelector('input[type="file"]')
    const file = new File(['x'], 'fond.jpg', { type: 'image/jpeg' })
    fireEvent.change(fileInput, { target: { files: [file] } })

    await waitFor(() => expect(global.fetch).toHaveBeenCalled())
    const [url, opts] = global.fetch.mock.calls[0]
    expect(url).toBe('/background')
    expect(opts.method).toBe('POST')
  })

  it('destination="new-game-backgrounds" — upload POST vers /new-game-backgrounds (PAS /background)', async () => {
    const { container } = render(<BackgroundsManager destination="new-game-backgrounds" backgrounds={[]} {...baseProps} />)
    const fileInput = container.querySelector('input[type="file"]')
    const file = new File(['x'], 'fond.jpg', { type: 'image/jpeg' })
    fireEvent.change(fileInput, { target: { files: [file] } })

    await waitFor(() => expect(global.fetch).toHaveBeenCalled())
    const [url] = global.fetch.mock.calls[0]
    expect(url).toBe('/new-game-backgrounds')
  })

  it('"Tout supprimer" envoie DELETE vers l\'endpoint de LA destination configurée', async () => {
    render(<BackgroundsManager
      destination="new-game-backgrounds"
      backgrounds={[{ path: '/files/bg1.jpg', duration: 10, opacity: 100 }]}
      {...baseProps}
    />)

    fireEvent.click(screen.getByText('Tout supprimer'))

    await waitFor(() => expect(global.fetch).toHaveBeenCalled())
    const [url, opts] = global.fetch.mock.calls[0]
    expect(url).toBe('/new-game-backgrounds')
    expect(opts.method).toBe('DELETE')
  })
})

describe('BackgroundsManager — suppression individuelle (confirmation requise)', () => {
  it('clic sur "Supprimer" (croix) sans confirmation → aucune requête réseau', async () => {
    window.confirm.mockReturnValue(false)
    render(<BackgroundsManager
      destination="background"
      backgrounds={[{ path: '/files/bg1.jpg', duration: 10, opacity: 100 }]}
      {...baseProps}
    />)

    fireEvent.click(screen.getByTitle('Supprimer'))

    expect(global.fetch).not.toHaveBeenCalled()
  })

  it('clic sur "Supprimer" avec confirmation → DELETE avec le nom de fichier en query param', async () => {
    render(<BackgroundsManager
      destination="background"
      backgrounds={[{ path: '/files/bg1.jpg', duration: 10, opacity: 100 }]}
      {...baseProps}
    />)

    fireEvent.click(screen.getByTitle('Supprimer'))

    await waitFor(() => expect(global.fetch).toHaveBeenCalled())
    const [url, opts] = global.fetch.mock.calls[0]
    expect(url).toBe('/background?file=bg1.jpg')
    expect(opts.method).toBe('DELETE')
  })
})

describe('BackgroundsManager — édition durée/opacité (PUT de la liste complète)', () => {
  it('changer la durée envoie un PUT avec la liste mise à jour, vers le bon endpoint', async () => {
    render(<BackgroundsManager
      destination="background"
      backgrounds={[{ path: '/files/bg1.jpg', duration: 10, opacity: 100 }]}
      {...baseProps}
    />)

    const durationInput = document.querySelector('.duration-input')
    fireEvent.change(durationInput, { target: { value: '25' } })

    await waitFor(() => expect(global.fetch).toHaveBeenCalled())
    const [url, opts] = global.fetch.mock.calls[0]
    expect(url).toBe('/background')
    expect(opts.method).toBe('PUT')
    const body = JSON.parse(opts.body)
    expect(body[0].duration).toBe(25)
  })
})
