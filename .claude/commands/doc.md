# Commande /doc - Mise à Jour de la Documentation

Tu es l'agent **Doc Updater** du système BuzzControl. Tu mets à jour la documentation après qu'une feature a été implémentée et validée.

## Argument reçu (optionnel)

$ARGUMENTS

## Instructions

### Étape 1 : Collecter les informations

**Si aucun argument n'est fourni**, récupère automatiquement :

1. **Version actuelle** : Lis `server-go/config.json` → champ `version`
2. **Branche courante** : `git branch --show-current`
3. **Commits récents** : `git log --oneline -20` pour identifier les changements
4. **Fichiers modifiés** : `git diff main --name-only` pour lister les fichiers impactés

**Si un argument est fourni**, utilise-le comme description de la feature :
```
/doc "Mode CHACUN_SON_TOUR pour Memory multi-équipes"
/doc bugfix "Correction du calcul des scores Memory"
```

### Étape 2 : Lire la procédure

Lis le fichier `.claude/agents/doc-updater.md` pour connaître les standards de documentation.

### Étape 3 : Analyser les changements

Identifie :
- **Type de changement** : Added / Changed / Fixed / Deprecated / Removed
- **Composants impactés** : Backend (Go), Frontend (React), Protocol, Models
- **Fichiers modifiés** : Liste complète depuis le diff avec main

### Étape 4 : Mettre à jour la documentation

| Fichier | Action |
|---------|--------|
| `CHANGELOG.md` | Ajouter l'entrée de version avec format Keep a Changelog |
| `CLAUDE.md` | Mettre à jour les sections impactées (Models, Protocol, UI) |
| `docs/ADMIN_GUIDE.md` | Documenter les fonctionnalités user-facing |
| `server-go/config.json` | Finaliser la version (reset z à 0) |

### Étape 5 : Commit et push

```bash
# Finaliser la version
git add server-go/config.json
git commit -m "docs(version): Finalize vX.Y.0"

# Commit documentation
git add CHANGELOG.md CLAUDE.md docs/ADMIN_GUIDE.md
git commit -m "docs: Update documentation for vX.Y.0"

# Push
git push origin <branche-courante>
```

## Inputs nécessaires

| Input | Source | Description |
|-------|--------|-------------|
| Version | `server-go/config.json` | Numéro de version actuel |
| Feature | Argument ou commits | Nom/description de la feature |
| Fichiers modifiés | `git diff main` | Liste des changements |
| Type | Déduit du contexte | feature / bugfix / hotfix |

## Format CHANGELOG.md

```markdown
## [X.Y.Z] - YYYY-MM-DD

### Added
- **[Composant]**: Description de la feature
  - Détail 1
  - Détail 2

### Changed
- **[Composant]**: Ce qui a changé

### Fixed
- **[Composant]**: Bug corrigé
```

## Règle de versionnement

| Contexte | Version finale |
|----------|----------------|
| **Feature** (minor) | Reset z à 0 → `2.40.0` |
| **Bugfix** (patch) | Garder z → `2.40.1` |
| **Breaking change** (major) | Incrémenter x → `3.0.0` |

## Exemples d'utilisation

```
/doc                                    # Auto-détecte depuis git
/doc "Nouveau mode Memory CHACUN_SON_TOUR"  # Feature spécifique
/doc bugfix "Correction affichage podium"   # Bugfix
/doc breaking "Migration WebSocket v2"      # Breaking change
```

## Checklist finale

Avant de terminer, vérifie :
- [ ] CHANGELOG.md : Entrée avec format correct et date du jour
- [ ] CLAUDE.md : Toutes les sections impactées mises à jour
- [ ] ADMIN_GUIDE.md : Instructions utilisateur claires (si applicable)
- [ ] config.json : Version finalisée (z=0 pour features)
- [ ] Pas de fautes ou erreurs Markdown
- [ ] Commits créés avec messages appropriés
- [ ] Push effectué sur la branche feature

## Commence maintenant

Collecte les informations et mets à jour la documentation pour : **$ARGUMENTS**

*(Si aucun argument, analyse les commits récents et fichiers modifiés)*
