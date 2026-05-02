# Procédure de Test — Self-Update : Stratégie Copy-and-Launch (issue #70)

**Version** : 4.0.11+
**Date** : 2026-05-02
**Issue** : #70 — refactoring de `performRestart()`
**Testeur** : QA

---

## Contexte du Changement

L'ancienne stratégie de mise à jour (`backup-and-overwrite`) remplaçait le binaire en cours
d'exécution **en place** :

```
[ancien] cp currentExe → currentExe.old
         cp newExe     → currentExe          ← écrase le binaire en cours d'exécution
         exec currentExe
```

Problèmes constatés :
- **Linux** : `text file busy` — le noyau refuse d'écraser un binaire en cours d'exécution
- **Windows** : risque de corruption si l'ancien processus lit encore son exécutable

**Nouvelle stratégie (`copy-and-launch`)** :

```
[nouveau] cp newExe → exeDir/buzzcontrol-vX.Y.Z-<platform>[.exe]   ← à côté du courant
          chmod 0755 (Unix)
          exec exeDir/buzzcontrol-vX.Y.Z-<platform>
          exit 0                                                      ← ancien processus quitte
```

Le binaire courant **n'est jamais modifié**. Le nouveau binaire est copié avec son nom
versionné dans le même répertoire, puis lancé. L'ancien processus se termine proprement.

---

## Prérequis

- [ ] Environnement : **LOCAL** (Windows ou Linux)
- [ ] Deux binaires compilés :
  - `buzzcontrol-v4.0.10-<platform>[.exe]` — version courante
  - `buzzcontrol-v4.0.11-<platform>[.exe]` — nouvelle version
- [ ] Accès shell pour vérifier les fichiers et processus
- [ ] `curl` disponible pour les appels API
- [ ] Port 80 accessible

---

## Scénario 1 — Non-régression : le binaire courant n'est pas modifié

**Objectif** : Vérifier que le fichier `buzzcontrol` courant n'est pas écrasé, renommé ou
supprimé pendant l'update.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Calculer le hash du binaire courant avant update : `sha256sum buzzcontrol-v4.0.10-linux-arm64` (Linux) ou `certutil -hashfile buzzcontrol-v4.0.10-windows-amd64.exe SHA256` (Windows) | Hash noté (ex: `a1b2c3...`) | | |
| 2 | Démarrer le serveur courant : `./buzzcontrol-v4.0.10-linux-arm64` | Serveur démarré, `curl http://localhost/version` retourne `4.0.10` | | |
| 3 | Placer le nouveau binaire dans `data/updates/` : `cp buzzcontrol-v4.0.11-linux-arm64 data/updates/` | Fichier présent dans `data/updates/` | | |
| 4 | Déclencher le self-update : `curl -s -X POST http://localhost/api/updates/apply -H "Content-Type: application/json" -d '{"version":"4.0.11"}'` | Réponse : `{"success":true,"message":"Server restarting with version 4.0.11..."}` | | |
| 5 | Attendre 5 s que le nouveau processus démarre | — | | |
| 6 | Recalculer le hash du fichier original : `sha256sum buzzcontrol-v4.0.10-linux-arm64` | Hash identique à l'étape 1 — **fichier inchangé** | | |
| 7 | Vérifier que le fichier original est toujours là et non renommé | `ls -la buzzcontrol-v4.0.10-*` → fichier présent sans suffixe `.old` ou `.bak` | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Le nouveau binaire versionné est copié à côté du courant

**Objectif** : Vérifier que le nouveau binaire est copié dans le **même répertoire** que
l'exécutable courant, sous son nom versionné.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lister le répertoire du serveur **avant** l'update : `ls -la /chemin/vers/serveur/` | Ne contient PAS de fichier `buzzcontrol-v4.0.11-*` | | |
| 2 | Déclencher le self-update (voir Scénario 1, étape 4) | — | | |
| 3 | Pendant les 3–5 s de délai avant restart, lister le répertoire : `watch -n0.5 ls -la` (Linux) | Apparition d'un fichier `buzzcontrol-v4.0.11-<platform>` dans le répertoire courant | | |
| 4 | Après le restart (nouveau processus actif), lister le répertoire | Le nouveau binaire versionné est présent : `buzzcontrol-v4.0.11-linux-arm64` (ou `.exe`) | | |
| 5 | Vérifier que le fichier n'est PAS dans `data/updates/` (il y a été copié, pas déplacé) | `ls data/updates/buzzcontrol-v4.0.11-*` → fichier toujours présent dans `data/updates/` aussi | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Le serveur reste opérationnel après le restart

