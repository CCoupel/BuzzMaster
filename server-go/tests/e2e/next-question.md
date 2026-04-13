# Scénarios E2E — Feature #39 : Bouton "Question suivante"

## Prérequis

- Serveur BuzzControl démarré sur http://localhost:80
- Chrome avec MCP claude-in-chrome disponible
- Au moins 4 questions créées dans la base (dont au moins 2 AVAILABLE)

---

## Scénario 1 : Bouton absent en phase active (STARTED)

### Contexte
Une partie est en cours (timer décompte).

### Étapes
1. Ouvrir http://localhost/admin/game dans Chrome
2. Sélectionner une question, cliquer "PRET" puis "START"
3. Observer le bandeau TV (section display-card)

### Résultat attendu
- Le bouton "Question suivante" est **absent** du bandeau TV
- Le badge de phase affiche "EN COURS"

### Vérification Chrome
```
Vérifier absent: .next-question-btn
Vérifier présent: .phase-badge.phase-running
```

---

## Scénario 2 : Bouton absent quand toutes les questions suivantes sont jouées

### Contexte
La dernière question disponible vient d'être stoppée. Toutes les questions après elle ont le statut PLAYED.

### Étapes
1. Ouvrir http://localhost/admin/game dans Chrome
2. S'assurer que la question sélectionnée est la dernière non jouée (toutes les suivantes = PLAYED)
3. Cliquer "START" puis "STOP"
4. Observer le bandeau TV en phase STOPPED

### Résultat attendu
- Le bouton "Question suivante" est **absent**
- Le badge de phase affiche "ARRET"

### Vérification Chrome
```
Vérifier présent: .phase-badge.phase-stopped
Vérifier absent: .next-question-btn
```

---

## Scénario 3 : Bouton visible en phase STOPPED avec question disponible

### Contexte
Une question vient d'être stoppée et des questions AVAILABLE existent après elle.

### Étapes
1. Ouvrir http://localhost/admin/game dans Chrome
2. Sélectionner la question #2 (avec des questions AVAILABLE après elle)
3. Cliquer "PRET" puis "START" puis "STOP"
4. Observer le bandeau TV

### Résultat attendu
- Le bouton "Question suivante : #3" (ou le numéro de la prochaine AVAILABLE) est **visible**
- Le texte du bouton contient "#" suivi de l'ID de la prochaine question non jouée

### Vérification Chrome
```
Attendre élément: .next-question-btn
Vérifier texte contient: "Question suivante"
Vérifier texte contient: "#"
```

---

## Scénario 4 : Bouton visible en phase REVEALED

### Contexte
La réponse vient d'être révélée.

### Étapes
1. Ouvrir http://localhost/admin/game dans Chrome
2. Sélectionner une question avec des questions disponibles après elle
3. Cliquer "PRET" → "START" → "STOP" → "REPONSE"
4. Observer le bandeau TV en phase REVEALED

### Résultat attendu
- Le badge de phase affiche "REPONSE"
- Le bouton "Question suivante : #X" est **visible**

### Vérification Chrome
```
Vérifier présent: .phase-badge.phase-revealed
Attendre élément: .next-question-btn
Vérifier texte contient: "Question suivante"
```

---

## Scénario 5 : Clic sur bouton → sélection de la question suivante

### Contexte
Le bouton "Question suivante" est visible en phase STOPPED.

### Étapes
1. Ouvrir http://localhost/admin/game dans Chrome
2. Amener le jeu en phase STOPPED avec des questions disponibles après la courante
3. Mémoriser le texte du bouton (ex: "Question suivante : #4")
4. Cliquer sur le bouton "Question suivante : #4"
5. Observer la liste de questions et le titre de la question sélectionnée

### Résultat attendu
- La question #4 est maintenant mise en surbrillance (selected) dans la liste des questions
- La phase repasse en PREPARE ou READY (selon la présence de buzzers)
- Le bouton "Question suivante" disparaît

### Vérification Chrome
```
Cliquer: .next-question-btn
Attendre absent: .next-question-btn
Vérifier présent: .question-card.selected (ou équivalent)
```

---

## Scénario 6 : Opacité 50% sur questions non jouées en phase STARTED

### Contexte
Une partie est en cours. Les questions non jouées (AVAILABLE) qui ne sont pas la question courante doivent apparaître atténuées.

### Étapes
1. Ouvrir http://localhost/admin/game dans Chrome
2. Sélectionner la question #2 et démarrer une partie (START)
3. Observer la liste des questions dans le panneau gauche

### Résultat attendu
- Les questions AVAILABLE autres que la question courante sont **atténuées** (opacity 0.5)
- La question courante (#2) s'affiche à pleine opacité
- Les questions déjà PLAYED s'affichent normalement (non-AVAILABLE = pas dimmed)

### Vérification Chrome
```javascript
// Exécuter dans la console Chrome
const cards = document.querySelectorAll('.questions-list > div')
cards.forEach(card => {
  const opacity = card.style.opacity
  console.log('opacity:', opacity || '1 (normal)')
})
// Les questions non jouées non courantes doivent afficher opacity: 0.5
```

---

## Scénario 7 : Opacité normale sur questions non jouées en phase STOPPED

### Contexte
La partie vient de s'arrêter (phase STOPPED ou REVEALED). Toutes les questions doivent être accessibles sans atténuation.

### Étapes
1. Ouvrir http://localhost/admin/game dans Chrome
2. Démarrer et stopper une partie
3. Observer la liste des questions dans le panneau gauche

### Résultat attendu
- Les questions AVAILABLE n'ont **plus** d'atténuation (opacity normale)
- Toutes les questions sont sélectionnables (cursor normal)

### Vérification Chrome
```javascript
// Exécuter dans la console Chrome
const cards = document.querySelectorAll('.questions-list > div')
const dimmed = Array.from(cards).filter(c => c.style.opacity === '0.5')
console.log('Questions atténuées:', dimmed.length)
// Résultat attendu: 0
```

---

## Scénario 8 : Bouton pointe vers la première AVAILABLE (saute les PLAYED intermédiaires)

### Contexte
La séquence de questions est : Q1(PLAYED), Q2(STOPPED/current), Q3(PLAYED), Q4(AVAILABLE).

### Étapes
1. Créer ou configurer 4 questions dans cet ordre
2. Jouer Q1 et Q3 (statut PLAYED), Q2 en cours
3. Stopper Q2
4. Observer le bouton "Question suivante"

### Résultat attendu
- Le bouton affiche "Question suivante : #4" (saute Q3 qui est PLAYED)
- Cliquer dessus sélectionne Q4

### Vérification Chrome
```
Vérifier texte: "Question suivante : #4"
```
