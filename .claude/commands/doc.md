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

## Action immédiate

**Détecter le mode** — Lire `~/.claude/teams/TEAM-Buzz/config.json` :
- **Fichier trouvé → Mode TEAM** : transmettre au CDP existant
- **Fichier absent** : spawner le CDP — il orchestrera les agents nécessaires

### Mode TEAM
```
SendMessage(
  type: "message",
  recipient: "cdp",
  content: "Mise à jour documentation demandée: $ARGUMENTS",
  summary: "Doc: $ARGUMENTS"
)
```

### Sans TEAM
```
subagent_type: "cdp"
description: "Documentation"
prompt: Mise à jour documentation demandée: $ARGUMENTS
```
