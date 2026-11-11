# Procédure de Test — Effets lumière pas à pas, par zone, vitesse et « Tester » (#223, milestone v11.7.0, LOT-2, révision 4)

**Version** : v11.7.0.x (QUALIF)
**Date** : 2026-11-11 (révision 4 : un effet et une vitesse par **zone** (Ambiance, Animateur, Public) et par moment, bouton « Tester », zone Public)
**Testeur** : QA (scénarios AUTO) / Utilisateur (scénarios HUMAIN)
**Contrat** : `contracts/light-effects.md` §3 à §7, §9 ; `contracts/ambiance-settings.md` §10
**Maquette** : `docs/mockup/v11.7.0/ui/ambiance_settings_panel__light_effects_zones.html`
**Plan** : `.claude/workflow/plan-11.7.0.md` (LOT-2, tâches Z5 à Z11)
**Procédure complémentaire** : `light-effects-211.md` (chenillard 4/6/8 groupes, vitesse, « Tester » ×2, zone Public, dégradé)
**Guide** : `docs/LIGHT_EFFECTS.md` (« Écrire un effet »)

## Prerequis

- [ ] Environnement : QUALIF (binaire Windows, pont Hue réel pour le volet « ampoules réelles »)
- [ ] Données : au moins 8 ampoules générales, rangées en **8 groupes de salle** (cf. `light-groups-225.md`, groupes sans nom), puis
      réductibles à 4 et 6 groupes ; **deux groupes de la zone Animateur** (1 ampoule chacun) et **un groupe de la zone Public** (1 ampoule, réglée à la main) ; équipes « Rouges » et « Bleus » ;
      un quiz avec une question chronométrée de 30 s
- [ ] Accès : page Ambiance › Lumière › Effets ; accès au dossier `data/files/effects/` de la QUALIF (explorateur de fichiers)
- [ ] Un éditeur de texte ordinaire (Bloc-notes) ; le guide `docs/LIGHT_EFFECTS.md` ouvert

## Scenarios

### Scenario 1 — Tests automatisés du lot

**Validation** : AUTO — **Issues** : #223

**Objectif** : chronogramme §4.5, variables, erreurs de fichiers, natifs, lecteur, vitesse, réglages et API passent.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `cd server-go && go test -race ./internal/effects ./internal/ambiancecfg ./internal/ambiance -count=1` | Tous verts, dont `TestChronogram_ReferenceTable_TickByTick` (chronogramme de référence du §4.5) | | |
| 2 | `cd server-go && go test -race ./internal/server ./cmd/server -run 'TW223' -count=1` | Tous verts (réglages par zone, vitesses, `/api/lighting/effects` par zone, `…/try` et `…/try/stop`, sauvegarde, `ROUNDS_UPDATE`) | | |
| 3 | `cd server-go/web && npx vitest run src/components/AmbianceSettingsPanel src/utils/lightEffects src/pages/AmbiancePage.tryButton.test.jsx` | Tous verts | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 2 — Les natifs et le catalogue

