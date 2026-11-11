# Écrire un effet lumière

> **Version** : v11.7.0 (#223, format pas à pas, modèle à trois zones).
> **Référence normative** : `contracts/light-effects.md` (§2 zones, §4 format, §6 réglages, §9 natifs).
> **Où régler** : Ambiance › Lumière › **Effets** (`/admin/ambiance`), voir `docs/ADMIN_GUIDE.md`.

Un **effet** dit, pas à pas, ce que font les ampoules pendant un moment du jeu : le chronomètre (`chrono`), l'attente
et les pauses (`calm`), l'attribution des points (`score`). Il s'écrit dans un fichier texte, sans recompiler le serveur.
Un fichier = un effet. Le moteur interprète le fichier ; vous ne programmez rien.

---

## 1. Les notions

| Mot | Sens |
|-----|------|
| **Zone** | Un ensemble de groupes : `ambiance` (la salle), `animateur` (pupitre, régie), `public`. |
| **Groupe** | Un sous-ensemble d'ampoules d'une zone, **numéroté** (`g1`, `g2`…) et **sans nom**. |
| **g0** | Toute la zone. |
| **Tick** | Un battement de l'horloge de l'effet. Un pas dure toujours un tick. |
| **Pas** | Ce qui est écrit sur les groupes à un tick : une table « cible : action ». |
| **Variable** | Un groupe qui avance d'un tick à l'autre (`gA`), pour faire tourner une lumière dans la salle. |

Un même fichier d'effet sert aux trois zones : c'est le **numéro** du groupe qui décide ce qui est allumé, dans la zone
où l'effet est joué.

## 2. Où sont les fichiers

```
data/files/effects/              ← VOS effets (un fichier .yaml par effet)
data/files/effects/natifs/       ← copies de référence des effets livrés (NE PAS MODIFIER)
```

- Le dossier `natifs/` est **réécrit à chaque démarrage** : toute modification y est perdue. Les effets livrés sont lus
  **dans le serveur** ; une mise à jour de BuzzControl met donc les effets livrés à jour.
- Les fichiers directement dans `data/files/effects/` (sous-dossiers ignorés) sont **les vôtres**. Ils sont sauvegardés
  avec la catégorie « ambiance » des sauvegardes.
- Limites : **64 Ko** par fichier, **100** fichiers. Le nom du fichier est libre ; c'est le champ `id` qui identifie l'effet.

## 3. Créer un effet : copier un effet livré

1. Copiez `data/files/effects/natifs/chenillard.yaml` dans `data/files/effects/`, par exemple `mon-chenillard.yaml`.
2. **Changez la clé `id`** (`id: mon-chenillard`). Un `id` différent crée un **nouvel** effet, qui s'ajoute au catalogue.
3. Modifiez les pas, les variables, la durée du tick (`duree_tick_ms`).
4. Enregistrez. Le serveur recharge le dossier dans les 2 secondes ; rien à redémarrer.
5. L'effet apparaît dans Ambiance › Lumière › Effets, dans la liste de ses moments.

> **Attention** : si vous gardez l'`id` d'un effet livré, votre fichier **remplace** l'effet livré (origine « surcharge »).
> Pour revenir à l'effet d'origine, supprimez votre fichier.

## 4. Structure d'un fichier

```yaml
# Commentaires : une ligne qui commence par #.
id: mon-effet               # [a-z0-9-], 1 à 40 caractères, unique
nom: Mon effet              # libellé du catalogue, 1 à 40 caractères
description: >              # 500 caractères au plus
  Une phrase qui dit ce que voit la salle.
version: 2                  # seule la valeur 2 est acceptée
moments: [chrono]           # chrono, calm, score (au moins un)
groupes_min: 3              # facultatif : nombre de groupes non vides à partir duquel la zone peut le jouer (défaut 0)
duree_tick_ms: 1000         # durée d'un tick, donc d'un pas : 250 à 60 000 ms
boucle: oui                 # oui (défaut) | non

variables:                  # facultatif, 8 au plus
  gA: g1; +1                # la tête : part du groupe 1, avance d'un groupe par tick

pre:                        # facultatif : joué une fois au début du moment
  { g0: [theme, 100] }
pas:                        # obligatoire, 1 à 200 pas : la boucle, un pas par tick
  - { g0: [theme, 100], gA: [urgence, 100] }
dernier:                    # facultatif : joué à la fin de chaque tour (avant de recommencer)
  { g0: [theme, 100] }
post:                       # facultatif : joué à la fin du moment
  { g0: [scene, 100] }
```

Clés de premier niveau : `id`, `nom`, `description`, `version`, `moments`, `groupes_min`, `duree_tick_ms`, `boucle`,
`variables`, `pre`, `pas`, `dernier`, `post`. Obligatoires : `id`, `nom`, `version`, `moments`, `duree_tick_ms`, `pas`.
**Une clé inconnue est une erreur**, jamais un réglage ignoré en silence.

Les mots-clés sont en français : `pre` (départ), `pas` (la boucle), `dernier` (fin de tour), `post` (fin du moment).

### 4.1 Moments

| Moment | Quand l'effet joue |
|--------|--------------------|
| `chrono` | Pendant le décompte d'une question |
| `calm` | Attente, pause, entracte |
| `score` | Pendant l'attribution des points (durée de l'impulsion SCORE) |

