# Contrat — Pilote Philips Hue Bridge (API REST locale)

> **Étend** `contracts/lighting.md` (#205). Tout ce qui n'est pas redéfini ici y reste valable.
> **Issues** : #206 (pilote), #207 (configuration + écran d'administration), #213 (éclairage par équipe)
> **Base validée** : `_work/reports/dev-backend-hue-bridge-spike-20260903-195500.md`, code `spike/hue-bridge/`
> **Plan** : `_work/reports/planner-v10-replan-hue-bridge-20260903-220000.md`
>
> Ce contrat est **normatif**. `dev-backend` peut l'ajuster si une contrainte technique l'impose,
> en documentant la raison (`contracts/README.md`), mais pas par confort d'implémentation.

---

## 1. Base technique — ce qui est déjà prouvé

Le spike `spike/hue-bridge/` a été exécuté sur un **vrai** Hue Bridge v2 (BSB002). Ces résultats
sont acquis et ne sont pas à re-démontrer :

| Point | Résultat mesuré |
|---|---|
| Découverte | mDNS `_hue._tcp` en **0,2 s**, puis SSDP en repli. **Aucune IP fixe requise**, aucun appel cloud. |
| Enregistrement | `POST /api` + appui bouton, erreur `101` tant que non pressé, relance toutes les 2 s. |
| Latence d'écriture | **19-61 ms**, p95 **48-59 ms**. |
| Fiabilité | **100 %** à 300 ms **et** à 150 ms d'intervalle (≈7 commandes/s). |
| Dépendances | **stdlib + `grandcat/zeroconf`**, déjà dans `server-go/go.mod`. « 100 % Go » tenu. |
| Conversion couleur | `rgbToXY` implémentée et exercée (`spike/hue-bridge/hue.go:313`). |

**Réutiliser le code du spike plutôt que le réécrire** : `discover.go`, `rgbToXY`, la boucle
d'enregistrement et le garde-fou `guardRequest` sont directement transposables.

---

## 2. Décision — ampoules individuelles, pas groupes/zones Hue

> **Tranché : le pilote adresse des ampoules individuelles. BuzzMaster détient le mapping
> équipe → ampoules dans sa propre configuration, et ne délègue aucun regroupement au bridge.**

La question méritait d'être posée : l'API Hue sait piloter un groupe en un seul appel, et
« une équipe = une zone » est séduisant. Quatre raisons tranchent contre.

1. **Le budget de débit est 10× plus favorable aux ampoules.** Philips recommande ~**10
   commandes/s** sur `/lights` (100 ms entre appels) mais **≤ 1 mise à jour/s** sur `/groups` — un
   groupe passe par une **diffusion Zigbee**, plus coûteuse pour le réseau maillé. Notre nombre
   d'ampoules est petit (quelques unités) : `N` écritures unitaires coûtent moins cher, dans ce
   budget, qu'une commande de groupe.
2. **#213 annule l'avantage principal du groupe.** Un groupe impose **un état unique à tous ses
   membres**. Or dès qu'on colore par équipe, chaque cible a une couleur **différente** — il
   faudrait un groupe par équipe, souvent d'une seule ampoule. Le groupe ne sert que le cas « toute
   la salle d'une seule couleur », qui n'est pas le cas différenciant du milestone.
3. **Aucune dépendance à un état externe qui peut dériver.** Une zone créée à la main dans
   l'application Hue peut être renommée, supprimée, recréée avec un autre id, modifiée par
   quelqu'un d'autre. La configuration BuzzMaster, elle, est sous notre contrôle et sauvegardée
   avec le reste.
4. **C'est ce que le spike a validé**, à 100 % sur du matériel réel. Les groupes ne l'ont pas été.

**Ce que l'on perd, et comment on le compense.** Le groupcast Zigbee est réellement simultané ;
`N` écritures unitaires séquentielles s'étalent sur ≈ `N × 40 ms`. À 2-6 ampoules, cela fait 80 à
240 ms. Deux mitigations, dans cet ordre :

- **N'écrire que ce qui change** (§5.3) — dans la plupart des transitions, une seule ampoule change.
- **Mesurer l'étalement réel** (§8, critère chiffré). Si et seulement si la mesure le rend
  visuellement gênant sur le cas « toute la salle d'une couleur », #206 pourra ajouter **un seul
  groupe Hue « général », créé et maintenu par BuzzMaster**, utilisé pour ce cas et lui seul.
  C'est une **optimisation locale documentée, pas un changement de modèle** — la configuration
  reste par ampoule. Ne pas l'implémenter par anticipation.

---

## 3. Décision — API Hue v1

Le pilote cible l'**API v1** (`/api/<clé>/lights/<id>/state`), celle que le spike a validée.

**Pourquoi pas la v2** (`/clip/v2`, en-tête `hue-application-key`) : elle impose HTTPS avec un
certificat dont le *Common Name* est l'identifiant du bridge, signé par une racine Signify — donc
soit embarquer cette racine et forcer `ServerName`, soit épingler l'empreinte. C'est un chantier
réel, pour un gain nul sur notre usage (allumer et colorer des ampoules).

**Risque assumé** : la v1 est officiellement dépréciée. **Mitigation structurelle** : le pilote est
derrière `lighting.Driver` (#205). Une migration v2 est un **pilote de remplacement**, pas une
refonte — c'est exactement ce pour quoi l'abstraction a été conçue. Candidat pour v10.1.

Le transport doit accepter **HTTP et HTTPS auto-signé** (`InsecureSkipVerify` sur le seul chemin
Hue, jamais globalement), comme le drapeau `-https` du spike : certains firmwares récents forcent
TLS.

---

## 4. Identité du matériel — normatif

### 4.1 Le bridge

Configuration : `bridge_ip` **et** `bridge_id`. La découverte remplit les deux.

Au démarrage, si `bridge_id` est connu et que l'IP a changé (DHCP), le pilote **re-découvre** et
met à jour l'IP tout seul. L'identifiant fait foi, pas l'adresse — sinon un bail DHCP renouvelé
casse l'installation un soir de jeu.

### 4.2 Les ampoules — par **nom**, jamais par id

Les identifiants Hue sont de petits entiers **réattribuables** après suppression d'une ampoule.
Écrire sur un id mémorisé, c'est risquer d'allumer une autre ampoule que celle voulue.

Règle, reprise du garde-fou du spike et **promue en production** :

1. La configuration stocke le **nom** de l'ampoule.
2. L'id est résolu au démarrage et sur changement de configuration, par nom **exact**.
3. **0 correspondance ⇒ l'ampoule est signalée introuvable.** **> 1 correspondance ⇒ refus, jamais
   de choix arbitraire.**
4. Avant écriture, l'id doit être un **entier strictement positif**.

> Le spike relisait l'ampoule **avant chaque écriture** pour re-vérifier son nom. En production
> c'est un aller-retour de trop à chaque changement de couleur : la re-vérification a lieu **à la
> résolution** et **périodiquement** (à chaque rafraîchissement de l'inventaire), pas à chaque
> écriture. C'est un assouplissement délibéré du garde-fou du spike, justifié par le fait que le
> **bridge de production est dédié à BuzzMaster** — pas le bridge domestique de l'utilisateur.

### 4.3 Garde-fou des requêtes

`guardRequest` (`spike/hue-bridge/guard.go`) est **repris**, avec sa liste blanche élargie au
strict nécessaire de la production :

| Autorisé | Usage |
|---|---|
| `POST /api` | enregistrement (bouton) |
| `GET /api/<clé>/lights` et `/lights/<id>` | inventaire, résolution par nom |
| `PUT /api/<clé>/lights/<id>/state` | **seule** écriture |
| `GET /api/<clé>/config` | identifiant/modèle/version du bridge, pour l'écran d'état |

Tout le reste reste **refusé avant émission** : groupes, scènes, règles, planifications, capteurs,
`resourcelinks`, whitelist, firmware, `DELETE`, renommage, API v2, chemins contenant `?`, `#` ou
`..`. Le test des cas interdits (`TestGuardAllowsOnlyTheThreeOperations`, 24 cas) est **repris et
étendu**.

---

## 5. Le pilote — normatif

### 5.1 Place dans l'architecture

Implémente `lighting.Driver` (`contracts/lighting.md` §3) : `Apply(ctx, State) error` + `Close()`.

Garantie du contrat #205, sur laquelle ce pilote **s'appuie** : `Apply` n'est appelé que depuis la
goroutine unique de l'écrivain. **Le pilote n'a donc pas à être sûr en accès concurrent**, et il a
le droit de bloquer.

Package : `server-go/internal/lighting/hue/`. Il peut importer `internal/lighting` (les types),
jamais `internal/game` ni `internal/protocol`.

### 5.2 De `State` aux ampoules

`State.Zones` porte des `ZoneState{Zone, Color [3]int, Intensity int}` (§3 de `lighting.md`).

| Zone | Ampoules ciblées |
|---|---|
| `"general"` | toutes les ampoules de rôle `general`, **plus** toute ampoule d'équipe dont l'équipe n'est pas nommée dans l'état courant |
| nom d'équipe (#213) | les ampoules affectées à cette équipe |

Une zone présente dans `State` mais sans aucune ampoule configurée est un **non-événement
silencieux**, pas une erreur.

**Conversion couleur** : `Color [3]int` (RGB 0-255) → CIE xy par `rgbToXY`
(`spike/hue-bridge/hue.go:313`, repris tel quel). `Intensity` 0-255 → `bri` Hue 1-254, et
`Intensity == 0` ⇒ `{"on": false}` plutôt qu'une luminosité nulle.

**`transitiontime`** : `0` (instantané). Le bridge applique 400 ms par défaut, ce qui délaverait
un flash d'événement. Constante nommée, pas un littéral dispersé.

### 5.3 N'écrire que ce qui change — obligatoire

Le pilote garde le dernier état **effectivement appliqué** par ampoule et **n'émet une écriture
que pour les ampoules dont l'état cible diffère**.

Ce n'est pas une optimisation de confort : c'est la principale mitigation du budget de débit (§2),
et elle est simple et manifestement correcte puisque le pilote est seul à écrire.

Un échec d'écriture **invalide** l'entrée mémorisée pour cette ampoule, afin que le prochain
`Apply` la retente.

### 5.4 Débit

- `lighting.Writer.MinInterval` passe de 100 ms à **250 ms** pour ce pilote : un `Apply` vaut
  jusqu'à `N` écritures HTTP, là où #205 raisonnait sur une écriture unique.
- Écritures **séquentielles**, sans temporisation artificielle ajoutée : à ~40 ms l'unité, six
  ampoules tiennent dans la fenêtre de 250 ms.
- Le budget cible est **≤ 10 écritures/s** sur `/lights`, conformément à la recommandation Philips.

### 5.5 Comportement dégradé — normatif

Le module est **optionnel**. Le serveur doit fonctionner normalement sans bridge, contrainte actée
dès le cadrage de #182.

| Situation | Comportement |
|---|---|
| `lighting.enabled == false` ou aucune clé | **Aucune goroutine, aucun appel réseau, aucune ligne de log.** Comportement strictement identique à aujourd'hui. |
| Bridge injoignable au démarrage | Le serveur démarre normalement. Reconnexion en **retrait exponentiel plafonné** (1 s → 60 s). |
| Bridge injoignable en cours de partie | La partie continue **sans latence perceptible**. `Apply` échoue vite (timeout court) et retourne une erreur ; l'écrivain n'est jamais bloqué. |
| Clé refusée (`401`/erreur Hue `1`) | État **« refusé »**, distinct d'« injoignable ». Pas de retentative en boucle : c'est un geste utilisateur qui débloque. |
| Une ampoule injoignable (éteinte au mur) | Les autres sont écrites normalement. Jamais d'abandon global pour une ampoule absente. |
| Journalisation | Une ligne au changement d'état, **jamais une ligne par échec**. Un bridge débranché ne doit pas inonder les logs d'une soirée. |

**Timeout HTTP : 2 s.** Le bridge est sur le LAN et répond en ~40 ms ; au-delà de 2 s il est
injoignable, pas lent.

### 5.6 Taxonomie d'état — trois issues, jamais deux

Reprise de `contracts/ai-key-validation.md` §3, dont c'est le précédent direct :

| État | Déclencheurs |
|---|---|
| `ok` | bridge joignable, clé acceptée |
| `refused` | clé absente, invalide ou révoquée ; bouton non pressé pendant l'enregistrement (erreur Hue `101`) |
| `unreachable` | DNS, réseau, TLS, timeout, bridge éteint |

**Ne jamais fondre `refused` et `unreachable` en une « erreur ».** Les gestes correctifs sont
opposés : réappairer contre rallumer/rebrancher. C'est l'ambiguïté qui a rendu #142 coûteux à
diagnostiquer.

---

## 6. Configuration — normatif

Section `lighting` de **`config.json`** (config *système*), déclarée selon la procédure établie du
projet : struct + champ dans `Config`, défauts dans `ApplyDefaults`, bloc de décodage **additif par
section** dans `handleConfig` (`internal/server/http.go` ~1400-1440).

```jsonc
"lighting": {
  "enabled": false,
  "bridge_ip": "192.168.1.101",
  "bridge_id": "001788fffea0591e",
  "api_key": "…",                 // SECRET — voir §6.1
  "api_key_configured": true,     // dérivé, jamais persisté
  "clear_api_key": false,         // request-only, jamais persisté
  "lights": [
    { "name": "BuzzHue1", "role": "general" },
    { "name": "BuzzHue2", "role": "team", "team": "Rouges" }   // role "team" = #213
  ]
}
```

> **Le schéma complet est figé ici, y compris `role: "team"` et `team`.** #207 n'implémente que
> `role: "general"` ; #213 active `"team"`. C'est délibéré : casser ce schéma en #213 imposerait une
> migration de `config.json` sur une section livrée quelques jours plus tôt.

⚠️ **La section se nomme `lighting`, jamais `ambiance`** : « ambiance » désigne déjà la catégorie de
sauvegarde couvrant `game-config.json` (`BackupPage.jsx`, #152).

### 6.1 La clé API est un secret

Même régime que les clés IA, sans exception :

- **Masquée** dans `GET /config.json` (`maskedConfigJSON`, `http.go:1320`).
- Motif « absente/vide ⇒ préservée, `clear_api_key: true` ⇒ effacée, `api_key_configured` dérivé
  jamais persisté ».
- Surcharge par **`BUZZCONTROL_HUE_API_KEY`**, **sans aucune écriture disque**.
- **Jamais dans les logs**, ni en clair ni tronquée.

**Vérifié** : `config.json` vit à la racine du serveur, alors que `handleFSBackup` archive
`h.dataDir` (`data/`) et `handleGameBackup` archive `data/files`. **`config.json` n'est donc inclus
dans aucune archive de sauvegarde** — la clé ne peut pas fuir par un backup. Ce fait doit être
préservé : ne pas déplacer la section `lighting` vers `game-config.json`.

> **Pourquoi `config.json` et non un fichier séparé** comme le `.hue-username` du spike : le projet
> a déjà un régime de secret éprouvé, avec masquage, surcharge par environnement, effacement
> explicite et exclusion des sauvegardes. Un second mécanisme n'apporterait rien et créerait un
> deuxième endroit où se tromper.

---

## 7. Endpoints HTTP — normatif

Enregistrés dans `setupRoutes` (`internal/server/http.go`). Toutes les réponses d'erreur portent la
taxonomie du §5.6.

### `POST /api/lighting/discover`
Découverte mDNS puis SSDP. **Aucun appel cloud.** Timeout 5 s.
Réponse : `200 {"bridges":[{"ip":"…","id":"…","model":"BSB002"}]}` — liste possiblement vide.

### `POST /api/lighting/register`
`POST /api` vers le bridge avec `{"devicetype":"buzzmaster#<hôte>"}`.
Corps : `{"bridge_ip":"…"}`.
- `200 {"result":"ok"}` — clé obtenue et **enregistrée côté serveur**, jamais renvoyée au client.
- `409 {"result":"refused","reason":"link_button_not_pressed"}` — **cas nominal**, pas une panne :
  l'utilisateur n'a pas encore appuyé. Le client relance.
- `503 {"result":"unreachable"}`.

### `GET /api/lighting/lights`
Inventaire : `200 {"lights":[{"id":"8","name":"BuzzHue1","reachable":true,"on":false}]}`.
Sert la sélection dans l'écran d'administration.

### `POST /api/lighting/test`
Corps : `{"name":"BuzzHue1"}` ou `{}` pour toutes les ampoules sélectionnées.
Effet : un flash bref puis **retour à l'état antérieur**. Réponse `200 {"result":"ok"}`.

### `GET /api/lighting/status`
`200 {"state":"ok|refused|unreachable|disabled","bridge_id":"…","bridge_ip":"…","lights_ok":2,"lights_total":3}`.
**Ne fait aucun appel bloquant** : renvoie l'état connu du pilote, jamais une interrogation du pont.

#### 7.1 Amendement du 2026-09-04 — l'indicateur du menu

Décision utilisateur : l'entrée « Ambiance » du menu abeille porte une **ampoule**, dont la
**forme** dit l'état et dont la couleur ne fait que renforcer. Les quatre états de cet endpoint s'y
projettent ainsi — correspondance **normative**, pour que le frontend n'ait pas à l'inventer :

| `state` | Glyphe | Couleur | Sens pour l'utilisateur |
|---|---|---|---|
| `ok` | ampoule **pleine, avec rayons** | verte | pont configuré et qui répond |
| `unreachable` | ampoule **en contour + pastille d'alerte** | orange | pont configuré mais qui ne répond plus |
| `refused` | ampoule **en contour + pastille d'alerte** | orange | pont configuré mais qui refuse la clé — même geste : aller voir la page |
| `disabled` | ampoule **en contour nu** | grise | aucun pont configuré |

**Trois glyphes distincts, pas une même forme recolorée** (révision 4 de la maquette). La
distinction reste lisible en niveaux de gris et pour un daltonien : rayons → contour + pastille →
contour nu. Trois teintes sur une forme unique auraient été indiscernables, et à peine lisibles à
15 pixels.

Un emoji ne permet ni de changer la couleur ni de changer la forme : les trois glyphes sont des
**SVG en ligne** tracés en `currentColor`. Tracés de référence dans
`docs/mockups/lighting-hue-config-207.html` §01.

`refused` et `unreachable` partagent la couleur **orange** : dans les deux cas le pont est
configuré et ne fonctionne pas, et la conduite à tenir est la même — ouvrir la page Ambiance. La
distinction reste **entière dans l'API et sur la page**, où elle commande deux gestes correctifs
opposés (§5.6) ; elle est simplement inutile à trois pixels dans un menu.

**Le contour nu ne réclame aucune attention.** Une fonctionnalité facultative non configurée ne
doit jamais ressembler à une alerte — d'où l'absence de pastille sur ce seul glyphe, qui est
précisément ce qui le distingue de l'état « injoignable ».

**Accessibilité** : l'entrée expose un `title` disant l'état en toutes lettres, et le SVG est
`aria-hidden` (le libellé « Ambiance » porte le sens). La forme étant devenue le porteur principal,
ce `title` n'est plus l'unique ligne de défense — mais il reste requis : un lecteur d'écran ne voit
aucune forme.

**Rafraîchissement** : le frontend interroge cet endpoint **au montage puis toutes les 30 s**, et
immédiatement après un enregistrement de configuration. Le précédent du projet (`useUpdates`,
`web/src/hooks/useUpdates.js`, appelé une fois au montage par `Navbar.jsx:87-89`) ne suffit pas
ici : un pont peut devenir injoignable **pendant** une session, alors qu'une mise à jour
disponible, elle, ne se volatilise pas. L'intervalle est acceptable précisément parce que cet
endpoint **ne fait aucune I/O** — il lit un état déjà en mémoire.

> Une diffusion WebSocket serait plus élégante, mais introduirait un mécanisme nouveau pour un
> indicateur cosmétique. Candidat d'amélioration, pas un prérequis.

---

## 8. Critères chiffrés attendus de #206

À produire, avec la même méthode que le spike (sortie JSON, p50/p95) :

| Mesure | Attendu |
|---|---|
| Latence d'une écriture | p95 ≤ **150 ms** (le spike a mesuré 48-59 ms) |
| **Étalement** entre la première et la dernière ampoule d'un `Apply` à N ampoules | mesuré et **publié** pour N = 2, 4, 6 |
| Rafale simulant des validations RAFALE successives | **100 %** de succès, et **≤ 10 écritures/s** |
| Bridge débranché en cours de partie | aucune latence perceptible sur les transitions, **une seule** ligne de log |

---

## 9. Ce qui appartient à #213

- Activation du `role: "team"` et du champ `team` du §6.
- Résolution de `Event.Teams` (`lighting.md` §2.2) en zones d'état.
- Règles de dégradation : moins d'ampoules que d'équipes, équipe sans ampoule, aucune affectation
  (⇒ retour au comportement « toute la salle en `general` »), ampoule affectée mais injoignable.
- Colonne « équipe » dans l'écran d'administration.

## 10. Hors de ce contrat

| Sujet | Où |
|---|---|
| Vocabulaire d'événements, écrivain, recensement des sites | `contracts/lighting.md` (#205, livré) |
| Conduite manuelle depuis `/anim`, restitution d'état à l'arrêt | #208 |
| Édition des scènes, effets répartis, synchronisation minuteur | v10.1 (#210, #211, #212) |
| Migration vers l'API Hue v2 | candidat v10.1 (§3) |
