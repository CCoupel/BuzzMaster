import { useEffect } from 'react'

// #210 (v11.8.0, LOT-2) — « quitter avec des modifications non enregistrées demande confirmation » (critère d'acceptation).
// L'application utilise <BrowserRouter> (pas de routeur de données : `useBlocker` est indisponible) ; la garde couvre donc :
//   1. la fermeture / le rechargement de l'onglet (`beforeunload`, message du navigateur) ;
//   2. un clic sur un lien interne vers une AUTRE page (<a href>, <Link>) : `window.confirm`, refus = navigation annulée (écouteur en
//      capture sur le document : il passe avant celui du routeur) ;
//   3. le bouton Précédent / Suivant du navigateur : une entrée d'historique « garde » est ajoutée tant que `dirty` ; l'appui sur
//      Précédent revient sur la page, la confirmation est demandée, et un refus remet la garde en place.
// Une navigation programmatique (`navigate()`) n'est pas interceptée : aucune n'existe dans la page Ambiance.

export const UNSAVED_MESSAGE = 'Des modifications de l\'effet ne sont pas enregistrées. Quitter la page et les perdre ?'

export function useUnsavedGuard(dirty, message = UNSAVED_MESSAGE) {
  useEffect(() => {
    if (!dirty) return undefined

    const onBeforeUnload = (e) => { e.preventDefault(); e.returnValue = '' }

    const onClick = (e) => {
      if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return
      const a = e.target?.closest?.('a[href]')
      if (!a || (a.target && a.target !== '_self') || a.hasAttribute('download')) return
      let url
      try { url = new URL(a.href, window.location.href) } catch { return }
      if (url.origin !== window.location.origin) return
      if (url.pathname === window.location.pathname && url.search === window.location.search) return
      if (!window.confirm(message)) { e.preventDefault(); e.stopPropagation() }
    }

    // Entrée d'historique « garde » : Précédent revient sur la page elle-même.
    let leaving = false
    window.history.pushState({ __unsavedGuard: true }, '')
    const onPop = () => {
      if (leaving) return
      if (window.confirm(message)) { leaving = true; window.history.back() }
      else window.history.pushState({ __unsavedGuard: true }, '')
    }

    window.addEventListener('beforeunload', onBeforeUnload)
    document.addEventListener('click', onClick, true)
    window.addEventListener('popstate', onPop)
    return () => {
      window.removeEventListener('beforeunload', onBeforeUnload)
      document.removeEventListener('click', onClick, true)
      window.removeEventListener('popstate', onPop)
    }
  }, [dirty, message])
}
