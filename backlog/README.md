# Backlog BuzzMaster

Ce dossier contient les spécifications détaillées de toutes les fonctionnalités du projet BuzzMaster, organisées par statut.

## Structure

```
backlog/
├── TODO/           # Fonctionnalités planifiées, pas encore démarrées
├── En-Cours/       # Implémentation en cours
├── DONE/           # Fonctionnalités complétées et livrées
└── README.md       # Ce fichier
```

## TODO (Planifié)

| Fichier | Description |
|---------|-------------|
| [websocket-broadcast-filtre.md](TODO/websocket-broadcast-filtre.md) | Filtrage des broadcasts WebSocket par type de client |
| [effet-neon-categorie.md](TODO/effet-neon-categorie.md) | Effet néon couleur catégorie sur TV et VJoueur |
| [qcm-marqueurs-indices.md](TODO/qcm-marqueurs-indices.md) | Marqueurs d'indices sur la barre de temps |
| [generateur-ia.md](TODO/generateur-ia.md) | Générateur de jeu complet via IA |
| [vjoueur-websocket-identification.md](TODO/vjoueur-websocket-identification.md) | Vérifier identification WebSocket des VJoueurs |
| [metadata-binaires.md](TODO/metadata-binaires.md) | Métadonnées dans les binaires (nom, version, icône) |

## En-Cours

| Fichier | Description |
|---------|-------------|
| [memory-game.md](En-Cours/memory-game.md) | Jeu de mémoire avec paires (Phases 1-5 complétées, une tâche restante) |

## DONE (Complété)

| Fichier | Version | Description |
|---------|---------|-------------|
| [tri-rapidite-reponse.md](DONE/tri-rapidite-reponse.md) | v2.44.1 | Tri équipes/joueurs par rapidité de buzz |
| [page-joueur.md](DONE/page-joueur.md) | v2.45.0 | Interface personnalisée pour jouer depuis smartphone (Phase 1) |
| [page-logs.md](DONE/page-logs.md) | v2.43.0 | Affichage des logs serveur en temps réel (WebSocket dédiée) |
| [mode-demo.md](DONE/mode-demo.md) | v2.40.0 | Mode démonstration avec données complètes |
| [qcm-indices-penalites.md](DONE/qcm-indices-penalites.md) | v2.38.0 | Indices automatiques pour QCM avec pénalités |
| [categories-questions.md](DONE/categories-questions.md) | v2.34.0 | Système de catégorisation et palmarès |
| [affichage-tv.md](DONE/affichage-tv.md) | v2.30.0 | Synchronisation des fonds d'écran |
| [timer-gameplay.md](DONE/timer-gameplay.md) | v2.29.0 | Décompte de préparation avant timer |
| [debug-tests.md](DONE/debug-tests.md) | v2.28.0 | Fonctionnalités de test sans buzzers |
| [gestion-scores.md](DONE/gestion-scores.md) | v2.18.0 | Points d'équipe dissociés des points joueurs |

## Légende des statuts

- **TODO** : Spécification validée, pas encore démarré
- **En-Cours** : Implémentation en cours
- **DONE** : Fonctionnalité implémentée et livrée

## Contribution

Pour ajouter une nouvelle fonctionnalité au backlog :

1. Créer un nouveau fichier `.md` dans le dossier `TODO/`
2. Utiliser le template suivant :

```markdown
# Nom de la fonctionnalité

**Statut** : TODO

## Description

[Description générale de la fonctionnalité]

## Objectifs

- [ ] Objectif 1
- [ ] Objectif 2

## Tâches

### Phase 1
- [ ] Tâche 1
- [ ] Tâche 2

## Version cible

vX.Y.Z
```

3. Mettre à jour ce README avec la référence au nouveau fichier
4. Committer les changements

## Cycle de vie d'une fonctionnalité

```
TODO/ ──────────► En-Cours/ ──────────► DONE/
       Démarrage            Mise en prod
```

### 1. Nouvelle fonctionnalité

**Action** : Créer le fichier dans `TODO/`

**Statut dans le fichier** :
```markdown
**Statut** : 📋 Planifié
```

### 2. Démarrage de l'implémentation

**Action** : Déplacer de `TODO/` vers `En-Cours/`

```bash
mv backlog/TODO/ma-feature.md backlog/En-Cours/
```

**Mise à jour du fichier** :
```markdown
**Statut** : ⏳ En cours (vX.Y.Z)
```

**Mise à jour des README** :
- Ce fichier : déplacer la ligne de la section TODO vers En-Cours
- `BACKLOG.md` : mettre à jour le lien

### 3. Feature complète et mise en production

**Action** : Déplacer de `En-Cours/` vers `DONE/`

```bash
mv backlog/En-Cours/ma-feature.md backlog/DONE/
```

**Mise à jour du fichier** :
```markdown
**Statut** : ✅ Complété (vX.Y.0)
```

**Mise à jour des README** :
- Ce fichier : déplacer la ligne vers DONE avec la version
- `BACKLOG.md` : mettre à jour le lien et ajouter la version

### Exemple complet

```bash
# 1. Démarrage de "QCM Marqueurs" pour v2.46.0
mv backlog/TODO/qcm-marqueurs-indices.md backlog/En-Cours/
# → Éditer le fichier : **Statut** : ⏳ En cours (v2.46.0)
# → Mettre à jour README.md et BACKLOG.md

# 2. Après /deploy PROD de v2.46.0
mv backlog/En-Cours/qcm-marqueurs-indices.md backlog/DONE/
# → Éditer le fichier : **Statut** : ✅ Complété (v2.46.0)
# → Mettre à jour README.md et BACKLOG.md
# → Commit : "docs(backlog): Move qcm-marqueurs-indices to DONE (v2.46.0)"
```

## Historique

- 2026-01-31 : Ajout filtrage broadcasts WebSocket par type de client
- 2026-01-30 : Ajout effet néon catégorie sur TV et VJoueur
- 2026-01-30 : Réorganisation du backlog en 3 dossiers (TODO, En-Cours, DONE)
- 2026-01-30 : Completion tri équipes/joueurs par rapidité de buzz (v2.44.1)
- 2026-01-26 : WebSocket dédiée pour logs (v2.43.0)
- 2026-01-26 : Completion page logs (v2.42.0)
- 2026-01-25 : Ajout tri équipes/joueurs par rapidité de buzz
- 2026-01-25 : Ajout marqueurs indices QCM sur barre de temps
- 2026-01-25 : Ajout de la page logs
- 2026-01-25 : Complétion page joueur (v2.41.0)
- 2026-01-23 : Ajout du mode demo (v2.40.0)
- 2026-01-20 : Création de la structure de backlog modulaire
- 2026-01-20 : Ajout du générateur IA
- 2026-01-20 : Ajout de la page joueur (/player)
