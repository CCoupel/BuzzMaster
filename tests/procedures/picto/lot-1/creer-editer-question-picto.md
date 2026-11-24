# Procedure de Test — Créer et éditer une question Picto (LOT-1, #191)

**Version** : 13.0.0.x (QUALIF)
**Date** : 2026-11-23
**Testeur** : QA / utilisateur
**Références** : `contracts/picto.md` §3.1, §4, §11 — maquette `docs/mockup/v13.0.0/ui/admin_question_editor__picto_settings.html`

## Prerequis

- [ ] Environnement : LOCAL ou QUALIF, page **Questions** de l'admin
- [ ] Données : au moins une catégorie existante
- [ ] Accès : interface admin

## Scenarios

### Scenario 1 — Le type Picto est proposé

**Validation** : HUMAIN — **Issues** : #191

**Objectif** : le sélecteur de type propose « ✏️ Picto », sans mention « Pictionary » ni « PICSONARY ».

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Page Questions → nouvelle question | Le bouton de type « ✏️ Picto » est visible avec les autres types | | |
| 2 | Choisir Picto | L'éditeur Picto s'affiche, conforme à la maquette (grille compacte, sans choix de source) | | |
| 3 | Lire tous les libellés de l'éditeur | Aucun libellé ne contient « Pictionary » ni « PICSONARY » | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 2 — Créer une question Picto valide

**Validation** : HUMAIN — **Issues** : #191

**Objectif** : créer une question avec 4 mots et la retrouver à l'identique.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Type Picto ; durée affichée | La durée est pré-remplie à **90** s | | |
| 2 | Saisir les mots (un par ligne) : `Éléphant`, `Jean-Pierre`, `L'arbre`, `chat 2` | Les résumés (chips) affichent les 4 mots | | |
| 3 | Choisir une catégorie (une seule) et des points | Sélection unique acceptée | | |
| 4 | Enregistrer | Enregistrement confirmé, la question apparaît dans la liste avec l'icône ✏️ | | |
| 5 | Rouvrir la question | Mots, ordre, catégorie, durée et points sont identiques à la saisie | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 3 — Refus des mots invalides, message du serveur affiché

**Validation** : HUMAIN — **Issues** : #191

**Objectif** : l'éditeur affiche tel quel le refus du serveur (une seule implémentation des règles, §3.1).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Saisir `Éléphant` puis `elephant`, enregistrer | Refus lisible (doublon), question non enregistrée, saisie conservée | | |
| 2 | Saisir un mot avec `!` ou `/`, enregistrer | Refus lisible (caractère non admis) | | |
| 3 | Saisir un mot de 41 caractères | Refus lisible (longueur) | | |
| 4 | Saisir 21 mots | Refus lisible (plus de 20 mots) | | |
| 5 | Vider la liste de mots | Refus lisible (liste vide) | | |
| 6 | Mettre la durée à 29 puis 301 | Refus lisible (durée 30 à 300) | | |
| 7 | Corriger puis enregistrer | Enregistrement accepté | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 4 — Modifier une question existante sans perdre les mots

**Validation** : HUMAIN — **Issues** : #191

**Objectif** : enregistrer à nouveau ne détruit pas la liste (piège de reconstruction §11).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Rouvrir la question du scénario 2, changer seulement les points, enregistrer | Enregistré | | |
| 2 | Rouvrir | Les 4 mots sont toujours présents, dans l'ordre | | |
| 3 | Retirer un mot, ajouter `souris`, enregistrer, rouvrir | La liste reflète exactement la nouvelle saisie (pas de cumul) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 5 — Pas de génération IA ni de carte MEMOTION pour Picto

**Validation** : HUMAIN — **Issues** : #191

**Objectif** : Picto est absent des listes qui ne le concernent pas.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir la génération IA | Picto n'est pas proposé | | |
| 2 | Éditer une question MEMOTION, ajouter une carte | Picto n'est pas proposé comme type de carte | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Criteres de Validation

- [ ] Tous les scénarios passent
- [ ] Les messages de refus sont lisibles
- [ ] Aucune régression sur l'éditeur des autres types (RAFALE, ROUE) : ouvrir et enregistrer une question de chaque

## Notes QA

Les règles de validation (bornes, caractères, doublons, durée) sont aussi couvertes en automatique :
`server-go/internal/server/picto_upload_191_test.go`.
