# Contrat — Effets lumière déclaratifs : groupes d'ambiance, fichiers d'effets, séquenceur (v11.7.0)

> Issues : #225 (groupes), #223 (séquenceur, chenillard), #299 (catalogue), #300, #301, #302, #211 (effets répartis, dégradés).
> Décisions utilisateur du 2026-11-11 (GATE 1.5) : D1 les effets ciblent des **groupes**, jamais une ampoule isolée ; D2 l'effet
> est choisi **par moment**, en global et par manche ; D3 les « rôles fins » de #211 sont hors v11.7.0 ; D4 le catalogue offre
> un aperçu simulé **et** un essai sur les ampoules.
> Contrainte d'architecture (non négociable) : un effet est un **fichier descriptif éditable à la main**, interprété par un
> séquenceur commun. L'éditeur de scènes v11.8.0 (#210) écrira ce même format.
> Contrats liés : `lighting.md` (écrivain, scènes), `hue-bridge.md` (pilote), `ambiance-settings.md` (réglages).
>
> **Révision 3 (2026-11-11, 2e refus du GATE LOT-1) — modèle figé par l'utilisateur.** Les groupes sont **numérotés, sans
> nom** ; un effet est écrit **pas à pas** (format unique, §4), avec des **variables de groupe** ; les motifs calculés
> (`uniform`, `shift`, `gauge`, `random`, `gradient`), `target`, `looks`, `stages` sont **abandonnés**. Noyau seul : pas de
> variante selon le temps restant, pas de hasard, pas d'étalement sur le chronomètre, pas de départ depuis l'équipe.
> **#300 (remplissage), #301 (vague), #302 (scintillement) sortent de v11.7.0.** Références de la décision :
> `_work/reports/effects-format-proposal-20261111-213000.md`, `_work/reports/effects-chronogram-r2-20261111-223000.md`.
>
> **Révision 4 (2026-11-11) — trois zones symétriques.** Les groupes se répartissent en trois **zones** : `ambiance`
> (la salle), `animateur`, `public`. Chaque zone a ses groupes numérotés `g1`…`gN` et son `g0` ; on choisit **un effet et
> une vitesse par moment et par zone**. Les réglages par groupe animateur (`host_effects`, `host_speeds`) disparaissent.
> Référence : `_work/reports/plan-ambiguities-20261111-234500.md`.
>
> **Révision 5 (2026-11-12, v11.8.0 — #210, #305) — édition, import et export.** Ajout du **§11** : document d'effet
> (forme JSON), routes `source`, `draft`, `PUT`, `DELETE`, `download`, `import`, champ additif `vars` des trames
> d'aperçu. Le séquenceur ne change pas.
>
> **Révision 6 (2026-11-12, v11.8.0, GATE 2) — évolutions du format, §12.** Le dernier groupe de la zone s'écrit
> **`g*`** (plus `gN`) ; **`gN` devient un nom de variable ordinaire** ; **24 variables** au plus (au lieu de 8) ;
> **40 actions** au plus par pas (au lieu de 16). `version` reste `2`. Un fichier v11.7.0 reste lu sans modification
> (§12.3). **Les §4.1 à §4.8 se lisent avec les amendements du §12.**
>
> **Révision 7 (2026-11-12, v11.8.0, 3e retour du GATE 2) — §12 réécrit.** Le **départ et le pas d'une variable sont des
> expressions** (`N` = nombre de groupes, `+ - * /`, parenthèses, autre variable : `gA: N/2; +1`, `gB: gA+1`) ; division
> **entière** ; **`gN` employé sans être déclaré est refusé** (plus de lecture de compatibilité — **cassant** pour un
> fichier utilisateur v11.7.0 qui l'emploie) ; la grille de l'éditeur a un nombre de groupes réglable dont la
> **dernière ligne est `g*`**. Remplace, dans la révision 6 : « un fichier v11.7.0 reste lu sans modification ».

---

## 1. Vocabulaire

| Terme | Définition |
|---|---|
| **Groupe d'ambiance** | Sous-ensemble **ordonné et numéroté** (g1…gN) des ampoules de rôle `general`, **sans nom**. Unité que pilote un effet. `g0` désigne toute la zone. |
| **Zone** | Ensemble des groupes d'un même rôle : `ambiance` (rôle `salle`), `animateur`, `public` (§2.5). Chaque zone a sa propre numérotation `g1`…`gN`, son `g0`, et son effet par moment. |
| **Effet** | Liste de **pas** : à chaque tick d'horloge, un pas écrit une couleur et une intensité sur un ou plusieurs groupes. Un fichier = un effet. |
| **Tick** | Battement de l'horloge de l'effet (`duree_tick_ms`). Un pas dure toujours un tick. |
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
    { "id": "g3f9a1", "role": "salle",     "lights": ["BuzzHue1", "BuzzHue4"] },   // zone ambiance, g1
    { "id": "g81c02", "role": "salle",     "lights": ["BuzzHue2"] },               // zone ambiance, g2
    { "id": "g5d7e4", "role": "animateur", "lights": ["BuzzHue8"] },               // zone animateur, g1
    { "id": "g0b6c9", "role": "animateur", "lights": ["BuzzHue9"] },               // zone animateur, g2
    { "id": "g7a2d1", "role": "public",    "lights": ["BuzzHue11", "BuzzHue12"] }  // zone public, g1
  ]
}
```

- `role` : `"salle"` (défaut, valeur absente ou vide), `"animateur"` ou `"public"` *(révision 4)*. Le rôle d'un groupe
  est sa **zone** : `salle` ⇒ zone `ambiance`, `animateur`, `public` (§2.5).
  Le **numéro** d'un groupe est son rang parmi **tous** les groupes de sa zone, vides compris (`g1`, `g2`… dans chaque
  zone). N = nombre de groupes **non vides** de la zone.
- `id` : identifiant **stable** du groupe (`[a-z0-9]{1,12}`), unique, attribué par le serveur quand il est absent. Depuis
  la révision 4 il n'est plus la clé d'aucun réglage ; il est conservé dans la configuration.
- Plafond : **12 groupes par zone**.
- **Révision 3 — `name` et `near_teams` sont supprimés.** Un groupe n'a qu'un numéro (son rang) ; l'interface l'appelle
  « Groupe 1 », « Groupe animateur 1 ». `near_teams` ne servait qu'à l'effet Vague (#301), sorti du milestone. Les deux
  clés ne sont plus acceptées à l'écriture (`400`) ; lues dans un `config.json` de QUALIF antérieur, elles sont ignorées
  et disparaissent au prochain enregistrement.
- `id` *(révision du GATE LOT-1)* : identifiant **stable** du groupe (`[a-z0-9]{1,12}`), unique. Attribué par le serveur à
  l'enregistrement quand il est absent ou vide ; le client le renvoie tel quel ensuite (renommer ou déplacer un groupe ne
  change pas son `id`). Il sert de clé aux effets des groupes animateur (§6). Un `id` en double ou mal formé ⇒ `400`.
- `lights` : noms d'ampoules, chacune de rôle `general` et présente dans **un seul** groupe. Un groupe vide est accepté :
  il **garde son numéro** (numéros stables pour l'interface et pour `GET /api/lighting/lights`) mais est **ignoré du
  séquenceur**. **N = nombre de groupes de salle non vides.** Le séquenceur calcule sa trame sur la suite ordonnée des
  groupes non vides (positions 1…N, sans renuméroter la configuration) et émet chaque état avec le **numéro réel** du
  groupe (`ZoneState.Group`). Exemple : groupes 1, 2 (vide), 3 ⇒ N = 2, la trame a deux positions, écrites sur
  `Group 1` et `Group 3`. *(Amendement du 2026-11-11, revue LOT-1, écart 2.)*
- Absent ou `[]` ⇒ aucun groupe : comportement v11.6.0 exact.
- Écriture : `POST /config.json {"lighting": {"groups": [...]}}`, remplacement complet du tableau. Refus `400` avec un
  message lisible : ampoule inconnue de `lights`, ampoule de rôle `team`, ampoule dans deux groupes, rôle inconnu, plus
  de 12 groupes dans une zone.
- Changer le rôle d'une ampoule en `team`, ou la retirer de `lights`, la retire de son groupe dans la même écriture.
- **Ampoule générale hors groupe** : elle appartient à la zone `ambiance` ; elle fait partie de son `g0` et suit donc les
  actions écrites sur `g0` ; elle n'est jamais visée par `g1`…`gN` ni par une variable. Une ampoule n'entre dans la zone
  `animateur` ou `public` **que par son groupe** : ces deux zones n'existent que si elles ont au moins un groupe non vide.

### 2.2 Modèle d'état — `lighting.ZoneState.Area` et `Group` (amendement de `lighting.md` §3)

`ZoneState` porte, pour la zone `"general"` de l'écrivain, deux champs *(révision 4 : remplace l'adressage par numéros
négatifs `HostGroup(k)`)* :

- `Area lighting.Area` : `AreaAmbiance` (valeur zéro), `AreaHost` (zone animateur), `AreaPublic` (zone public) ;
- `Group int` : `0` = toute la zone, `n ≥ 1` = le groupe numéro n de cette zone.

`{Zone:"general"}` (les deux champs à zéro) reste l'état de toute la zone ambiance, comme en v11.6.0, et reste toujours
émis en premier. Une entrée `Group ≥ 1` **précise** l'entrée `Group 0` de sa zone pour ses ampoules. Une zone pour
laquelle aucune entrée n'est émise n'est **pas écrite** (zone public sans effet, §2.5).

### 2.3 Pilote Hue (amendement de `hue-bridge.md` §5.2, §5.3, §5.8)

> **Révision 4.** `LightSpec` porte `Area` et `Group`. Groupes Hue : `buzzmaster-general` (ampoules de la zone ambiance)
> et `buzzmaster-general-<n>` ; `buzzmaster-host` et `buzzmaster-host-<n>` ; `buzzmaster-public` et
> `buzzmaster-public-<n>`. Un groupe Hue de zone existe à partir de 2 ampoules, un groupe Hue de groupe à partir de
> `SubGroupMinLights` (3). Quand tous les groupes d'une zone ont le même état cible, une seule écriture sur le groupe Hue
> de la zone. Les anciens `buzzmaster-general-host<k>` disparaissent à la réconciliation. Une ampoule des zones animateur
> ou public ne reçoit jamais une entrée de la zone ambiance. Les puces ci-dessous valent pour la zone ambiance.

- `LightSpec` gagne `Group int` (0 = hors groupe). La clé de regroupement d'`Apply` devient (rôle, équipe, groupe).
- Résolution de l'état cible d'une ampoule générale du groupe n : l'entrée `Group == n` si elle existe, sinon l'entrée
  `Group == 0`.
- **`buzzmaster-general` exclut les ampoules des zones animateur et public** (§2.5 : jamais entraînées par la salle). Ses
  membres sont les ampoules de rôle `general` hors de ces deux zones ; « toutes les ampoules générales » ci-dessous s'entend de
  même. *(Amendement du 2026-11-11, revue LOT-1, écart 4.)*
- **Quand toutes les ampoules générales ont le même état cible**, le pilote écrit `buzzmaster-general` comme aujourd'hui
  (une écriture). C'est le cas de toute scène sans effet réparti : **aucune régression de débit**.
- Groupes Hue : un `LightGroup` `buzzmaster-general-<n>` par groupe d'ambiance de **3 ampoules ou plus**. En dessous,
  écriture par ampoule. Le seuil vaut `hue.SubGroupMinLights = 3` ; il est à confirmer par la mesure du §8.2 et
  `dev-backend` peut l'amender en documentant le chiffre mesuré.
- Réconciliation, repli par ampoule, invalidation du cache par membre : règles du §5.8, inchangées.
- Extinction à l'arrêt, `ON`/`OFF`/Flash : agissent sur la zone `general` entière (`Group 0`) ; le séquenceur est
  inactif dans ces cas (§5.3), donc aucune entrée `Group ≥ 1` n'est émise.

### 2.5 Les trois zones — normatif (révision 4)

| Zone | Rôle des groupes | Sans effet pour le moment vivant | ON / OFF / Flash | Scène de salle | Couleur `scene` |
|---|---|---|---|---|---|
| `ambiance` | `salle` | scène de salle ordinaire (v11.6.0) | **oui** (inchangé) | oui | couleur de la scène du moment |
| `animateur` | `animateur` | **blanc fixe** (`#FFFFFF`, 255) | non | non | blanc |
| `public` | `public` | **rien : la zone n'est pas pilotée** | non | non | blanc |

