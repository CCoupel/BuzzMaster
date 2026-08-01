---
name: prod-push-race
description: A dispatched PROD deploy can complete before a user's "stop"/"no" reaches the agent — always verify actual remote state, never assume a stop message arrived in time
metadata:
  type: feedback
---

Un ordre de déploiement PROD envoyé à `deployer` peut s'exécuter (push + tag + CI) en quelques
secondes. Si l'utilisateur envoie une annulation juste après ("non, pas de push en prod!"), le
message d'arrêt peut arriver APRÈS que le push ait déjà eu lieu — la dispatch et le message
suivant de l'utilisateur ne sont pas synchronisés, il n'y a pas de garantie d'ordre.

**Ce qui s'est passé** : un deploy PROD dispatché a poussé sur `origin/main`, taggé, et déclenché
la CI avant qu'un message "STOP" du CDP (envoyé dès réception du refus utilisateur) n'atteigne le
deployer. Le deployer a correctement respecté la consigne (aucune action corrective automatique),
mais le résultat était déjà public.

**Comment appliquer** :
- Après avoir relayé un ordre d'annulation à un agent, **toujours vérifier indépendamment** l'état
  réel (`git log origin/main`, `git tag -l`, état des issues GitHub, statut CI) avant de rapporter
  quoi que ce soit à l'utilisateur — ne jamais supposer que le stop est arrivé à temps.
- Si l'action a déjà eu lieu malgré l'annulation, le signaler immédiatement et factuellement à
  l'utilisateur (chronologie précise, ce qui est public) plutôt que de tenter un rollback
  automatique — laisser l'utilisateur décider (revert, ou laisser en l'état si le contenu était de
  toute façon validé).
- Cette latence est inhérente au dispatch asynchrone multi-agents ; elle ne se résout pas en
  écrivant des messages "STOP" plus vite — la seule protection fiable est de vérifier l'état
  réel après coup, jamais de faire confiance à l'ordre chronologique des messages.
