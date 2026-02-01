# Plan d'Implémentation : Auto-Update (v2.50.0)

**Date** : 2026-02-01
**Feature** : Mise à jour automatique du serveur
**Version** : 2.50.0
**Status** : En attente de validation
**Branche** : feature/auto-update

---

## 1. Analyse des Dépendances

### Dépendances Détectées

| Composant | Dépend de | Raison |
|-----------|----------|--------|
| Frontend (UpdatePage) | Backend API | Nouvelles actions REST (/api/updates/*) |
| Frontend (Navbar badge) | Backend API | GET /api/updates/check |
| Frontend (UpdatePage) | Contrats API | Structure réponses JSON |

**Type de dépendance** : SÉQUENTIEL
- Backend doit implémenter les 4 endpoints REST AVANT que frontend les consomme
- Frontend peut être parallélisé après backend si indépendant des autres changements

### Contrats API (Draft - à valider)

Fichier : `contracts/auto-update-endpoints.md`

**Endpoints** :
1. `GET /api/updates` - Liste les versions disponibles avec cache 1h
2. `GET /api/updates/check` - Vérification rapide (update_available, current, latest)
3. `POST /api/updates/download` - Télécharge binaire spécifique
4. `POST /api/updates/apply` - Applique mise à jour et redémarre

**Config** :
```json
{
  "server": {
    "auto_check_updates": true
  }
}
```

---

## 2. Breakdown des Tâches

### Phase 1 : Backend API - Gestion des versions

**Fichiers à créer** :
- `server-go/internal/server/updater.go` - Logique de mise à jour
- `server-go/internal/server/github_client.go` - Client GitHub API (optionnel)

**Fichiers à modifier** :
- `server-go/cmd/server/main.go` - Routes /api/updates/*
- `server-go/internal/config/config.go` - Option auto_check_updates
- `server-go/internal/server/server.go` - Enregistrement des routes

**Tâches dev-backend** :
1. [ ] Créer structure `UpdateInfo` et `VersionsResponse`
2. [ ] Implémenter client GitHub API avec cache 1h (GET releases)
3. [ ] Endpoint GET /api/updates - Lister versions
4. [ ] Endpoint GET /api/updates/check - Vérification rapide
5. [ ] Détection plateforme (runtime.GOOS, runtime.GOARCH)
6. [ ] Filtrer asset approprié (windows-amd64 ou linux-arm64)
7. [ ] Ajouter config option auto_check_updates (défaut: true)
8. [ ] Tests unitaires pour GitHub client
9. [ ] Tests unitaires pour endpoints

**Estimé** : 3-4 heures

---

### Phase 2 : Backend Téléchargement & Redémarrage

**Fichiers à créer/modifier** :
- `server-go/internal/server/updater.go` - Expansion (download/apply)
- `server-go/internal/utils/file_backup.go` - Backup/restore utilitaires

**Fichiers à modifier** :
- `server-go/cmd/server/main.go` - Routes POST /api/updates/download, /api/updates/apply
- `server-go/internal/game/engine.go` - SaveState/RestoreState pour redémarrage

**Tâches dev-backend** :
1. [ ] Endpoint POST /api/updates/download
   - Paramètre version
   - Télécharge binaire depuis GitHub
   - Vérifie intégrité (size minimum > 40MB)
   - Retourne path, size, checksum
2. [ ] Endpoint POST /api/updates/apply
   - Paramètre version et path
   - Sauvegarde état du jeu si actif
   - Backup exécutable actuel (.bak ou .old)
   - Remplacement atomique (rename)
   - Démarrage nouvel exécutable
   - Attendre 5 secondes de succès
   - Rollback automatique si crash
3. [ ] Mécanisme graceful shutdown
   - Notifier clients WebSocket
   - Fermer connexions proprement
   - Attendre 2 secondes max
4. [ ] Mécanisme de restauration d'état
   - Charger état sauvegardé après redémarrage
   - Notifier clients de reconnexion
5. [ ] Tests d'intégrité des binaires
6. [ ] Gestion erreurs réseau (GitHub inaccessible)
7. [ ] Logs détaillés de l'opération
8. [ ] Tests unitaires

**Estimé** : 4-5 heures

---

### Phase 3 : Frontend - Interface de Mise à Jour

**Fichiers à créer** :
- `server-go/web/src/pages/UpdatePage.jsx` - Page gestion des mises à jour
- `server-go/web/src/hooks/useUpdates.js` - Hook personnalisé
- `server-go/web/src/components/UpdateModal.jsx` - Modal update (optionnel)

**Fichiers à modifier** :
- `server-go/web/src/components/Navbar.jsx` - Badge notification
- `server-go/web/src/App.jsx` - Route /admin/updates (optionnel)

**Tâches dev-frontend** :
1. [ ] Hook useUpdates.js
   - GET /api/updates - récupérer liste
   - GET /api/updates/check - vérification rapide
   - POST /api/updates/download - télécharger
   - POST /api/updates/apply - appliquer
   - Gestion loading/error/success
   - Retry logic

2. [ ] Composant Badge Navbar
   - Afficher badge si update_available = true
   - Badge "1" ou indicateur visuel
   - Clic → accès à UpdatePage
   - Check on page load

3. [ ] Page UpdatePage.jsx
   - Version actuelle vs dernière
   - Dropdown liste versions
   - Afficher notes de release (CHANGELOG)
   - Download button + barre progression
   - "Appliquer et redémarrer" button
   - Warning si jeu en cours
   - Disable buttons si déjà en téléchargement/application

4. [ ] Styles & UX
   - Cohérence avec design système
   - États: idle, downloading, downloaded, applying, error
   - Messages utilisateur clairs

5. [ ] Comportement redémarrage
   - Après POST /api/updates/apply : attendre reconnexion
   - Poll /api/updates/check toutes les 2 secondes
   - Rechargement page auto quand serveur revient
   - Afficher version mise à jour

6. [ ] Tests React (composants)

**Estimé** : 2-3 heures

---

### Phase 4 : Tests & Sécurité

**Tâches test-writer** :
1. [ ] Tests unitaires Go
   - GitHub client (mock API)
   - Endpoint /api/updates
   - Endpoint /api/updates/check
   - Endpoint /api/updates/download (mock download)
   - Endpoint /api/updates/apply (mock restart)
   - Platform detection
   - Cache invalidation

2. [ ] Tests React
   - Hook useUpdates
   - Badge display logic
   - UpdatePage rendering
   - Loading states

3. [ ] Tests E2E (Chrome)
   - Scénario 1 : Vérifier badge "update available"
   - Scénario 2 : Ouvrir UpdatePage et voir version
   - Scénario 3 : Télécharger version (sans appliquer)
   - Scénario 4 : Annuler téléchargement mid-way
   - Scénario 5 : Appliquer mise à jour et attendre reconnexion

**Estimé** : 2-3 heures

---

## 3. Stratégie Implémentation

### Ordre Séquentiel (DÉPENDANCE DÉTECTÉE)

```
Phase 1 : Backend API
    ↓
Phase 2 : Backend Download/Apply + Config
    ↓
Phase 3 : Frontend (UpdatePage, Badge, Hook)
    ↓
Phase 4 : Tests
    ↓
Phase 5 : Review
    ↓
Phase 6 : QA
```

### Raison du Séquentiel

Frontend dépend des 4 endpoints REST créés par backend. Les endpoints ne peuvent pas être mockés sans contrats API validés.

**Dépendance critique** :
- Frontend **doit attendre** les réponses réelles de /api/updates/check et /api/updates pour fonctionner
- Backend **peut modifier les contrats** si contrainte technique
- Frontend **consulte les contrats finaux** et implémente

---

## 4. Fichiers Impactés

### Création (6 fichiers)

| Fichier | Type | Responsable |
|---------|------|-------------|
| `internal/server/updater.go` | Go | dev-backend |
| `internal/server/github_client.go` | Go | dev-backend |
| `internal/utils/file_backup.go` | Go | dev-backend |
| `web/src/pages/UpdatePage.jsx` | React | dev-frontend |
| `web/src/hooks/useUpdates.js` | React | dev-frontend |
| `web/src/components/UpdateModal.jsx` | React (optionnel) | dev-frontend |

### Modification (5 fichiers)

| Fichier | Type | Responsable |
|---------|------|-------------|
| `cmd/server/main.go` | Go | dev-backend |
| `internal/config/config.go` | Go | dev-backend |
| `internal/server/server.go` | Go | dev-backend |
| `web/src/components/Navbar.jsx` | React | dev-frontend |
| `web/src/App.jsx` | React (optionnel) | dev-frontend |

### Validation (1 fichier)

| Fichier | Type | Action |
|---------|------|--------|
| `contracts/auto-update-endpoints.md` | Spécification | Refine + finalize |

---

## 5. Contrats API (À Valider)

Fichier : `contracts/auto-update-endpoints.md`

Endpoints définis :
- GET /api/updates
- GET /api/updates/check
- POST /api/updates/download
- POST /api/updates/apply

Config option :
- `server.auto_check_updates` (bool, défaut: true)

**Validation requise** : ✅ Le plan doit valider les contrats avant dev

---

## 6. Risques & Mitigations

| Risque | Sévérité | Mitigation |
|--------|----------|-----------|
| GitHub API rate limiting | MOYENNE | Cache 1h obligatoire, fallback graceful |
| Téléchargement corrompu | ÉLEVÉE | Vérifier size min (40MB), tester exécution |
| Nouveau serveur crash | CRITIQUE | Rollback automatique après 5s timeout |
| Perte état jeu | CRITIQUE | Save state avant redémarrage, restore après |
| Fichier verrouillé Windows | HAUTE | Atomic rename (.bak → old, new → active) |
| Frontend timeout reconnexion | MOYENNE | Retry logic 2s × 30 tentatives |
| Permissions fichiers | MOYENNE | Vérifier droits R/W sur exécutable |

---

## 7. Timeline Estimation

| Phase | Tâche | Estimé |
|-------|-------|--------|
| **Phase 1** | Backend API | 3-4h |
| **Phase 2** | Backend Download/Apply | 4-5h |
| **Phase 3** | Frontend UI | 2-3h |
| **Phase 4** | Tests | 2-3h |
| **Review** | Code review | 1-2h |
| **QA** | Test execution | 1-2h |
| **Docs** | Documentation | 1h |
| **Deploy** | Deploy to QUALIF | 0.5h |
| **TOTAL** | | 15-20h |

---

## 8. Points de Validation

### Validation 1 : Plan
**À valider par l'utilisateur** :
- ✅ Dépendances backend → frontend
- ✅ Contrats API dans contracts/auto-update-endpoints.md
- ✅ Task breakdown
- ✅ Timeline réaliste

### Validation 2 : Backend
**Après dev-backend complète** :
- Endpoints fonctionnels
- Cache 1h implémenté
- Platform detection OK
- State save/restore OK

### Validation 3 : Frontend
**Après dev-frontend complète** :
- Badge visible
- UpdatePage fonctionnelle
- Download/Apply workflow OK

### Validation 4 : Tests
**Après test-writer complète** :
- Tests unitaires passent
- Tests React passent
- Scénarios E2E valides

### Validation 5 : Code Review
**Code review** :
- Quality pass
- No breaking changes

### Validation 6 : QA
**Exécution tests** :
- All tests pass
- Manual validation OK

---

## 9. Questions de Design (Décisions en attente)

1. **Frontend Check Interval** ?
   - Option A : Check on page load only
   - Option B : Check every 1 hour (background)
   - Option C : Check on app init + hourly (recommandé)

2. **auto_check_updates Default** ?
   - Option A : true (check enabled by default)
   - Option B : false (user opt-in)
   - Recommandé : true (transparent updates)

3. **Checksum Verification** ?
   - Option A : Just file size validation (< 40MB threshold)
   - Option B : SHA256 checksum from GitHub
   - Recommandé : SHA256 (security)

4. **Rollback Strategy** ?
   - Option A : Automatic (if new server fails)
   - Option B : Manual user action
   - Recommandé : Automatic (safety)

5. **Restart Timeout** ?
   - Suggested : 5 seconds
   - If not started in 5s → rollback to old binary

---

## 10. Prochaines Étapes

1. **✅ CDP** : Validation du plan par l'utilisateur
2. **Phase 2** : dev-backend - Implémenter Phase 1 + 2 (API + Download/Apply)
3. **Phase 3** : dev-frontend - Implémenter UpdatePage, Badge, Hook
4. **Phase 4** : test-writer - Écrire tests unitaires + E2E
5. **Phase 5** : code-reviewer - Review code + tests
6. **Phase 6** : QA - Exécuter tests
7. **Phase 7** : doc-updater - CHANGELOG, documentation
8. **Phase 8** : deploy - Deploy to QUALIF

---

## Conclusion

Feature **"Auto-Update v2.50.0"** prête pour implémentation avec :
- ✅ Plan structuré
- ✅ Contrats API définis
- ✅ Dépendances identifiées (séquentiel backend → frontend)
- ✅ Timeline réaliste (15-20h)
- ✅ Risques mitigés
- ✅ Validation checkpoints clairs

**Prochaine étape** : Validation du plan par l'utilisateur ⏸️
