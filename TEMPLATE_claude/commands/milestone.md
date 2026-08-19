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

  /milestone new <version> [date]   Creer un milestone (ex: v1.4 2026-05-01)
  /milestone status                  Voir la progression des milestones actifs
  /milestone close [version]         Cloturer un milestone

Exemples :
  /milestone new v1.4
  /milestone new v1.4 2026-06-01
  /milestone status
  /milestone close v1.4
  /milestone close                   (clot le milestone le plus avance)
```

---

## Mode NEW — Creer un milestone

Declenche si `$ARGUMENTS` commence par `new`.

### Etape 1 — Recuperer owner/repo

```bash
gh repo view --json owner,name --jq '"repos/" + .owner.login + "/" + .name'
```

### Etape 2 — Completer et valider la version cible

> Voir `context/COMMON.md` section 5.7 — le titre du milestone fixe integralement `X.Y.Z`, seule source de verite pour tout le cycle. Le titre GitHub complet est `vX.Y.Z` ou `vX.Y.Z — <nom>` : la version est toujours le prefixe, le nom (optionnel) est purement descriptif et ne participe a aucun calcul.

**a) `X` et `Y` obligatoires** — si `<version>` n'en precise pas un, le demander explicitement a l'utilisateur avant de continuer (jamais de valeur par defaut) :
```
Version incomplete : <version>. X et Y sont obligatoires (ex: v1.4). Quelle version cible ?
```

**b) `Z` optionnel, complete automatiquement** :
```bash
# Releases existantes pour ce X.Y
EXISTING_Z=$(git tag --list "v${X}.${Y}.*" --sort=-v:refname | head -1)   # ex: v1.4.2
```
- Si `<version>` precise deja `Z` → le garder tel quel.
- Sinon, si des tags `vX.Y.*` existent → `Z = Z_max + 1`.
- Sinon (aucune release `X.Y.*`) → `Z = 0`.

`VERSION="vX.Y.Z"` complet — jamais de `Z` implicite. C'est ce prefixe, et lui seul, qui sert a toute comparaison/validation/parsing par la suite (unicite, ordre, matching a la promotion prod, tri dans `/backlog`).

**c) Nom descriptif (optionnel)** :
```
Nom descriptif du milestone (optionnel, ex: "Authentification OAuth2") :
```
- Si un nom est fourni → `TITLE="${VERSION} — <nom>"` (separateur " — ", em dash entoure d'espaces).
- Si vide → `TITLE="${VERSION}"`.

`TITLE` est ce qui est effectivement écrit dans le champ `title` GitHub et utilise pour toutes les commandes `gh` qui referencent le milestone par son titre exact (`--milestone "<TITLE>"`). `VERSION` reste la seule valeur utilisee pour les comparaisons de version.

**d) Unicite et ordre** — la comparaison porte sur le **prefixe version** de chaque milestone existant, jamais sur le titre complet (deux milestones ne peuvent pas partager le meme `VERSION`, quel que soit leur nom) :
```bash
# Derniere version prod connue (dernier tag, toutes lignes confondues)
LAST_PROD=$(git tag --list 'v*' --sort=-v:refname | head -1)
# Milestones existants dont le prefixe version correspond (titre = VERSION ou VERSION + " — ...")
gh api repos/{owner}/{repo}/milestones \
  --jq ".[] | select(.title == \"${VERSION}\" or (.title | startswith(\"${VERSION} — \")))"
```
- Si un tag existe deja pour `VERSION`, ou si un milestone dont le prefixe version est `VERSION` existe deja → alerter, ne pas creer :
  ```
  VERSION existe deja (tag ou milestone <titre existant>). Choisir une autre version.
  ```
- Si `VERSION` n'est pas strictement posterieur a `LAST_PROD` → alerter, ne pas creer :
  ```
  VERSION n'est pas posterieur a la derniere version livree (<LAST_PROD>). Choisir une autre version.
  ```

**e) Coherence avec les labels des issues** — reportee a l'etape 5, une fois les issues selectionnees (voir `context/GITHUB.md` section 8.3 pour le mapping labels -> segment).

### Etape 3 — Creer le milestone sur GitHub

```bash
# Sans date
gh api repos/{owner}/{repo}/milestones \
  --method POST \
  -f title="<TITLE>" \
  -f description="Release <VERSION>"

# Avec date (format ISO 8601)
gh api repos/{owner}/{repo}/milestones \
  --method POST \
  -f title="<TITLE>" \
  -f description="Release <VERSION>" \
  -f due_on="<YYYY-MM-DD>T23:59:59Z"