## 5. Un pas, une action

Un pas (`pre`, un élément de `pas`, `dernier`, `post`) est une table **« cible : action »**, 1 à 16 actions.

```yaml
- { g0: [theme, 100], gA: [urgence, 100] }   # sur toute la zone, le thème ; sur gA, la couleur d'urgence
- { g2: [eteint] }                           # le groupe 2 s'éteint
- { g0: ["#0A84FF", 60] }                    # bleu fixe à 60 %
```

- **Action** : `[couleur, intensité]`, intensité de 0 à 100 (0 = éteint), ou `[eteint]`. Le fondu est toujours nul.
- **Couleurs** : `"#RRGGBB"`, ou un nom :

| Nom | Vaut |
|-----|------|
| `theme` | le thème de la question (blanc à défaut) |
| `equipe` | l'équipe créditée (moment `score` ; blanc hors de ce moment) |
| `urgence` | blanc ; **orange `#FFAA00` à 10 s ou moins** ; **rouge `#E61E1E` à 5 s ou moins** (hors chronomètre : blanc) |
| `scene` | la couleur de la scène de salle du moment (blanc pour la zone animateur) |

Les couleurs nommées sont **vivantes** : elles sont recalculées à chaque tick. Un groupe peint en `urgence` vire donc
seul à l'orange, puis au rouge.

- Les actions d'un pas s'appliquent **dans l'ordre écrit** : si deux visent la même ampoule, la dernière gagne.
- **Un groupe non cité garde son état** du pas précédent. Avant toute action, la zone a sa scène ordinaire.

## 6. Cibles

| Écriture | Désigne |
|----------|---------|
| `g0` | **toute la zone** où l'effet est joué. Pour `ambiance` : toutes les ampoules de la salle, groupées ou non. Pour `animateur` et `public` : toutes les ampoules de leurs groupes. Jamais une ampoule d'une autre zone ni d'une équipe. |
| `g1`, `g2`… | le groupe de ce **numéro réel** dans la zone. **Absent ou vide ⇒ l'action est ignorée.** |
| `gN` | le dernier groupe non vide |
| `gA`, `gB`… | une **variable** déclarée dans `variables` |
| `gA+1`, `gA-2` | le **voisin** du groupe que désigne `gA`, à m rangs (après le dernier, on revient au premier) |

- **N** = nombre de groupes **non vides** de la zone où l'effet est joué. Les rangs 1…N sont la suite ordonnée de ces
  groupes ; tout calcul reboucle sur N. Le fichier ignore la taille de la zone.
- **Numéro réel et rang** : `g3` désigne le groupe numéro 3. Le **départ** d'une variable et ses positions sont des
  **rangs** parmi les groupes non vides : avec les groupes 1, 2 (vide), 3, `gA: g2; 0` désigne le 2e groupe non vide,
  c'est-à-dire le groupe 3. Une **liste** cite des numéros réels ; les groupes absents ou vides en sont retirés.
- **Pas sauté** : un pas dont **aucune** action ne vise une cible existante est sauté. Il ne consomme pas de tick et ne
  fait pas avancer les variables. Si tous les pas sont sautés, l'effet est inapplicable à la zone.

## 7. Variables de groupe

