# Procédure de Test — Seuils d'équipes SOLO/multi RAFALE et MEMOTION (#201)

**Version** : v8.0.0 (branche `milestone/v8.0.0`)
**Date** : 2026-09-01
**Testeur** : Utilisateur (validation manuelle — ni `qa` ni `deployer` n'exécutent cette procédure,
aucun navigateur fiable dans les sessions agents)
**Issue** : #201 — rationalisation SOLO/multi via un helper partagé (`participantsCountConform`,
`server-go/internal/game/engine.go`), appliqué à MEMORY (inchangé)/MEMOTION/RAFALE.
**SHA fix** : `e2917395` (RAFALE multi ≥2), `d3c6fb20` (helper partagé + RAFALE SOLO=1)
**SHA tests** : `b1261783`, `e45d82c3` + complément `test-writer` (SHA `1176deff`)

## Contexte du changement

Avant #201, RAFALE et MEMOTION avaient des règles de conformité de sélection d'équipes
incohérentes avec MEMORY :

| Type | Avant #201 | Après #201 |
|------|-----------|-----------|
| MEMORY | SOLO = exactement 1 équipe, multi ≥ 2 équipes | **inchangé** (déjà cette règle) |
| MEMOTION | toujours ≥ 1 équipe, quel que soit le mode | **SOLO = exactement 1, multi ≥ 2** |
| RAFALE SOLO | aucune équipe requise (0 accepté) | **exactement 1 équipe requise** |
| RAFALE multi | ≥ 1 équipe suffisait | **≥ 2 équipes requises** |

Ce changement est **visible utilisateur** : des configurations qui démarraient auparavant (RAFALE
SOLO sans équipe, RAFALE/MEMOTION multi avec 1 seule équipe) sont maintenant bloquées — DÉMARRER
reste inactif tant que le nombre d'équipes sélectionnées ne correspond pas à la règle du mode.

---

## Prérequis

- [ ] Environnement : QUALIF (binaire Windows buildé, cf. `docs/QUALIF_PROCEDURE.md`)
- [ ] Un quiz avec une question `RAFALE` (réservoir peuplé, cf. `tests/procedures/rafale-v8.md`
  Prérequis) et une question `MEMOTION`
- [ ] Au moins 2 équipes créées, chacune avec au moins 1 buzzer physique ou VJoueur assigné
- [ ] Poste `/anim` (ou `/admin`) ouvert pour la sélection des équipes participantes

---

## Scénario 1 — RAFALE SOLO, 0 équipe sélectionnée → START bloqué (nouveau depuis #201)

**Objectif** : Vérifier que RAFALE SOLO exige désormais une sélection explicite — avant #201, RAFALE
SOLO démarrait sans aucune équipe.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur `/anim`, configurer une manche RAFALE mode `SOLO` (catégorie + difficulté valides), **NE SÉLECTIONNER AUCUNE équipe** | Bouton DÉMARRER grisé/non cliquable | | |
| 2 | Tenter DÉMARRER malgré tout | **Refusé** — le jeu reste en PREPARE, aucune manche ne démarre | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — RAFALE SOLO, 1 équipe sélectionnée → START fonctionne

**Objectif** : Contrôle positif — le cas normal (1 équipe pour un jeu SOLO) continue de fonctionner.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur `/anim`, configurer une manche RAFALE mode `SOLO`, sélectionner **1** équipe participante | Bouton DÉMARRER actif, jeu passe en READY | | |
| 2 | DÉMARRER | La manche démarre normalement, se déroule comme d'habitude (cf. `tests/procedures/rafale-v8.md` scénario 1) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — RAFALE multi, 1 seule équipe → START bloqué (nouveau depuis #201)

