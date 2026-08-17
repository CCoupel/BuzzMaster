# Procédure de Test — Conformité des participants PREPARE↔READY (#172)

**Version** : à définir (branche `feature/anim-question-display`)
**Date** : 2026-08-17
**Branche** : feature/anim-question-display
**Testeur** : QA

---

## Contexte de la Feature

Plan de référence (fait foi) : `_work/reports/plan-20260817-122307.md`.

Avant #172, une partie MEMORY ou MEMOTION pouvait démarrer sans sélection d'équipe valide
(rattrapage automatique aléatoire côté MEMORY, MEMOTION sans participante) — bug d'origine.
#172 étend la sortie de la phase `PREPARE` déjà existante : elle exige désormais que
**tous les buzzers actifs aient répondu** (comportement d'aujourd'hui, inchangé) **ET** que
**la sélection de participants soit conforme à la règle du mode** :

| Mode | Règle de conformité |
|------|---------------------|
| SPEEDY, QCM, ARDOISE | déjà couvert (≥1 équipe active) — **aucun changement** |
| MEMORY SOLO | exactement une équipe sélectionnée |
| MEMORY multi (CHACUN_SON_TOUR / TANT_QUE_JE_GAGNE) | au moins deux équipes sélectionnées |
| MEMOTION | au moins une équipe sélectionnée |

**Nouveauté structurelle** : si la conformité cesse d'être vraie **alors que la question est
déjà en `READY`** (une équipe retirée de la sélection), la question **repasse automatiquement
en `PREPARE`** — et inversement, la rétablir fait **repasser en `READY` sans geste
supplémentaire** (pas de nouvelle attente des buzzers). Ce retour arrière est **borné à
`READY`** : il ne doit **jamais** se produire une fois la partie `STARTED` ou au-delà — un
verrou sur `Engine.Start` (refus hors phase `READY`) garantit qu'aucune manche ne peut être
lancée sur une sélection non conforme.

**Interfaces concernées** (aucun nouveau badge de phase, aucune nouvelle phase) :
- **Régie** (`GamePage.jsx`) : dans le sélecteur d'équipes MEMORY/MEMOTION, une équipe dont
  les buzzers n'ont pas tous répondu apparaît **grisée, non cliquable**
  (`title="Buzzer(s) non prêt(s)"`), sans disparaître de la liste. Le libellé du sélecteur
  affiche en plus un motif d'attente : « · Buzzers en attente », « · sélectionnez une équipe »
  (SOLO), « · sélectionnez au moins deux équipes » (multi) ou « · sélectionnez au moins une
  équipe » (MEMOTION).
- **Tablette animateur** (`/anim`) : le sous-libellé du bouton **LANCER**, tant que la phase
  est `PREPARE`, affiche un motif court à la place du texte générique « indispo. » :
  « buzzers », « 1 équipe » ou « 2 équipes ».

> **Prérequis d'exécution** : cette procédure est écrite à partir des critères d'acceptation
> du plan (section 6) et non du code final — à exécuter une fois le développement Bloc B/C
> livré (E2, `qa`). Si un des comportements ci-dessous ne correspond pas exactement à ce qui
> est affiché, vérifier d'abord `contracts/CHANGELOG.md` / le handoff dev-backend/dev-frontend
> avant de conclure à une anomalie — un léger écart de libellé peut avoir été arbitré en revue.

---

## Prérequis

- [ ] Environnement : QUALIF (ou LOCAL avec build de la branche `feature/anim-question-display`)
- [ ] Au moins 3 équipes créées, chacune avec au moins un buzzer physique ou virtuel assigné
- [ ] Accès régie (`/game`) **et** tablette animateur (`/anim`) ouverts simultanément sur le
      même jeu, pour observer les deux surfaces en parallèle
- [ ] Au moins une question de chaque type dans le quiz : SPEEDY, QCM, ARDOISE, MEMORY (SOLO
      et un mode multi), MEMOTION