```

Afficher la confirmation :
```
Milestone <TITLE> cree.
URL : https://github.com/{owner}/{repo}/milestone/<numero>
```

### Etape 4 — Lister les issues ouvertes

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

Entrez les numeros des issues a associer au milestone <TITLE>
(separees par des virgules, ex: 42,38,51 — ou "all" pour toutes, "0" pour aucune) :
```

### Etape 5 — Associer les issues selectionnees

Pour chaque issue selectionnee :

```bash
gh issue edit <numero> --milestone "<TITLE>"
```

### Etape 6 — Verifier la coherence labels ↔ version

> Voir `context/GITHUB.md` section 8.3 — mapping labels -> segment de version.

Recuperer les labels de **toutes** les issues actuellement associees au milestone (celles de l'etape 5, jamais en delta) :

```bash
gh issue list --milestone "<TITLE>" --json labels --jq '[.[].labels[].name] | unique'
```

Determiner le segment le plus fort attendu (`breaking` > `feature`/`enhancement` > le reste) et comparer au segment reellement incremente par `VERSION` par rapport a `LAST_PROD` (X different → X, sinon Y different → Y, sinon → Z).

Si incoherence → avertissement **non-bloquant** :
```
⚠️  Attention : issue(s) <label> incluses mais seul <segment> a change dans <VERSION>.
Continuer quand meme ? [O/n]
```
Si non → proposer de corriger la version ou la liste d'issues, sans annuler ce qui est deja cree.

Cette meme verification (sur l'ensemble des issues associees, jamais en delta) se redeclenche a chaque association ulterieure d'une issue au milestone en cours de cycle (`gh issue edit --milestone`).

Afficher le recapitulatif final :

```
Milestone <TITLE> configure.

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
  v1.4.0 — Authentification OAuth2  ████████░░░░░░░░  50%  (3/6 issues)
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

`<version>` designe toujours un prefixe `vX.Y.Z` — le titre reel peut porter un nom apres
" — " (ex: `/milestone close v1.4.0` doit matcher `v1.4.0 — Authentification OAuth2`).
Le resultat de cette etape est `TITLE`, le titre exact du milestone trouve, utilise pour
toutes les commandes `gh` suivantes (`--milestone` exige une correspondance exacte).

```bash
# Avec version specifiee — matching par prefixe
gh api repos/{owner}/{repo}/milestones \
  --jq ".[] | select(.title == \"<version>\" or (.title | startswith(\"<version> — \")))"
# → TITLE = .title du resultat

# Sans version → le plus avance
gh api repos/{owner}/{repo}/milestones \
  --jq 'sort_by(.closed_issues / (.open_issues + .closed_issues + 0.001) | -.) | .[0]'
# → TITLE = .title du resultat
```

### Etape 2 — Rapport pre-cloture

Lister toutes les issues du milestone :

```bash
# Issues fermees
gh issue list --milestone "<TITLE>" --state closed --json number,title

# Issues ouvertes
gh issue list --milestone "<TITLE>" --state open --json number,title,labels
```

Afficher le rapport :

```
Milestone <TITLE> — Rapport avant cloture

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
      → Entrer la prochaine version (ex: v1.5.0, voir context/COMMON.md section 5) — et
        optionnellement son nom si un nouveau milestone doit etre cree
  [B] Fermer toutes les issues et cloturer
      → Les issues seront fermees avec le commentaire "Cloture avec le milestone <TITLE>"
  [C] Cloturer le milestone sans toucher aux issues
      → Les issues restent ouvertes (non liees a un milestone)
  [D] Annuler

Votre choix :
```

**Si [A] — Reporter** :

```bash
# Verifier si le prochain milestone existe deja (matching par prefixe) → NEXT_TITLE
gh api repos/{owner}/{repo}/milestones \
  --jq ".[] | select(.title == \"<next-version>\" or (.title | startswith(\"<next-version> — \")))"

# Si n'existe pas : creer (titre = version seule, pas de nom dans ce flux non-interactif)
gh api repos/{owner}/{repo}/milestones --method POST -f title="<next-version>"
# → NEXT_TITLE = "<next-version>"

# Reporter chaque issue ouverte
gh issue edit <numero> --milestone "<NEXT_TITLE>"
```

Afficher confirmation :
```
Issues reportees vers <NEXT_TITLE> :
  → #51 — Export PDF des rapports
  → #47 — Lenteur page dashboard
```

**Si [B] — Fermer** :

```bash
# Fermer chaque issue ouverte avec un commentaire
gh issue close <numero> --comment "Cloture avec le milestone <TITLE>"
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
Milestone <TITLE> cloture.

  Bilan :
    ✅ Issues terminees  : <N>
    ↩️  Issues reportees : <N> → <NEXT_TITLE>   (ou)
    🔒 Issues fermees    : <N>                   (ou)
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
