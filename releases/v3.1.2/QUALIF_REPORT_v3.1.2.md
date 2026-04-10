# Rapport de Qualification - BuzzControl v3.1.2

## Informations Déploiement

| Champ            | Valeur                                     |
|------------------|--------------------------------------------|
| Version          | 3.1.2                                      |
| Environnement    | QUALIF                                     |
| Date             | 2026-02-23                                 |
| Branche          | feature/usb-unified-modal                  |
| Commit           | e66d304e05d834cdd99b641c4d7420c2db40714f   |
| Statut final     | **SUCCESS**                                |

---

## Contexte de la Release

Cette version unifie la gestion USB des buzzers en un point d'entrée unique (`USBConfigModal`), avec correction du cycle de vie du port série lors du flash firmware.

### Commits inclus (depuis v3.1.1)

| Commit  | Description |
|---------|-------------|
| e66d304 | fix(flash): Retry loop for version check (disconnect→1s→reconnect 115200→1s→AT+VERSION, max 5 attempts) |
| c30f9d7 | fix(flash): Increase delays before version check (3s reboot + 1.5s AT ready + 3s timeout) |
| 4ae541e | feat(flash): Auto-scroll logs, verify version via AT+VERSION after flash, green bar on success |
| f725cac | fix(hooks): Disconnect/reconnect/disconnect port after flash to ensure clean USB re-enumeration |
| 27fd6df | fix(hooks): Explicitly close SerialPort after flash to release USB before buzzer reboot |
| e5a67bb | chore(version): Align web/package.json to 3.1.2 |
| 77bc1fd | docs: Update documentation for v3.1.2 |
| 384129d | chore(test): Add missing test infrastructure (vitest + testing-library) |
| 68a2869 | test(components): Fix broken USBConfigModal test assertions |
| 3c9e51d | fix(components): Polish USBConfigModal flash handler |
| 48d9708 | test(components): Add USBConfigModal Flash Firmware unit tests |
| 017d85c | refactor(admin): Remove inline USB flash from ConfigPage, unify in USBConfigModal |
| 842cd98 | feat(components): Add flash firmware section to USBConfigModal |
| cb115e2 | chore(version): Bump to 3.1.2 |

### Fonctionnalités incluses

- **Modale USB unifiée** : Section "Flash Firmware" déplacée de `ConfigPage` vers `USBConfigModal`
- **Retry loop vérification version** : disconnect → 1s → reconnect 115200 → 1s → AT+VERSION, max 5 tentatives
- **Délais vérification version** : 3s attente reboot + 1.5s AT ready + 3s timeout (fiabilité accrue)
- **Auto-scroll logs flash** : Les logs de flash défilent automatiquement vers le bas
- **Vérification version post-flash** : AT+VERSION envoyé après flash, version vérifiée, barre verte si succès
- **Fix re-énumération USB** : Séquence disconnect/reconnect/disconnect après flash pour garantir une re-énumération propre du port
- **Fix SerialPort** : Fermeture explicite du port série après flash pour libérer l'USB avant reboot buzzer
- **ConfigPage épuré** : Suppression du bloc inline flash firmware
- **Tests unitaires** : Couverture complète des comportements `USBConfigModal`
- **Versions alignées** : `config.json` et `package.json` tous deux à 3.1.2

---

## Build Results

### Frontend (React)

| Champ        | Résultat                          |
|--------------|-----------------------------------|
| Commande     | `npm run build`                   |
| Statut       | SUCCESS                           |
| Bundle JS    | `index-DVDAUAmr.js` - 594.16 kB (gzip: 181.59 kB) |
| CSS          | `index-DXg0EGiG.css` - 192.81 kB (gzip: 30.01 kB)  |
| Durée        | 2.47s                             |
| Avertissement| Chunk > 500 kB (connu, non bloquant) |

### Backend Go - Windows (amd64)

| Champ        | Résultat                          |
|--------------|-----------------------------------|
| Commande     | `go build -o server.exe ./cmd/server` |
| Statut       | SUCCESS                           |
| Taille       | 21 MB                             |
| Timestamp    | 2026-02-23 14:12                  |
| Fichier      | `server-go/server.exe`            |

### Backend Go - Linux ARM64

| Champ        | Résultat                          |
|--------------|-----------------------------------|
| Commande     | `GOOS=linux GOARCH=arm64 go build -o buzzcontrol ./cmd/server` |
| Statut       | SUCCESS                           |
| Taille       | 20 MB                             |
| Fichier      | `server-go/buzzcontrol`           |

---

## Vérifications Post-Build

| Vérification                              | Résultat  |
|-------------------------------------------|-----------|
| Serveur précédent arrêté via `/shutdown`   | PASS      |
| Serveur v3.1.2 démarré (fenêtre visible)   | PASS      |
| `GET /version` → `3.1.2`                 | PASS      |
| `GET /` → HTTP 200                        | PASS      |
| `GET /admin/config` → HTTP 200            | PASS      |

---

## Vérifications Fonctionnelles QUALIF

Les vérifications suivantes doivent être effectuées manuellement par le QA/utilisateur :

| Vérification                                                        | Statut attendu |
|---------------------------------------------------------------------|----------------|
| Interface admin accessible sur `/admin/config`                      | A VERIFIER     |
| Section "Firmware Buzzers" affiche le bouton "Flash via USB"        | A VERIFIER     |
| Clic sur "Flash via USB" ouvre la modale `USBConfigModal`           | A VERIFIER     |
| Modale affiche la section flash firmware en bas                     | A VERIFIER     |
| Flash firmware fonctionne via USB (si buzzer connecté)              | A VERIFIER     |
| Port série libéré correctement après flash (buzzer reboot OK)       | A VERIFIER     |
| Re-énumération USB propre après flash (disconnect/reconnect/disconnect) | A VERIFIER |
| Logs flash défilent automatiquement (auto-scroll)                   | A VERIFIER     |
| Barre verte affichée après flash réussi avec version vérifiée       | A VERIFIER     |
| AT+VERSION retourne la bonne version firmware après flash           | A VERIFIER     |
| Ancienne duplication de code flash absente dans ConfigPage          | A VERIFIER     |

---

## Verification Pre-Build

| Champ           | Valeur     |
|-----------------|------------|
| config.json     | 3.1.2 (OK) |
| package.json    | 3.1.2 (OK) |
| Alignement      | CONFORME   |

---

## Git Operations

Conformément à la procédure QUALIF :
- **Pas de merge vers main** (QUALIF uniquement)
- **Pas de tag Git** (QUALIF uniquement)
- Branche conservée : `feature/usb-unified-modal`

---

## Plan de Rollback

En cas de problème :
1. Arrêter le serveur : `curl http://localhost/shutdown`
2. Restaurer le binaire précédent (v3.1.1 disponible sur GitHub Releases)
3. Ou retourner sur la branche stable : `git checkout main`

---

## Décision Finale

**QUALIF : SUCCESS**

Le serveur v3.1.2 est démarré et répond correctement. Les builds Windows et Linux ARM64 sont disponibles. Le serveur reste actif pour les tests de validation manuelle de l'utilisateur.

**Action requise** : Validation manuelle des vérifications fonctionnelles, en particulier :
- Auto-scroll des logs de flash
- Barre verte + vérification version via AT+VERSION après flash réussi (commit 4ae541e)
- Re-énumération USB propre après flash (commit f725cac)
