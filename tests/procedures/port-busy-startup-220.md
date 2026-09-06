# Procédure de Test — Démarrage sur Port Occupé (#220)

**Version** : 9.0.0.x (milestone v9.0.0, Lot 0 / Batch 0)
**Date** : 2026-09-04
**Issue** : #220
**Testeur** : QA / Utilisateur (validation manuelle obligatoire — jamais exécuté par `qa`/`deployer`)

---

## Contexte

Avant #220, une boucle de retry existait déjà (commit f625bbe) mais elle était **non bloquante et
non observable** : `Start()` renvoyait toujours `nil` immédiatement, même port occupé — le serveur
annonçait « Server started successfully » et ouvrait le navigateur sur une URL **morte** pendant
que la vraie tentative de bind tournait en silence dans une goroutine détachée. Le cas `EACCES`
(port < 1024 sans droits) faisait mourir la goroutine en silence : process vivant, serveur
injoignable, aucun message.

**#220 rend l'attente bloquante ET observable** :
- Port HTTP occupé (`EADDRINUSE`) → boucle avec backoff progressif (500 ms → 1 s → 5 s plafonné),
  loggée, **jamais** de « started successfully » ni d'ouverture navigateur avant le bind réel.
- Permission refusée (`EACCES`, ex. port 80 sans droits) → boucle **aussi**, à cadence lente (30 s),
  avec un message actionnable — **jamais** de sortie brutale (`os.Exit`).
- `Ctrl+C` doit interrompre proprement l'attente, quelle que soit sa cause.
- DNS (port 53) et mDNS restent **non fatals** pour le démarrage HTTP, mais leur échec devient
  **visible** dans `/ws/logs` et le tampon de logs (au lieu de partir dans la sortie standard).

⚠️ **Ce scénario inverse délibérément le Scénario 4 de** `tests/procedures/bugfix-http-port-retry.md`
**(voir la note ajoutée en tête de ce fichier)** : avant #220, `EACCES` devait faire planter le
serveur sans retry ; après #220, il doit boucler sans jamais planter.

---

## Prérequis

- [ ] Environnement : **LOCAL** (Windows et/ou Linux — le scénario 4 est spécifique à Linux)
- [ ] Binaire `buzzcontrol` compilé depuis la branche `milestone/v9.0.0` (Lot 0 mergé)
- [ ] Accès à un shell pour lancer des commandes et occuper des ports manuellement
- [ ] `curl` ou navigateur pour valider les réponses HTTP
- [ ] Accès à l'interface `/ws/logs` (onglet Logs de l'admin, ou client WS manuel) pour vérifier la
      visibilité des messages DNS/mDNS
- [ ] Deux versions compilées du binaire pour le scénario 5 (auto-update) : `buzzcontrol-vX.Y.Z`
      (ancienne) et `buzzcontrol-vX.Y.Z+1` (nouvelle) — cf. prérequis du scénario 3 de l'ancienne
      procédure `bugfix-http-port-retry.md`

---

## Scénario 1 — Port HTTP déjà occupé au lancement

**Objectif** : Vérifier qu'un port occupé bloque visiblement le démarrage, sans faux signal de succès.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Occuper le port HTTP configuré (ex: `nc -l 8080` ou `python -m http.server 8080`, selon `config.json`) | Port occupé | | |
| 2 | Dans un autre terminal, démarrer le serveur : `./buzzcontrol` | **Aucun** message « Server started successfully » | | |
| 3 | Observer la fenêtre/console | **Aucune** ouverture de navigateur | | |
| 4 | Observer les logs | Message(s) d'attente répétés, nommant le **port résolu** (celui réellement tenté) et **sa provenance** (`config.json`, valeur par défaut du code, ou `--port`) | | |
| 5 | Attendre ~2 s | Les logs successifs montrent un intervalle **croissant** (ex: ~500 ms puis ~1 s), pas un flot fixe toutes les 500 ms | | |
| 6 | Requêter `curl http://localhost:<port>/version` pendant l'attente | Pas de réponse (connexion refusée) — le serveur n'écoute pas encore réellement | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Libération du port → reprise normale

**Objectif** : Vérifier que le démarrage reprend et s'achève dès que le port se libère.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Poursuivre depuis le Scénario 1 (serveur en attente) | — | | |
| 2 | Libérer le port occupé (terminer `nc`/`python`) | — | | |
| 3 | Observer les logs dans les 5 s suivantes | Message de succès de bind (ex: port désormais lié) | | |
| 4 | Requêter `curl http://localhost:<port>/version` | Réponse 200 avec le numéro de version | | |
| 5 | Naviguer vers `http://localhost:<port>/admin/game` | Interface admin accessible | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — `Ctrl+C` pendant l'attente

**Objectif** : Vérifier une interruption propre pendant que le serveur attend un port occupé.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Occuper le port HTTP configuré (comme Scénario 1) | Port occupé | | |
| 2 | Démarrer le serveur dans un terminal au premier plan : `./buzzcontrol` | Le serveur entre en attente (logs d'attente visibles) | | |
| 3 | Appuyer `Ctrl+C` pendant l'attente (avant de libérer le port) | Le process se termine **rapidement** (quelques centaines de ms, pas 500 ms–5 s d'attente résiduelle) | | |
| 4 | Vérifier le code de sortie du process (`echo $?` sous Linux) | Arrêt propre, pas de crash/panic dans la console | | |
| 5 | Vérifier qu'aucun process `buzzcontrol` résiduel ne tourne (`ps aux \| grep buzzcontrol` / gestionnaire des tâches) | Aucun process orphelin | | |
| 6 | Libérer le port occupé par le terminal 1, puis relancer `./buzzcontrol` normalement | Démarrage normal, sans message résiduel de l'ancienne tentative | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Permission refusée (`EACCES`, Linux)

