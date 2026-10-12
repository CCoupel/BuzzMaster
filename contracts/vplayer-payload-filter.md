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

---

## 5. Ciblage des broadcasts par événement (#129)

> Ajouté par #129 (`_work/reports/planner-20260803-170653.md`,
> maquette `_work/mockups/129-broadcast-targeting.md`). §1-§4 ci-dessus restent inchangés.

#127 réduit **la taille** du payload VJoueur pendant PREPARE/READY. #129 traite la **fréquence** :
trois événements par-participant déclenchaient un `UPDATE` complet vers tous les VJoueurs, à
n'importe quel moment de la partie. Les VJoueurs n'y consomment aucune donnée — sauf, pour certains,
l'écho de leur **propre** état.

### Règle générale

> Un `UPDATE` déclenché par un événement propre à **un** participant ne doit être diffusé à
> l'ensemble des VJoueurs que si son contenu est réellement consommé par eux. À défaut, il est
> adressé à Admin/TV, et **ciblé** sur le seul participant concerné quand celui-ci a besoin de
> l'écho de son propre état.

### Matrice par événement

| Événement | Admin | TV | Participant concerné | Autres VJoueurs | Buzzers |
|---|---|---|---|---|---|
| `onPlayerDisconnected` | `UPDATE` | `UPDATE` | — | **aucun envoi** | `UPDATE` |
| `handlePlayerConnect` (création et reconnexion) | `UPDATE` | `UPDATE` | **`UPDATE` ciblé** | **aucun envoi** | `UPDATE` |
| `ARDOISE_INPUT` | `UPDATE` | **aucun envoi** | **aucun envoi** | **aucun envoi** | **aucun envoi** |
| `PONG` en phase `PREPARE` | `UPDATE` | **aucun envoi** | aucun envoi *(déjà #127)* | aucun envoi *(déjà #127)* | `UPDATE` |
| `BUTTON` / `VPLAYER_QCM_ANSWER` *(phase 3, séparable)* | `UPDATE` | `UPDATE` | **`UPDATE` ciblé** | **aucun envoi** | `UPDATE` |

La ligne `PONG` complète la matrice §1 : #127 avait retiré les VJoueurs de la rafale per-PONG en
conservant Admin + TV + buzzers ; #129 en retire aussi la TV. Les **deux bornes** de la fenêtre lui
restent acquises — entrée en PREPARE (`handleReady`) et transition en READY (`broadcastGameState`).

### Justification du ciblage, consommateur par consommateur

| Donnée retirée | Qui la lisait côté VJoueur | Verdict |
|---|---|---|
| État de connexion des autres joueurs | `sortedPlayers` — gaté sur `!isVPlayer` depuis #127 T3.1 | non consommée |
| `ARDOISE_ANSWERS` | aucun : `VPlayerPage.jsx` gère sa saisie en état local (`ardoiseText`) et ne relit jamais ce champ ; `PlayerDisplay.jsx:2427` est gaté `showAnswer && !isVPlayer` | non consommée |
| `ARDOISE_ANSWERS` côté TV pendant la saisie | `PlayerDisplay.jsx:2427` ne rend qu'en phase `REVEALED` | non consommée avant REVEAL |
| Horodatage de buzz des autres joueurs | `teamsByQcmAnswer` — ne rend qu'en `STOPPED`/`REVEALED` | non consommée pendant `STARTED` |
| Progression « prêt » côté TV pendant PREPARE | `PlayerDisplay.jsx:1358-1373` (`showPrepare`) n'affiche qu'un libellé statique, sans référence à `Ready`/`bumpers`/`teams` ; le carrousel de scores ne lit que `SCORE`/`COLOR`/`NAME`, inchangés d'un PONG à l'autre | non consommée |

### Exception explicite — les buzzers physiques restent servis en per-PONG

`broadcastGameState()` ne cible pas `ClientTypeBuzzer` : l'`UPDATE` per-PONG de `handlePong` est donc
le **seul** signal de phase que reçoivent les buzzers physiques sur le chemin WebSocket pendant la
fenêtre PREPARE→READY, et le firmware s'en sert (`handleUpdateAction` — assignation d'équipe,
rotation grise). **Ne pas les retirer de ce broadcast** sans traiter d'abord leur alimentation en
changements de phase. Leur payload (`SerializeForBuzzer`) est par ailleurs le plus léger des quatre.

