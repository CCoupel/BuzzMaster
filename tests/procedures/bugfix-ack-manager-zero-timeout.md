# Procédure de Test — Bugfix AckManager Zero Timeout

**Version** : 4.0.9+
**Date** : 2026-05-02
**Branche** : bugfix/ack-manager-zero-timeout
**Testeur** : QA

---

## Contexte du Bug

`AckManager.Start()` appelait `time.NewTicker(0)` lorsque `AckTimeoutMs` valait `0`.
Go panique immédiatement avec :

```
panic: non-positive interval for NewTicker
```

**Condition de déclenchement** : le fichier `config.json` est **absent** au démarrage du
serveur. `config.Get()` tombe dans son fallback mais, dans certains chemins de code, la
`ServerConfig` pouvait avoir `AckTimeoutMs = 0` (valeur zéro de Go) au lieu de la valeur
par défaut `2000`.

**Fix appliqué** (`internal/server/ack_manager.go`) :

```go
timeout := time.Duration(m.cfg.AckTimeoutMs) * time.Millisecond
if timeout <= 0 {
    timeout = 2 * time.Second   // guard : évite time.NewTicker(0)
}
```

---

## Prérequis

- [ ] Environnement : **LOCAL** (Windows ou Linux)
- [ ] Binaire `buzzcontrol` compilé depuis la branche `bugfix/ack-manager-zero-timeout`
- [ ] Accès shell pour déplacer/supprimer `config.json`
- [ ] Optionnel : `curl` pour valider les réponses HTTP

---

## Scénario 1 — Démarrage sans config.json (régression principale)

**Objectif** : Vérifier que le serveur démarre sans panic même en l'absence de `config.json`.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Aller dans le dossier du binaire (`server-go/` ou dossier de distribution) | — | | |
| 2 | Renommer ou supprimer `config.json` : `mv config.json config.json.bak` | Fichier absent du dossier courant | | |
| 3 | Démarrer le serveur : `./buzzcontrol` (Linux) ou `buzzcontrol.exe` (Windows) | Log `Warning: Could not load config.json, using defaults` — **aucun panic, le serveur continue de démarrer** | | |
| 4 | Observer les logs pendant 3 s | Aucune ligne `panic:` ou `non-positive interval` — le serveur reste stable | | |
| 5 | Requêter `curl http://localhost/version` (ou ouvrir le navigateur) | Réponse 200 avec numéro de version | | |
| 6 | Arrêter le serveur (`Ctrl+C` ou `curl http://localhost/shutdown`) | Arrêt propre | | |
| 7 | Restaurer `config.json` : `mv config.json.bak config.json` | Fichier restauré | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Démarrage normal avec config.json valide (non-régression)

**Objectif** : Vérifier que le comportement habituel n'est pas affecté par le fix.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | S'assurer que `config.json` est présent avec `ack_timeout_ms` absent ou à `2000` | Fichier présent | | |
| 2 | Démarrer le serveur : `./buzzcontrol` | Démarrage normal, pas de log `Warning: Could not load config.json` | | |
| 3 | Observer les logs pendant 3 s | Aucun panic, aucun log d'erreur ACK | | |
| 4 | Requêter `curl http://localhost/version` | Réponse 200 | | |
| 5 | Arrêter le serveur | Arrêt propre | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — config.json présent avec ack_timeout_ms explicitement à 0

**Objectif** : Vérifier que même une valeur `0` dans `config.json` est gérée sans panic.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir `config.json` et modifier le champ `ack_timeout_ms` à `0` dans la section `server` | Fichier sauvegardé avec `"ack_timeout_ms": 0` | | |
| 2 | Démarrer le serveur : `./buzzcontrol` | Démarrage sans panic — le guard interne applique 2 s de fallback | | |
| 3 | Observer les logs pendant 3 s | Aucune ligne `panic:` | | |
| 4 | Requêter `curl http://localhost/version` | Réponse 200 | | |
| 5 | Arrêter le serveur et restaurer `ack_timeout_ms` à `2000` | Config restaurée | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Protocole ACK fonctionnel après démarrage sans config.json

**Objectif** : Vérifier que le protocole ACK buzzer fonctionne normalement même avec les
valeurs par défaut (pas de config.json).

**Prérequis** : Au moins un buzzer BuzzClick physique connecté, ou utiliser le simulateur.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Démarrer le serveur sans `config.json` (cf. Scénario 1, étape 2-3) | Serveur démarré sans panic | | |
| 2 | Connecter un buzzer physique (ou simulateur WebSocket sur `/ws/buzzer`) | Buzzer visible dans l'interface admin (`/admin/teams`) | | |
| 3 | Déclencher un `LED_SET` depuis l'admin (ex: attribution de couleur d'équipe) | Badge `ACK_PENDING` apparaît brièvement sur le buzzer dans l'UI, puis disparaît à réception de l'ACK | | |
| 4 | Vérifier les logs serveur | Log `AckManager: retry` absent si l'ACK est reçu dans les 2 s (délai fallback) | | |

**Verdict** : [ ] PASS  [ ] FAIL  [ ] N/A (pas de buzzer disponible)

---

## Critères de Validation

- [ ] Scénario 1 : le serveur démarre sans crash en l'absence de `config.json`
- [ ] Scénario 2 : comportement normal non régressé avec un `config.json` valide
- [ ] Scénario 3 : `ack_timeout_ms: 0` dans `config.json` ne provoque pas de panic
- [ ] Aucune ligne `panic: non-positive interval for NewTicker` dans les logs
- [ ] Le serveur reste opérationnel (HTTP 200 sur `/version`) dans tous les scénarios

---

## Notes QA

[Espace pour observations, version du binaire testé, logs relevés]
