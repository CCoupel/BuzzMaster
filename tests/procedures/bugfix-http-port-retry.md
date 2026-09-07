# Procédure de Test — Bugfix HTTP Port Retry au Self-Update

**Version** : 4.0.8+
**Date** : 2026-05-01
**Commit** : f625bbe
**Testeur** : QA

> ⚠️ **Scénario 4 superseded par #220** (milestone v9.0.0, 2026-09-04) — voir
> `tests/procedures/port-busy-startup-220.md`. #220 inverse volontairement ce comportement :
> `EACCES` (permission refusée) ne doit **plus** faire planter le serveur sans retry ; il doit
> désormais boucler à cadence lente avec un message actionnable, exactement comme un port occupé.
> Ce fichier est conservé tel quel pour l'historique du fix original (f625bbe) — les Scénarios 1 à 3
> restent valides tels que décrits.

---

## Contexte du Bug

Lors d'un self-update, le processus serveur courant démarre le nouveau binaire **avant** de libérer
lui-même le port 80. Le nouveau processus appelait `ListenAndServe()` une seule fois et échouait
immédiatement avec `address already in use`, laissant le serveur inaccessible.

**Fix appliqué** (`internal/server/http.go`) : boucle de retry toutes les 500 ms dans
`HTTPServer.Start()` + helper `isPortInUse()` pour distinguer cette erreur des erreurs fatales.

---

## Prérequis

- [ ] Environnement : **LOCAL** (Windows ou Linux)
- [ ] Binaire `buzzcontrol` compilé depuis la branche `bugfix/http-port-retry`
- [ ] Accès à un shell pour lancer des commandes
- [ ] Port 80 accessible (ou port de test configurable si besoin de droits root)
- [ ] Optionnel : `curl` ou navigateur pour valider les réponses HTTP

---

## Scénario 1 — Démarrage normal (régression)

**Objectif** : Vérifier que le serveur démarre normalement quand le port est libre.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | S'assurer que le port 80 est libre : `ss -tlnp \| grep :80` (Linux) ou `netstat -ano \| findstr :80` (Windows) | Aucun processus sur le port 80 | | |
| 2 | Démarrer le serveur : `./buzzcontrol` (Linux) ou `buzzcontrol.exe` (Windows) | Log `Server starting on port 80` sans message `busy, retrying` | | |
| 3 | Après 1 s, requêter `curl http://localhost/version` | Réponse 200 avec le numéro de version (ex: `4.0.8`) | | |
| 4 | Arrêter le serveur (`Ctrl+C` ou `curl http://localhost/shutdown`) | Serveur arrêté proprement | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Port occupé puis libéré (régression principale)

**Objectif** : Vérifier que le serveur entre en mode retry et finit par démarrer quand le port se libère.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Occuper le port 80 manuellement : `nc -l 80` (Linux) ou `python -m http.server 80` (Windows/Linux) | Port 80 occupé | | |
| 2 | Dans un autre terminal, démarrer le serveur : `./buzzcontrol` | Log `Port 80 busy, retrying in 500ms...` apparaît en boucle (toutes les ~500 ms) — le processus **ne plante pas** | | |
| 3 | Attendre 2–3 cycles de retry (environ 1–2 s) | Logs de retry continuent, serveur toujours en cours d'exécution | | |
| 4 | Libérer le port : terminer `nc`/`python` dans le premier terminal | — | | |
| 5 | Dans les 2 s suivantes, observer les logs du serveur | Log `Server starting on port 80` (sans `busy`) — le serveur a bindé le port | | |
| 6 | Requêter `curl http://localhost/version` | Réponse 200 avec le numéro de version | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Self-update complet (golden path)

**Objectif** : Valider le flux de mise à jour automatique de bout en bout, sans interruption de service visible.

**Prérequis spécifiques** :
- Disposer de deux versions compilées : `buzzcontrol-vX.Y.Z` (ancienne) et `buzzcontrol-vX.Y.Z+1` (nouvelle)
- Copier la nouvelle version dans `data/updates/` du serveur courant (ou utiliser le flux download depuis `/admin/updates`)

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Démarrer l'ancienne version : `./buzzcontrol-vX.Y.Z` | Serveur opérationnel sur port 80 — `curl /version` retourne `X.Y.Z` | | |
| 2 | Naviguer vers `http://localhost/admin/updates` dans le navigateur | Page de mises à jour affichée, version disponible détectée | | |
| 3 | Cliquer sur **Appliquer la mise à jour** (ou `POST /api/updates/apply`) | Message de confirmation : mise à jour en cours, redémarrage imminent | | |
| 4 | Observer les logs du serveur (terminal) | Séquence attendue :<br>1. `Applying update...`<br>2. `Starting new process...` (le nouveau binaire est lancé)<br>3. `Port 80 busy, retrying in 500ms...` (nouveau processus attend)<br>4. L'ancien processus se termine et libère le port<br>5. `Server starting on port 80` (nouveau processus bind) | | |
| 5 | Dans les 5 s après l'apply, requêter `curl http://localhost/version` | Réponse 200 avec la **nouvelle** version `X.Y.Z+1` | | |
| 6 | Naviguer vers `http://localhost/admin/game` | Interface admin accessible, état du jeu intact | | |
| 7 | Vérifier que les WebSocket se reconnectent (onglet admin ouvert pendant l'update) | Reconnexion automatique dans les 5 s, pas d'état cassé | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Erreur fatale non confondue avec port occupé

**Objectif** : Vérifier que les erreurs non-récupérables (ex : port invalide, permissions) font bien planter le serveur et ne déclenchent pas de retry infini.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur Linux, démarrer le serveur sur le port 80 sans droits root : `./buzzcontrol --port 80` (si les permissions sont refusées) | Log d'erreur fatal (ex: `bind: permission denied`) — le serveur s'arrête sans retry | | |
| 2 | Vérifier que le message n'est PAS `Port 80 busy, retrying in 500ms...` | Le log indique une erreur fatale, pas un retry | | |

> **Note** : Si le serveur tourne avec les droits appropriés ou sur un port > 1024, ce scénario
> peut être adapté en configurant un port invalide (ex: `99999`) dans `config.json`.

**Verdict** : [ ] PASS  [ ] FAIL  [ ] N/A (droits root disponibles)

---

## Critères de Validation

- [ ] Scénario 1 : démarrage normal non régressé
- [ ] Scénario 2 : retry fonctionne et le serveur finit par démarrer
- [ ] Scénario 3 : self-update complet sans coupure prolongée du service
- [ ] Aucun message `address already in use` n'apparaît comme erreur fatale lors d'un update
- [ ] La variable de retry ne masque pas les erreurs fatales (scénario 4)

---

## Notes QA

[Espace pour observations, timing mesuré, logs relevés]
