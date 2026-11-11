# Écrire un effet lumière

> **Version** : v11.7.0 (#223), guide en cours — les effets livrés et le schéma sont ceux du serveur courant.
> **Référence normative** : `contracts/light-effects.md` (§3 à §4). Ce guide en est la version pratique.
> **Interface** : Ambiance › Lumière › **Effets** (`/admin/ambiance`), voir `docs/ADMIN_GUIDE.md`.

Un **effet** décrit ce que font les ampoules pendant un moment du jeu : le chronomètre (`chrono`), l'attente
et les pauses (`calm`), l'attribution des points (`score`). Il s'écrit dans un fichier texte, sans recompiler le
serveur. Le moteur interprète le fichier ; vous ne programmez rien.

---

## 1. Où sont les fichiers

```
data/files/effects/              ← VOS effets (un fichier .yaml par effet)
data/files/effects/natifs/       ← copies de référence des effets livrés (NE PAS MODIFIER)
```

- Le dossier `natifs/` est **réécrit à chaque démarrage** : toute modification y est perdue. Il sert de référence.
- Les effets livrés sont lus **dans le serveur**, pas dans `natifs/`. Une mise à jour de BuzzControl met donc les
  effets livrés à jour.
- Les fichiers de `data/files/effects/` (hors `natifs/`) sont **les vôtres**. Ils sont sauvegardés avec la catégorie
  « ambiance » des sauvegardes.
- Un fichier est lu s'il est directement dans `effects/` et se termine par `.yaml`. Les sous-dossiers sont ignorés.
  Limites : **64 Ko** par fichier, **100** fichiers.
- Le nom du fichier est libre ; c'est le champ `id` qui identifie l'effet.

## 2. Créer un effet : copier un effet livré

Le plus simple est de partir d'un effet livré :

1. Copiez `data/files/effects/natifs/chenillard.yaml` dans `data/files/effects/`, par exemple sous le nom `mon-chenillard.yaml`.
2. **Changez la clé `id`** (par exemple `id: mon-chenillard`). Un `id` différent crée un **nouvel** effet, qui
   s'ajoute au catalogue.
3. Modifiez les apparences (`looks`), la vitesse (`step_ms`), les étapes (`stages`).
4. Enregistrez le fichier. Le serveur le recharge dans les 2 secondes ; rien à redémarrer.
5. Dans Ambiance › Lumière › Effets, l'effet apparaît dans la liste de son moment.

> **Attention** : si vous gardez l'`id` d'un effet livré, votre fichier **remplace** l'effet livré (origine
> « surcharge »). Pour revenir à l'effet d'origine, supprimez votre fichier : le serveur reprend le natif.

## 3. Structure d'un fichier

```yaml
# Commentaires : une ligne qui commence par #.
id: mon-effet               # identifiant : a-z, 0-9, tiret, 1 à 40 caractères, unique
name: Mon effet             # libellé du catalogue, 1 à 40 caractères
description: >              # texte du catalogue, 500 caractères au plus
  Une phrase qui dit ce que voit la salle.
version: 1                  # toujours 1 pour l'instant
moments: [chrono]           # où l'effet est proposé : chrono, calm, score (au moins un)
target: groups              # groups (un état par groupe) ou all (un état pour toute la zone)
min_groups: 3               # target: groups seulement ; défaut 1 ; 1 à 12
duration_ms: 0              # 0 = dure tout le moment ; > 0 = s'arrête seul (obligatoire pour score)

looks:                      # apparences nommées, réutilisées par les motifs
  fond:    { color: theme,   intensity: 100 }
  tete:    { color: urgency, intensity: 100, fade_ms: 300 }

pattern:                    # un seul motif de base
  type: shift
  step_ms: 1000
  cells: [tete]
  rest: fond

stages:                     # variantes conditionnelles (6 au plus) ; facultatif
  - when: { groups_lt: 5 }
    pattern: { cells: [tete] }
```

Clés de premier niveau : `id`, `name`, `description`, `version`, `moments`, `target`, `min_groups`, `duration_ms`,
`looks`, `pattern`, `stages`. Obligatoires : `id`, `name`, `version`, `moments`, `looks`, `pattern`.
**Une clé inconnue est une erreur**, jamais un réglage ignoré en silence : vérifiez l'orthographe.