| Déclaration | Sens |
|-------------|------|
| `gA: g1; +1` | progression : départ au groupe 1, un groupe de plus à chaque tick |
| `gB: g2; +2` | départ au groupe 2, deux groupes de plus par tick |
| `gD: gN; -1` | départ au dernier groupe, recule d'un groupe par tick |
| `gE: g3; 0` | désigne toujours le groupe 3 |
| `gC: [g1, g4, g3, g7]` | liste : un élément par tick, reprise au début après le dernier ; les groupes absents sont retirés (liste vide ⇒ actions ignorées) |

- Au cran k, une progression désigne la position `((départ − 1 + incrément × k) mod N) + 1`. Un départ au-delà de N
  reboucle (`g7` avec N = 4 ⇒ position 3). Incrément entre −12 et +12.
- Le nom d'une variable est `g` suivi d'une lettre majuscule (autre que `N`), puis lettres ou chiffres : `gA`, `gB2`…

## 8. Déroulement et règle d'avance

1. **Démarrage** (début du moment, ou de l'essai) : le compteur de tour k vaut 0. Si `pre` existe, il est joué au tick 0.
2. **Boucle** : sans `pre`, le pas 0 est joué au tick 0 ; avec `pre`, il est joué au tick 1. Puis un pas par tick. Après
   le dernier pas, `dernier` (s'il existe) occupe un tick ; puis retour au pas 0.
3. **Avance des variables** : `pre` et le **premier** pas voient les variables à leur départ (k = 0). Ensuite k augmente
   de 1 **à chaque pas de la boucle joué**. `pre`, `dernier` et `post` **ne font pas avancer** : `dernier` voit les mêmes
   valeurs que le pas qui le précède. k ne revient jamais à 0 quand la boucle recommence.
4. `boucle: non` : un seul tour (`pas`, puis `dernier`), puis l'état est tenu jusqu'à la fin du moment.
5. **Fin du moment** : `post` est joué immédiatement, sans attendre un tick. Dans les zones `ambiance` et `animateur`,
   l'état suivant repeint toute la zone : `post` n'y a pas d'effet visible. Dans la zone `public`, rien ne le recouvre :
   `post` reste appliqué jusqu'au prochain effet.

**Exemple de chronogramme** (4 groupes, `gA: g1; +1`, `gB: g2; +2`, un `pre`, deux pas, un `dernier`) :

```yaml
id: exemple-chronogramme
nom: Exemple
version: 2
moments: [chrono]
duree_tick_ms: 1000
variables:
  gA: g1; +1
  gB: g2; +2
pre:
  { g0: [scene, 100] }
pas:
  - { gA: [urgence, 100], gB: [theme, 100] }
  - { gA: [theme, 100], gB: [urgence, 100] }
dernier:
  { g0: [scene, 100] }
```

| Tick | Joué | k | gA | gB |
|------|------|---|----|----|
| 0 | `pre` | 0 | g1 | g2 |
| 1 | pas 0 | 0 | g1 | g2 |
| 2 | pas 1 | 1 | g2 | g4 |
| 3 | `dernier` | 1 | g2 | g4 |
| 4 | pas 0 | 2 | g3 | g2 |
| 5 | pas 1 | 3 | g4 | g4 |
| 6 | `dernier` | 3 | g4 | g4 |
| 7 | pas 0 | 4 | g1 | g2 |

## 9. Trois zones, un même fichier

| Zone | Groupes | Ce que `g0` désigne | Sans effet pour le moment |
|------|---------|---------------------|---------------------------|
| `ambiance` | rôle `salle` | toutes les ampoules de la salle | scène de salle ordinaire |
| `animateur` | rôle `animateur` | toutes les ampoules des groupes animateur | **blanc fixe** |
| `public` | rôle `public` | toutes les ampoules des groupes public | **rien** : la zone n'est pas pilotée |

- Un **chenillard** peut tourner sur les groupes de la zone animateur, comme dans la salle.
- Une zone **Animateur ou Public sans groupe non vide** n'existe pas : son réglage est conservé, sans effet. La zone **Ambiance** existe toujours.
- `groupes_min` compte les groupes **non vides de la zone** : un effet qui demande 3 groupes est grisé dans une zone qui
  n'en a que 2, avec la raison affichée (« demande 3 groupes, 2 dans cette zone »).
- Une ampoule n'entre dans les zones `animateur` ou `public` **que par son groupe**.

## 10. Erreurs de validation

Une erreur porte le **fichier**, la **ligne** et un **message en français** :

```
mon-effet.yaml, ligne 12 : la variable « gB » n'est pas déclarée dans variables
```

| Message (type) | Cause |
|----------------|-------|
| format de la version précédente | `version: 1` (motifs calculés) : réécrire en pas à pas |
| YAML illisible | indentation ou deux-points mal placés |
| clé inconnue | faute de frappe dans une clé |
| clé obligatoire absente | `id`, `nom`, `version`, `moments`, `duree_tick_ms` ou `pas` manque |
| `id` invalide ou en double | caractères hors a-z, 0-9, tiret ; ou deux fichiers utilisateur de même `id` |
| `moments` vide ou valeur inconnue | utiliser `chrono`, `calm` ou `score` |
| `duree_tick_ms` hors bornes | entre 250 et 60 000 ms |
| `pas` vide ou trop long | 1 à 200 pas |
| cible mal écrite | voir §6 |
| variable non déclarée | `gB` utilisé sans `gB:` dans `variables` |
| déclaration de variable mal écrite | voir §7 |
| couleur invalide | ni `"#RRGGBB"`, ni `theme`, `equipe`, `urgence`, `scene` |
| intensité hors 0-100 | corriger la valeur |
| fichier trop gros | plus de 64 Ko |

**Un fichier invalide ne casse rien** : il est écarté et son erreur est signalée (`GET /api/lighting/effects`, champ
`errors`, et une ligne de journal). La version précédente du même `id` reste active si elle existe en mémoire ; sinon le
natif de même `id`.

## 11. Rechargement et vitesse

- Le serveur vérifie le dossier **toutes les 2 secondes** et recharge en cas de changement. Pas besoin de redémarrer.
- `POST /api/lighting/effects/reload` force un rechargement immédiat. Le rechargement est atomique.
- **La vitesse ne s'écrit pas dans le fichier.** Elle se règle dans Ambiance › Lumière › Effets, par moment et par zone :
  `0.5`, `0.75`, `1`, `1.5` ou `2`. La durée réelle d'un tick est `duree_tick_ms` ÷ vitesse, **jamais sous 250 ms**. Un
  changement de vitesse prend effet au tick suivant, sans faire repartir l'effet.
- Un effet très rapide peut donc être plafonné : l'interface l'indique.

## 12. Exemples livrés

**Chenillard** (`natifs/chenillard.yaml`) : une tête à la couleur d'urgence, le reste au thème, un groupe par seconde.

```yaml
id: chenillard
nom: Chenillard
version: 2
moments: [chrono]
groupes_min: 3
duree_tick_ms: 1000
variables:
  gA: g1; +1
pas:
  - { g0: [theme, 100], gA: [urgence, 100] }
```

**Chenillard avec traînée** (`natifs/chenillard-traine.yaml`) : le même, avec une traînée à 50 % de chaque côté
(`gA-1`, `gA+1`). Proposé à partir de 5 groupes.

**Pulsation** (`natifs/pulsation.yaml`) : deux pas de 500 ms, thème à 100 %, puis couleur d'urgence à 50 %.

**Couleur d'équipe** (`natifs/couleur-equipe.yaml`) : `g0: [equipe, 100]`, fixe pendant l'attribution des points.

**Dégradé** (`natifs/degrade.yaml`) : une couleur par groupe, en cycle de quatre couleurs, fixe pendant les phases
calmes. Une salle plus petite en montre le début.

Effets livrés à ce jour : `pulsation`, `chenillard`, `chenillard-traine`, `couleur-equipe`, `degrade`. Les effets
`remplissage`, `scintillement`, `vague` et `vague-lente` sont sortis de la v11.7.0 et restent au backlog.

## 13. Revenir à l'effet d'origine

- **Effet livré** : supprimez (ou renommez hors `.yaml`) votre fichier qui porte le même `id`. Le natif reprend au
  rechargement suivant.
- **Effet à vous** : supprimez son fichier. Un moment qui le réglait joue alors sa valeur par défaut (`pulsation` pour
  `chrono` dans la salle, aucun effet ailleurs).

## 14. Vérifier son effet

- **Catalogue** : Ambiance › Lumière › Effets. Un effet inapplicable à une zone est grisé, avec sa raison.
- **Erreurs** : `GET /api/lighting/effects` (champ `errors`) ou le journal du serveur.
- **Essai** : le bouton « Tester » (à côté du choix d'effet) joue l'effet dix secondes sur sa zone, à la vitesse réglée.
  « Arrêter » l'interrompt. Pendant une question, le bouton est grisé, et le serveur refuse l'essai (409 `game`).
