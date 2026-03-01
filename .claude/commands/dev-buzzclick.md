---
name: dev-buzzclick
description: Lancer le developpement firmware BuzzClick (ESP32-C3)
---

# Commande /dev-buzzclick

Cette commande lance l'agent `dev-buzzclick` pour implementer du code firmware ESP32.

## Usage

```
/dev-buzzclick <description de la tache>
```

## Mot-clé help

`/dev-buzzclick help` → Affiche :

```
## /dev-buzzclick - Aide

**Description** : Développement firmware BuzzClick (ESP32-C3)

**Usage** :
  /dev-buzzclick help                              Afficher cette aide
  /dev-buzzclick <description>                     Implémenter une tâche
  /dev-buzzclick Ajouter support OTA               Support mise à jour sans fil
  /dev-buzzclick Corriger reconnexion WiFi         Avec backoff exponentiel
  /dev-buzzclick Ajouter animation LED arc-en-ciel Pour phase COUNTDOWN

**Fichiers** : src/BuzzClick/, src/Common/
```

## Exemples

```
/dev-buzzclick Ajouter le support OTA pour mise a jour sans fil
/dev-buzzclick Corriger la reconnexion WiFi avec backoff exponentiel
/dev-buzzclick Ajouter animation LED arc-en-ciel pour phase COUNTDOWN
```

## Références

**Contexte projet :** Voir `context/COMMON.md` section 1
**Workflow DEV :** Voir `context/DEVELOPMENT.md`
- Ordre BuzzClick : section 7
- Standards ESP32 : section 7
- Règles : section 8

## Action immédiate

**Détecter le mode** — Lire `~/.claude/teams/myTEAM/config.json` :
- **Fichier trouvé → Mode TEAM** : envoyer le prompt au teammate `buzzclick-dev`
- **Fichier absent → Mode SOLO** : spawner un sous-agent jetable

### Mode TEAM
```
SendMessage(
  type: "message",
  recipient: "buzzclick-dev",
  content: "Implémente la tâche BuzzClick suivante.\n\n**Contexte projet :** Voir context/COMMON.md section 1\n**Workflow DEV :** Voir context/DEVELOPMENT.md sections 7-8\n\n**Tâche :** $ARGUMENTS",
  summary: "Dev BuzzClick: $ARGUMENTS"
)
```

### Mode SOLO
Lance le sous-agent `dev-buzzclick` avec le Task tool :
```
subagent_type: "dev-buzzclick"
description: "Implémenter firmware BuzzClick"
prompt:
  Implémente la tâche BuzzClick suivante pour BuzzControl.
  Contexte projet : voir context/COMMON.md section 1
  Workflow DEV : voir context/DEVELOPMENT.md sections 7-8
  Tâche : $ARGUMENTS
```

## Quand utiliser

- Modification firmware BuzzClick (src/BuzzClick/)
- Modification code partagé (src/Common/)
- Bugs communication TCP/UDP
- Animations LED
- Gestion boutons/interruptions

## Workflow complet

Pour une feature impliquant BuzzClick + serveur, utiliser `/feature`.
Le CDP orchestrera dev-backend puis dev-buzzclick.
