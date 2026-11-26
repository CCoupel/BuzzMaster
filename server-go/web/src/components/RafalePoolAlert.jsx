import PoolAlert from './PoolAlert'

/**
 * RafalePoolAlert — alerte de disponibilité du Pool Speedy (contrat rafale.md §7.2), partagée entre QuestionsPage et
 * GamePage. Enveloppe de `PoolAlert` sur l'instance `rafale` (v13.1.0) : mêmes props, même rendu, même appel
 * `GET /api/rafale/pool`.
 */
export default function RafalePoolAlert(props) {
  return <PoolAlert instance="rafale" {...props} />
}
