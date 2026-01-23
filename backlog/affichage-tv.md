# Affichage TV

**Statut** : ✅ Complété (v2.30.0)

## Description

Synchronisation centralisée des changements d'image de fond sur tous les écrans TV.

## Fonctionnalités implémentées

- [x] **Synchronisation des changements d'image de fond**
  - Le serveur centralise le timing et notifie tous les clients
  - `CurrentBackgroundIndex` dans GameState (backend)
  - Goroutine de cycling dans main.go
  - Action `BACKGROUND_CHANGE` dans le protocole WebSocket
  - Tous les clients TV reçoivent l'index synchronisé
  - Transitions simultanées sur tous les écrans

## Version

v2.30.0