### 3.1 Moments (`moments`)

| Moment | Quand l'effet joue |
|--------|--------------------|
| `chrono` | Pendant le décompte d'une question |
| `calm` | Attente, pause, entracte |
| `score` | Pendant l'attribution des points (durée `duration_ms` ou durée de l'impulsion SCORE) |

`score` prime sur `chrono` et `calm` pendant sa durée.

### 3.2 Cible (`target`) et nombre de groupes (`min_groups`)

| `target` | Ce qui est piloté | Applicable quand |
|----------|-------------------|------------------|
| `all` | **Toute** la zone générale, un seul état | Toujours (même sans groupe) |
| `groups` | **Chaque** groupe de salle non vide reçoit son propre état | Le nombre de groupes de salle non vides est ≥ `min_groups` |

- `min_groups` ne concerne que `target: groups`. Avec `target: all`, la clé doit être absente ou valoir 1.
- Un effet `target: groups` **n'est pas applicable** au groupe animateur si `min_groups` vaut plus de 1 ; il est alors
  grisé dans le sélecteur, avec son motif.
- Avec `target: all`, seul le motif `uniform` est accepté. Les conditions `groups_lt` et `groups_gte` sont refusées.
- Un effet inapplicable joue la valeur par défaut du moment (`pulsation` pour `chrono`, rien pour `calm` et `score`).

### 3.3 Apparences (`looks`)

Une apparence est une couleur et une intensité, nommée pour être réutilisée.

```yaml
looks:
  fond:    { color: theme,   intensity: 100 }   # couleur du thème de la question
  pale:    { color: "#FFAA00", intensity: 60, fade_ms: 500 }   # couleur fixe, fondu de 500 ms
```

| Clé | Valeurs | Défaut |
|-----|---------|--------|
| `color` | `scene` (couleur de la scène de salle du moment), `theme` (thème de la question ; blanc à défaut), `team` (équipe créditée, moment `score`), `urgency` (pendant le chronomètre : blanc, orange `#FFAA00` à 10 s ou moins, rouge `#E61E1E` à 5 s ou moins ; hors chronomètre : blanc), ou `"#RRGGBB"` | `scene` |
| `intensity` | 0 à 100 (pourcentage ; 0 = éteint) | 100 |
| `fade_ms` | 0 à 5000 : fondu confié au pont | 0 |

Un `fade_ms` élevé entre deux teintes éloignées peut délaver la couleur (interpolation du pont). Ce n'est pas une
erreur, mais testez sur les ampoules.

### 3.4 Motifs (`pattern.type`)

Un motif décide **comment** les apparences se répartissent dans le temps et entre les groupes. Les durées de pas
(`step_ms`, `ms`) vont de 250 à 60 000 ms.

#### `uniform` — toute la salle alterne

```yaml
pattern:
  type: uniform
  phases:                   # 1 à 8 phases, durées de 250 ms au moins
    - { look: haut, ms: 500 }
    - { look: bas,  ms: 500 }
```

Tous les groupes (ou toute la zone si `target: all`) montrent la même apparence, qui alterne selon `phases`. Le
cycle est calé sur l'horloge murale, comme la pulsation.

#### `shift` — un bloc se déplace

```yaml
pattern:
  type: shift
  step_ms: 1000
  cells: [trainee, tete, trainee]   # 1 à 12 apparences : le bloc qui se déplace
  rest: fond                        # apparence des groupes hors du bloc (obligatoire)
  wrap: true                        # true (défaut) : le bloc revient de l'autre côté
  direction: forward                # forward (défaut), backward ou bounce
```

Le bloc `cells` avance d'un groupe à chaque pas ; les autres groupes montrent `rest`.

#### `gauge` — une jauge qui se remplit ou se vide

```yaml
pattern:
  type: gauge
  source: remaining        # remaining (temps restant) ou elapsed (temps écoulé)
  on: tete                 # apparence des groupes allumés
  off: fond                # apparence des groupes éteints
  edge: pale               # facultatif : apparence du groupe frontière
  direction: backward      # forward ou backward
```

