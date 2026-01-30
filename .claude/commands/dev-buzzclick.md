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

## Agent lance

Lance directement l'agent `dev-buzzclick` avec le contexte :
- Branche Git actuelle
- Description de la tache
- Contraintes firmware (watchdog, memoire, interruptions)

## Quand utiliser

- Modification du firmware BuzzClick (src/BuzzClick/)
- Modification du code partage (src/Common/)
- Bugs de communication TCP/UDP
- Animations LED
- Gestion boutons/interruptions
- Optimisation memoire ESP32

## Workflow complet

Pour une feature complete impliquant BuzzClick + serveur :

```
/feature "Ajouter synchronisation temps NTP pour BuzzClick"
```

Le CDP orchestrera automatiquement dev-backend (serveur) puis dev-buzzclick (firmware).
