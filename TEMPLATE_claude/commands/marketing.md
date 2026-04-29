# Commande /marketing

Mettre a jour le site marketing Github Pages (branche `gh-pages`) apres une release.

## Usage

```
/marketing [version]
```

## Argument recu

$ARGUMENTS

## Mots-cles de controle

**Reference :** Voir `context/COMMON.md` section 12

| Mot-cle | Action |
|---------|--------|
| `help` | Affiche l'aide et les mots-cles disponibles |
| `status` | Affiche l'etat du workflow en cours |
| `plan` | Affiche le plan sans executer |
| `skip <section>` | Saute une section du site |

Si `$ARGUMENTS` commence par un mot-cle -> executer l'action correspondante.
Sinon -> workflow normal.

## Exemples

```
/marketing               # Met a jour le site avec la derniere release
/marketing v1.3.0        # Force la version a documenter
/marketing plan          # Affiche ce qui sera mis a jour sans modifier
```

## Role

Generer ou mettre a jour le site statique publie sur la branche `gh-pages`.
Le site est **bilingue (FR/EN)** avec un commutateur de langue.
La version affichee est recuperee dynamiquement depuis les **tags Git** du repo.

## Recuperation de la version

Si aucune version n'est passee en argument, recuperer le dernier tag semantique :

```bash
git tag --sort=-version:refname | head -1
```

Si le repo est GitHub, on peut aussi utiliser l'API :

```bash
gh api repos/{ORG}/{PROJECT}/tags --jq '.[0].name'
```

La version recuperee est utilisee dans toutes les sections du site (Hero, Features, Roadmap).

## Prerequis

**Reference** : Voir `context/GITHUB.md` sections 1 (auth), 3 (milestones), 5 (releases/tags)

- [ ] Un tag Git existe sur le repo (`git tag --list`)
- [ ] `CHANGELOG.md` a jour
- [ ] `README.md` a jour avec le positionnement produit
- [ ] Issues GitHub ouvertes/fermees (pour la Roadmap)
- [ ] Milestone GitHub correspondant a la version (recommande)

## Workflow

```
/marketing [version]
    |
    v
[COLLECTE] --> Lire CHANGELOG, README, releases GitHub, issues GitHub, milestone GitHub
    |
    v
[GENERATION] --> Generer ou mettre a jour les sections du site
    |
    v
[TRADUCTION] --> Produire les fichiers locales/fr.json et locales/en.json
    |
    v
[COMMIT gh-pages] --> Commiter et pousser sur la branche gh-pages
    |
    v
[RAPPORT] --> Confirmer les sections mises a jour
```

## Collecte du milestone

Dans la phase COLLECTE, recuperer les donnees du milestone correspondant a la version :

```bash
# Milestone cloture correspondant a la version (source principale)
gh api repos/{owner}/{repo}/milestones \
  --jq '.[] | select(.title=="<version>")'

# Issues fermees dans ce milestone (= ce qui a ete livre)
gh issue list --milestone "<version>" --state closed \
  --json number,title,labels \
  --jq '.[] | [.number, .title, (.labels | map(.name) | join(", "))] | @tsv'

# Issues ouvertes dans le prochain milestone (= roadmap "a venir")
gh api repos/{owner}/{repo}/milestones \
  --jq '[.[] | select(.state=="open")] | sort_by(.due_on) | .[0]' | \
  xargs -I{} gh issue list --milestone "{}" --state open \
  --json number,title,labels
```

Si aucun milestone n'existe pour la version → fallback sur CHANGELOG.md et issues GitHub classiques.

## Structure du site genere

```
gh-pages/
├── index.html              # Page principale (FR par defaut)
├── assets/
│   ├── style.css           # Styles communs
│   └── lang.js             # Gestion commutateur FR/EN
└── locales/
    ├── fr.json             # Textes FR
    └── en.json             # Textes EN
```

## Sections obligatoires

### Hero / Accroche (`id="hero"`)

- Nom du projet et baseline en FR et EN
- **Version courante recuperee dynamiquement** via `git tag --sort=-version:refname | head -1`
- Bouton "Voir les releases" pointant vers `https://github.com/{ORG}/{PROJECT}/releases`
- Call-to-action principal (ex : "Installer", "Voir la doc")

### Problematiques (`id="problems"`)

