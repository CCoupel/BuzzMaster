# Procedure de Test — Achat de voyelles de la Roue de la Fortune

**Version** : 12.1.0 (LOT-1.2)
**Date** : 2026-11-21
**Testeur** : QA (scenarios AUTO) · utilisateur (scenarios HUMAIN)
**Issue** : #147 — Achat de voyelles
**Références** : `contracts/roue.md` §15.1, §15.3 ; maquettes `docs/mockup/v12.1.0/ui/{admin_game_roue,anim_roue,tv_roue_started,admin_question_editor_roue}__vowels_bonus.html`, conception `docs/mockup/v12.1.0/conception/game_state_roue__vowels_bonus_timing.html`

> Phrase de référence « TOTO TATA » : O et A présentes, E I U Y absentes. Prix par défaut 250. Les scénarios AUTO sont couverts
> par `*_147_test.go` et `*.147.test.jsx` que `qa` joue ; les HUMAIN exigent un jugement visuel, sonore ou d'usage.

## Prerequis

- [ ] Environnement : LOCAL ou QUALIF, serveur BuzzControl 12.1.0.x
- [ ] Données : 2 équipes avec buzzers ; une question ROUE par réglage testé ; la roue par défaut
- [ ] Accès : admin (`/questions`, `/game`), animateur (tablette), TV (`/tv`), tablette joueur
- [ ] Pour avoir une cagnotte : jouer d'abord une consonne et terminer le tour (« Tour suivant »)

## Scenarios

### Scenario 1 — Réglages par défaut d'une question v12.0 et enregistrement

**Validation** : AUTO — **Issues** : #147
**Tests** : `roue_vowels_147_test.go` (game : `DefaultsCopiedAtStart`, `NineFieldsAlwaysSerialised`) ; (server : `Upload_DefaultsWritten`, `Upload_RefusalCodes`, `Upload_PriceBounds`) ; `RoueVowels.147.test.jsx` (éditeur) ; `QuestionsPage.roue.147.test.jsx`.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `go test ./internal/game ./internal/server -run "Vowel.*147|Vowels147"` | Tous verts | | |
| 2 | `vitest run RoueVowels.147 QuestionsPage.roue.147` | Tous verts | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 2 — Achat avant de tourner : présente, absente, « 1 » / « plusieurs »

**Validation** : AUTO — **Issues** : #147
**Tests** : `Before_*`, `Matrix`, `RefusalsLeaveStateUntouched`, `RefusalOrder`, `AlreadyProposedRefusedWithoutDebit`, `NoVowelLeft`.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `go test ./internal/game -race -run Vowel147` | Tous verts, y compris la matrice moment × nombre | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 3 — Achat après l'arrêt, les deux moments, cases spéciales, tour BONUS

**Validation** : AUTO — **Issues** : #147 #308
**Tests** : `After_*`, `Both_BeforeAndAfter`, `SpecialCasesNoAfterPurchase`, `BonusRestartsTurnAndResetsFlags`, `SpinWithoutConsonant`, `SpinResetsStopVowel`.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `go test ./internal/game -race -run Vowel147` | Tous verts | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 4 — Gain d'une voyelle, débit de cagnotte, scores intacts

**Validation** : AUTO — **Issues** : #147
**Tests** : `GainOptions`, `GainAvailableForNextVowel`, `AbsentNeverRefunded`, `Before_PresentDebitsAndLights` (aucun score de partie modifié), `ConcurrentSameVowelDebitsOnce`.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `go test ./internal/game -race -run Vowel147` | Tous verts | | |
| 2 | `go test ./cmd/server -run Wire147` | Achat par l'animateur et refus à l'émetteur seul verts ; la phrase ne fuit pas vers la TV / tablette | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 5 — L'éditeur : section Voyelles

**Validation** : HUMAIN — **Issues** : #147

**Objectif** : Juger la lisibilité de la section et sa conformité à la maquette `admin_question_editor__roue_vowels_bonus.html`.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `/questions`, type **Roue**, nouvelle question | Section Voyelles : interrupteur activé, « Avant de tourner », « Plusieurs voyelles » (×2), prix 250, « Sans gain » | | |
| 2 | Choisir « Après l'arrêt de la roue » | Le réglage « Avant de tourner » est grisé, celui d'« Après l'arrêt » actif | | |
| 3 | Choisir « Les deux » | Les deux réglages « 1 / plusieurs » sont actifs | | |
| 4 | Choisir « Prix × nombre de lettres » | L'exemple chiffré (3 E à 250 → +750) s'affiche | | |
| 5 | Saisir un prix de 0 | Le champ est signalé invalide ; **Ajouter** grisé | | |
| 6 | Désactiver l'interrupteur | Tous les réglages sont grisés mais gardent leurs valeurs | | |
| 7 | Comparer à la maquette | Disposition et libellés conformes | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 6 — Admin : acheter avant de tourner (étape 1)

**Validation** : HUMAIN — **Issues** : #147

