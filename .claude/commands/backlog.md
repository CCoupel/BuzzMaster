# Commande /backlog - Gestion du Backlog

## Références

**Contexte projet :** Voir `context/COMMON.md` section 1

## Argument reçu

$ARGUMENTS

## Mot-clé help

`/backlog help` → Affiche :

```
## /backlog - Aide

**Description** : Gestion du backlog via GitHub Issues

**Usage** :
  /backlog help                  Afficher cette aide
  /backlog                       Afficher tableau synthétique TODO/En-Cours
  /backlog "Description feature" Créer nouvelle issue GitHub enhancement

**Source** :
  GitHub Issues avec label "backlog" → https://github.com/CCoupel/BuzzMaster/issues?q=label%3Abacklog
  Spécifications détaillées → backlog/TODO/, backlog/En-Cours/
```

## Source de vérité

Le backlog est géré via **GitHub Issues** avec le label `backlog` :
- Issues ouvertes avec label `TODO` : Fonctionnalités planifiées
- Issues ouvertes avec label `En-Cours` : Implémentation en cours
- Issues fermées avec label `DONE` : Complétées (voir CHANGELOG.md)
- Issues fermées avec label `REMOVED` : Abandonnées

Les fichiers dans `backlog/` contiennent les **spécifications détaillées** référencées par les issues.

## Comportement

### Si aucun argument fourni → Afficher le backlog synthétique

1. Lister les issues GitHub ouvertes avec le label `backlog` :
   ```bash
   gh issue list --repo CCoupel/BuzzMaster --label backlog --state open --json number,title,labels,body --limit 50
   ```
2. Si `gh` n'est pas disponible ou authentifié, **fallback** sur les fichiers locaux :
   - Lire `backlog/README.md` pour identifier les fichiers TODO et En-Cours
   - Lire CHAQUE fichier pour extraire : nom, description courte, phases restantes
3. **NE PAS lire les fichiers DONE** (consulter CHANGELOG.md pour l'historique)

4. Afficher sous forme de **TABLEAU SYNTHÉTIQUE** :

```
## Backlog BuzzControl

### ⏳ EN COURS

| # | Feature | Description | Phases restantes |
|---|---------|-------------|------------------|
| #5 | memory-game Phase 7 | Modes de scoring avancés | Phase 7 |

### 📋 PLANIFIÉ

| # | Feature | Description | Labels |
|---|---------|-------------|--------|
| #1 | generateur-ia | Générateur de jeu via IA | ai |
| #2 | metadata-binaires | Métadonnées dans binaires | backend, ci-cd |
| #3 | usb-modal-layout-compact | Layout modale USB compact | frontend |
| #4 | websocket-broadcast-filtre | Filtrage broadcasts WebSocket | backend |

---
📊 Total : X en cours, Y planifiées
🔗 GitHub Issues : https://github.com/CCoupel/BuzzMaster/issues?q=label%3Abacklog
📜 Pour l'historique complété : voir CHANGELOG.md
```

**RÈGLES D'AFFICHAGE :**
- Tableau compact, une ligne par feature
- Inclure le numéro d'issue GitHub (#N)
- NE PAS détailler le contenu de chaque phase
- NE PAS afficher les entrées DONE

### Si argument fourni → Créer une nouvelle issue GitHub

**ÉTAPE PRÉLIMINAIRE** : Vérifier si une issue existante correspond au sujet

1. Chercher dans les issues ouvertes :
   ```bash
   gh issue list --repo CCoupel/BuzzMaster --label backlog --state open --json number,title --limit 50
   ```
2. Identifier si une issue correspond au sujet
3. **Si correspondance trouvée** → Demander à l'utilisateur :
   - "L'issue #N `<titre>` semble correspondre. Voulez-vous :"
   - Option A : Mettre à jour l'issue existante
   - Option B : Créer une nouvelle issue séparée
4. **Si aucune correspondance** → Créer une nouvelle issue

**PROCESSUS DE CRÉATION** :

1. Générer un nom de fichier à partir de la description (kebab-case)
2. Créer l'issue GitHub :
   ```bash
   gh issue create --repo CCoupel/BuzzMaster \
       --title "<Titre de la fonctionnalité>" \
       --label "enhancement,backlog,TODO" \
       --body "## Description

   <Description fournie par l'utilisateur>

   ## Objectifs

   - [ ] À définir

   ## Tâches

   ### Phase 1
   - [ ] À définir

   ## Spécification détaillée

   📄 Voir [`backlog/TODO/<nom>.md`](https://github.com/CCoupel/BuzzMaster/blob/main/backlog/TODO/<nom>.md)

   ## Version cible

   À déterminer"
   ```
3. Optionnellement, créer le fichier de spécification `backlog/TODO/<nom>.md`
4. Mettre à jour `backlog/README.md` si un fichier de spec a été créé
5. **Afficher un résumé** de ce qui a été créé (numéro d'issue, lien)
6. **Demander confirmation** à l'utilisateur avant de commit et push

## Exemples

### Mode lecture

```
/backlog
```

→ Affiche le tableau synthétique des issues TODO et En-Cours

### Mode ajout

```
/backlog Mode sombre pour l'interface admin
```

→ Crée l'issue GitHub #N et optionnellement `backlog/TODO/mode-sombre-admin.md`

## Légende des statuts

- ⏳ **En cours** : Implémentation en cours (label `En-Cours`)
- 📋 **Planifié** : Non démarré (label `TODO`)

## Ce que cette commande NE FAIT PAS

- ❌ Ne liste pas les features DONE (voir CHANGELOG.md)
- ❌ Ne détaille pas le contenu de chaque phase (lire le fichier directement)
- ❌ Ne modifie pas les issues fermées

## Fallback sans gh

Si `gh` n'est pas disponible ou authentifié, la commande utilise les fichiers locaux `backlog/` comme source de vérité de secours. Elle affiche un avertissement :

```
⚠️ GitHub CLI non disponible. Affichage depuis les fichiers locaux backlog/.
Pour synchroniser avec GitHub Issues : gh auth login && ./scripts/create-github-issues.sh
```

## Commence maintenant

**Argument reçu** : $ARGUMENTS

- Si vide → Lister les issues GitHub (ou fallback fichiers locaux), afficher tableau synthétique
- Si texte → Créer une nouvelle issue GitHub avec label backlog et optionnellement un fichier spec
