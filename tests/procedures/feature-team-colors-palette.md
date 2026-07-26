# Procédure de Test — Palette de 16 couleurs d'équipe (#113)

**Version** : à définir (branche `feature/team-colors-palette`)
**Date** : 2026-07-26
**Branche** : feature/team-colors-palette
**Testeur** : QA

---

## Contexte de la Feature

La palette de couleurs d'équipe passe de 8 à 16 : 8 teintes déclinées chacune en un **ton vif**
(S=100 % L=55 %) et un **ton profond** (S=100 % L=35 %). L'attribution automatique à la création
d'équipe épuise les 8 tons vifs (rangs 1-8) avant d'utiliser les tons profonds (rangs 9-16), et
recycle au rang 1 au-delà de 16 équipes.

Le champ `COLOR_NAME` (contrat `Team.COLOR_NAME`, voir `contracts/models.md`) est désormais
renseigné à chaque attribution/sélection de couleur, ce qui permet au serveur de calculer la
couleur LED **exacte** du buzzer physique au lieu d'une approximation par teinte. Un palier
d'atténuation LED relatif au ton (`dimIntensityFor`) évite qu'un ton profond ne paraisse quasi
éteint en état atténué (25 % d'intensité classique).

**Référence contractuelle** : `contracts/models.md` § "Palette d'équipes (#113)" (table des 16
couleurs) et la maquette de validation `_work/mockups/113-team-colors-palette.html` — c'est sur
cette maquette que se juge la conformité visuelle de la livraison.

**Aucun changement firmware.** Le mécanisme d'intensité multiplicative existe déjà côté buzzer.

---

## Prérequis

- [ ] Environnement : QUALIF (ou LOCAL avec build de la branche `feature/team-colors-palette`)
- [ ] Au moins 2 buzzers physiques BuzzClick disponibles (pour le Scénario 3)
- [ ] Un affichage TV (`/tv`) accessible en parallèle de l'admin
- [ ] Un backup (fichier de configuration/équipes) créé **avant** cette feature, pour le
      Scénario 4 (non-régression)
- [ ] Accès à `TeamsPage`, `PlayerDisplay`/`/tv`, PALMARES (`CategoryPalmaresPage`)

---

## Scénario 1 — Création de 16 équipes : ordre d'attribution

**Objectif** : Vérifier que la création successive de 16 équipes attribue les 16 couleurs de la
palette dans l'ordre du rang (8 tons vifs d'abord, puis 8 tons profonds), sans doublon, et que le
sélecteur reflète correctement l'état "déjà prise".

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur `TeamsPage`, créer une 1ère équipe | Couleur attribuée = Rouge `[255,26,26]` (rang 1) | | |
| 2 | Créer 7 équipes supplémentaires (total 8) | Chaque équipe reçoit un ton vif différent, dans l'ordre : Orange, Jaune, Vert, Cyan, Bleu, Violet, Rose (rangs 2-8) | | |
| 3 | Créer une 9ᵉ équipe | Couleur attribuée = Grenat (rouge-profond, rang 9) — 1er ton profond | | |
| 4 | Créer 7 équipes supplémentaires (total 16) | Rangs 10-16 attribués dans l'ordre : Ambre, Or, Émeraude, Turquoise, Marine, Indigo, Magenta | | |
| 5 | Comparer visuellement l'ensemble des 16 couleurs à `_work/mockups/113-team-colors-palette.html` § "01 — La palette" | Correspondance exacte (nom, RGB visuel) | | |
| 6 | Créer une 17ᵉ équipe | La couleur recycle le rang 1 (Rouge), sans erreur ni doublon bloquant | | |
| 7 | Ouvrir le sélecteur de couleur d'une équipe existante | 2 rangées de 8 pastilles (tons vifs en haut, tons profonds en bas), conforme à la maquette § "05 — Sélecteur" | | |
| 8 | Observer les pastilles portées par d'autres équipes | Hachurées/marquées "déjà prise", mais restent cliquables | | |
| 9 | Cliquer une pastille "déjà prise" | La couleur est appliquée normalement (2 équipes peuvent partager la même couleur) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Affichage TV avec 16 équipes

