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

## Agent lancé

Lance directement l'agent `dev-buzzclick` avec le contexte projet.

## Quand utiliser

- Modification firmware BuzzClick (src/BuzzClick/)
- Modification code partagé (src/Common/)
- Bugs communication TCP/UDP
- Animations LED
- Gestion boutons/interruptions

## Workflow complet

Pour une feature impliquant BuzzClick + serveur, utiliser `/feature`.
Le CDP orchestrera dev-backend puis dev-buzzclick.
