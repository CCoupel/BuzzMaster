# Contrat — Effets lumière déclaratifs : groupes d'ambiance, fichiers d'effets, séquenceur (v11.7.0)

> Issues : #225 (groupes), #223 (séquenceur, chenillard), #299 (catalogue), #300, #301, #302, #211 (effets répartis, dégradés).
> Décisions utilisateur du 2026-11-11 (GATE 1.5) : D1 les effets ciblent des **groupes**, jamais une ampoule isolée ; D2 l'effet
> est choisi **par moment**, en global et par manche ; D3 les « rôles fins » de #211 sont hors v11.7.0 ; D4 le catalogue offre
> un aperçu simulé **et** un essai sur les ampoules.
> Contrainte d'architecture (non négociable) : un effet est un **fichier descriptif éditable à la main**, interprété par un
> séquenceur commun. L'éditeur de scènes v11.8.0 (#210) écrira ce même format.
> Contrats liés : `lighting.md` (écrivain, scènes), `hue-bridge.md` (pilote), `ambiance-settings.md` (réglages).

---

## 1. Vocabulaire

| Terme | Définition |
|---|---|
| **Groupe d'ambiance** | Sous-ensemble **ordonné et numéroté** (1…N) des ampoules de rôle `general`. Unité que pilote un effet. |
| **Groupe animateur** | Groupe d'ambiance portant le rôle `animateur` (**plusieurs possibles**, §2.5, révision du GATE LOT-1) : numéroté à part (A1, A2…), jamais entraîné par les effets de salle, blanc fixe par défaut, avec ses propres effets par moment. |
| **Effet** | Description déclarative de ce que montre chaque groupe au fil du temps. Un fichier = un effet. |
| **Moment** | Situation de jeu pour laquelle on choisit un effet : `chrono`, `calm`, `score` (§6). |
| **Séquenceur** | Interprète unique des effets : calcule une **trame** (un état par groupe) à chaque pas. |
| **Trame** | `[]GroupLook` : pour chaque groupe, couleur RGB, intensité 0-255, fondu en ms. |

Les ampoules de rôle `team` ne font partie d'aucun groupe : elles montrent toujours la couleur de leur équipe
(`hue-bridge.md` §5.2, inchangé). Un effet n'écrit **jamais** une ampoule d'équipe.

---

## 2. Groupes d'ambiance (#225)

### 2.1 Configuration — `config.json`, section `lighting`

```jsonc
"lighting": {
  "lights": [ { "name": "BuzzHue1", "role": "general" }, … ],   // inchangé
  "groups": [                                                    // NOUVEAU — ordre du tableau = numéro (1-based)
    { "id": "g3f9a1", "name": "Scène gauche", "role": "salle", "lights": ["BuzzHue1", "BuzzHue4"], "near_teams": ["Rouges"] },
    { "id": "g81c02", "name": "Bar",          "role": "salle", "lights": ["BuzzHue2"],             "near_teams": [] },
    { "id": "g5d7e4", "name": "Pupitre",      "role": "animateur", "lights": ["BuzzHue8"],         "near_teams": [] },
    { "id": "g0b6c9", "name": "Régie",        "role": "animateur", "lights": ["BuzzHue9"],         "near_teams": [] }
  ]
}
```

- `role` : `"salle"` (défaut, valeur absente ou vide) ou `"animateur"` (§2.5). **Plusieurs groupes `animateur` sont
  permis** *(révision du GATE LOT-1 : la limite « un seul » est levée)*.
  Le **numéro** d'un groupe de salle est son rang parmi **tous** les groupes de rôle `salle`, vides compris. Un groupe
  animateur a son **propre rang** parmi les groupes de rôle `animateur` (A1, A2…, vides compris) et ne compte jamais dans
  N. `near_teams` est refusé sur un groupe animateur.
- `id` *(révision du GATE LOT-1)* : identifiant **stable** du groupe (`[a-z0-9]{1,12}`), unique. Attribué par le serveur à
  l'enregistrement quand il est absent ou vide ; le client le renvoie tel quel ensuite (renommer ou déplacer un groupe ne
  change pas son `id`). Il sert de clé aux effets des groupes animateur (§6). Un `id` en double ou mal formé ⇒ `400`.
- Plafonds : **12 groupes de salle** et **4 groupes animateur**.
- `name` : libellé libre (1 à 40 caractères), unique sans tenir compte de la casse. Sert à l'affichage ; les effets
  n'utilisent que le **numéro**.