**Objectif** : Vérifier la lisibilité de l'écran TV avec 16 équipes actives, sans défilement
(contrainte d'affichage statique du projet).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Avec les 16 équipes du Scénario 1 actives, ouvrir `/tv` | Les 16 badges/barres de score s'affichent | | |
| 2 | Observer l'intégralité de l'écran | Aucun défilement (`overflow: hidden` respecté), contenu tient dans le viewport | | |
| 3 | Comparer avec la maquette § "03 — Affichage TV" (fond sombre, 16 badges + 16 barres) | Les 16 couleurs restent distinctes et lisibles à distance, y compris les paires vif/profond d'une même teinte | | |
| 4 | Faire varier les scores (buzz, attribution de points) | Les barres/couleurs se mettent à jour normalement, aucune régression d'affichage | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Buzzers physiques : ton vif et ton profond, pleine intensité et atténuée

**Objectif** : Valider sur buzzer réel (pas en simulation) que la LED reproduit exactement la
couleur d'équipe, y compris en ton profond et en état atténué — le point explicitement signalé
comme "à valider en test réel" par la maquette § "04 — Buzzers physiques".

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Assigner un buzzer physique à une équipe en ton **vif** (ex. Bleu `[26,94,255]`) | L'anneau LED affiche un bleu vif correspondant à l'écran | | |
| 2 | Assigner un second buzzer physique à une équipe en ton **profond** (ex. Bleu-profond/Marine `[0,54,179]`) | L'anneau LED affiche un bleu plus sombre, distinct du ton vif, correspondant à l'écran | | |
| 3 | Démarrer une question NORMAL, ne pas faire buzzer ces 2 équipes (état "non buzzé" = atténué) | Les 2 anneaux passent en intensité atténuée | | |
| 4 | Observer le buzzer en ton **vif** atténué | Intensité ≈ 64/255 (comportement inchangé par rapport à avant #113) | | |
| 5 | Observer le buzzer en ton **profond** atténué | **Reste visiblement allumé** (intensité relevée à ≈100/255) — ne doit **pas** paraître quasi éteint | | |
| 6 | Comparer les 2 anneaux atténués côte à côte | La pénombre perçue est équivalente entre les deux tons (c'est le critère qualitatif à juger en salle éclairée) | | |
| 7 | Faire buzzer chaque équipe à son tour | Retour à pleine intensité (BLINK/SOLID), couleur inchangée | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Non-régression : backup antérieur à la feature

**Objectif** : Vérifier qu'un backup/état d'équipes créé avant #113 (sans `COLOR_NAME`) reste
chargeable et jouable normalement, sans migration.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Charger un backup créé avant cette feature (équipes avec `COLOR` mais sans `COLOR_NAME`) | Chargement réussi, aucune erreur | | |
| 2 | Observer les couleurs d'équipe sur `TeamsPage` | Couleurs affichées normalement (via le repli par teinte si nécessaire) | | |
| 3 | Assigner un buzzer physique à une de ces équipes legacy | La LED s'allume dans une couleur cohérente avec l'écran (résolution par approximation de teinte, pas de gris) | | |
| 4 | Jouer une partie complète avec ces équipes (buzz, scores, QCM) | Comportement normal, aucun blocage lié à l'absence de `COLOR_NAME` | | |
| 5 | Modifier la couleur d'une équipe legacy via le sélecteur | `COLOR_NAME` est désormais renseigné ; la LED devient exacte à partir de ce moment | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Cohérence de rendu multi-écrans (non-régression)

**Objectif** : Vérifier qu'une même couleur d'équipe est rendue à l'identique sur l'admin, la TV,
le podium, le PALMARES et l'écran VJoueur (critère d'acceptation du plan).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Choisir une équipe en ton profond (ex. Émeraude `vert-profond`) | — | | |
| 2 | Comparer sa couleur sur `TeamsPage` (badge équipe), `/tv` (badge/barre), le podium de fin de partie, `CategoryPalmaresPage` (PALMARES) et l'écran VJoueur d'un de ses joueurs | La même teinte/luminosité est perçue sur les 5 écrans — aucune divergence visible | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation

- [ ] Scénario 1 : 16 couleurs attribuées dans l'ordre du rang, sélecteur conforme (2×8, "déjà prise" cliquable)
- [ ] Scénario 2 : TV avec 16 équipes lisible, sans défilement
- [ ] Scénario 3 : LED buzzer exacte (vif et profond), ton profond visible en état atténué
- [ ] Scénario 4 : backup antérieur à la feature chargeable et jouable sans migration
- [ ] Scénario 5 : rendu identique sur admin/TV/podium/PALMARES/VJoueur pour une même couleur

## Notes QA

[Espace pour observations, capture d'écran, couleurs exactes observées sur buzzer réel (photo
recommandée pour le Scénario 3), version du binaire testé, date de test]
