# Procédure de Test — Carte MEMOTION polymorphe : verrou de type + non-régression (#184)

**Version** : v7.0.0 (branche `milestone/v7.0.0`)
**Date** : 2026-08-21
**Testeur** : QA
**Issue** : #184 — `MotionCard.TYPE`, verrou de type (contrat §3.2), contexte d'hôte normalisé, barème
`POINTS_RULE`. Aucune carte non-SPEEDY n'est encore **jouable** en v7.0.0 (QCM en carte arrive en
#185) : cette procédure couvre uniquement ce qui est observable dès #184 — l'éditeur (`QuestionsPage.jsx`).
**Référence** : `contracts/question-types.md` §3.1-3.2 (verrou), maquette
`docs/mockups/memotion-card-type-selector-184.html`, plan `_work/reports/plan-memotion-v700-20260821.md`

---

## ⚠️ Prérequis obligatoire — non-régression #160

**Avant toute validation de cette procédure, rejouer intégralement `tests/procedures/anim-memotion-160.md`,
SANS AUCUNE MODIFICATION de ce fichier ni de ses scénarios.** C'est un critère de fait explicite du
plan #184 : « la procédure QA MEMOTION de #160 passe sans modification ». Un scénario de #160 qui ne
passerait plus, ou qui nécessiterait d'être modifié pour passer, est une régression de #184 à
signaler immédiatement — ne pas tenter de le "réparer" en adaptant #160.