- `lights` : noms d'ampoules, chacune de rôle `general` et présente dans **un seul** groupe. Un groupe vide est accepté :
  il **garde son numéro** (numéros stables pour l'interface et pour `GET /api/lighting/lights`) mais est **ignoré du
  séquenceur**. **N = nombre de groupes de salle non vides.** Le séquenceur calcule sa trame sur la suite ordonnée des
  groupes non vides (positions 1…N, sans renuméroter la configuration) et émet chaque état avec le **numéro réel** du
  groupe (`ZoneState.Group`). Exemple : groupes 1, 2 (vide), 3 ⇒ N = 2, la trame a deux positions, écrites sur
  `Group 1` et `Group 3`. *(Amendement du 2026-11-11, revue LOT-1, écart 2.)*
- `near_teams` : équipes installées près de ce groupe (optionnel). Sert d'origine à un effet `origin: team` (§4.5).
  Une équipe figure dans un seul groupe au plus.
- Absent ou `[]` ⇒ aucun groupe : comportement v11.6.0 exact.
- Écriture : `POST /config.json {"lighting": {"groups": [...]}}`, remplacement complet du tableau. Refus `400` avec un
  message lisible : ampoule inconnue de `lights`, ampoule de rôle `team`, ampoule dans deux groupes, nom en double,
  plus de 12 groupes de salle, plus de 4 groupes animateur.
- Changer le rôle d'une ampoule en `team`, ou la retirer de `lights`, la retire de son groupe dans la même écriture.
- **Ampoule générale hors groupe** : elle garde la scène de salle ordinaire et ne participe pas aux effets répartis.
  Elle participe aux effets `target: all`.

### 2.2 Modèle d'état — `lighting.ZoneState.Group` (amendement de `lighting.md` §3)

`ZoneState` gagne un champ `Group int`. `0` (valeur zéro) = toute la zone, comportement actuel. `n ≥ 1` avec
`Zone == "general"` = le groupe d'ambiance n. Une entrée `{Zone:"general", Group:0}` reste toujours émise en premier
(elle sert les ampoules hors groupe et le repli) ; les entrées `Group ≥ 1` la **précisent** pour leurs ampoules.
Un groupe animateur est adressé par un numéro **négatif** : `lighting.HostGroup(k) = -k` pour le groupe animateur de rang
k (A1 ⇒ `-1`, A2 ⇒ `-2`…) : `{Zone:"general", Group:-k}`. *(Révision du GATE LOT-1 : remplace la constante unique
`GroupHost = -1`, qui reste la valeur de A1.)*

### 2.3 Pilote Hue (amendement de `hue-bridge.md` §5.2, §5.3, §5.8)

- `LightSpec` gagne `Group int` (0 = hors groupe). La clé de regroupement d'`Apply` devient (rôle, équipe, groupe).
- Résolution de l'état cible d'une ampoule générale du groupe n : l'entrée `Group == n` si elle existe, sinon l'entrée
  `Group == 0`.
- **`buzzmaster-general` exclut les ampoules du groupe animateur** (§2.5 : jamais entraîné par la salle). Ses membres
  sont les ampoules de rôle `general` hors groupe animateur ; « toutes les ampoules générales » ci-dessous s'entend de
  même. *(Amendement du 2026-11-11, revue LOT-1, écart 4.)*
- **Quand toutes les ampoules générales ont le même état cible**, le pilote écrit `buzzmaster-general` comme aujourd'hui
  (une écriture). C'est le cas de toute scène sans effet réparti : **aucune régression de débit**.
- Groupes Hue : un `LightGroup` `buzzmaster-general-<n>` par groupe d'ambiance de **3 ampoules ou plus**. En dessous,
  écriture par ampoule. Le seuil vaut `hue.SubGroupMinLights = 3` ; il est à confirmer par la mesure du §8.2 et
  `dev-backend` peut l'amender en documentant le chiffre mesuré.
- Réconciliation, repli par ampoule, invalidation du cache par membre : règles du §5.8, inchangées.
- Extinction à l'arrêt, `ON`/`OFF`/Flash : agissent sur la zone `general` entière (`Group 0`) ; le séquenceur est
  inactif dans ces cas (§5.3), donc aucune entrée `Group ≥ 1` n'est émise.

### 2.5 Groupes animateur (amendement du GATE 2, révisé au GATE LOT-1 — 2026-11-11)

Un groupe de rôle `animateur` éclaire l'animateur (pupitre, régie). Il remplace, pour v11.7.0, les « rôles fins » de #211.

> **Révision du GATE LOT-1 — plusieurs groupes animateur.** On crée des groupes animateur comme des groupes de salle
> (4 au plus), chacun avec **ses propres effets par moment**. Tout ce qui suit s'applique **à chaque groupe animateur,
> indépendamment des autres**. Rang k (A1, A2…) = rang parmi les groupes `animateur` ; adresse d'état `Group = -k` ;
> groupe Hue `buzzmaster-general-host<k>` ; forme API `"host<k>"` (`"host1"`, `"host2"`…) ; clé des effets = `id` du
> groupe (§6). Un groupe animateur vide garde son rang et est ignoré du lecteur.

- **Jamais entraîné par la salle** : aucune scène de salle, aucun effet de salle, ni le mode `ON` / `OFF`, ni le Flash, ni
  l'essai d'un effet (§7.4) ne modifient ses ampoules. Seuls l'interrupteur général `lighting.enabled` et l'extinction à
  l'arrêt du serveur l'éteignent.
- **Comportement par défaut — livré au LOT-1** : **blanc fixe** (`#FFFFFF`, intensité 255), à tout moment, dès que
  l'éclairage est actif. `Scene()` émet `{Zone:"general", Group:GroupHost}` avec cet état si le groupe existe et n'est pas
  vide.
- **Effet par moment — livré au LOT-2** : réglage `LightHostEffects` (§6) — `light.host_effects.{chrono,calm,score}` dans
  le modèle de réglages (manche, API, état de jeu), `lighting.host_effects.{chrono,calm,score}` dans `config.json` pour la
  valeur globale ; global et par manche.
  `""` = blanc fixe. L'effet est rendu par le même séquenceur, **sur ce seul groupe** (N = 1) : un effet dont
  `target` vaut `groups` et `min_groups` > 1 est inapplicable au groupe animateur et reste grisé dans ses sélecteurs ; un
  effet `target: all` lui est toujours applicable (§4.6). La couleur `scene` vaut blanc
  pour ce groupe. Effet absent, invalide ou inapplicable ⇒ blanc fixe.
- **Pilote** : `LightSpec.Group = -k` ; groupe Hue `buzzmaster-general-host<k>` à partir de `SubGroupMinLights`
  ampoules, écriture par ampoule en dessous. Une ampoule d'un groupe animateur ne reçoit jamais l'entrée `Group 0`.
  L'ancien groupe Hue `buzzmaster-general-host` est supprimé par la réconciliation (préfixe `buzzmaster-`).
- **Essai** : `POST /api/lighting/groups/test {"group": "host1"}` (§2.4).

### 2.4 Essai d'un groupe — `POST /api/lighting/groups/test`

> **Révision du GATE LOT-1 (2026-11-11)** — le clignotement blanc était trop discret : l'essai devient une **séquence
> visible**. Ce paragraphe remplace la version précédente.

`{"group": 2}` (groupe de salle) ou `{"group": "host1"}` (groupe animateur A1) joue sur les ampoules du groupe, **une
fois**, la séquence :

| Pas | État | Durée |
|---|---|---|
| 1 | éteint | 1000 ms |
| 2 | blanc, 50 % | 1000 ms |
| 3 | blanc, 100 % | 1000 ms |
| 4 | rouge `#FF0000`, 100 % | 1000 ms |
| 5 | vert `#00FF00`, 100 % | 1000 ms |
| 6 | bleu `#0000FF`, 100 % | 1000 ms |
| fin | **retour à l'état vivant** du groupe (scène, effet ou blanc fixe du moment) | — |

- **Durée d'un pas : 1000 ms** (`groupTestStepMs`, 6 s au total), fondu 0. Chaque pas coûte une écriture de groupe Hue
  (groupe de 3 ampoules ou plus) ou une écriture par ampoule (1 ou 2 ampoules) : au plus 1 écriture de groupe par seconde
  et 2 écritures d'ampoule par seconde, dans le budget du pont (§5.4, `hue-bridge.md` §5.4).
- **Mécanisme imposé** : la séquence est un état d'essai porté par `ambiance.Controller` (goroutine sur `Host.Ctx()`,
  durée de pas injectable pour les tests) et lu par `Scene()`, qui émet `{Zone:"general", Group:n ou -k}` avec l'état du
  pas ; `NotifyState()` à chaque pas. **Aucune écriture directe au pilote** : pas de cache à invalider, et le retour à
  l'état vivant est une simple re-dérivation (`NotifyState()` à la fin). L'essai prime sur l'effet du groupe, sur `ON` /
  `OFF` et sur le Flash **pour ce seul groupe**, le temps de la séquence.
