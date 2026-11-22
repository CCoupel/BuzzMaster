# Procedure de Test — BONUS posé sur une case à points, éphémère ou permanent

**Version** : 12.2.0 (LOT-1)
**Date** : 2026-11-22
**Testeur** : QA (scénarios AUTO) · utilisateur (scénarios HUMAIN)
**Issues** : #316 — BONUS sur une case à points · #317 — BONUS éphémère / permanent
**Références** : `contracts/roue.md` §17 (rév. 9.1), `contracts/wheel-component.md` §9 ; maquettes
`docs/mockup/v12.2.0/ui/{tv_roue_started__bonus_on_value,admin_question_editor__roue_bonus_mode}.html`
**Remplace** : `roue-bonus-308.md` (v12.1 : le BONUS était une case ajoutée, la main était rendue ; plus vrai en v12.2.0).

> Un BONUS est une **marque 🎁 posée sur une case à points** : le nombre de cases ne change pas. Arrêt sur une case
> marquée = tour normal au montant de la case, **sans relance**. Le cadeau reste géré **hors système** par l'animateur.
> Marque à l'écran : case **dorée + ruban**, **🎁 seul** — ni montant ni mot « BONUS » sur la case ; le montant est sur
> le bandeau « BONUS ! case 800 ». BANQUEROUTE, PERDU et les valeurs sont **identiques à v12.1**.
> Les scénarios AUTO sont couverts par des tests automatisés que `qa` joue ; les HUMAIN exigent un jugement visuel ou sonore.

## Prerequis

- [ ] Environnement : QUALIF (ou LOCAL), serveur BuzzControl 12.2.0.x
- [ ] Données : une question ROUE à phrase valide (ex. « Combien pour ce chien dans la vitrine »), 2 équipes avec buzzers,
      roue par défaut 1000 / 50 / 2 / 1 ; une question créée en v12.1 avec 2 BONUS (`ROUE_BONUS_COUNT = 2`, sans mode) si disponible
- [ ] Accès : admin (`/questions`, `/game`, Paramètres), animateur (tablette), TV (`/tv`), une tablette joueur

## Scenarios

### Scenario 1 — Génération : cases, BANQUEROUTE / PERDU et marques (propriétés)

**Validation** : AUTO — **Issues** : #316

**Objectif** : `N = V + B + P` cases (les BONUS ne comptent plus) ; valeurs, BANQUEROUTE et PERDU identiques à v12.1 ; `X` marques
exactement, sur des cases à points, jamais voisines d'une case spéciale ni d'une autre marque ; au plus `⌊(g−1)/2⌋` par
intervalle ; répartition `g / (b+1)` ; tirage uniforme dans l'intervalle ; case maximum marquable sans BANQUEROUTE ; `X = 0` = v12.1.
**Tests automatisés** : `internal/game/roue_wheel_316_test.go` (`ZeroBonusIsV120Wheel`, `Properties`, `NeverFailsAtTheMaximumWithAdversarialRandom`,
`HomogeneousDistributionAcrossIntervals`, `PositionUniformInsideTheLargestInterval`, `NoSpecialCasesFirstBonusAnywhereMaxCaseAllowed`,
`MaxCaseNeverMarkedWhenBankruptNextToIt`, `ExampleSingleValidPlace`, `MaxBonusTable`).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `go test ./internal/game -run 'RoueWheel316'` | Tous verts | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 2 — Départ de manche : réduction, repli, migration

**Validation** : AUTO — **Issues** : #316

**Objectif** : au `START`, jamais de refus ; BONUS réduits au maximum permis ; roue invalide → globale → défaut ; une question v12.1 se joue
(ses BONUS sont posés sur des cases à points) ; la roue perd les cases que les BONUS ajoutaient.
**Tests automatisés** : `internal/game/roue_bonus_316_test.go` (`StartKeepsCaseCountAndSpecials`, `ReducedAtStartNeverRefused`, `NoRoomMeansNoMarkAndStillStarts`,
`V121QuestionMigrates`, `MarksFollowTheInjectedRandomSource`), `roue_wheel_316_test.go` (`EffectiveParams*`).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `go test ./internal/game -run 'RoueBonus316_(Start|Reduced|NoRoom|V121|Marks)|RoueWheel316_Effective'` | Tous verts | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 3 — Arrêt sur une case marquée (moteur)

