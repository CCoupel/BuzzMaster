# E2E Tests - Server Parameters Config (v2.49.0)

## Prérequis
- Serveur BuzzControl lancé sur `http://localhost` (port HTTP)
- Page admin accessible : `http://localhost/admin`
- Navigateur Chrome avec MCP claude-in-chrome actif
- Section "Parametres serveur" visible dans la page Configuration

---

## Scénario 1 : Charger la page Configuration et afficher les paramètres serveur

### Étapes
1. Naviguer vers `http://localhost/admin`
2. Cliquer sur le menu abeille (🐝)
3. Sélectionner "⚙️ Config"
4. Attendre le chargement complet de la page Configuration
5. Chercher la section "Parametres serveur"

### Vérifications
- Page Configuration est chargée (`/admin/settings`)
- Titre de section visible : "Parametres serveur"
- Description affichée : "Configuration du comportement du serveur au demarrage et mode de fonctionnement."
- Deux toggles présents : "Ouvrir les navigateurs automatiquement" et "Mode debug"
- Bouton "Enregistrer" visible

### Acceptation
✅ Section "Parametres serveur" chargée et affichée correctement

---

## Scénario 2 : Charger les valeurs actuelles des paramètres

### Étapes
1. Page Configuration chargée
2. Observer les états des toggles "Ouvrir les navigateurs automatiquement" et "Mode debug"
3. Vérifier que les valeurs correspondent à la configuration du serveur

### Vérifications
- Les toggles reflètent l'état actuel depuis `/config.json`
- Si `auto_open_browsers=true`, le toggle doit être coché
- Si `debug=false`, le toggle ne doit pas être coché
- Les valeurs sont chargées au montage de la page

### Acceptation
✅ Paramètres chargés depuis le serveur correctement

---

## Scénario 3 : Basculer le paramètre "auto_open_browsers"

### Étapes
1. Page Configuration chargée
2. Localiser le toggle "Ouvrir les navigateurs automatiquement"
3. Cliquer sur le toggle pour le basculer
4. Observer le changement d'état visuel (coché/non coché)

### Vérifications
- Le toggle bascule visuellement
- L'état du composant React change (observable via DevTools)
- Pas d'erreur dans la console

### Acceptation
✅ Toggle basculé avec feedback visuel

---

## Scénario 4 : Basculer le paramètre "debug"

### Étapes
1. Page Configuration chargée
2. Localiser le toggle "Mode debug"
3. Cliquer sur le toggle pour le basculer
4. Observer le changement d'état visuel

### Vérifications
- Le toggle bascule visuellement
- L'état du composant React change
- Pas d'erreur dans la console

### Acceptation
✅ Toggle "Mode debug" fonctionne

---

## Scénario 5 : Sauvegarder les paramètres serveur

### Étapes
1. Page Configuration chargée
2. Basculer les deux toggles (mettre à jour les valeurs)
3. Cliquer sur le bouton "Enregistrer"
4. Attendre la confirmation (disparition du state "loading" du bouton)
5. Ouvrir la console Network pour vérifier la requête

### Vérifications
- Bouton "Enregistrer" devient temporairement en état "loading"
- Une requête POST est envoyée à `/config.json`
- Le corps de la requête contient les nouveaux paramètres :
  ```json
  {
    "server": {
      "auto_open_browsers": true/false,
      "debug": true/false
    }
  }
  ```
- Réponse HTTP 200 OK
- Aucun message d'erreur

### Acceptation
✅ Paramètres sauvegardés avec succès

---

## Scénario 6 : Recharger la page et vérifier la persistance

### Étapes
1. Après le Scénario 5, recharger la page (F5)
2. Attendre que Configuration se charge à nouveau
3. Vérifier l'état des toggles

### Vérifications
- Les nouveaux paramètres sont affichés après recharge
- Les valeurs persistent (reflètent les modifications précédentes)
- Pas d'erreur lors du chargement

### Acceptation
✅ Paramètres persisted correctement

---

## Scénario 7 : Gestion des erreurs lors de la sauvegarde

### Étapes
1. (Simulé) Interrompre la connexion réseau
2. Basculer un toggle
3. Cliquer sur "Enregistrer"
4. Observer la réaction du système

### Vérifications
- Une alerte d'erreur s'affiche
- Le message d'erreur est informatif
- Le bouton "Enregistrer" revient à l'état normal
- Les changements locaux ne sont pas perdus

### Acceptation
✅ Gestion des erreurs correcte

---

## Scénario 8 : Interaction avec d'autres sections de Configuration

### Étapes
1. Page Configuration chargée
2. Modifier les paramètres serveur
3. Scroller jusqu'à une autre section (ex: Neon Effect)
4. Modifier un paramètre dans cette section aussi
5. Cliquer sur "Enregistrer" dans la section Neon Effect
6. Vérifier que les changements Neon sont sauvegardés
7. Recharger et vérifier les deux ensembles de paramètres

### Vérifications
- Les deux sections peuvent être éditées indépendamment
- La sauvegarde de l'une n'affecte pas l'autre
- Pas de conflit entre les requêtes POST
- Les deux sections persist après recharge

### Acceptation
✅ Sections indépendantes et cohérentes

---

## Scénario 9 : Responsive - Affichage sur petit écran

### Étapes
1. Redimensionner la fenêtre à 600px (mobile/tablet)
2. Naviguer vers Configuration
3. Localiser la section "Parametres serveur"
4. Tester les toggles et le bouton d'enregistrement

### Vérifications
- Textes lisibles et bien espacés
- Toggles cliquables
- Bouton "Enregistrer" visible et utilisable
- Pas de débordement horizontale (overflow)
- Responsive design fonctionne

### Acceptation
✅ Responsivité correcte

---

## Notes de Validation

### Points clés
- Section "Parametres serveur" est une nouvelle addition
- Utilise le même pattern que "Neon Effect" pour le chargement/sauvegarde
- Endpoint `/config.json` en POST supporte les paramètres serveur
- État local gérés par `useState` dans ConfigPage

### Considérations
- Les paramètres ne prennent effet qu'au redémarrage du serveur (auto_open_browsers, debug)
- Une indication pourrait être ajoutée (ex: "Redémarrer le serveur pour appliquer les changements")
- Le mode debug peut incluire des logs supplémentaires

### Points à déboguer si problèmes
- Console React pour erreurs de chargement
- Network tab pour vérifier les requêtes POST
- DevTools pour inspecter l'état `serverParams`
- Vérifier que `config.json` est bien écrit côté serveur
