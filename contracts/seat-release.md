# Contrat Libération de Place VJoueur (#134)

> **Feature** : #134 — élargir « Réinscription » à un joueur actuellement connecté
> **Milestone** : `v5.10.x - Stabilité VJoueur`
> **Plan** : `_work/reports/planner-20260804-115318.md`
> **Maquette** : `_work/mockups/134-seat-release.md`
> **Dernière mise à jour** : 2026-08-04

Ce contrat fixe le comportement de `RELEASE_BUMPER_NAME` lorsque le joueur ciblé est **connecté**.
Le cas « joueur déconnecté » (#122) est **inchangé** et reste la référence : voir §3.

---

## 1. Le modèle de données sépare déjà identité de session et score

C'est le point central de cette feature, et il ne nécessite **aucun concept nouveau** :

| Notion | Où elle vit | Sort lors d'une libération |
|---|---|---|
| **Identité de session** | la **clé** `ID` dans `Engine.data.Bumpers`, le `vplayer_id` en `localStorage`, le `PlayerID` du client dans `WebSocketHub` | **invalidée** |
| **Score, équipe, historique** | le **struct** `*Bumper` (`NAME`, `TEAM`, `SCORE`, …) | **intégralement conservé** |

Le mécanisme existe déjà : `reattachVirtualPlayerUnsafe` (`engine.go`) re-clé un bumper sous un
nouvel `ID` en conservant le même struct — son commentaire le dit explicitement, *« the seat is
rendered, not recreated, which is precisely what preserves the score/team/history »*.

**Libérer une place = changer la clé, garder le struct.** Il n'y a pas de « slot libéré mais score
conservé » à inventer : c'est la définition même du re-clé.

---

## 2. `RELEASE_BUMPER_NAME` sur un joueur CONNECTÉ — séquence contractuelle

**Action** : `RELEASE_BUMPER_NAME { ID }` — **inchangée**, aucune nouvelle action.
Le serveur choisit son comportement selon l'état de connexion du bumper ciblé.

L'ordre des étapes est **normatif** — chacune dépend de la précédente :

| # | Étape | Pourquoi cet ordre |
|---|---|---|
| 1 | `PLAYER_EVICTED { REASON: "SEAT_RELEASED" }` **ciblé** via `SendToPlayerID(ancienID, …)` | `SendToPlayerID` résout par le `PlayerID` du client, qui vaut encore l'**ancien** ID. Après re-clé, le joueur ne serait plus adressable |
| 2 | `evictionRegistry.Record(ancienID, "SEAT_RELEASED")` | permet à un `PLAYER_CONNECT` portant l'ID périmé (notification perdue) d'obtenir le vrai motif au lieu d'un `ENROLLMENT_CLOSED` générique |
| 3 | Re-clé du bumper : `ancienID` → nouvel ID, **même struct**, `Connected = false` | invalide la session ; conserve score/équipe/historique |
| 4 | `reclaimAuthorizedUntil` posé sur le bumper re-clé (TTL existant) | ouvre la reprise du siège par le chemin #122, inchangé |
| 5 | `broadcastUpdate()` | l'admin voit la carte passer déconnectée |

### Pourquoi le re-clé est obligatoire, et pas seulement une commodité

Sans lui, le joueur évincé qui se reconnecte avec son ancien `ID` emprunte le **cas 1** de
`ReconnectOrCreateVirtualPlayer`, qui le reconnecte **et remet `reclaimAuthorizedUntil` à zéro** —
annulant silencieusement la libération que l'animateur vient de demander. Le re-clé ferme cette
course en rendant l'ancien ID irrésoluble.

### Ce que le serveur ne fait PAS

**La connexion WebSocket n'est pas fermée de force.** Deux raisons :

1. `PLAYER_EVICTED` est déposé sur le canal `Send` du client ; le fermer immédiatement risquerait de
   perdre la notification avant son écriture par `writePump` — précisément l'échec à éviter.
2. C'est déjà le contrat de `DELETE_BUMPER` (`handleDeleteBumper` ne ferme jamais la connexion) : le
   client se retire lui-même après traitement. Diverger ici créerait deux comportements pour un même
   type d'événement.

L'invalidation réelle de la session est le **re-clé** (étape 3), pas la fermeture du socket.

---

## 3. `RELEASE_BUMPER_NAME` sur un joueur DÉCONNECTÉ — strictement inchangé (#122)

Aucune éviction, aucun re-clé, aucun `PLAYER_EVICTED` : uniquement `reclaimAuthorizedUntil` +
`ReclaimRequested = false`, exactement comme aujourd'hui. **Non-régression verrouillée** — c'est le
cas d'usage nominal de #122 (« récupération de nom assistée »).

---

## 4. Nouveau motif `SEAT_RELEASED`

| | |
|---|---|
| **Valeur** | `"SEAT_RELEASED"` |
| **Porté par** | `PLAYER_EVICTED { REASON }` (serveur → VJoueur ciblé) et `PLAYER_REJECTED { REASON }` (réponse à un `PLAYER_CONNECT` portant l'ID périmé) |
| **Famille** | motif de **renvoi** (`REDIRECT_MESSAGES`), comme `PLAYER_REMOVED` / `GAME_RESET` / `SESSION_EXPIRED` — **pas** un motif de rejet à la soumission (`REJECTION_MESSAGES`) |

**Distinction avec `PLAYER_REMOVED`** — les deux libèrent la place, mais :

| | `PLAYER_REMOVED` (`DELETE_BUMPER`) | `SEAT_RELEASED` (`RELEASE_BUMPER_NAME`) |
|---|---|---|
| Le bumper | supprimé de l'état | **conservé** |
| Score / équipe | perdus | **conservés** |
| Reprise du siège | nouvelle inscription à zéro | reprise du siège **avec son score** |

Le message affiché doit refléter cette différence : le joueur doit comprendre qu'il **retrouvera son
score** en se réinscrivant.

---

## 5. Reprise du siège — aucun chemin nouveau

Elle emprunte le chemin de reprise #122 **tel quel** : `PLAYER_CONNECT` **sans `ID`** + nom
correspondant + autorisation valide → `reattachVirtualPlayerUnsafe` → nouvel ID, score conservé.

Le joueur arrive naturellement sans `ID` : le traitement de `PLAYER_EVICTED` côté client purge
`vplayer_id` du `localStorage`. Les deux branches sont donc couvertes :

| Situation du client | Chemin | Résultat |
|---|---|---|
| A traité `PLAYER_EVICTED` (nominal) | `PLAYER_CONNECT` sans ID → reprise #122 | siège repris, score conservé |
| N'a pas reçu `PLAYER_EVICTED` | `PLAYER_CONNECT` avec ID périmé → registre d'éviction | `PLAYER_REJECTED { SEAT_RELEASED }`, le client purge et retente |

**N'importe quel joueur** peut reprendre le siège, pas seulement l'occupant précédent : c'est
volontaire (spec #134) — le siège porte le score, pas la personne.

---

## 6. Compatibilité

**Aucun changement BREAKING.**

- Aucune action, aucun champ ajouté, renommé, retiré ni retypé. Seule une **nouvelle valeur** de
  `REASON` apparaît, sur des messages existants.
- Un client antérieur recevant `SEAT_RELEASED` retombe sur `DEFAULT_REDIRECT_MESSAGE`
  (`REDIRECT_MESSAGES[reason] || DEFAULT_REDIRECT_MESSAGE`, déjà en place et déjà testé pour un motif
  inconnu) : il est renvoyé à l'inscription avec un texte générique mais correct, et peut se
  réinscrire normalement.
- Le comportement sur un joueur déconnecté est inchangé (§3).

> **Réserve connue, client antérieur** : `VPlayerPage.jsx` teste `reason in REDIRECT_MESSAGES` à un
> endroit (traitement d'un `PLAYER_REJECTED`). Un client antérieur prendra la branche « motif non
> reconnu » pour `SEAT_RELEASED` — dégradé, jamais bloquant, et sans objet dès que le client est à
> jour. Documenté ici pour ne pas être redécouvert comme un défaut.