**Objectif** : Vérifier que RAFALE multi (`CHACUN_SON_TOUR`/`TANT_QUE_JE_GAGNE`/`MAILLON_FAIBLE`)
exige désormais ≥2 équipes — avant #201, 1 seule équipe suffisait.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur `/anim`, configurer une manche RAFALE mode `CHACUN_SON_TOUR` (catégorie + difficulté valides), sélectionner **1 seule** équipe participante | Bouton DÉMARRER grisé/non cliquable (auparavant : actif avec 1 équipe) | | |
| 2 | Tenter DÉMARRER malgré tout | **Refusé** — le jeu reste en PREPARE | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — RAFALE multi, 2 équipes → START fonctionne

**Objectif** : Contrôle positif — le seuil ≥2 est bien atteignable normalement.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur `/anim`, configurer une manche RAFALE mode `CHACUN_SON_TOUR`, sélectionner **2** équipes participantes | Bouton DÉMARRER actif, jeu passe en READY | | |
| 2 | DÉMARRER | La manche démarre normalement (cf. `tests/procedures/rafale-v8.md` scénario 2) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — MEMOTION multi, 1 seule équipe → START bloqué (nouveau depuis #201)

**Objectif** : Même vérification que le scénario 3, transposée à MEMOTION — avant #201, MEMOTION
acceptait toujours ≥1 équipe quel que soit le mode.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur `/anim`, préparer une question MEMOTION mode `CHACUN_SON_TOUR`, sélectionner **1 seule** équipe participante | Bouton DÉMARRER grisé/non cliquable (auparavant : actif avec 1 équipe) | | |
| 2 | Tenter DÉMARRER malgré tout | **Refusé** — le jeu reste en PREPARE | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 6 — MEMOTION multi, 2 équipes → START fonctionne

**Objectif** : Contrôle positif — même vérification que le scénario 4, transposée à MEMOTION.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur `/anim`, préparer une question MEMOTION mode `CHACUN_SON_TOUR`, sélectionner **2** équipes participantes | Bouton DÉMARRER actif, jeu passe en READY | | |
| 2 | DÉMARRER | La manche démarre normalement, la grille MEMOTION est jouable | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Point de vigilance frontend — `prepareWaitReason.js` (hors périmètre de cette procédure)

Signalé par `dev-backend` (handoffs `_work/handoff/dev-backend-20260901-093512.md` et
`-104512.md`), **non corrigé, hors périmètre backend** : `server-go/web/src/utils/prepareWaitReason.js`
calcule encore l'ancienne règle RAFALE (`count >= 1` pour multi, `true` pour SOLO) et un libellé qui
ne distingue pas SOLO/multi pour RAFALE. Conséquence probable observable pendant cette procédure :
le **message d'attente** affiché en PREPARE (texte secondaire sous le bouton LANCER) peut rester
incorrect ou trompeur (ex. ne pas mentionner qu'il manque une 2e équipe) MÊME SI le blocage du
bouton DÉMARRER lui-même est correct (le blocage réel dépend de `participantsConform` côté serveur,
pas de ce libellé). Noter toute incohérence de libellé observée dans les scénarios 1, 3 et 5
ci-dessus — ne pas la compter comme un échec de CES scénarios (dont l'objet est le blocage réel de
DÉMARRER), mais comme un point à transmettre à `dev-frontend` si confirmé.

## Critères de Validation

- [ ] RAFALE SOLO sans équipe sélectionnée bloque DÉMARRER (scénario 1)
- [ ] RAFALE SOLO avec 1 équipe démarre normalement (scénario 2)
- [ ] RAFALE multi avec 1 seule équipe bloque DÉMARRER (scénario 3)
- [ ] RAFALE multi avec 2 équipes démarre normalement (scénario 4)
- [ ] MEMOTION multi avec 1 seule équipe bloque DÉMARRER (scénario 5)
- [ ] MEMOTION multi avec 2 équipes démarre normalement (scénario 6)
- [ ] Aucune régression sur MEMORY (règle inchangée par #201) ni sur les scénarios existants de
  `tests/procedures/rafale-v8.md`

## Notes QA

[Espace pour observations — noter en particulier tout libellé de message d'attente RAFALE
incohérent avec le blocage réel, cf. section "Point de vigilance frontend" ci-dessus]
