# Procedure de Test — Composition de la roue par question

**Version** : 12.1.0 (LOT-2)
**Date** : 2026-11-22
**Testeur** : QA (scenarios AUTO) · utilisateur (scenarios HUMAIN, en QUALIF)
**Issue** : #309 — Roue réglable par question
**Références** : `contracts/roue.md` §16 ; maquettes `docs/mockup/v12.1.0/ui/admin_settings__wheel_bonus.html` et `admin_question_editor__roue_wheel_fields.html`

> Règle : **champ présent sur la question = paramètre personnalisé ; champ absent = le paramètre suit le réglage global.**
> Cinq paramètres : points maximum, pas, nombre de Banqueroute, de Perdu, de BONUS. Les durées restent toujours globales.
> Les scénarios AUTO sont couverts par `*_309_test.go` et `*.309.test.jsx` que `qa` joue ; les HUMAIN exigent un jugement visuel ou d'usage.

## Prerequis

- [ ] Environnement : QUALIF, serveur BuzzControl 12.1.0.x
- [ ] Données : la roue globale par défaut (1000 / 50 / 2 / 1 / 0 BONUS) ; une question ROUE v12.0 existante ; si possible une question créée au LOT-1 avec des BONUS
- [ ] Accès : admin (`/config` Paramètres, `/questions`, `/game`), TV (`/tv`)

## Scenarios

### Scenario 1 — Le moteur compose la roue : global, personnalisé, 0, repli

**Validation** : AUTO — **Issues** : #309
**Tests** : `internal/game/roue_composition_309_test.go` (`NoFieldIsGlobalWheelAndV120`, `Lot1QuestionKeepsItsWheel`, `EachParameterAlone`, `SeveralTogether`, `ZeroIsCustomValueNotAbsent`, `NonCustomFollowsGlobalChange`, `CustomDoesNotFollowGlobalChange`, `StepCorrectedAgainstEffectiveMaximum`, `BonusReducedBeforeAnyFallback`, `InvalidCompositionFallsBackToWholeGlobalWheel`, `GlobalChangedAfterSaveFallsBack`, `DurationsAlwaysGlobal`, `QuestionJSONRoundTrip`) ; `roue_effective_309_test.go`.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `go test ./internal/game -race -run "Composition309\|Effective309"` | Tous verts | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 2 — HTTP et configuration : enregistrement, refus, BONUS global

**Validation** : AUTO — **Issues** : #309
**Tests** : `internal/server/roue_composition_309_test.go` (`Upload_*`, `GameConfig_*`, `Lot1QuestionFileStillLoads`) ; `internal/config/gameconfig_bonus_309_test.go`.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `go test ./internal/server ./internal/config -run "RoueCompo309\|WheelBonus309"` | Tous verts : champ vide retiré, `0` enregistré, pas corrigé, refus `ROUE_WHEEL_INVALID`, `bonus_count` global 0 à 4 | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 3 — Écrans : éditeur, carte, formulaire, hook (vitest)

**Validation** : AUTO — **Issues** : #309
**Tests** : `WheelConfigCard.309.test.jsx`, `RoueComposition.309.test.jsx`, `pages/QuestionsPage.roue.309.test.jsx`, `hooks/useWebSocket.gameStateFields.309.test.js`.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `vitest run` sur ces fichiers | Tous verts | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 4 — Paramètres : la carte « Roue de la Fortune » en deux groupes

**Validation** : HUMAIN — **Issues** : #309

**Objectif** : Juger la lisibilité de la carte et sa conformité à la maquette `admin_settings__wheel_bonus.html`.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir les Paramètres, carte « 🎡 Roue de la Fortune » | Deux groupes : **Points** (maximum, pas) et **Cases spéciales** (Banqueroute, Perdu, 🎁 BONUS à 0) ; texte « Réglages par défaut des questions ROUE ; chaque question peut en personnaliser un ou plusieurs » | | |
| 2 | Mettre BONUS à 2 | L'aperçu montre 2 cases dorées « 🎁 BONUS » en plus ; le résumé compte les cases | | |
| 3 | Mettre BONUS à 4 puis Banqueroute à 60 | Message de refus (plus de 64 cases) ; Enregistrer grisé | | |
| 4 | Revenir à une valeur valable, Enregistrer | Enregistré ; relire la page : BONUS conservé | | |
| 5 | « Roue par défaut » | Retour à 1000 · 50 · 2 · 1 · 0 | | |
| 6 | Comparer à la maquette | Disposition, libellés et dorure conformes | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 5 — Éditeur de question : composition liée au global, champ par champ

**Validation** : HUMAIN — **Issues** : #309