Les groupes allumés sont au prorata de la source. Recalcul à chaque seconde du chronomètre.

#### `random` — des groupes qui s'allument au hasard

```yaml
pattern:
  type: random
  step_ms: 800
  ratio: 0.3               # OU count: 3 (jamais les deux)
  on: pale
  off: fond
```

À chaque pas, un tirage choisit les groupes `on` ; les autres sont `off`. Le tirage est déterministe pour une graine
donnée.

#### `gradient` — un dégradé réparti sur les groupes

```yaml
pattern:
  type: gradient
  stops: [crete, equipe]   # 2 à 6 apparences
  step_ms: 1000
  travel: outward          # none, forward, backward ou outward
  origin: team             # first, last, center ou team (origin n'existe qu'avec outward)
  repeat: false
  rest: fond               # accepté seulement pour une propagation
```

Sémantique exacte :

- **`travel: none`** — dégradé **fixe** : le premier groupe montre le premier arrêt, le dernier groupe le dernier arrêt,
  les autres sont interpolés. Pas de retour au début ; `repeat`, `step_ms` et `origin` sont sans effet.
- **`travel: forward` ou `backward`, `repeat: true`** (défaut) — **anneau qui tourne** : le dégradé est fermé (après le
  dernier arrêt, il revient au premier), réparti sur un tour complet des groupes, sans couture. Il avance d'un groupe
  par pas.
- **Propagation** — `travel: outward`, ou `forward` / `backward` avec `repeat: false` : le dégradé se **propage dans le
  temps** à partir d'un groupe de départ (`origin`). Ce n'est pas une atténuation selon la distance :
  - un groupe encore non atteint montre `rest` (s'il n'y a pas de `rest`, il garde la scène de salle) ;
  - un groupe atteint parcourt les arrêts **un par pas**, dans l'ordre ;
  - `repeat: false` : au-delà du dernier arrêt, il reste dessus ;
  - `repeat: true` : il reboucle, et des ondes successives partent du même point.

Exemple de propagation (5 groupes, départ au groupe 2, arrêts `crete` puis `equipe`, `repeat: false`, `rest: fond`) :

```
pas 0 :  fond   CRETE  fond   fond   fond
pas 1 :  CRETE  equipe CRETE  fond   fond
pas 2 :  equipe equipe equipe CRETE  fond
pas 3 :  equipe equipe equipe equipe CRETE
pas 4 :  equipe equipe equipe equipe equipe   (tenu)
```

### 3.5 Étapes (`stages`)

Une étape remplace des parties du motif quand une **condition** est vraie. Elle est écrite comme une liste, et **la
première dont toutes les conditions sont vraies** s'applique (6 étapes au plus).

```yaml
stages:
  - when: { remaining_lte: 5 }          # 5 secondes ou moins avant la fin du chrono
    looks:
      fond: { color: "#E61E1E", intensity: 30 }   # remplace l'apparence « fond »
    pattern:
      step_ms: 250                      # remplace la clé du motif de base
  - when: { groups_lt: 5 }
    pattern: { cells: [tete] }
```

Conditions (`when`) :

| Condition | Vraie quand |
|-----------|-------------|
| `remaining_lte: <s>` | Le temps restant est ≤ `<s>` secondes (chrono seulement) |
| `remaining_gt: <s>` | Le temps restant est > `<s>` secondes (chrono seulement) |
| `groups_lt: <n>` | Le nombre de groupes de salle non vides est < `<n>` |
| `groups_gte: <n>` | Le nombre de groupes de salle non vides est ≥ `<n>` |

Hors `chrono`, les conditions `remaining_*` sont fausses. Aucune étape vraie ⇒ le motif de base.

### 3.6 Contexte fourni par le serveur

Vos effets n'ont pas à le déclarer ; il est fourni automatiquement :

- **N** : le nombre de groupes de salle **non vides** (un groupe vide est ignoré).
- **Temps restant et durée** du chronomètre (moment `chrono`).
- **Couleurs** : `scene`, `theme` (thème de la question), `team` (équipe créditée).
- **Groupe d'origine** (`origin: team`) : le groupe dont `near_teams` contient l'équipe créditée ; à défaut, le groupe 1.
- **Graine** : l'heure de démarrage de l'effet (pour `random`).

