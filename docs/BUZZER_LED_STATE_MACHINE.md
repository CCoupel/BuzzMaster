# Machine à état LED BuzzClick (v3.4.0)

Ce document décrit le comportement des LED des buzzers physiques BuzzClick.

## Principe : contrôle serveur-driven

Depuis v3.4.0, **le serveur est l'unique source de vérité pour l'état LED** pendant le jeu. Le firmware ne calcule plus l'état LED — il applique uniquement l'action `LED_SET` reçue du serveur.

Le firmware conserve deux animations autonomes (indépendantes du jeu) :
- **Boot sequence** (phases 1-6) : avant la connexion au serveur
- **Grey rotation** : buzzer sans équipe assignée (tourne 1 LED grise sur 3)

Toutes les autres transitions LED sont déclenchées par un `LED_SET` envoyé par le serveur.

## Règle d'acceptation des buzz

Le serveur n'accepte un appui buzzer (`BUTTON`) **que si la phase est `STARTED`** (timer en cours).

En `READY`, `COUNTDOWN`, `PAUSED`, `REVEALED`, `STOPPED` : le message `BUTTON` est ignoré par le serveur. Le firmware ne filtre pas.

## Les 4 états de buzz

| État | Signification |
|------|---------------|
| `NONE` | Personne n'a buzzé |
| `MOI` | Ce buzzer est le premier buzz de son équipe |
| `EQUIPE` | Un coéquipier a buzzé en premier (pas ce buzzer) |
| `AUTRE` | Au moins une autre équipe a buzzé, mais pas la mienne |

**Règles** :
- Un seul `MOI` par équipe maximum (le premier buzz de l'équipe).
- Les états sont **cumulatifs** : si équipe A puis équipe B buzzent, le `MOI` de A est conservé, le premier buzzeur de B devient `MOI`, les équipes C/D... restent `AUTRE`.
- Un nouveau buzz d'une équipe B ne réinitialise pas les états `MOI`/`EQUIPE` de l'équipe A.

## États communs (tous types de jeu)

| Situation | LED |
|-----------|-----|
| Pas d'équipe assignée | Gris rotation (1 LED sur 3 tourne, firmware-driven) |
| STOPPED | SOLID 100% couleur équipe |
| PREPARE | SOLID 100% couleur équipe |

## NORMAL

| Phase | Buzz | LED |
|-------|------|-----|
| READY | NONE | SOLID 100% équipe |
| COUNTDOWN | NONE | SOLID 100% équipe |
| STARTED | NONE | DIM 25% équipe |
| STARTED | MOI | BLINK 100% équipe |
| STARTED | EQUIPE | SOLID 100% équipe |
| STARTED | AUTRE | DIM 25% équipe |
| PAUSED | MOI | BLINK 100% équipe |
| PAUSED | EQUIPE | SOLID 100% équipe |
| PAUSED | AUTRE | DIM 25% équipe |
| REVEALED | MOI | BLINK 100% équipe |
| REVEALED | EQUIPE | SOLID 100% équipe |
| REVEALED | AUTRE / NONE | DIM 25% équipe |

## QCM

La couleur réponse (rouge/vert/jaune/bleu) est assignée par le serveur à chaque buzzer.

**L'équipe fait un** : chaque buzzer représente une réponse. MOI et EQUIPE ont toujours le même affichage — dès qu'un membre de l'équipe buzz, toute l'équipe bascule ensemble.

| Phase | Buzz | LED |
|-------|------|-----|
| READY / COUNTDOWN | NONE | SOLID 100% réponse |
| STARTED | NONE | SOLID 100% réponse |
| STARTED | MOI / EQUIPE *(mon équipe a buzzé)* | SOLID 100% équipe *(cache la réponse)* |
| STARTED | AUTRE | SOLID 100% réponse |
| PAUSED | MOI / EQUIPE | SOLID 100% équipe *(cache la réponse)* |
| PAUSED | AUTRE | SOLID 100% réponse |
| REVEALED | MOI/EQUIPE — bonne rép. + 1er buzz | BLINK 100% réponse |
| REVEALED | MOI/EQUIPE — bonne rép. + pas 1er | SOLID 100% réponse |
| REVEALED | AUTRE / NONE — ou mauvaise rép. | DIM 25% réponse |

## MEMORY

Le comportement varie selon le mode Memory.

**Mode SOLO** : une seule équipe joue à la fois.

| Phase | Équipe | LED |
|-------|--------|-----|
| STOPPED / PREPARE / READY / REVEALED | — | SOLID 100% équipe |
| STARTED | Active | SOLID 100% équipe |
| STARTED | Inactive | DIM 25% équipe |
| PAUSED | Toutes | DIM 25% équipe |

**Autres modes (ex: CHACUN_SON_TOUR)** : ordre de passage visible, équipes non sélectionnées éteintes.

| Phase | Équipe | LED |
|-------|--------|-----|
| STOPPED / PREPARE / READY / REVEALED | — | SOLID 100% équipe |
| STARTED | Active | SOLID 100% équipe |
| STARTED | Prochaine | SOLID 50% équipe |
| STARTED | Autres participantes | DIM 25% équipe |
| STARTED | Non sélectionnées | OFF (0%) |
| PAUSED | Toutes | DIM 25% équipe |

## Actions WebSocket serveur → buzzer

| Action | Déclencheur | Envoi |
|--------|-------------|-------|
| `LED_SET` | Tout changement d'état LED | Par buzzer ou broadcast |

Payload `LED_SET` :
```json
{
  "COLOR": [255, 0, 0],
  "INTENSITY": 255,
  "EFFECT": "SOLID"
}
```

`EFFECT` : `"SOLID"` (steady) | `"BLINK"` (100%↔25% à 400ms) | `"DIM"` (steady atténué, alias sémantique)

## Actions supprimées (pre-v3.4.0)

Les actions suivantes n'existent plus et ne doivent pas être utilisées :

| Action supprimée | Remplacée par |
|------------------|---------------|
| `QCM_COLOR` | `LED_SET` SOLID 100% couleur réponse |
| `QCM_DIM` | `LED_SET` DIM 25% couleur réponse |
| `QCM_REVEAL` | `LED_SET` BLINK ou SOLID selon résultat |
| `QCM_RESET` | `LED_SET` SOLID 100% couleur équipe |

## Reconnexion

À la réception d'un `HELLO` pendant une partie active, le serveur renvoie immédiatement le dernier état LED connu pour ce buzzer (`resendLEDOnReconnect`). Si aucun état n'est connu, le serveur calcule l'état depuis la phase courante et le type de jeu.
