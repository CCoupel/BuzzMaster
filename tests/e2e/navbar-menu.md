# E2E Tests - Navbar Menu Feature (v2.48.0)

## Prérequis
- Serveur BuzzControl lancé sur `http://localhost` (port HTTP)
- Page admin accessible : `http://localhost/admin`
- Navigateur Chrome avec MCP claude-in-chrome actif

---

## Scénario 1 : Menu s'ouvre au clic sur le logo BuzzControl

### Étapes
1. Naviguer vers `http://localhost/admin`
2. Attendre le chargement complet de la navbar
3. Localiser le bouton logo BuzzControl à gauche avec indicateur `▼`
4. Cliquer sur le bouton logo BuzzControl
5. Vérifier que le menu déroulant s'affiche avec les options Config et Logs

### Vérifications
- Élément `.navbar-menu-dropdown` est visible
- Contient deux items : `⚙️ Config` et `📋 Logs`
- Animation slideDown s'exécute

### Acceptation
✅ Menu s'affiche immédiatement après le clic

---

## Scénario 2 : Menu se ferme au clic sur un item (Config)

### Étapes
1. Depuis le scénario 1, menu déjà ouvert
2. Cliquer sur `⚙️ Config` dans le menu
3. Attendre la navigation vers la page Config
4. Vérifier que le menu s'est fermé
5. Vérifier que la page Config est chargée (`/admin/settings`)

### Vérifications
- Menu `.navbar-menu-dropdown` disparaît du DOM
- URL change vers `/admin/settings`
- Page Config contient ses éléments (formulaires, paramètres, etc.)
- Bouton logo BuzzControl n'est plus "actif" (visuelle)

### Acceptation
✅ Navigation vers Config et fermeture du menu sont synchrones

---

## Scénario 3 : Menu se ferme au clic extérieur

### Étapes
1. Depuis la page Admin
2. Cliquer sur le bouton logo BuzzControl → Menu s'ouvre
3. Cliquer en dehors du menu (ex: sur le titre "BuzzControl" ou dans la zone des liens de jeu)
4. Vérifier que le menu se ferme sans navigation

### Vérifications
- Menu `.navbar-menu-dropdown` disparaît
- URL reste `/admin` (pas de navigation)
- Aucune erreur dans la console
- Autre élément cliqué reste dans son état normal

### Acceptation
✅ Menu se ferme au clic extérieur sans effet de bord

---

## Scénario 4 : Config et Logs n'apparaissent pas dans la navbar principale

### Étapes
1. Naviguer vers `http://localhost/admin`
2. Examiner la navbar complète
3. Rechercher les textes "Config" et "Logs" dans la section "navbar-links"

### Vérifications
- Zone "Jeu" contient : Jeu, Scores, Palmarès, Historique
- Zone "Config" contient : Joueurs, Questions (SANS Config et Logs)
- Config et Logs UNIQUEMENT dans le menu déroulant

### Acceptation
✅ Config et Logs sont retirés de la navbar principale et uniquement dans le menu

---

## Scénario 5 : Menu s'ouvre/ferme en cliquant sur le logo BuzzControl plusieurs fois

### Étapes
1. Page Admin chargée
2. Cliquer sur le logo BuzzControl → Menu s'ouvre
3. Cliquer sur le logo BuzzControl → Menu se ferme
4. Cliquer sur le logo BuzzControl → Menu s'ouvre
5. Cliquer sur le logo BuzzControl → Menu se ferme
6. Répéter 2-3 fois

### Vérifications
- Menu s'ouvre et ferme correctement à chaque clic
- Pas de lag ou délai inhabité
- Animations sont fluides
- État du composant reste cohérent

### Acceptation
✅ Toggle menu fonctionne correctement

---

## Scénario 6 : Menu se ferme au clic sur l'item "Logs"

### Étapes
1. Page Admin, menu ouvert
2. Cliquer sur `📋 Logs`
3. Attendre la navigation vers `/admin/logs`
4. Vérifier que le menu s'est fermé

### Vérifications
- Menu disparaît du DOM
- URL change vers `/admin/logs`
- Page Logs se charge avec ses contenus (tableau de logs, filtres, etc.)

### Acceptation
✅ Logs fonctionne comme Config

---

## Scénario 7 : Vérifier l'accessibilité - Attributs ARIA

### Étapes
1. Inspecter le bouton logo BuzzControl
2. Vérifier la présence d'attributs ARIA

### Vérifications
- `aria-label="Menu de navigation"` est présent
- `title="Menu"` est présent
- Bouton a `role="button"` implicite (ou explicite si <button>)

### Acceptation
✅ Attributs accessibilité présents

---

## Scénario 8 : Responsive - Menu sur petit écran

### Étapes
1. Redimensionner la fenêtre à 600px (mobile)
2. Naviguer vers `/admin`
3. Cliquer sur le logo BuzzControl
4. Vérifier que le menu s'affiche correctement

### Vérifications
- Menu reste visible et utilisable
- Positionnement correct (pas hors écran)
- Texte lisible même sur petit écran
- Items du menu sont cliquables

### Acceptation
✅ Menu responsive sur petits écrans

---

## Notes de Validation

### Points clés
- Menu déroulant est une animation CSS (slideDown)
- Fermeture au clic extérieur utilise `useRef` + `useEffect` + `document.addEventListener`
- Items du menu sont `<NavLink>` de React Router
- Pas de rechargement de page (SPA)

### Considérations
- Le menu peut être ouvert dans plusieurs pages (/admin, /anim)
- La pastille de connexion (navbar-status) doit rester intacte
- Les raccourcis clavier (ESC pour fermer) pourraient être ajoutés en v2.49.0

### Points à déboguer si problèmes
- Console pour erreurs React
- Network tab pour vérifier les routes
- React DevTools pour inspecter l'état `isMenuOpen`
