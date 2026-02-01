# Mise à jour automatique du serveur

**Statut** : 📋 Planifié

## Description

Permettre au serveur BuzzControl de détecter les nouvelles versions disponibles sur GitHub, de les télécharger directement depuis l'interface admin, et de relancer automatiquement le serveur sur la nouvelle version.

## Objectifs

- [ ] Lister les versions disponibles sur GitHub (releases)
- [ ] Télécharger le binaire approprié (Windows/ARM64) directement depuis l'admin
- [ ] Remplacer l'exécutable actuel par la nouvelle version
- [ ] Relancer automatiquement le serveur sur la nouvelle version
- [ ] Afficher une notification quand une mise à jour est disponible

## Contexte technique

### API GitHub Releases
```
GET https://api.github.com/repos/CCoupel/BuzzMaster/releases
GET https://api.github.com/repos/CCoupel/BuzzMaster/releases/latest
```

Chaque release contient :
- `tag_name` : Version (ex: "v2.49.0")
- `body` : Notes de release (CHANGELOG)
- `assets[]` : Binaires téléchargeables
  - `buzzcontrol-vX.Y.Z-windows-amd64.exe`
  - `buzzcontrol-vX.Y.Z-linux-arm64`

### Détection de la plateforme
Le serveur connaît sa plateforme via `runtime.GOOS` et `runtime.GOARCH` :
- Windows : `windows` + `amd64`
- Raspberry Pi : `linux` + `arm64`

## Tâches

### Phase 1 - Backend : API de gestion des versions
- [ ] Endpoint `GET /api/updates` : Liste des versions disponibles sur GitHub
  - Retourne : `[{ version, date, notes, download_url, current: bool }]`
  - Filtre l'asset correspondant à la plateforme actuelle
  - Cache 1 heure (éviter rate limiting GitHub)
- [ ] Endpoint `GET /api/updates/check` : Vérification rapide
  - Retourne : `{ update_available, current, latest, release_url }`
- [ ] Détecter automatiquement la plateforme pour sélectionner le bon binaire

### Phase 2 - Backend : Téléchargement et mise à jour
- [ ] Endpoint `POST /api/updates/download` : Télécharge une version spécifique
  - Paramètre : `{ version: "2.50.0" }`
  - Télécharge le binaire dans un dossier temporaire
  - Vérifie l'intégrité (taille minimale, exécutable valide)
  - Retourne : `{ success, path, size }`
- [ ] Endpoint `POST /api/updates/apply` : Applique la mise à jour
  - Remplace l'exécutable actuel (renommage atomique)
  - Déclenche le redémarrage du serveur
- [ ] Mécanisme de redémarrage gracieux :
  - Sauvegarder l'état du jeu si en cours
  - Arrêter proprement les connexions WebSocket
  - Relancer le nouvel exécutable
  - Restaurer l'état du jeu

### Phase 3 - Frontend : Interface de mise à jour
- [ ] Badge notification dans Navbar si mise à jour disponible
- [ ] Page/Modal de gestion des mises à jour :
  - Version actuelle vs dernière version
  - Liste des versions disponibles (dropdown)
  - Notes de release (CHANGELOG)
  - Bouton "Télécharger" avec barre de progression
  - Bouton "Appliquer et redémarrer"
  - Avertissement si jeu en cours

### Phase 4 - Sécurité et robustesse
- [ ] Backup de l'ancien exécutable avant remplacement
- [ ] Rollback automatique si le nouveau serveur ne démarre pas
- [ ] Vérification de signature/checksum (optionnel)
- [ ] Option dans config : `auto_check_updates: true/false`
- [ ] Gestion des erreurs réseau (GitHub inaccessible)

## Workflow utilisateur

```
1. Notification badge "Nouvelle version disponible" dans navbar
                    ↓
2. Clic → Modal avec détails de la version
   - Version actuelle : 2.49.0
   - Nouvelle version : 2.50.0
   - Notes de release
                    ↓
3. Bouton "Télécharger v2.50.0"
   - Barre de progression
   - "Téléchargement terminé ✓"
                    ↓
4. Bouton "Appliquer et redémarrer"
   - Confirmation si jeu en cours
   - "Le serveur va redémarrer..."
                    ↓
5. Rechargement automatique de la page
   - Connexion au nouveau serveur
   - Version 2.50.0 active
```

## Fichiers concernés

| Fichier | Modification |
|---------|--------------|
| `cmd/server/main.go` | Endpoints `/api/updates/*` |
| `internal/server/updater.go` | Logique de mise à jour (nouveau) |
| `internal/config/config.go` | Option `auto_check_updates` |
| `web/src/components/Navbar.jsx` | Badge notification |
| `web/src/pages/UpdatePage.jsx` | Page de mise à jour (nouveau) |
| `web/src/hooks/useUpdates.js` | Hook de gestion (nouveau) |

## Considérations

- **Rate limiting GitHub API** : 60 req/h sans auth → cache obligatoire
- **Permissions fichiers** : L'exécutable doit pouvoir se remplacer lui-même
- **Windows** : Fichier verrouillé pendant exécution → renommer puis relancer
- **Linux** : Plus simple, remplacement direct possible
- **Rollback** : Conserver l'ancienne version pour récupération

## Risques

| Risque | Mitigation |
|--------|------------|
| Téléchargement corrompu | Vérifier taille minimale + test exécution |
| Nouveau serveur crash | Rollback automatique après timeout |
| Perte état jeu | Sauvegarde/restauration état avant redémarrage |
| GitHub inaccessible | Mode dégradé, pas de blocage |

## Version cible

v2.50.0