**Validation** : AUTO — **Issues** : #316

**Objectif** : fin du freinage sur une case marquée : sous-phase LETTER, événement `BONUS` (VALUE = valeur de la case, à la place de
`LAND_VALUE`), pas de relance, cagnottes et scores inchangés, une seule notice « bonne réponse » + comète ; consonne au montant de la case ;
voyelle après l'arrêt si la question le permet ; BANQUEROUTE / PERDU inchangés.
**Tests automatisés** : `internal/game/roue_bonus_316_test.go` (`StopOnMarkedCase*`, `ConsonantAfterMarkedStopPaysTheCaseValue`, `VowelAfterMarkedStop`,
`NoRespinAfterAMarkedStop`, `NoticeBonusOnceAndOnlyOnMarkedCases`, `SpecialsKeepV120EffectOnAMarkedWheel`, `SegmentJSONAlwaysCarriesBONUS`).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `go test ./internal/game -run 'RoueBonus316'` | Tous verts | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 4 — Mode éphémère / permanent (moteur)

**Validation** : AUTO — **Issues** : #317

**Objectif** : mode effectif copié au `START` (question > global > éphémère) et remis à vide hors manche ; éphémère : la première action
acceptée qui quitte LETTER (consonne présente / absente / déjà proposée, voyelle) retire la marque de la case jouée, une action refusée
ne consomme rien ; la case redevient une case à points normale ; permanent : la marque reste et un nouvel arrêt redonne un BONUS.
**Tests automatisés** : `internal/game/roue_bonus_mode_317_test.go`.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `go test ./internal/game -run 'RoueMode317'` | Tous verts | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 5 — HTTP : enregistrement, refus, aperçu, configuration globale

**Validation** : AUTO — **Issues** : #316 #317

**Objectif** : `roue_bonus_mode` (absent / vide = global, `ROUE_BONUS_MODE_INVALID`) ; `ROUE_BONUS_COUNT_INVALID` au-dessus du maximum
(BONUS envoyé ou hérité), `ROUE_WHEEL_INVALID` d'abord ; `wheel-preview` : `COUNT = N`, marques d'un tirage, refus sans réduction ;
`game-config.json` : `bonus_mode` (défaut éphémère), `bonus_count` borné, une roue sans BONUS valable en v12.1 le reste.
**Tests automatisés** : `internal/server/roue_bonus_316_test.go` (`RoueBonus316_*`, `RoueBonus317_*`).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `go test ./internal/server -run 'RoueBonus31[67]'` | Tous verts | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 6 — Écrans : marque, fondu, annonces, interrupteur du mode (composants)

**Validation** : AUTO — **Issues** : #316 #317

**Objectif** : case marquée = doré + ruban + 🎁 seul (montant masqué, `aria-label` « 800, BONUS ») en bandeau et en arc ; retrait par
fondu sans recréer la roue (mêmes nœuds, même position) ; bandeau du BONUS (fondu en éphémère seulement) ; interrupteur « une seule fois
par manche » sous le compteur (🔗/✏️, grisé à 0) dans l'éditeur et la carte des Paramètres ; champ `roue_bonus_mode` envoyé ou retiré.
**Tests automatisés** : `web/src/components/roue/RoueBonusMark.316.test.jsx`, `RoueBonusMode.317.test.jsx`, `web/src/pages/QuestionsPage.roueBonusMode.317.test.jsx`.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `cd server-go/web && npx vitest run src/components/roue/RoueBonusMark.316.test.jsx src/components/roue/RoueBonusMode.317.test.jsx src/pages/QuestionsPage.roueBonusMode.317.test.jsx` | Tous verts | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 7 — Répartition des BONUS sur trois roues, BANQUEROUTE / PERDU à leurs places v12.1

**Validation** : HUMAIN — **Issues** : #316

