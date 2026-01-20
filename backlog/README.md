# Backlog BuzzMaster

Ce dossier contient les spécifications détaillées de toutes les fonctionnalités du projet BuzzMaster, organisées par fichier.

## Structure

Chaque fichier correspond à une fonctionnalité ou un ensemble de fonctionnalités cohérentes :

| Fichier | Statut | Description |
|---------|--------|-------------|
| [gestion-scores.md](gestion-scores.md) | ✅ v2.18.0 | Points d'équipe dissociés des points joueurs |
| [categories-questions.md](categories-questions.md) | ✅ v2.34.0 | Système de catégorisation et palmarès |
| [timer-gameplay.md](timer-gameplay.md) | ✅ v2.29.0 | Décompte de préparation avant timer |
| [qcm-indices-penalites.md](qcm-indices-penalites.md) | ⏳ v2.38.0 | Indices automatiques pour QCM avec pénalités |
| [debug-tests.md](debug-tests.md) | ✅ v2.28.0 | Fonctionnalités de test sans buzzers |
| [affichage-tv.md](affichage-tv.md) | ✅ v2.30.0 | Synchronisation des fonds d'écran |
| [memory-game.md](memory-game.md) | ✅ v2.33.0 | Jeu de mémoire avec paires |
| [generateur-ia.md](generateur-ia.md) | 📋 Planifié | Générateur de jeu complet via IA |

## Légende des statuts

- ✅ **Complété** : Fonctionnalité implémentée et livrée
- ⏳ **En cours** : Implémentation en cours
- 📋 **Planifié** : Spécification validée, pas encore démarré
- 🔮 **Idée** : Concept à explorer

## Contribution

Pour ajouter une nouvelle fonctionnalité au backlog :

1. Créer un nouveau fichier `.md` dans ce dossier
2. Utiliser le template suivant :

```markdown
# Nom de la fonctionnalité

**Statut** : 📋 Planifié

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

## Historique

- 2026-01-20 : Création de la structure de backlog modulaire
- 2026-01-20 : Ajout du générateur IA