### Écho ciblé — obligation

`handlePlayerConnect` **doit** émettre un `UPDATE` ciblé vers le participant concerné : c'est par la
carte `bumpers` que le VJoueur retrouve son bumper et son `CONNECTED` après reconnexion
(`VPlayerPage.jsx:112-114`). Le retirer sans écho casserait #118/#120/#122.

**Contrainte** : le chemin d'écho ciblé ne doit **jamais** appeler
`ApplyVPlayerBroadcastConnEvents()` — cette fonction évalue le registre de badges pour **tout** le
plateau, et la déclencher sur un envoi à un seul destinataire marquerait `MessageLost` sur tous les
autres VJoueurs qui n'ont rien reçu. Même invariant que #127 CA7.

### État initial à la connexion — inchangé

`sendStateToClient()` (émis sur `HELLO`) continue d'envoyer le payload **complet et non réduit** au
client qui se connecte, quel que soit son type. C'est délibéré et **ne doit pas être « optimisé »** :
c'est la seule source dont dispose une session sans `vplayer_id` pour retrouver son identité par
balayage `NAME` (cf. §2, condition 3).

### Regroupement temporel (`ARDOISE_INPUT`, phase 2 — séparable)

Les `UPDATE` déclenchés par `ARDOISE_INPUT` peuvent être regroupés sur une fenêtre ≤ 150 ms.

**Invariant qui rend le regroupement sûr** : un payload `UPDATE` est toujours construit **au moment
de l'émission** à partir de l'état vivant du moteur, jamais mis en tampon à l'avance. Une émission
retardée ne peut donc être que **redondante**, jamais périmée.

