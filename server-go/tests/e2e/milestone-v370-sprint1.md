# Scénarios E2E — Milestone v3.7.0 Sprint 1+2

## Prérequis

- Serveur BuzzControl v3.7.0 démarré sur http://localhost:80
- Chrome avec MCP claude-in-chrome disponible
- Données de test : au moins quelques questions avec des catégories variées

---

## Scénario 1 — #61 Boost des couleurs équipes

### Objectif
Vérifier que les couleurs d'équipe affichées dans l'interface admin et sur la TV
ne sont jamais pâles ou trop sombres (saturation min 70%, luminosité [40%–65%]).

### Étapes

1. Ouvrir http://localhost/admin/teams dans Chrome
2. Créer ou sélectionner une équipe avec couleur rouge vif (#FF0000)
3. Observer la couleur de la carte d'équipe dans `/admin/teams`
4. Ouvrir http://localhost/admin/game dans Chrome
5. Observer la couleur de la même équipe dans les TeamCards

### Résultats attendus

- La couleur affichée est bien saturée (rouge intense, pas rose pâle)
- Les couleurs grises ou très sombres sont rehaussées automatiquement
- Pas de dégradation visuelle ni couleur blanche/noire brute

### Vérification Chrome

```javascript
// Sur /admin/teams — vérifier que les TeamCards ont des couleurs visibles
const cards = document.querySelectorAll('[style*="rgb("]')
cards.forEach(card => {
  const style = card.getAttribute('style')
  // Ne doit pas contenir de gris pur (r==g==b)
  console.log('Color:', style)
})
```

### Cas limites à tester

- Équipe avec couleur grise (#808080) → doit être rehaussée
- Équipe avec couleur noire (#000000) → doit être rehaussée à lightness 40%
- Équipe avec couleur blanche (#FFFFFF) → doit être rehaussée à lightness 65%

---

## Scénario 2 — #43 Badge version cliquable dans la Navbar

### Objectif
Vérifier que cliquer sur le badge de version dans la Navbar navigue vers la page
des mises à jour.

### Étapes

1. Ouvrir http://localhost/admin/game dans Chrome
2. Repérer le badge version en haut à gauche (ex: `v3.7.0`)
3. Cliquer sur le badge
4. Observer la navigation

### Résultats attendus

- Le badge affiche `v3.7.0` (ou la version courante)
- Clic → navigation vers `/admin/updates`
- Si une mise à jour est disponible, un badge `!` orange est visible sur le badge version

### Vérification Chrome

```javascript
// Vérifier la présence du badge et son rôle cliquable
const badge = document.querySelector('.version-badge-clickable')
console.assert(badge !== null, 'Badge version doit exister')
console.assert(badge.getAttribute('role') === 'button', 'Badge doit avoir role=button')
console.assert(badge.getAttribute('tabindex') === '0', 'Badge doit être focusable')
```

### Test navigation

1. Ouvrir http://localhost/admin/teams dans Chrome
2. Cliquer badge version → URL doit devenir http://localhost/admin/updates
3. Ouvrir http://localhost/anim/game (si accessible) → clic badge → URL doit contenir `/anim/updates`

---

## Scénario 3 — #40 Filtres catégories dans GamePage

### Prérequis

- Au moins 3 questions avec des catégories différentes (GEOGRAPHY, SCIENCE, HISTORY par exemple)
- Ces questions doivent avoir le champ CATEGORY renseigné via le formulaire d'édition

### Étapes

1. Ouvrir http://localhost/admin/game dans Chrome
2. Observer la zone "Questions" en bas à gauche

#### Étape 3a — Vérification barre de filtres

3. Si des questions avec catégories existent, la barre de filtres catégories doit apparaître
4. Des pills (boutons ronds avec icône emoji) doivent être visibles, une par catégorie présente

#### Étape 3b — Activation d'un filtre

5. Cliquer sur la pill "Géographie" (🌍)
6. La liste de questions doit filtrer : seules les questions de catégorie GEOGRAPHY restent visibles
7. La pill active doit avoir un style visuel distinct (ex: bordure ou opacité modifiée via classe `active`)
8. Un bouton `×` doit apparaître pour réinitialiser

#### Étape 3c — Désactivation du filtre

9. Cliquer à nouveau sur la pill "Géographie" → le filtre se désactive
10. Toutes les questions sont à nouveau visibles
11. Le bouton `×` disparaît

#### Étape 3d — Réinitialisation

12. Activer un filtre
13. Cliquer sur le bouton `×`
14. Toutes les questions sont à nouveau visibles

### Résultats attendus

- Pills visibles seulement si des catégories existent dans le deck de questions
- Filtrage instantané sans rechargement
- Bouton reset visible uniquement quand un filtre est actif
- Message "Aucune question dans cette catégorie" si le filtre donne 0 résultat

### Vérification Chrome

```javascript
// Vérifier la barre de filtres
const bar = document.querySelector('.category-filter-bar')
console.assert(bar !== null, 'Barre de filtres doit exister si des catégories sont présentes')

// Vérifier les pills
const pills = document.querySelectorAll('.category-filter-pill')
console.log('Nombre de pills catégories:', pills.length)

// Vérifier le bouton reset (avant activation = absent)
const reset = document.querySelector('.category-filter-reset')
console.assert(reset === null, 'Reset button absent avant activation du filtre')
```

---

## Scénario 4 — #51 Double QR code TV enrollment

### Objectif
Vérifier que la vue TV affiche deux QR codes côte à côte lors de la phase
d'enrollment (enrôlement des joueurs virtuels).

### Prérequis

- Au moins un joueur virtuel configuré ou enrollment activé depuis l'admin

### Étapes

1. Ouvrir http://localhost/tv dans Chrome (onglet 1)
2. Ouvrir http://localhost/admin/game dans Chrome (onglet 2)
3. Dans l'onglet admin, activer l'enrollment (bouton "Joueurs" ou similaire)
4. Passer en vue ENROLL
5. Observer l'onglet TV

### Résultats attendus

- Deux QR codes affichés côte à côte
- QR code 1 : URL de la page joueur (ex: `http://192.168.x.x/player`)
- QR code 2 : URL alternative ou même URL selon le contexte
- Les deux QR codes sont scanables
- Pas de scroll sur la vue TV

### Vérification Chrome (onglet TV)

```javascript
// Vérifier la présence de 2 QR codes
const qrCodes = document.querySelectorAll('canvas, [data-qr], .qr-code')
console.log('Nombre de QR codes:', qrCodes.length)
console.assert(qrCodes.length >= 2, 'Deux QR codes doivent être présents')

// Vérifier absence de scroll
console.assert(
  document.body.scrollHeight <= window.innerHeight,
  'Pas de scroll sur la vue TV'
)
```

### URL QR code — vérification port (#63)

Le QR code doit inclure le port si différent de 80 :
- Port 80 : `http://192.168.1.100/player` (sans port)
- Port 8080 : `http://192.168.1.100:8080/player` (avec port)

```javascript
// Vérifier que window.location.host est utilisé (inclut le port si non-80)
console.log('Host actuel:', window.location.host)
// Exemple attendu: "192.168.1.100:8080" si port non-standard, "192.168.1.100" sinon
```

---

## Scénario 5 — #44 Noms de fichiers versionnés dans les backups

### Objectif
Vérifier que le téléchargement de backup inclut le numéro de version
dans le nom du fichier (`Content-Disposition`).

### Étapes

#### Test 5a — Full backup (fs-backup)

1. Ouvrir http://localhost/admin/backup dans Chrome
2. Cliquer sur le bouton "Télécharger backup complet" (ou accéder directement à http://localhost/fs-backup)
3. Observer la boîte de dialogue de téléchargement du navigateur

**Résultat attendu** : Le fichier proposé s'appelle `buzzcontrol-full-backup-v3.7.0-YYYYMMDD.tar` (contient `v3.7.0`)

#### Test 5b — Game backup

1. Cliquer sur "Télécharger backup jeu" (ou accéder à http://localhost/game-backup)

**Résultat attendu** : Le fichier s'appelle `buzzcontrol-game-backup-v3.7.0-YYYYMMDD.tar`

#### Test 5c — Backup sélectif

1. Cliquer sur "Télécharger backup sélectif" avec "Questions" coché
(ou accéder à http://localhost/backup-select?questions=true)

**Résultat attendu** : Le fichier s'appelle `buzzcontrol-backup-v3.7.0-YYYYMMDD.tar`

### Vérification Chrome (DevTools Network)

```javascript
// Dans l'onglet Network de DevTools, surveiller la requête /fs-backup
// Vérifier l'en-tête Content-Disposition dans la réponse
// Valeur attendue: attachment; filename="buzzcontrol-full-backup-v3.7.0-YYYYMMDD.tar"

// Ou via fetch :
fetch('/fs-backup')
  .then(r => {
    const cd = r.headers.get('Content-Disposition')
    console.log('Content-Disposition:', cd)
    console.assert(cd.includes('v3.7.0'), 'Version doit être dans le nom de fichier')
    console.assert(cd.includes('buzzcontrol-full-backup'), 'Préfixe doit être buzzcontrol-full-backup')
  })
```

---

## Scénario 6 — #57 Déduplication VJoueur à la reconnexion

### Objectif
Vérifier qu'un joueur virtuel qui se reconnecte n'apparaît pas en double
dans la liste des joueurs.

### Étapes

1. Ouvrir http://localhost/player dans Chrome (onglet joueur)
2. Saisir un nom (ex: "TestJoueur") et rejoindre une session
3. Fermer et rouvrir l'onglet joueur
4. Observer la liste des joueurs dans l'admin

### Résultats attendus

- "TestJoueur" n'apparaît qu'une seule fois dans la liste
- Pas de doublon avec l'ancienne connexion zombie
- La reconnexion est transparente

---

## Scénario 7 — #50 LED COMET à l'attribution de points

### Objectif
Vérifier que les buzzers physiques affichent l'animation COMET (flash doré ~2s)
quand des points sont attribués.

### Prérequis

- Au moins un buzzer BuzzClick physique connecté en WebSocket
- Firmware v3.7.0+ sur le buzzer

### Étapes

1. Ouvrir http://localhost/admin/game dans Chrome
2. Démarrer une question
3. Attribuer des points à un buzzer via le bouton "+1 point" ou "BUMPER_POINTS"
4. Observer la LED du buzzer physique

### Résultats attendus

- La LED du buzzer s'illumine en doré avec un effet de "comète" pendant ~2 secondes
- Après l'animation, la LED revient à la couleur d'équipe normale

### Vérification logs serveur

Surveiller les logs dans http://localhost/admin/logs :
```
LED_SET → ACTION:LED_SET, EFFECT:"COMET", COLOR:[255,215,0]
```
