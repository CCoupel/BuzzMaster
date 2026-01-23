# Commande /backlog - Gestion du Backlog

## Argument reçu

$ARGUMENTS

## Structure du backlog

Le backlog est organisé en fichiers séparés dans le dossier `backlog/` :
- `backlog/README.md` : Index principal avec statuts
- `backlog/<nom-feature>.md` : Spécification détaillée par feature

## Comportement

### Si aucun argument fourni → Afficher le backlog

1. Lire le fichier `backlog/README.md`
2. Afficher un résumé structuré :

```
## Backlog BuzzControl

### ✅ Complété
- gestion-scores.md (v2.18.0)
- categories-questions.md (v2.34.0)
...

### ⏳ En cours
- qcm-indices-penalites.md (v2.38.0)

### 📋 Planifié
- page-joueur.md
- generateur-ia.md

### 🔮 Idées
- (aucune)
```

### Si argument fourni → Ajouter au backlog

1. Générer un nom de fichier à partir de la description (kebab-case)
2. Créer le fichier `backlog/<nom>.md` avec le template :

```markdown
# <Titre de la fonctionnalité>

**Statut** : 📋 Planifié

## Description

<Description fournie par l'utilisateur>

## Objectifs

- [ ] À définir

## Tâches

### Phase 1
- [ ] À définir

## Version cible

vX.Y.Z (à déterminer)
```

3. Mettre à jour `backlog/README.md` pour ajouter la référence
4. Commit et push

## Exemples

### Mode lecture

```
/backlog
```

→ Affiche le résumé du backlog avec tous les fichiers et leurs statuts

### Mode ajout

```
/backlog Mode sombre pour l'interface admin
```

→ Crée `backlog/mode-sombre-admin.md` et met à jour le README

## Légende des statuts

- ✅ **Complété** : Fonctionnalité implémentée et livrée
- ⏳ **En cours** : Implémentation en cours
- 📋 **Planifié** : Spécification validée, pas encore démarré
- 🔮 **Idée** : Concept à explorer

## Commence maintenant

**Argument reçu** : $ARGUMENTS

- Si vide → Lire `backlog/README.md` et afficher le résumé
- Si texte → Créer un nouveau fichier backlog et mettre à jour le README
