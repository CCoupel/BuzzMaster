# Procédure de Test — Recalibrage ping/pong + transmission des délais (#130)

**Version** : à définir (branche `bugfix/vjoueur-ping-tolerance`)
**Date** : 2026-08-04
**Branche** : bugfix/vjoueur-ping-tolerance
**Testeur** : QA

---

## Contexte du Bug

Avec la configuration précédente (ping serveur toutes les 3 s, `ReadDeadline` 5 s), la marge affichée
de 2 s ne tolérait en réalité **aucune** perte : un seul ping perdu reportait le pong suivant à 6 s,
au-delà du délai, et le serveur fermait la connexion — un événement banal sur un WiFi de salle. Côté
client, le seuil de silence de 9 s (dérivé, codé en dur) faisait qu'un lien réellement mort n'était
détecté qu'après 9 à 10 s.

**Correctif attendu** (`contracts/liveness-timing.md`) :
- Cadence ping/`HEARTBEAT` serveur resserrée à **2 s** (au lieu de 3 s).
- `ReadDeadline` serveur porté à **7 s**, ce qui donne pour la première fois une **tolérance réelle
  de 2 pings perdus** (0 avant).
- Le serveur transmet désormais son seuil de liaison morte au client via le champ
  `DEAD_LINK_TIMEOUT_MS` du message `HEARTBEAT` (déjà émis depuis #118), au lieu que le client le
  déduise d'un multiplicateur codé en dur.
- Buzzers physiques : **strictement inchangés** (hors périmètre, cadence 3 s / `ReadDeadline` 5 s).

> **⚠️ Valeur ajustée au GATE 2** : le plan initial proposait `DEAD_LINK_TIMEOUT_MS = 5000` (détection
> 5,0–5,5 s). **L'utilisateur a choisi la variante plus réactive `DEAD_LINK_TIMEOUT_MS = 4000`**
> (détection **4,0–4,5 s**, marge réduite à 1 s au-dessus de l'à-coup réseau de 3 s à absorber) —
> arbitrage reconnu dans `contracts/liveness-timing.md` (commit `86fe14d`). **Toutes les valeurs
> temporelles de cette procédure reflètent 4000 ms**, pas les 5000 ms encore visibles dans la
> maquette `_work/mockups/130-timing-recalibration.md` (non mise à jour après l'arbitrage) — se fier
> au contrat et à cette procédure en cas de divergence.

**Objectif affiché du correctif : aucun changement visible en fonctionnement nominal.** Le seul
changement observable doit l'être en condition dégradée — et dans le bon sens (moins de
déconnexions, détection plus rapide d'un lien réellement mort).

Maquette de référence : `_work/mockups/130-timing-recalibration.md` (chronogrammes, machine à états
de la surveillance de liaison, points de contrôle X1→X10 repris ci-dessous).

---

## Prérequis

- [ ] Environnement : QUALIF (ou LOCAL avec build de la branche `bugfix/vjoueur-ping-tolerance`)
- [ ] Au moins 10 appareils/onglets `/player`, un accès admin (`/game`) et un accès TV (`/tv`)
- [ ] Au moins 2 buzzers physiques appairés (scénario 8, X9)
- [ ] Un chronomètre (ou l'horloge du téléphone) pour mesurer les délais de reconnexion
- [ ] Outils navigateur (onglet réseau / console) sur au moins un appareil, pour inspecter le
      contenu des trames `HEARTBEAT` (scénario 7, X8)
- [ ] Un moyen de couper le réseau d'un appareil à la demande (mode avion) — et idéalement un moyen
      de simuler une perte de paquet isolée (limitation réseau navigateur, ou throttling `chrome://`)
- [ ] Accès aux logs serveur
- [ ] Si possible, un serveur QUALIF tournant sur Raspberry Pi (scénario 9, X10) ; à défaut,
      documenter l'environnement réellement utilisé et signaler le point comme non couvert

---

## Scénario 1 — Partie nominale, aucun changement visible (X1)

**Objectif** : Confirmer l'objectif affiché du correctif — rien ne doit changer en fonctionnement
normal.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Inscrire 10 VJoueurs, jouer une partie normale (plusieurs questions, ~20 minutes) sur un réseau stable | | | |
| 2 | Observer les 10 appareils, l'admin et la TV pendant toute la durée | **Aucune** reconnexion spontanée, aucun badge de connexion parasite, comportement visuellement identique à avant le correctif | | |
| 3 | Vérifier les logs serveur en fin de partie | Aucune fermeture de connexion inattendue, aucune erreur | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Tolérance aux à-coups réseau, LE défaut principal corrigé (X2/X4)

**Objectif** : Vérifier que le vrai problème de ce lot est résolu — un incident réseau bref ou une
perte de paquet isolée ne doivent plus faire tomber la connexion. **C'est le scénario le plus
important de cette procédure.**

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Un VJoueur en jeu, réseau stable | | | |
| 2 | Couper son réseau (mode avion) pendant **3 secondes exactement**, puis le rétablir | **Aucune** reconnexion déclenchée — le joueur ne voit rien, son écran reste stable (X2) | | |
| 3 | Répéter 3 fois, avec un timing légèrement différent à chaque fois | Comportement identique à chaque fois | | |
| 4 | Si un moyen de simulation est disponible (limitation/blocage réseau navigateur), simuler la perte d'**un seul** ping isolé (~2 s de coupure ciblée sur un cycle) | Connexion **maintenue** — c'était le défaut principal avant #130 (un seul ping perdu suffisait à faire tomber la connexion) (X4) | | |
| 5 | À défaut de simulation ciblée, reproduire l'étape 4 en observant le comportement lors d'une coupure réseau de ~2 s (proche d'un cycle de ping) plusieurs fois de suite | Aucune déconnexion sur une coupure ≤ 3 s | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Lien réellement mort : détection accélérée (X3)

**Objectif** : Vérifier que la détection d'un lien mort est bien plus rapide qu'avant (4,0–4,5 s au
lieu de 9–10 s), sans être trop agressive.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Un VJoueur en jeu | | | |
| 2 | Couper franchement son réseau (mode avion, sans le rétablir) et chronométrer | Le bandeau de reconnexion apparaît en environ **4 à 5 secondes** (contre 9-10 s avant le correctif) | | |
| 3 | Rétablir le réseau | Reprise normale de la partie | | |
| 4 | Répéter 2-3 fois, en notant le délai mesuré à chaque fois | Délais cohérents, tous dans la fourchette ~4-5 s (pas de valeur aberrante) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Reconnexion après lien mort : fenêtre d'inversion (X5)

**Objectif** : Vérifier l'inversion assumée de l'ordre de détection (le client détecte désormais
**avant** le serveur — ~4,2 s contre 7 s) : le joueur retrouve son identité, et **aucun** badge de
connexion parasite n'apparaît sur les AUTRES joueurs pendant cette fenêtre. C'est le point de
vigilance principal de ce lot (R1 du plan).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Au moins 4 VJoueurs en jeu, dont un qui va couper son réseau | | | |
| 2 | Couper franchement le réseau d'un des joueurs (mode avion), laisser passer la reconnexion (bandeau puis reprise, ~4-5 s) | Le joueur retrouve son nom, son équipe, son score — identiques à avant la coupure | | |
| 3 | Pendant toute la fenêtre entre la coupure et ~7-8 secondes après, observer les badges de connexion des AUTRES joueurs (côté admin) | **Aucun** badge orange/rouge parasite sur les autres joueurs — seul le joueur réellement coupé montre un badge, et seulement pendant sa propre coupure | | |
| 4 | Répéter avec 2 joueurs coupés simultanément | Même résultat — aucune interférence croisée entre les badges | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Admin et TV non affectés (X6)

**Objectif** : Vérifier que la coupure réseau d'un VJoueur n'a aucun effet sur les connexions Admin/TV.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir `/game` (admin) et `/tv` (TV), au moins 3 VJoueurs en jeu | | | |
| 2 | Couper le réseau d'un VJoueur (mode avion) | L'admin et la TV restent stables — pas de reconnexion, pas de gel, pas de badge inattendu sur leur propre connexion | | |
| 3 | Rétablir le réseau du VJoueur | Admin et TV toujours stables | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 6 — Compatibilité ascendante : ancien serveur (X7)

**Objectif** : Vérifier qu'un client à jour reste fonctionnel face à un serveur qui n'envoie pas
encore `DEAD_LINK_TIMEOUT_MS` — repli sur 9000 ms, comportement #118 à l'identique.

> **Si un serveur antérieur n'est pas disponible pour ce test**, cocher N/A et le signaler — ce
> scénario peut être couvert par les tests automatisés (compatibilité CA8) à défaut d'un
> environnement dédié.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Connecter un client `/player` à jour à un serveur d'une version antérieure à #130 (sans `DEAD_LINK_TIMEOUT_MS`) | La partie fonctionne normalement | | |
| 2 | Inspecter la console/outils réseau du client | Le seuil de liaison morte appliqué est bien 9000 ms (repli `INTERVAL_MS × 3` sur l'ancien `INTERVAL_MS=3000`, ou 9000 ms par défaut) | | |
| 3 | Simuler une coupure réseau franche | Détection après ~9-10 s (comportement #118, pas le nouveau 4-5 s) — cohérent avec un serveur qui n'a pas le correctif | | |

**Verdict** : [ ] PASS  [ ] FAIL  [ ] N/A (pas de serveur antérieur disponible)

---

## Scénario 7 — Inspection du contenu `HEARTBEAT` (X8)

**Objectif** : Vérifier directement le contenu des trames envoyées par le serveur.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir l'onglet réseau (WebSocket) d'un client `/player`, `/game` ou `/tv` | | | |
| 2 | Observer les trames `HEARTBEAT` reçues sur ~10-15 secondes | Une trame `HEARTBEAT` toutes les **2 secondes** (pas 3) | | |
| 3 | Inspecter le contenu de chaque trame | Contient bien `INTERVAL_MS: 2000` **et** `DEAD_LINK_TIMEOUT_MS: 4000` | | |
| 4 | Répéter sur les 3 types de client (`/player`, `/game`, `/tv`) | Même contenu sur les trois | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 8 — Non-régression buzzers physiques (X9)

**Objectif** : Vérifier que les buzzers physiques ne sont affectés par aucun aspect de ce correctif
(hors périmètre explicite).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Au moins 2 buzzers physiques appairés, en jeu pendant toute la durée du scénario 1 (partie nominale ~20 min) | | | |
| 2 | Observer leur comportement (LED, réactivité, cadence) | Identique à avant le correctif — aucune nouvelle déconnexion, aucun ralentissement | | |
| 3 | Vérifier (si accès technique) qu'aucun `HEARTBEAT` n'est envoyé aux buzzers | Confirmé — les buzzers n'ont jamais reçu ce message, avant comme après #130 | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 9 — Charge serveur sur la durée, idéalement sur Raspberry Pi (X10)

**Objectif** : Vérifier que le resserrement de la cadence de ping (+50 % de ticks) n'a pas d'impact
notable sur la charge serveur, en particulier sur le matériel cible (Raspberry Pi).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer une partie de ~20 minutes avec 10-12 clients connectés (VJoueurs + admin + TV), sur Raspberry Pi si possible | | | |
| 2 | Observer la charge CPU/mémoire du serveur pendant toute la durée (`top`, `htop`, ou équivalent) | Pas de hausse notable par rapport à une partie équivalente avant #130 | | |
| 3 | Vérifier la réactivité générale de l'interface admin pendant la partie | Aucun ralentissement perceptible | | |

**Verdict** : [ ] PASS  [ ] FAIL  [ ] Non couvert (préciser l'environnement réellement utilisé si pas de Raspberry Pi disponible)

---

## Critères de Validation

- [ ] Aucun changement visible en fonctionnement nominal (scénario 1, X1)
- [ ] Une coupure réseau ≤ 3 s ou la perte d'un ping isolé ne déclenche **jamais** de reconnexion —
      c'était le défaut principal du lot avant #130 (scénario 2, X2/X4)
- [ ] Un lien réellement mort est détecté en ~4-5 s, contre 9-10 s avant le correctif (scénario 3, X3)
- [ ] Le joueur qui se reconnecte après un lien mort retrouve son identité, et aucun badge parasite
      n'apparaît sur les autres joueurs pendant la fenêtre d'inversion (scénario 4, X5)
- [ ] Admin et TV ne sont jamais affectés par la coupure d'un VJoueur (scénario 5, X6)
- [ ] Compatibilité ascendante avec un serveur antérieur (repli 9000 ms) (scénario 6, X7)
- [ ] Le `HEARTBEAT` contient `INTERVAL_MS: 2000` et `DEAD_LINK_TIMEOUT_MS: 4000`, toutes les 2 s
      (scénario 7, X8)
- [ ] Aucune régression sur les buzzers physiques : jamais de `HEARTBEAT`, cadence 3 s/5 s inchangée
      (scénario 8, X9)
- [ ] Aucune hausse notable de charge serveur, idéalement vérifiée sur Raspberry Pi (scénario 9, X10)
- [ ] Aucune erreur/`panic` dans les logs serveur pendant toute la procédure
- [ ] Suite automatisée complète (Go + React) au vert avant validation finale (CA12), en particulier
      `internal/server/heartbeat_test.go` et `web/src/pages/VPlayerPage.reconnect.test.jsx`

## Notes QA

[Espace pour observations]

> Note pour QA : cette procédure utilise `DEAD_LINK_TIMEOUT_MS = 4000` (détection 4,0-4,5 s),
> la valeur retenue à l'arbitrage GATE 2 — **pas** les 5000 ms (5,0-5,5 s) encore visibles dans
> `_work/mockups/130-timing-recalibration.md`, qui n'a pas été mise à jour après l'arbitrage. En cas
> de divergence entre la maquette et cette procédure, se fier à `contracts/liveness-timing.md` et à
> cette procédure.
