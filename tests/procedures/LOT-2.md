# Procédure de Test — LOT-2 v13.1.0 : Pool Picto sur le moteur de pool généralisé (#324)

**Version** : v13.1.0.x (QUALIF)
**Date** : 2026-11-25
**Testeur** : QA (scénarios `AUTO`) ; **l'utilisateur** pour les scénarios `HUMAIN` (rendu, libellés, parcours)
**Contrats** : `contracts/picto-pool.md` rév. 2 (§1 moteur, §3 marquage, §4 HTTP, §5 IA, §6 UI, §7 sauvegarde, §9 non-régression), `contracts/picto.md` §17.13 (libellés)
**Tests automatiques** : `tests/INDEX.md` (lignes #324) — NR Rafale Speedy (`pool_rafale_nr_324_test.go`, `pool_rafale_api_nr_324_test.go`), Pool Picto (`pool_picto_324_test.go`), libellés (`questionTypeMeta.labels.324.test.js`, `QuestionsPage.pool-tabs.324.test.jsx`)

> Sauvegarder `data/` avant. Prévoir une clé API IA valide, un réservoir Rafale Speedy existant (v13.0.0), 3 équipes avec joueurs et une tablette de dessinateur. Postes : `/admin`, `/anim`, `/tv`.

## Prérequis

- [ ] QUALIF, réservoir Rafale Speedy non vide, clé IA configurée
- [ ] Catégories personnalisées disponibles pour les filtres

---

## Scénario 0 — Non-régression automatique Rafale Speedy et Pool Picto (rappel QA)

**Validation** : AUTO — **Issues** : #324

**Objectif** : la généralisation du moteur de pool ne change rien pour Rafale Speedy et le Pool Picto respecte le contrat.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `go test ./internal/game -run TestPoolRafaleNR` et `go test ./internal/server -run 'TestPoolRafaleAPI_NR\|TestPoolPicto_'` | Verts : fichiers octet pour octet, formes HTTP, prompt IA golden, sauvegarde, 409, job unique | | |
| 2 | `npx vitest run` sur les fichiers `*.324.test.*` | Verts : libellés, onglets, modale et alerte paramétrées | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 1 — Onglets et libellés Rafale Speedy / Pool Speedy / Pool Picto

**Validation** : HUMAIN — **Issues** : #324

**Objectif** : Onglets et libellés Rafale Speedy / Pool Speedy / Pool Picto.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Page Quiz : observer les onglets ; ouvrir le sélecteur de type d'une question ; lancer une question RAFALE sur /admin, /anim, /tv | Onglets « Questions », « 🌀 Pool Speedy », « ✏️ Pool Picto » ; le type s'appelle « Rafale Speedy » (🌀) partout (sélecteur, cartes, page Jeu, anim, TV, notifications) ; plus aucun « Rafale » seul ni « Réservoir RAFALE » à l'écran ; Picto reste « Picto » ✏️ | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Pool Picto : ajout d'un mot et refus

**Validation** : HUMAIN — **Issues** : #324

**Objectif** : Pool Picto : ajout d'un mot et refus.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Onglet Pool Picto → « Un mot » : ajouter « Éléphant » (catégorie A, difficulté 2) ; essayer « ELEPHANT » même catégorie, « chat<3 », un mot de 41 caractères, un mot vide | Mot créé (colonne Mot, pas de colonne réponse) ; refus lisibles : doublon, caractère interdit, trop long, vide ; le même mot reste accepté dans une autre catégorie | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Pool Picto : collage d'une liste

**Validation** : HUMAIN — **Issues** : #324

**Objectif** : Pool Picto : collage d'une liste.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Formulaire → « Une liste » : coller 5 lignes dont un doublon interne, un mot invalide, une ligne vide ; valider ; refaire avec plus de 200 mots | Mots valides créés en une fois ; refus listés dans le formulaire (jamais tronqués), doublon refusé dès sa 2ᵉ occurrence ; au-delà de 200 mots : refus explicite | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Génération IA du Pool Picto (smoke)

**Validation** : HUMAIN — **Issues** : #324

**Objectif** : Génération IA du Pool Picto (smoke).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | « ✨ Générer via IA » sur Pool Picto : paliers proposés, thème « Animaux », lancer 10 mots, suivre la progression | Paliers 10 / 20 / 50 ; mots dessinables, courts, sans nom propre ni marque ; mots invalides ou en doublon écartés ; progression ; les mots arrivent dans le tableau | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Génération IA du Pool Speedy inchangée + job unique croisé

**Validation** : HUMAIN — **Issues** : #324

**Objectif** : Génération IA du Pool Speedy inchangée + job unique croisé.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | « ✨ Générer via IA » sur Pool Speedy (titre « Générer des questions pour le Pool Speedy ») : 10 questions ; pendant le job, tenter une génération sur Pool Picto puis sur Quiz | Paliers 10/20/50/100/200 et comportement identiques à v13.0.0 ; une seconde génération (Picto ou Quiz) est refusée tant que le job tourne (un seul job à la fois, tous chemins) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 6 — Garde pendant une manche Picto

**Validation** : HUMAIN — **Issues** : #324

**Objectif** : Garde pendant une manche Picto.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer une manche Picto (STARTED) puis ouvrir Pool Picto ; essayer ajout, suppression, remise à zéro, génération IA ; mettre en PAUSE et réessayer | Message explicite ; la liste du Pool Picto est grisée / indisponible (409) en STARTED et PAUSED, de même que les actions d'écriture ; elle redevient lisible à l'arrêt de la manche ; Pool Speedy reste lisible et modifiable | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 7 — Sauvegarde / restauration / réinitialisation « Pools » (trois états)

**Validation** : HUMAIN — **Issues** : #324

**Objectif** : Sauvegarde / restauration / réinitialisation « Pools » (trois états).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | BackupPage : cocher « Pools » (parent), puis seulement « Pool Speedy », puis seulement « Pool Picto » ; télécharger ; réinitialiser ; restaurer l'archive. Restaurer aussi une archive v13.0.0 (sans files/picto) | Case parente à trois états (cochée / partielle / décochée) ; la clé rafale seule n'embarque pas le Pool Picto et inversement ; réinitialisation idem ; l'archive v13.0.0 se restaure à l'identique, Pool Picto vide, sans erreur | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 8 — Éditeur : source « Pool » du type Picto

**Validation** : HUMAIN — **Issues** : #324

**Objectif** : Éditeur : source « Pool » du type Picto.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Éditeur : créer une NOUVELLE question Picto (observer la source proposée d'office), cliquer une catégorie, filtrer difficultés, observer l'alerte de pool ; enregistrer puis rouvrir ; basculer sur « Saisis dans la question » ; rouvrir une question Picto v13.0.0 (créée avant la mise à jour) | Une NOUVELLE question Picto démarre sur la source « 🗂 Pool Picto » avec le filtre = catégorie de la question (éditeur de mots masqué) ; alerte « disponibles » lisible ; enregistrement fidèle (source, catégories, difficultés) ; une question v13.0.0 s'ouvre en mots saisis, ses mots intacts, aucun filtre parasite ; basculer source Mots → Pool → Mots ne perd rien | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 9 — Manche Picto source Pool : tirage, marquage, pool vide

**Validation** : HUMAIN — **Issues** : #324

**Objectif** : Manche Picto source Pool : tirage, marquage, pool vide.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Question Picto source Pool (3 mots en pool) : lancer la manche, voir le mot côté dessinateur seulement, terminer ; relancer jusqu'à épuisement ; NEW_GAME | Mot tiré dans le pool filtré et marqué utilisé ; indice de catégorie visible sur TV et anim (jamais le mot) ; pool épuisé → READY/START refusé avec POOL_EMPTY ; NEW_GAME remet les marques à zéro | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 10 — Manche Rafale Speedy rejouée (non-régression)

**Validation** : HUMAIN — **Issues** : #324

**Objectif** : Manche Rafale Speedy rejouée (non-régression).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Rejouer en QUALIF une manche Rafale Speedy complète : SOLO, multi-équipes, alphabétique, pré-tirage de la question suivante, avec le réservoir existant de v13.0.0 | Comportement identique à v13.0.0 : pioche stratifiée, questions marquées utilisées, compteurs, pré-tirage, lettres ; fichiers reservoir.json / rafale_used.json lisibles par la version précédente ; aucune erreur dans les logs | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Criteres de Validation

- [ ] Scénario 0 PASS
- [ ] Scénarios 1 à 10 PASS (libellés corrects, aucun écart avec v13.0.0 pour Rafale Speedy)
- [ ] Les questions Picto v13.0.0 et les archives v13.0.0 se comportent comme avant

## Notes QA

