# Commande /milestone

Gerer les milestones GitHub du projet : creation avec association d'issues, suivi de progression, cloture avec gestion des issues non terminees.

## Usage

```
/milestone new <version> [YYYY-MM-DD]   # Creer un milestone et associer des issues
/milestone status                        # Progression du ou des milestones actifs
/milestone close [version]               # Cloturer un milestone
```

## Argument recu

$ARGUMENTS

---

Si `$ARGUMENTS` est vide → afficher l'aide ci-dessous.
Sinon → detecter le mode selon le premier mot de `$ARGUMENTS`.

## Aide (aucun argument)

```
Commandes disponibles :

  /milestone new <version> [date]   Creer un milestone (ex: v1.2.0 2026-05-01)
  /milestone status                  Voir la progression des milestones actifs
  /milestone close [version]         Cloturer un milestone

Exemples :
  /milestone new v1.2.0
  /milestone new v1.2.0 2026-06-01
  /milestone status
  /milestone close v1.2.0
  /milestone close                   (clot le milestone le plus avance)
```

---

## Mode NEW — Creer un milestone

Declenche si `$ARGUMENTS` commence par `new`.

### Etape 1 — Recuperer owner/repo

```bash
gh repo view --json owner,name --jq '"repos/" + .owner.login + "/" + .name'
```

### Etape 2 — Creer le milestone sur GitHub

```bash
# Sans date
gh api repos/{owner}/{repo}/milestones \
  --method POST \
  -f title="<version>" \
  -f description="Release <version>"

# Avec date (format ISO 8601)
gh api repos/{owner}/{repo}/milestones \
  --method POST \
  -f title="<version>" \
  -f description="Release <version>" \
  -f due_on="<YYYY-MM-DD>T23:59:59Z"
```

Afficher la confirmation :
```
Milestone <version> cree.
URL : https://github.com/{owner}/{repo}/milestone/<numero>
```

### Etape 3 — Lister les issues ouvertes

```bash
gh issue list --state open --limit 100 \
  --json number,title,labels,milestone \
  --jq '.[] | select(.milestone == null) | [.number, .title, (.labels | map(.name) | join(", "))] | @tsv'
```

Afficher sous forme de tableau :

```
Issues disponibles (sans milestone) :

  #  | Titre                              | Labels
-----|------------------------------------|---------
  42 | Ajouter auth OAuth2                | feature
  38 | Crash au demarrage iOS             | bug
  35 | Refactor module auth               | refactor
  51 | Export PDF des rapports            | feature
  47 | Lenteur page dashboard             | bug, performance

Entrez les numeros des issues a associer au milestone <version>
(separees par des virgules, ex: 42,38,51 — ou "all" pour toutes, "0" pour aucune) :
```

### Etape 4 — Associer les issues selectionnees

Pour chaque issue selectionnee :

```bash
gh issue edit <numero> --milestone "<version>"
```

Afficher le recapitulatif final :

```
Milestone <version> configure.

Issues associees :
  ✅ #42 — Ajouter auth OAuth2
  ✅ #38 — Crash au demarrage iOS
  ✅ #51 — Export PDF des rapports

  3 issues / 0 % complete (0 fermees)
  Echeance : <date ou "non definie">

Prochaines etapes :
  /backlog #42     Demarrer le travail sur une issue
  /milestone status   Suivre la progression
```

---

## Mode STATUS — Progression des milestones actifs

Declenche si `$ARGUMENTS` est `status` ou vide apres `milestone`.

### Etape 1 — Recuperer les milestones ouverts

```bash
gh api repos/{owner}/{repo}/milestones \
  --jq '.[] | select(.state=="open") | {number, title, open_issues, closed_issues, due_on, description}'
```

### Etape 2 — Afficher la progression

Pour chaque milestone ouvert, calculer le pourcentage et afficher :

```
Milestones actifs :

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  v1.2.0  ████████░░░░░░░░  50%  (3/6 issues)
  Echeance : 2026-06-01  (J-46)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  Issues terminees (3) :
    ✅ #42 — Ajouter auth OAuth2
    ✅ #38 — Crash au demarrage iOS
    ✅ #43 — Refactor module auth

  Issues restantes (3) :
    ❌ #51 — Export PDF des rapports       [feature]
    ❌ #47 — Lenteur page dashboard        [bug, performance]
    ❌ #55 — Tests E2E checkout            [test]

Commandes disponibles :
  /backlog #51         Demarrer une issue
  /milestone close     Cloturer le milestone
```