**Objectif** : juger sur des roues réelles que les marques sont bien réparties (« homogène ») et que BANQUEROUTE / PERDU n'ont pas bougé.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Éditeur, question ROUE : personnaliser la composition = roue par défaut 1000 / 50 / 2 / 1, BONUS = 4 | Aperçu à 22 cases, 4 cases 🎁 | | |
| 2 | Cliquer « 🎲 Autre tirage » 5 fois | BANQUEROUTE et PERDU **toujours aux mêmes cases** (1000 · BANQ · 950 · 50 · 900 · 100 · 850 · PERDU …) ; les 🎁 changent de place ; jamais deux 🎁 côte à côte ; jamais un 🎁 contre une BANQUEROUTE ou un PERDU ; répartis le long de la roue (pas tous du même côté) | | |
| 3 | Composition 1000 / 100 / 2 / 1 (12 cases) | Au plus **2** BONUS acceptés ; avec 2, ils sont éloignés l'un de l'autre | | |
| 4 | Composition 100 / 10 / **0** / **0** (sans BANQUEROUTE ni PERDU), BONUS = 3 | 10 cases ; 3 🎁 jamais voisins ; la case 100 (la plus forte) peut en porter un sur certains tirages | | |
| 5 | Comparer la roue par défaut avec le contrat `roue.md` §10.1 / une manche v12.1 | Mêmes valeurs, mêmes BANQUEROUTE / PERDU qu'en v12.1 | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 8 — Marque visible sur tous les écrans : 🎁 seul sur la case, montant sur le bandeau

**Validation** : HUMAIN — **Issues** : #316

