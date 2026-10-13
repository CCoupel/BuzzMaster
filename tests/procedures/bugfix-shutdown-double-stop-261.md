# Procédure de Test — Bugfix #261 : panic « close of closed channel » à l'arrêt

**Version** : 11.2.1
**Date** : 2026-10-12
**Testeur** : utilisateur (QA n'exécute pas les procédures manuelles)

## Contexte

`/shutdown` appelait `stop()` deux fois (hook `OnShutdown` puis fin de `main`) ; le second
`BroadcasterManager.Stop()` paniquait (`close of closed channel`, `broadcaster.go`). Arrêt effectif mais code de sortie non nul.

## Prérequis

- [ ] Environnement : LOCAL
- [ ] Binaire du serveur compilé depuis `milestone/v11.2.1`
- [ ] Port 80 libre, `curl` disponible

## Scénario 1 — Arrêt par /shutdown

**Objectif** : l'arrêt est propre, sans panic, code de sortie 0.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer le serveur au premier plan (`./server.exe`) | Démarrage normal | | |
| 2 | Dans un autre terminal : `curl -s http://localhost/shutdown` | Réponse HTTP reçue | | |
| 3 | Observer la console du serveur | Aucun `panic`, aucun `close of closed channel`, aucune trace de goroutine | | |
| 4 | Relever le code de sortie (`echo $?` ; PowerShell : `$LASTEXITCODE`) | `0` | | |
| 5 | Vérifier que le port 80 est libéré | Plus d'écoute sur le port 80 | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 2 — Arrêt par Ctrl+C (non-régression)

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer le serveur, attendre « Server started successfully » | Démarrage normal | | |
| 2 | Ctrl+C | Arrêt sans panic, code de sortie 0 | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Critères de Validation

- [ ] Aucun `panic` à l'arrêt, quel que soit le déclencheur
- [ ] Code de sortie 0

## Notes QA

