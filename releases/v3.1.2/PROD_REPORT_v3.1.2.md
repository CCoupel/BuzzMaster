# Rapport de Déploiement PROD - BuzzControl v3.1.2

## Informations Déploiement

| Champ            | Valeur                                     |
|------------------|--------------------------------------------|
| Version          | 3.1.2                                      |
| Environnement    | PROD                                       |
| Date             | 2026-02-27                                 |
| Branche          | feature/usb-unified-modal                  |
| Commit HEAD      | b2f7fc9 (docs: Release v3.1.2)             |
| Tag Git          | v3.1.2                                     |
| Statut final     | **SUCCESS**                                |

---

## Résumé des Changements

### Ajouts
- **USBConfigModal unifiée** : config WiFi AT et flash firmware réunis dans une seule modale
- **Badge firmware type** dans ConfigPage (Full merged / App only)

### Corrections
- Bouton "Flash via USB" désactivé quand firmware app-only (`IS_MERGED=false`)
- Champ `IS_MERGED` propagé via WebSocket dans le handler `FIRMWARE_VERSION`
- Flash USB esptool-js : écriture depuis 0x0 + `hard_reset` + vérification `AT+VERSION` post-flash
- Retry loop vérification version : disconnect → 1s → reconnect 115200 → 1s → AT+VERSION, max 5 tentatives
- Délais vérification version augmentés : 3s reboot + 1.5s AT ready + 3s timeout

---

## Phase 1 : Arrêt Serveur

| Vérification | Résultat |
|--------------|----------|
| Serveur précédent arrêté via `/shutdown` | PASS |

---

## Phase 2 : Nettoyage

| Fichier              | Action   |
|----------------------|----------|
| `server-go/nul`      | Supprimé |
| `server-go/server-output.txt` | Supprimé |

---

## Phase 3 : Vérification Versions

| Fichier              | Version  | Statut   |
|----------------------|----------|----------|
| `server-go/config.json` | 3.1.2 | CONFORME |
| `server-go/web/package.json` | 3.1.2 | CONFORME |

---

## Phase 4 : Build Production

### Frontend (React)

| Champ        | Résultat                                               |
|--------------|--------------------------------------------------------|
| Commande     | `npm run build`                                        |
| Statut       | SUCCESS                                                |
| Bundle JS    | `index-ABYyqdyN.js` - 595.13 kB (gzip: 181.94 kB)    |
| CSS          | `index-CcMAZHMs.css` - 193.39 kB (gzip: 30.13 kB)    |
| Durée        | 2.38s                                                  |

### Backend Go - Windows (amd64) — optimisé production

| Champ        | Résultat                                                      |
|--------------|---------------------------------------------------------------|
| Commande     | `go build -ldflags="-s -w" -o server.exe ./cmd/server`        |
| Statut       | SUCCESS                                                       |
| Taille       | 17 MB (optimisé vs 21 MB dev)                                 |
| Timestamp    | 2026-02-27 18:05                                              |

### Backend Go - Linux ARM64 — optimisé production

| Champ        | Résultat                                                                          |
|--------------|-----------------------------------------------------------------------------------|
| Commande     | `GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o buzzcontrol ./cmd/server`  |
| Statut       | SUCCESS                                                                           |
| Taille       | 16 MB (optimisé)                                                                  |
| Timestamp    | 2026-02-27 18:05                                                                  |

### Vérification Locale

| Vérification              | Résultat |
|---------------------------|----------|
| Serveur démarré           | PASS     |
| `GET /version` → `3.1.2`  | PASS     |

---

## Phase 5 : Git Operations

| Opération                        | Résultat                                    |
|----------------------------------|---------------------------------------------|
| Push `feature/usb-unified-modal` | Already up-to-date                          |
| Squash merge → main              | Déjà effectué (commit `1b1e072` sur main)   |
| Tag `v3.1.2` local              | Existait déjà                               |
| Tag `v3.1.2` remote             | Existait déjà                               |
| Release GitHub v3.1.2           | Existante — 3 binaires disponibles          |

**Note** : Le merge et le tag ont été créés lors d'une opération précédente. L'état Git était cohérent à l'arrivée de ce déploiement.

---

## Phase 6 : CI GitHub Actions

| Champ            | Valeur                                                              |
|------------------|---------------------------------------------------------------------|
| Statut CI        | SUCCESS (release publiée avec 3 binaires)                           |
| Release URL      | https://github.com/CCoupel/BuzzMaster/releases/tag/v3.1.2          |

### Binaires Release GitHub

| Fichier                                  | Statut    |
|------------------------------------------|-----------|
| `buzzcontrol-v3.1.2-windows-amd64.exe`  | PRESENT   |
| `buzzcontrol-v3.1.2-linux-arm64`        | PRESENT   |
| `buzzclick-v3.1.2-merged.bin`           | PRESENT   |

---

## Phase 7 : Validation Release GitHub

| Vérification                                        | Résultat |
|-----------------------------------------------------|----------|
| Téléchargement `buzzcontrol-v3.1.2-windows-amd64.exe` (17 MB) | PASS |
| Démarrage exécutable release (fenêtre visible)      | PASS     |
| `GET /version` → `3.1.2`                           | PASS     |

---

## Plan de Rollback

En cas de problème critique :
1. Télécharger v3.1.1 : `https://github.com/CCoupel/BuzzMaster/releases/tag/v3.1.1`
2. Arrêter le serveur : `curl http://localhost/shutdown`
3. Remplacer `server.exe` par le binaire v3.1.1
4. Redémarrer

---

## Décision Finale

**PROD : SUCCESS**

Le serveur v3.1.2 tourne depuis l'exécutable de la release GitHub. Les 3 binaires sont disponibles sur GitHub Releases. La branche `feature/usb-unified-modal` est conservée.
