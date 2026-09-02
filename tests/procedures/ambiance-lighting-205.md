# Procédure de Test — Éclairage d'ambiance : fondations (#205, milestone v10.0.0)

**Version** : v10.0.0.x (QUALIF)
**Date** : 2026-09-02
**Testeur** : QA / CDP (voir note ci-dessous)
**Contrat** : `contracts/lighting.md`
**Plan** : `_work/reports/planner-v10-plan-205-20260902-203000.md`

## Important — périmètre sans effet visible

#205 n'introduit **aucun comportement observable nouveau** pour l'utilisateur :
aucun pilote matériel réel, aucune configuration, aucun écran. Le seul pilote livré
est un pilote factice de test (`internal/lighting.FakeDriver`). C'est délibéré — la
valeur de #205 est la fondation dont dépendent #206 (pilote BLE), #207
(configuration/UI) et #213 (éclairage par équipe).

**Conséquence pour cette procédure** : il n'y a pas de scénario "l'utilisateur voit
la salle changer de couleur" à valider ici — ce sera la procédure de #207/#206. Ce
qui doit être validé ici est la **non-régression stricte** du comportement LED
existant, plus quelques vérifications de robustesse observables sans matériel.

## Prérequis

- [ ] Environnement : QUALIF ou LOCAL (aucune dépendance matérielle)
- [ ] Binaire buildé depuis la branche `milestone/v10.0.0` (ou merge ultérieur)
- [ ] Au moins 1 quiz avec buzzers physiques ou virtuels (VJoueur) configuré, comme
      pour toute session de jeu normale — aucun matériel d'éclairage requis

## Scénarios

### Scénario 1 — Non-régression LED stricte (CA1)

**Objectif** : vérifier qu'aucun comportement LED existant (buzzers) n'a changé.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | `git diff <base milestone/v10.0.0>..HEAD -- server-go/cmd/server/led_test.go server-go/cmd/server/led_broadcast_132_test.go` | Diff **vide** — ces deux fichiers de test n'ont pas été modifiés | | |
| 2 | `cd server-go && go test ./cmd/server/... -run 'TestLEDSet|TestLEDBroadcast132' -v` | Tous verts, aucune régression | | |
| 3 | Lancer une partie QCM normale, faire buzzer 2 équipes, révéler | Les LED des buzzers réagissent exactement comme avant #205 (couleurs, timing, COMET sur les points) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 2 — Démarrage/arrêt serveur sans configuration d'ambiance (CA2)

**Objectif** : vérifier qu'aucun comportement/log parasite n'apparaît au démarrage
alors que l'éclairage d'ambiance n'est pas configuré (cas #205, toujours vrai tant
que #207 n'est pas livré).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | Démarrer le serveur (`./server.exe` ou binaire QUALIF) avec un `config.json` sans section `lighting` (cas normal aujourd'hui) | Démarrage normal, aucune erreur | | |
| 2 | Consulter les logs serveur au démarrage (`/ws/logs` ou fichier de log) | **Aucune** ligne mentionnant "Ambiance" ou "lighting" | | |
| 3 | Laisser tourner une partie complète normalement | Aucune ligne de log "Ambiance: driver error" ou similaire | | |
| 4 | Arrêter le serveur (`curl -s http://localhost/shutdown`) | Arrêt propre, pas de blocage ni de panic | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 3 — Suite automatisée #205 (exécution QA)

**Objectif** : faire exécuter par QA la suite automatisée complète et consigner le
résultat (remplace un test manuel — aucune interface à observer).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | `cd server-go && go build ./...` | Compile sans erreur | | |
| 2 | `go test ./internal/lighting/... -v` | 100% vert (paquet pur : sémantique de l'écrivain, CA2/CA4/CA5) | | |
| 3 | `go test ./internal/lighting/... -race` | Vert — aucune race détectée | | |
| 4 | `go test ./cmd/server/... -run 'TestAmbianceExhaustiveness|TestCA|TestDevAmbiance|TestIntegration_GoldenPath' -v` | 100% vert (CA1 est un `t.Skip` documenté, pas un échec — voir Scénario 1 pour sa preuve réelle) | | |
| 5 | `go test ./cmd/server/... -run 'TestAmbianceExhaustiveness|TestCA5' -race` | Vert — aucune race détectée | | |
| 6 | `go test ./cmd/server/... -run 'TestLEDSet|TestLEDBroadcast132'` | Vert (non-régression, cf. Scénario 1) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 4 — Vérification volontaire du test d'exhaustivité (CA3, à faire une fois)

**Objectif** : confirmer que le test d'exhaustivité rougit vraiment s'il est
contourné — build de confiance dans le filet de sécurité, pas un test de
non-régression à répéter à chaque QUALIF.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | Dans une copie de travail jetable, ajouter temporairement dans `cmd/server/main.go` une fonction `func (a *App) handleTestNouveauTruc() { a.sendLEDSetAllBuzzers() }` (jamais appelée, peu importe) | — | | |
| 2 | `go test ./cmd/server/... -run TestAmbianceExhaustiveness_MainGoMatchesRegistry -v` | **Échoue**, message nommant `handleTestNouveauTruc -> sendLEDSetAllBuzzers` et renvoyant vers `contracts/lighting.md §6` | | |
| 3 | Retirer la fonction ajoutée (`git checkout -- server-go/cmd/server/main.go`) | Le dépôt de travail retrouve son état propre | | |
| 4 | Relancer le test | Redevient vert | | |

**Note** : `TestAmbianceExhaustiveness_CatchesUnregisteredSite` (automatisé,
`cmd/server/ambiance_sites_test.go`) fait déjà cette preuve sans toucher au
dépôt — ce scénario manuel est une contre-vérification ponctuelle, à faire une
fois avant la clôture de #205, pas à chaque QUALIF.

**Verdict** : [ ] PASS  [ ] FAIL

## Critères de Validation

- [ ] Scénario 1 (non-régression LED) : PASS
- [ ] Scénario 2 (coût nul sans configuration) : PASS
- [ ] Scénario 3 (suite automatisée) : PASS
- [ ] Scénario 4 (exhaustivité, une fois) : PASS
- [ ] Aucune régression constatée sur une partie QCM/MEMORY/MEMOTION/RAFALE normale

## Notes QA

- Cette procédure ne couvre **aucun scénario fonctionnel utilisateur** (voir la
  note de périmètre en tête de fichier) — la validation manuelle utilisateur de
  l'éclairage réel de la salle interviendra avec #206/#207.
- Les scénarios 1, 3 et 4 sont essentiellement des commandes `go test` : ils
  peuvent être exécutés par `qa` (agent) sans navigateur, contrairement à une
  procédure fonctionnelle classique (règle projet : les scénarios manuels
  nécessitant un navigateur restent du ressort de l'utilisateur).
- Le Scénario 2 (logs au démarrage) nécessite d'observer le serveur réel — à la
  charge de `deployer`/QA selon la procédure QUALIF standard.
