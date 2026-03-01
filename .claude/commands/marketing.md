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

## Instructions

Utilise le Task tool pour lancer le sous-agent marketing-release avec les paramètres suivants :

```
subagent_type: "marketing-release"
description: "Créer contenus marketing"
prompt: voir ci-dessous
```

### Prompt à transmettre au sous-agent

```
Crée les contenus de communication pour une release BuzzControl.

**Contexte projet :** Voir `context/COMMON.md` section 1
**Versionnement :** Voir `context/COMMON.md` section 5

**Input utilisateur :** $ARGUMENTS

**Livrables :**
- Release notes : releases/vX.Y.Z/
- Posts réseaux sociaux (Twitter, LinkedIn)
- Newsletter (si major)

**Ton par type :**
- Major (x.0.0) : Très enthousiaste
- Minor (x.y.0) : Modéré
- Patch (x.y.z) : Calme, rassurant
```

## Action immédiate

**Détecter le mode** — Lire `~/.claude/teams/myTEAM/config.json` :
- **Fichier trouvé → Mode TEAM** : envoyer le prompt au teammate `marketing-release`
- **Fichier absent → Mode SOLO** : spawner un sous-agent jetable

### Mode TEAM
```
SendMessage(
  type: "message",
  recipient: "marketing-release",
  content: [prompt "Prompt à transmettre au sous-agent" ci-dessus],
  summary: "Marketing: $ARGUMENTS"
)
```

### Mode SOLO
Lance le sous-agent `marketing-release` avec le Task tool.
