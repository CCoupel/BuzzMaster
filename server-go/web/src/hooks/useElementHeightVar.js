import { useEffect, useRef } from 'react'

/**
 * useElementHeightVar — mesure la hauteur RÉELLE d'un élément et la partage
 * via une variable CSS globale (`document.documentElement`), pour que
 * d'autres éléments (souvent hors de portée du composant qui la mesure)
 * puissent en dériver leur propre mise en page sans dupliquer de constante.
 *
 * Extrait de `RegieMessageBar.jsx` (#177/F1) — patron identique, désormais
 * partagé avec `Navbar.jsx` (#179/F3) : deux consommateurs, donc extraction
 * en pièce partagée plutôt qu'un copier-coller qui aurait fini par diverger
 * (voir `_work/reports/plan-20260818-212304.md` §"Mutualisation").
 * **Aucun changement de comportement** par rapport au code d'origine.
 *
 * - Mesure initiale via `getBoundingClientRect().height`, puis suivi par
 *   `ResizeObserver` (`borderBoxSize` quand disponible, repli sur
 *   `contentRect.height`).
 * - N'écrit la variable CSS que si la valeur **arrondie au pixel supérieur**
 *   (`Math.ceil`, #180) change — évite des invalidations de layout à chaque
 *   fraction de pixel rapportée par `ResizeObserver`. `Math.ceil` plutôt que
 *   `Math.round` (#177/#179 d'origine) : un arrondi standard peut SOUS-estimer
 *   la hauteur réelle (ex. 44.3px → 44px), donc réserver un peu moins que la
 *   place réellement occupée — suffisant pour déclencher une scrollbar selon
 *   le zoom/la résolution. `Math.ceil` garantit une réservation TOUJOURS
 *   ≥ la hauteur réelle.
 * - Nettoyage au démontage : `disconnect()` de l'observer **et** remise de
 *   la variable à `0px` — aucune réservation résiduelle si l'élément mesuré
 *   n'est plus monté (ex. navigation vers une route plein écran).
 *
 * @param {import('react').RefObject<HTMLElement>} ref - élément à mesurer.
 * @param {string} varName - nom de la variable CSS à écrire (ex. `--navbar-h`).
 */
export default function useElementHeightVar(ref, varName) {
  // Dernière hauteur (arrondie, px) effectivement écrite dans la variable
  // CSS — évite de réécrire à chaque pixel/fraction rapportés par
  // ResizeObserver (invalidations de layout inutiles).
  const lastHeightRef = useRef(null)

  useEffect(() => {
    const el = ref.current
    if (!el) return

    const applyHeight = (height) => {
      // #180 — Math.ceil, pas Math.round : la réservation d'espace doit
      // toujours être >= la hauteur réelle, jamais en dessous.
      const rounded = Math.ceil(height)
      if (rounded === lastHeightRef.current) return
      lastHeightRef.current = rounded
      document.documentElement.style.setProperty(varName, `${rounded}px`)
    }

    applyHeight(el.getBoundingClientRect().height)

    const observer = new ResizeObserver((entries) => {
      const entry = entries[0]
      if (!entry) return
      const height = entry.borderBoxSize?.[0]?.blockSize ?? entry.contentRect.height
      applyHeight(height)
    })
    observer.observe(el)

    return () => {
      observer.disconnect()
      lastHeightRef.current = null
      document.documentElement.style.setProperty(varName, '0px')
    }
  }, [ref, varName])
}
