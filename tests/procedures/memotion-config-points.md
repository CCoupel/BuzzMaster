# Procédure de Test — MEMOTION Points Configurables par Difficulté

**Version** : 5.0.5
**Date** : 2026-05-06
**Testeur** : QA

## Prérequis

- [ ] Environnement : LOCAL ou QUALIF
- [ ] Serveur démarré (`go build -o server.exe ./cmd/server && ./server.exe`)
- [ ] Interface admin accessible sur `http://localhost/`
- [ ] Au moins 2 équipes configurées (ex: Rouge et Bleu)

---

## Scénarios

### Scénario 1 — Points configurés via l'éditeur (config complète 2/6/10)

**Objectif** : Vérifier qu'une question MEMOTION avec `MotionConfig{2, 6, 10}` affiche et
attribue les points configurés (pas les valeurs par défaut 1/3/5).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|------------------|----------------|------|
| 1 | Ouvrir l'éditeur de questions (`/questions`) | Page de liste des questions s'affiche | | |
| 2 | Créer une question MEMOTION (ou en éditer une existante) | Formulaire MEMOTION ouvert | | |
| 3 | Dans la section "Points par difficulté" : saisir 1★=2, 2★=6, 3★=10 | Champs acceptent les valeurs | | |
| 4 | Ajouter 3 cartes : une 1★, une 2★, une 3★ | Cartes créées avec difficulté correcte | | |
| 5 | Sauvegarder la question | Question sauvegardée sans erreur | | |
| 6 | Sur l'affichage TV (`/tv`), lancer la question | Grille MEMOTION affichée | | |
| 7 | Sélectionner la carte 1★ → Flip → Révéler → Valider équipe Rouge | Score équipe Rouge = **2pts** (pas 1pt) | | |
| 8 | Sélectionner la carte 2★ → Flip → Révéler → Valider équipe Bleu | Score équipe Bleu = **6pts** (pas 3pts) | | |
| 9 | Sélectionner la carte 3★ → Flip → Révéler → Valider équipe Rouge | Score équipe Rouge = 2 + **10pts** = **12pts** (pas 5pts) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 2 — Fallback vers les valeurs par défaut (sans MotionConfig)

**Objectif** : Vérifier qu'une question MEMOTION sans configuration de points utilise
les valeurs par défaut (1★=1pt, 2★=3pts, 3★=5pts).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|------------------|----------------|------|
| 1 | Créer une question MEMOTION sans modifier les points (laisser vide / 0) | Formulaire valide | | |
| 2 | Ajouter 3 cartes : une 1★, une 2★, une 3★ | Cartes créées | | |
| 3 | Sauvegarder et lancer la question | Question active en jeu | | |
| 4 | Jouer la carte 1★ → valider équipe Rouge | Score Rouge = **1pt** (valeur par défaut) | | |
| 5 | Jouer la carte 2★ → valider équipe Bleu | Score Bleu = **3pts** (valeur par défaut) | | |
| 6 | Jouer la carte 3★ → valider équipe Rouge | Score Rouge = 1 + **5pts** = **6pts** (valeur par défaut) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 3 — Config partielle (1★ configuré, 2★ et 3★ à zéro)

**Objectif** : Vérifier que les valeurs à 0 dans MotionConfig déclenchent le fallback
sur les défauts pour les difficultés concernées.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|------------------|----------------|------|
| 1 | Créer une question MEMOTION avec Points1Star=5, Points2Star=0, Points3Star=0 | Formulaire valide | | |
| 2 | Sauvegarder et lancer | Question active | | |
| 3 | Jouer carte 1★ → valider | Score = **5pts** (valeur configurée) | | |
| 4 | Jouer carte 2★ → valider | Score = **3pts** (fallback sur défaut, car 2★=0) | | |
| 5 | Jouer carte 3★ → valider | Score = **5pts** (fallback sur défaut, car 3★=0) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 4 — Affichage TV des points par difficulté

**Objectif** : Vérifier que l'affichage TV reflète correctement les points configurés
sur chaque carte (icônes étoiles + valeur).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|------------------|----------------|------|
| 1 | Lancer une question MEMOTION avec config 2/6/10 | Grille affichée sur TV | | |
| 2 | Observer les badges de difficulté sur les cartes UNPLAYED | Chaque carte affiche sa difficulté (★ / ★★ / ★★★) | | |
| 3 | Sélectionner une carte 3★ | Face recto visible, bouton "Flip" disponible | | |
| 4 | Cliquer "Flip" → face VERSO (question) | La carte affiche la question | | |
| 5 | Cliquer "Révéler" → face REVEAL (réponse) | La carte affiche la réponse + 10pts mis en évidence | | |
| 6 | Valider l'équipe gagnante | Score équipe mis à jour avec +10pts | | |
| 7 | Vérifier la carte est marquée DONE (visuellement) | Carte grisée / marquée avec le nom de l'équipe | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 5 — Non-régression : persistance de la configuration dans le fichier JSON

**Objectif** : Vérifier que `MotionConfig` est correctement sérialisé et rechargé
lors d'un redémarrage du serveur.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|------------------|----------------|------|
| 1 | Créer une question MEMOTION avec config 3/7/12 et sauvegarder | Fichier question créé | | |
| 2 | Redémarrer le serveur (`curl -s http://localhost/shutdown && ./server.exe`) | Serveur redémarré | | |
| 3 | Ouvrir l'éditeur, retrouver la question | Points affichés : 1★=3, 2★=7, 3★=12 | | |
| 4 | Lancer la question et jouer une carte 3★ | Score = **12pts** (config rechargée correctement) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation

- [ ] Tous les scénarios nominaux (1, 2, 3) passent avec les points attendus
- [ ] L'affichage TV (scénario 4) reflète les valeurs configurées
- [ ] La persistance JSON (scénario 5) préserve la configuration après redémarrage
- [ ] Aucune régression sur les questions MEMOTION sans MotionConfig (scénario 2)
- [ ] Les valeurs 0 dans MotionConfig déclenchent bien le fallback (scénario 3)

## Notes QA

[Espace pour observations — versions testées, anomalies, cas limites découverts]