**Validation** : HUMAIN — **Issues** : #223

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Page Effets, listes d'effets | Natifs proposés : **Pulsation**, **Chenillard** (dès 3 groupes), **Chenillard avec traînée** (dès 5 groupes), **Couleur de l'équipe**, **Dégradé** (dès 2 groupes) ; plus de Remplissage, Scintillement, Vague | | |
| 2 | Avec 2 groupes dans la zone Ambiance, ouvrir la liste « Pendant le chronomètre » | Chenillard et Chenillard avec traînée grisés avec « demande 3 groupes, 2 dans cette zone » / « demande 5 groupes, 2 dans cette zone » | | |
| 3 | Mêmes listes dans le bloc de la zone Animateur (1 groupe) puis de la zone Public | La raison cite le nombre de groupes **de cette zone** ; **tous** les effets sont proposés à chaque zone (plus de limitation « g0 seul » pour l'animateur), grisés seulement si la zone a trop peu de groupes | | |
| 4 | Ouvrir `data/files/effects/natifs/` | Copies de référence en français (`nom:`, `pas:`…), en-tête « ne pas modifier ici » | | |
| 5 | Chaque effet du catalogue | Une durée de tick (`tick_ms`) est affichée ou utilisée pour l'étiquette « plafonné » (scénario 6) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 3 — Écrire un effet avec une variable (copier un natif)

**Validation** : HUMAIN — **Issues** : #223

**Objectif** : un effet se crée en copiant un natif et en changeant quelques lignes, sans redémarrer ni recompiler.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Copier `natifs/chenillard.yaml` dans `data/files/effects/`, le renommer `mon-chenillard.yaml`, changer `id: mon-chenillard` et `nom: Mon chenillard` | — | | |
| 2 | Attendre au plus 3 s (ou « Recharger les fichiers ») | « Mon chenillard (personnalisé) » apparaît dans la liste du chronomètre | | |
| 3 | Dans le fichier, remplacer la variable par `gA: gN; -1` (part du dernier groupe et recule) | Le point lumineux tourne dans l'autre sens, sans redémarrage | | |
| 4 | Ajouter une seconde variable `gB: g2; +2` et une action `gB: ["#FF0000", 100]` dans `pas` | Un second point rouge, deux fois plus rapide dans sa progression | | |
| 5 | Remplacer l'action par `gA+1: ["#00FF00", 100]` | Le voisin du point (le groupe suivant, avec rebouclage sur le dernier) passe au vert | | |
| 6 | Ajouter `dernier: { g0: [theme, 100] }` | À chaque tour complet, toute la salle revient au thème avant que le point reparte | | |
| 7 | Ajouter `boucle: non` | Un seul tour, puis l'état final est tenu jusqu'à la fin du chronomètre | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 4 — Casser un fichier : message fichier + ligne, le reste continue

**Validation** : HUMAIN — **Issues** : #223

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Dans `mon-chenillard.yaml`, remplacer `gA` par `gB` dans une action (variable non déclarée), enregistrer | — | | |
| 2 | Attendre au plus 3 s, regarder la page Effets | Bandeau d'erreur : « mon-chenillard.yaml, ligne N : la variable « gB » n'est pas déclarée dans variables » (nom du fichier, bon numéro de ligne, texte en français) | | |
| 3 | Vérifier le chenillard natif et la pulsation | Toujours disponibles ; le serveur n'a pas planté | | |
| 4 | Lire le journal du serveur | UNE seule ligne pour ce fichier (pas une par cycle de 2 s) | | |
| 5 | Essayer : une clé en anglais (`name:`), `duree_tick_ms: 100`, une couleur `#GG0000`, une intensité à 150, `groupe1:` comme cible | Un message distinct pour chaque, chacun avec fichier et ligne | | |
| 6 | Copier un ancien fichier au format précédent (`version: 1`, `looks`, `pattern`) | Message « format d'effet de la version précédente : réécrire en pas à pas » | | |
| 7 | Pendant une partie, casser le fichier de l'effet en cours de lecture | La dernière version valide continue de jouer (pas de coupure) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 5 — Surcharge d'un natif et restauration

**Validation** : HUMAIN — **Issues** : #223

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Copier `chenillard.yaml` en gardant **`id: chenillard`**, changer une couleur, enregistrer | Le catalogue indique « natif modifié » ; le chenillard joue la version modifiée | | |
| 2 | Supprimer (ou renommer en `.off`) le fichier copié | Retour à l'effet d'origine en moins de 3 s, sans redémarrage | | |
| 3 | Modifier la copie de référence dans `natifs/` | Aucun effet sur le rendu (les natifs viennent du binaire) ; réécrite au redémarrage | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 6 — Effet et vitesse par zone et par moment, global et par manche

**Validation** : HUMAIN — **Issues** : #223

**Objectif** : un bloc par zone, chacun de trois sélecteurs d'effet avec sa vitesse (maquette `zones`).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Page Effets, réglages globaux, zone Ambiance | Chronomètre = « Pulsation (par défaut) », Phases calmes = « Aucun effet », Équipe qui marque = « Aucun effet » ; à côté de chacun un sélecteur de vitesse sur **×1** (grisé quand aucun effet n'est joué) | | |
| 2 | Blocs « Zone Animateur » et « Zone Public » (avec au moins un groupe chacune) | Chaque bloc a ses trois moments ; première option « Aucun effet (blanc fixe) » pour l'animateur, « Aucun effet » pour le public ; mention « sans effet : blanc fixe » / « sans effet : non pilotée » | | |
| 3 | Retirer tous les groupes publics (page Groupes), revenir aux Effets | Le bloc « Zone Public » disparaît, remplacé par « Aucun groupe public — en créer un » ; les réglages déjà faits pour cette zone sont **conservés** (ils reviennent avec le premier groupe public) | | |
| 4 | Choisir le Chenillard pour le chronomètre de la zone Ambiance, enregistrer, F5 | Choix conservé, **sans toucher** aux deux autres zones | | |
| 5 | Ouvrir la liste des vitesses | ×0,5 · ×0,75 · ×1 · ×1,5 · ×2 | | |
| 6 | Choisir **×2** pour le chronomètre de l'ambiance, enregistrer, F5 | La vitesse est conservée ; changer d'effet ne la change pas ; les vitesses des autres zones restent ×1 | | |
| 7 | Choisir un effet à tick de 400 ms (copie d'un natif, `duree_tick_ms: 400`) en ×2 | Étiquette « plafonné » : un effet ne descend jamais sous 250 ms par tick | | |
| 8 | « Ambiance de la manche » : sélecteurs en « Hérité (Chenillard) » et vitesse « Hérité (×2) », pour chaque zone | Les valeurs globales de la zone sont affichées entre parenthèses ; **aucun** bouton « Tester » dans cet éditeur | | |
| 9 | Surcharger le chronomètre de la manche par « Pulsation » ×0,5 (zone Ambiance) | Badge « manche » ; la question de cette manche pulse lentement, les autres jouent le chenillard rapide | | |
| 10 | Repasser en « Hérité » | La surcharge disparaît | | |
| 11 | Supprimer le fichier d'un effet choisi | Avertissement « absent ou invalide », valeur conservée « (introuvable) » ; le repli dépend de la zone : aucun effet (ambiance), blanc fixe (animateur), rien d'envoyé (public) | | |
| 12 | Réordonner ou supprimer un groupe de la zone Animateur | Les réglages d'effet de la zone **ne changent pas** (le groupe n'est plus la clé d'aucun réglage) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 6b — Bouton « Tester » (à côté de chaque choix d'effet)

