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

---

## 1. Vocabulaire

| Terme | Définition |
|---|---|
| **Groupe d'ambiance** | Sous-ensemble **ordonné et numéroté** (g1…gN) des ampoules de rôle `general`, **sans nom**. Unité que pilote un effet. `g0` désigne toute la zone. |
| **Groupe animateur** | Groupe d'ambiance portant le rôle `animateur` (**plusieurs possibles**, §2.5, révision du GATE LOT-1) : numéroté à part (A1, A2…), jamais entraîné par les effets de salle, blanc fixe par défaut, avec ses propres effets par moment. |
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
    { "id": "g3f9a1", "role": "salle",     "lights": ["BuzzHue1", "BuzzHue4"] },   // g1
    { "id": "g81c02", "role": "salle",     "lights": ["BuzzHue2"] },               // g2
    { "id": "g5d7e4", "role": "animateur", "lights": ["BuzzHue8"] },               // groupe animateur 1
    { "id": "g0b6c9", "role": "animateur", "lights": ["BuzzHue9"] }                // groupe animateur 2
  ]
}
```

- `role` : `"salle"` (défaut, valeur absente ou vide) ou `"animateur"` (§2.5). **Plusieurs groupes `animateur` sont
  permis** *(révision du GATE LOT-1 : la limite « un seul » est levée)*.
  Le **numéro** d'un groupe de salle est son rang parmi **tous** les groupes de rôle `salle`, vides compris. Un groupe
  animateur a son **propre rang** parmi les groupes de rôle `animateur` (A1, A2…, vides compris) et ne compte jamais dans
  N.
- **Révision 3 — `name` et `near_teams` sont supprimés.** Un groupe n'a qu'un numéro (son rang) ; l'interface l'appelle
  « Groupe 1 », « Groupe animateur 1 ». `near_teams` ne servait qu'à l'effet Vague (#301), sorti du milestone. Les deux
  clés ne sont plus acceptées à l'écriture (`400`) ; lues dans un `config.json` de QUALIF antérieur, elles sont ignorées
  et disparaissent au prochain enregistrement.
- `id` *(révision du GATE LOT-1)* : identifiant **stable** du groupe (`[a-z0-9]{1,12}`), unique. Attribué par le serveur à
  l'enregistrement quand il est absent ou vide ; le client le renvoie tel quel ensuite (renommer ou déplacer un groupe ne
  change pas son `id`). Il sert de clé aux effets des groupes animateur (§6). Un `id` en double ou mal formé ⇒ `400`.
- Plafonds : **12 groupes de salle** et **4 groupes animateur**.
- `lights` : noms d'ampoules, chacune de rôle `general` et présente dans **un seul** groupe. Un groupe vide est accepté :
  il **garde son numéro** (numéros stables pour l'interface et pour `GET /api/lighting/lights`) mais est **ignoré du
  séquenceur**. **N = nombre de groupes de salle non vides.** Le séquenceur calcule sa trame sur la suite ordonnée des
  groupes non vides (positions 1…N, sans renuméroter la configuration) et émet chaque état avec le **numéro réel** du
  groupe (`ZoneState.Group`). Exemple : groupes 1, 2 (vide), 3 ⇒ N = 2, la trame a deux positions, écrites sur
  `Group 1` et `Group 3`. *(Amendement du 2026-11-11, revue LOT-1, écart 2.)*
- Absent ou `[]` ⇒ aucun groupe : comportement v11.6.0 exact.
- Écriture : `POST /config.json {"lighting": {"groups": [...]}}`, remplacement complet du tableau. Refus `400` avec un
  message lisible : ampoule inconnue de `lights`, ampoule de rôle `team`, ampoule dans deux groupes, plus de 12 groupes
  de salle, plus de 4 groupes animateur.
- Changer le rôle d'une ampoule en `team`, ou la retirer de `lights`, la retire de son groupe dans la même écriture.
- **Ampoule générale hors groupe** : elle fait partie de `g0` (toute la zone) et suit donc les actions écrites sur `g0` ;
  elle n'est jamais visée par `g1`…`gN` ni par une variable.

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
  `""` = blanc fixe. L'effet est rendu par le même séquenceur, **sur ce seul groupe** : pour lui, `g0` = `g1` = ce
  groupe, et N = 1. *(Révision 3)* Seul un effet **qui n'écrit que `g0`** lui est applicable (§4.7) ; les autres restent
  grisés dans ses sélecteurs. La couleur `scene` vaut blanc pour ce groupe. Effet absent, invalide ou inapplicable ⇒
  blanc fixe.
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

Un pas (`pre`, chaque élément de `pas`, `dernier`, `post`) est une table `cible: action`, 1 à 16 actions.

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
| `g0` | toute la zone : dans la salle, toutes les ampoules d'ambiance, groupées ou non (jamais celles des groupes animateur ni des équipes) |
| `g1`, `g2`… | le groupe de salle de ce numéro (numéro réel, §2.1) ; **absent ou vide ⇒ action ignorée** |
| `gN` | le dernier groupe non vide |
| `gA`, `gB`… (`g` + une lettre majuscule autre que `N`, puis lettres ou chiffres) | une variable déclarée dans `variables` |
| `gA+1`, `gA-2` | le voisin du groupe que désigne `gA`, à m rangs ; après le dernier groupe on revient au premier |

N = nombre de groupes de salle **non vides**. Les positions 1…N sont la suite ordonnée de ces groupes ; tout calcul
reboucle sur N (le fichier ne connaît pas la taille de la salle).

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
- 8 variables au plus par effet. N = 0 ⇒ toute action sur une variable est ignorée.

### 4.5 Déroulement et règle d'avance — normatif

1. **Démarrage** (début du moment, ou de l'essai) : cran k = 0. Si `pre` existe, il est joué au tick 0.
2. **Boucle** : au tick suivant, pas 0, puis un pas par tick. Après le dernier pas de `pas`, `dernier` (s'il existe)
   occupe un tick ; puis retour au pas 0.
3. **Avance des variables** : PRE et le **premier** pas 0 voient les variables à leur départ (k = 0). Ensuite k augmente
   de 1 **à chaque pas de la boucle joué**. **PRE, `dernier` (LAST) et POST ne font pas avancer** : `dernier` voit les
   mêmes valeurs que le pas qui le précède. k ne revient jamais à 0 quand la boucle recommence.
4. `boucle: non` : un seul tour (`pas`, puis `dernier`), puis l'état est tenu jusqu'à la fin du moment.
5. **Fin du moment** : `post` (s'il existe) est appliqué **immédiatement**, sans attendre un tick, avec les variables à
   leur dernière valeur ; l'état suivant (scène, autre effet) le recouvre aussitôt. POST ne retarde jamais la scène
   suivante ; il n'est visible que sur les ampoules que celle-ci ne repeint pas.

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
Pour un groupe animateur de rang k, tout est écrit sur `Group = -k`.

### 4.7 Applicabilité et repli

| Cible de l'effet | Applicable si |
|---|---|
| Salle | N ≥ `groupes_min`, et au moins un pas de la boucle n'est pas sauté (§4.3) |
| Groupe animateur | l'effet **n'écrit que `g0`** (aucune autre cible dans `pre`, `pas`, `dernier`, `post`) |

Un effet inapplicable fait jouer au moment sa valeur par défaut (§3.3). Le catalogue et le sélecteur l'indiquent
(« demande 5 groupes, 3 configurés » ; « anime plusieurs groupes »).

### 4.8 Erreurs de validation

Chaque erreur porte `file`, `line` (0 si inconnue), `message` en français, sans jargon :
`"mon-effet.yaml, ligne 12 : la variable « gB » n'est pas déclarée dans variables"`. Liste minimale : YAML illisible, clé
inconnue, clé obligatoire absente, `id` invalide ou en double, `version` non prise en charge, `moments` vide ou valeur
inconnue, `duree_tick_ms` hors bornes, `pas` vide ou trop long, cible mal écrite, variable non déclarée, déclaration de
variable mal écrite, couleur invalide, intensité hors 0-100, fichier trop gros.

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

**Groupes animateur** : le lecteur calcule **une trame indépendante par groupe animateur non vide** (réglage
`light.host_effects.<id>`, même moment vivant, N = 1, écrite sur `Group = -k`). Seul le point 1 s'y applique : les points 2 et 3 ne le concernent pas
(§2.5).

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
- **Vitesse réglable à l'usage** *(ajout à la validation de la révision 3, 2026-11-11)* — un **multiplicateur de vitesse
  par moment**, à côté de chaque choix d'effet, pour la salle et pour chaque groupe animateur, en global et par manche.
  Le fichier d'effet n'est jamais modifié : `duree_tick_ms` reste la durée de référence.

  | Champ (`Settings`) | JSON (manche, API, état de jeu) | `config.json` (valeur globale) | Défaut |
  |---|---|---|---|
  | `LightSpeeds[<moment>]` | `light.speeds.<moment>` | `lighting.speeds.<moment>` | `1` |
  | `LightHostSpeeds[<id>][<moment>]` | `light.host_speeds.<id>.<moment>` | `lighting.host_speeds.<id>.<moment>` | `1` |

  - Valeurs permises : **`0.5`, `0.75`, `1`, `1.5`, `2`** (nombre JSON). Toute autre valeur ⇒ `400` / `INVALID_AMBIANCE`.
  - **Durée réelle d'un tick = `duree_tick_ms` ÷ vitesse, jamais sous 250 ms** (cadence de l'écrivain) : un effet à
    400 ms en ×2 tourne à 250 ms, pas à 200 ms. La vitesse n'agit que sur l'horloge : pas, variables et couleurs sont
    inchangés.
  - Mêmes règles que les effets : forme complète (les trois moments toujours présents ; une entrée par groupe animateur
    existant), `Partial` (valeurs surchargées seulement), `Sources.light.speeds.<moment>` et
    `Sources.light.host_speeds.<id>.<moment>` ∈ `global | round`, sérialisation sans `omitempty` dans
    `AMBIANCE_EFFECTIVE`, `id` de groupe inconnu écarté sans erreur, purge à l'enregistrement des groupes.
  - Le réglage vaut pour le moment, quel que soit l'effet choisi ; il est sans effet quand aucun effet n'est joué. Un
    changement de vitesse s'applique au tick suivant, sans faire repartir l'effet.
  - Absents de `config.json` et de `rounds.json` ⇒ vitesse 1 partout.
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
      "origin": "native", "file": "natifs/chenillard.yaml", "min_groups": 3,
      "applicable": true, "reason": "", "host_applicable": false }
  ],
  "errors": [ { "file": "mon-effet.yaml", "line": 12, "message": "la variable « gB » n'est pas déclarée dans variables" } ],
  "unresolved": [ { "moment": "calm", "id": "mon-effet", "scope": "global" } ],
  "dir": "data/files/effects"
}
```

