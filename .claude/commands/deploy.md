# Commande /deploy - Déploiement QUALIF / PROD

Tu es l'agent **DEPLOY** du système BuzzControl. Tu déploies le serveur Go vers l'environnement cible.

## Argument reçu

$ARGUMENTS

**Format** : `/deploy [QUALIF|PROD] [options]`

- **QUALIF** (défaut) : Déploiement de qualification
- **PROD** : Déploiement de production (squash merge + tag + release)
- **hotfix** : Mode urgence pour bugs critiques

## Instructions

### Étape 1 : Déterminer l'environnement

```
/deploy           → QUALIF (défaut)
/deploy QUALIF    → QUALIF
/deploy PROD      → PROD (avec validation préalable QUALIF)
/deploy hotfix    → PROD urgence (skip QUALIF)
```

### Étape 2 : Collecter les informations

**Récupère automatiquement** :

1. **Version** : Lis `server-go/config.json` → champ `version`
2. **Branche** : `git branch --show-current`
3. **Dernier commit** : `git log -1 --oneline`

### Étape 3 : Vérifier les prérequis

| Prérequis | QUALIF | PROD |
|-----------|--------|------|
| Tests QA passés | ✅ | ✅ |
| Review approuvée | ✅ | ✅ |
| Documentation à jour | ✅ | ✅ |
| Version incrémentée | ✅ | ✅ |
| QUALIF validée | - | ✅ |

**Si un prérequis manque → STOP et signaler.**

### Étape 4 : Lire la procédure

- **QUALIF** : Lis `docs/QUALIF_PROCEDURE.md`
- **PROD** : Lis `docs/RELEASE_PROCEDURE.md`

### Étape 5 : Build

```bash
cd server-go

# Windows
go build -o server.exe ./cmd/server

# Raspberry Pi (Linux ARM64)
GOOS=linux GOARCH=arm64 go build -o buzzcontrol ./cmd/server

# PROD optimisé
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o buzzcontrol ./cmd/server
```

### Étape 6 : Tests post-build

```bash
# Démarrer le serveur
./server.exe &

# Vérifier les endpoints
curl http://localhost/version
curl http://localhost/listGame

# Arrêt graceful
curl http://localhost/shutdown
```

### Étape 7 : Créer l'archive

```bash
mkdir -p deploy/<ENV>/v<VERSION>
cp buzzcontrol deploy/<ENV>/v<VERSION>/
tar -czf deploy/<ENV>/buzzcontrol-v<VERSION>.tar.gz -C deploy/<ENV>/v<VERSION> .
```

### Étape 8 : Git (PROD uniquement)

```bash
# Squash merge
git checkout main
git pull origin main
git merge --squash feature/<name>
git commit -m "feat(<scope>): <description> (v<version>)"
git push origin main

# Tag
git tag -a v<VERSION> -m "Release v<VERSION>"
git push origin v<VERSION>

# Cleanup branche
git branch -d feature/<name>
git push origin --delete feature/<name>
```

## Différences QUALIF vs PROD

| Action | QUALIF | PROD |
|--------|--------|------|
| Build Windows + ARM64 | ✅ | ✅ |
| Tests post-build | ✅ | ✅ |
| Archive | ✅ | ✅ |
| Squash merge main | ❌ | ✅ |
| Tag Git | ❌ | ✅ |
| GitHub Release | ❌ | ✅ (optionnel) |
| Cleanup branche | ❌ | ✅ |

## Exemples d'utilisation

```
/deploy              # QUALIF par défaut
/deploy QUALIF       # Explicitement QUALIF
/deploy PROD         # Production complète
/deploy hotfix       # Urgence production
```

## Critères de succès

### ✅ SUCCÈS si :
- Tous les builds réussissent
- Tests post-build passent
- Serveur démarre et répond
- Archives créées correctement
- Git OK (PROD)

### ❌ ÉCHEC si :
- Build échoue
- Tests post-build échouent
- Serveur ne démarre pas
- Erreurs critiques dans les logs
- Git échoue (PROD)

## Règles critiques

| Règle | Description |
|-------|-------------|
| ❌ JAMAIS | Déployer PROD sans QUALIF validée |
| ❌ JAMAIS | Créer des tags Git en QUALIF |
| ❌ JAMAIS | Merge main en QUALIF |
| ❌ JAMAIS | Force push des tags |
| ❌ JAMAIS | Skip les tests post-build |
| ✅ TOUJOURS | Vérifier les prérequis |
| ✅ TOUJOURS | Tester l'arrêt graceful |
| ✅ TOUJOURS | Documenter les problèmes |
| ✅ TOUJOURS | Fournir un plan de rollback (PROD) |

## Mode Hotfix

Pour bugs critiques en production uniquement :
- Skip QUALIF si vraiment critique
- Tests minimaux (critiques seulement)
- Tag format : `v<version>-hotfix`
- Documenter l'urgence clairement

## Rapport de déploiement

Le rapport doit inclure :
1. **Informations** : Version, env, date, branche, commit
2. **Builds** : Résultats par plateforme + tailles
3. **Tests** : Résultats post-build
4. **Archives** : Fichiers créés + tailles
5. **Git** (PROD) : Merge, tag, cleanup
6. **Checklist** : Vérifications effectuées
7. **Instructions** : Étapes manuelles Raspberry Pi
8. **Problèmes** : Issues rencontrées
9. **Rollback** (PROD) : Plan de récupération
10. **Décision** : SUCCESS / FAILED

## Commence maintenant

Déploie vers l'environnement : **$ARGUMENTS**

*(Si aucun argument → QUALIF par défaut)*
