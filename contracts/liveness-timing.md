# Contrat Timing de Liaison — serveur ↔ clients web (#130)

> **Bugfix** : #130 — recalibrage ping/pong + transmission des délais serveur→client
> **Milestone** : `v5.10.x - Stabilité VJoueur`
> **Plan** : `_work/reports/planner-20260803-214210.md`
> **Maquette** : `_work/mockups/130-timing-recalibration.md`
> **Dernière mise à jour** : 2026-08-04 (ajustement GATE 2 — voir encadré ci-dessous)

Ce contrat fixe les paramètres de détection de liaison morte entre le serveur et les clients web
(`/ws/admin`, `/ws/tv`, `/ws/player`). Il complète l'entrée `[20260729]` du changelog (#118), qui a
introduit `HEARTBEAT { INTERVAL_MS }`.

> **Ajustement GATE 2 (2026-08-04)** : le plan initial recommandait `DEAD_LINK_TIMEOUT_MS = 5000`
> (détection 5,0–5,5 s, marge de 2 s au-dessus de l'à-coup réseau de 3 s à absorber), avec une
> variante à `4000` mentionnée en note. **L'utilisateur a choisi la variante réactive à 4000**
> (détection 4,0–4,5 s, marge réduite à 1 s). Toutes les valeurs de ce contrat reflètent ce choix —
> voir `_work/handoff/task-dev-backend-20260804-090721.md`.

**Hors périmètre** : les buzzers physiques (`/ws/buzzer`, `internal/server/websocket_buzzer.go`) et
le canal de logs (`/ws/logs`) — voir §5.

---

## 1. Principe : le serveur est la source de vérité unique

Le client ne code en dur **aucun** seuil de liaison. Il applique les valeurs annoncées par le
serveur, et ne retombe sur des valeurs par défaut que tant qu'il n'a rien reçu.

Le porteur de ces valeurs est le message **`HEARTBEAT`**, déjà émis à chaque tick du `writePump` vers
tous les clients web depuis #118. Il est étendu, pas remplacé.

> **Pourquoi `HEARTBEAT` et non `HELLO` ni un nouveau message de configuration** :
> `HEARTBEAT` est déjà émis vers les trois types de clients web, il porte déjà `INTERVAL_MS`, et le
> client le consomme déjà pour en dériver son seuil. Surtout, il est **répété** : une valeur de
> configuration envoyée une seule fois à la connexion serait définitivement perdue si ce message
> était manqué, alors qu'ici la valeur est réaffirmée à chaque tick. `HELLO` est par ailleurs
> restreint aux clients Admin (`broadcastHello`), ce qui l'exclut d'office.

---

## 2. Message `HEARTBEAT` — payload étendu

**Direction** : serveur → client web. **Aucune réponse attendue.**

```json
{
  "ACTION": "HEARTBEAT",
  "MSG": {
    "INTERVAL_MS": 2000,
    "DEAD_LINK_TIMEOUT_MS": 4000
  }
}
```

| Champ | Type | Depuis | Description |
|---|---|---|---|
| `INTERVAL_MS` | entier (ms) | #118 | Cadence réelle du ticker serveur — **inchangé dans son sens**, sa valeur passe de 3000 à 2000 |
| `DEAD_LINK_TIMEOUT_MS` | entier (ms) | **#130, nouveau** | Silence au-delà duquel le client doit considérer la liaison morte, fermer le socket et se reconnecter |

`DEAD_LINK_TIMEOUT_MS` est une **valeur absolue**, pas un multiplicateur : le serveur maîtrise ainsi
directement le seuil, sans dépendre d'un facteur laissé au client.

---

## 3. Règle d'application côté client

```
seuil = DEAD_LINK_TIMEOUT_MS                si le champ a été reçu
      = INTERVAL_MS × 3                     sinon, si un HEARTBEAT a été reçu   (comportement #118)
      = 3000 × 3 = 9000 ms                  sinon                               (repli initial #118)
```

Le compteur de silence est réarmé par **n'importe quel** message entrant, `HEARTBEAT` compris — la
surveillance est passive, le client n'émet aucun battement périodique (choix d'architecture #118).

---

## 4. Valeurs contractuelles

| Paramètre | Avant (#118) | Après (#130) |
|---|---|---|
| Cadence ping protocolaire + `HEARTBEAT` (`writePumpTickPeriod`) | 3000 ms | **2000 ms** |
| `ReadDeadline` serveur, réarmé par le `PongHandler` | 5000 ms | **7000 ms** |
| Pings intégralement perdus tolérés côté serveur | **0** | **2** |
| `SetWriteDeadline` par écriture | 3000 ms | 3000 ms *(inchangé — voir §5)* |
| Seuil client de liaison morte | 9000 ms *(dérivé)* | **4000 ms** *(transmis, ajustement GATE 2)* |
| Granularité de vérification côté client | 1000 ms | **500 ms** |
| Détection effective d'un lien réellement mort | 9,0 – 10,0 s | **4,0 – 4,5 s** |
| À-coup réseau absorbé sans reconnexion | < 9 s | **< 4 s** |

### Justification du `ReadDeadline` serveur

Pour tolérer `N` pings entièrement perdus avec une cadence `P`, le prochain pong exploitable arrive
au plus tôt à `(N+1) × P` :

```
D ≥ (N+1) × P + RTT_max + marge
D ≥ (2+1) × 2000 + 500 + 500 = 7000 ms
```

**Constat sur la configuration précédente** : avec `P = 3000` et `D = 5000`, un **seul** ping perdu
suffisait à fermer la connexion (pong suivant à 6000 ms > 5000 ms). La marge affichée de 2 s ne
représentait donc aucune tolérance réelle à la perte. C'est le point que ce contrat corrige.

### Ordre de détection — inversion délibérée

Le client détecte désormais **avant** le serveur (4,0–4,5 s contre 7 s), là où le rapport était
inversé (9–10 s contre 5 s). C'est **voulu** : sur un lien mort, la trame de fermeture du serveur
n'atteint jamais le client (problème fondateur de #118) — c'est donc au client de reprendre
l'initiative. La connexion serveur périmée est absorbée par la garde anti-zombie existante
(`IsPlayerIDConnected`, #109/#120) : la nouvelle connexion s'enregistre, puis l'ancienne expire sans
que le joueur soit marqué déconnecté.

---

## 5. Périmètres explicitement non modifiés

| Élément | Valeurs | Raison |
|---|---|---|
| `internal/server/websocket_buzzer.go` (ticker 3 s, `ReadDeadline` 5 s) | inchangées | Firmware ESP32-C3 : resserrer la cadence augmenterait la charge de terminaux contraints et sortirait du périmètre « stabilité VJoueur ». Les buzzers n'ont jamais reçu `HEARTBEAT` (`TestBuzzerHub_NeverEmitsHeartbeat`) |
| `internal/server/logswebsocket.go` (60 s / 10 s) | inchangées | Canal de diagnostic, pas de contrainte de réactivité |
| `SetWriteDeadline` (3000 ms) | inchangée | L'abaisser rapprocherait le seuil d'échec d'écriture, qui **ferme la connexion** dès la première erreur (scénario A, round 1) — l'inverse du but recherché. Le relever relève d'un autre arbitrage, à traiter séparément |

---

## 6. Compatibilité

**Aucun changement BREAKING.** `DEAD_LINK_TIMEOUT_MS` est purement additif.

| Combinaison | Comportement | Verdict |
|---|---|---|
| Nouveau client + nouveau serveur | seuil = 4000 ms | nominal |
| **Ancien** client + nouveau serveur | champ ignoré, seuil = `INTERVAL_MS × 3` = 6000 ms | fonctionne, et bénéficie déjà d'une partie du gain |
| Nouveau client + **ancien** serveur | champ absent, repli sur `INTERVAL_MS × 3` = 9000 ms | fonctionne, comportement #118 à l'identique |
| Client sans aucun `HEARTBEAT` reçu | repli 3000 × 3 = 9000 ms | fonctionne |

Les quatre combinaisons sont fonctionnelles : aucune ne peut produire un seuil nul, négatif ou
inférieur à la cadence de battement.