**Objectif** : Juger que l'éditeur rend clair ce qui suit le global et ce qui est personnalisé (maquette `admin_question_editor__roue_wheel_fields.html`).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `/questions`, type **Roue**, nouvelle question | Section « 🎡 Roue — composition » : 5 paramètres **pré-remplis avec les valeurs des Paramètres**, marqués « 🔗 global », non modifiables ; résumé « Tous les paramètres suivent les Paramètres » | | |
| 2 | Cliquer ✏️ sur **Banqueroute** | Seul ce champ devient éditable, marqué « personnalisé », avec le rappel « global : 2 — 🔗 pour y revenir » ; le résumé le cite | | |
| 3 | Mettre Banqueroute à 0 | 0 est accepté comme valeur personnalisée ; l'aperçu n'a plus de Banqueroute | | |
| 4 | Personnaliser aussi **BONUS** (3) | L'aperçu montre 3 cases BONUS ; le résumé cite les deux champs | | |
| 5 | Cliquer 🔗 sur Banqueroute | Le champ revient au global (valeur globale affichée, non modifiable) ; BONUS reste personnalisé | | |
| 6 | Saisir un pas qui ne divise pas le maximum (maximum 100, pas 30) | Message « Pas ajusté à 25 » ; jamais de refus | | |
| 7 | Saisir 70 Banqueroute | Refus lisible (plus de 64 cases) ; **Ajouter** grisé | | |
| 8 | Comparer à la maquette | Disposition, 🔗 / ✏️, badges conformes | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 6 — Enregistrer, rouvrir, et suivre les Paramètres

**Validation** : HUMAIN — **Issues** : #309

**Objectif** : Vérifier à l'usage que les valeurs globales sont bien suivies, et les personnalisées conservées.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Créer une question A tout global, une question B avec « BONUS = 2 » personnalisé seul, une question C avec Banqueroute = 0 personnalisé | Les trois s'enregistrent | | |
| 2 | Rouvrir A, B, C | A : tout 🔗 ; B : seul BONUS personnalisé (2) ; C : seule Banqueroute personnalisée (0) | | |
| 3 | Dans les Paramètres, passer le maximum à 500 et le pas à 50 ; **Enregistrer** | | | |
| 4 | Lancer A en jeu | Roue de 500 maximum, 0 BONUS (global) | | |
| 5 | Lancer B | Roue de 500 maximum **avec 2 BONUS** | | |
| 6 | Lancer C | Roue de 500 maximum, **sans** Banqueroute | | |
| 7 | Rouvrir C dans l'éditeur | Le maximum affiche 500 (suit le global), Banqueroute reste 0 personnalisée | | |
| 8 | Regarder la durée de rotation et de freinage de A, B, C | Identiques : toujours celles des Paramètres | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 7 — Question existante : aucune surprise (v12.0 et LOT-1)

**Validation** : HUMAIN — **Issues** : #309

**Objectif** : S'assurer qu'aucune question déjà saisie ne change de roue.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer une question ROUE v12.0 (avant la mise à jour) | Roue globale identique à celle de v12.0 (22 cases par défaut) | | |
| 2 | Lancer une question créée au LOT-1 avec des BONUS | Même roue qu'au LOT-1 : ses BONUS, le reste suit le global | | |
| 3 | Rouvrir ces deux questions dans l'éditeur | Question v12.0 : tout 🔗 ; question LOT-1 : seul BONUS personnalisé | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 8 — Composition devenue invalide après un changement des Paramètres

**Validation** : HUMAIN — **Issues** : #309

**Objectif** : Vérifier le signalement et la sécurité du départ.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Question D : pas personnalisé à 400 (maximum global 1000) ; enregistrer | Enregistrée | | |
| 2 | Paramètres : maximum à 400 ; enregistrer | Les Paramètres ne revalident pas les questions | | |
| 3 | Rouvrir D dans l'éditeur | Message « Cette roue n'est plus valable avec les réglages globaux actuels… la roue globale sera utilisée » ; **Ajouter** grisé tant que non corrigé | | |
| 4 | Lancer D en jeu | La manche démarre normalement (jamais refusée) avec la roue globale | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Criteres de Validation

- [ ] Scénarios AUTO rejoués par `qa`
- [ ] Scénarios HUMAIN passent sur QUALIF
- [ ] Aucune régression : tests ROUE v12.0 et LOT-1 (`*_144_*`, `*_145_*`, `*_308_*`, `*_147_*`) verts

## Notes QA

Une valeur personnalisée égale au global reste personnalisée (elle ne suit plus le global) ; seul 🔗 la fait suivre à nouveau.
