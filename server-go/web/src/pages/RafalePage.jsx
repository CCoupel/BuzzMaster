import PoolPage from './PoolPage'

/**
 * RafalePage — « Pool Speedy » (`/admin/rafale`). Enveloppe mince de `PoolPage` sur l'instance `rafale` (v13.1.0,
 * contrat picto-pool.md §6) : écran, libellés et chemins inchangés. Le contenu historique (contrat rafale.md §9,
 * #197, #203) vit désormais dans PoolPage.jsx.
 */
export default function RafalePage() {
  return <PoolPage instance="rafale" />
}
