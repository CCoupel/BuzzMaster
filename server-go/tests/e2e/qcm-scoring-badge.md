# Tests E2E : QCM Scoring — Pénalité par Joueur et Badge REVEALED (v3.3.2)

## Prérequis
- Serveur démarré sur http://localhost
- Admin connecté à /admin/game
- Au moins 2 équipes créées (ex: "Les Rouges", "Les Bleus")
- Une question QCM créée avec :
  - Indices activés (QCM_HINTS_ENABLED = true)
  - Pénalité 1 hint = 67% (QCM_PENALTY_1 = 0.67)
  - Pénalité 2 hints = 33% (QCM_PENALTY_2 = 0.33)
  - Bonne réponse : RED (bouton A)
  - Valeur : 10 points, durée : 30s

---

## Scénario 1 : Badge +pts en phase REVEALED (équipe correcte)

### Étapes
1. Sélectionner la question QCM dans la liste
2. Cliquer START
3. Simuler un buzz de "Les Rouges" : Ctrl+clic sur le joueur (bouton A → RED = correct)
4. Cliquer STOP pour arrêter la question
5. Cliquer REPONSE pour révéler la réponse

### Vérification
- [ ] En phase REVEALED, la carte équipe "Les Rouges" affiche un badge bleu "+10 pts"
- [ ] "Les Bleus" (n'a pas buzzé ou mauvaise réponse) n'affiche PAS de badge points
- [ ] Le badge est animé (apparition scale 0→1)
- [ ] La couleur du badge est bleue (différente du badge Memory vert)

---

## Scénario 2 : Badge +pts avec pénalité 1 hint

### Prérequis supplémentaires
- Question QCM avec indices activés, même config que ci-dessus

### Étapes
1. Sélectionner la question QCM → START
2. Attendre que le 1er indice se déclenche automatiquement OU injecter via l'état
   (simuler : `e.state.QcmInvalidated = ["GREEN"]` avant le buzz en test unitaire)
3. Simuler un buzz de "Les Rouges" APRES le 1er indice (bouton A = RED)
4. STOP → REPONSE

### Vérification
- [ ] Badge affiché : "+7 pts" (10 × 0.67 = 6.7 → arrondi 7, minimum 1)
- [ ] Pas "+10 pts" (la pénalité s'applique bien car indice révélé AVANT le buzz)
- [ ] "Les Bleus" : aucun badge (n'a pas buzzé ou mauvaise réponse)

---

## Scénario 3 : Pénalité individuelle — deux joueurs buzzent à des moments différents

### Description
Vérifie que la pénalité est basée sur les hints au MOMENT DU BUZZ de chaque
joueur individuellement, pas sur les hints actuels au moment du clic admin.

### Prérequis supplémentaires
- 2 équipes avec 1 joueur chacune
- Hints activés sur la question QCM

### Étapes (test unitaire Go — vérification via engine_test.go)
1. Joueur A (Les Rouges) buzze SANS hint → HINTS_AT_BUZZ = 0
2. 1 hint est révélé (QcmInvalidated = ["GREEN"])
3. Joueur B (Les Bleus) buzze AVEC 1 hint → HINTS_AT_BUZZ = 1
4. Admin clique sur "Les Rouges" pour attribuer les points

### Vérification
- [ ] Les Rouges reçoit 10 pts (HINTS_AT_BUZZ=0 → pas de pénalité)
- [ ] Les Bleus reçoit 7 pts (HINTS_AT_BUZZ=1 → pénalité 67%)
- [ ] Les deux calculs sont indépendants du nombre d'hints ACTUELS du jeu

---

## Scénario 4 : Pas de badge pour mauvaise réponse

### Étapes
1. Sélectionner question QCM → START
2. Simuler buzz de "Les Rouges" avec bouton B (GREEN = mauvaise réponse)
3. STOP → REPONSE

### Vérification
- [ ] "Les Rouges" n'affiche PAS de badge "+pts" en phase REVEALED
- [ ] (car la réponse était incorrecte — ANSWER_COLOR != QCM_CORRECT)

---

## Scénario 5 : Pas de badge si équipe n'a pas buzzé

### Étapes
1. Sélectionner question QCM → START
2. Simuler buzz uniquement de "Les Rouges" (bonne réponse)
3. "Les Bleus" ne buzze pas
4. STOP → REPONSE

### Vérification
- [ ] "Les Rouges" : badge "+10 pts" affiché
- [ ] "Les Bleus" : aucun badge (TIME = 0, n'a pas buzzé)

---

## Scénario 6 : Badge disparaît après PREPARE (nouvelle question)

### Étapes
1. Réaliser le Scénario 1 (badge affiché)
2. Sélectionner une nouvelle question (transition vers PREPARE)

### Vérification
- [ ] Le badge "+pts" disparaît dès la transition PREPARE
- [ ] Les cartes équipes reviennent à l'état normal (PRET badge si applicable)