**Objectif** : Vérifier que `EACCES` boucle sans jamais planter le process, avec un message actionnable.

**Spécifique Linux** — nécessite un environnement où le port testé est effectivement restreint
(port < 1024 sans droits root ; `CAP_NET_BIND_SERVICE` absent). Sous Windows ou en conteneur avec
privilèges, marquer **N/A**.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Configurer un port < 1024 sans lancer en root, ex: `./buzzcontrol --port 80` (utilisateur non-root) | — | | |
| 2 | Observer les logs immédiatement | **Pas** de crash, **pas** de fermeture immédiate du process | | |
| 3 | Observer le message affiché | Message **explicite et actionnable** : mentionne le port, la permission refusée, et la remédiation (utiliser un port ≥ 1024 via `config.json`/`--port`, ou élever les privilèges) | | |
| 4 | Vérifier la cadence des messages répétés (attendre ~1 min) | Cadence **lente** (~30 s entre deux messages), pas de flot rapide | | |
| 5 | Vérifier que le process reste vivant tout du long (`ps aux \| grep buzzcontrol`) | Process toujours présent, jamais de sortie brutale | | |
| 6 | `Ctrl+C` pendant l'attente lente | Arrêt propre et **rapide** (pas besoin d'attendre le prochain cycle de 30 s) | | |
| 7 | Relancer avec un port ≥ 1024 (ex: `--port 8080`) | Démarrage normal, immédiat | | |

**Verdict** : [ ] PASS  [ ] FAIL  [ ] N/A (environnement sans restriction de port)

---

## Scénario 5 — Non-régression : auto-update pendant que l'ancien process tient le port

**Objectif** : Vérifier que le scénario historique (raison d'être de la boucle depuis f625bbe)
fonctionne toujours après #220 — cf. Scénario 3 de `tests/procedures/bugfix-http-port-retry.md`,
rejoué ici pour confirmer la non-régression avec la nouvelle sémantique bloquante.

**Prérequis spécifiques** : disposer des deux binaires `buzzcontrol-vX.Y.Z` (ancienne) et
`buzzcontrol-vX.Y.Z+1` (nouvelle), la nouvelle étant accessible depuis le flux de mise à jour
(`data/updates/` ou `/admin/updates`).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Démarrer l'ancienne version : `./buzzcontrol-vX.Y.Z` | Serveur opérationnel — `curl /version` retourne `X.Y.Z` | | |
| 2 | Naviguer vers `http://localhost/admin/updates` et appliquer la mise à jour | Confirmation : mise à jour en cours, redémarrage imminent | | |
| 3 | Observer les logs pendant la transition | Séquence attendue : nouveau process lancé → attend le port (logs d'attente nommant le port) → ancien process se termine et libère le port → nouveau process bind avec succès | | |
| 4 | Dans les 5 s après l'apply, requêter `curl http://localhost/version` | Réponse 200 avec la **nouvelle** version `X.Y.Z+1` | | |
| 5 | Vérifier que les WebSocket se reconnectent (onglet admin ouvert pendant l'update) | Reconnexion automatique, pas d'état cassé | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 6 — DNS/mDNS non fatal mais visible

**Objectif** : Vérifier qu'un échec DNS (port 53, ex: `systemd-resolved` déjà dessus sous Linux) ou
mDNS ne bloque pas le démarrage HTTP, mais devient visible dans `/ws/logs`.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sous Linux avec `systemd-resolved` actif (port 53 occupé), démarrer `./buzzcontrol` normalement (port HTTP libre) | Le serveur HTTP démarre normalement malgré le port 53 occupé | | |
| 2 | Ouvrir l'onglet Logs de l'admin (`/ws/logs`) ou se connecter en client WS manuel | Une entrée **WARN** mentionnant l'échec DNS est visible (pas seulement dans la sortie console) | | |
| 3 | Vérifier que le reste du serveur (buzzers, jeu) fonctionne normalement | Aucun impact fonctionnel de l'échec DNS | | |

**Verdict** : [ ] PASS  [ ] FAIL  [ ] N/A (port 53 libre dans cet environnement)

---

## Critères de Validation

- [ ] Scénario 1 : aucun faux signal de succès (log, navigateur) tant que le bind n'a pas abouti
- [ ] Scénario 2 : le démarrage reprend et s'achève dès la libération du port
- [ ] Scénario 3 : `Ctrl+C` interrompt proprement pendant l'attente, sans process orphelin
- [ ] Scénario 4 : `EACCES` boucle sans jamais planter, message actionnable, cadence lente
- [ ] Scénario 5 : le scénario auto-update historique n'est pas régressé
- [ ] Scénario 6 : échec DNS/mDNS non fatal mais visible dans `/ws/logs`
- [ ] Aucun `os.Exit` observé sur une erreur de bind, quelle qu'elle soit (scénarios 1 et 4)

---

## Notes QA

[Espace pour observations, timing mesuré, logs relevés]