**Objectif** : vérification visuelle de la décision « logo 🎁 seul, ni montant ni mot BONUS sur la case ».

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer une manche (2 BONUS, mode quelconque) ; regarder la TV en bandeau, puis en arc | Les cases marquées sont **dorées avec un ruban cadeau** et **uniquement 🎁** au centre — **ni montant, ni texte « BONUS »** sur la case | | |
| 2 | Même contrôle sur la tablette joueur, la tablette animateur, la page admin de jeu | Même rendu (🎁 seul, doré + ruban) | | |
| 3 | Aperçu de l'éditeur et aperçu de la carte Paramètres | 🎁 seul sur la case ; le montant apparaît au survol (aperçus admin seulement) | | |
| 4 | Amener la roue sur une case marquée (arrêt manuel ou simulé par l'admin) | Bandeau plein écran « BONUS ! case **800** » : **le montant est sur le bandeau**, pas sur la case | | |
| 5 | Contrôler la TV | Aucun défilement, tout reste dans l'écran (TV statique) | | |
| 6 | Inspecter la case (outils du navigateur) : `aria-label` de la roue | « Segment : 800, BONUS » (montant annoncé aux lecteurs d'écran) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 9 — Arrêt sur une case marquée : bandeau, son, comète, consonne au montant, pas de relance

**Validation** : HUMAIN — **Issues** : #316

**Objectif** : juger l'expérience de jeu (son, lumière, lisibilité) à l'arrêt sur un BONUS.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer puis arrêter la roue sur une case marquée | Bandeau « BONUS ! case N » ; son « bonne réponse » ; comète sur le buzzer de l'équipe, **ni flash ni pulsation** | | |
| 2 | Lire l'annonce | « 🎁 BONUS — case N : proposez une consonne » (plus de « rejouent ») | | |
| 3 | Vérifier les commandes admin / animateur | **Pas de bouton « Lancer la roue »** : on propose une consonne (ou une voyelle après l'arrêt si la question le permet) | | |
| 4 | Proposer une consonne présente | Gain = nombre d'occurrences × montant de la case ; cagnotte créditée normalement | | |
| 5 | Vérifier les scores de la partie | Aucun point n'est attribué au BONUS lui-même ; le cadeau est remis par l'animateur | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 10 — Mode éphémère : fondu de la case et du bandeau, case normale ensuite

**Validation** : HUMAIN — **Issues** : #317

**Objectif** : juger le fondu (≈ 600 ms) et la fluidité : la roue ne saute pas, ne se recrée pas.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Réglage éphémère (défaut) ; arrêt sur une case marquée | Case dorée avec 🎁 | | |
| 2 | Choisir la consonne (présente puis, sur un autre essai, absente) | Le doré, le ruban et le 🎁 **s'effacent en fondu**, la case reprend **sa couleur habituelle** et **son montant réapparaît** ; le bandeau s'efface par le même fondu ; la roue ne bouge pas | | |
| 3 | Refaire un tour et s'arrêter sur la même case | Tour **normal** (pas de bandeau BONUS) ; la case reste normale jusqu'à la fin de la manche | | |
| 4 | Vérifier les autres cases marquées | Elles restent dorées et jouables | | |
| 5 | Système avec « réduire les animations » activé | Le retrait est immédiat, sans fondu | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 11 — Mode permanent : la marque reste, un second BONUS est possible

**Validation** : HUMAIN — **Issues** : #317

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Paramètres → carte Roue : désactiver « une seule fois par manche » (permanent), Enregistrer | Réglage enregistré, relu après rechargement | | |
| 2 | Arrêt sur une case marquée puis choix de la consonne | **La marque reste** (aucun fondu sur la case) ; le bandeau est retiré tout de suite | | |
| 3 | Refaire un tour jusqu'à s'arrêter sur la même case | Un **nouveau BONUS** est déclenché (bandeau, son, comète) | | |
| 4 | Question personnalisée : ✏️ du mode = éphémère dans l'éditeur alors que le global est permanent | Cette question se joue en éphémère ; les autres restent permanentes | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 12 — Réglage du mode : Paramètres et éditeur (interrupteur sous le compteur)

**Validation** : HUMAIN — **Issues** : #317

**Objectif** : juger l'ergonomie retenue (maquette B : option du compteur BONUS, pas un 6ᵉ paramètre).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Paramètres → carte Roue | Interrupteur « une seule fois par manche » **sous le compteur BONUS**, activé par défaut (éphémère) | | |
| 2 | Mettre le compteur BONUS à 0 | L'interrupteur est grisé | | |
| 3 | Éditeur : question qui suit le global | Interrupteur lié (🔗) et non modifiable ; ✏️ pour personnaliser, 🔗 pour revenir au global | | |
| 4 | Enregistrer une question avec un mode personnalisé, la rouvrir | Mode conservé ; en revenant au global (🔗) puis enregistrer, le mode est retiré | | |
| 5 | Question à 0 BONUS (personnalisé) | Interrupteur grisé | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 13 — Trop de BONUS : refus à l'enregistrement, réduction au départ

**Validation** : HUMAIN — **Issues** : #316

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Éditeur : composition 1000 / 100 / 2 / 1, BONUS = 4 | Message de refus « Trop de BONUS : 2 au plus pour cette roue (…) » lisible ; la question ne peut pas être enregistrée | | |
| 2 | Paramètres → carte Roue : mêmes valeurs | Même refus, **Enregistrer désactivé**, rien n'est écrit | | |
| 3 | Revenir à 2 BONUS | Enregistrement possible | | |
| 4 | Jouer une question v12.1 dont le nombre de BONUS dépasse désormais le maximum | Le départ n'est **jamais refusé** : les BONUS sont réduits (journal serveur) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 14 — Question v12.1 migrée ; nouveaux emplacements à la manche suivante

**Validation** : HUMAIN — **Issues** : #316 #317

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir dans l'éditeur une question v12.1 avec 2 BONUS | Information de migration lisible (BONUS désormais posés sur des cases à points, roue plus courte) ; la question s'enregistre sans erreur | | |
| 2 | Jouer cette question | Elle se joue ; 2 cases marquées ; mode = réglage global (éphémère par défaut) | | |
| 3 | Relancer la même question (manche suivante) | Les 🎁 sont à **de nouvelles places** ; BANQUEROUTE et PERDU **n'ont pas bougé** | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Criteres de Validation

- [ ] Tous les scénarios AUTO passent (Go + vitest) ; le raccord Go → hook (fixture `roue-gamestate.312.json`) est couvert au Batch 2
- [ ] Marque lisible sur tous les écrans : 🎁 seul, montant sur le bandeau
- [ ] Éphémère : fondu sans saut de la roue ; permanent : marque conservée
- [ ] BANQUEROUTE / PERDU identiques à v12.1, aucune régression sur l'achat de voyelles ni sur les autres types de cases

## Notes QA

[Espace pour observations]
