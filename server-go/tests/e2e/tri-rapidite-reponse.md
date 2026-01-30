# Tests E2E : Tri par Rapidité de Réponse (v2.44.1)

## Prérequis
- Serveur démarré sur http://localhost
- Admin connecté à /admin
- TV affichage sur /tv
- 3-4 équipes créées avec joueurs (minimum)

---

## Scénario 1 : Buzz première équipe (classement 🏆)

### Étapes
1. Sélectionner une question
2. Cliquer START (30s)
3. Attendre environ 2 secondes
4. Cliquer sur équipe "Les Rouges" → buzz

### Vérification
- [ ] "Les Rouges" remonte immédiatement en haut de la colonne équipes
- [ ] Badge 🏆 apparaît avant le nom
- [ ] Temps affiché : ~2000ms (±500ms)
- [ ] Animation fluide (pas de saccades)
- [ ] Autres équipes restent en bas de liste

---

## Scénario 2 : Buzz deuxième équipe (classement 🥈)

### Étapes
1. Après le buzz de Les Rouges (Scenario 1)
2. Attendre environ 3 secondes de plus
3. Cliquer sur équipe "Les Bleus" → buzz
4. Vérifier la réorganisation

### Vérification
- [ ] Les Bleus se place juste après Les Rouges (par temps)
- [ ] Badge 🥈 apparaît avant le nom des Bleus
- [ ] Temps affiché pour Bleus : ~5000ms (±500ms)
- [ ] Les Rouges : toujours 🏆, Les Bleus : 🥈
- [ ] Animation réorganisation fluide (300ms)
- [ ] Équipes non buzzées restent en bas

---

## Scénario 3 : Buzz troisième équipe (classement 🥉)

### Étapes
1. Après buzz Les Bleus
2. Attendre environ 2 secondes
3. Cliquer sur équipe "Les Verts" → buzz
4. Vérifier classement final

### Vérification
- [ ] Classement final : Rouges 🏆, Bleus 🥈, Verts 🥉
- [ ] Temps correct pour chaque équipe
- [ ] Équipes non buzzées sans badge en bas (Oranges, etc.)
- [ ] Pas de temps affichés pour non-buzzées
- [ ] Animations 300ms fluides et synchronisées

---

## Scénario 4 : Buzz joueur au sein équipe

### Étapes
1. Vérifier tri joueurs au sein de chaque équipe
2. Ouvrir le détail d'une équipe (ex: Rouges)
3. Cliquer sur un joueur (ex: Alice) → buzz
4. Vérifier réorganisation joueurs

### Vérification
- [ ] Joueur Alice apparaît en haut de sa liste d'équipe
- [ ] Temps joueur affiché à côté du nom (XXXms)
- [ ] Flash animation visible (scale 0.95→1.0, 500ms, vert)
- [ ] Autres joueurs en bas de liste
- [ ] Tri stable si temps égaux

---

## Scénario 5 : Phase PAUSED - Tri persiste

### Étapes
1. Après START de Scenario 1
2. Cliquer PAUSE
3. Vérifier que tri persiste

### Vérification
- [ ] Équipes restent triées par temps (pas retour au tri par score)
- [ ] Temps et badges toujours visibles
- [ ] Tri stable (ordre inchangé)
- [ ] Badge PAUSE visible

---

## Scénario 6 : Phase REVEALED - Tri persiste

### Étapes
1. Après PAUSE, cliquer REPONSE (phase REVEALED)
2. Vérifier tri et affichage

### Vérification
- [ ] Tri persiste en REVEALED
- [ ] Temps et badges toujours visibles
- [ ] Scores peuvent être mis à jour (clic sur équipe)
- [ ] Tri stable

---

## Scénario 7 : Retour à STOP - Retour tri par score

### Étapes
1. Après REVEALED, cliquer STOP
2. Sélectionner nouvelle question
3. Vérifier retour au tri par score

### Vérification
- [ ] Équipes triées par SCORE (ancien comportement)
- [ ] Temps masqués (pas affichés)
- [ ] Badges 🏆 🥈 🥉 disparaissent
- [ ] Pas de flash animation
- [ ] Équipe avec plus de points en haut

---

## Scénario 8 : Responsive - Tablet (768x1024)

### Étapes
1. Redimensionner navigateur à 768px de largeur
2. Sélectionner question, cliquer START
3. Après 2s, buzzer avec une équipe
4. Vérifier affichage en phase STARTED

