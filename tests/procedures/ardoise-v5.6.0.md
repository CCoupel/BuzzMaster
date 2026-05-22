# Procédure de Test — Mode ARDOISE v5.6.0

**Version** : 5.6.0  
**Date** : 2026-05-20  
**Testeur** : QA  
**Issues** : #86 (backend), #87 (QuestionsPage), #88 (ArdoiseKeyboard + VPlayer), #89 (GamePage admin), #90 (PlayerDisplay TV)

---

## Prérequis

- [ ] Environnement : QUALIF (serveur local ou serveur de qualification)
- [ ] Données : au moins 2 équipes configurées avec un joueur/buzzer chacune
- [ ] Accès : interface admin (`/`), TV (`/tv`), VJoueur (`/player`)
- [ ] Navigateurs : Chrome/Firefox pour admin + TV + VJoueur (3 onglets ou appareils séparés)

---

## Scénario 1 — Création d'une question ARDOISE (éditeur, #87)

**Objectif** : Vérifier que l'éditeur de questions propose le type ARDOISE avec les champs appropriés.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir l'interface admin → onglet "Questions" | Page QuestionsPage affichée, formulaire "Nouvelle Question" visible | | |
| 2 | Observer la rangée des boutons de type | Les boutons Normal / QCM / Memory / Memotion / ⌨️ Ardoise sont présents | | |
| 3 | Cliquer sur "⌨️ Ardoise" | Le bouton devient actif (surligné), les champs QCM (Rouge/Vert/Jaune/Bleu) disparaissent | | |
| 4 | Vérifier que le champ "Bonne réponse (animateur)" apparaît | Le label indique "Bonne réponse * (animateur)" et le placeholder précise que c'est visible animateur seulement | | |
| 5 | Vérifier la section "Clavier virtuel" | Deux boutons : "⌨️ AZERTY — Clavier texte complet" et "🔢 Pavé numérique — Chiffres uniquement" | | |
| 6 | AZERTY est actif par défaut | Le bouton AZERTY est surligné | | |
| 7 | Cliquer sur "Pavé numérique" | Le bouton NUMPAD devient actif, AZERTY se désactive | | |
| 8 | Recliquer sur AZERTY | AZERTY redevient actif | | |
| 9 | Vérifier que les champs images (question/réponse) sont masqués | La section "Images" n'est pas affichée pour le type ARDOISE | | |
| 10 | Saisir un titre de question, une bonne réponse, et sauvegarder | La question apparaît dans la liste avec le type "ARDOISE" | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Clavier virtuel ArdoiseKeyboard (#88)

**Objectif** : Vérifier le rendu et l'interaction du clavier virtuel sur VJoueur.

### 2a — Layout AZERTY

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir le VJoueur (`/player`) avec une question ARDOISE active en phase PREPARE ou READY | Le clavier AZERTY est visible en bas de l'écran (overlay) | | |
| 2 | Observer le clavier | 3 rangées de touches (10 + 10 + 8), touche ⌫ (rouge) et ␣ présentes | | |
| 3 | Observer la zone de texte au-dessus du clavier | Affiche "Votre réponse…" (placeholder) | | |
| 4 | Taper quelques lettres (ex: "P", "A", "R", "I", "S") | La zone de texte se met à jour en temps réel : "PARIS" | | |
| 5 | Appuyer sur ⌫ | Le dernier caractère est supprimé : "PARI" | | |
| 6 | Appuyer sur ␣ | Un espace est ajouté | | |
| 7 | Cliquer sur ✕ (effacer tout) | Le texte est entièrement effacé | | |

**Verdict** : [ ] PASS  [ ] FAIL

### 2b — Layout NUMPAD

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Créer une question ARDOISE avec clavier NUMPAD, lancer la partie | Le clavier affiché sur VJoueur est un pavé numérique | | |
| 2 | Observer le clavier | 4 rangées (7/8/9, 4/5/6, 1/2/3, ./0/⌫), pas de lettres | | |
| 3 | Taper "42" | La zone de texte affiche "42" | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Envoi de la réponse et verrouillage (#88)

**Objectif** : Vérifier que le clavier se verrouille correctement selon la phase.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Phase READY : observer le clavier VJoueur | Les touches sont désactivées (grisées), overlay "⏳ En attente…" visible | | |
| 2 | L'animateur clique START | Les touches deviennent actives (pas d'overlay), le texte saisi précédent est effacé si nouvelle question | | |
| 3 | Taper "Paris" pendant la phase STARTED | Frappe transmise en temps réel (throttle 200ms), pas de blocage | | |
| 4 | L'animateur clique STOP | Le clavier se verrouille instantanément — overlay "✓ Réponse envoyée" si du texte a été saisi | | |
| 5 | Après verrouillage : tenter de taper une lettre | La frappe ne modifie pas le texte affiché | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Panneau admin temps réel (#89)

**Objectif** : Vérifier que l'animateur voit les réponses en temps réel dans GamePage.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer une question ARDOISE, phase STARTED | Dans GamePage, un panneau "⌨️ Réponses équipes" apparaît avec la liste des équipes | | |
| 2 | Observer les équipes sans réponse | Chaque équipe affiche "—" (no-answer) | | |
| 3 | Un joueur VJoueur tape sa réponse | La réponse apparaît en temps réel dans le panneau, sans refresh | | |
| 4 | Vérifier que le panneau est absent pour TYPE=QCM | Lancer une question QCM → le panneau ARDOISE n'est pas visible | | |
| 5 | Phase STOPPED : le panneau reste visible | Les réponses sont toujours affichées, pas de boutons "+N pts" | | |
| 6 | L'animateur clique "Révéler" → phase REVEALED | Le header du panneau affiche "✓ [bonne réponse]" + des boutons "+N pts" apparaissent | | |
| 7 | Cliquer sur "+N pts" pour une équipe | Les points sont attribués à l'équipe (score mis à jour dans TeamCard) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Affichage TV en phase REVEALED (#90)

**Objectif** : Vérifier l'écran de révélation ARDOISE sur la TV.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Phase REVEALED sur une question ARDOISE | La TV affiche un écran dédié ARDOISE (ardoise-reveal-layout) | | |
| 2 | Observer la bonne réponse | Un bandeau "Réponse correcte" suivi de la réponse (ex: "Le Nil") | | |
| 3 | Observer la liste des équipes | Chaque équipe affiche son badge coloré + sa réponse (ou "—" si pas de réponse) | | |
| 4 | Les équipes avec réponse | Classe "has-answer" visible (fond légèrement différent) | | |
| 5 | Si ≥ 9 équipes configurées | Seules 8 équipes sont affichées (pas de scroll) | | |
| 6 | Vérifier qu'il n'y a pas de scroll | La TV ne scroll pas — overflow: hidden respecté | | |
| 7 | Ouvrir le VJoueur en phase REVEALED | Le VJoueur N'affiche PAS l'écran ARDOISE TV (affichage standard à la place) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 6 — Non-régression (#86-#90)

**Objectif** : Vérifier qu'aucune régression n'a été introduite sur les autres types de questions.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer une question NORMAL | Fonctionnement normal — le clavier ARDOISE n'apparaît pas sur VJoueur | | |
| 2 | Lancer une question QCM | Boutons buzz actifs — le clavier ARDOISE n'apparaît pas | | |
| 3 | Lancer une question MEMORY | Fonctionnement normal | | |
| 4 | Lancer une question MEMOTION | Fonctionnement normal, écran MEMOTION sur TV | | |
| 5 | Vérifier que ARDOISE_ANSWERS est bien {} après une nouvelle question | Lancer une 2e question ARDOISE : les réponses de la question précédente n'apparaissent plus | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation

- [ ] Tous les scénarios nominaux passent (SC1 à SC5)
- [ ] Aucune régression sur QCM / NORMAL / MEMORY / MEMOTION (SC6)
- [ ] Pas de scroll sur TV (#90 — contrainte statique)
- [ ] Max 8 équipes sur TV (#90)
- [ ] Le panneau admin disparaît si TYPE ≠ ARDOISE (#89)
- [ ] Le clavier se verrouille à la fin de la phase STARTED (#88)

## Notes QA

[Espace pour observations]
