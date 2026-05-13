# Procédure de Test — Bugfix #83 — SO_LINGER(0) port libre après arrêt serveur

**Version** : 5.1.3+
**Date** : 2026-05-13
**Testeur** : QA
**Bug corrigé** : après arrêt du serveur, le port restait en TIME_WAIT, causant une erreur "port busy" au redémarrage immédiat

## Contexte du Bug

Quand le serveur BuzzControl s'arrête (fermeture fenêtre console ou Ctrl+C), les connexions TCP
acceptées (navigateurs, WebSockets) pouvaient rester dans l'état **TIME_WAIT** pendant 30–120 s.
Lors d'un redémarrage immédiat, le serveur obtenait `WSAEADDRINUSE` (Windows) ou
`EADDRINUSE` (Linux) même si le socket d'écoute avait été libéré.

**Fix** : `lingerListener` wraps le `net.Listener` et appelle `SetLinger(0)` sur chaque connexion
acceptée, forçant l'envoi d'un **RST** à la fermeture (au lieu du FIN/ACK qui génère TIME_WAIT).

## Prérequis

- [ ] Environnement : LOCAL ou QUALIF (Windows recommandé pour reproduire le bug original)
- [ ] Serveur BuzzControl démarré sur port 80 (ou `--port 9090` pour les tests)
- [ ] Au moins un navigateur connecté sur l'interface (`/`)
- [ ] Accès terminal pour lancer/arrêter le serveur et observer les messages

## Scénarios

---

### Scénario 1 — Arrêt via fermeture de la fenêtre console (Windows)

**Objectif** : Vérifier qu'après fermeture brutale de la fenêtre, le port est disponible immédiatement

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Démarrer le serveur dans une fenêtre console Windows | Serveur démarre, logs de démarrage visibles | | |
| 2 | Ouvrir un navigateur sur `http://localhost/` | Page admin chargée (connexion WebSocket active) | | |
| 3 | **Fermer la fenêtre console** (clic sur la croix) | Serveur s'arrête | | |
| 4 | **Immédiatement** (< 2 s), ouvrir un autre terminal | — | | |
| 5 | Relancer le serveur : `./buzzcontrol.exe` | Serveur démarre sans erreur `WSAEADDRINUSE` | | |
| 6 | Ouvrir `http://localhost/` dans le navigateur | Page admin accessible | | |

**Verdict** : [ ] PASS  [ ] FAIL

**Avant le fix** : étape 5 produisait `listen tcp :80: bind: Only one usage of each socket address…` pendant ~30–120 s.

---

### Scénario 2 — Arrêt via Ctrl+C (Linux/Windows)

**Objectif** : Vérifier qu'après Ctrl+C, le port est libéré sans délai

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Démarrer le serveur : `./buzzcontrol` (ou `./buzzcontrol.exe`) | Logs de démarrage visibles | | |
| 2 | Ouvrir `http://localhost/` dans un navigateur | Page chargée, WS connecté | | |
| 3 | Dans le terminal serveur, presser **Ctrl+C** | Serveur s'arrête (message "shutdown" dans les logs) | | |
| 4 | **Immédiatement** relancer : `./buzzcontrol` | Serveur redémarre sans erreur port-busy | | |
| 5 | Vérifier dans les logs | Aucun message `address already in use` / `EADDRINUSE` | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 3 — Arrêt via endpoint `/shutdown` (arrêt propre)

**Objectif** : Vérifier que l'arrêt via l'API fonctionne également sans TIME_WAIT

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Démarrer le serveur | — | | |
| 2 | Connecter plusieurs onglets navigateur | 2–3 connexions WebSocket actives | | |
| 3 | Lancer l'arrêt : `curl -s http://localhost/shutdown` | Réponse `{"status":"shutting down"}` ou similaire | | |
| 4 | Attendre 1 s (shutdown gracieux) | Serveur s'arrête | | |
| 5 | Relancer le serveur immédiatement | Démarre sans erreur de port | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 4 — Vérification absence TIME_WAIT (Linux uniquement)

**Objectif** : Confirmer via `ss` que les connexions acceptées ne génèrent plus de TIME_WAIT

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Démarrer le serveur sur port 9090 (`--port 9090`) | — | | |
| 2 | Ouvrir 3 onglets navigateur sur `http://localhost:9090/` | 3 connexions WebSocket actives | | |
| 3 | Dans un autre terminal : `ss -tan \| grep 9090` | Connexions ESTABLISHED visibles | | |
| 4 | Arrêter le serveur (Ctrl+C) | — | | |
| 5 | **Dans les 2 secondes** : `ss -tan \| grep 9090` | **Aucune ligne TIME_WAIT** pour les connexions acceptées | | |

**Verdict** : [ ] PASS  [ ] FAIL

**Note** : Sans SO_LINGER(0), on verrait des lignes `TIME_WAIT` pour les paires de port des connexions acceptées.

---

### Scénario 5 — Non-régression : connexions normales non interrompues

**Objectif** : Vérifier que SO_LINGER(0) ne perturbe pas les connexions actives normales

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Démarrer le serveur | — | | |
| 2 | Lancer une partie (Quiz ou MEMOTION) | Partie en cours, WebSocket actif | | |
| 3 | Faire buzzer plusieurs équipes | Buzzers enregistrés correctement | | |
| 4 | Fermer le navigateur manuellement | Connexion WS fermée proprement | | |
| 5 | Rouvrir le navigateur | Reconnexion réussie, état de jeu retrouvé | | |
| 6 | Redémarrer le serveur via `/shutdown` | Connexions fermées proprement, pas d'erreur côté client | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation

- [ ] Redémarrage immédiat possible après fermeture fenêtre console (Windows)
- [ ] Redémarrage immédiat possible après Ctrl+C
- [ ] Aucune ligne `TIME_WAIT` sur les ports acceptés (Linux, vérifiable via `ss -tan`)
- [ ] Aucune régression sur les parties en cours (WebSocket, buzzers)
- [ ] Logs propres au démarrage (pas de `address already in use`)

## Notes QA

[Espace pour observations, captures d'écran, version OS testée]

---

*Procédure générée par test-writer — BuzzControl bugfix #83*