**Objectif** : Vérifier que le nouveau processus démarre correctement et que le service
est disponible dans les 10 s suivant le déclenchement du self-update.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Démarrer le serveur courant (v4.0.10) | `curl http://localhost/version` → `4.0.10` | | |
| 2 | Ouvrir l'interface admin dans le navigateur (`http://localhost/admin`) | Interface affichée, connexion WebSocket établie | | |
| 3 | Déclencher le self-update via l'API | Réponse `success:true`, le navigateur commence à afficher "connexion perdue" | | |
| 4 | Attendre 5–10 s | — | | |
| 5 | `curl http://localhost/version` | Réponse `4.0.11` — nouveau processus actif | | |
| 6 | Recharger l'interface admin | Interface accessible, WebSocket reconnecté | | |
| 7 | Vérifier PID du processus courant : `pgrep -a buzzcontrol` (Linux) | Seul le **nouveau** processus (`v4.0.11`) est actif — l'ancien a bien terminé | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Résilience : fichier update corrompu / trop petit

**Objectif** : Vérifier que si le fichier de mise à jour est invalide (trop petit, corrompu),
le serveur courant **continue de fonctionner** sans crash ni restart intempestif.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Créer un faux fichier de mise à jour trop petit : `echo "fake" > data/updates/buzzcontrol-v9.9.9-linux-arm64` | Fichier de 5 octets dans `data/updates/` | | |
| 2 | Tenter l'apply via API : `curl -s -X POST ... -d '{"version":"9.9.9"}'` | Réponse `400 Bad Request` : `"Downloaded file appears corrupted"` | | |
| 3 | Observer les logs du serveur | Aucun redémarrage — log `Step 1/4 FAILED` ou validation échouée | | |
| 4 | Vérifier que le serveur répond toujours : `curl http://localhost/version` | Réponse normale avec la version courante | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Résilience : répertoire de l'exe non accessible en écriture

**Objectif** : Vérifier que si la copie échoue (permissions insuffisantes sur le répertoire
de l'exe), le serveur actuel **ne crashe pas** et reste opérationnel.

> **Note** : Ce scénario est spécifique à Linux. Sur Windows, les permissions de répertoire
> fonctionnent différemment.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | (Linux) Retirer les droits d'écriture sur le répertoire du serveur : `chmod 555 /chemin/vers/serveur/` | Répertoire en lecture seule | | |
| 2 | Déclencher le self-update via l'API | Réponse `success:true` (le handler répond avant le restart) | | |
| 3 | Attendre 5 s | — | | |
| 4 | Observer les logs | Log `Step 2/4 FAILED: Cannot copy new binary` — **pas de panic**, pas de crash | | |
| 5 | Vérifier que le serveur courant répond toujours | `curl http://localhost/version` → version courante inchangée | | |
| 6 | Restaurer les permissions : `chmod 755 /chemin/vers/serveur/` | — | | |

**Verdict** : [ ] PASS  [ ] FAIL  [ ] N/A (Windows)

---

## Critères de Validation

- [ ] **Scénario 1** : hash du binaire courant identique avant et après update
- [ ] **Scénario 2** : nouveau binaire versionné présent dans le répertoire de l'exe
- [ ] **Scénario 3** : service disponible en moins de 10 s, seul le nouveau PID actif
- [ ] **Scénario 4** : fichier corrompu rejeté proprement, serveur courant stable
- [ ] **Scénario 5** : erreur de copie loguée sans panic, serveur courant stable (Linux)
- [ ] Aucun fichier `.old` ou `.bak` créé pendant le process (ancienne stratégie éliminée)
- [ ] Le répertoire `data/updates/` conserve le nouveau binaire source (copie, pas déplacement)

---

## Notes QA

[Espace pour observations, version du binaire testé, logs relevés, timing mesuré]
