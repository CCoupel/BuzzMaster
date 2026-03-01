# Commande /deploy - Arrêt et Redéploiement QUALIF / PROD

Arrête le serveur en cours puis redéploie vers l'environnement cible.

## Argument reçu

$ARGUMENTS

## Mot-clé help

`/deploy help` → Affiche l'aide ci-dessous :

```
## /deploy - Aide

**Description** : Arrêt et redéploiement vers QUALIF / PROD

**Usage** :
  /deploy help              Afficher cette aide
  /deploy                   Déployer en QUALIF (défaut)
  /deploy QUALIF            Build Windows, tests locaux
  /deploy PREPROD           Build Windows + ARM64, validation finale
  /deploy PROD              Build + squash merge + tag + release GitHub
  /deploy hotfix            Mode urgence pour bugs critiques

**Workflow** : QUALIF → PREPROD → PROD
```

**Format** : `/deploy [QUALIF|PREPROD|PROD|hotfix]`

- **QUALIF** (défaut) : Build Windows, redémarrage local pour tests
- **PREPROD** : Build Windows + ARM64, redémarrage local pour validation finale
- **PROD** : Build Windows + ARM64 + squash merge + tag + release GitHub
- **hotfix** : Mode urgence pour bugs critiques

## Instructions

Utilise le Task tool pour lancer le sous-agent deploy avec les paramètres suivants :

```
subagent_type: "deploy"
description: "Redéploiement [ENV]"
prompt: voir ci-dessous
```

### Prompt à transmettre au sous-agent

```
Arrête et redéploie le serveur BuzzControl.
Environnement cible : $ARGUMENTS (QUALIF par défaut si vide)
```

## Action immédiate

**Détecter le mode** — Lire `~/.claude/teams/myTEAM/config.json` :
- **Fichier trouvé → Mode TEAM** : transmettre au CDP pour dispatch
- **Fichier absent → Mode SOLO** : spawner un sous-agent jetable

### Mode TEAM
```
SendMessage(
  type: "message",
  recipient: "cdp",
  content: "Déploiement demandé: $ARGUMENTS (QUALIF par défaut si vide)",
  summary: "Deploy: $ARGUMENTS"
)
```

### Mode SOLO
Lance le sous-agent `deploy` avec le Task tool.