Decrire les problemes concrets que le projet resout :
- Contexte et situation actuelle (avant le projet)
- Pain points identifies, illustres avec icones
- Public cible concerne

Source : extraire depuis `README.md` section probleme/contexte et `CLAUDE.md` si disponible.

### Solutions (`id="solutions"`)

Presenter les reponses apportees :
- Correspondance probleme → solution (format avant/apres)
- Benefices mesurables (gain de temps, fiabilite, securite...)
- Fonctionnalites cles de la version courante (depuis `CHANGELOG.md`)

### Fonctionnalites (`id="features"`)

Liste des fonctionnalites principales :
- Icone + titre + description courte pour chaque fonctionnalite
- Badge "Nouveau" sur les fonctionnalites de la derniere version
- Distinguer fonctionnalites stables et experimentales si applicable

### Installation (`id="install"`)

Couvrir les 3 scenarios :

**Depuis les sources**
```bash
git clone https://github.com/{ORG}/{PROJECT}.git
cd {PROJECT}
<commande d'installation specifique au projet>
<commande de demarrage>
```

**Depuis les releases binaires**

| Plateforme | Fichier | Commande |
|------------|---------|----------|
| Windows | `.exe` | Double-cliquer sur l'installeur |
| Linux (Debian) | `.deb` | `sudo dpkg -i {project}_X.Y.Z.deb` |
| macOS | `.dmg` | Ouvrir et suivre l'installeur |

**Configuration**
- Variables d'environnement ou fichier de config principal
- Ports par defaut
- Permissions systeme si applicable

### Roadmap (`id="roadmap"`)

Generer automatiquement en privilegiant les milestones GitHub (source la plus precise) :

**Source prioritaire — milestones** :
- **Milestone cloture** correspondant a la version → colonne "Livre" (issues fermees du milestone)
- **Milestone(s) ouvert(s)** → colonnes "En cours" et "A venir" selon les labels des issues

**Fallback sans milestone** :
- Issues fermees recentes → colonne "Livre"
- Issues ouvertes avec label `roadmap` ou `enhancement` → colonne "A venir"
- Issues ouvertes avec label `in progress` → colonne "En cours"

Format :
```
[ Livre ✓ ]                    [ En cours ⚙ ]           [ A venir ○ ]
- #42 Auth OAuth2 (v1.2.0)    - #51 Export PDF (#v1.3)  - #60 API v2 (#v2.0)
- #38 Fix crash iOS (v1.2.0)  - #47 Perf dashboard      - #61 Mode offline
```

Si aucune issue ni milestone disponible : afficher les elements du `CHANGELOG.md` comme historique.

## Commutateur de langue

Integrer dans le `<header>` un toggle visible sur toutes les sections :

```html
<div class="lang-switcher">
  <button class="lang-btn active" data-lang="fr">FR</button>
  <span>|</span>
  <button class="lang-btn" data-lang="en">EN</button>
</div>
```

Tous les textes du site portent l'attribut `data-i18n="cle"`.
Le fichier `lang.js` charge `locales/fr.json` ou `locales/en.json` et remplace
dynamiquement les valeurs au changement de langue. La preference est sauvegardee
en `localStorage`.

## Mise a jour du site existant

Si le site existe deja sur `gh-pages` :
1. Mettre a jour la version dans le Hero
2. Ajouter les nouvelles fonctionnalites dans la section Features (badge "Nouveau")
3. Mettre a jour la section Solutions avec les apports de la version
4. Regenerer la Roadmap depuis les issues courantes
5. Verifier que les commandes d'installation sont toujours valides

## Rapport final

```
Site marketing mis a jour — vX.Y.Z

Sections mises a jour :
- [x] Hero (version dynamique : vX.Y.Z)
- [x] Problematiques
- [x] Solutions (N nouvelles fonctionnalites)
- [x] Fonctionnalites (N badges "Nouveau")
- [x] Installation
- [x] Roadmap (N issues ouvertes, N livrees)

Traductions :
- [x] locales/fr.json
- [x] locales/en.json

Commit pousse sur gh-pages.
URL : https://{ORG}.github.io/{PROJECT}/

Voulez-vous :
a) Valider — le site est en ligne
b) Modifier une section specifique
c) Regenerer la roadmap uniquement
```

## Agent

Lance l'agent `marketing-release` defini dans `.claude/agents/marketing-release.md`