- **Réponse immédiate** (la requête n'attend pas la fin) : `200 {"ok": true, "duration_ms": 6000}`.
- **Un seul essai à la fois** : un nouvel appel (même groupe ou autre) **remplace** l'essai en cours, dont le groupe
  revient aussitôt à son état vivant. **`429` n'est plus renvoyé** par cette route.
- **Arrêt** : `POST /api/lighting/groups/test/stop` (sans corps) arrête l'essai en cours et rend le groupe à son état
  vivant ; `200 {"ok": true}` même si aucun essai ne tourne.
- Erreurs, vérifiées avant de démarrer, formes de `/api/lighting/test` : `400` groupe inconnu ou vide ; `409` éclairage
  désactivé (`lighting.enabled == false`) ou clé refusée ; `503` pont injoignable (état connu du pilote, sans E/S).
- L'arrêt du serveur et la désactivation de l'éclairage annulent l'essai (`quiesceAmbiance`).

**Forme de `group`** (requête ici, et champ `group` de chaque ampoule dans `GET /api/lighting/lights`) : `0` = hors
groupe (réponse seulement), entier `n ≥ 1` = groupe de salle numéro n, chaîne `"host<k>"` (`"host1"`, `"host2"`…) =
groupe animateur de rang k. Une ampoule de rôle `team` porte `0`. *(Revue LOT-1, écart 1 ; `"host"` devient `"host<k>"`
au GATE LOT-1.)*

---

## 3. Fichiers d'effets

### 3.1 Format : YAML

Un fichier `.yaml` par effet, encodé en UTF-8. Choix motivé :

- **Commentaires** (`#`) et texte sur plusieurs lignes, sans guillemets ni virgules : lisible et annotable dans un
  éditeur de texte ordinaire.
- **Erreurs localisées** (ligne, colonne) fournies par l'analyseur.
- **Aucun nouveau module** : `go.yaml.in/yaml/v4` est déjà dans `go.mod` (dépendance indirecte), il devient direct. Il
  figure dans la liste de `TestCA7_NoNewThirdPartyDependency`.
- Décodage **strict** (`KnownFields(true)`) : une clé inconnue est une erreur, jamais un réglage ignoré en silence.

Alternative écartée : JSON avec commentaires (aucune dépendance, mais virgules, guillemets et accolades à tenir à la
main, et un analyseur de commentaires à écrire et maintenir). Si la version candidate du module YAML posait un problème
bloquant, c'est le repli prévu ; le schéma du §4 ne change pas.

### 3.2 Emplacements

| Emplacement | Contenu | Écrit par |
|---|---|---|
| binaire (`assets/effects/*.yaml`, `go:embed`) | effets **natifs** | la version du serveur |
| `data/files/effects/natifs/` | **copie de référence** des natifs, réécrite à chaque démarrage, avec un en-tête « ne pas modifier ici : copier dans le dossier parent » | le serveur |
| `data/files/effects/*.yaml` | effets **de l'utilisateur** | l'utilisateur (puis l'éditeur v11.8.0) |

- Le serveur lit les natifs **dans le binaire**, jamais dans `natifs/` : modifier ou supprimer la copie de référence est
  sans effet, et une mise à jour du serveur met à jour les natifs.
- Un fichier utilisateur dont l'`id` est celui d'un natif le **surcharge** ; tout autre `id` **s'ajoute** au catalogue.
- **Restaurer un natif** : supprimer (ou renommer hors `.yaml`) le fichier utilisateur de même `id`.
- L'`id` est porté par le fichier (clé `id`), le nom de fichier est libre. Deux fichiers utilisateur de même `id` : le
  premier par ordre alphabétique de nom est retenu, l'autre est signalé en erreur.
- Sous-dossiers ignorés (hors `natifs/`, jamais lu). Limites : 64 Ko par fichier, 100 fichiers.
- Sauvegarde : `data/files/effects/` (sans `natifs/`) est inclus dans la catégorie `ambiance` de `/backup-select`, dans
  `/game-backup`, et dans la liste d'autorisation de `/restore`.

### 3.3 Chargement

- Au démarrage, puis **à chaud** : le serveur compare toutes les 2 s la signature du dossier (noms, tailles, dates) et
  recharge en cas de changement. `POST /api/lighting/effects/reload` force un rechargement immédiat.
- Le rechargement remplace le catalogue de façon atomique. Un effet en cours de lecture repart du début avec sa nouvelle
  définition.
- **Un fichier invalide ne casse rien** : il est écarté et signalé (§7.1 `errors[]`, une ligne de journal par changement
  d'état du fichier, jamais une ligne par cycle). La version précédente de cet `id` reste active si elle existe en
  mémoire ; sinon le natif de même `id` ; sinon l'effet est absent.
- Un moment réglé sur un effet absent, invalide ou inapplicable (§4.6) joue la **valeur par défaut du moment** (§6 :
  `pulsation` pour `chrono`, aucun effet pour `calm` et `score`) et l'écart est visible dans
  `GET /api/lighting/effects` (`unresolved[]`).

---

## 4. Schéma d'un effet — normatif

```yaml
# Chenillard — un point lumineux fait le tour de la salle, une position par seconde.
id: chenillard            # [a-z0-9-], 1 à 40 caractères, unique
name: Chenillard          # libellé du catalogue (1 à 40 caractères)
description: >            # texte du catalogue (500 caractères au plus)
  Un point lumineux fait le tour de la salle pendant le chronomètre.
  Il passe à l'orange puis au rouge quand le temps presse.
version: 1                # version du schéma ; seule la valeur 1 est acceptée
moments: [chrono]         # moments où l'effet est proposé : chrono, calm, score
target: groups            # groups = un état par groupe ; all = un seul état pour toute la zone générale
min_groups: 3             # target: groups seulement — en dessous, l'effet est inapplicable (§4.6) ; défaut 1
duration_ms: 0            # 0 = dure autant que le moment ; > 0 = s'arrête seul (obligatoire pour score)

looks:                    # apparences nommées, utilisées par les motifs
  fond:    { color: theme,   intensity: 100 }
  trainee: { color: theme,   intensity: 50 }
  tete:    { color: urgency, intensity: 100 }

pattern:                  # un seul motif par étape
  type: shift
  step_ms: 1000
  cells: [trainee, tete, trainee]
  rest: fond
  wrap: true

stages:                   # variantes selon une condition ; la première qui s'applique remplace les clés données
  - when: { groups_lt: 5 }
    pattern: { cells: [tete] }
```

### 4.1 Clés de premier niveau

`id`, `name`, `description`, `version`, `moments`, `target`, `min_groups`, `duration_ms`, `looks`, `pattern`, `stages`.
Obligatoires : `id`, `name`, `version`, `moments`, `looks`, `pattern`. Toute autre clé est refusée.

### 4.2 Apparence (`looks.<nom>`)

| Clé | Valeurs | Défaut |
|---|---|---|
| `color` | `scene` (couleur de la scène de salle du moment), `theme` (thème de la question, blanc à défaut), `team` (équipe créditée, moment `score`), `urgency` (blanc, orange `#FFAA00` à 10 s ou moins, rouge `#E61E1E` à 5 s ou moins — couleurs de `lighting.md` §8.2), ou `"#RRGGBB"` | `scene` |
| `intensity` | 0 à 100 (pourcentage ; 0 = éteint) | 100 |
| `fade_ms` | 0 à 5000 : fondu confié au pont (`ZoneState.TransitionMs`) | 0 |

`fade_ms > 0` entre deux teintes éloignées peut délaver la couleur (interpolation du pont, `lighting.md` §8.2) : la
documentation utilisateur le signale ; ce n'est pas une erreur de validation.

### 4.3 Motifs (`pattern.type`) — le vocabulaire du séquenceur

Un effet **compose** ces motifs avec ses apparences et ses paramètres. Ajouter un motif demande une évolution du
serveur ; tout le reste se fait dans le fichier.

| Type | Paramètres | Rendu |
|---|---|---|
| `uniform` | `phases: [{look, ms}]` (1 à 8) | Tous les groupes (ou toute la zone si `target: all`) montrent la même apparence, qui alterne selon `phases`. Calé sur l'horloge murale (`maintenant mod cycle`), comme la pulsation actuelle. |
| `shift` | `cells: [look…]`, `rest: look`, `step_ms`, `wrap` (défaut `true`), `direction` (`forward` \| `backward` \| `bounce`, défaut `forward`) | Le bloc `cells` se déplace d'un groupe par pas ; les autres groupes montrent `rest`. |
| `gauge` | `source` (`remaining` \| `elapsed`), `on: look`, `off: look`, `edge: look` (optionnel), `direction` | Les groupes allumés sont au prorata de la source (part du chronomètre restant ou écoulé) ; `edge` est le groupe frontière. Recalcul à chaque seconde du chronomètre. |
| `random` | `step_ms`, `count` (nombre de groupes en `on`, ou `ratio` 0-1), `on: look`, `off: look` | À chaque pas, un tirage de groupes passe en `on`, les autres en `off`. Générateur à graine : déterministe pour une graine donnée. |
| `gradient` | `stops: [look…]` (2 à 6), `step_ms`, `travel` (`none` \| `forward` \| `backward` \| `outward`), `origin` (`first` \| `last` \| `center` \| `team`), `repeat` (défaut `true`), `rest: look` (optionnel) | Dégradé réparti sur les groupes, fixe ou en mouvement. **Sémantique exacte au §4.3.1.** |

#### 4.3.1 Motif `gradient` — sémantique exacte *(amendement du 2026-11-11, LOT-2 Batch 2.1)*

Notations : `stops` = S₀…S₍k−1₎ ; N groupes rendus, positions p = 0…N−1 dans l'ordre de parcours ; s = numéro du pas
(0 au démarrage de l'effet, +1 tous les `step_ms`). « Interpoler » = interpolation linéaire RGB **et** intensité entre
deux arrêts consécutifs ; `fade_ms` d'une position = celui de l'arrêt le plus proche.

**1. `travel: none` — dégradé fixe, du premier au dernier arrêt.** Le groupe 1 montre S₀, le dernier groupe montre
S₍k−1₎, les groupes intermédiaires sont interpolés : abscisse x = p / (N−1) × (k−1). N = 1 ⇒ S₀. **Jamais de retour vers
S₀** : `repeat` n'a aucun effet avec `travel: none`. `step_ms` et `origin` sont sans effet.

**2. `travel: forward` / `backward`, `repeat: true` (défaut) — anneau qui tourne.** Le dégradé est **fermé** : après
S₍k−1₎ il revient vers S₀, réparti sur un tour complet des N groupes ; il avance d'un groupe par pas, sans couture.
x = ((p − s) mod N) / N × k pour `forward` (p + s pour `backward`) ; interpolation entre S₍⌊x⌋₎ et S₍(⌊x⌋+1) mod k₎.

**3. Propagation — `travel: outward`, ou `forward` / `backward` avec `repeat: false`.** Le dégradé se **propage dans
le temps** à partir d'un groupe de départ ; ce n'est **pas** une atténuation selon la distance.
- Départ : `outward` ⇒ le groupe `origin` (`first`, `last`, `center` = position ⌊(N−1)/2⌋, `team` = §4.5), propagation
  vers les deux bords ; `forward` ⇒ premier groupe ; `backward` ⇒ dernier groupe.
- d = distance en groupes entre p et le départ. Le front atteint la distance d au pas s = d.
- **Groupe non encore atteint** (d > s) : apparence `rest` ; si `rest` est absent, aucun état n'est émis pour ce groupe
  (il garde la scène de salle, entrée `Group 0`).
- **Groupe atteint** (d ≤ s), âge a = s − d : il montre S₍a₎ — il parcourt les arrêts **un par pas**, dans l'ordre, sans
  interpolation (le fondu éventuel est celui de `fade_ms`).
  - `repeat: false` : au-delà du dernier arrêt, il **reste sur S₍k−1₎** (a plafonné à k−1). La propagation est terminée
    au pas (distance maximale) + k − 1 ; l'état est ensuite tenu jusqu'à la fin de l'effet (`duration_ms` ou fin du moment).
  - `repeat: true` : il reboucle (S₍a mod k₎) : ondes successives qui partent du même point.

Exemple (`vague`, 5 groupes, départ au groupe 2, `stops: [crete, equipe]`, `repeat: false`, `rest: fond`) :

```
pas 0 :  fond   CRETE  fond   fond   fond
pas 1 :  CRETE  equipe CRETE  fond   fond
pas 2 :  equipe equipe equipe CRETE  fond
pas 3 :  equipe equipe equipe equipe CRETE
pas 4 :  equipe equipe equipe equipe equipe   (tenu)
```

Validation : `rest` doit nommer une apparence de `looks` ; `rest` et `origin` ne sont acceptés que pour une propagation
(point 3), `origin` seulement avec `outward` ; sinon erreur de validation.

Contraintes communes : `step_ms` ≥ 250 (cadence de l'écrivain, `hue-bridge.md` §5.4) et ≤ 60 000 ; chaque `ms` de `phases`
≥ 250. Une apparence citée doit exister dans `looks`.

### 4.4 Étapes (`stages`)

Liste ordonnée d'au plus 6 entrées `{ when, looks?, pattern? }`. À chaque pas, la **première** entrée dont toutes les
conditions sont vraies s'applique : ses `looks` remplacent les apparences de même nom, les clés de son `pattern`
remplacent celles du motif de base (`type` compris, s'il est donné). Aucune entrée vraie ⇒ définition de base.

Conditions (`when`) : `remaining_lte: <secondes>`, `remaining_gt: <secondes>`, `groups_lt: <n>`, `groups_gte: <n>`.
`remaining_*` n'a de sens que pour le moment `chrono` ; ailleurs la condition est fausse.

### 4.5 Contexte fourni par le serveur

| Variable | Source |
|---|---|
| nombre de groupes N | groupes de **salle non vides** de `lighting.groups` (§2.1 ; chaque groupe animateur est rendu à part, N = 1) |
| temps restant, durée totale | `GameState.CurrentTime`, `GameState.Delay` (moment `chrono`) |
| couleurs `scene`, `theme`, `team` | mêmes fonctions que la table de scènes (`ambiance.Scene`, `ThemeColor`, `TeamNameColor`) |
| groupe d'origine `team` | groupe dont `near_teams` contient l'équipe créditée ; à défaut le groupe 1 |
| graine | horodatage de démarrage de l'effet |

### 4.6 Applicabilité et repli

L'applicabilité dépend de `target` *(amendement du 2026-11-11, LOT-2)* :

| `target` | Salle (N = groupes de salle non vides) | Groupe animateur |
|---|---|---|
| `all` | **Toujours applicable, même avec N = 0.** L'effet produit un seul état, écrit sur la zone générale entière (`ZoneState{Zone:"general", Group:0}`) : il ne dépend pas des groupes. | Toujours applicable (un seul état, écrit sur `GroupHost`). |
| `groups` | Applicable si N ≥ `min_groups` (défaut 1, donc jamais avec N = 0). | Applicable si `min_groups` ≤ 1 (rendu avec N = 1). |

- **`min_groups` ne concerne que `target: groups`.** Avec `target: all`, la clé doit être absente ou valoir 1 ; une valeur
  supérieure est une erreur de validation (« min_groups n'a pas de sens avec target: all »), jamais une clé ignorée.
- **`target: all` n'accepte que le motif `uniform`** (les autres motifs répartissent des états entre groupes) ; les
  conditions `groups_lt` / `groups_gte` y sont refusées. Erreur de validation sinon.
- Non-régression v11.6.0 : `pulsation` (`target: all`) reste jouée sans aucun groupe configuré.

Un effet inapplicable fait jouer au moment sa valeur par défaut (§3.3). Le catalogue et le sélecteur l'indiquent
(« demande 3 groupes, 2 configurés »).

### 4.7 Erreurs de validation

Chaque erreur porte `file`, `line` (0 si inconnue), `message` en français, sans jargon :
`"chenillard.yaml, ligne 14 : l'apparence « tete » n'est pas définie dans looks"`. Liste minimale : YAML illisible, clé
inconnue, clé obligatoire absente, `id` invalide ou en double, `version` non prise en charge, `moments` vide ou valeur
inconnue, `type` de motif inconnu, paramètre hors bornes, apparence non définie, couleur invalide, `duration_ms` absent
pour un effet `score`, fichier trop gros.

---

## 5. Séquenceur

### 5.1 Découpage imposé

- **`internal/effects`** (nouveau, pur, sans goroutine ni E/S réseau) : types, `Parse([]byte) (Effect, []Error)`,
  `LoadDir(embedded fs.FS, dir string) Catalogue`, et la fonction de rendu
  `Render(e Effect, ctx Context, elapsed time.Duration) Frame`. `Render` est **déterministe** : mêmes entrées, même trame.
  N'importe ni `internal/game` ni `internal/lighting/hue`.
- **`internal/ambiance`** : `Controller` gagne le lecteur (`runEffects`), une goroutine unique sur `Host.Ctx()`, sur le
  modèle de `RunChronoPulse` : à chaque tic (100 ms, jamais une cadence d'écriture) elle détermine le moment vivant et
  l'effet résolu, calcule la trame, et si elle diffère de la précédente la publie (`atomic.Pointer[Frame]`) puis appelle
  `Writer().NotifyState()`. `Scene()` lit la trame publiée et émet les `ZoneState{Zone:"general", Group:n}`.
- Horloge et tic injectables par `Controller` (jamais au niveau du package), pour des tests sans attente réelle
  (#297, #298).

### 5.2 Moment vivant

Déterminé à partir de `DeriveEvent()` (jamais d'un état mémorisé à part) :

| Moment | Condition |
|---|---|
| `chrono` | `KindRunning` et `GameState.Delay > 0` (condition actuelle de la pulsation) |
| `calm` | `KindIdle`, `KindPauseAll` ou `KindEntracte` |
| `score` | pendant `duration_ms` après un `NotifyPulse(KindScore)` (les 6 sites existants), borné à la durée de l'impulsion SCORE |
| aucun | tout autre cas : pas de trame, scène ordinaire |

`score` prime sur `chrono` et `calm` pendant sa durée.

### 5.3 Précédence — normatif

1. `lighting.enabled == false`, pilote absent ⇒ le lecteur ne publie rien.
2. Mode `ON` / `OFF` ou Flash engagé ⇒ **aucune trame** : `OverrideGeneral` s'applique comme aujourd'hui. Même test que
   la pulsation (`ambiance.go`, `Scene`) : `lightingMode() == ModeAuto && !isLightingFlashOn()`.
3. Essai d'un effet (§7.3) ⇒ prime sur le moment vivant, jamais sur le point 2.
4. Sinon, trame de l'effet résolu du moment vivant.

L'arrêt du serveur (`quiesceAmbiance`) annule le lecteur avant l'extinction.

**Groupes animateur** : le lecteur calcule **une trame indépendante par groupe animateur non vide** (réglage
`light.host_effects.<id>`, même moment vivant, N = 1, écrite sur `Group = -k`). Seul le point 1 s'y applique : les points 2 et 3 ne le concernent pas
(§2.5).

### 5.4 Budget d'écriture

Le séquenceur ne publie qu'une trame **différente** de la précédente, et l'écrivain garde sa cadence (250 ms). Le coût
réel est celui des groupes dont l'état change (§2.3). Les effets natifs respectent : au plus 6 écritures par pas, au plus
10 écritures par seconde en régime établi, pour 12 groupes. Un test calcule ce coût pour chaque natif (pilote factice).

### 5.5 Pulsation existante

Jusqu'à sa migration (LOT-2, dernier batch), l'`id` réservé `pulsation` désigne le code actuel (`RunChronoPulse`). Après
migration, `pulsation` est un effet natif (`uniform`, `target: all`, trois étapes) et le code dédié est retiré. Exigence :
**rendu identique** (couleurs, intensités 255 / 128 / 77, cycle 1 s, découpe 250 / 750 ms à 5 s ou moins, fondu 0, une
écriture de groupe par phase) — prouvé par un test qui compare les trames à `ChronoPulseColor` sur toute la plage.

---

## 6. Réglages — effet par moment (amendement de `ambiance-settings.md` §2, §3)

> **Deux écritures, une seule règle — normatif (2026-11-11).** Chaque réglage d'effet a **deux chemins**, selon le fichier :
>
> | Où | Chemin (salle) | Chemin (groupe animateur) |
> |---|---|---|
> | `config.json` (valeur **globale**), `POST /config.json` | `lighting.effects.<moment>` | `lighting.host_effects.<moment>` |
> | Modèle `Settings` / `Partial` : `rounds.json` → `Round.AMBIANCE`, `ROUNDS_UPDATE`, `GET /api/ambiance/settings`, `GameState.AMBIANCE_EFFECTIVE` | `light.effects.<moment>` | `light.host_effects.<moment>` |
>
> C'est la règle déjà en place pour le mode : `lighting.default_mode` (`config.json`) ↔ `light.mode` (`Settings`),
> `ambiance-settings.md` §2. La section de `config.json` s'appelle `lighting` ; la branche du modèle de réglages s'appelle
> `light`. La conversion se fait une seule fois, dans `ambiancecfg.FromConfig` / `ToConfigPatch`. **`lighting.*` n'apparaît
> jamais dans `rounds.json` ni dans `AMBIANCE_EFFECTIVE` ; `light.*` n'apparaît jamais dans `config.json`.**

| Champ (`Settings`) | JSON | Source globale (`config.json`) | Défaut | Surchargeable par manche |
|---|---|---|---|---|
| `LightEffects["chrono"]` | `light.effects.chrono` | `lighting.effects.chrono` | `"pulsation"` | oui |
| `LightEffects["calm"]` | `light.effects.calm` | `lighting.effects.calm` | `""` (aucun effet) | oui |
| `LightEffects["score"]` | `light.effects.score` | `lighting.effects.score` | `""` (scène SCORE actuelle) | oui |

| `LightHostEffects["chrono"]` | `light.host_effects.chrono` | `lighting.host_effects.chrono` | `""` (blanc fixe) | oui |
| `LightHostEffects["calm"]` | `light.host_effects.calm` | `lighting.host_effects.calm` | `""` (blanc fixe) | oui |
| `LightHostEffects["score"]` | `light.host_effects.score` | `lighting.host_effects.score` | `""` (blanc fixe) | oui |

- `light.host_effects` règle les **groupes animateur** (§2.5) : mêmes règles de validation, de sources et de
  sérialisation que `light.effects`.
- **Révision du GATE LOT-1 — une entrée par groupe animateur, clé = `id` du groupe.** Les trois lignes
  `LightHostEffects[…]` ci-dessus se lisent **par groupe** : `LightHostEffects[<id>][<moment>]`.
  - `config.json` : `"lighting": { "host_effects": { "g5d7e4": { "chrono": "", "calm": "", "score": "couleur-equipe" },
    "g0b6c9": { "chrono": "pulsation", "calm": "", "score": "" } } }`.
  - Modèle de réglages : `"light": { "host_effects": { "<id>": { "chrono": "…", "calm": "…", "score": "…" } } }`. Forme
    complète (`global`, `effective`, `AMBIANCE_EFFECTIVE`) : **une entrée par groupe animateur existant**, les trois
    moments toujours présents ; aucun groupe animateur ⇒ `{}`. `Partial` : seuls les groupes et moments surchargés.
    `Sources.light.host_effects.<id>.<moment>` ∈ `global | round`.
  - Un groupe animateur sans entrée ⇒ blanc fixe aux trois moments.
  - **`id` inconnu** (groupe supprimé) : l'entrée est **écartée sans erreur** à l'écriture (`POST /config.json`,
    `ROUNDS_UPDATE`) et ignorée à la résolution — exception voulue à la règle « clé inconnue refusée », parce qu'un `id`
    meurt avec son groupe. Enregistrer `lighting.groups` purge de `lighting.host_effects` les `id` disparus. L'`id`
    d'effet, lui, reste validé (`400` / `INVALID_AMBIANCE`).
  - L'ancienne forme plate `{chrono, calm, score}` (codée au LOT-2, jamais livrée en PROD) n'est plus acceptée à
    l'écriture ; lue dans un `config.json` ou un `rounds.json` de QUALIF, elle est ignorée sans erreur.
- Valeur : `""` ou un `id` d'effet. `"chrono": ""` = aucune animation pendant le chronomètre (scène fixe).
- Validation à l'écriture (`POST /config.json`, `ROUNDS_UPDATE`) : l'`id` doit exister dans le catalogue et déclarer ce
  moment ; sinon `400` / `INVALID_AMBIANCE`. Un `id` devenu absent après coup n'invalide pas la configuration (§3.3).
- Forme complète : `"light": { "mode": "AUTO", "effects": { "chrono": "pulsation", "calm": "", "score": "" } }` — les trois
  clés toujours présentes. En `Partial` : seules les clés surchargées. `Sources.light.effects.<moment>` ∈
  `global | round` (pas de geste direct en v11.7.0).
- `GameState.AMBIANCE_EFFECTIVE.SETTINGS.light.effects` toujours sérialisé (règle « pas d'`omitempty` »).
- Le clignotement or de l'ampoule de l'équipe créditée (`lighting.md` §8.1) n'est pas un effet de ce contrat : inchangé,
  toujours actif.
- Non-régression : `config.json` sans `lighting.effects` et `rounds.json` sans `light.effects` ⇒ comportement v11.6.0 exact.

---

## 7. API HTTP

### 7.1 `GET /api/lighting/effects`

```json
{
  "groups": 4,
  "effects": [
    { "id": "chenillard", "name": "Chenillard", "description": "…", "moments": ["chrono"],
      "origin": "native", "file": "natifs/chenillard.yaml", "target": "groups", "min_groups": 3,
      "applicable": true, "reason": "", "host_applicable": false }
  ],
  "errors": [ { "file": "mon-effet.yaml", "line": 14, "message": "l'apparence « tete » n'est pas définie dans looks" } ],
  "unresolved": [ { "moment": "calm", "id": "mon-effet", "scope": "global" } ],
  "dir": "data/files/effects"
}
```

`target` ∈ `groups` | `all` ; `min_groups` vaut `0` pour un effet `target: all`. `applicable` concerne la salle,
`host_applicable` le groupe animateur (§4.6). `origin` ∈ `native` | `user` | `override` (fichier utilisateur qui surcharge un natif). `reason` explique
`applicable: false`. Ordre : natifs (ordre du §9), puis utilisateur par `name`. Aucune E/S vers le pont.

### 7.2 `POST /api/lighting/effects/reload`

Recharge le dossier. Réponse : celle du §7.1.

### 7.3 `GET /api/lighting/effects/{id}/preview?groups=N&moment=chrono`

Trames calculées par le serveur avec `effects.Render` — le navigateur ne fait que les rejouer (aucun second
interpréteur). `groups` : 1 à 12 (défaut : nombre configuré, 6 s'il vaut 0). Contexte simulé : chronomètre de 20 s pour
`chrono`, thème `#4F46E5`, équipe `#E53935`, groupe d'origine 1.

```json
{ "id": "chenillard", "groups": 6, "duration_ms": 20000, "loop": true,
  "frames": [ { "at_ms": 0, "groups": [ { "color": [79,70,229], "intensity": 128, "fade_ms": 0 }, … ] }, … ] }
```

Une trame n'est listée que si elle diffère de la précédente ; 400 trames au plus. `404` effet inconnu ; `400`
paramètre invalide.

### 7.4 `POST /api/lighting/effects/{id}/try` et `POST /api/lighting/effects/try/stop`

`{"seconds": 10}` (1 à 20, défaut 10) : joue l'effet sur les ampoules, puis retour à l'état vivant. Un nouvel essai
remplace le précédent. `409` avec `reason` : `disabled` (éclairage désactivé), `mode` (mode `ON`/`OFF` ou Flash),
`game` (une question est en cours : phase autre que STOPPED / NEW_GAME / ENROLL), `not_applicable`. Pont injoignable,
clé refusée, pilote occupé : formes de `/api/lighting/test` (§2.4), jamais `502`. L'état de l'essai n'est pas persisté.

### 7.5 Existants modifiés

`POST /config.json` : `lighting.groups`, `lighting.effects`, `lighting.host_effects` (§2.1, §6). `GET /api/ambiance/settings` et `ROUNDS_UPDATE` :
`light.effects`, `light.host_effects`. `GET /api/lighting/lights` : chaque ampoule porte `group` (`0` | `n` | `"host<k>"`, §2.4).

---

## 8. Critères chiffrés et mesure

### 8.1 Tests exigés

- `effects.Render` : cas de référence par motif (table de trames attendues), bornes (N = 1, N = 12), déterminisme de
  `random` à graine fixe.
- Chaque natif : valide, applicable selon `target` et `min_groups` (§4.6 — `pulsation` applicable avec N = 0), budget du §5.4.
- Fichier invalide : chacune des erreurs du §4.7 produit un message avec fichier et ligne ; le serveur démarre et les
  autres effets restent disponibles.
- Surcharge et restauration : fichier utilisateur de même `id` ⇒ `origin: override` ; fichier supprimé ⇒ retour au natif
  à chaud.
- Non-régression : sans groupes ni réglage d'effet, la suite `State` émise est identique à v11.6.0 (scènes, pulsation,
  SCORE, Flash, ON/OFF, extinction).

### 8.2 Mesure sur matériel (QUALIF)

À relever sur le pont réel et à consigner dans le rapport de QUALIF : durée d'un pas de chenillard pour 4, 6 et 8 groupes ;
régularité à l'œil ; seuil `SubGroupMinLights` (3 par défaut). Le banc `hue/bench_real_test.go` est étendu aux groupes.

---

## 9. Effets natifs v11.7.0

| `id` | Motif | Moments | Issue | Rendu |
|---|---|---|---|---|
| `pulsation` | `uniform`, `target: all` | chrono | existant (#212) | Respiration de toute la salle, orange puis rouge en fin de chrono |
| `chenillard` | `shift` | chrono | #223 | Tête `urgency`, traînée à 50 %, fond au thème ; sans traînée sous 5 groupes |
| `remplissage` | `gauge` | chrono | #300 | Groupes éteints au prorata du temps écoulé ; rouge et clignotement du groupe frontière à 5 s ou moins |
| `vague` | `gradient`, `travel: outward`, `origin: team`, `repeat: false`, `stops: [crête blanche, couleur de l'équipe]`, `rest` = thème atténué | score | #301 | Une crête blanche part du groupe de l'équipe créditée et gagne les bords ; derrière elle la salle prend la couleur de l'équipe (§4.3.1 point 3). `rest` est explicite : la scène SCORE ordinaire est déjà à la couleur de l'équipe, la vague serait invisible sans lui |
| `vague-lente` | `gradient`, `travel: forward`, `repeat: true`, `fade_ms` long, intensité basse | calm | #301 | Dégradé fermé qui tourne lentement dans la salle, sans couture (§4.3.1 point 2) |
| `degrade` | `gradient`, `travel: none` | calm | #211 | Dégradé fixe multi-couleurs : premier arrêt sur le groupe 1, dernier arrêt sur le dernier groupe (§4.3.1 point 1) |
| `scintillement` | `random` | calm | #302 | Quelques groupes s'allument doucement au hasard sur la teinte de la scène |
| `couleur-equipe` | `uniform`, `target: all`, `duration_ms` = durée de l'impulsion SCORE | score | #211 (groupe animateur) | Couleur de l'équipe créditée, fixe, le temps de l'attribution — pensé pour le groupe animateur, utilisable en salle |

Effets applicables au groupe animateur en v11.7.0 : `pulsation` et `couleur-equipe` (`target: all`), et tout effet
utilisateur `target: all` ou `target: groups` avec `min_groups` ≤ 1 (§4.6).

Les valeurs exactes (durées, intensités) sont dans les fichiers `assets/effects/*.yaml` ; elles peuvent être ajustées
après l'essai sur matériel sans changer ce contrat.

## 10. Hors de ce contrat

| Sujet | Où |
|---|---|
| Éditeur de scènes et d'effets dans l'interface | v11.8.0 (#210) — écrit le format du §4 |
| Rôles fins par ampoule (accentuation, fond) | hors v11.7.0 (décision D3) ; le besoin « animateur » est couvert par le groupe animateur (§2.5) |
| Effets sur les ampoules d'équipe, effets des buzzers | inchangés (`hue-bridge.md` §5.2, `LED_SET_PROTOCOL.md`) |
| Geste direct de choix d'effet depuis le popover | non prévu |
