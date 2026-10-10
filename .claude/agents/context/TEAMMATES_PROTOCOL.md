# Adaptations projet — TEAMMATES_PROTOCOL

> Compagnon de `TEAMMATES_PROTOCOL.template.md`. Prime sur le template.

## Adresse du teamleader

Dans cette installation, l'adresse `SendMessage` du teamleader est **`team-lead`**, pas `main`
(`main` est rejeté : « addresses the main conversation itself »). Partout où le template écrit
`SendMessage({ to: "main", ... })`, utiliser `SendMessage({ to: "team-lead", ... })`.