- **Symétrie** : dans chaque zone, les groupes sont `g1`…`gN` et `g0` désigne toute la zone. Un effet joué sur une zone
  ne voit que les groupes de cette zone ; le même fichier d'effet sert aux trois. Un chenillard peut tourner sur les
  groupes de la zone animateur.
- **Un effet et une vitesse par moment et par zone** (§6), en global et par manche.
- **Zone animateur** : jamais touchée par la scène de salle, ni par `ON` / `OFF`, ni par le Flash. Sans effet : blanc
  fixe, dès que l'éclairage est actif.
- **Zone public** *(réponse utilisateur : « si pas de public, on ne touche à rien ; si le groupe existe, par défaut, comme
  les autres, on n'y applique rien »)* :
  - aucun groupe public ⇒ aucun changement, nulle part ;
  - sans effet pour le moment vivant, **le serveur n'envoie rien** à ces ampoules : ni scène de salle, ni blanc fixe.
    Elles restent dans l'état où elles sont (réglé à la main, ou laissé par le dernier effet) ;
  - au **démarrage du serveur** : rien n'est envoyé à la zone public ;
  - à la **fin d'un moment** où un effet y jouait : `post` est appliqué s'il existe (§4.5) ; sinon les ampoules gardent
    le dernier état de l'effet. C'est la seule zone où POST est visible ;
  - `ON` / `OFF` / Flash : sans action sur la zone public ;
  - **interrupteur général éteint** (`lighting.enabled = false`) et **extinction à l'arrêt du serveur** : les ampoules de
    la zone public sont éteintes comme toutes les ampoules assignées (`lighting.md` §10.4, inchangé) ;
  - **après un essai** (essai d'un groupe §2.4, essai d'un effet §7.4) sur la zone public alors qu'aucun effet n'y joue :
    l'état des ampoules est **lu sur le pont au début de l'essai et réécrit à la fin** (allumé / éteint, luminosité,
    couleur). Lecture impossible ⇒ les ampoules sont éteintes à la fin de l'essai.
- **Zone sans groupe non vide** (`animateur`, `public`) : elle n'existe pas ; ses réglages sont conservés, sans effet, et
  l'interface ne montre pas son bloc.

### 2.4 Essai d'un groupe — `POST /api/lighting/groups/test`

> **Révision du GATE LOT-1 (2026-11-11)** — le clignotement blanc était trop discret : l'essai devient une **séquence
> visible**. Ce paragraphe remplace la version précédente.

`{"group": 2}` (groupe 2 de la zone ambiance), `{"group": "host1"}` (groupe 1 de la zone animateur) ou
`{"group": "public1"}` (groupe 1 de la zone public) joue sur les ampoules du groupe, **une fois**, la séquence :

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
  durée de pas injectable pour les tests) et lu par `Scene()`, qui émet l'entrée du groupe (`Area` et `Group`, §2.2) avec l'état du
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
groupe (réponse seulement), entier `n ≥ 1` = groupe n de la zone ambiance, chaîne `"host<k>"` = groupe k de la zone
animateur, chaîne `"public<k>"` = groupe k de la zone public *(révision 4)*. Une ampoule de rôle `team` porte `0`. *(Revue LOT-1, écart 1 ; `"host"` devient `"host<k>"`
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

Les **mots-clés des fichiers sont en français** (révision 3), comme les couleurs nommées.

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
- Un moment réglé sur un effet absent, invalide ou inapplicable (§4.7) joue la **valeur par défaut du moment** (§6 :
  `pulsation` pour `chrono`, aucun effet pour `calm` et `score`) et l'écart est visible dans
  `GET /api/lighting/effects` (`unresolved[]`).

---

## 4. Schéma d'un effet — normatif (révision 3 : format pas à pas)

```yaml
# Chenillard — un point lumineux fait le tour de la salle, un groupe par seconde.
id: chenillard               # [a-z0-9-], 1 à 40 caractères, unique
nom: Chenillard              # libellé du catalogue (1 à 40 caractères)
description: >               # texte du catalogue (500 caractères au plus)
  Un point lumineux fait le tour de la salle pendant le chronomètre.
version: 2                   # version du format ; seule la valeur 2 est acceptée
moments: [chrono]            # moments où l'effet est proposé : chrono, calm, score
groupes_min: 3               # facultatif : en dessous, l'effet n'est pas applicable à la salle (défaut 0)
duree_tick_ms: 1000          # durée d'un tick, donc d'un pas
boucle: oui                  # oui (défaut) | non
variables:                   # facultatif
  gA: g1; +1                 # part de g1, avance d'un groupe
pre:                         # facultatif — PRE : joué une fois au démarrage, hors boucle
  { g0: [theme, 100] }
pas:                         # la boucle : un pas par tick, dans l'ordre
  - { g0: [theme, 100], gA: [urgence, 100] }
dernier:                     # facultatif — LAST : joué à la fin de chaque tour, avant de revenir au pas 0
  { g0: [theme, 100] }
post:                        # facultatif — POST : appliqué à la fin du moment
  { g0: [scene, 100] }
```

### 4.1 Clés de premier niveau

`id`, `nom`, `description`, `version`, `moments`, `groupes_min`, `duree_tick_ms`, `boucle`, `variables`, `pre`, `pas`,
`dernier`, `post`. Obligatoires : `id`, `nom`, `version`, `moments`, `duree_tick_ms`, `pas` (1 à 200 pas). Toute autre clé
est refusée. `duree_tick_ms` : 250 à 60 000. Un fichier `version: 1` (motifs calculés) est refusé avec le message
« format d'effet de la version précédente : réécrire en pas à pas ».

### 4.2 Un pas, une action

Un pas (`pre`, chaque élément de `pas`, `dernier`, `post`) est une table `cible: action`, 1 à 16 actions *(révision 6 : 40, §12)*.

- **Action** : `[couleur, intensité]` — intensité de 0 à 100 (pourcentage ; 0 = éteint) — ou `[eteint]`. Fondu toujours
  nul (`TransitionMs = 0`).
- **Couleur** : `"#RRGGBB"`, ou un nom : `theme` (thème de la question, blanc à défaut), `equipe` (équipe créditée,
  moment `score` ; blanc hors de ce moment), `urgence` (blanc ; orange `#FFAA00` à 10 s ou moins ; rouge `#E61E1E` à 5 s
  ou moins — hors moment `chrono` : blanc), `scene` (couleur de la scène de salle du moment ; blanc pour un groupe
  animateur). Les couleurs nommées sont **vivantes** : elles sont résolues à chaque rendu, pas au moment où le pas est
  joué — un groupe peint en `urgence` vire de lui-même à l'orange puis au rouge.
- Les actions d'un pas s'appliquent **dans l'ordre écrit** ; si deux actions touchent la même ampoule, la dernière gagne
  (`g0` puis `gA` : `gA` repeint son groupe).
- **Un groupe non cité garde l'état** reçu aux pas précédents. Avant toute action, la zone a sa scène ordinaire.

### 4.3 Cibles

| Écriture | Désigne |
|---|---|
| `g0` | toute la zone où l'effet est joué : pour `ambiance`, toutes les ampoules d'ambiance, groupées ou non ; pour `animateur` et `public`, toutes les ampoules des groupes de la zone. Jamais une ampoule d'une autre zone ni d'une équipe |
| `g1`, `g2`… | le groupe de ce **numéro réel** dans la zone (§2.1) ; **absent ou vide ⇒ action ignorée** |
| `gN` | le dernier groupe non vide — ***révision 6 : s'écrit `g*`, §12*** |
| `gA`, `gB`… (`g` + une lettre majuscule autre que `N`, puis lettres ou chiffres) | une variable déclarée dans `variables` — ***révision 6 : `N` est autorisée, §12*** |
| `gA+1`, `gA-2` | le voisin du groupe que désigne `gA`, à m rangs ; après le dernier groupe on revient au premier |

N = nombre de groupes **non vides** de la zone où l'effet est joué. Les **rangs** 1…N sont la suite ordonnée de ces
groupes ; tout calcul reboucle sur N (le fichier ne connaît pas la taille de la zone).

**Numéro réel et rang** *(précision du Batch R)* : une cible directe `g3` désigne le groupe **numéro 3** (ignorée s'il est
absent ou vide). Le **départ** d'une variable et ses positions sont des **rangs** parmi les groupes non vides : avec les
groupes 1, 2 (vide), 3, `gA: g2; 0` désigne le 2e groupe non vide, c'est-à-dire le groupe 3. Une **liste** cite des
numéros réels ; les groupes absents ou vides en sont retirés.

**Pas sauté** : un pas de la boucle dont **aucune** action ne vise une cible existante est sauté — il ne consomme pas de
tick et ne fait pas avancer les variables. Si tous les pas de la boucle sont sautés, l'effet est inapplicable (§4.7).

### 4.4 Variables de groupe

| Déclaration | Sens |
|---|---|
| `gA: g1; +1` | progression : départ au groupe 1, +1 groupe par cran |
| `gB: g2; +2` | départ au groupe 2, +2 par cran |
| `gD: gN; -1` | départ au dernier groupe, recule d'un groupe par cran |
| `gE: g3; 0` | désigne toujours le groupe 3 |
| `gC: [g1, g4, g3, g7]` | liste : un élément par cran, reprise au début après le dernier ; les groupes absents sont retirés de la liste (liste vide ⇒ actions sur `gC` ignorées) |

- Progression : au cran k, la variable désigne la position `((départ − 1 + incrément × k) mod N) + 1`. Un départ au-delà
  de N reboucle (`g7` avec N = 4 ⇒ position 3). Incrément : entier de −12 à +12.
- 8 variables au plus par effet *(révision 6 : 24, §12)*. N = 0 ⇒ toute action sur une variable est ignorée.

### 4.5 Déroulement et règle d'avance — normatif

1. **Démarrage** (début du moment, ou de l'essai) : cran k = 0. Si `pre` existe, il est joué au tick 0.
2. **Boucle** : le pas 0 est joué au tick 1 s'il y a un `pre`, **au tick 0 sinon** ; puis un pas par tick. Après le dernier pas de `pas`, `dernier` (s'il existe)
   occupe un tick ; puis retour au pas 0.
3. **Avance des variables** : PRE et le **premier** pas 0 voient les variables à leur départ (k = 0). Ensuite k augmente
   de 1 **à chaque pas de la boucle joué**. **PRE, `dernier` (LAST) et POST ne font pas avancer** : `dernier` voit les
   mêmes valeurs que le pas qui le précède. k ne revient jamais à 0 quand la boucle recommence.
4. `boucle: non` : un seul tour (`pas`, puis `dernier`), puis l'état est tenu jusqu'à la fin du moment.
5. **Fin du moment** : `post` (s'il existe) est rendu **immédiatement**, sans attendre un tick, avec les variables à
   leur dernière valeur, et ne retarde jamais l'état suivant. *(Révision 4, précision du Batch R)* Dans les zones
   `ambiance` et `animateur`, l'état suivant (scène de salle, blanc fixe, autre effet) repeint toute la zone : **POST n'y
   a aucun effet visible** et n'est pas écrit sur les ampoules. Dans la zone `public`, rien ne le recouvre : **POST est
   appliqué et tenu** jusqu'au prochain effet (§2.5).

Chronogramme de référence (4 groupes, `gA: g1; +1`, `gB: g2; +2`, `pre`, deux pas, `dernier`) :

| Tick | Joué | k | gA | gB |
|---|---|---|---|---|
| 0 | PRE | 0 | g1 | g2 |
| 1 | pas 0 | 0 | g1 | g2 |
| 2 | pas 1 | 1 | g2 | g4 |
| 3 | LAST | 1 | g2 | g4 |
| 4 | pas 0 | 2 | g3 | g2 |
| 5 | pas 1 | 3 | g4 | g4 |
| 6 | LAST | 3 | g4 | g4 |
| 7 | pas 0 | 4 | g1 | g2 |

### 4.6 Rendu vers l'écrivain

Le séquenceur tient, pour la zone, un **état de base** (dernière action sur `g0`) et un état par groupe. La trame émise
est : une entrée `{Zone:"general", Group:0}` portant l'état de base (ou la scène ordinaire tant que `g0` n'a pas été
écrit), puis une entrée `Group:n` pour chaque groupe dont l'état diffère de la base. **Un effet qui n'écrit que `g0`
produit donc une seule entrée `Group 0`** — une seule écriture `buzzmaster-general`, comme la pulsation d'origine.
Les entrées portent l'`Area` de la zone jouée (§2.2).

### 4.7 Applicabilité et repli (révision 4 : même règle pour les trois zones)

Un effet est **applicable à une zone** si la zone existe, si N ≥ `groupes_min` (N = groupes non vides de la zone) et si
au moins un pas de la boucle n'est pas sauté (§4.3). La règle « groupe animateur = effet qui n'écrit que `g0` » est
supprimée.

Un effet inapplicable fait jouer au moment la valeur par défaut de la zone (§6). Le catalogue et le sélecteur
l'indiquent (« demande 5 groupes, 3 dans cette zone »).

### 4.8 Erreurs de validation

Chaque erreur porte `file`, `line` (0 si inconnue), `message` en français, sans jargon :
`"mon-effet.yaml, ligne 12 : la variable « gB » n'est pas déclarée dans variables"`. Liste minimale : YAML illisible, clé
inconnue, clé obligatoire absente, `id` invalide ou en double, `version` non prise en charge, `moments` vide ou valeur
inconnue, `duree_tick_ms` hors bornes, `pas` vide ou trop long, cible mal écrite, variable non déclarée, déclaration de
variable mal écrite, couleur invalide, intensité hors 0-100, fichier trop gros.

**Textes écrits par le serveur (v11.8.0, §11.4 et §11.7)** — deux refus de plus, qui ne valent que pour un texte
**enregistré** (`PUT`) ou **importé**, pas pour le chargement du dossier :

- **Alias YAML** : un texte qui contient un alias (`*nom`, référence à une ancre `&nom`) est refusé, avec la ligne du
  premier alias : « les alias YAML (*nom) ne sont pas acceptés : écrivez les valeurs en toutes lettres ». Le contrôle
  se fait sur l'arbre du document, **avant** tout décodage : aucun alias n'est développé. Une ancre seule (`&nom`, sans
  alias) est acceptée.
- **Identifiant réservé** : un `id` qui est un mot de route (`reload`, `try`, `draft`, `import`) ou un **nom de
  périphérique Windows** (`con`, `prn`, `aux`, `nul`, `com1` à `com9`, `lpt1` à `lpt9`) est refusé, avec la ligne de la
  clé `id` — le nom du fichier écrit dérive de l'`id` (§11.4) et `con.yaml` n'est pas un nom de fichier utilisable sous
  Windows. Messages : « « reload » est un mot réservé : changez la clé id » ; « « con » est un nom réservé par Windows :
  changez la clé id ».

Un fichier **déposé à la main** dans le dossier garde les règles de v11.7.0 : son nom est libre, son `id` peut être
l'un de ces mots, et ses alias sont traités par le chargeur (dont l'analyseur borne lui-même le développement des
alias ; la taille du fichier reste limitée à 64 Ko).

