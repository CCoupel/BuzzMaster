# Contrat Filtrage Payload VJoueur (#127)

> **Bugfix** : #127 — rafale de broadcasts non groupée à PREPARE→READY, déconnexions VJoueur
> **Plan** : `_work/reports/planner-20260802-212049.md`
> **Maquette de référence** : `_work/mockups/127-broadcast-matrix.md`
> **Dernière mise à jour** : 2026-08-02

Ce contrat est la **référence unique** pour ce que reçoit un client `/ws/player` (`ClientTypeVPlayer`).
Il complète `buzzer-payload-filter.md` (même principe, pour les buzzers physiques).

---

## 1. Matrice de diffusion pendant PREPARE → READY

Portée : uniquement la fenêtre PREPARE→READY. Toutes les autres phases sont inchangées.

| Événement serveur | Admin | TV | VJoueur | Buzzers |
|---|---|---|---|---|
| `handleReady()` — entrée en PREPARE | `UPDATE` complet | `UPDATE` filtré | `UPDATE` **réduit** | `UPDATE` minimal |
| `handlePong()` — chaque PONG, tant que tous ne sont pas prêts | `UPDATE` complet | `UPDATE` filtré | **rien** *(changement #127)* | `UPDATE` minimal |
| `TransitionToReady()` → `OnStateChange` | `UPDATE` complet | `UPDATE` **filtré** *(changement #127)* | `UPDATE` **réduit** *(changement #127)* | inchangé |
| `broadcastReady()` — action `READY` | `READY` | `READY` | jamais (inchangé) | `READY` |
| `handlePong()` — dernier PONG, après la transition | `UPDATE` complet | `UPDATE` filtré | **rien** *(changement #127)* | `UPDATE` minimal |

**Conséquence contractuelle** : sur une partie à N participants, un VJoueur recevait N+2 messages
`UPDATE` dans cette fenêtre ; il en reçoit désormais exactement **2** (entrée en PREPARE, puis
transition en READY), quel que soit N.

**Le VJoueur ne reçoit toujours jamais l'action `READY`** — c'est le champ `GAME.PHASE` du second
`UPDATE` qui porte l'information. Comportement inchangé, rappelé ici car il est structurant.

---

## 2. Payload `UPDATE` réduit destiné à `ClientTypeVPlayer`

### Règle d'application — conditionnelle, pas globale

Le payload est réduit **si et seulement si les trois conditions sont réunies** :

1. `Action == "UPDATE"` — les autres actions sont sérialisées à l'identique ;
2. `MSG.GAME.PHASE ∈ { "PREPARE", "READY" }` — lu dans le payload lui-même, jamais passé en paramètre ;
3. le client destinataire a un `PlayerID` identifié (`SetClientPlayerID` déjà appelé, c.-à-d.
   après `PLAYER_CONNECT`).

Si une seule condition manque → **payload complet filtré** (`SerializeForWebClient`, comportement actuel).

> **Pourquoi la condition 3 est obligatoire** : une session VJoueur antérieure à l'identité par ID
> (`playerSession.id` absent) retrouve son bumper en **balayant la carte par NAME**
> (`VPlayerPage.jsx:118-122`). Un client non encore identifié côté serveur doit donc recevoir la
> carte complète, sinon il ne peut plus jamais s'identifier.

### Contenu du payload réduit

```jsonc
{
  "ACTION": "UPDATE",
  "VERSION": "…",
  "MSG": {
    "GAME":    { /* inchangé, à l'identique — aucun champ retiré */ },
    "teams":   { /* inchangé — TOUTES les équipes */ },
    "bumpers": { "<PlayerID du destinataire>": { /* son bumper, tous champs, moins OTA/ACK */ } }
  }
}
```

| Clé | Traitement | Justification |
|---|---|---|
| `GAME` | **inchangé** | contient `PHASE`, `QUESTION`, MEMORY/MEMOTION/ARDOISE — consommé par `PlayerDisplay` monté en mode VJoueur |
| `teams` | **inchangé, carte complète** | `PlayerDisplay.jsx:1822` (barre équipes MEMORY) et `:2069` (barre équipes MEMOTION) lisent `teams[nom]` **sans gating `!isVPlayer`** — réduire casserait la couleur des chips des autres équipes. Coût mesuré : 96 o/équipe, négligeable. |
| `bumpers` | **réduit au seul bumper du destinataire**, champs OTA/ACK retirés comme dans `SerializeForWebClient` | seul `bumpers[playerSession.id]` est lu côté VJoueur (`VPlayerPage.jsx:112-114`) |
| `config` | retiré (comme `SerializeForWebClient`) | inchangé |

### Ce qui n'est PAS réduit — et pourquoi

Ces trois consommateurs de la carte complète **ne sont pas gatés sur `!isVPlayer`** et sont donc
protégés par la condition de phase (aucun d'eux ne rend pendant PREPARE/READY) :

| Consommateur | Fichier | Phases où il rend | Données nécessaires |
|---|---|---|---|
| Barre d'équipes MEMORY | `PlayerDisplay.jsx:1817-1852` | `COUNTDOWN`, `STARTED`, `PAUSED`, `REVEALED` | `teams` complet |
| Barre d'équipes MEMOTION | `PlayerDisplay.jsx:2064-2110` | hors PREPARE/READY | `teams` complet |
| Badges d'équipe QCM | `PlayerDisplay.jsx:690-732`, `:1541-1576` | `STOPPED`, `REVEALED` | `bumpers` complet |

**Invariant à préserver** : après READY, le premier `UPDATE` reçu par le VJoueur (buzz → `broadcastUpdate`,
ou changement de phase → `broadcastGameState`) porte une phase hors PREPARE/READY et **restaure donc la
carte complète** avant que l'un de ces consommateurs n'en ait besoin.

---

## 3. Règle de fan-out (implémentation contrainte)

Le payload réduit dépendant du destinataire, la diffusion aux VJoueurs passe d'un envoi unique
mutualisé à un envoi individualisé. **Contrainte de contrat** (et non simple recommandation) :

- **aucun `json.Marshal` ne doit être exécuté en tenant `WebSocketHub.mu`** — le verrou est global et
  la contention sous verrou est précisément l'un des mécanismes suspectés dans #127 ;
- séquence imposée : *(a)* relevé des destinataires sous `RLock`, *(b)* construction des payloads
  **hors verrou**, *(c)* envoi des octets pré-sérialisés sous un unique `Lock` ;
- le nœud `GAME` (jusqu'à ~11 Ko en MEMOTION) doit être conservé en `json.RawMessage` et **jamais
  re-sérialisé** par destinataire : un seul `Unmarshal` du payload commun, puis N `Marshal` ne portant
  que la carte `bumpers`.

---

## 4. Compatibilité

**Aucun changement BREAKING.**

- Le schéma des objets n'est pas modifié : ni champ renommé, ni champ retiré d'un bumper conservé,
  ni type changé. Seul le **nombre d'entrées** de la carte `bumpers` varie, sur deux messages, pour
  un seul type de client.
- Un client antérieur qui itérerait `bumpers` pendant PREPARE/READY verrait une entrée au lieu de N —
  audit fait (§2) : aucun consommateur VJoueur ne le fait dans ces phases.
- Admin, TV et buzzers physiques reçoivent exactement ce qu'ils recevaient (à la correction de
  filtrage de `broadcastGameState` près, qui ne fait que retirer aux TV des champs OTA/ACK
  qu'elles n'utilisent pas).
