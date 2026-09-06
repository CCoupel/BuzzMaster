# Procédure de Test — Entracte programmée (#214)

**Version** : 9.0.0.x (milestone v9.0.0, Batch 3)
**Date** : 2026-09-04
**Issue** : #214
**Maquette** : `docs/mockups/entracte-programme-214.html`
**Testeur** : QA / Utilisateur (validation manuelle obligatoire — jamais exécuté par `qa`/`deployer`)

---

## Contexte

#214 n'introduit **pas** un nouveau mécanisme — c'est un **second déclencheur** du mécanisme
ENTRACTE existant (#119) : en plus du bouton manuel de la navbar (configuration globale, inchangé),
une entrée du déroulé de type ENTRACTE peut désormais déclencher une pause avec **sa propre**
configuration (titre/sous-titre/image), le temps de son passage. Panneau, transition, LEDs et
estompage des 4 surfaces sont réutilisés tels quels. **À la sortie, la configuration globale reprend
la main automatiquement** — sans quoi le bouton manuel afficherait le texte de la dernière pause
programmée.

---

## Prérequis

- [ ] Environnement : QUALIF ou LOCAL, serveur démarré
- [ ] Accès admin (`/admin/quiz`, onglet Questions) et animateur (`/anim`)
- [ ] Une configuration d'entracte globale déjà enregistrée, **différente** de celle qu'on va donner
      à l'entrée programmée (ex. globale = « ENTRACTE » / « Retour dans 20mn », programmée =
      « PAUSE DÉJEUNER » / « Retour à 14h00 ») — condition nécessaire pour distinguer visuellement
      les deux sources

---

## Scénario 1 — Créer une entrée ENTRACTE avec sa propre configuration

**Objectif** : Vérifier que chaque occurrence porte sa propre configuration éditable.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur `/admin/quiz`, créer une nouvelle question, choisir le type **Entracte** | Type sélectionnable dans le sélecteur de type, éditeur ENTRACTE affiché | | |
| 2 | Saisir un titre (« PAUSE DÉJEUNER »), un sous-titre (« Retour à 14h00 »), uploader une image différente de l'image globale | Champs modifiables, aperçu mis à jour | | |
| 3 | Enregistrer | Question créée, apparaît dans le déroulé avec le badge de type « Entracte » | | |
| 4 | Vérifier que l'éditeur réutilise la même structure que le formulaire d'entracte global (Backstage) | Mêmes champs (titre/sous-titre/image/taille panneau/animation), présentation cohérente | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Jouer l'entrée dans le déroulé (PREPARE → READY → START)

**Objectif** : Vérifier le cycle standard, sans buzzer/équipe/minuteur, et l'affichage TV/VJoueur.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Depuis `/admin`, sélectionner l'entrée ENTRACTE | Rien de spécial encore affiché sur TV/VJoueur | | |
| 2 | Observer le cycle PREPARE → READY | Aucune sélection d'équipe requise, aucun buzzer mobilisé, cycle identique à n'importe quelle autre question | | |
| 3 | Cliquer START | Le panneau apparaît en fondu sur TV **et** VJoueur, avec le titre/sous-titre/image de **cette occurrence** (« PAUSE DÉJEUNER » / « Retour à 14h00 »), PAS la config globale | | |
| 4 | Observer la régie et l'interface animateur | Estompées, comme pour l'entracte manuel | | |
| 5 | Observer les LEDs des buzzers physiques (si disponibles) | Éteintes | | |
| 6 | Tenter une action de jeu quelconque pendant la pause (ex. buzz physique) | Refusée, sans effet | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Sortir de la pause programmée

**Objectif** : Vérifier que le geste de sortie fonctionne (jamais d'entracte sans issue) et le retour
à l'affichage normal.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Depuis la navbar, cliquer le bouton de sortie d'entracte (même geste que pour sortir d'un entracte manuel) | La pause se termine, retour à l'écran d'attente/déroulé sur TV et VJoueur | | |
| 2 | Vérifier qu'aucun blocage n'a eu lieu (le geste a bien fonctionné du premier coup) | Pas de pause bloquée sans moyen d'en sortir | | |
| 3 | Poursuivre le déroulé normalement (sélectionner la question suivante) | Fonctionnement normal, rien de cassé après la pause | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Le bouton manuel retrouve la configuration globale (règle de restauration)

**Objectif** : **LE scénario central de #214** — vérifier qu'une pause programmée ne laisse pas son
texte derrière elle.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Juste après le Scénario 3 (sortie de la pause programmée), déclencher l'entracte **manuel** depuis le bouton de la navbar | Le panneau affiche la configuration **globale** (« ENTRACTE » / « Retour dans 20mn »), **PAS** « PAUSE DÉJEUNER » / « Retour à 14h00 » | | |
| 2 | Sortir de cet entracte manuel | Retour normal | | |
| 3 | Aller sur `/admin/backstage` → onglet Entracte, vérifier le formulaire de configuration globale | Affiche toujours « ENTRACTE » / « Retour dans 20mn » — jamais écrasé par la pause programmée | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Aucun point comptabilisé, absente du palmarès et du « à suivre »

**Objectif** : Vérifier que l'entrée ENTRACTE n'est pas traitée comme une question à score.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Noter le score des équipes avant de jouer l'entrée ENTRACTE | Score de référence noté | | |
| 2 | Jouer l'entrée ENTRACTE en entier (Scénarios 2-3) | — | | |
| 3 | Vérifier le score des équipes après | **Identique** au score de référence — aucun point attribué | | |
| 4 | Consulter le palmarès (`/palmares`) | L'entrée ENTRACTE n'y apparaît pas | | |
| 5 | Sur `/admin`, observer le bouton « à suivre » (question suivante) quand l'entrée ENTRACTE est juste après la question en cours | L'entrée ENTRACTE est ignorée par le raccourci « à suivre » (il pointe vers la prochaine vraie question, pas vers l'entracte) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 6 — Non nestable en carte MEMOTION

**Objectif** : Vérifier qu'ENTRACTE n'est pas proposé comme type imbricable.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Éditer une carte MEMOTION, ouvrir le sélecteur de type de carte | Le type « Entracte » n'apparaît **pas** dans la liste des types imbricables | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 7 — Non-régression : l'entracte manuel global reste inchangé

**Objectif** : Vérifier qu'aucune régression n'a été introduite sur le mécanisme existant.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sans jamais avoir joué d'entrée ENTRACTE dans cette session, déclencher l'entracte manuel depuis la navbar | Comportement identique à avant #214 : configuration globale, panneau, transition, estompage, LEDs | | |
| 2 | Modifier la configuration globale pendant que l'entracte manuel est actif | Enregistrement réussi, panneau affiché **inchangé** jusqu'à la fin de la pause (gel de configuration C4, inchangé) | | |
| 3 | Sortir | Retour normal | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation

- [ ] Chaque occurrence ENTRACTE porte sa propre configuration, éditable et distincte de la globale
- [ ] Cycle PREPARE→READY→START standard, ni buzzer ni équipe ni minuteur
- [ ] Sortie fonctionnelle du premier coup, jamais de pause bloquée
- [ ] **Configuration globale restaurée automatiquement** après une pause programmée (Scénario 4)
- [ ] Aucun point, absente du palmarès et du « à suivre »
- [ ] Non imbricable en carte MEMOTION
- [ ] Entracte manuel global non régressé

---

## Notes QA

[Espace pour observations, captures d'écran, anomalies constatées]