---

## 5. Séquenceur

### 5.1 Découpage imposé

- **`internal/effects`** (nouveau, pur, sans goroutine ni E/S réseau) : types, `Parse([]byte) (Effect, []Error)`,
  `LoadDir(embedded fs.FS, dir string) Catalogue`, et la fonction de rendu
  `Render(e Effect, ctx Context, tick int) Frame` (révision 3 : rendu des pas du §4.5 / §4.6 ; `ctx` porte N, les
  numéros réels des groupes non vides et les couleurs nommées résolues). `Render` est **déterministe** : mêmes entrées,
  même trame.
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
| `score` | pendant la durée de l'impulsion SCORE (`ScorePulseDuration`) après un `NotifyPulse(KindScore)` (les 6 sites existants) |
| aucun | tout autre cas : pas de trame, scène ordinaire |

`score` prime sur `chrono` et `calm` pendant sa durée.

### 5.3 Précédence — normatif

1. `lighting.enabled == false`, pilote absent ⇒ le lecteur ne publie rien.
2. Mode `ON` / `OFF` ou Flash engagé ⇒ **aucune trame** : `OverrideGeneral` s'applique comme aujourd'hui. Même test que
   la pulsation (`ambiance.go`, `Scene`) : `lightingMode() == ModeAuto && !isLightingFlashOn()`.
3. Essai d'un effet (§7.3) ⇒ prime sur le moment vivant, jamais sur le point 2.
4. Sinon, trame de l'effet résolu du moment vivant.

L'arrêt du serveur (`quiesceAmbiance`) annule le lecteur avant l'extinction.

**Zones animateur et public** *(révision 4)* : le lecteur calcule **une trame par zone existante** (trois au plus), avec
le réglage de la zone (`light.effects.<zone>.<moment>`, `light.speeds.<zone>.<moment>`), le même moment vivant et les N
groupes de la zone. Pour ces deux zones, seul le point 1 s'applique ; l'essai d'un effet (§7.4) ne touche que la zone
qu'il vise. Zone public sans effet : aucune entrée émise (§2.5).

### 5.4 Budget d'écriture

Le séquenceur ne publie qu'une trame **différente** de la précédente, et l'écrivain garde sa cadence (250 ms). Le coût
réel est celui des groupes dont l'état change (§2.3). Les effets natifs respectent : au plus 6 écritures par pas, au plus
10 écritures par seconde en régime établi, pour 12 groupes. Un test calcule ce coût pour chaque natif (pilote factice).

### 5.5 Pulsation (révision 3)

`pulsation` est un effet natif joué par le lecteur (commit `7a7a33aa`, conservé) ; le code dédié reste retiré. Avec le
noyau seul, **le rendu n'est plus identique à v11.6.0** (décision utilisateur) : phase haute au thème à 100 %, phase
basse à la couleur `urgence` à 50 %, 500 ms chacune.

| Temps restant | v11.6.0 | Révision 3 |
|---|---|---|
| plus de 10 s | thème 100 % / thème 50 % | thème 100 % / **blanc** 50 % |
| 10 s ou moins | thème 100 % / orange 50 % | identique |
| 5 s ou moins | thème 100 % / rouge 30 %, battement 250 / 750 ms | thème 100 % / rouge **50 %**, battement **500 / 500 ms** |

Le test d'identité `internal/ambiance/pulsation_identity_test.go` est **remplacé** par un test du rendu ci-dessus (même
balayage : thèmes × temps restant × horloge). Une seule écriture de groupe par phase (§4.6).

---

## 6. Réglages — effet et vitesse par moment et par zone (révision 4)

> Remplace les versions précédentes de cette section (effet par moment pour la salle, `host_effects` / `host_speeds` par
> groupe animateur). Amende `ambiance-settings.md` §2, §3, §10.

**Deux écritures, une seule règle.** La section de `config.json` s'appelle `lighting` ; la branche du modèle de réglages
(`rounds.json` → `Round.AMBIANCE`, `ROUNDS_UPDATE`, `GET /api/ambiance/settings`, `GameState.AMBIANCE_EFFECTIVE`) s'appelle
`light`. Même règle que `lighting.default_mode` ↔ `light.mode`. `lighting.*` n'apparaît jamais dans `rounds.json` ni dans
`AMBIANCE_EFFECTIVE` ; `light.*` n'apparaît jamais dans `config.json`.

| Champ (`Settings`) | Modèle de réglages | `config.json` (valeur globale) | Défaut | Par manche |
|---|---|---|---|---|
| `LightEffects[<zone>][<moment>]` | `light.effects.<zone>.<moment>` | `lighting.effects.<zone>.<moment>` | `pulsation` pour `ambiance.chrono` ; `""` partout ailleurs | oui |
| `LightSpeeds[<zone>][<moment>]` | `light.speeds.<zone>.<moment>` | `lighting.speeds.<zone>.<moment>` | `1` | oui |

`<zone>` ∈ `ambiance`, `animateur`, `public` ; `<moment>` ∈ `chrono`, `calm`, `score`.

```json
"light": {
  "mode": "AUTO",
  "effects": { "ambiance":  { "chrono": "pulsation", "calm": "", "score": "" },
               "animateur": { "chrono": "", "calm": "", "score": "couleur-equipe" },
               "public":    { "chrono": "", "calm": "", "score": "" } },
  "speeds":  { "ambiance":  { "chrono": 1, "calm": 1, "score": 1 },
               "animateur": { "chrono": 1, "calm": 1, "score": 1 },
               "public":    { "chrono": 1, "calm": 1, "score": 1 } }
}
```

- **Effet** : `""` ou l'`id` d'un effet. `""` = aucun effet : scène de salle (`ambiance`), blanc fixe (`animateur`), zone
  non pilotée (`public`) — §2.5. Validation à l'écriture (`POST /config.json`, `ROUNDS_UPDATE`) : l'`id` doit exister dans
  le catalogue et déclarer ce moment ; sinon `400` / `INVALID_AMBIANCE`. Un `id` devenu absent après coup n'invalide pas
  la configuration (§3.3).
- **Vitesse** : `0.5`, `0.75`, `1`, `1.5`, `2` (nombre JSON) ; toute autre valeur ⇒ `400` / `INVALID_AMBIANCE`. **Durée
  réelle d'un tick = `duree_tick_ms` ÷ vitesse, jamais sous 250 ms.** Le fichier d'effet n'est jamais modifié. Un
  changement de vitesse est pris **au tick suivant**, sans faire repartir l'effet. Sans effet joué, la vitesse est sans
  objet.
- **Forme complète** (`global`, `effective`, `AMBIANCE_EFFECTIVE`) : les trois zones et les trois moments toujours
  présents, sans `omitempty`, que la zone existe ou non. `Partial` (manche) : seules les valeurs surchargées.
  `Sources.light.effects.<zone>.<moment>` et `Sources.light.speeds.<zone>.<moment>` ∈ `global | round` (pas de geste
  direct).