Un groupe animateur est rendu seul (N = 1). Son effet ne peut donc utiliser que `target: all` ou un `min_groups` de 1.

## 4. Erreurs de validation

Une erreur porte le **fichier**, la **ligne** et un **message en français** :

```
mon-effet.yaml, ligne 14 : l'apparence « tete » n'est pas définie dans looks
```

Erreurs les plus fréquentes :

| Message (type) | Cause |
|----------------|-------|
| YAML illisible | Indentation ou deux-points mal placés |
| clé inconnue | Faute de frappe dans une clé |
| clé obligatoire absente | `id`, `name`, `version`, `moments`, `looks` ou `pattern` manque |
| `id` invalide ou en double | Caractères hors a-z, 0-9, tiret ; ou même `id` que deux fichiers utilisateur |
| `version` non prise en charge | Seule `version: 1` existe |
| `moments` vide ou valeur inconnue | Utiliser `chrono`, `calm` ou `score` |
| type de motif inconnu | Utiliser `uniform`, `shift`, `gauge`, `random` ou `gradient` |
| paramètre hors bornes | Pas < 250 ms, `intensity` > 100, `fade_ms` > 5000, `ratio` hors de 0–1 |
| apparence non définie | Un nom cité dans un motif ou une étape n'existe pas dans `looks` |
| couleur invalide | Ni `scene`, `theme`, `team`, `urgency`, ni `"#RRGGBB"` |
| `duration_ms` absent pour `score` | Un effet `score` doit dire combien de temps il dure |
| fichier trop gros | Plus de 64 Ko |

**Un fichier invalide ne casse rien** : il est écarté, son erreur est signalée (dans la réponse de
`GET /api/lighting/effects`, champ `errors`, et une ligne de journal), et les autres effets restent disponibles. La
version précédente du même `id` reste active si elle existe en mémoire ; sinon le natif de même `id`.

## 5. Rechargement

- Le serveur vérifie le dossier **toutes les 2 secondes** et recharge en cas de changement (nom, taille ou date).
- `POST /api/lighting/effects/reload` force un rechargement immédiat.
- Le rechargement est atomique. Un effet en cours repart du début avec sa nouvelle définition.
- Pas besoin de redémarrer le serveur.

## 6. Revenir à l'effet d'origine

- **Effet livré** : supprimez (ou renommez hors `.yaml`) votre fichier qui porte le même `id`. Le natif reprend au
  rechargement suivant.
- **Effet à vous** : supprimez son fichier. Un moment qui le réglait joue alors sa valeur par défaut (`pulsation` pour
  `chrono`, aucun effet pour `calm` et `score`).

## 7. Effets livrés à ce jour

| `id` | Motif | Moments | Rendu |
|------|-------|---------|-------|
| `pulsation` | `uniform`, `target: all` | chrono | Respiration de la salle, orange puis rouge en fin de chrono (rendu identique à la pulsation d'origine ; le code historique la joue encore, sa migration vers le moteur est le dernier lot de la v11.7.0) |
| `chenillard` | `shift`, `target: groups`, `min_groups: 3` | chrono | Une tête `urgency` et une traînée à 50 % sur le fond du thème ; sans traînée sous 5 groupes |
| `couleur-equipe` | `uniform`, `target: all` | score | Couleur de l'équipe créditée, fixe, le temps de l'impulsion SCORE |

Les autres effets prévus (`remplissage`, `vague`, `vague-lente`, `degrade`, `scintillement`) arrivent dans les lots
suivants. Le motif `gradient` est déjà disponible dans le moteur : un effet utilisateur peut l'employer dès maintenant.

## 8. Vérifier son effet

- **Catalogue** : Ambiance › Lumière › Effets. L'effet apparaît dans la liste de ses moments ; une raison s'affiche
  s'il est inapplicable (« demande 3 groupes, 2 configurés »).
- **Erreurs** : `GET /api/lighting/effects` (champ `errors`) ou le journal du serveur.
- **Essai et aperçu** : prévus dans les lots suivants (essai d'un effet, aperçu simulé).
