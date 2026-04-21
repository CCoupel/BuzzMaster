# Scénarios E2E — Feature Milestone v3.6.0 : Page Mises à Jour

## Prérequis

- Serveur BuzzControl démarré sur http://localhost:80
- Chrome avec MCP claude-in-chrome disponible
- Connexion Internet disponible (pour les tests de fetch GitHub)
- Ou simuler une erreur réseau via les DevTools (Network throttling → Offline)

---

## Scénario 1 : Navigation vers la page Mises à Jour

### Contexte
Vérifier que la page `/admin/updates` est accessible et s'affiche correctement.

### Étapes
1. Ouvrir http://localhost/admin/game dans Chrome
2. Cliquer sur le menu "🐝" (menu abeille)
3. Cliquer sur "Mises à jour"
4. Vérifier l'URL dans la barre d'adresse

### Résultat attendu
- L'URL est `http://localhost/admin/updates`
- La page affiche le titre "Mises à jour"
- Le bouton "Actualiser" est présent

### Vérification Chrome
```
Vérifier URL: /admin/updates
Vérifier présent: h1 contenant "Mises à jour"
Vérifier présent: .btn-refresh
```

---

## Scénario 2 : Présence du bouton "Télécharger" pour les versions non installées

### Contexte
Vérifier que les versions disponibles différentes de la version actuelle affichent un bouton "Télécharger".

### Étapes
1. Ouvrir http://localhost/admin/updates dans Chrome
2. Attendre le chargement de la liste des versions (spinner disparaît)
3. Observer les lignes de version disponibles

### Résultat attendu
- La liste des versions s'affiche (`.versions-table`)
- Pour les versions qui ne sont PAS la version actuelle : bouton "Télécharger" visible
- La version actuelle affiche le badge "Actuelle" (et pas de bouton "Télécharger")

### Vérification Chrome
```
Attendre absent: .loading-spinner
Vérifier présent: .versions-table
Vérifier présent: .btn-download (au moins un)
Vérifier présent: .badge-current
Vérifier absent: .btn-download dans la ligne .current
```

---

## Scénario 3 : Affichage d'un message d'erreur en cas d'échec réseau

### Contexte
Simuler une panne réseau et vérifier qu'un message d'erreur clair est affiché.

### Étapes
1. Ouvrir les DevTools Chrome (F12)
2. Aller dans l'onglet "Network"
3. Sélectionner "Offline" dans le menu déroulant de throttling
4. Ouvrir http://localhost/admin/updates dans Chrome (ou cliquer sur "Actualiser")
5. Observer le message d'erreur

### Résultat attendu
- Un bloc d'erreur est affiché : `.update-error`
- Le texte commence par "Erreur :"
- La page ne crash pas (pas d'écran blanc)

### Vérification Chrome
```
(Activer mode Offline dans DevTools Network)
Cliquer: .btn-refresh
Attendre présent: .update-error
Vérifier texte contient: "Erreur"
```

```javascript
// Alternative : simuler l'erreur via fetch mock dans la console
const originalFetch = window.fetch
window.fetch = () => Promise.reject(new Error('Network error'))
// Cliquer sur Actualiser → observer l'erreur
window.fetch = originalFetch // Restaurer
```

---

## Scénario 4 : Bouton "Appliquer" absent si aucun téléchargement complété

### Contexte
Vérifier que le bouton "Appliquer" n'est jamais visible si aucun téléchargement n'a été effectué.
C'est le comportement attendu après le fix issue #44 (bouton désactivé pendant `loading`).

### Étapes
1. Ouvrir http://localhost/admin/updates dans Chrome (page fraîche, sans téléchargement précédent)
2. Observer toutes les lignes de version disponibles
3. Vérifier l'absence du bouton "Appliquer"

### Résultat attendu
- Aucun bouton "Appliquer" (`.btn-apply`) n'est visible
- Chaque version non-installée affiche uniquement "Télécharger"

### Vérification Chrome
```
Vérifier absent: .btn-apply
Vérifier présent: .btn-download (pour versions non-actuelles)
```

---

## Scénario 5 : Bouton "Appliquer" désactivé pendant un chargement en cours

### Contexte
Vérifier que le bouton "Appliquer" (et "Actualiser") est désactivé pendant qu'une opération est en cours.
C'est le fix de l'issue #44 : le bouton ne doit pas permettre de double-clic.

### Étapes
1. Ouvrir http://localhost/admin/updates dans Chrome
2. Attendre le chargement initial
3. Cliquer sur "Télécharger" pour une version disponible
4. PENDANT le téléchargement (spinner visible) : observer l'état du bouton "Actualiser"

### Résultat attendu
- Pendant le téléchargement : `.btn-refresh` est désactivé (`disabled`)
- L'indicateur de téléchargement (`.download-progress`) est visible
- Après completion : bouton "Appliquer" apparaît

### Vérification Chrome
```javascript
// Exécuter dans la console pendant le téléchargement
document.querySelector('.btn-refresh').disabled // → true
document.querySelector('.download-progress')   // → visible
```

---

## Scénario 6 : Modal de confirmation si partie en cours avant d'appliquer

### Contexte
Vérifier que si une partie est en cours, une confirmation est demandée avant d'appliquer la mise à jour.

### Prérequis supplémentaires
- Avoir téléchargé une version (bouton "Appliquer" visible)
- Une partie doit être en cours (phase ≠ STOP) — démarrer une partie dans l'onglet Jeu

### Étapes
1. Démarrer une partie dans l'onglet `/admin/game`
2. Ouvrir `/admin/updates` dans un autre onglet
3. Cliquer sur "Appliquer" pour la version téléchargée

### Résultat attendu
- Une modale de confirmation apparaît : `.confirm-modal`
- Titre : "⚠️ Partie en cours"
- Boutons : "Annuler" et "Appliquer quand même"
- Cliquer "Annuler" → modale disparaît, pas de redémarrage
- Cliquer "Appliquer quand même" → redémarrage lancé

### Vérification Chrome
```
Vérifier présent: .confirm-modal
Vérifier texte contient: "Partie en cours"
Cliquer: .btn-cancel
Vérifier absent: .confirm-modal
```

---

## Scénario 7 : Notes de version expansibles

### Contexte
Vérifier que les notes de version peuvent être affichées/masquées via le bouton chevron.

### Étapes
1. Ouvrir http://localhost/admin/updates dans Chrome
2. Trouver une version avec des notes (bouton "Notes ▶" visible)
3. Cliquer sur le bouton "Notes ▶"
4. Observer les notes de version

### Résultat attendu
- Les notes s'affichent (`.version-notes-expanded`)
- Le chevron pointe vers le bas (`.chevron.expanded`)
- Cliquer à nouveau sur "Notes ▼" masque les notes

### Vérification Chrome
```
Cliquer: .btn-toggle-notes (première occurrence)
Attendre présent: .version-notes-expanded
Vérifier présent: .chevron.expanded
Cliquer: .btn-toggle-notes (même bouton)
Attendre absent: .version-notes-expanded
```
