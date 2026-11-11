# Procédure de Test — Catalogue des effets : aperçu simulé et essai sur la salle (#299, milestone v11.7.0, LOT-3)

**Version** : v11.7.0.x (QUALIF)
**Date** : 2026-11-12
**Testeur** : QA (scénario AUTO) / Utilisateur (scénarios HUMAIN)
**Contrat** : `contracts/light-effects.md` §7.1 (catalogue), §7.2 (rechargement), §7.3 (aperçu), §7.4 (essai)
**Maquette** : `docs/mockup/v11.7.0/ui/ambiance_page__effects_catalogue_zones.html`
**Plan** : `.claude/workflow/plan-11.7.0.md` (LOT-3, tâches 13, 15, 16)
**Procédures voisines** : `light-effects-223.md` (réglages, bouton « Tester »), `light-effects-211.md` (effets natifs sur ampoules réelles)

## Prerequis

- [ ] Environnement : QUALIF (binaire Windows ; pont Hue réel pour le volet « ampoules réelles »)
- [ ] Données : zone Ambiance avec **4 groupes** d'une ampoule au moins ; un groupe animateur (1 ampoule) ; un groupe public
      (1 ampoule réglée à la main) ; équipes « Rouges » et « Bleus » ; un quiz avec une question chronométrée
- [ ] Accès : Ambiance › Lumière › Effets, section « Catalogue des effets » ; accès au dossier `data/files/effects/`
- [ ] Un fichier d'effet personnalisé valide (copie de `natifs/chenillard.yaml` avec un autre `id`) et un fichier cassé

## Scenarios

### Scenario 1 — Tests automatisés du lot

**Validation** : AUTO — **Issues** : #299

**Objectif** : l'aperçu est calculé par le moteur (aucun second interpréteur), l'API et le catalogue passent.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `cd server-go && go test -race ./internal/effects -run Preview -count=1` | Verts : une trame par tick (`tick`, `step` pre / pas:n / dernier), durée 20 s ou 5 s (score), `chrono_s`, tête du chenillard de g1 à gN, couleur d'urgence selon le chrono simulé de 20 s, trames identiques à `Render` pour chaque natif et 1 à 12 groupes, 400 trames au plus, `boucle: non` ⇒ `loop` faux | | |
| 2 | `cd server-go && go test -race ./internal/server -run TW299 -count=1` | Verts : forme de `/preview` (dont `tick`, `step`, `moment`, `tick_ms`, `chrono_s`, plus de `fade_ms`), `steps` du catalogue, `seconds` par défaut de l'essai, groupes par défaut (configurés, sinon 6), 400 / 404 / 405, aucune E/S vers le pont, essai du catalogue (corps vide ⇒ zone ambiance, ×1, 10 s) | | |
| 3 | `cd server-go/web && npx vitest run src/components/LightEffectsCatalogue` | Verts : cartes, aperçu rejoué (tick, chronomètre simulé), essai, « utilisé : … », erreurs | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 2 — Cartes du catalogue

