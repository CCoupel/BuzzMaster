# Procédure de Test — LOT-1 v13.1.0 : présélection des gagnants (#321) et x/net (#322)

**Version** : v13.1.0.x (QUALIF)
**Date** : 2026-11-24
**Testeur** : QA (scénarios `AUTO`) ; **l'utilisateur** pour les scénarios `HUMAIN` (rendu, lisibilité de la sélection)
**Contrats** : `contracts/award-preselect.md` (§1 règles, §2 table par type, §3 repli ARDOISE, §4 MEMOTION)
**Tests automatiques** : `tests/INDEX.md` (lignes #321) — `awardPreselect.321.test.js`, `GamePage.award-preselect.321.test.jsx`, `AnimPage.memotion-preselect.321.test.jsx`

> Sauvegarder `data/` avant. Prévoir 3 équipes avec joueurs (tablettes VJoueur ou buzzers). Postes : `/admin`, `/anim`, `/tv`.
> **Rafale Picto** (type `RAFALE_PICTO`) est déclaré dans la table mais n'existe qu'à partir du LOT-3 : son scénario est joué au LOT-3.

## Prérequis

- [ ] QUALIF, partie en cours, 3 équipes
- [ ] Une question de chaque type du tableau ci-dessous

---

## Scénario 0 — x/net v0.62.0 et vulnérabilités (#322)

**Validation** : AUTO — **Issues** : #322

**Objectif** : la dépendance `golang.org/x/net` est à jour et `govulncheck` ne signale plus de faille atteignable.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `grep golang.org/x/net server-go/go.mod` | `v0.62.0` | | |
| 2 | `cd server-go && govulncheck ./...` (chaîne native `commands.env`) | « No vulnerabilities found » (aucune entrée atteignable) | | |
| 3 | `go build ./... && go test ./internal/server/... ./cmd/server/...` | Build et tests WebSocket/HTTP verts | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 1 — PICTO (un mot)

**Validation** : HUMAIN — **Issues** : #321

**Objectif** : à l'entrée en `REVEALED`, la sélection d'attribution est présélectionnée selon `award-preselect.md` ; le crédit reste manuel.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Question Picto, équipe choisie ; l'équipe trouve le mot (« Trouvé ») puis révélation | L'équipe qui dessinait est cochée d'office ; montant = barème. Avec une manche en TIMEOUT / STOPPED / NO_WORD : aucune case cochée ; chip « +N pts » sur l'équipe cochée ; décochée ou aucune case : pas de chip | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — MEMORY SOLO

**Validation** : HUMAIN — **Issues** : #321

**Objectif** : à l'entrée en `REVEALED`, la sélection d'attribution est présélectionnée selon `award-preselect.md` ; le crédit reste manuel.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Partie MEMORY en SOLO, jouer quelques paires, révéler | L'équipe participante est cochée | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — MEMORY multi-équipes

**Validation** : HUMAIN — **Issues** : #321

**Objectif** : à l'entrée en `REVEALED`, la sélection d'attribution est présélectionnée selon `award-preselect.md` ; le crédit reste manuel.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | MEMORY à 3 équipes : une équipe sans paire, une avec paires, une dont les erreurs ramènent le score à 0 | Seules les équipes de score > 0 sont cochées ; les équipes MEMORY gardent leur chip habituelle | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — RAFALE (Rafale Speedy) SOLO

**Validation** : HUMAIN — **Issues** : #321

**Objectif** : à l'entrée en `REVEALED`, la sélection d'attribution est présélectionnée selon `award-preselect.md` ; le crédit reste manuel.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Manche en SOLO avec compteur > 0, puis une seconde fois avec compteur 0 | Équipe cochée si compteur retenu > 0, sinon rien ; les cartes RAFALE gardent leur chip habituelle | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — RAFALE multi-équipes + MAILLON_FAIBLE

**Validation** : HUMAIN — **Issues** : #321

**Objectif** : à l'entrée en `REVEALED`, la sélection d'attribution est présélectionnée selon `award-preselect.md` ; le crédit reste manuel.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | 3 équipes, compteurs différents dont un à 0 ; refaire en MAILLON_FAIBLE (compteur retombé à 0 mais meilleur > 0) | Équipes à compteur retenu > 0 cochées (RAFALE_TEAM_BEST en MAILLON_FAIBLE) ; les cartes RAFALE gardent leur chip habituelle | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 6 — QCM niveau équipe puis niveau joueur

**Validation** : HUMAIN — **Issues** : #321

**Objectif** : à l'entrée en `REVEALED`, la sélection d'attribution est présélectionnée selon `award-preselect.md` ; le crédit reste manuel.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | QCM, plusieurs équipes répondent juste et faux ; révéler. Refaire avec POINTS_TARGET = PLAYER | Équipes (puis joueurs) ayant donné la bonne couleur cochés ; aucune bonne réponse → rien ; couleur assignée sans réponse ignorée ; les cartes QCM gardent leur chip habituelle (+ chip « +N pts » sur les équipes cochées) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 7 — SPEEDY

**Validation** : HUMAIN — **Issues** : #321

**Objectif** : à l'entrée en `REVEALED`, la sélection d'attribution est présélectionnée selon `award-preselect.md` ; le crédit reste manuel.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | SPEEDY, 3 buzzeurs, révéler ; refaire sans aucun buzz | Seul le premier buzzeur (joueur, ou son équipe au niveau équipe) est coché ; l'admin peut le décocher ; sans buzz rien n'est coché ; chip « +N pts » sur l'équipe/joueur coché ; décoché → la chip disparaît (retour PRET ou rien) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 8 — ARDOISE

**Validation** : HUMAIN — **Issues** : #321

**Objectif** : à l'entrée en `REVEALED`, la sélection d'attribution est présélectionnée selon `award-preselect.md` ; le crédit reste manuel.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | ARDOISE, réponse attendue « Éléphant » ; équipes : « elephant », « ÉLÉPHANT », « girafe », « Le Éléphant » | Les deux premières cochées (repli accents/casse/espaces), les autres non. ANSWER vide → rien ; chip « +N pts » sur les équipes cochées | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 9 — ROUE et ENTRACTE

**Validation** : HUMAIN — **Issues** : #321

**Objectif** : à l'entrée en `REVEALED`, la sélection d'attribution est présélectionnée selon `award-preselect.md` ; le crédit reste manuel.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Révéler une question ROUE puis passer un ENTRACTE | Aucune case cochée (crédit auto ROUE / aucun point) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 10 — Comportement commun

**Validation** : HUMAIN — **Issues** : #321

**Objectif** : à l'entrée en `REVEALED`, la sélection d'attribution est présélectionnée selon `award-preselect.md` ; le crédit reste manuel.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur un QCM : décocher une équipe, « Tout sélectionner », modifier le champ Points ; créditer une équipe puis revenir ; changer de question | Sélection toujours modifiable ; modifier « Points » ne recoche ni ne décoche ; équipe déjà créditée jamais cochée ; sélection vidée au changement de question ; rien n'est envoyé avant « Attribuer les points » ; chip « +N pts » en en-tête de carte pour chaque équipe cochée non créditée (montant ≠ 0), qui disparaît à la décoche (retour PRET/rien) ; un montant négatif (pénalité) s'affiche en rouge ; une équipe déjà créditée n'a pas de chip | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 11 — MEMOTION admin

**Validation** : HUMAIN — **Issues** : #321

**Objectif** : à l'entrée en `REVEALED`, la sélection d'attribution est présélectionnée selon `award-preselect.md` ; le crédit reste manuel.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Carte MEMOTION en REVEAL sur /admin (mode multi-équipes puis SOLO) | L'équipe qui a la main est présélectionnée (✓) ; seuls elle et « Perdu » sont proposés (règle de jeu existante) ; rien n'est envoyé avant le bouton « Valider » | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 12 — MEMOTION anim

**Validation** : HUMAIN — **Issues** : #321

**Objectif** : à l'entrée en `REVEALED`, la sélection d'attribution est présélectionnée selon `award-preselect.md` ; le crédit reste manuel.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Même carte sur /anim (tablette) | Même présélection (confirmation `confirmWinner`, anim seulement) ; seuls l'équipe qui a la main et « PERSONNE » sont proposés ; « VALIDER » envoie le gagnant ; résultat identique sur la TV | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Criteres de Validation

- [ ] Scénario 0 (#322) PASS
- [ ] Tous les scénarios 1 à 12 PASS (aucune sélection cochée à tort, jamais d'unité déjà créditée ni à montant ≤ 0)
- [ ] Les attributions groupées existantes (#244 / #251 / #254 / #289) se comportent comme avant

## Notes QA