**Objectif** : Vérifier le déroulé à l'écran admin, la clarté des boutons et le retournement au bouton (maquette `admin_game_roue__vowels_bonus.html`). Question : moment « Avant de tourner », « Plusieurs », prix 250, « Mise remboursée ».

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Cagnotte de l'équipe < 250 | « Acheter une voyelle » grisé, avec la cagnotte affichée | | |
| 2 | Cagnotte ≥ 250 : cliquer « Acheter une voyelle », choisir O, « Acheter le O — 250 » | Cases du O éclairées sur la TV ; cagnotte débitée de 250 ; **rien ne se retourne seul** | | |
| 3 | Cliquer « Retourner les cases » | Les cases se retournent une à une ; la cagnotte est créditée du gain (mise remboursée : +250) | | |
| 4 | Regarder l'étape 1 | « Lancer la roue », « Acheter une voyelle » (autre voyelle possible), « Réponse validée » | | |
| 5 | Acheter E (absente) | Son « mauvaise réponse » ; cagnotte débitée sans remboursement ; tour perdu, fin de tour (étape 7) | | |
| 6 | Régler la question sur « 1 voyelle » et recommencer | Après une voyelle, « 1 voyelle par tour : déjà achetée » grisé | | |
| 7 | Lancer la roue | Plus aucun achat jusqu'au tour suivant | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 7 — Admin : acheter après l'arrêt de la roue (étapes 4 et 6)

**Validation** : HUMAIN — **Issues** : #147

**Objectif** : Juger le parcours « voyelle à la place de la consonne » et l'enchaînement. Question : moment « Après l'arrêt », « Plusieurs », prix 250.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Étape 1 | Seul « Lancer la roue » ; un rappel précise que l'achat se fait après l'arrêt | | |
| 2 | Lancer, arrêter sur une case valeur | Titre « Consonne ou voyelle » ; la case n'est pas jouée si une voyelle est achetée | | |
| 3 | Acheter le O | Cases éclairées, cagnotte débitée ; au retournement, étape 6 | | |
| 4 | Étape 6 | « Réponse validée », « Tour suivant » / « Équipe suivante » **et « ou une autre voyelle »** ; aucune consonne n'est plus acceptée | | |
| 5 | Acheter une autre voyelle absente | Tour perdu, fin de tour | | |
| 6 | Arrêter sur Banqueroute, Tour perdu ou BONUS | Aucun achat proposé après l'arrêt | | |
| 7 | Question avec « 1 voyelle » après l'arrêt | À l'étape 6 : « 1 voyelle par arrêt : déjà achetée » grisé | | |
| 8 | Question « Les deux » | Achat possible avant de tourner **et** après l'arrêt | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 8 — Animateur : achat depuis la tablette

**Validation** : HUMAIN — **Issues** : #147

**Objectif** : Juger l'ergonomie de la rangée des voyelles et du bouton d'achat (maquette `anim_roue__vowels_bonus.html`).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Étape 1 (moment « Avant de tourner ») | Rangée des voyelles et bouton « Voyelle · 250 » grisé tant qu'aucune voyelle n'est choisie | | |
| 2 | Choisir O | Le bouton devient « Acheter le O — 250 » | | |
| 3 | Acheter | Mêmes effets qu'à l'admin ; texte lisible en tablette, sans texte explicatif superflu | | |
| 4 | Cagnotte insuffisante / plus de voyelle | « Cagnotte < 250 » / « Plus de voyelle », grisé | | |
| 5 | Étape 6 après une voyelle (après l'arrêt) | « Autre voyelle » ou « 1 voyelle à ce moment » selon le réglage | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 9 — TV et tablette joueur : alphabet, cagnotte, gain

**Validation** : HUMAIN — **Issues** : #147

**Objectif** : Juger la lisibilité sur écran réel, l'absence de défilement et la confidentialité (maquette `tv_roue_started__vowels_bonus.html`).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Acheter une voyelle présente (gain « Prix × nombre de lettres ») | Alphabet complet sur une ligne, voyelle marquée trouvée ; cagnotte de l'équipe débitée puis créditée au retournement | | |
| 2 | Acheter une voyelle absente | Voyelle marquée absente, cagnotte débitée, aucun gain | | |
| 3 | Regarder la TV pendant tout le déroulé | Aucun défilement ; la phrase reste cachée ; cases éclairées lisibles | | |
| 4 | Regarder la tablette joueur | Mêmes informations ; aucun bouton d'achat | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 10 — Dévoilement et pause pendant une voyelle

**Validation** : HUMAIN — **Issues** : #147

**Objectif** : Vérifier à l'usage les gestes de secours.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Acheter une voyelle présente, puis « Dévoiler » avant le retournement | Phrase dévoilée ; la voyelle reste payée, aucun gain | | |
| 2 | Acheter une voyelle, « Retourner les cases » puis **Pause** pendant l'animation | L'animation se fige ; aucune action ROUE n'est acceptée | | |
| 3 | Reprendre | L'animation reprend là où elle s'était arrêtée | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Criteres de Validation

- [ ] Scénarios AUTO rejoués par `qa`
- [ ] Scénarios HUMAIN passent sur QUALIF
- [ ] Aucune régression : tests ROUE v12.0 et BONUS (`*_144_*`, `*_145_*`, `*_308_*`) verts

## Notes QA

Voyelle absente : jamais remboursée, tour perdu. Aucun score de partie n'est touché avant la résolution de la manche.