**Validation** : HUMAIN — **Issues** : #299

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir Ambiance › Lumière › Effets, descendre au catalogue | Une carte par effet : Pulsation, Chenillard, Chenillard avec traînée, Couleur de l'équipe, Dégradé, puis les effets de l'utilisateur ; sous chaque nom l'étiquette **natif**, **personnalisé** ou **natif modifié** et le ou les moments (Chronomètre, Phases calmes, Équipe qui marque) | | |
| 2 | Lire la ligne « Fichier » | Nom du fichier, nombre de pas de la boucle (« 2 pas »), durée du tick (« tick de 1 s »), nombre de groupes demandé dans la zone (« demande 5 groupes ») | | |
| 3 | Copier un natif sous un autre `id` | La carte apparaît en quelques secondes, étiquette « personnalisé » | | |
| 4 | Copier `chenillard.yaml` en gardant `id: chenillard` | La carte du chenillard passe à « natif modifié » avec « remplace natifs/… — supprimez-le pour revenir à l'effet d'origine » | | |
| 5 | Chercher un bouton pour créer, modifier ou supprimer un effet | **Aucun** (l'éditeur est prévu en v11.8.0) | | |
| 6 | « ↻ Recharger les fichiers » après avoir déposé un fichier | Le fichier est pris en compte tout de suite | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 3 — Aperçu simulé tick par tick

**Validation** : HUMAIN — **Issues** : #299

**Objectif** : l'aperçu montre ce que l'effet fera, calculé par le serveur, sans toucher aux ampoules.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | « ▶ Aperçu » sur le **Chenillard** (zone Ambiance à 4 groupes) | Quatre ampoules dessinées étiquetées g1…g4 ; le point lumineux va de g1 à g4 puis repart, **un groupe par seconde** ; sous les ampoules « tick n · pas 0 » (le pas joué : « PRE », « pas n », « dernier » ; jamais « post ») **à chaque tick**, même quand les ampoules ne changent pas, et un compte à rebours « chronomètre simulé : 20 s » qui descend | | |
| 2 | Observer la couleur de la tête | Blanche, puis **orange** sous 10 s, puis **rouge** sous 5 s du compte à rebours simulé | | |
| 3 | Pendant l'aperçu, regarder les **vraies ampoules** | Elles ne bougent pas (aperçu purement visuel) | | |
| 4 | Lancer l'aperçu d'un autre effet | Le premier s'arrête : **un seul aperçu à la fois** ; le bouton redevient « ▶ Aperçu » | | |
| 5 | « ■ Arrêter » | L'aperçu s'arrête | | |
| 6 | Aperçu du **Chenillard avec traînée** alors que la zone n'a que 3 groupes | Bouton « Essayer » grisé (« Demande 5 groupes ») mais l'**aperçu reste disponible** : « simulation avec 6 groupes » | | |
| 7 | Aperçu du **Dégradé** (phases calmes) | Une couleur par groupe, scène fixe, « tick 0 · pas 0 », **pas** de compte à rebours (aucun chronomètre hors chronomètre) | | |
| 8 | Aperçu de **Couleur de l'équipe** (moment score) | Toutes les ampoules à la couleur de l'équipe simulée (rouge) ; l'aperçu dure **5 s** puis recommence (les autres : 20 s) | | |
| 9 | Aperçu d'un effet `boucle: non` (à créer) | Il joue une fois puis garde son état final | | |
| 10 | Casser le fichier de l'effet, recharger, demander l'aperçu | L'effet reste prévisualisable avec sa dernière version valide, ou « Aperçu indisponible » s'il a disparu ; l'interface reste utilisable | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 4 — Fichiers en erreur et catalogue vide

**Validation** : HUMAIN — **Issues** : #299

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Déposer un fichier avec une variable non déclarée, un autre au format précédent (`version: 1`) | Bandeau rouge « 2 fichiers n'ont pas pu être chargés » avec, pour chacun, le nom, la **ligne** et la cause en français ; « à réécrire en pas à pas » pour l'ancien format | | |
| 2 | Regarder les autres cartes | Inchangées, toujours jouables | | |
| 3 | Un seul fichier cassé | Message au singulier « 1 fichier n'a pas pu être chargé » | | |
| 4 | Réparer, recharger | Le bandeau disparaît | | |
| 5 | Rendre le dossier d'effets illisible (droits) | « Aucun effet disponible » et le bandeau ; les effets natifs, embarqués dans le serveur, restent joués en partie | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 5 — « utilisé : zone, moment »

**Validation** : HUMAIN — **Issues** : #299

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Réglages par défaut | Pulsation : « utilisé : Ambiance, chronomètre » ; les autres cartes sans mention | | |
| 2 | Régler le chenillard sur la zone Animateur au chronomètre à ×1,5, enregistrer | Carte du chenillard : « utilisé : Animateur, chronomètre (×1,5) » ; la vitesse n'apparaît que si elle n'est pas ×1 | | |
| 3 | Régler « Couleur de l'équipe » sur Animateur et Public | La mention cite les deux zones | | |
| 4 | Surcharger un effet dans l'ambiance d'une **manche** | La mention ne change pas : les manches ne sont pas listées | | |
| 5 | Retirer tous les groupes publics | La zone Public n'est plus citée (elle n'existe plus) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 6 — Volet « ampoules réelles » : « Essayer sur la salle » (zone Ambiance, ×1)