- [ ] Accès aux logs serveur (pour vérifier l'absence de blocage/erreur silencieuse)

---

## Scénario 1 — SPEEDY, QCM, ARDOISE : non-régression (test central, risque R3)

**Objectif** : Vérifier que les trois modes simples atteignent `READY` **exactement comme
aujourd'hui**, avec une seule équipe active — aucun durcissement ne doit être introduit par
#172.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Une seule équipe active (1 buzzer assigné), les autres équipes sans buzzer | | | |
| 2 | Charger une question SPEEDY et cliquer READY (régie) | Entrée en `PREPARE` | | |
| 3 | Faire répondre le buzzer de l'équipe active (PONG) | Passage en `READY` **immédiat**, comme avant #172 — aucun sélecteur de participants ne doit apparaître pour ce type | | |
| 4 | Répéter les étapes 2-3 avec une question QCM, puis une question ARDOISE | Même comportement : passage en `READY` dès que l'unique équipe active répond | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — MEMORY SOLO : conformité et sélecteur régie

**Objectif** : Vérifier que la sortie de `PREPARE` exige exactement une équipe sélectionnée,
et que le sélecteur reflète l'état « prêt » des équipes.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Au moins 2 équipes actives (buzzers assignés) | | | |
| 2 | Charger une question MEMORY mode SOLO, cliquer READY | Entrée en `PREPARE`, sélecteur d'équipes MEMORY affiché en régie | | |
| 3 | Avant que les buzzers répondent, observer le sélecteur | Toutes les équipes apparaissent grisées/non cliquables (`title="Buzzer(s) non prêt(s)"`), libellé « · Buzzers en attente » | | |
| 4 | Faire répondre tous les buzzers (PONG) | Les équipes redeviennent normales dans le sélecteur (cliquables), libellé passe à « · sélectionnez une équipe » (aucune équipe sélectionnée) | | |
| 5 | Sélectionner une équipe dans le sélecteur | Passage en `READY` | | |
| 6 | Observer la tablette `/anim` au même instant | Le bouton LANCER n'affiche plus de sous-libellé d'attente (phase READY) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — MEMORY multi (CHACUN_SON_TOUR / TANT_QUE_JE_GAGNE) : au moins deux équipes

**Objectif** : Vérifier le seuil de conformité spécifique au mode multi.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Au moins 3 équipes actives, tous buzzers répondus | | | |
| 2 | Charger une question MEMORY mode CHACUN_SON_TOUR (ou TANT_QUE_JE_GAGNE), READY | Entrée en `PREPARE`, sélecteur affiché | | |
| 3 | Sélectionner une seule équipe | Reste en `PREPARE`, libellé « · sélectionnez au moins deux équipes » | | |
| 4 | Ajouter une deuxième équipe | Passage en `READY` | | |
| 5 | Observer la tablette `/anim` pendant l'étape 3 | Sous-libellé LANCER affiche « 2 équipes » | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — MEMOTION : au moins une équipe

**Objectif** : Vérifier la règle MEMOTION et confirmer la correction du bug d'origine (plus
de démarrage sans participante).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Au moins 2 équipes actives, tous buzzers répondus | | | |
| 2 | Charger une question MEMOTION, READY | Entrée en `PREPARE`, sélecteur MEMOTION affiché, libellé « · sélectionnez au moins une équipe » | | |
| 3 | Sélectionner une équipe | Passage en `READY` | | |
| 4 | Lancer la partie et jouer un tour | La partie attribue bien les points à l'équipe sélectionnée (pas de perte de points, cf. bug d'origine) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Retour arrière READY → PREPARE, observé sur les deux surfaces