- Clé de zone ou de moment inconnue ⇒ refus (`400` / `INVALID_AMBIANCE`).
- **Lecture d'une configuration antérieure** (QUALIF, jamais en PROD) : `lighting.effects.{chrono,calm,score}` et
  `lighting.speeds.{chrono,calm,score}` à plat sont lus comme ceux de la zone `ambiance` ; `host_effects` et
  `host_speeds` sont ignorés. Ces formes ne sont plus acceptées à l'écriture et disparaissent au prochain enregistrement.
  Même tolérance pour `light.*` dans `rounds.json`.
- Le clignotement or de l'ampoule de l'équipe créditée (`lighting.md` §8.1) n'est pas un effet de ce contrat : inchangé.
- Non-régression : sans clé `lighting.effects` / `lighting.speeds` ni groupe, comportement v11.6.0 pour les scènes, le
  SCORE, le Flash, ON / OFF et l'extinction ; la pulsation suit le §5.5.

---

## 7. API HTTP

### 7.1 `GET /api/lighting/effects`

```json
{
  "groups": { "ambiance": 4, "animateur": 2, "public": 0 },
  "effects": [
    { "id": "chenillard", "name": "Chenillard", "description": "…", "moments": ["chrono"],
      "origin": "native", "file": "natifs/chenillard.yaml", "min_groups": 3, "tick_ms": 1000, "steps": 1,
      "applicable": { "ambiance": true, "animateur": false, "public": false } }
  ],
  "errors": [ { "file": "mon-effet.yaml", "line": 12, "message": "la variable « gB » n'est pas déclarée dans variables" } ],
  "unresolved": [ { "zone": "ambiance", "moment": "calm", "id": "mon-effet", "scope": "global" } ],
  "dir": "data/files/effects"
}
```