Si aucun milestone actif :
```
Aucun milestone actif.
Creer un milestone : /milestone new <version>
```

---

## Mode CLOSE — Cloturer un milestone

Declenche si `$ARGUMENTS` commence par `close`.

Si aucune version precisee → utiliser le milestone avec le plus de progression (le plus avance).

### Etape 1 — Identifier le milestone

```bash
# Avec version specifiee
gh api repos/{owner}/{repo}/milestones \
  --jq '.[] | select(.title=="<version>")'

# Sans version → le plus avance
gh api repos/{owner}/{repo}/milestones \
  --jq 'sort_by(.closed_issues / (.open_issues + .closed_issues + 0.001) | -.) | .[0]'
```

### Etape 2 — Rapport pre-cloture

Lister toutes les issues du milestone :

```bash
# Issues fermees
gh issue list --milestone "<version>" --state closed --json number,title

# Issues ouvertes
gh issue list --milestone "<version>" --state open --json number,title,labels
```

Afficher le rapport :

```
Milestone <version> — Rapport avant cloture

  ✅ #42 — Ajouter auth OAuth2             [closed]
  ✅ #38 — Crash au demarrage iOS          [closed]
  ✅ #43 — Refactor module auth            [closed]
  ❌ #51 — Export PDF des rapports         [open]  [feature]
  ❌ #47 — Lenteur page dashboard          [open]  [bug]

  5 issues au total — 3 terminees (60%) — 2 non terminees
```

### Etape 3 — Gestion des issues non terminees

Si des issues sont encore ouvertes, proposer :

```
2 issues non terminees. Que faire ?

  [A] Reporter vers le prochain milestone
      → Entrer le nom du prochain milestone (ex: v1.3.0)
  [B] Fermer toutes les issues et cloturer
      → Les issues seront fermees avec le commentaire "Cloture avec le milestone <version>"
  [C] Cloturer le milestone sans toucher aux issues
      → Les issues restent ouvertes (non liees a un milestone)
  [D] Annuler

Votre choix :
```

**Si [A] — Reporter** :

```bash
# Verifier si le prochain milestone existe, sinon le creer
gh api repos/{owner}/{repo}/milestones --jq '.[] | select(.title=="<next-version>")'

# Si n'existe pas : creer
gh api repos/{owner}/{repo}/milestones --method POST -f title="<next-version>"

# Reporter chaque issue ouverte
gh issue edit <numero> --milestone "<next-version>"
```

Afficher confirmation :
```
Issues reportees vers <next-version> :
  → #51 — Export PDF des rapports
  → #47 — Lenteur page dashboard
```

**Si [B] — Fermer** :

```bash
# Fermer chaque issue ouverte avec un commentaire
gh issue close <numero> --comment "Cloture avec le milestone <version>"
```

**Si [C] — Cloturer sans toucher** :

Continuer directement à l'etape 4.

### Etape 4 — Cloturer le milestone

```bash
gh api repos/{owner}/{repo}/milestones/<numero> \
  --method PATCH \
  -f state="closed"
```

### Etape 5 — Rapport de cloture

```
Milestone <version> cloture.

  Bilan :
    ✅ Issues terminees  : <N>
    ↩️  Issues reportees : <N> → <next-version>   (ou)
    🔒 Issues fermees    : <N>                     (ou)
    📌 Issues en suspens : <N> (toujours ouvertes)

  URL : https://github.com/{owner}/{repo}/milestone/<numero>?closed=1

Prochaines etapes :
  /deploy prod              Si le milestone correspond a une release
  /marketing <version>      Publier les release notes
```

---

## Prerequis

**Reference** : Voir `context/GITHUB.md` sections 1 (auth), 1.2 (owner/repo), 3 (milestones), 2 (issues)

- CLI GitHub (`gh`) installe et authentifie (`gh auth login`)
- Le projet doit etre un repo GitHub (remote `origin` pointe vers GitHub)

## Agent

Execution directe sans delegation — utilise uniquement `gh` pour interagir
avec l'API GitHub Milestones et Issues.