**Objectif** : Vérifier le cœur de la feature (D4) — retirer une équipe en `READY` casse la
conformité et ramène en `PREPARE`, sur la régie **et** la tablette `/anim` simultanément ;
la rétablir fait repasser en `READY` sans geste supplémentaire.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | MEMORY SOLO, une équipe sélectionnée, phase `READY` (comme scénario 2) — régie et `/anim` ouvertes en parallèle | Les deux surfaces affichent la phase READY (bouton LANCER actif) | | |
| 2 | Depuis la régie, retirer l'équipe sélectionnée du sélecteur | La phase repasse en `PREPARE` | | |
| 3 | Observer la tablette `/anim` immédiatement après | Elle reflète elle aussi `PREPARE` — bouton LANCER redevient inactif avec sous-libellé « 1 équipe » | | |
| 4 | Re-sélectionner l'équipe depuis la régie, **sans** provoquer de nouveau PONG buzzer | Passage immédiat en `READY`, sur les deux surfaces | | |
| 5 | Répéter avec MEMORY multi : passer de 2 à 1 équipe sélectionnée en `READY` | Retour en `PREPARE` observé sur les deux surfaces | | |
| 6 | Vérifier les logs serveur pendant toute la manipulation | Aucune erreur, pas de clignotement anormal (léger va-et-vient attendu si la sélection oscille, cf. plan R5 — comportement normal) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 6 — Verrou de démarrage : START refusé en PREPARE

**Objectif** : Vérifier qu'aucune manche ne peut démarrer sur une sélection non conforme
(verrou `Engine.Start`, B4) — le risque principal du plan (R1) si ce verrou est oublié ou
contournable.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | MEMORY SOLO ou multi, phase `PREPARE` (sélection non conforme ou buzzers en attente) | | | |
| 2 | Côté régie, observer le bouton de lancement | Désactivé (grisé), non cliquable | | |
| 3 | Côté tablette `/anim`, observer le bouton LANCER | Désactivé, sous-libellé d'attente affiché (« buzzers », « 1 équipe » ou « 2 équipes ») | | |
| 4 | Si un outil de rejeu de message WebSocket est disponible (devtools / outil interne), envoyer manuellement une action `START` alors que la phase est `PREPARE` | Le serveur **refuse** la transition (log de refus explicite), la phase **reste `PREPARE`** — aucun COUNTDOWN ne démarre | | |
| 5 | Sinon (pas d'outil de rejeu disponible) | Cocher N/A ci-dessous — ce point est couvert par le test automatisé dédié (`TestEngine_Start_RefusesOutsideReady`, dev-backend D5) | | |

**Verdict** : [ ] PASS  [ ] FAIL  [ ] N/A (étape 4 déléguée au test automatisé D5, pas d'outil de rejeu disponible)

---

## Critères de Validation

- [ ] SPEEDY, QCM, ARDOISE : `READY` atteint exactement comme avant #172, aucune régression
      (scénario 1)
- [ ] MEMORY SOLO : `READY` exige exactement une équipe sélectionnée (scénario 2)
- [ ] MEMORY multi : `READY` exige au moins deux équipes sélectionnées (scénario 3)
- [ ] MEMOTION : `READY` exige au moins une équipe, points correctement attribués (scénario 4)
- [ ] Retour arrière READY→PREPARE visible sur régie **et** tablette `/anim`, rétablissement
      sans geste supplémentaire (scénario 5)
- [ ] Aucune tentative de START n'aboutit hors phase READY (scénario 6)
- [ ] Sélecteur régie : équipe non prête grisée, non cliquable, jamais masquée (arbitrage H)
- [ ] Aucune nouvelle valeur de phase, aucun nouveau badge — uniquement le sous-libellé
      existant (`/anim`) et le libellé complémentaire (régie) sont modifiés
- [ ] Aucune erreur/`panic` dans les logs serveur pendant toute la procédure
- [ ] Suite automatisée complète (Go + React) au vert avant validation finale, en particulier
      les tests `engine_prepare_ready_rollback_test.go` (D4), les tests dev-backend D1/D2/D3/D5/D6/D7
      et `TestEngine_Start_WithCountdown` (modifié pour #172 B4 — vérifier que la modification
      est bien celle documentée dans le handoff test-writer, pas une régression)

## Notes QA

[Espace pour observations]

> Note pour QA : le scénario 6 étape 4 peut être coché N/A si aucun outil de rejeu de message
> WebSocket n'est disponible dans l'environnement de test — il est alors couvert par le test
> automatisé `TestEngine_Start_RefusesOutsideReady` (dev-backend, D5). Ne pas cocher FAIL par
> défaut dans ce cas.