### Vérification
- [ ] Colonne équipes réduite mais lisible
- [ ] Temps visible (font-size adaptée)
- [ ] Badge toujours visible et pas coupé
- [ ] Pas de débordement horizontal
- [ ] Animations fluides (300ms)

---

## Scénario 9 : Responsive - Mobile (320x640)

### Étapes
1. Redimensionner navigateur à 320px de largeur
2. Sélectionner question, cliquer START
3. Après 2s, buzzer avec une équipe
4. Vérifier affichage

### Vérification
- [ ] Temps visible (font-size très réduite 0.7rem)
- [ ] Noms équipes lisibles (pas coupés)
- [ ] Badges visibles
- [ ] Pas de débordement horizontal
- [ ] Animations fluides et rapides

---

## Scénario 10 : Équipes sans buzz - Comportement

### Étapes
1. Créer 4 équipes : A, B, C, D
2. Sélectionner question, START
3. Buzzer : A à 2s, C à 4s
4. Vérifier ordre

### Vérification
- [ ] Ordre final : A (2000ms) 🏆, C (4000ms) 🥈, B et D (non buzzés en bas)
- [ ] B et D pas de temps affiché
- [ ] Pas de badge pour B et D
- [ ] Temps correct pour A et C

---

## Scénario 11 : Plusieurs buzz rapides

### Étapes
1. Créer 5 équipes
2. START
3. Buszer rapidement : équipe1 à 0.5s, équipe2 à 0.6s, équipe3 à 0.7s
4. Observer réorganisation

### Vérification
- [ ] Ordre stable : équipe1, équipe2, équipe3, équipe4, équipe5
- [ ] Temps très rapprochés (501ms, 601ms, 701ms)
- [ ] Tri stable même avec petits écarts
- [ ] Animations fluides

---

## Scénario 12 : Buzz équipe vs buzz joueur

### Étapes
1. Équipe A buze (2 joueurs dans équipe)
2. Attendre 1s
3. Premier joueur (Alice) buze
4. Attendre 1s
5. Deuxième joueur (Bob) buze
6. Vérifier ordre joueurs

### Vérification
- [ ] Équipe A remonte avec badge 🏆 (temps équipe = temps Alice)
- [ ] Joueurs dans A : Alice (2000ms), Bob (3000ms), puis non-buzzés
- [ ] Temps Alice < Temps Bob
- [ ] Flash animation sur Alice, puis sur Bob
- [ ] Pas de réorganisation équipe (A reste en haut)

---

## Points Critiques à Tester

- [ ] **Tri stable** : Même temps = ordre préservé
- [ ] **Calcul temps correct** : Vérifier ms (1000 µs = 1ms, pas l'inverse)
- [ ] **Badges corrects** : 🏆 rang 1, 🥈 rang 2, 🥉 rang 3, rien après
- [ ] **Équipes non buzzées** (TIME=0) toujours en bas, jamais au-dessus
- [ ] **Animations 300ms** fluides, pas de saccades (60fps min)
- [ ] **Flash 500ms** visible sur nouveau buzz
- [ ] **Phase-aware** : Tri OFF en STOP/PREPARE/READY, ON en STARTED/PAUSED/REVEALED
- [ ] **Responsive** : Mobile 320px, Tablet 768px, Desktop 1920px
- [ ] **Affichage temps** : Formaté XXXms, lisible à tous les niveaux de zoom

---

## Notes Techniques

### Timestamps
- `gameState.GAME_TIME` : Timestamp serveur au démarrage (µs)
- `team.TIME` : Timestamp du buzz de l'équipe (µs)
- `bumper.TIME` : Timestamp du buzz du joueur (µs)

### Calcul ms
```
timeMs = Math.round((team.TIME - gameState.GAME_TIME) / 1000)
```

### Phases
- STOP/PREPARE/READY : Tri par score, temps masqué
- STARTED/PAUSED/REVEALED : Tri par temps (si > 0), temps visible

### layoutId Framer-motion
- `layoutId={team-${name}}` pour équipes
- `layoutId={buzzer-${mac}}` pour joueurs
- Spring transition (stiffness: 300, damping: 30)

---

## Exécution

À exécuter avec **MCP claude-in-chrome** :
1. Naviguer vers http://localhost/admin
2. Connecter un compte admin
3. Exécuter chaque scénario dans l'ordre
4. Prendre screenshots des points critiques
5. Rapporter les résultats (PASS/FAIL/BLOQUE)
