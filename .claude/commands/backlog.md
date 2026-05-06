# Commande /backlog

Consulter le backlog ou créer une nouvelle entrée (issue) dans le repo GitHub.
L'implémentation se fait via `/feature` ou `/bugfix` — `/backlog` gère uniquement les issues.

## Usage

```
/backlog                        # Lister toutes les issues ouvertes
/backlog <description>          # Créer une nouvelle issue dans le backlog
```

## Argument reçu

$ARGUMENTS

---

Si `$ARGUMENTS` est vide ou est un filtre (`label:`, `@me`, `milestone:`) → Mode 1 (liste).
Sinon → Mode 2 (création).

---

## Mode 1 : Liste (sans argument ou filtre)

### Etapes

1. Exécuter `gh issue list --state open --limit 50` avec les flags adaptés au filtre
2. Afficher sous forme de tableau :

```markdown
## Backlog — Issues ouvertes

| # | Titre | Labels | Milestone | Assignee | Maj |
|---|-------|--------|-----------|----------|-----|
| 42 | Ajouter auth OAuth2 | feature | v1.2 | - | 2025-01-10 |
| 38 | Crash au démarrage iOS | bug | v1.2 | @user | 2025-01-08 |
| 35 | Refactor module auth | refactor | — | - | 2025-01-05 |
```

3. Proposer les actions disponibles :

```
Pour créer une issue :
  /backlog <description>         Créer une nouvelle entrée dans le backlog

Pour implémenter une issue existante :
  /feature #42                   Lancer le workflow feature sur l'issue #42
  /bugfix #38                    Lancer le workflow bugfix sur l'issue #38
```

### Filtres disponibles

| Syntaxe | Effet |
|---------|-------|
| `/backlog` | Toutes les issues ouvertes |
| `/backlog label:bug` | Issues avec le label "bug" |
| `/backlog label:feature` | Issues avec le label "feature" |
| `/backlog @me` | Issues qui me sont assignées |
| `/backlog milestone:<nom>` | Issues d'un milestone |

**Implémentation des filtres :**
Détecter si `$ARGUMENTS` correspond à un filtre (`label:`, `@me`, `milestone:`)
et traduire en flags `gh issue list` correspondants (`--label`, `--assignee @me`, `--milestone`).

---

## Mode 2 : Création d'une issue

### Etape 0 — Vérifier les doublons et contradictions

```bash
gh issue list --state open --search "<mots-clés extraits de $ARGUMENTS>" --limit 10 \
  --json number,title,labels,state
```

Analyser les résultats :

| Situation | Action |
|-----------|--------|
| Aucune issue similaire | Continuer vers Etape 1 |
| Issue(s) similaire(s) trouvée(s) | Afficher et demander confirmation |
| Issue contradictoire trouvée | Signaler et demander comment procéder |

**Si similaire(s) détectée(s) :**
```
⚠ Des issues existantes ressemblent à votre demande :

  #42 — Ajouter auth OAuth2  [feature]
  #51 — Support login Google  [feature]

C'est une nouvelle issue distincte, ou l'une de ces issues couvre déjà le besoin ?
  1. C'est distinct — créer une nouvelle issue
  2. #42 couvre le besoin — utiliser cette issue
  3. #51 couvre le besoin — utiliser cette issue
```

Si l'utilisateur choisit une issue existante → afficher la confirmation et s'arrêter :
```
Issue existante sélectionnée : #<numero> — <titre>
Pour l'implémenter : /feature #<numero>   ou   /bugfix #<numero>
```

**Si contradiction détectée** (ex : demande de supprimer quelque chose qu'une autre issue demande d'ajouter) :
```
⚠ Contradiction détectée avec une issue existante :

  #38 — Ajouter le module de cache Redis  [feature, in-progress]

Votre demande semble en conflit avec cette issue. Comment procéder ?
  1. Créer quand même la nouvelle issue
  2. Commenter sur #38 pour signaler le conflit
  3. Annuler
```

Si l'utilisateur choisit 2 → ajouter un commentaire :
```bash
gh issue comment 38 --body "⚠ Conflit potentiel avec : <$ARGUMENTS>"
```

### Etape 1 — Inférer le type (bug ou feature)

Analyser `$ARGUMENTS` pour détecter le type :

| Indices dans la description | Type inféré |
|-----------------------------|-------------|
| "crash", "erreur", "bug", "ne fonctionne pas", "broken", "fix" | `bug` |
| "ajouter", "nouveau", "feature", "améliorer", "implémenter", "support" | `feature` |
| Ambigu ou aucun indice clair | Demander |

Si ambigu :
```
Cette issue est un bug ou une feature ?
  1. feature
  2. bug
```
Attendre la réponse avant de continuer.

### Etape 2 — Associer à un milestone

Récupérer les milestones ouverts :

```bash
gh api repos/{owner}/{repo}/milestones \
  --jq '.[] | select(.state=="open") | {number, title, open_issues, closed_issues}'
```

Afficher et demander :

```
Milestones disponibles :
  1. v1.2  (3 issues ouvertes)
  2. v1.3  (0 issues ouvertes)
  3. Aucun milestone

Associer cette issue à quel milestone ? [1/2/3]
```

Si aucun milestone ouvert :
```
Aucun milestone actif. Créer un milestone d'abord avec /milestone new <version>, ou continuer sans.
Continuer sans milestone ? [O/n]
```

### Etape 3 — Créer l'issue

```bash
gh issue create \
  --title "<$ARGUMENTS>" \
  --label "<bug|feature>" \
  --milestone "<version>" \   # omis si aucun milestone sélectionné
  --body ""
```

### Etape 4 — Confirmation

```
✅ Issue créée : #<numero> — <titre>
   Label     : <bug|feature>
   Milestone : <version ou "aucun">
   URL       : https://github.com/{owner}/{repo}/issues/<numero>

Pour l'implémenter :
  /feature #<numero>   ou   /bugfix #<numero>
```

---

## Exemples

```bash
/backlog                              # Lister toutes les issues ouvertes
/backlog label:bug                    # Lister les issues taguées "bug"
/backlog @me                          # Lister les issues qui me sont assignées
/backlog milestone:v1.2               # Lister les issues du milestone v1.2
/backlog Ajouter authentification OAuth2    # Créer une issue feature
/backlog Crash au login avec Google         # Créer une issue bug
```

---

## Prérequis

**Référence** : Voir `context/GITHUB.md` sections 1 (auth), 2 (issues), 3 (milestones)

- CLI GitHub (`gh`) installé et authentifié (`gh auth login`)
- Le projet doit être un repo GitHub (remote `origin` pointe vers GitHub)

---

## Agent

Exécution directe sans délégation — utilise `gh` pour interagir avec l'API GitHub Issues et Milestones.