Cette procédure-ci (#184) ne remplace pas #160, elle s'y **ajoute**.

---

## Prérequis

- [ ] Environnement : QUALIF ou LOCAL
- [ ] Au moins une des 9 questions MEMOTION existantes du dépôt (`data/files/questions/{14,15,16,36,
      37,38,83,84,85}`) accessible dans l'éditeur `/admin` → Questions
- [ ] Droit de créer/modifier une question MEMOTION

---

## Scénario 1 — Carte neuve : sélecteur de type déverrouillé, verrouillage réactif

**Objectif** : Vérifier le prédicat exact du contrat §3.2 — écart à la valeur de création d'un
`OwnedField`, pas la non-nullité (piège documenté : `DIFFICULTY` vaut `1` à la création, ne doit
**pas** compter comme "renseigné").

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Créer une nouvelle question MEMOTION, ajouter une carte | Sélecteur de type **cliquable** (SPEEDY/QCM proposés — pas ARDOISE/MEMORY, non nestables en v7.0.0), aucune raison de verrou affichée | | |
| 2 | Renseigner uniquement le thème (RECTO_THEME) de la carte | Sélecteur **toujours cliquable** — le thème appartient à la carte, pas au type (contrat §3.1) | | |
| 3 | Déplacer la difficulté sur ★★ (tous les autres champs vides) | Sélecteur **toujours cliquable** — la difficulté ne verrouille jamais | | |
| 4 | Ramener la difficulté sur ★ | Sélecteur toujours déverrouillé (aucun effet, elle ne verrouillait déjà pas) | | |
| 5 | Sélectionner le type QCM sur cette carte, ne rien saisir dans le sous-éditeur QCM | Sélecteur **toujours cliquable** — les seuils/pénalités par défaut (0.25/0.125, 0.67/0.33) ne comptent pas comme "renseignés" (piège du contrat §3.2, prédicat sur l'écart à la valeur de création) | | |
| 6 | Saisir une réponse à la case ROUGE du sous-éditeur QCM | Sélecteur se **désactive**, une raison est affichée à côté (texte orienté vers le geste utile : supprimer la carte et la recréer) | | |
| 7 | Vider à nouveau cette réponse ROUGE (tout redevient vide) | Sélecteur se **réactive** — le verrou est réactif (contrat §3.2, cas limite "OwnedFields ramenés à leurs valeurs de création") | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Carte SPEEDY : verrou sur ANSWER_TEXT/ANSWER_IMAGE

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Nouvelle carte, type SPEEDY (par défaut), saisir énoncé (QUESTION_TEXT) et image d'énoncé | Sélecteur **toujours cliquable** — QUESTION_TEXT/QUESTION_IMAGE appartiennent à la carte, pas au type | | |
| 2 | Saisir un texte de réponse (ANSWER_TEXT) | Sélecteur se **désactive**, raison mentionnant la face RÉPONSE | | |
| 3 | Vider le texte de réponse | Sélecteur se réactive | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Cartes existantes : les 9 questions MEMOTION historiques s'ouvrent verrouillées sur SPEEDY

**Objectif** : Non-régression stricte — contrat §3.2 : « les 9 questions MEMOTION existantes
s'ouvrent verrouillées sur SPEEDY... et se réenregistrent sans erreur ».

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir une question MEMOTION existante (ex: `data/files/questions/14`) dans l'éditeur | Chaque carte affiche le type **SPEEDY** sélectionné, sélecteur **désactivé** (ANSWER_TEXT déjà renseigné) | | |
| 2 | Répéter pour au moins 2 autres questions parmi `{15, 16, 36, 37, 38, 83, 84, 85}` | Même constat : SPEEDY verrouillé sur chacune | | |
| 3 | Sans rien modifier, enregistrer la question | Enregistrement **sans erreur**, aucun message bloquant | | |
| 4 | Rouvrir la question après enregistrement | Contenu identique à avant (thèmes, énoncés, réponses, difficulté) — aucune migration silencieuse | | |
| 5 | Modifier un champ de carte (ex: l'énoncé) sur une carte verrouillée, enregistrer | Enregistrement accepté (le verrou porte sur le **type**, pas sur les autres champs) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Sous-éditeur QCM en carte : affichage seul, pas encore jouable (#185)

**Objectif** : Vérifier que #184 livre l'édition sans ouvrir de jeu prématuré — QCM en carte n'est
**pas jouable** avant #185.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Nouvelle carte MEMOTION, sélectionner QCM | Sous-éditeur QCM monté en face VERSO (4 réponses colorées, désignation de la bonne réponse, seuils d'indices) — face REVEAL **absente** (pas de contenu propre, la réponse est l'une des 4 propositions, contrat §3 maquette point 2) | | |
| 2 | Remplir les 4 réponses + désigner la bonne, enregistrer | Enregistrement accepté | | |
| 3 | Lancer une manche incluant cette carte QCM depuis `/anim`, sélectionner la carte | Comportement **inchangé par rapport à une carte SPEEDY** en v7.0.0 — pas de grille QCM interactive dédiée sur `/tv`/`/anim` (attendu, arrive avec #185) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation Globale

- [ ] `tests/procedures/anim-memotion-160.md` rejoué intégralement, **sans modification**, PASS
- [ ] Carte neuve : sélecteur déverrouillé, `DIFFICULTY=1` à la création ne verrouille pas (piège §3.2)
- [ ] Verrouillage sur contenu propre au type (SPEEDY : réponse ; QCM : une réponse ou un seuil)
- [ ] Verrou réactif (vidage → redéverrouillage)
- [ ] Les 9 questions MEMOTION existantes s'ouvrent verrouillées sur SPEEDY et se réenregistrent sans erreur
- [ ] Carte QCM éditable, visuellement correcte, mais pas jouable avant #185 (aucune régression de périmètre)

---

## Non-Régression (suite automatisée, à exécuter avant validation manuelle)

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `cd server-go && go build ./... && go test ./... -race` | Build OK, tous les tests PASS, y compris les suites `*_184_test.go` (B-B1→B-B8, B-T1) | | |
| 2 | `cd server-go/web && npx vitest run` | Tous les tests PASS, y compris `hostContext.test.js`, `typeState.test.js`, `motionCardLock.test.js`, `questionTypeMeta.test.js` | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Notes QA

[Espace pour observations]
