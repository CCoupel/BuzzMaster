# Commande /marketing - Communication de Release

Lance le sous-agent MARKETING pour créer les contenus de communication d'une nouvelle version.

## Argument reçu (optionnel)

$ARGUMENTS

## Mot-clé help

`/marketing help` → Affiche :

```
## /marketing - Aide

**Description** : Créer les contenus de communication d'une release

**Usage** :
  /marketing help                          Afficher cette aide
  /marketing                               Auto-détecte la version
  /marketing 2.40.0                        Version spécifique
  /marketing 2.40.0 PROD                   Version + environnement
  /marketing "Mode Memory multi-équipes"   Focus sur une feature

**Livrables** : Release notes, posts réseaux sociaux, newsletter (major)
```

**Formats possibles** :
- `/marketing` : Auto-détecte la version actuelle
- `/marketing 2.40.0` : Version spécifique
- `/marketing 2.40.0 PROD` : Version + environnement
- `/marketing "Mode Memory multi-équipes"` : Focus sur une feature

## Action immédiate

**Détecter le mode** — Lire `~/.claude/teams/myTEAM/config.json` :
- **Fichier trouvé → Mode TEAM** : transmettre au CDP existant
- **Fichier absent** : spawner le CDP — il orchestrera les agents nécessaires

### Mode TEAM
```
SendMessage(
  type: "message",
  recipient: "cdp",
  content: "Communication marketing demandée: $ARGUMENTS",
  summary: "Marketing: $ARGUMENTS"
)
```

### Sans TEAM
```
subagent_type: "cdp"
description: "Marketing release"
prompt: Communication marketing demandée: $ARGUMENTS
```