**`steps`** *(complément du LOT-3)* : nombre de pas de la boucle (longueur de `pas`, sans compter `pre`, `dernier` ni
`post`) — affiché « 2 pas » sur la ligne « Fichier » du catalogue.
Les noms de champs de l'API restent en anglais : `name` = `nom` du fichier, `min_groups` = `groupes_min` (0 par défaut),
**`tick_ms` = `duree_tick_ms`** *(ajout du Batch R : sert à l'indication « plafonné » de la vitesse)*. *(Révision 4)*
`groups` donne le nombre de groupes non vides **par zone** ; `applicable` est une table **par zone** (§4.7 ; zone
inexistante ⇒ `false`) ; les champs `host_applicable` et `reason` sont supprimés (l'interface déduit la raison de
`min_groups` et de `groups`). `origin` ∈ `native` | `user` | `override`. Ordre : natifs (ordre du §9), puis utilisateur
par `name`. Aucune E/S vers le pont.

### 7.2 `POST /api/lighting/effects/reload`

Recharge le dossier. Réponse : celle du §7.1.

### 7.3 `GET /api/lighting/effects/{id}/preview?groups=N&moment=chrono`

Trames calculées par le serveur avec `effects.Render` — le navigateur ne fait que les rejouer (aucun second
interpréteur). `groups` : 1 à 12 (défaut : nombre de groupes de la zone ambiance, 6 s'il vaut 0). `moment` : l'un des
moments déclarés par l'effet (défaut : le premier). Vitesse ×1. Contexte simulé : thème `#4F46E5`, équipe `#E53935`,
couleur `scene` blanche ; pour `chrono`, un chronomètre qui descend de `chrono_s` secondes.

```json
{ "id": "chenillard", "moment": "chrono", "groups": 6, "tick_ms": 1000,
  "duration_ms": 20000, "chrono_s": 20, "loop": true,
  "frames": [
    { "at_ms": 0,    "tick": 0, "step": "pas:0",
      "groups": [ { "color": [255,255,255], "intensity": 255 }, { "color": [79,70,229], "intensity": 255 }, … ] },
    { "at_ms": 1000, "tick": 1, "step": "pas:0", "groups": [ … ] }
  ] }
```

*(Compléments du LOT-3, 2026-11-11 — champs additifs, pour couvrir la maquette `ambiance_page__effects_catalogue_zones`.)*

- **Une trame par tick**, dans l'ordre, **même si elle est identique à la précédente** : c'est ce qui permet d'afficher le
  numéro du tick et le pas joué à chaque battement. (Remplace « une trame n'est listée que si elle diffère ».) Si une
  couleur nommée change entre deux ticks (`urgence` à 10 s et à 5 s), une trame supplémentaire est insérée à cet instant,
  avec le même `tick` et le même `step`.
- **`tick`** (entier ≥ 0) : numéro du tick depuis le début de l'effet (§4.5).
- **`step`** (chaîne) : le pas joué à ce tick — **`"pre"`**, **`"pas:<n>"`** (n = rang du pas dans `pas`, à partir de 0 :
  `"pas:0"`, `"pas:1"`…) ou **`"dernier"`**. Un pas sauté (§4.3) n'apparaît pas, puisqu'il ne consomme pas de tick.
  `"post"` n'apparaît jamais : l'aperçu ne simule pas la fin du moment.
- **`at_ms`** : instant de la trame depuis le début de l'aperçu (`tick` × `tick_ms` pour une trame de tick).
- **`groups`** : un état par groupe, dans l'ordre `g1`…`gN` : `color` (RGB résolu), `intensity` (0-255). Le champ
  `fade_ms` est supprimé (pas de fondu, §4.2).
- **Groupe jamais écrit par l'effet** *(revue LOT-3, 2026-11-12)* : tant qu'aucun pas ne l'a peint (ni par `g0`, ni
  directement, ni par une variable), un groupe apparaît dans l'aperçu avec la **scène ordinaire simulée : blanc
  `[255,255,255]`, intensité `200`**. `200` est une valeur d'affichage propre à l'aperçu (celle de la scène de salle hors
  question, `lighting.md` §8) : elle distingue à l'œil un groupe « laissé à la scène » d'un groupe peint en blanc à 100 %
  (intensité `255`, comme la tête du chenillard dans l'exemple ci-dessus).
- **Aperçu calculé même sous `groupes_min`** *(revue LOT-3)* : la route ne refuse pas un `groups` inférieur à
  `groupes_min` et ne consulte pas l'applicabilité ; elle rend l'effet tel qu'il se jouerait avec ce nombre de groupes.
  C'est l'interface qui lit `applicable` (§7.1) : pour un effet inapplicable, elle demande l'aperçu avec assez de groupes
  et l'annonce (« simulation avec N groupes »).
- **Aucun pas visible** *(revue LOT-3)* : si, pour le `groups` demandé, tous les pas de l'effet sont sautés (§4.3), la
  réponse est `200` avec **`"frames": []`** (tableau vide, jamais `null`) ; `duration_ms`, `tick_ms`, `chrono_s` et les
  autres champs de premier niveau restent renseignés.
- **`duration_ms`** — durée de l'aperçu, **fixée par le moment** : **20 000** pour `chrono` et `calm`, **5 000** pour
  `score`. L'interface lit cette valeur ; elle ne la fixe pas elle-même. `loop: true` : l'aperçu se rejoue depuis le
  début après `duration_ms`.
- **`chrono_s`** : durée du chronomètre simulé en secondes — `20` pour `chrono`, `0` pour les autres moments (pas de
  compte à rebours à afficher). Temps restant affiché = `chrono_s` − `at_ms` / 1000.
- **`tick_ms`** : durée d'un tick de l'effet (`duree_tick_ms`).
- 400 trames au plus (au-delà, l'aperçu est tronqué et `duration_ms` raccourci d'autant). `404` effet inconnu ; `400`
  paramètre invalide ou moment non déclaré par l'effet.

### 7.4 `POST /api/lighting/effects/{id}/try` et `POST /api/lighting/effects/try/stop` (révision 4)

Joue l'effet sur les ampoules d'**une zone**, puis rend la zone à son état vivant. Corps :
`{"zone": "ambiance", "speed": 1, "seconds": 10}` — `zone` ∈ `ambiance` (défaut), `animateur`, `public` ; `speed` ∈ valeurs
du §6 (défaut `1`) ; `seconds` de 1 à 20 (défaut **10**, ou **5** si l'effet ne déclare que le moment `score`). Deux appelants :

| Appelant | Appel | Rendu |
|---|---|---|
| Bouton **« Tester »** de l'onglet Effets, à côté d'un choix d'effet (zone, moment) | `zone` = la zone du réglage, `speed` = la vitesse réglée pour ce moment | l'effet tel qu'il sera joué en jeu, sur sa zone |
| Bouton **« Essayer sur la salle »** du catalogue | v11.7.0 : corps vide ou `{}` ; **v11.8.0 : `{"zone": <zone choisie>}`** (§11.8) | vitesse ×1, sur la zone choisie (`ambiance` par défaut) |

- Contexte simulé : thème `#4F46E5`, équipe `#E53935`, chronomètre simulé qui descend de 15 s (la couleur `urgence`
  passe par l'orange puis le rouge pendant un essai de 10 s).
- Réponse immédiate `200 {"ok": true, "duration_ms": 10000}`. Un nouvel essai (ou un essai de groupe, §2.4) **remplace**
  celui en cours ; `…/try/stop` l'arrête ; dans les deux cas la zone revient à son état vivant (zone public : §2.5).
- `409` avec `reason` : `disabled` (éclairage désactivé ou clé refusée), `game` (une question est en cours : phase autre
  que STOPPED / NEW_GAME / ENROLL), `not_applicable` (zone inexistante ou effet inapplicable à cette zone, §4.7), `mode`
  (mode `ON` / `OFF` ou Flash — **zone `ambiance` seulement**). `503` pont injoignable. `400` paramètre invalide, `404`
  effet inconnu. Jamais `502`. L'état de l'essai n'est pas persisté.

### 7.5 Existants modifiés

`POST /config.json` : `lighting.groups`, `lighting.effects`, `lighting.speeds` (§2.1, §6). `GET /api/ambiance/settings` et `ROUNDS_UPDATE` :
`light.effects`, `light.speeds`. `GET /api/lighting/lights` : chaque ampoule porte `group` (`0` | `n` | `"host<k>"` | `"public<k>"`, §2.4).

---

## 8. Critères chiffrés et mesure

### 8.1 Tests exigés

- `effects.Render` (révision 3) : le **chronogramme de référence du §4.5** tick par tick (variables et couleur de chaque
  groupe) ; progression, incrément négatif, liste, voisins, rebouclage (N = 1, 4, 8, 12) ; groupe absent ignoré, pas
  sauté ; groupe non cité = état conservé ; ordre des actions ; `boucle: non` ; couleurs nommées vivantes ; trame réduite
  à `Group 0` pour un effet qui n'écrit que `g0`.
- Lecteur : PRE au démarrage du moment, POST sans retard (sans effet visible en `ambiance` et `animateur`, appliqué et
  tenu en `public`) ; vitesse — tick divisé par le multiplicateur, plancher de 250 ms, changement pris au tick suivant.
- Zones (révision 4) : une trame par zone existante ; un effet de zone ne touche aucune ampoule d'une autre zone ;
  zone animateur en blanc fixe sans effet et insensible à ON / OFF / Flash ; zone public jamais écrite sans effet ni
  essai, insensible à ON / OFF / Flash, éteinte par l'interrupteur général et à l'arrêt, état lu puis réécrit autour
  d'un essai ; aucun groupe `animateur` ni `public` ⇒ suite d'états identique à celle d'avant la révision 4.
- Essai d'effet : zone et vitesse du corps, remplacement, arrêt, refus (`mode` pour la seule zone ambiance).
- Chaque natif : valide, applicable selon `groupes_min` et §4.7 (`pulsation` applicable à la zone ambiance avec N = 0),
  budget du §5.4.
- Fichier invalide : chacune des erreurs du §4.8 produit un message avec fichier et ligne ; un fichier `version: 1` est
  refusé avec son message ; le serveur démarre et les autres effets restent disponibles.
- Surcharge et restauration : fichier utilisateur de même `id` ⇒ `origin: override` ; fichier supprimé ⇒ retour au natif
  à chaud.
- Non-régression : sans groupes ni réglage d'effet, la suite `State` émise est identique à v11.6.0 (scènes, pulsation,
  SCORE, Flash, ON/OFF, extinction).

### 8.2 Mesure sur matériel (QUALIF)

À relever sur le pont réel et à consigner dans le rapport de QUALIF : durée d'un pas de chenillard pour 4, 6 et 8 groupes ;
régularité à l'œil ; seuil `SubGroupMinLights` (3 par défaut). Le banc `hue/bench_real_test.go` est étendu aux groupes.

---

## 9. Effets natifs v11.7.0 (révision 3)

| `id` | Moments | `groupes_min` | Issue | Pas | Rendu |
|---|---|---|---|---|---|
| `pulsation` | chrono | 0 | existant (#212) | `g0: [theme, 100]` puis `g0: [urgence, 50]`, tick 500 ms | Toute la salle respire ; la phase basse passe à l'orange puis au rouge (§5.5) |
| `chenillard` | chrono | 3 | #223 | `gA: g1; +1` ; un pas `g0: [theme, 100], gA: [urgence, 100]`, tick 1000 ms | Tête à la couleur d'urgence, fond au thème, un groupe par seconde |
| `chenillard-traine` | chrono | 5 | #223 | même variable ; un pas `g0: [theme, 100], gA-1: [theme, 50], gA+1: [theme, 50], gA: [urgence, 100]` | Le même avec une traînée à 50 % de part et d'autre de la tête |
| `couleur-equipe` | score | 0 | #211 (groupes animateur) | un pas `g0: [equipe, 100]`, tick 1000 ms | Couleur de l'équipe créditée le temps de l'attribution |
| `degrade` *(livré avec le Batch R)* | calm | 2 | #211 | un pas qui donne une couleur à chacun des groupes `g1` à `g12` (cycle de 4 couleurs), tick 60 000 ms | Scène multicolore fixe, une couleur par groupe ; une salle plus petite en montre le début |

#223 demandait « pas de traînée sous 5 groupes » dans un seul effet : sans variante, la règle devient **deux effets
natifs**, `chenillard-traine` n'étant proposé qu'à partir de 5 groupes.

Sortis de v11.7.0 (noyau seul) : `remplissage` (#300), `scintillement` (#302), `vague` et `vague-lente` (#301).

Les valeurs exactes (durées, couleurs, intensités) sont dans les fichiers `assets/effects/*.yaml` ; elles peuvent être
ajustées après l'essai sur matériel sans changer ce contrat.

## 10. Hors de ce contrat

| Sujet | Où |
|---|---|
| Éditeur d'effets dans l'interface, import et export | v11.8.0 (#210, #305) — **§11** de ce contrat |
| Remplissage (#300), vague (#301), scintillement (#302) ; variantes selon le temps restant, hasard, étalement sur le chronomètre, départ depuis l'équipe, fondu | hors v11.7.0 (révision 3) — backlog |
| Rôles fins par ampoule (accentuation, fond) | hors v11.7.0 (décision D3) ; le besoin « animateur » est couvert par la zone animateur (§2.5) |
| Effets sur les ampoules d'équipe, effets des buzzers | inchangés (`hue-bridge.md` §5.2, `LED_SET_PROTOCOL.md`) |
| Geste direct de choix d'effet depuis le popover | non prévu |

---

## 11. Édition, import et export des effets (v11.8.0 — #210, #305) — révision 5

> Décisions utilisateur du 2026-11-12 (GATE 1.5, `_work/reports/plan-ambiguities-20261112-100000.md`) :
> **E1** l'éditeur est une grille **pas × cibles** (colonnes PRE, pas 0…n, DERNIER, POST ; lignes `g0`, `g1`…`g12`,
> précédées d'une ligne par variable déclarée), avec un panneau de déclaration des variables et un onglet
> **Texte** (le fichier YAML, validé en direct) ; **E2** un effet natif est en **lecture seule** : on le duplique ;
> l'interface et l'import n'écrivent jamais un fichier portant l'`id` d'un natif ; **E3** un enregistrement depuis la
> grille **régénère** le fichier (commentaires `#` perdus, avertissement) ; l'import et l'onglet Texte écrivent le
> texte **tel quel** ; **E4** l'aperçu de l'éditeur est joué sur plusieurs tailles de salle à la fois.
>
> Décisions du GATE 2 (2026-11-12, `plan-ambiguities-20261112-103000.md` et `-104000.md`) : **E5** la grille n'a **pas de
> ligne « voisin »** (`gA-1`, `gA+1`) : un voisin lu dans un fichier est **converti en variable** à l'ouverture (§11.2) ;
> les voisins restent valides dans le fichier et dans l'onglet Texte (révision 7 : la variable créée **suit** l'autre,
> `gB: gA-1`) ; **E6** les variables sont affichées **en haut** de
> la grille mais écrites **après** `g0` et les groupes ; **E7** tout essai sur les ampoules (catalogue, éditeur)
> **demande la zone** ; l'essai de l'éditeur joue le **brouillon**, sans enregistrer (§11.10).
>
> Le format de fichier évolue à la marge (**§12**, `version: 2` conservée). Le serveur reste le **seul** analyseur, le seul
> validateur et le seul interprète : le navigateur n'analyse jamais de YAML et ne calcule aucune trame.
> Les routes de ce chapitre ont la même protection que les autres routes `/api/lighting/*` (aucune de plus).

### 11.1 Document d'effet — forme JSON d'échange

Forme JSON d'un effet, produite et relue par le serveur (`effects.Document`). Les noms sont ceux de l'API (§7.1).

```json
{
  "id": "mon-chenillard", "name": "Mon chenillard", "description": "Un point fait le tour de la zone.",
  "moments": ["chrono"], "min_groups": 3, "tick_ms": 1000, "loop": true,
  "variables": [
    { "name": "gA", "kind": "progression", "start": "N/2", "step": "+1" },
    { "name": "gB", "kind": "progression", "start": "gA+1", "step": null },
    { "name": "gC", "kind": "list", "items": ["g1", "g4", "g3"] }
  ],
  "pre":   [ { "target": "g0", "color": "theme", "intensity": 100 } ],
  "steps": [ [ { "target": "g0", "color": "theme", "intensity": 100 },
               { "target": "gA-1", "color": "theme", "intensity": 50 },
               { "target": "gA", "color": "urgence", "intensity": 100 } ] ],
  "last":  null,
  "post":  [ { "target": "g2", "off": true } ]
}
```

| Champ | Fichier (§4) | Règle |
|---|---|---|
| `id`, `name`, `description` | `id`, `nom`, `description` | bornes du §4 |
| `moments`, `min_groups`, `tick_ms`, `loop` | `moments`, `groupes_min`, `duree_tick_ms`, `boucle` | `min_groups` 0 = clé absente ; `loop` true = défaut |
| `variables[]` | `variables` | **ordre du tableau = ordre d'écriture**. `kind: "progression"` : `start` et `step` sont des **expressions en texte** (§12.2), `step` = `null` quand la déclaration n'a pas de pas (« suit », §12.2). `kind: "list"` : `items[]` (`"g1"`…`"g12"`). `[]` = aucune |
| `pre`, `last`, `post` | `pre`, `dernier`, `post` | tableau d'actions, ou `null` = pas absent |
| `steps[][]` | `pas` | 1 à 200 pas, chacun 1 à 40 actions, **dans l'ordre d'application** |
| action | `cible: [couleur, intensité]` | `target` écrit comme dans le fichier (`g0`, `g3`, `g*`, `gA`, `gA+1`, `gA-2`) ; `color` = `"#RRGGBB"` ou `theme` / `equipe` / `urgence` / `scene` ; `intensity` 0-100. `[eteint]` ⇔ `{"target": …, "off": true}` (sans `color` ni `intensity`) |

- `version` n'est pas dans le document : le serveur écrit toujours `version: 2`.
- Tous les champs sont toujours présents dans une réponse (`variables: []`, `pre: null`…), jamais omis.
- **Aller-retour garanti** : pour tout effet valide `e`, `Parse(Serialize(DocumentOf(e)))` rend le même effet
  (mêmes trames pour tout N de 1 à 12). Test exigé sur chaque natif et sur les fichiers d'exemple (§11.9).

**Ordre canonique des actions d'un pas** (celui qu'écrit la grille) : `g0` ; puis `g1`…`g12` par numéro croissant ; puis
`g*` ; puis les variables **dans l'ordre de `variables`** — la dernière déclarée est écrite en dernier, donc gagne
quand deux variables désignent le même groupe. Une variable repeint toujours le fond et les groupes fixes. (Un voisin,
qui ne peut venir que d'un fichier ou de l'onglet Texte, se range juste avant sa variable, par décalage croissant.)
`custom_order` (§11.2) est vrai quand au moins un pas du fichier n'est pas dans cet ordre.

### 11.2 `GET /api/lighting/effects/{id}/source`

Ouvre un effet du catalogue dans l'éditeur.

```json
{ "id": "chenillard", "origin": "native", "file": "natifs/chenillard.yaml", "read_only": true,
  "yaml": "# Chenillard — …\nid: chenillard\n…", "effect": { … document §11.1 … },
  "has_comments": true, "custom_order": false,
  "grid": { "editable": true, "effect": { … document sans voisin … },
            "converted": [ { "from": "gA-1", "to": "gB" }, { "from": "gA+1", "to": "gC" } ], "blocked": [] } }
```

**`grid` — le document tel que la grille l'affiche (E5).** `effect` (ci-dessus) est le document fidèle au fichier ;
`grid.effect` est le même effet **sans aucune cible voisine** dans ses pas : chaque voisin `gX+m` / `gX-m` est remplacé
par une variable **qui suit** `gX` — déclaration `gY: gX+m` (sans pas, §12.2), exacte dans tous les cas, que `gX` soit
une avance ou une suite de groupes. *(Révision 7 : remplace la table de conversion par départ décalé ; une cible `g*`
n'est plus convertie, elle a sa ligne dans la grille.)*

- Nom de la variable créée : la première lettre libre de `gA` à `gZ`. Un même voisin utilisé dans plusieurs pas donne
  une seule variable. Les variables créées sont insérées **juste avant** leur variable d'origine, par décalage
  croissant : la variable d'origine reste écrite après elles et continue de gagner (ordre canonique, §11.1).
- Seul cas non convertible : la conversion ferait dépasser 24 variables.
- **Garantie** : `grid.effect` rend les mêmes trames que `effect` pour tout N de 1 à 12 (test exigé, §11.9). Avec la
  règle de position du §4.4, le voisin à `m` rangs d'une progression est exactement la progression partie de `m` rangs
  plus loin.
- `converted[]` liste les remplacements (pour le message de l'interface) ; vide s'il n'y avait aucun voisin (alors
  `grid.effect` = `effect`).
- Un voisin non convertible ⇒ `grid.editable: false`, `grid.effect: null`, `blocked[]` = les cibles en cause : la grille
  est affichée en lecture seule d'après `effect`, l'effet se modifie dans l'onglet Texte.
- Rien n'est écrit : le fichier n'est réécrit (sans voisin) qu'à un enregistrement fait depuis la grille.

- `yaml` : le texte du fichier tel qu'il est lu (natif : le texte embarqué, **sans** l'en-tête de la copie de référence).
- `read_only` : vrai pour `origin` = `native` **et** `override` (E2 : l'interface n'écrit jamais un `id` de natif).
- `has_comments` : le texte contient au moins un commentaire `#`. `custom_order` : §11.1.
- `404` effet inconnu.

### 11.3 `POST /api/lighting/effects/draft` — valider, convertir et prévisualiser un brouillon

Rien n'est écrit, rien n'est joué. Sert à la fois à la validation en direct, au passage grille ↔ texte et à l'aperçu
multi-tailles. Corps (64 Ko au plus) — **exactement une** des deux clés `effect` ou `yaml` :

```json
{ "effect": { … document §11.1 … },        // brouillon de la grille
  "yaml": "id: mon-effet\n…",              // ou brouillon de l'onglet Texte
  "groups": [1, 3, 6, 12, 4],              // tailles de salle à prévisualiser : 0 à 8 valeurs distinctes de 1 à 12
  "moment": "chrono" }                      // facultatif ; défaut : premier moment déclaré
```

Réponse `200` dans tous les cas où le corps JSON est lisible :

```json
{ "ok": true, "errors": [],
  "effect": { … document … }, "yaml": "id: mon-effet\nnom: …", "has_comments": false, "custom_order": false,
  "grid": { "editable": true, "effect": { … }, "converted": [], "blocked": [] },
  "moment": "chrono",
  "previews": [ { … réponse du §7.3 pour groups = 1 … }, { … groups = 3 … }, … ] }
```

- **Validation** : celle du §4, par le même code que le chargement des fichiers. Brouillon invalide ⇒ `ok: false`,
  `errors[]` (`line` — 0 pour un brouillon `effect`, la ligne du texte pour un brouillon `yaml` — et `message` du §4.8,
  sans nom de fichier), `previews: []` ; `effect` est `null` si le texte n'est pas analysable, `yaml` est `""` si le
  document n'est pas sérialisable. Pour un brouillon `effect`, chaque erreur porte en plus `path` quand elle vise un
  endroit précis (`"steps[2]"`, `"variables[0]"`, `"pre"`, `"tick_ms"`…), pour que la grille la place.
- **`grid`** : même objet qu'au §11.2, calculé sur le brouillon (pour le passage Texte → Grille). `null` si `ok: false`.
- **Conversion** : `effect` = le document du brouillon (analysé depuis `yaml`, ou renvoyé normalisé) ; `yaml` = le texte
  **régénéré** depuis le document (§11.4), toujours — c'est le texte qu'afficherait l'onglet Texte après une
  modification dans la grille. `has_comments` et `custom_order` décrivent le brouillon `yaml` reçu (faux pour `effect`).
- **Aperçus** : un élément par valeur de `groups`, dans l'ordre demandé, chacun de la forme du §7.3 (`id` = l'`id` du
  brouillon, mêmes règles : une trame par tick, `frames: []` si aucun pas visible, calculé même sous `groupes_min`).
  `moment` non déclaré par le brouillon ⇒ `ok: false` avec l'erreur correspondante.
- L'`id` du brouillon n'a pas à être libre : l'unicité n'est vérifiée qu'à l'enregistrement.
- `400` : JSON illisible, les deux clés ou aucune, `groups` hors bornes. `413` : corps trop gros.

**Champ additif des trames (§7.3 et ici)** — `vars` : pour chaque variable déclarée, le **numéro réel** du groupe
qu'elle désigne à ce tick (`0` = aucun) : `"vars": { "gA": 3, "gB": 2 }`. Objet vide `{}` sans variable. Ajouté aussi à
`GET /api/lighting/effects/{id}/preview` (additif, sans effet sur le catalogue v11.7.0).

### 11.4 Enregistrer — `PUT /api/lighting/effects/{id}`

Corps (64 Ko au plus) : `{"effect": {…}}` **ou** `{"yaml": "…"}`, et `"create": true` pour une création.

| Cas | Résultat |
|---|---|
| `effect` | le fichier est **régénéré** : clés dans l'ordre du §4.1, un pas par ligne en écriture `{ cible: [couleur, intensité] }`, `description` en bloc `>`, sans commentaire, fin de ligne `\n`, UTF-8 |
| `yaml` | le texte est écrit **tel quel** (commentaires conservés), après validation |
| `create: true` et `id` déjà au catalogue (natif ou utilisateur) | `409 {"reason": "exists"}` |
| `create` absent et `id` inconnu | `404` |
| `id` d'un natif (y compris un natif surchargé) | `409 {"reason": "native"}` — rien n'est écrit (E2) |
| `id` du corps ≠ `{id}` du chemin | `400` (l'identifiant ne se change pas : dupliquer) |
| `id` réservé : mot de route (`reload`, `try`, `draft`, `import`) ou nom de périphérique Windows (`con`, `prn`, `aux`, `nul`, `com1`-`com9`, `lpt1`-`lpt9`) — §4.8 | `400` avec `errors[]` |
| texte `yaml` contenant un alias YAML (`*nom`) — §4.8 | `400` avec `errors[]` (ligne de l'alias) — rien n'est écrit |
| brouillon invalide | `400 {"ok": false, "errors": [...]}` (forme du §11.3) — rien n'est écrit |
| 100 fichiers atteints (création) | `409 {"reason": "limit"}` |

- **Fichier écrit** : celui qui définit déjà cet `id` (son nom est conservé) ; sinon `<id>.yaml` ; si ce nom est pris
  par un fichier qui ne définit pas cet `id` (autre effet, fichier invalide), `<id>-2.yaml`, `<id>-3.yaml`…
- **Écriture atomique** (fichier temporaire dans le même dossier — nom ne finissant pas par `.yaml` — puis renommage),
  jamais dans `natifs/`, jamais hors de `data/files/effects/`.
- Rechargement immédiat du catalogue (comme §7.2) avant de répondre : `200 {"ok": true, "file": "mon-effet.yaml",
  "catalogue": { … réponse du §7.1 … }}`. Un effet en cours de lecture repart avec sa nouvelle définition (§3.3).
- Un `PUT` sur un fichier modifié entre-temps à la main l'écrase : dernier écrit gagne (pas de verrou).

### 11.5 Supprimer — `DELETE /api/lighting/effects/{id}`

- `origin: user` : le fichier est supprimé ; `origin: override` : le fichier utilisateur est supprimé, **le natif
  reprend** (« Revenir à l'effet d'origine ») ; `origin: native` : `409 {"reason": "native"}`. `404` effet inconnu.
- Rechargement immédiat ; `200 {"ok": true, "catalogue": {…}}`.
- Les réglages qui citaient l'effet ne sont **pas** modifiés : le moment joue sa valeur par défaut et l'écart figure
  dans `unresolved[]` (§3.3, §7.1). C'est l'interface qui prévient avant de supprimer (« utilisé : … »).

### 11.6 Télécharger — `GET /api/lighting/effects/{id}/download` (#305)

Le texte du fichier (celui de `yaml` au §11.2), pour tout effet du catalogue, natifs compris.
`Content-Type: application/yaml; charset=utf-8`, `Content-Disposition: attachment; filename="<id>.yaml"`. `404` inconnu.

### 11.7 Importer — `POST /api/lighting/effects/import` (#305)

`multipart/form-data` : champ `files` répété (1 à 20 fichiers, `.yaml` ou `.yml`, 64 Ko chacun, 2 Mo au total), champ
facultatif `on_conflict` = `ask` (défaut) | `replace` | `keep_both`. Chaque fichier est traité seul ; un fichier refusé
n'empêche pas les autres. Réponse `200` :

```json
{ "results": [
    { "file": "aller-retour.yaml", "status": "imported", "id": "aller-retour", "name": "Aller-retour", "stored": "aller-retour.yaml" },
    { "file": "mon-chenillard.yaml", "status": "conflict", "id": "mon-chenillard", "name": "Mon chenillard", "existing": { "name": "Mon chenillard", "file": "mon-chenillard.yaml" } },
    { "file": "pulsation.yaml", "status": "native", "id": "pulsation", "errors": [ { "line": 2, "message": "« pulsation » est l'identifiant d'un effet livré : changez la clé id" } ] },
    { "file": "casse.yaml", "status": "invalid", "errors": [ { "line": 12, "message": "la variable « gB » n'est pas déclarée dans variables" } ] } ],
  "catalogue": { … réponse du §7.1 … } }
```

| `status` | Sens | Écrit |
|---|---|---|
| `imported` | nouvel effet | oui, **tel quel** |
| `replaced` | `on_conflict: replace` : remplace l'effet utilisateur de même `id` (dans son fichier) | oui, tel quel |
| `renamed` | `on_conflict: keep_both` : importé sous un `id` suffixé (`<id>-2`, `-3`… le premier libre, tronqué à 40) ; `new_id` le donne. **Seule la valeur de la clé `id` est réécrite** dans le texte ; le reste, commentaires compris, est intact | oui |
| `conflict` | `on_conflict: ask` et un effet **utilisateur** de même `id` existe : l'interface demande, puis renvoie ce seul fichier avec `replace` ou `keep_both` | non |
| `native` | l'`id` est celui d'un effet livré (E2) — quel que soit `on_conflict` | non |
| `invalid` | validation du §4 en échec (dont format `version: 1`, YAML illisible), **alias YAML**, `id` réservé (mot de route ou nom de périphérique Windows) — §4.8 ; `errors[]` : `line`, `message` | non |
| `too_big` | plus de 64 Ko | non |
| `limit` | 100 fichiers atteints | non |

- Nom du fichier écrit (`stored`) : règle du §11.4 (`<id>.yaml`…) ; le nom du fichier envoyé ne sert qu'aux messages
  (il n'est jamais utilisé comme chemin). Deux fichiers du même envoi avec le même `id` : le second est un `conflict`
  (ou suit `on_conflict`).
- Écriture atomique, rechargement **une fois** en fin de traitement. `400` : envoi illisible, aucun fichier, plus de 20
  fichiers, extension refusée. `413` : plus de 2 Mo.

### 11.8 Comportement de l'éditeur (normatif pour l'interface)

- **Deux onglets, une seule source** : *Grille* (document) et *Texte* (YAML). Le brouillon est envoyé à §11.3 après
  chaque modification (temporisation de 300 ms) : erreurs, aperçus et conversion viennent de la réponse.
  - Grille → Texte : si la grille a été modifiée depuis la dernière synchronisation, le texte devient le `yaml`
    régénéré (commentaires perdus : avertissement si `has_comments`) ; sinon le texte est inchangé.
  - Texte → Grille : possible seulement si le texte est valide (`ok: true`) ; la grille prend `effect`.
  - **Enregistrer** : envoie `yaml` si le texte est la dernière source modifiée (ou si rien n'a été modifié dans la
    grille), `effect` sinon.
  - La grille travaille sur `grid.effect` (§11.2) : Texte → Grille est refusé aussi quand `grid.editable` est faux.
- **Grille** : une case vide = le groupe garde son état (aucune action). **Ordre d'affichage des lignes ≠ ordre
  d'écriture** : les lignes des variables sont **en haut**, dans l'ordre de `variables`, puis `g0`, les groupes numérotés, `g*` ;
  les actions d'un pas sont écrites dans l'ordre canonique (§11.1 : fond, groupes, `g*`, puis variables).
- **Groupes de la grille** *(révision 7)* : la grille affiche `n` lignes de groupes, **6 par défaut** (`g1`…`g5` puis
  `g*`) ; **la dernière ligne est toujours `g*`** et écrit la cible `g*` ; les autres écrivent `g<numéro>`.
  - « Ajouter un groupe » (jusqu'à 12 lignes) : dans tous les pas, les actions de la ligne `g*` sont **renommées**
    `g<n>` (la ligne garde son contenu et devient un groupe numéroté) ; une nouvelle ligne `g*` vide apparaît.
  - « Retirer un groupe » (au moins 1 ligne) : les actions de la ligne `g*` sont **supprimées** (confirmation si elle
    n'est pas vide), puis celles de la ligne `g<n-1>` sont renommées `g*`.
  - Le nombre de lignes **n'est pas enregistré** dans l'effet. À l'ouverture : `max(5, plus grand g<k> utilisé)` lignes
    numérotées, plus la ligne `g*`. Un effet qui écrit `g12` (natif `degrade`) s'ouvre donc avec `g1`…`g12` et la ligne
    `g*` (13 lignes) ; « Ajouter un groupe » est alors indisponible.
  - Les salles de l'aperçu (1, 3, 6, 12 et zones réelles) sont indépendantes du nombre de lignes.
- **Pas de ligne « voisin »** (E5) : la grille ne crée jamais de cible `gA+1` / `gA-1`. Un décalage se fait par une
  autre variable. À l'ouverture, `grid.converted` non vide ⇒ message « les voisins … sont remplacés par les variables
  … : même rendu » ; `grid.editable` faux ⇒ grille en lecture seule, modification par l'onglet Texte.
- **Variables** : déclarées dans le panneau, une ligne par variable — nom proposé (`gA`…`gX`, 24 au plus), type
  **« Avance régulière »** (`progression` : champs **départ** et **pas**, des expressions du §12.2 avec propositions `1`, `N/2`,
  `N`, `gA+1` ; pas vide = « suit ») ou **« Suite de groupes »** (`list`). Les erreurs d'expression viennent de `draft`
  (`path` : `variables[i].start`, `variables[i].step`).
  *(révision 6)* L'explication et la **suite obtenue pour 1, 3, 6 et 12 groupes** — lue dans `vars` des aperçus du
  brouillon, jamais calculée par le navigateur — ne sont affichées que **dans une bulle** ouverte par un bouton « ? ». Une variable déclarée ajoute
  sa ligne ; les variables se réordonnent (l'ordre décide laquelle gagne) ; supprimer une variable retire sa ligne et
  ses actions.
- **Pinceau** : info-bulle sur chaque couleur et sur l'intensité (définitions du §4.2) ; couleur libre choisie sur une
  roue teinte / saturation avec luminosité, champ `#RRGGBB`, aperçu et couleurs récentes (composant maison, aucune
  dépendance) ; les couleurs récentes sont mémorisées par navigateur.
- **Pas à pas** *(révision 6)* : l'aperçu de l'éditeur a lecture / pause, tick précédent, tick suivant, retour au début
  et une réglette de position ; un clic sur l'en-tête d'un pas de la grille place l'aperçu sur le premier tick où ce
  pas est joué. À l'arrêt, tick, pas joué et `vars` restent affichés. Aucune route nouvelle : l'interface se déplace
  dans les trames déjà reçues (§11.3). L'aperçu des cartes du catalogue garde lecture / arrêt seulement.
- **Aperçu** : tailles demandées = `1, 3, 6, 12`, plus le nombre de groupes de chaque zone réelle qui n'est pas déjà
  dans la liste (zones existantes seulement) ; toutes les rangées sont jouées **sur la même horloge**. Sous `groupes_min`,
  la rangée porte « sous le minimum : non joué en salle ». Le pas joué est surligné dans la grille ; `vars` est affiché.
- **Essai sur les ampoules** (E7) : l'éditeur a un bloc « Essayer sur la salle » avec le **choix de la zone** (zones
  existantes seulement) ; il joue le **brouillon courant**, sans enregistrer (§11.10). Le bouton « Essayer sur la salle »
  du catalogue demande lui aussi la zone et l'envoie à la route existante (§7.4, champ `zone`). La dernière zone
  choisie est retenue par navigateur.
- Effet `read_only` : grille et texte en lecture ; « Dupliquer », « Télécharger » et « Essayer sur la salle » seulement.
- **Dupliquer** : ouvre l'éditeur sur une copie (`id` proposé `<id>-copie`, libre), créée par `PUT … create: true`.

### 11.10 Essayer un brouillon — `POST /api/lighting/effects/draft/try` (E7)

Joue un brouillon **non enregistré** sur les ampoules d'une zone. Corps : celui du §11.3 sans `groups` ni `moment`
(`effect` **ou** `yaml`), plus les champs du §7.4 : `{"effect": {…}, "zone": "animateur", "speed": 1, "seconds": 10}`.

- Mêmes règles, mêmes valeurs par défaut, mêmes réponses et mêmes refus que `POST /api/lighting/effects/{id}/try`
  (§7.4) : `200 {"ok": true, "duration_ms": …}`, `409` avec `reason` (`disabled`, `game`, `not_applicable`, `mode`),
  `503`. L'applicabilité (§4.7) est calculée sur le brouillon et la zone demandée.
- Brouillon invalide ⇒ `400 {"ok": false, "errors": [...]}` ; rien n'est joué.
- Rien n'est écrit sur le disque, le catalogue n'est pas modifié ; l'`id` du brouillon peut être celui d'un effet
  existant (c'est le brouillon qui est joué, pas l'effet enregistré).
- Arrêt : route existante `POST /api/lighting/effects/try/stop`. Un essai en remplace un autre, quel qu'en soit l'origine.
- L'identifiant `draft` étant réservé (§11.4), `…/draft/try` ne peut pas désigner un effet du catalogue.

### 11.9 Tests exigés (v11.8.0)

- Aller-retour §11.1 sur les 5 natifs, sur l'exemple du §4.5 et sur un effet à liste, voisins, `[eteint]`, `pre`,
  `dernier`, `post`, `boucle: non` : mêmes trames pour N = 1, 3, 6, 12.
- Sérialisation : texte régénéré stable (deux enregistrements successifs = même fichier), relu sans erreur.
- `draft` : chaque erreur du §4.8 remonte avec `line` (brouillon `yaml`) ou `path` (brouillon `effect`) ; `previews`
  dans l'ordre de `groups` ; `vars` conforme au chronogramme du §4.5 ; rien n'est écrit sur le disque.
- `PUT` : création, modification (nom de fichier conservé), `exists`, `native` (natif et natif surchargé), `id`
  réservé, `id` différent du chemin, invalide (fichier inchangé), limite de 100, écriture atomique (aucun `.yaml`
  partiel visible du chargeur), catalogue renvoyé à jour, texte `yaml` écrit à l'octet près.
- `DELETE` : utilisateur, surcharge (retour au natif), natif refusé ; réglages inchangés et `unresolved[]` renseigné.
- `download` : contenu identique au fichier, en-têtes ; natif sans l'en-tête de la copie de référence.
- `import` et `PUT` : alias YAML refusé avec sa ligne (alias simple, alias imbriqués), ancre seule acceptée ; `id`
  `con`, `nul`, `com1`, `lpt9` refusés avec leur message, `console` et `com10` acceptés ; `keep_both` ne propose jamais
  un identifiant réservé.
- `import` : chacun des `status` ; `keep_both` ne change que la clé `id` ; nom envoyé hostile (`../x.yaml`, chemin
  absolu, nom de périphérique Windows) jamais utilisé comme chemin ; rien n'est écrit hors de `data/files/effects/` ni
  dans `natifs/` ; YAML à ancres / alias imbriqués refusé sans explosion mémoire.
- `grid` (E5) : `chenillard-traine` converti en `gB: gA-1`, `gC: gA+1`, `gA` déclarée en dernier ; voisin d'une suite
  de groupes converti de même ; **mêmes trames**
  que l'original pour N = 1 à 12 ; dépassement de 24 variables ⇒ `editable: false` ;
  effet sans voisin ⇒ `grid.effect` identique à `effect`.
- `draft/try` : joue le brouillon sur la zone demandée sans rien écrire ; mêmes refus que §7.4 ; brouillon invalide `400`.
- Non-régression : catalogue, aperçu, essai et rechargement de v11.7.0 inchangés (hors champ additif `vars`).

---

## 12. Évolutions du format en v11.8.0 — révision 7 (#210)

> Décisions utilisateur du 2026-11-12 (GATE 2) : `_work/reports/plan-ambiguities-20261112-111000.md` (**E8** dernier
> groupe = `g*`, `gN` devient un nom de variable ; **E9** 24 variables, limite d'actions relevée) et
> `plan-ambiguities-20261112-113000.md` (**E10** départ et pas en **expressions**, division **entière**, une variable
> peut en **suivre** une autre ou **partir** d'elle ; **E11** `gN` non déclaré = **refus** ; **E12** grille de l'éditeur à
> nombre de groupes réglable, dernière ligne = `g*`, §11.8).
> Ce chapitre **amende** les §4.1 à §4.8 ; tout ce qu'il ne cite pas est inchangé. `version` reste `2`.

### 12.1 Ce qui change

| Sujet | v11.7.0 (§4) | v11.8.0 |
|---|---|---|
| Dernier groupe non vide de la zone | `gN` | **`g*`** — cible (`g*: [theme, 100]`) ; dans une expression, `g*` vaut `N` |
| Nom d'une variable | `g` + majuscule **autre que N**, puis lettres ou chiffres | `g` + **une majuscule quelconque**, puis lettres ou chiffres : `gN` est une variable ordinaire |
| `gN` sans variable `gN` déclarée | le dernier groupe | **refusé** (§12.4) |
| Déclaration d'une progression | `gA: g<k>; <entier>` ou `gA: gN; <entier>` | `gA: <départ>` ou `gA: <départ>; <pas>` — **expressions** (§12.2) |
| Variables par effet | 8 | **24** |
| Actions par pas | 16 | **40** (une par cible possible : `g0`, 12 groupes, `g*`, 24 variables = 38, arrondi) |

- `g*` désigne, dans la zone où l'effet est joué, le **dernier groupe non vide** : `g3` dans une zone de 3 groupes, `g6`
  dans une zone de 6. Avec N = 0, toute action sur `g*` est ignorée. Ne pas confondre avec `g0` (toute la zone).
- `g*` s'écrit sans guillemets dans le YAML (`{ g*: [theme, 100] }`).
- Cibles d'un pas : inchangées par ailleurs (`g0`, `g1`…`g12`, `g*`, une variable, le voisin d'une variable `gA+1`).
  `g*+1` n'est pas une cible. **Une cible n'est jamais une expression** : les expressions ne s'écrivent que dans
  `variables`.
- Les listes (`gC: [g1, g4, g3]`) sont inchangées.
- Limites inchangées : 200 pas, 64 Ko par fichier, 100 fichiers, 12 groupes par zone. Les 5 effets natifs ne changent
  pas (aucun n'emploie `gN` ; leurs déclarations `g1; +1` restent valides telles quelles).

### 12.2 Expressions du départ et du pas — normatif

```
déclaration := départ                      # sans pas : « suit »
             | départ ";" pas              # avec pas : « avance »
départ, pas := expression
expression  := terme { ("+" | "-") terme }
terme       := facteur { ("*" | "/") facteur }
facteur     := entier | "N" | "g*" | "g" entier | variable | "(" expression ")" | ("+" | "-") facteur
```

| Élément | Vaut |
|---|---|
| entier (0 à 999) | lui-même |
| `N` | le nombre de groupes **non vides** de la zone où l'effet est joué |
| `g*` | `N` (le rang du dernier groupe) |
| `g1`…`g12` | le nombre écrit (`g3` = 3) — écriture v11.7.0, conservée |
| `gA` (variable) | le **rang** (1…N) du groupe que désigne cette variable ; **dans le départ seulement** — le pas ne cite jamais de variable |
| `+ - *` | arithmétique entière |
| `/` | **division entière** : le quotient est **tronqué** (`5/2` = 2, `2/3` = 0, `-5/3` = −1) |

- Espaces libres. 60 caractères au plus par expression. Un signe en tête est admis (`+1`, `-N/3`).
- **Rang obtenu → groupe** : la valeur `v` d'une expression désigne le rang `((v − 1) mod N) + 1`, avec un reste toujours
  positif — règle de rebouclage du §4.4, étendue aux valeurs nulles et négatives : `N+1` ⇒ rang 1, **`0` ⇒ rang N (le
  dernier groupe)**, `−1` ⇒ rang N−1. Les rangs sont ceux des groupes non vides (§4.3).
- **Valeur d'une variable au cran `k`** (le cran est celui du §4.5, inchangé) :
  - **avec pas** — `gA: D; P` : rang de `D(0) + P × k`. `D(0)` est le départ évalué au cran 0 : une variable citée y
    vaut sa position **de départ**. `P` est évalué une fois (il ne dépend que de `N`).
  - **sans pas** — `gA: D` : rang de `D(k)`, le départ **réévalué à chaque cran** : une variable citée y vaut sa position
    **au cran `k`**. `gB: gA+1` est donc toujours le groupe après `gA` ; `gE: N/2` (aucune variable citée) désigne un
    groupe fixe.
- **Dépendances** : une variable peut en citer une autre, déclarée avant ou après elle, progression ou liste. Une
  boucle (`gA: gB+1`, `gB: gA+1`, ou `gA: gA+1`) est une **erreur de validation**. L'ordre d'évaluation suit les
  dépendances ; l'ordre de déclaration ne sert qu'à l'ordre d'écriture (§11.1).
- **Sans valeur** : une variable ne désigne aucun groupe — et les actions qui la visent sont ignorées — quand N = 0,
  quand elle cite une liste vide pour cette zone, ou quand une division par zéro survient **à l'exécution**
  (`6/(N-3)` avec N = 3). Une division par zéro écrite en dur (`N/0`) est une erreur de validation.
- **Cas à zéro** (division entière) : `N/2` avec N = 1 vaut 0 ⇒ rang 1 (le seul groupe) ; `N/3` avec N = 2 vaut 0 ⇒
  rang 2 (le dernier groupe) ; **un pas qui vaut 0 n'avance pas** (`gA: 1; N/3` avec N = 2 reste sur `g1`).
- Écritures v11.7.0 : `gA: g1; +1`, `gB: g2; +2`, `gE: g3; 0` gardent exactement leur sens. L'ancien `gD: gN; -1`
  devient `gD: g*; -1` (ou `N; -1`).
- Le pas constant n'est plus borné à ±12 : il reboucle comme le reste.

### 12.3 Messages de validation (complète §4.8)

Chaque message porte fichier et ligne (ou `path` pour un brouillon de l'éditeur, §11.3) :

- « « gN » n'est pas déclarée dans variables. gN est désormais une variable comme les autres : pour le dernier groupe,
  écrivez g* » (§12.4) ;
- « la variable « gB » n'est pas déclarée dans variables » (cible ou expression) ;
- « l'expression « gA+ » est incomplète », « l'expression « N//2 » est mal écrite », « parenthèse non fermée » ;
- « division par zéro dans « N/0 » » ;
- « les variables gA et gB dépendent l'une de l'autre » ;
- « le pas ne peut pas citer une variable (gA) » ;
- « 24 variables au plus », « un pas compte de 1 à 40 actions » ;
- cible mal écrite : « … (g0, g1…, g*, ou une variable comme gA) ».

### 12.4 Compatibilité — **[BREAKING] pour un fichier utilisateur v11.7.0 qui emploie `gN`**

| Écriture v11.7.0 | En v11.8.0 |
|---|---|
| `gA: g1; +1`, `gB: g2; +2`, `gE: g3; 0`, listes, voisins | **inchangé**, même rendu |
| `gN: [theme, 100]` (cible) sans variable `gN` | **refusé** — message du §12.3 |
| `gD: gN; -1` (départ) sans variable `gN` | **refusé** — même message |
| incrément au-delà de ±12 | accepté (refusé en v11.7.0) |

- Le fichier refusé est **écarté** comme tout fichier invalide (§3.3) : il figure dans `errors[]` et dans le bandeau du
  catalogue avec son nom, sa ligne et le message ; les autres effets restent chargés ; un réglage qui le citait joue
  la valeur par défaut du moment (`unresolved[]`). **Correction** : remplacer `gN` par `g*`.
- **Aucun effet natif n'est concerné.** Le cas ne touche qu'un fichier écrit à la main entre la v11.7.0 et la v11.8.0.
  À annoncer dans les notes de version et dans `docs/LIGHT_EFFECTS.md`.
- Dans l'autre sens, un fichier v11.8.0 qui utilise `g*`, une expression, une variable en `N`, plus de 8 variables ou
  plus de 16 actions par pas est refusé par un serveur v11.7.0 (message de validation).

### 12.5 Exemples — fichiers et chronogrammes

**Exemple A — départ au milieu, variable qui suit, pas relatif.**

```yaml
id: milieu-et-suiveur
nom: Milieu et suiveur
version: 2
moments: [chrono]
duree_tick_ms: 1000
variables:
  gB: gA+1            # sans pas : toujours le groupe juste après gA
  gC: N; -N/3         # part du dernier groupe, recule d'un tiers de zone par pas
  gA: N/2; +1         # part du milieu (division entière), avance d'un groupe par pas
pas:
  - { g0: [theme, 100], gB: [theme, 50], gC: ["#14B8A6", 100], gA: [urgence, 100] }
```

Pas de `pre` : le pas 0 est joué au tick 0, et le cran `k` vaut le numéro du tick.

| Zone | `N/2` | `N/3` | gA (`N/2; +1`) aux ticks 0, 1, 2… | gB (`gA+1`) | gC (`N; -N/3`) |
|---|---|---|---|---|---|
| **5 groupes** | 2 | 1 | g2, g3, g4, g5, g1, g2 | g3, g4, g5, g1, g2, g3 | g5, g4, g3, g2, g1, g5 |
| **6 groupes** | 3 | 2 | g3, g4, g5, g6, g1, g2 | g4, g5, g6, g1, g2, g3 | g6, g4, g2, g6, g4, g2 |
| **2 groupes** | 1 | **0** | g1, g2, g1, g2 | g2, g1, g2, g1 | **g2, g2, g2, g2** (pas nul : ne bouge pas) |
| **1 groupe** | **0** ⇒ rang 1 | 0 | g1, g1, g1 | g1, g1, g1 | g1, g1, g1 |

Détail pour 5 groupes :

| Tick | Joué | k | gA | gB | gC | État |
|---|---|---|---|---|---|---|
| 0 | pas 0 | 0 | g2 | g3 | g5 | fond au thème ; g3 à 50 % ; g5 turquoise ; g2 à la couleur d'urgence |
| 1 | pas 0 | 1 | g3 | g4 | g4 | g4 : `gB` puis `gC` écrivent le même groupe, **`gC` gagne** (écrite après) ; g3 urgence |
| 2 | pas 0 | 2 | g4 | g5 | g3 | g5 à 50 % ; g3 turquoise ; g4 urgence |
| 3 | pas 0 | 3 | g5 | g1 | g2 | g1 à 50 % ; g2 turquoise ; g5 urgence |
| 4 | pas 0 | 4 | g1 | g2 | g1 | g2 à 50 % ; g1 : `gC` puis `gA`, **`gA` gagne** (urgence) |

**Exemple B — partir d'une autre variable, puis avancer seule ; `gN` comme variable ; `g*` comme cible.**

```yaml
id: croisement
nom: Croisement
version: 2
moments: [chrono]
groupes_min: 3
duree_tick_ms: 1000
variables:
  gA: 1; +1           # part du premier groupe, avance
  gN: g*; -1          # « gN » est ici une variable comme une autre : part du dernier groupe, recule
  gD: gA+1; +2        # AVEC pas : part du groupe après le départ de gA (g2), puis avance de 2
pre:
  { g0: [theme, 100], g*: [scene, 100] }     # au départ, le dernier groupe à la couleur de la scène
pas:
  - { g0: [theme, 100], gA: [urgence, 100], gN: [urgence, 100], gD: [theme, 40] }
```

Zone de **4 groupes** (`g*` = `g4`) — `gD` avec pas : `D(0) + 2k` = 2, 4, 6, 8… ⇒ g2, g4, g2, g4 :

| Tick | Joué | k | gA | gN | gD |
|---|---|---|---|---|---|
| 0 | PRE | 0 | g1 | g4 | g2 |
| 1 | pas 0 | 0 | g1 | g4 | g2 |
| 2 | pas 0 | 1 | g2 | g3 | g4 |
| 3 | pas 0 | 2 | g3 | g2 | g2 |
| 4 | pas 0 | 3 | g4 | g1 | g4 |
| 5 | pas 0 | 4 | g1 | g4 | g2 |

Écrite **sans** pas (`gD: gA+1`), `gD` vaudrait g2, g2, g3, g4, g1, g2 : toujours le groupe après `gA`.

### 12.6 Tests exigés

- Chronogrammes du §12.5 tick par tick : exemple A sur 5, 6, 2 et 1 groupes ; exemple B sur 4 groupes ; variante sans pas.
- Expressions : priorité des opérateurs, parenthèses, signe en tête, division tronquée (positifs et négatifs), valeurs
  0, négatives et supérieures à N (rebouclage), `g*` = `N`, `g<k>` = k, pas nul, pas relatif.
- Suivre / partir : `gB: gA+1` réévalué à chaque cran ; `gB: gA+1; +2` figé au départ ; variable citant une liste ;
  chaîne de trois variables ; déclaration avant et après la variable citée ; liste vide ⇒ sans valeur.
- Erreurs : chacun des messages du §12.3, avec la ligne ; boucle directe, indirecte et sur soi-même ; division par zéro
  écrite en dur (validation) et à l'exécution (variable sans valeur, effet non interrompu).
- `gN` : cible et départ sans déclaration ⇒ refus avec le message ; déclarée ⇒ variable ordinaire, `gN+1` voisin.
- Compatibilité §12.4 : les 5 natifs et les exemples de `docs/LIGHT_EFFECTS.md` (hors `gN`) rendent **les mêmes trames**
  qu'en v11.7.0 pour N = 1 à 12, y compris avec des groupes vides.
- Limites : 24 variables acceptées, 25 refusées ; 40 actions acceptées, 41 refusées.
- Coût du rendu : `Render` d'un effet de 24 variables (dont une chaîne de dépendances) et 40 actions par pas, au tick
  10 000, reste compatible avec le lecteur à 10 Hz — banc de mesure (`go test -bench`), valeur consignée dans le
  rapport de QA ; au-delà de 1 ms par appel, `dev-backend` mémoïse au lieu de rejouer depuis le tick 0.