Les noms de champs de l'API restent en anglais : `name` = `nom` du fichier, `min_groups` = `groupes_min` (0 par défaut).
Le champ `target` est **supprimé** (révision 3). `applicable` concerne la salle, `host_applicable` les groupes animateur
(§4.7). `origin` ∈ `native` | `user` | `override` (fichier utilisateur qui surcharge un natif). `reason` explique
`applicable: false`. Ordre : natifs (ordre du §9), puis utilisateur par `name`. Aucune E/S vers le pont.

### 7.2 `POST /api/lighting/effects/reload`

Recharge le dossier. Réponse : celle du §7.1.

### 7.3 `GET /api/lighting/effects/{id}/preview?groups=N&moment=chrono`

Trames calculées par le serveur avec `effects.Render` — le navigateur ne fait que les rejouer (aucun second
interpréteur). `groups` : 1 à 12 (défaut : nombre configuré, 6 s'il vaut 0). Contexte simulé : chronomètre de 20 s pour
`chrono`, thème `#4F46E5`, équipe `#E53935`. La réponse couvre PRE, puis les tours de la boucle sur 20 s.

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

`POST /config.json` : `lighting.groups`, `lighting.effects`, `lighting.host_effects`, `lighting.speeds`, `lighting.host_speeds` (§2.1, §6). `GET /api/ambiance/settings` et `ROUNDS_UPDATE` :
`light.effects`, `light.host_effects`, `light.speeds`, `light.host_speeds`. `GET /api/lighting/lights` : chaque ampoule porte `group` (`0` | `n` | `"host<k>"`, §2.4).

---

## 8. Critères chiffrés et mesure

### 8.1 Tests exigés

- `effects.Render` (révision 3) : le **chronogramme de référence du §4.5** tick par tick (variables et couleur de chaque
  groupe) ; progression, incrément négatif, liste, voisins, rebouclage (N = 1, 4, 8, 12) ; groupe absent ignoré, pas
  sauté ; groupe non cité = état conservé ; ordre des actions ; `boucle: non` ; couleurs nommées vivantes ; trame réduite
  à `Group 0` pour un effet qui n'écrit que `g0`.
- Lecteur : PRE au démarrage du moment, POST publié à la fin du moment sans retarder la scène suivante ; vitesse — tick
  divisé par le multiplicateur, plancher de 250 ms, changement pris au tick suivant.
- Chaque natif : valide, applicable selon `groupes_min` et §4.7 (`pulsation` applicable avec N = 0 et sur un groupe
  animateur), budget du §5.4.
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
| `pulsation` | chrono | 0 | existant (#212) | `g0: [theme, 100]` puis `g0: [urgence, 50]`, tick 500 ms | Toute la salle respire ; la phase basse passe à l'orange puis au rouge (§5.5). Jouable sur un groupe animateur |
| `chenillard` | chrono | 3 | #223 | `gA: g1; +1` ; un pas `g0: [theme, 100], gA: [urgence, 100]`, tick 1000 ms | Tête à la couleur d'urgence, fond au thème, un groupe par seconde |
| `chenillard-traine` | chrono | 5 | #223 | même variable ; un pas `g0: [theme, 100], gA-1: [theme, 50], gA+1: [theme, 50], gA: [urgence, 100]` | Le même avec une traînée à 50 % de part et d'autre de la tête |
| `couleur-equipe` | score | 0 | #211 (groupes animateur) | un pas `g0: [equipe, 100]`, tick 1000 ms | Couleur de l'équipe créditée le temps de l'attribution. Jouable sur un groupe animateur |
| `degrade` | calm | 2 | #211 | un pas qui donne une couleur à chacun des groupes `g1` à `g12` (cycle de 4 couleurs), tick 60 000 ms | Scène multicolore fixe, une couleur par groupe ; une salle plus petite en montre le début |

#223 demandait « pas de traînée sous 5 groupes » dans un seul effet : sans variante, la règle devient **deux effets
natifs**, `chenillard-traine` n'étant proposé qu'à partir de 5 groupes.

Sortis de v11.7.0 (noyau seul) : `remplissage` (#300), `scintillement` (#302), `vague` et `vague-lente` (#301).

Les valeurs exactes (durées, couleurs, intensités) sont dans les fichiers `assets/effects/*.yaml` ; elles peuvent être
ajustées après l'essai sur matériel sans changer ce contrat.

## 10. Hors de ce contrat

| Sujet | Où |
|---|---|
| Éditeur de scènes et d'effets dans l'interface | v11.8.0 (#210) — écrit le format du §4 (grille ticks × groupes) |
| Remplissage (#300), vague (#301), scintillement (#302) ; variantes selon le temps restant, hasard, étalement sur le chronomètre, départ depuis l'équipe, fondu | hors v11.7.0 (révision 3) — backlog |
| Rôles fins par ampoule (accentuation, fond) | hors v11.7.0 (décision D3) ; le besoin « animateur » est couvert par le groupe animateur (§2.5) |
| Effets sur les ampoules d'équipe, effets des buzzers | inchangés (`hue-bridge.md` §5.2, `LED_SET_PROTOCOL.md`) |
| Geste direct de choix d'effet depuis le popover | non prévu |
