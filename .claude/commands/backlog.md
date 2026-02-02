# Commande /backlog - Gestion du Backlog

## Références

**Contexte projet :** Voir `context/COMMON.md` section 1

## Argument reçu

$ARGUMENTS

## Structure du backlog

Le backlog est organisé en fichiers séparés dans le dossier `backlog/` :
- `backlog/TODO/` : Fonctionnalités planifiées
- `backlog/En-Cours/` : Implémentation en cours
- `backlog/DONE/` : Complétées (NON GÉRÉES par cette commande)

## Comportement

### Si aucun argument fourni → Afficher le backlog synthétique

1. Lire `backlog/README.md` pour identifier les fichiers TODO et En-Cours
2. Lire CHAQUE fichier pour extraire : nom, description courte, phases restantes
3. **NE PAS lire les fichiers DONE** (consulter CHANGELOG.md pour l'historique)

4. Afficher sous forme de **TABLEAU SYNTHÉTIQUE** :

```
## Backlog BuzzControl

### ⏳ EN COURS

| Feature | Description | Phases restantes | Version |
|---------|-------------|------------------|---------|
| memory-game | Jeu de mémoire avec paires | Phase 5 (1 tâche), Phase 6, Phase 7 | v2.33.0 |

### 📋 PLANIFIÉ

| Feature | Description | Phases | Cible |
|---------|-------------|--------|-------|
| websocket-broadcast-filtre | Filtrage broadcasts WebSocket | 3 phases | v2.47.0 |
| qcm-marqueurs-indices | Marqueurs indices sur barre temps | 3 phases | v2.42.0 |
| generateur-ia | Générateur de jeu via IA | 6 phases | - |
| metadata-binaires | Métadonnées dans binaires | 3 phases | v2.47.0 |
| bugfix-neon-effet-parametres | Bugfix paramètres néon | 1 phase | v2.46.1 |
| navbar-menu-connexion | Menu déroulant pastille connexion | 2 phases | v2.47.0 |
| admin-joueur-card-style | Style neutre cartes joueurs | 1 phase | v2.48.0 |

---
📊 Total : X en cours, Y planifiées
💡 Pour les détails d'une feature : lire `backlog/TODO/<nom>.md` ou `backlog/En-Cours/<nom>.md`
📜 Pour l'historique complété : voir CHANGELOG.md
```

**RÈGLES D'AFFICHAGE :**
- Tableau compact, une ligne par feature
- Colonne "Phases restantes" : liste courte (ex: "Phase 5, 6, 7" ou "3 phases")
- NE PAS détailler le contenu de chaque phase
- NE PAS afficher les entrées DONE

### Si argument fourni → Ajouter au backlog

**ÉTAPE PRÉLIMINAIRE** : Vérifier si une entrée existante correspond au sujet

1. Lire `backlog/README.md` pour lister les entrées TODO et En-Cours
2. Identifier si une entrée correspond au sujet
3. **Si correspondance trouvée** → Demander à l'utilisateur :
   - "Une entrée existante `backlog/TODO/<nom>.md` semble correspondre. Voulez-vous :"
   - Option A : Mettre à jour l'entrée existante
   - Option B : Créer une nouvelle entrée séparée
4. **Si aucune correspondance** → Créer une nouvelle entrée

**PROCESSUS DE CRÉATION** :

1. Générer un nom de fichier à partir de la description (kebab-case)
2. Créer le fichier `backlog/TODO/<nom>.md` avec le template :

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
4. **Afficher un résumé** de ce qui a été créé
5. **Demander confirmation** à l'utilisateur avant de commit et push
6. Si confirmé → Commit et push

## Exemples

### Mode lecture

```
/backlog
```

→ Affiche le tableau synthétique des features TODO et En-Cours

### Mode ajout

```
/backlog Mode sombre pour l'interface admin
```

→ Crée `backlog/TODO/mode-sombre-admin.md` et met à jour le README

## Légende des statuts

- ⏳ **En cours** : Implémentation en cours
- 📋 **Planifié** : Non démarré

## Ce que cette commande NE FAIT PAS

- ❌ Ne liste pas les features DONE (voir CHANGELOG.md)
- ❌ Ne détaille pas le contenu de chaque phase (lire le fichier directement)
- ❌ Ne modifie pas les entrées DONE

## Commence maintenant

**Argument reçu** : $ARGUMENTS

- Si vide → Lire `backlog/README.md`, extraire TODO et En-Cours, afficher tableau synthétique
- Si texte → Créer un nouveau fichier backlog dans TODO/ et mettre à jour le README