**Validation** : HUMAIN — **Issues** : #223

**Objectif** : essayer un effet, tel qu'il sera joué (zone, vitesse réglée), sans lancer de partie (contrat §7.4).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Zone Ambiance, chronomètre = Chenillard ×1, cliquer « ▶ Tester » | Réponse immédiate ; l'effet joue environ 10 s sur les ampoules **de la zone Ambiance seulement** ; le bouton devient « ■ Arrêter » (infobulle : zone et vitesse) | | |
| 2 | Pendant le test, changer la vitesse (×2) | Le test s'arrête ; relancer : le point va deux fois plus vite | | |
| 3 | Lancer un test dans une autre zone pendant le premier | **Un seul** test à la fois : le second remplace le premier ; l'ancienne zone retrouve son état vivant | | |
| 4 | « ■ Arrêter » | La zone retrouve aussitôt son état vivant (scène, blanc fixe pour l'animateur) | | |
| 5 | Test d'un effet inapplicable à la zone, ou sans effet choisi | Bouton grisé avec la raison (« Effet inapplicable à cette zone », « Aucun effet choisi ») | | |
| 6 | Popover Ambiance sur ON / OFF ou Flash actif, tester la zone **Ambiance** | Refusé : bouton grisé / message « L'éclairage est forcé sur ON / OFF ou le Flash est actif » ; la zone **Animateur** reste testable | | |
| 7 | Lancer une question, revenir sur la page Effets | Boutons « Tester » grisés (« Une question est en cours ») ; réactivés à la fin | | |
| 8 | Éclairage d'ambiance désactivé, ou pont éteint | Bouton grisé (désactivé) / message « Pont injoignable » ; l'interface reste utilisable | | |
| 9 | Test en zone Public (lampe réglée à la main avant) | L'effet joue sur la lampe du public ; **à la fin la lampe retrouve exactement son réglage d'avant** (lu sur le pont puis réécrit) ; éteinte si la lecture était impossible | | |
| 10 | Remarque (revue M3) : tester la zone Public avec un pont qui répond mal (lent ou partiellement coupé) | Le démarrage d'un test ou d'un essai en zone Public lit d'abord l'état des lampes sur le pont (1 requête, 1,5 s au plus) : le bouton « Tester » peut donc **répondre avec jusqu'à 1,5 s de latence** ; noter le délai observé ; l'interface reste utilisable et, si la lecture échoue, les lampes sont éteintes à la fin | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 7 — Chenillard, vitesse, dégradé : voir `light-effects-211.md`

**Validation** : HUMAIN — **Issues** : #223, #211

Les scénarios « ampoules réelles » des effets natifs (chenillard sur 4, 6 et 8 groupes avec relevés §8.2, vitesse ×0,5 / ×2, pulsation au nouveau rendu, couleur de l'équipe, dégradé, essai ×1 du catalogue, zone Public) sont dans `tests/procedures/light-effects-211.md`.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Jouer `light-effects-211.md` | Verdict consigné dans cette procédure-là | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 8 — Précédence : ON / OFF / Flash priment sur l'effet

**Validation** : HUMAIN — **Issues** : #223

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Chenillard en cours, popover Ambiance : OFF | Salle éteinte, effet interrompu | | |
| 2 | AUTO | L'effet reprend (état vivant) | | |
| 3 | ON puis Flash | Salle blanche / clignotante, pas de chenillard | | |
| 4 | Ampoule d'équipe pendant l'effet | Garde la couleur de son équipe, jamais modifiée par l'effet | | |
| 4b | Effet joué sur la zone Animateur et sur la zone Public pendant ON / OFF / Flash | Ni ON, ni OFF, ni Flash ne touchent ces deux zones : leur effet continue | | |
| 5 | Ampoule générale **hors groupe** pendant un chenillard | Appartient à la zone Ambiance : suit `g0` (le fond), jamais la tête | | |
| 6 | Interrupteur général désactivé en plein effet, puis arrêt du serveur | Tout s'éteint (zones Animateur et Public comprises), aucune ampoule ne se rallume toute seule | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 9 — Zones Animateur et Public : un effet et une vitesse par moment et par zone

**Validation** : HUMAIN — **Issues** : #223

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Bloc « Zone Animateur », liste du chronomètre | « Aucun effet (blanc fixe) » par défaut ; tous les effets proposés, grisés seulement si la zone a trop peu de groupes | | |
| 2 | Zone Animateur : « Quand une équipe marque » = « Couleur de l'équipe » ; zone Public : « Aucun effet » ; enregistrer | — | | |
| 3 | Attribuer des points à « Rouges » | **Toute** la zone Animateur (ses deux groupes) prend la couleur des Rouges, puis redevient blanche ; la zone Public ne change pas ; la salle suit son propre effet | | |
| 4 | Zone Animateur : « Pulsation » ×2 ; zone Ambiance : « Pulsation » ×0,5 ; zone Public : « Pulsation » ×1 | Les trois zones pulsent à des vitesses différentes, **indépendamment** | | |
| 5 | Zone Public : un effet « Couleur de l'équipe » ; attribuer des points | La lampe du public prend la couleur de l'équipe ; **à la fin du moment, l'état posé par l'effet (POST) est tenu** (c'est la seule zone où il est visible) ; sans effet, plus rien n'est envoyé à cette zone | | |
| 6 | Réordonner les groupes de la zone Animateur | Les réglages d'effet et de vitesse **ne changent pas** (aucune dépendance au groupe) | | |
| 7 | Supprimer tous les groupes de la zone Public, enregistrer, recréer un groupe public | Les réglages de la zone Public sont conservés ; sans effet, la zone redevient non pilotée | | |
| 8 | OFF / ON / Flash dans le popover | Aucune des deux zones n'est modifiée | | |
| 9 | Même réglage par manche (mode surcharge) | Valeur de manche prioritaire par zone, par moment et par réglage (effet / vitesse), « Hérité » sinon | | |
| 10 | Config / rounds.json issus de la QUALIF 11.7.0.x antérieure (`effects` à plat, `host_effects`, `host_speeds`) | Le serveur démarre ; les valeurs à plat sont lues comme la zone Ambiance, les `host_*` ignorés sans erreur ; les groupes animateur existants sont en blanc fixe | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 10 — Sauvegarde et restauration des fichiers d'effets

**Validation** : AUTO — **Issues** : #223

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `cd server-go && go test ./internal/server -run 'TW223.*Backup|TW223.*Restore' -count=1` | Verts : `files/effects/` (hors `natifs/`) dans la catégorie `ambiance`, restauration vers le même dossier | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Criteres de Validation

- [ ] Écrire un effet avec une variable par copie, le casser (message fichier + ligne), le restaurer : sans redémarrage
- [ ] Effet et vitesse réglables par zone et par moment, global et par manche ; « plafonné » sous 250 ms
- [ ] Bouton « Tester » : un seul à la fois, à la vitesse réglée, refusé pendant une question, état rétabli en zone Public
- [ ] Chenillard, vitesse, dégradé sur ampoules réelles : voir `light-effects-211.md`
- [ ] Précédence ON / OFF / Flash respectée, aucune écriture fantôme à l'arrêt
- [ ] Zones Animateur et Public : effet et vitesse par moment et par zone, insensibles à la salle, à ON / OFF / Flash et entre elles

## Notes QA

[Espace pour observations.]
