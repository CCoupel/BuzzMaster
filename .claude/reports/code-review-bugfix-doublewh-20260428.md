# Revue de Code — Bugfix double WS (commit 111d184)

**Date** : 2026-04-28  
**Commit** : `111d184` — `fix(#11): restrict GameProvider to admin routes — no double WS`  
**Fichier** : `server-go/web/src/App.jsx` uniquement  
**Mode** : general (focus qualité + correction complète)

---

## Verdict : ✅ APPROUVE

Correction minimale, chirurgicale, correcte. Toutes les vérifications demandées passent.

---

## Vérifications point par point

### 1. Routes admin ont bien `GameProvider` comme parent ✅

```jsx
if (isAdminRoute) {
  return (
    <GameProvider>
      <AdminContent />   // useGame() valide ici
    </GameProvider>
  )
}
```

`isAdminRoute = pathname.startsWith('/admin') || pathname.startsWith('/anim')` — même prédicats qu'avant. `AdminContent` (ex-`AppContent`) utilise `useGame()` pour alimenter `Navbar`. Toutes les routes admin (GamePage, QuestionsPage, ConfigPage, TeamsPage, etc.) restent dans `AdminContent` → aucune page admin ne perd l'accès à `useGame()`. ✅

### 2. Routes `/tv`, `/player`, `/` n'ont plus `GameProvider` ✅

```jsx
// Non-admin branch — aucun GameProvider dans l'arbre
return (
  <div className="app">
    <main className="main-content fullscreen">
      <Routes>
        <Route path="/" element={<EnrollPage />} />
        <Route path="/player" element={<VPlayerPage />} />
        <Route path="/tv" element={<PlayerDisplay />} />
      </Routes>
    </main>
  </div>
)
```

Pas de `GameProvider` → pas de connexion `/ws/admin` parasite. Chaque page gère sa propre connexion WS dédiée. ✅

### 3. Pas de régression sur `/` ✅

`/` → `<EnrollPage />` dans les deux branches (ancienne et nouvelle). `EnrollPage` utilise `useWebSocket('/ws/player')` (confirmé, cf. import grep). Comportement identique, sans GameContext. ✅

### 4. Pas de `useGame()` résiduel dans PlayerDisplay, VPlayerPage, EnrollPage ✅

Vérification directe sur la branche post-fix :
- `PlayerDisplay.jsx` : `useWebSocket('/ws/tv')` ✅
- `VPlayerPage.jsx` : `useWebSocket('/ws/player')` ✅
- `EnrollPage.jsx` : `useWebSocket('/ws/player')` ✅

Aucun import `useGame` / `GameContext` dans ces fichiers.

### 5. Build `npm run build` ✅

Confirmé par dev-frontend. L'usage de `useLocation()` dans `App` est valide — `App` est rendu dans `<BrowserRouter>` (comme l'était `AppContent` qui l'utilisait déjà).

---

## Qualité de l'implémentation

**CSS classes préservées** :
- `AdminContent` : `<main className="main-content">` (avec Navbar) ✅
- Non-admin branch : `<main className="main-content fullscreen">` (sans Navbar) ✅

Comportement visuel identique à avant pour chaque route.

**Séparation logique claire** : `AdminContent` porte le layout admin (Navbar + routes), la branche non-admin est purement standalone. Nommage `AdminContent` plus explicite que `AppContent`. ✅

**Pas de routes manquées** : Les 3 routes non-admin (`/`, `/player`, `/tv`) sont toutes couvertes. Les routes admin (`/admin/*`, `/anim/*`, `/admin/logs`, `/anim/logs`) restent dans `AdminContent`. ✅

---

## Observation (INFO)

`GameProvider` se remonte à chaque navigation admin→non-admin→admin (la condition `isAdminRoute` change → React démonte/remonte le composant). Cela entraîne une reconnexion `/ws/admin` à chaque retour sur une page admin depuis `/tv` ou `/player`. En pratique, admin et TV sont sur des appareils distincts, donc ce n'est pas un problème fonctionnel. Comportement attendu et correct.

---

## Verdict Final

```
[X] APPROUVE — Prêt pour merge / QA
[ ] APPROUVE AVEC RESERVES
[ ] REFUSE
```
