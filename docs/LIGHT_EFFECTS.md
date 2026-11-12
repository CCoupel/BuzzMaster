# Écrire un effet lumière

> **Version** : v11.8.0 — format de fichier **révision 7** (`version: 2`, #210) : `g*` pour le dernier groupe, expressions
> dans les variables, 24 variables et 40 actions par pas. **Changement cassant** pour un fichier écrit à la main en v11.7.0
> qui emploie `gN` (§17). Import, téléchargement et suppression depuis le catalogue (#305, §15) ; éditeur dans l'interface
> (#210, §16).
> **Référence normative** : `contracts/light-effects.md` (§2 zones, §4 format, §6 réglages, §9 natifs, §11 éditeur, §12 format r7).
> **Où régler** : Ambiance › Lumière › **Effets** (`/admin/ambiance`), voir `docs/ADMIN_GUIDE.md`.

Un **effet** dit, pas à pas, ce que font les ampoules pendant un moment du jeu : le chronomètre (`chrono`), l'attente
et les pauses (`calm`), l'attribution des points (`score`). Il s'écrit dans un fichier texte, sans recompiler le serveur,
ou dans l'éditeur de l'interface (§16). Un fichier = un effet. Le moteur interprète le fichier ; vous ne programmez rien.

---

## 1. Les notions

| Mot | Sens |
|-----|------|
| **Zone** | Un ensemble de groupes : `ambiance` (la salle), `animateur` (pupitre, régie), `public`. |
| **Groupe** | Un sous-ensemble d'ampoules d'une zone, **numéroté** (`g1`, `g2`…) et **sans nom**. |
| **g0** | Toute la zone. |
| **g\*** | Le **dernier groupe non vide** de la zone (v11.8.0). |
| **N** | Le nombre de groupes **non vides** de la zone où l'effet est joué. |
| **Tick** | Un battement de l'horloge de l'effet. Un pas dure toujours un tick. |
| **Pas** | Ce qui est écrit sur les groupes à un tick : une table « cible : action ». |
| **Variable** | Un nom de groupe qui change d'un tick à l'autre (`gA`), pour faire tourner une lumière dans la salle. |

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

Deux façons d'écrire un effet : à la main, dans ce dossier (ci-dessous), ou dans l'éditeur de l'interface (§16).

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

variables:                  # facultatif, 24 au plus
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

Un pas (`pre`, un élément de `pas`, `dernier`, `post`) est une table **« cible : action »**, **1 à 40 actions**.

```yaml
- { g0: [theme, 100], gA: [urgence, 100] }   # sur toute la zone, le thème ; sur gA, la couleur d'urgence
- { g2: [eteint] }                           # le groupe 2 s'éteint
- { g0: ["#0A84FF", 60] }                    # bleu fixe à 60 %
- { g*: [scene, 100] }                       # le dernier groupe non vide, à la couleur de la scène
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
| `g1`, `g2`… `g12` | le groupe de ce **numéro réel** dans la zone. **Absent ou vide ⇒ l'action est ignorée.** |
| `g*` | le **dernier groupe non vide** de la zone (v11.8.0). Avec N = 0, l'action est ignorée. Écrit sans guillemets : `{ g*: [theme, 100] }`. |
| `gA`, `gB`… | une **variable** déclarée dans `variables` |
| `gA+1`, `gA-2` | le **voisin** du groupe que désigne `gA`, à m rangs (après le dernier, on revient au premier). Valide dans un fichier ; l'éditeur le remplace par une variable (§16). |

- **N** = nombre de groupes **non vides** de la zone où l'effet est joué. Les rangs 1…N sont la suite ordonnée de ces
  groupes ; tout calcul reboucle sur N. Le fichier ignore la taille de la zone.
- **`g*` et `gN`** : `g*` est le dernier groupe. `gN` **n'est plus** une cible : c'est un nom de variable comme les autres
  (§17). `g*+1` n'est pas une cible. **Une cible n'est jamais une expression** : les expressions ne s'écrivent que dans
  `variables`.
- **Numéro réel et rang** : `g3` désigne le groupe numéro 3. Le **départ** d'une variable et ses positions sont des
  **rangs** parmi les groupes non vides : avec les groupes 1, 2 (vide), 3, `gA: g2; 0` désigne le 2e groupe non vide,
  c'est-à-dire le groupe 3. Une **liste** cite des numéros réels ; les groupes absents ou vides en sont retirés.
- **Pas sauté** : un pas dont **aucune** action ne vise une cible existante est sauté. Il ne consomme pas de tick et ne
  fait pas avancer les variables. Si tous les pas sont sautés, l'effet est inapplicable à la zone.

## 7. Variables de groupe

Une variable se déclare avec un **départ** et, en option, un **pas**. Le départ et le pas sont des **expressions**
(v11.8.0), évaluées par le serveur.

| Déclaration | Sens |
|-------------|------|
| `gA: g1; +1` | avance : départ au groupe 1, un groupe de plus à chaque tick |
| `gB: g2; +2` | avance : départ au groupe 2, deux groupes de plus par tick |
| `gE: g3; 0` | désigne toujours le groupe 3 |
| `gD: g*; -1` | avance : départ au dernier groupe, recule d'un groupe par tick (v11.7.0 : `gD: gN; -1`) |
| `gA: N/2; +1` | avance : départ au milieu (division entière), un groupe par tick |
| `gB: gA+1` | **suit** : sans pas, désigne toujours le groupe juste après `gA` |
| `gC: [g1, g4, g3, g7]` | liste : un élément par tick, reprise au début après le dernier ; les groupes absents sont retirés (liste vide ⇒ actions ignorées) |

### 7.1 Expressions (départ et pas)

```
déclaration := départ                      # sans pas : « suit »
             | départ ";" pas              # avec pas : « avance »
départ, pas := expression
expression  := terme { ("+" | "-") terme }
terme       := facteur { ("*" | "/") facteur }
facteur     := entier | "N" | "g*" | "g" entier | variable | "(" expression ")" | ("+" | "-") facteur
```

| Élément | Vaut |
|---------|------|
| entier (0 à 999) | lui-même |
| `N` | le nombre de groupes **non vides** de la zone où l'effet est joué |
| `g*` | `N` (le rang du dernier groupe) |
| `g1`…`g99` | le nombre écrit (`g3` = 3) — écriture v11.7.0, conservée (les **cibles** restent `g1`…`g12`) |
| `gA` (variable) | le **rang** (1…N) du groupe que désigne cette variable ; **dans le départ seulement** — le pas ne cite jamais de variable |
| `+ - *` | arithmétique entière |
| `/` | **division entière** : le quotient est **tronqué** (`5/2` = 2, `2/3` = 0, `-5/3` = −1) |

- Espaces libres. 60 caractères au plus par expression. Un signe en tête est admis (`+1`, `-N/3`).
- **Plafond ±1 000 000** : une constante hors de cette borne est refusée à la validation (« dépasse la valeur permise »).
  Un calcul qui sort de la borne à l'exécution rend la variable sans valeur, comme une division par zéro. Le calcul se fait
  sur 64 bits : un même effet désigne le même groupe sur un système 32 bits et sur un système 64 bits.
- **Rang obtenu → groupe** : la valeur `v` désigne le rang `((v − 1) mod N) + 1`, avec un reste toujours positif.
  `N+1` ⇒ rang 1 ; **`0` ⇒ rang N (le dernier groupe)** ; `−1` ⇒ rang N−1.
- **Valeur d'une variable au cran `k`** (le cran est celui de §8, inchangé) :
  - **avec pas** — `gA: D; P` : rang de `D(0) + P × k`. `D(0)` est le départ évalué au cran 0 : une variable citée y vaut
    sa position **de départ**. `P` est évalué une fois (il ne dépend que de `N`).
  - **sans pas** — `gA: D` : rang de `D(k)`, le départ **réévalué à chaque cran** : une variable citée y vaut sa position
    **au cran `k`**. `gB: gA+1` est donc toujours le groupe après `gA`. `gE: N/2` (aucune variable citée) désigne un
    groupe fixe.
- **Dépendances** : une variable peut en citer une autre, déclarée avant ou après elle, progression ou liste. Une boucle
  (`gA: gB+1`, `gB: gA+1`, ou `gA: gA+1`) est une **erreur de validation**. L'ordre de déclaration ne sert qu'à l'ordre
  d'écriture (§16.3).
- **Sans valeur** : une variable ne désigne aucun groupe — et les actions qui la visent sont ignorées — quand N = 0,
  quand elle cite une liste vide pour cette zone, ou quand une division par zéro survient **à l'exécution**
  (`6/(N-3)` avec N = 3). Une division par zéro écrite en dur (`N/0`) est une erreur de validation.
  Exemple : `gA: 1; N/(N-1)` sur une salle d'un seul groupe divise par zéro à chaque tick ; la variable ne désigne alors
  aucun groupe pour cette salle, et l'aperçu du simulateur l'affiche avec une valeur nulle (`vars` à 0).
- **Cas à zéro** (division entière) : `N/2` avec N = 1 vaut 0 ⇒ rang 1 (le seul groupe) ; `N/3` avec N = 2 vaut 0 ⇒
  rang 2 (le dernier groupe) ; **un pas qui vaut 0 n'avance pas** (`gA: 1; N/3` avec N = 2 reste sur `g1`).
- Écritures v11.7.0 : `gA: g1; +1`, `gB: g2; +2`, `gE: g3; 0` gardent exactement leur sens.
- Le pas constant n'est plus borné à ±12 : il reboucle comme le reste.
- **Coût** : un effet qui ne contient pas de `g0` et laisse des groupes sans jamais les écrire se calcule un peu plus
  lentement aux ticks très élevés. Sans effet visible en usage normal.
- Une variable s'appelle `g` suivi d'une **majuscule quelconque**, puis lettres ou chiffres : `gA`, `gB2`, `gN`.

### 7.2 Exemple A — départ au milieu, variable qui suit, pas relatif

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

### 7.3 Exemple B — partir d'une autre variable, puis avancer seule ; `gN` comme variable ; `g*` comme cible

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
  n'en a que 2, avec la raison affichée (« Demande 3 groupes ; la zone Animateur en a 2 »).
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
| `id` réservé ou nom Windows | à l'import, à l'enregistrement et dans le brouillon de l'éditeur : `reload`, `try`, `draft`, `import` ; `con`, `prn`, `aux`, `nul`, `com1`-`com9`, `lpt1`-`lpt9`. Un fichier déposé à la main dans le dossier avec un tel `id` reste chargé. |
| alias YAML | à l'import, à l'enregistrement et dans le brouillon : `*nom` est refusé, avec sa ligne ; écrire la valeur en toutes lettres. Le chargeur du dossier ne refuse pas un alias. |
| `moments` vide ou valeur inconnue | utiliser `chrono`, `calm` ou `score` |
| `duree_tick_ms` hors bornes | entre 250 et 60 000 ms |
| `pas` vide ou trop long | 1 à 200 pas, 1 à 40 actions par pas |
| cible mal écrite | voir §6 (`g0`, `g1`…`g12`, `g*`, ou une variable comme `gA`) |
| « « gN » n'est pas déclarée dans variables… écrivez g* » | `gN` employé sans variable `gN` (§17) |
| variable non déclarée | `gB` utilisé sans `gB:` dans `variables` |
| expression mal écrite | « l'expression « gA+ » est incomplète », « « N//2 » est mal écrite », « parenthèse non fermée » |
| division par zéro | « division par zéro dans « N/0 » » (écrite en dur) |
| dépendance circulaire | « les variables gA et gB dépendent l'une de l'autre » |
| pas qui cite une variable | « le pas ne peut pas citer une variable (gA) » : seul le départ peut en citer une |
| trop de variables ou d'actions | « 24 variables au plus », « un pas compte de 1 à 40 actions » |
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

Effets livrés à ce jour : `pulsation`, `chenillard`, `chenillard-traine`, `couleur-equipe`, `degrade`. Aucun effet livré
n'emploie `gN` ; leurs écritures v11.7.0 (`g1; +1`) gardent leur sens. Les effets
`remplissage`, `scintillement`, `vague` et `vague-lente` sont sortis de la v11.7.0 et restent au backlog.

## 13. Revenir à l'effet d'origine

- **Effet livré** : supprimez (ou renommez hors `.yaml`) votre fichier qui porte le même `id`. Le natif reprend au
  rechargement suivant. Depuis le catalogue (v11.8.0), le bouton « ↺ Revenir à l'effet d'origine » fait la même chose.
- **Effet à vous** : supprimez son fichier. Un moment qui le réglait joue alors sa valeur par défaut (`pulsation` pour
  `chrono` dans la salle, aucun effet ailleurs). Depuis le catalogue (v11.8.0), le bouton « 🗑 Supprimer » fait la même chose.

## 14. Vérifier son effet

- **Catalogue** : Ambiance › Lumière › Effets, sous les réglages. Chaque carte indique l'origine de l'effet (natif,
  personnalisé, natif modifié), ses moments, son nombre de pas et l'endroit où il est utilisé.
- **Erreurs** : le bandeau du catalogue liste les fichiers invalides (fichier, ligne, cause), comme
  `GET /api/lighting/effects` (champ `errors`) et le journal du serveur.
- **Aperçu** : « ▶ Aperçu » rejoue l'effet sur des ampoules dessinées g1, g2… ; le numéro du tick et le pas joué
  (PRE, pas 0, dernier) s'affichent. L'aperçu ne simule ni la fin du moment ni la vitesse.
- **Essai sur la salle** : « 💡 Essayer sur la salle » demande la zone, puis joue l'effet sur cette zone, à la vitesse ×1,
  pendant la durée par défaut du serveur (10 secondes, 5 secondes pour un effet de score seul). « ■ Arrêter l'essai »
  l'interrompt.
- **Tester** (réglages) : joue l'effet sur sa zone, à la vitesse réglée pour ce moment. Le dernier essai lancé remplace
  l'autre.
- **Grisage** : l'essai est grisé pendant une question en cours, et le serveur refuse un essai pendant une question
  (409 `game`).

## 15. Importer, télécharger, supprimer depuis le catalogue (v11.8.0, #305)

Le catalogue (Ambiance › Lumière › Effets) permet de gérer les fichiers sans passer par le dossier du serveur.
Référence normative : `contracts/light-effects.md` §11.5 à §11.7 ; routes dans `contracts/http-endpoints.md`.

- **Importer…** : choisissez un ou plusieurs fichiers `.yaml` ou `.yml` (bouton « ⭱ Importer… ») ou déposez-les sur le
  catalogue. 20 fichiers au plus par envoi, 64 Ko par fichier.
- **Chaque fichier est vérifié seul**, avec les règles du §4 (et du §10 pour les messages). Un fichier invalide n'est
  jamais écrit et n'empêche pas les autres. Le panneau de résultat affiche une ligne par fichier :
  - **importé** : nouvel effet, écrit tel quel ;
  - **remplacé** : remplace un effet à vous de même `id` (choix « Remplacer ») ;
  - **gardé en double** : importé sous un identifiant suffixé (`<id>-2`…) ; seule la valeur de la clé `id` est modifiée,
    le reste du fichier (commentaires compris) est intact (choix « Garder les deux ») ;
  - **refusé** : la ligne et la cause sont indiquées. Cas courants : fichier en `version: 1`, YAML illisible, `id` réservé
    (`reload`, `try`, `draft`, `import`), **nom réservé par Windows** (`con`, `prn`, `aux`, `nul`, `com1` à `com9`,
    `lpt1` à `lpt9`, car l'`id` sert de nom de fichier et le serveur tourne aussi sous Windows), **alias YAML** (`*nom`) —
    écrivez les valeurs en toutes lettres, un alias pouvant faire grossir un petit fichier de façon démesurée — ou fichier
    trop lourd.
- **Même `id` qu'un effet à vous** : une fenêtre demande **Remplacer**, **Garder les deux** ou **Annuler**. Elle est posée
  fichier par fichier, après le traitement des autres. « Annuler » ignore ce fichier.
- **Effet livré** : un fichier qui porte l'`id` d'un effet livré est **refusé** (ligne « pulsation.yaml », message clair
  pour changer la clé `id`). Il n'est jamais écrit et ne remplace jamais un natif. Pour surcharger un natif, passez par la
  copie décrite au §3 et gardez volontairement son `id`, à la main dans le dossier.
- **Télécharger** : le lien « ⭳ Télécharger » donne le fichier `<id>.yaml` de tout effet, livrés compris, identique au
  fichier du serveur. Pour garder une copie avant une suppression, téléchargez d'abord.
- **Supprimer** (effet à vous) et **Revenir à l'effet d'origine** (natif modifié, §13) : une fenêtre de confirmation cite
  les moments qui utilisent l'effet (« utilisé : … ») et propose de télécharger le fichier avant. Un effet livré n'a ni
  l'un ni l'autre. Les réglages qui citaient l'effet ne sont pas modifiés : le moment joue sa valeur par défaut.
- **Effet importé** : il apparaît aussitôt dans le catalogue et dans les sélecteurs des réglages, sans redémarrage.
- **Essai sur la salle** : le bouton demande la zone (liste accolée : Ambiance, toujours proposée, puis Animateur et Public
  s'ils ont au moins un groupe). Une zone qui a trop peu de groupes pour l'effet est refusée, avec la raison (« Demande 5
  groupes ; la zone Animateur en a 3 »). Le dernier choix de zone est retenu dans ce navigateur.
- Rien n'est écrit hors de `data/files/effects/`, ni dans `natifs/`, quel que soit le nom du fichier envoyé : le nom
  envoyé ne sert qu'aux messages.

## 16. Créer et modifier un effet dans l'interface (v11.8.0, #210)

L'éditeur s'ouvre dans le sous-onglet **Effets** de Ambiance › Lumière. Il remplace les réglages et le catalogue le
temps de l'édition ; « Retour au catalogue » le ferme. Référence : `contracts/light-effects.md` §11.1 à §11.10.

### 16.1 Ouvrir l'éditeur

- **＋ Nouvel effet** : une fenêtre demande le **nom** ; l'**identifiant** se propose à partir du nom (`[a-z0-9-]`, 40
  caractères au plus). Refusés : identifiant déjà pris, mal formé, réservé ou nom Windows. Un identifiant réservé ou un nom
  Windows est aussi refusé dès le brouillon, avant l'enregistrement. Le nouvel effet part d'une
  seule ligne `g0` au thème à 100 %.
- **✎ Modifier** (effet à vous) : ouvre l'effet dans l'éditeur, modifiable.
- **👁 Ouvrir** (effet livré, ou natif modifié) : ouvre l'effet **en lecture seule**. Pour le changer, dupliquez-le.
- **⧉ Dupliquer** : ouvre une copie dont l'identifiant se propose sous la forme `<id>-copie`, libre. Possible depuis le
  catalogue et depuis l'éditeur (la copie part alors du document tel qu'il est à l'écran).
- **Enregistrer** crée ou modifie le fichier. Le catalogue est mis à jour sans redémarrage. Un identifiant déjà pris
  (`exists`) ou un natif (`native`) est refusé sans écriture.

### 16.2 La grille et l'onglet Texte

- **Grille** : en colonnes, PRE, les pas numérotés, DERNIER, POST ; en lignes, **les variables en haut**, puis `g0`, puis
  les groupes numérotés, puis **`g*`** (dernière ligne). Une case vide garde l'état du groupe (aucune action).
- **Onglet Texte** : le fichier YAML, modifiable, validé en direct avec les erreurs et leur ligne.
- **Passage Grille → Texte** : si la grille a été modifiée, le texte est **régénéré** depuis l'effet. Les **commentaires
  sont perdus** : un avertissement l'indique. Sinon, le texte reste tel quel.
- **Passage Texte → Grille** : possible seulement si le texte est valide. Enregistrer depuis le Texte garde les commentaires.
- **Voisins** : un fichier qui emploie `gA-1` ou `gA+1` s'ouvre dans la grille avec ces voisins **remplacés par des
  variables** qui suivent (`gB: gA-1`) ; un message le dit, et le rendu est identique. Le texte garde les voisins si on ne
  modifie pas la grille. Seul un dépassement de 24 variables met la grille en lecture seule (modification par le Texte).
- **Groupes de la grille** : 6 lignes numérotées (`g1`…`g5`) plus `g*` par défaut. **Ajouter un groupe** (12 au plus) :
  l'ancienne ligne `g*` garde son contenu et devient un groupe numéroté ; une nouvelle ligne `g*` apparaît. **Retirer un
  groupe** (1 au moins) : la ligne `g*` est supprimée avec son contenu (confirmation si elle n'est pas vide) et la ligne
  au-dessus devient `g*`. Le nombre de lignes n'est pas enregistré : à l'ouverture, `max(5, plus grand groupe utilisé)`
  lignes numérotées s'affichent, plus `g*`. Un effet qui écrit `g12` s'ouvre donc avec 13 lignes.

### 16.3 Variables et ordre d'écriture

- **Panneau Variables** : une ligne par variable, 24 au plus. Pour chacune : le **nom** (`gA`…`gZ`), le **type**
  « Avance régulière » (départ et pas, expressions du §7.1, pas vide = « suit ») ou « Suite de groupes » (liste).
  Déclarer, modifier, réordonner et supprimer une variable ; supprimer une variable retire sa ligne et ses actions.
- **Bulle « ? »** : explique la variable et montre la **suite obtenue pour 1, 3, 6 et 12 groupes**. Ces valeurs viennent du
  serveur, le navigateur ne calcule rien.
- **Erreurs d'expression** : affichées sous la variable fautive, avec le message du serveur.
- **Ordre d'écriture** : dans un pas, le fond (`g0`) s'écrit d'abord, puis les groupes, puis `g*`, puis les variables dans
  l'ordre du panneau. **La dernière variable déclarée gagne** quand deux désignent le même groupe. Les lignes des variables
  s'affichent en haut de la grille, mais leur écriture reste après `g0` et les groupes.

### 16.4 Pas, pinceaux et couleurs

- **Pas** : ajouter, dupliquer, insérer avant, déplacer, supprimer ; 200 pas au plus, 40 actions par pas. Le menu ▾ d'un
  pas propose : dupliquer, insérer un pas avant, déplacer à gauche ou à droite, supprimer ; le bouton ＋ de
  l'en-tête ajoute un pas.
- **Pinceaux** : thème, équipe, urgence, scène, couleur libre, éteint, gomme ; chacun a une info-bulle qui dit ce qu'il
  allume, et l'intensité a la sienne.
- **Couleur libre** : une roue (teinte, saturation, luminosité) ou un champ `#RRGGBB`, avec aperçu et couleurs récentes
  (mémorisées par navigateur). Composant maison, aucune dépendance ajoutée.
- **Effet livré** : grille et texte en lecture seule ; « Dupliquer » et « Télécharger » restent disponibles.

### 16.5 Simulateur et essai du brouillon

- **Simulateur pas à pas** : rangées de **1, 3, 6 et 12 groupes**, plus les zones réelles de taille différente. Toutes les
  rangées jouent **sur la même horloge** : lecture / pause, tick précédent, tick suivant, retour au début, réglette.
  Un clic sur l'en-tête d'un pas place l'aperçu sur le premier tick où ce pas est joué ; le pas joué est surligné dans la
  grille. Une rangée sous le minimum de groupes le dit (« sous le minimum : non joué en salle »).
- **Essai du brouillon** : le bloc « Essayer sur la salle » demande la zone et joue le **brouillon courant**, sur les vraies
  ampoules, **sans enregistrer** (`…/draft/try`, §11.10). Les mêmes refus que l'essai du catalogue s'affichent, plus
  « corrigez les erreurs ». « ■ Arrêter l'essai » l'interrompt.
- **Erreurs** : celles du serveur, avec le même texte que le catalogue ; « Enregistrer » est impossible tant qu'il en reste.

### 16.6 Quitter

- « Retour au catalogue » avec des modifications non enregistrées demande confirmation (« Modifications non enregistrées »).
  Changer de sous-onglet garde l'éditeur ouvert avec ses modifications. Quitter la page avec un éditeur modifié
  demande confirmation. Pour un lien vers une autre page ou Précédent, c'est la fenêtre de l'éditeur (Enregistrer /
  Abandonner / Rester). Fermer ou recharger l'onglet affiche l'invite du navigateur.
- Retirer un groupe qui n'est pas vide demande confirmation.

## 17. Compatibilité v11.7.0 → v11.8.0 — **changement cassant**

**`gN` ne désigne plus le dernier groupe.** C'est désormais un nom de variable ordinaire (comme `gA`). Pour le dernier
groupe, écrivez **`g*`**.

| Écriture v11.7.0 | En v11.8.0 |
|---|---|
| `gA: g1; +1`, `gB: g2; +2`, `gE: g3; 0`, listes, voisins | **inchangé**, même rendu |
| `gN: [theme, 100]` (cible) sans variable `gN` | **refusé** : « « gN » n'est pas déclarée dans variables… écrivez g* » |
| `gD: gN; -1` (départ) sans variable `gN` | **refusé** — même message |
| incrément au-delà de ±12 | accepté (refusé en v11.7.0) |

- **Correction** : remplacer `gN` par `g*`. Pour `gD: gN; -1`, écrire `gD: g*; -1`.
- Le fichier refusé est **écarté** comme tout fichier invalide : il figure dans `errors[]` et dans le bandeau du catalogue
  avec son nom, sa ligne et le message ; les autres effets restent chargés ; un réglage qui le citait joue la valeur par
  défaut du moment.
- **Aucun effet natif n'est concerné.** Le cas ne touche qu'un fichier écrit à la main entre la v11.7.0 et la v11.8.0.
- **Sens inverse** : un fichier v11.8.0 qui utilise `g*`, une expression, une variable en `N`, plus de 8 variables ou plus
  de 16 actions par pas est **refusé par un serveur v11.7.0** (message de validation). Ce sont des limites relevées : 24
  variables et 40 actions par pas.
