# Contrat Sérialisation Payload WebSocket par type de client (v3.8.0)

> **Feature** : #41 étendu — Réduire payload WS pour tous les types de clients  
> **Branche** : `feature/ws-broadcast-ack-v380`  
> **Créé** : 2026-04-28 (analyse QUALIF — VPlayerPage embed PlayerDisplay)  
> **Mis à jour** : 2026-08-13 (#155/#156, v6.2.0) — décision de sérialisation pour `ClientTypeAnim`,
> voir §"Animateur" ci-dessous.
> **Mis à jour** : 2026-08-14 (#163, v6.2.0.10) — clarification documentaire (aucun changement de
> payload) sur `QCM_CORRECT`/`ANSWER` côté animateur, voir §"Clarification (#163 → révisée #166)"
> ci-dessous.
> **Mis à jour** : 2026-08-15 (#166, v6.2.0.15) — la clarification #163 ci-dessus est **corrigée** :
> la réponse n'est plus retenue jusqu'au reveal, elle est rendue en permanence et floutée. Aucun
> changement de payload.

---

## Contexte

VPlayerPage.jsx embarque directement `<PlayerDisplay>` — la page TV de poche.
Les besoins TV et VPlayer sont donc identiques côté messages.

---

## Tableau 1 — Filtrage par type de message

| Action (Server→Client) | Admin | TV | VPlayer | Buzzer |
|------------------------|-------|----|---------|--------|
| `UPDATE` / `UPDATE_TIMER` | ✅ full | ✅ partiel | ✅ partiel | ✅ réduit |
| `START` / `CONTINUE` / `STOP` / `PAUSE` | ✅ | ✅ | ✅ | ✅ |
| `READY` / `RESET` | ✅ | ✅ | ✅ | ✅ |
| `REVEAL` | ✅ | ✅ | ✅ | ❌ |
| `QUESTIONS` | ✅ | ❌ | ❌ | ❌ |
| `CLIENTS` | ✅ | ❌ | ❌ | ❌ |
| `FIRMWARE_VERSION` | ✅ | ❌ | ❌ | ❌ |
| `BACKGROUND_CHANGE` | ✅ | ✅ | ✅ | ❌ |
| `QCM_HINT` | ✅ | ✅ | ✅ | ❌ |
| `CONFIG_UPDATE` | ✅ | ✅ | ✅ | ❌ |
| `ENROLLMENT_UPDATE` | ✅ | ✅ | ✅ | ❌ |
| `TEAM_POINTS` / `BUMPER_POINTS` | ✅ | ✅ | ✅ | ❌ |
| `PLAYER_REJECTED` | ❌ | ❌ | ✅ | ❌ |
| `PLAYER_CONNECTED` / `PLAYER_ASSIGNED` | ✅ | ❌ | ✅ | ❌ |
| `MEMORY_SET_TEAMS` / `FLIP_MEMORY_CARD` | ✅ | ✅ | ❌ | ❌ |
| `REMOTE` | ✅ | ✅ | ❌ | ❌ |
| `HELLO` | ❌ | ❌ | ❌ | ✅ |
| `LED_SET` / `OTA_UPDATE` / `WIFI_CONFIG` | ❌ | ❌ | ❌ | ✅ |

---

## Tableau 2 — Payload UPDATE par type de client

| Champ `UPDATE.MSG` | Admin | TV + VPlayer | Buzzer |
|--------------------|-------|-------------|--------|
| `phase` | ✅ | ✅ | ❌ |
| `timer` | ✅ | ✅ | ❌ |
| `question` (texte, TYPE, média, catégorie) | ✅ | ✅ | ❌ |
| `question.QCM_ANSWERS` | ✅ | ✅ | ❌ |
| `question.MEMORY_*` | ✅ | ✅ | ❌ |
| `teams[*].NAME / COLOR / SCORE / STATUS` | ✅ | ✅ | ✅ (toutes équipes) |
| `teams[*].MEMBERS` | ✅ | ✅ | ❌ |
| `bumpers[*].ID / NAME / TEAM / CONNECTED / IS_VIRTUAL / TIME / ANSWER_COLOR` | ✅ | ✅ | ✅ (tous buzzers) |
| `bumpers[*].FIRMWARE_VERSION` | ✅ | ❌ | ❌ |
| `bumpers[*].IS_OUTDATED / OTA_STATUS` | ✅ | ❌ | ❌ |
| `bumpers[*].ACK_PENDING` | ✅ | ❌ | ❌ |
| `GAME.QUIZ_*` sauf `QUIZ_OBJECTIVES` | ✅ | ✅ | ❌ |
| `GAME.QUIZ_HIDDEN_FIELDS` (**v6.1.0**) | ✅ | ✅ — la TV en a besoin pour **appliquer** la préférence | ❌ |
| `GAME.QUIZ_OBJECTIVES` (**v6.1.0**) | ✅ | ❌ | ❌ |
| `config` (paramètres serveur) | ✅ | ❌ | ❌ |
| `history` / `palmares` | ✅ | ✅ | ❌ |
| `remote` | ✅ | ✅ | ❌ |
| `neonEffect` | ✅ | ✅ | ❌ |
| `enrollmentOpen` | ✅ | ✅ | ❌ |

---

## Animateur (`ClientTypeAnim`, v6.2.0 — #155/#156)

### Décision : pas de sérialiseur dédié — réutilisation de `SerializeForWebClient`

`serializeForClientType` (`internal/server/websocket.go`, la fonction qui choisit le sérialiseur
par type pour `BroadcastToTypes` — voir `contracts/websocket-actions.md` sur l'allow-list
entrante #154 pour le contexte de cette fonction) route `ClientTypeAnim` vers le **même**
`SerializeForWebClient` que TV et VPlayer, exactement comme le fait déjà
`serializeForClientType(ClientTypeVPlayer)`. **Aucun `SerializeForAnim` n'est créé.**

### Justification

- Le besoin payload de l'animateur (Tableaux 1 et 2 ci-dessus) est un **sous-ensemble strict** de
  ce que `SerializeForWebClient` retire déjà pour TV/VPlayer : aucun champ firmware/OTA/ACK
  (`AdminOnlyBumperFields`), aucun `QUIZ_OBJECTIVES` (`AdminOnlyGameFields`), aucune `config`
  serveur. Un quatrième sérialiseur quasi identique dupliquerait ces deux listes partagées sans
  bénéfice — l'avertissement déjà présent plus haut dans ce document (« Trois sites doivent
  appliquer ce retrait, pas un seul ») deviendrait « quatre sites », pour un contenu strictement
  identique.
- La réduction *supplémentaire* propre à l'animateur ne porte pas sur le contenu de `bumpers`/
  `teams`/`GAME` (ce que `SerializeForWebClient` filtre), mais sur **quelles actions lui sont
  diffusées du tout** — `QUESTIONS`, `CLIENTS` et `CONFIG_UPDATE` ne sont jamais envoyées à
  `ClientTypeAnim` (voir `contracts/websocket-endpoints.md` §"Filtres de diffusion par type").
  C'est une décision **au niveau de l'appelant** (quels types figurent dans les arguments de
  `BroadcastToTypes`/`sendStateToClient`), pas au niveau du sérialiseur — un sérialiseur ne décide
  jamais QUI reçoit un message, seulement CE QUI est dans le message envoyé à un type donné.
  `NEXT_QUESTION` suit la même logique en sens inverse : diffusée uniquement à `ClientTypeAnim`
  via `BroadcastToTypes(msg, ClientTypeAnim)`, sans avoir besoin d'un sérialiseur particulier
  puisqu'un seul type la reçoit.

### ⚠️ Piège à ne pas réintroduire

`serializeForClientType` (`websocket.go`) se termine par un `default: return msg.SerializeForAdmin()`.
**Un `ClientTypeAnim` non explicitement ajouté au `switch` recevrait donc, par défaut, le payload
admin COMPLET** (firmware/OTA/ACK, `QUIZ_OBJECTIVES`) — l'exact inverse de la décision ci-dessus.
L'ajout du `case ClientTypeAnim: return msg.SerializeForWebClient()` est la toute première ligne de
code de #155 (tâche B1) et doit être couverte par un test dédié qui vérifie explicitement
l'absence de ces champs — pas seulement par relecture. Voir
`_work/reports/plan-20260813-092950.md` §0.2 pour l'analyse complète de ce risque.

### Clarification (#163 → révisée #166, v6.2.0.15) — `QCM_CORRECT` / `ANSWER` atteignent `/anim` avant `REVEALED`

**Constat, pas un changement de contrat — mais la nature de l'affichage a changé entre #163 et
#166, cette section est corrigée en conséquence.** Depuis #163, `question.QCM_CORRECT` et
`question.ANSWER` transitent sur `/ws/anim` dès le chargement de la question (`SerializeForWebClient`
ne les retire pas — voir Tableau 1/2 ci-dessus, aucune ligne dédiée), exactement comme pour TV et
VPlayer. **Ce constat est inchangé.** Ce qui change, c'est ce que le client en fait avant `REVEALED` :

- **#163 (v6.2.0.10, dépassé)** : la réponse n'était **pas rendue du tout** avant `REVEALED` — une
  garde de rendu conditionnelle (`revealed && ...`) empêchait toute apparition dans le DOM.
- **#166 (v6.2.0.15, actuel)** : la réponse est **rendue en permanence** dès qu'une question est
  chargée, dans un composant dédié (`AnimAnswerZone.jsx`), **floutée** (`filter: blur()`) et
  d'opacité réduite tant que `phase !== 'REVEALED'`, nette ensuite — mêmes dimensions dans les deux
  états, seul le style change. `AnimQcmOptions.jsx` conserve en parallèle son propre marquage
  (liseré vert + ✓) sous la même garde `revealed`, désormais purement décoratif au-dessus d'une
  donnée déjà présente à l'écran.

**Le flou n'est ni un masque ni un mécanisme de confidentialité.** Il évite la lecture involontaire
par l'animateur — c'est son seul objectif — mais ne résiste ni à un regard appuyé, ni aux outils de
développement du navigateur, ni à une copie du texte (`user-select: none` bloque uniquement la
sélection à la souris). Aucun filtrage serveur n'est ajouté — cela casserait l'affichage au moment
du reveal, puisque le payload ne serait alors mis à jour qu'à `REVEAL`, pas avant.

Conséquence pratique, **renforcée** par rapport à #163 (déjà documentée en risque R4 du plan
#166) : la réponse est désormais visible à l'écran en permanence, sous un voile CSS — la tablette
animateur ne doit pas être visible du public. C'était déjà vrai quand la réponse n'était que dans
le payload (#163) ; ça l'est davantage maintenant qu'elle est physiquement à l'écran (#166).

**Aucun tableau ci-dessus n'est modifié par cette clarification** — elle documente un comportement
de rendu client déjà en vigueur depuis #155/#156 (le payload) et #166 (l'affichage flouté).

---

## Sérialiseurs à implémenter (backend Go)

### `SerializeForAdmin(msg)` — payload complet (existant, inchangé)

### `SerializeForWebClient(msg)` — TV + VPlayer

Supprimer de `UPDATE.MSG` :
- `bumpers[*].FIRMWARE_VERSION`
- `bumpers[*].IS_OUTDATED`
- `bumpers[*].OTA_STATUS`
- `bumpers[*].ACK_PENDING`
- `config`
- **`GAME.QUIZ_OBJECTIVES` (v6.1.0)** — premier champ du nœud `GAME` à être filtré par type de
  client. Ce n'est pas une optimisation de payload mais une **règle de confidentialité** :
  l'objectif de la partie est une consigne d'animation qui ne doit pas être lisible depuis un
  écran TV ni depuis les outils de développement d'un VJoueur (`game-state.md`,
  § « `QUIZ_OBJECTIVES` — champ à diffusion restreinte »).

> ⚠️ **Trois sites doivent appliquer ce retrait, pas un seul.** Le filtrage des champs
> admin-only est aujourd'hui dupliqué entre `SerializeForWebClient`
> (`internal/protocol/messages.go:560`), `SerializeForVPlayer` (`:625`) et le fan-out chaud
> (`cmd/server/main.go:2718`), qui réimplémente la règle pour éviter un aller-retour
> `map[string]interface{}` par destinataire. La discipline en place pour les champs de bumper —
> **une seule liste exportée et partagée** (`AdminOnlyBumperFields`, `messages.go:542`) — doit
> être reprise à l'identique pour les champs du nœud `GAME` (p. ex. `AdminOnlyGameFields`).
> Un chemin oublié = une fuite silencieuse, invisible en test unitaire du sérialiseur.

### `SerializeForBuzzer(msg)` — buzzers physiques

Conserver uniquement dans `UPDATE.MSG` :
- `bumpers[*]` : `{ID, NAME, TEAM, CONNECTED, IS_VIRTUAL, TIME}` (sans champs firmware/ACK)
- `teams[*]` : `{NAME, COLOR, STATUS}` (sans MEMBERS)

Supprimer entièrement : `question`, `timer`, `phase`, `config`, `history`, `remote`, `neonEffect`, `enrollmentOpen`

---

## Impact réseau estimé

| Type | UPDATE actuel | UPDATE après |
|------|--------------|-------------|
| Admin | ~5 KB | ~5 KB (inchangé) |
| TV / VPlayer | ~5 KB | ~3 KB (−40% : sans firmware/OTA/config) |
| Buzzer | ~5 KB | ~0.5 KB (−90% : bumpers+teams uniquement) |

---

## Implémentation recommandée

Dans `WebSocketHub.BroadcastToTypes()`, passer la sérialisation en paramètre selon le type :

```go
func (h *WebSocketHub) broadcastUpdate(msg *protocol.Message) {
    adminPayload := msg.SerializeForAdmin()
    webPayload   := msg.SerializeForWebClient()

    h.mu.RLock()
    defer h.mu.RUnlock()
    for client := range h.clients {
        switch client.Type {
        case ClientTypeAdmin:
            client.Send <- adminPayload
        case ClientTypeTV, ClientTypeVPlayer:
            client.Send <- webPayload
        }
    }
}
```

Buzzer géré séparément via `BuzzerWebSocketHub.BroadcastIfRelevant()` — voir `buzzer-payload-filter.md`.

---

## ENTRACTE / ENTRACTE_CONFIG (v6.5.2, #119)

**Aucun filtre n'est modifié par ce lot.** Les deux champs sont ajoutés au nœud `GAME` et se
comportent comme n'importe quel champ non listé dans `AdminOnlyGameFields` :

| Type de client | Reçoit `ENTRACTE` / `ENTRACTE_CONFIG` | Mécanisme |
|---|---|---|
| `admin` | ✅ | `SerializeForAdmin` — marshal complet |
| `tv` | ✅ | `SerializeForWebClient` — liste de **retrait**, les clés inconnues passent |
| `player` | ✅ | idem, y compris sur le chemin chaud (`GAME` est recopié tel quel en `json.RawMessage`) |
| `anim` | ✅ | routé vers `SerializeForWebClient` |
| `buzzer` | ❌ | `SerializeForBuzzer` est une liste d'**autorisation** (`PHASE`, `TIME`, `CURRENT_TIME`) |

### ⚠️ Rectification #128 (v6.5.2) — le filtrage ne se limitait qu'à `UPDATE`

Jusqu'à #128, `SerializeForWebClient` renvoyait la charge utile **intacte** pour toute action autre
que `ActionUpdate`. Or `START`, `STOP`, `PAUSE`, `CONTINUE` et `UPDATE_TIMER` transportent le
`GameState` complet vers TV, VJoueur et animateur — `UPDATE_TIMER` **une fois par seconde**. Tous les
champs réservés à l'admin (`QUIZ_OBJECTIVES`, métadonnées firmware/OTA/ACK par buzzer) leur
parvenaient donc en clair, et la règle de confidentialité de `QUIZ_OBJECTIVES` posée en v6.1.0 était
inopérante hors `UPDATE`.

**La règle est désormais fondée sur la forme de la charge utile, pas sur le nom de l'action** : une
charge utile portant un nœud `GAME` est filtrée, les autres passent intactes (repli existant en cas
d'échec de désérialisation). Énumérer les actions concernées reproduirait le défaut à la première
action ajoutée. Détail complet : `contracts/vplayer-payload-filter.md` §6.

`ARDOISE_ANSWERS` (#128) est retiré **du seul VJoueur** : la TV l'affiche au REVEAL et `/anim` le
liste en direct (#158). Il ne peut donc pas rejoindre `AdminOnlyGameFields` — d'où une seconde liste
exportée, `VPlayerOnlyGameFields`, appliquée sur le seul chemin VJoueur. Conséquence : le VJoueur ne
partage plus sa charge utile avec la TV et l'animateur.

`ENTRACTE_CONFIG_SAVED` (v6.5.2, arbitrage 2026-08-20) fait exception : il rejoint
`AdminOnlyGameFields`, aux côtés de `QUIZ_OBJECTIVES`, et n'atteint donc **que l'admin**. C'est la
configuration *enregistrée*, distincte de la configuration *diffusée* gelée pendant une pause ;
seule la page Quiz l'utilise. Rappel du piège documenté plus haut : ce retrait est ré-implémenté sur
**trois** sites, qui lisent tous la même liste exportée — ne jamais écrire ce nom de champ en dur
ailleurs.

L'exclusion des buzzers est **voulue** : les LEDs sont pilotées par le serveur depuis la v3.4.0, le
firmware n'a aucune décision à prendre sur l'entracte. Ne pas ajouter `ENTRACTE` à la liste
d'autorisation du sérialiseur buzzer.

> Rappel du piège documenté plus haut : le retrait des champs admin est ré-implémenté sur **trois**
> sites. Ce lot n'y touche pas — mais toute tentative future de restreindre `ENTRACTE` à certains
> types devrait passer par la liste partagée exportée, jamais par un littéral en ligne.