**Validation** : HUMAIN — **Issues** : #299

**Objectif** : le bouton du catalogue joue l'effet sur les **vraies ampoules de la zone Ambiance**, à la vitesse d'origine, puis rend l'éclairage à son état normal (contrat §7.4).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Régler la vitesse du chronomètre de la zone Ambiance sur ×2, puis « 💡 Essayer sur la salle (10 s) » sur le Chenillard | Le point tourne **à ×1** (un groupe par seconde, la vitesse réglée n'est pas appliquée) pendant 10 s sur les **ampoules de la zone Ambiance seulement** ; la zone Animateur reste en blanc fixe, la zone Public ne bouge pas | | |
| 2 | Observer la couleur de la tête pendant l'essai de 10 s | Blanc puis orange puis rouge (chronomètre simulé qui descend de 15 s) | | |
| 3 | Fin de l'essai | La zone Ambiance revient à sa scène normale ; le bouton redevient « 💡 Essayer… » | | |
| 4 | « ■ Arrêter l'essai » en cours | Retour immédiat à l'état normal | | |
| 5 | « Essayer » sur **Couleur de l'équipe** | Essai de **5 s** (durée choisie par le serveur pour un effet de score seul ; 10 s pour les autres) : toute la zone Ambiance prend la couleur de l'équipe simulée | | |
| 6 | Lancer un essai du catalogue puis le bouton « ▶ Tester » d'un réglage | Le test remplace l'essai : **un seul** à la fois ; et inversement | | |
| 7 | Chenillard avec traînée sur 3 groupes | Bouton grisé « Demande 5 groupes » | | |
| 8 | Popover Ambiance sur ON / OFF, ou Flash actif | Bouton grisé : « L'éclairage est forcé sur ON ou OFF, ou le Flash est actif : repassez en AUTO » ; l'aperçu reste disponible | | |
| 9 | Lancer une question | Bouton grisé : « Une question est en cours… » ; réactivé à la fin | | |
| 10 | Désactiver l'éclairage d'ambiance ; puis éteindre le pont | Bouton grisé « L'éclairage d'ambiance est désactivé » / « Pont injoignable » ; l'interface reste utilisable | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 7 — Volet « ampoules réelles » : les autres zones ne sont pas touchées par le catalogue

**Validation** : HUMAIN — **Issues** : #299

**Objectif** : l'essai du catalogue ne concerne que la zone Ambiance ; Animateur et Public passent par le bouton « Tester » des réglages.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Pendant un essai du catalogue, observer la lampe du groupe animateur | Blanc fixe, inchangée | | |
| 2 | Pendant un essai du catalogue, observer la lampe du public (réglée à la main) | Inchangée (rien n'est envoyé à la zone Public) | | |
| 3 | Zone Animateur / Public : essayer un effet | Via « ▶ Tester » des réglages (zone et vitesse réglées), pas via le catalogue ; à la fin de l'essai en zone Public la lampe retrouve son réglage d'avant | | |
| 4 | Redémarrer le serveur pendant un essai | L'essai n'est pas conservé : tout revient à l'état normal | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Criteres de Validation

- [ ] Scénario 1 (AUTO) vert, dont « les trames de l'aperçu sont celles de `Render` »
- [ ] Aperçu : g1…gN, tick, chronomètre simulé, couleur d'urgence, un seul à la fois, disponible même si l'effet est inapplicable (« simulation avec 6 groupes »)
- [ ] « Essayer sur la salle » : zone Ambiance seulement, ×1, un seul essai à la fois, retour à l'état normal, grisé avec la raison
- [ ] Erreurs de fichiers lisibles (nom, ligne, cause), le reste du catalogue intact ; aucun bouton d'édition
- [ ] « utilisé : zone, moment (×vitesse) » d'après les réglages globaux

## Notes QA

[Espace pour observations.]
