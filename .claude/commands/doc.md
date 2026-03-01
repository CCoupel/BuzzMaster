# Commande /doc - Mise à Jour de la Documentation

Lance le sous-agent DOC pour mettre à jour la documentation après validation d'une feature.

## Argument reçu (optionnel)

$ARGUMENTS

## Mot-clé help

`/doc help` → Affiche :

```
## /doc - Aide

**Description** : Mettre à jour la documentation après validation

**Usage** :
  /doc help                     Afficher cette aide
  /doc                          Auto-détecte depuis git
  /doc "description"            Feature spécifique
  /doc bugfix "description"     Documenter un bugfix
  /doc breaking "description"   Documenter un breaking change

**Fichiers** : CHANGELOG.md, CLAUDE.md, ADMIN_GUIDE.md, config.json
```

**Formats possibles** :
- `/doc` : Auto-détecte depuis git
- `/doc "description"` : Feature spécifique
- `/doc bugfix "description"` : Bugfix
- `/doc breaking "description"` : Breaking change

## Instructions

Utilise le Task tool pour lancer le sous-agent doc-updater avec les paramètres suivants :

```
subagent_type: "doc-updater"
description: "Mettre à jour documentation"
prompt: voir ci-dessous
```

### Prompt à transmettre au sous-agent

```
Mets à jour la documentation pour BuzzControl.

**Contexte projet :** Voir `context/COMMON.md` section 1
**Versionnement :** Voir `context/COMMON.md` section 5
**Checklists :** Voir `context/COMMON.md` section 7

**Input utilisateur :** $ARGUMENTS

**Fichiers à mettre à jour :**
- CHANGELOG.md (format Keep a Changelog)
- CLAUDE.md (sections impactées)
- docs/ADMIN_GUIDE.md (si applicable)
- backlog/*.md (statut, items, choix)
- config.json (finaliser version)

**Règle versionnement :**
- Feature : z=0 (2.40.3 → 2.40.0)
- Bugfix : garder z
- Breaking : x++
```

## Action immédiate

**Détecter le mode** — Lire `~/.claude/teams/myTEAM/config.json` :
- **Fichier trouvé → Mode TEAM** : envoyer le prompt au teammate `doc-updater`
- **Fichier absent → Mode SOLO** : spawner un sous-agent jetable

### Mode TEAM
```
SendMessage(
  type: "message",
  recipient: "doc-updater",
  content: [prompt "Prompt à transmettre au sous-agent" ci-dessus],
  summary: "Doc: $ARGUMENTS"
)
```

### Mode SOLO
Lance le sous-agent `doc-updater` avec le Task tool.