Deux obligations : *(a)* tout changement de phase vide immédiatement le regroupement en attente
(les dernières frappes doivent être présentes à l'affichage du résultat) ; *(b)* la fenêtre reste
≤ 150 ms, pour que la saisie reste perçue comme temps réel côté admin.

### Compatibilité

**Aucun changement BREAKING.** Aucun message, champ ni type n'est ajouté, renommé, retiré ni retypé.
Seul le **jeu des destinataires** de messages existants change.

**Correction d'équité obtenue au passage** : `ARDOISE_ANSWERS` — donc le texte que les autres équipes
sont en train de saisir — n'est plus transmis aux navigateurs des VJoueurs **par le broadcast
déclenché par la frappe**.

> ⚠️ **Rectification (#128, 2026-08-20)** : cette phrase a longtemps été lue comme « le VJoueur ne
> reçoit plus les réponses des autres équipes ». C'était faux. #129 a fermé un **déclencheur**, pas
> la fuite : le champ restait dans le nœud `GAME`, et **tout autre** broadcast le transportait —
> au premier rang desquels le tic de chronomètre, **une fois par seconde**. Voir §6.

---

## 6. Retrait par champ, toutes actions confondues (#128, v6.5.2)

### Deux défauts, à corriger ensemble

| | Défaut | Portée |
|---|---|---|
| **A** | Le filtrage par type de client ne s'appliquait **qu'à `ActionUpdate`** | **Tous** les champs réservés à l'admin, vers TV, VJoueur et animateur |
| **B** | `ARDOISE_ANSWERS` n'appartenait à **aucune** liste de retrait | Le VJoueur le recevait **aussi** sur `ActionUpdate` |

Corriger A seul laisse `ARDOISE_ANSWERS` partir, faute d'appartenir à une liste. Corriger B seul
laisse les autres actions passer à côté du filtre. Les deux sont nécessaires.

**Sept sites de diffusion** transportaient le `GameState` complet vers le VJoueur sans aucun
filtrage — `broadcastStart`, `broadcastStop`, `broadcastPause`, `broadcastPauseAll`,
`broadcastContinue`, `broadcastTimerUpdate`, `broadcastCountdownUpdate` — soit les actions `START`,
`STOP`, `PAUSE`, `CONTINUE` et `UPDATE_TIMER`. **`UPDATE_TIMER` part une fois par seconde** : la
fuite n'était pas ponctuelle, elle était continue.

Fuyaient à chacun de ces envois : `QUIZ_OBJECTIVES` (dont la règle de confidentialité v6.1.0 était
donc inopérante hors `UPDATE`), `ARDOISE_ANSWERS`, et par buzzer `FIRMWARE_VERSION`, `IS_OUTDATED`,
`OTA_STATUS`, `OTA_PERCENT`, `ACK_PENDING`.

### Règle — la forme de la charge utile, jamais le nom de l'action

> Une charge utile qui porte un nœud `GAME` est filtrée. Une charge utile qui n'en porte pas passe
> intacte.

Énumérer les actions concernées reproduirait exactement le défaut corrigé : la prochaine action
transportant le `GameState` serait ajoutée sans que personne ne pense à cette liste, et la fuite
reviendrait en silence. Le seul critère qui ne se périme pas est ce que la charge utile **contient**.

En pratique, `SerializeForWebClient` n'a plus de garde sur l'action : elle désérialise, et **en cas
d'échec ou d'absence des clés attendues, renvoie la charge utile intacte**. Les actions sans nœud
`GAME` (`REVEAL`, `HELLO`, `LED_SET`…) empruntent ce repli et sont **inchangées**.

`SerializeForVPlayer` conserve en revanche sa garde `ActionUpdate` : elle protège la **réduction du
nœud `bumpers`**, qui reste propre à `UPDATE` (§2). Deux mécanismes distincts dans la même fonction
— le retrait de champs, qui s'applique partout via le repli, et la réduction de `bumpers`, qui reste
conditionnelle.

### Une liste de retrait propre au VJoueur

`ARDOISE_ANSWERS` ne peut pas simplement rejoindre `AdminOnlyGameFields` : **la TV et l'animateur en
ont un besoin légitime** — la TV affiche les réponses par équipe au REVEAL, `/anim` les liste en
direct (#158). Le retrait doit viser le seul VJoueur, ce que le mécanisme binaire admin/non-admin ne
savait pas exprimer.

D'où une seconde liste exportée, `VPlayerOnlyGameFields`, appliquée au seul chemin VJoueur :

| Champ | admin | tv | anim | **player** | buzzer |
|---|:---:|:---:|:---:|:---:|:---:|
| `QUIZ_OBJECTIVES` | ✅ | ❌ | ❌ | ❌ | ❌ |
| `ARDOISE_ANSWERS` | ✅ | ✅ | ✅ | **❌** | ❌ |

Conséquence : le VJoueur ne partage plus sa charge utile avec la TV et l'animateur. Il lui faut son
propre sérialiseur sur le chemin générique — `serializeForClientType` le routait jusqu'ici vers
`SerializeForWebClient`, comme la TV.

### Trois sites, une seule liste

Le retrait des champs `GAME` est ré-implémenté sur **trois** chemins — `SerializeForWebClient`,
`SerializeForVPlayer`, et la fonction de retrait du chemin chaud de fan-out (`cmd/server/main.go`).
**Les trois doivent lire les listes exportées.** Un site oublié laisse la fuite ouverte sur ce
chemin, sans erreur ni symptôme visible : c'est le risque principal de ce correctif.

### Risque résiduel assumé

`ARDOISE_ANSWERS` **reste envoyé à la TV**, qui en a besoin au REVEAL. Le projet n'ayant aucune
authentification, un joueur qui connaît l'URL peut ouvrir `/tv` et y lire les réponses avant le
REVEAL. **Ce chemin n'est pas fermé par ce lot.**

Le fermer supposerait soit de ne transmettre `ARDOISE_ANSWERS` à la TV qu'en phase `REVEALED` —
filtrage dépendant de la phase, plus fragile — soit de contrôler l'accès à `/tv`, hors périmètre et
sans objet dans une application qui n'a pas d'authentification.

Ce qui est fermé, et qui était l'objet de #128 : le chemin **par défaut**, celui qu'un joueur
emprunte sans rien chercher — les outils de développement sur sa propre page.
