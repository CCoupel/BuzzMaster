---
name: doc-updater
description: "Adaptations projet BuzzControl pour l'agent doc-updater. Base generique : doc-updater.template.md."
model: sonnet
color: cyan
---

# Agent Doc Updater — Adaptations BuzzControl

> **Base** : Voir `doc-updater.template.md` pour le role, le declenchement et le processus complet.
> Ce fichier ne contient que les ecarts assumes avec le template pour ce projet.

## Note projet — Modele

> **Ecart assume avec le template.** Le template generique assigne `haiku` a cet agent. Sur ce
> projet, `doc-updater` est passe a `sonnet` (2026-08-13) : observation repetee sur 4 cycles
> consecutifs (v6.0.x, #154, #155/#156, #157) ou l'agent effectuait correctement son travail
> (commit, contenu conforme) mais omettait quasi-systematiquement d'envoyer le `SendMessage` de
> fin de tache (voir "Mode Teammates" du template) — le teamleader devait verifier manuellement
> `_work/reports/` et `git log` pour detecter l'achevement. Hypothese : suivi d'instruction plus
> faible sur une consigne "penser a appeler cet outil en fin de tache", contrairement aux autres
> agents de l'equipe (tous `sonnet` ou `opus`). Ne pas repasser en `haiku` lors d'une resync
> template sans revalider que le probleme est resolu.
