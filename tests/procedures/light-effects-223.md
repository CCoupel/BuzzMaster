# Procédure de Test — Effets lumière pas à pas, vitesse et chenillard (#223, milestone v11.7.0, LOT-2, révision 3)

**Version** : v11.7.0.x (QUALIF)
**Date** : 2026-11-11 (révision 3 : format d'effet « pas à pas » à variables de groupe, vitesse réglable, groupes sans nom)
**Testeur** : QA (scénarios AUTO) / Utilisateur (scénarios HUMAIN)
**Contrat** : `contracts/light-effects.md` §3 à §7, §9 ; `contracts/ambiance-settings.md` §10
**Maquette** : `docs/mockup/v11.7.0/ui/ambiance_settings_panel__light_effects_numbered.html`
**Plan** : `.claude/workflow/plan-11.7.0.md` (LOT-2, tâches R5 à R14)
**Guide** : `docs/LIGHT_EFFECTS.md` (« Écrire un effet »)

## Prerequis

- [ ] Environnement : QUALIF (binaire Windows, pont Hue réel pour le volet « ampoules réelles »)
- [ ] Données : au moins 8 ampoules générales, rangées en **8 groupes de salle** (cf. `light-groups-225.md`, groupes sans nom), puis
      réductibles à 4 et 6 groupes ; **deux groupes animateur** (1 ampoule chacun) ; équipes « Rouges » et « Bleus » ;
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
| 2 | `cd server-go && go test -race ./internal/server ./cmd/server -run 'TW223' -count=1` | Tous verts (réglages, vitesses, `/api/lighting/effects`, sauvegarde, `ROUNDS_UPDATE`) | | |
| 3 | `cd server-go/web && npx vitest run src/components/AmbianceSettingsPanel.223.test.jsx src/utils/lightEffects.223.parity.test.js` | Tous verts | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 2 — Les natifs et le catalogue

**Validation** : HUMAIN — **Issues** : #223

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Page Effets, listes d'effets | Natifs proposés : **Pulsation**, **Chenillard** (dès 3 groupes), **Chenillard avec traînée** (dès 5 groupes), **Couleur de l'équipe**, **Dégradé** (dès 2 groupes) ; plus de Remplissage, Scintillement, Vague | | |
| 2 | Avec 2 groupes configurés, ouvrir la liste « Pendant le chronomètre » | Chenillard et Chenillard avec traînée grisés avec « demande 3 groupes, 2 configurés » / « demande 5 groupes… » | | |
| 3 | Bloc d'un groupe animateur | Seuls **Pulsation** et **Couleur de l'équipe** sont proposés ; les autres grisés « anime plusieurs groupes » | | |
| 4 | Ouvrir `data/files/effects/natifs/` | Copies de référence en français (`nom:`, `pas:`…), en-tête « ne pas modifier ici » | | |

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

### Scenario 6 — Effet et vitesse par moment, global et par manche

**Validation** : HUMAIN — **Issues** : #223

**Objectif** : trois sélecteurs d'effet, chacun avec sa vitesse (maquette `numbered`).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Page Effets, réglages globaux | Chronomètre = « Pulsation (par défaut) », Phases calmes = « Aucun effet », Équipe qui marque = « Aucun effet » ; à côté de chacun un sélecteur de vitesse sur **×1** (grisé quand aucun effet n'est joué) | | |
| 2 | Choisir le Chenillard pour le chronomètre, enregistrer, F5 | Choix conservé | | |
| 3 | Ouvrir la liste des vitesses | ×0,5 · ×0,75 · ×1 · ×1,5 · ×2 | | |
| 4 | Choisir **×2** pour le chronomètre, enregistrer, F5 | La vitesse est conservée ; changer d'effet ne la change pas | | |
| 5 | Choisir un effet à tick de 400 ms (copie d'un natif, `duree_tick_ms: 400`) en ×2 | Étiquette « plafonné » : un effet ne descend jamais sous 250 ms par tick | | |
| 6 | « Ambiance de la manche » : sélecteurs en « Hérité (Chenillard) » et vitesse « Hérité (×2) » | Les valeurs globales sont affichées entre parenthèses | | |
| 7 | Surcharger le chronomètre de la manche par « Pulsation » ×0,5 | Badge « manche » ; la question de cette manche pulse lentement, les autres jouent le chenillard rapide | | |
| 8 | Repasser en « Hérité » | La surcharge disparaît | | |
| 9 | Supprimer le fichier d'un effet choisi | Avertissement « absent ou invalide », valeur conservée « (introuvable) », aucun effet joué | | |
| 10 | Voir la section « 🎤 Groupes animateur » | Un bloc de trois sélecteurs (effet + vitesse) par « Groupe animateur k » ; pas de bloc ni de lien de création en manche s'il n'y a aucun groupe animateur | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 7 — Volet « ampoules réelles » : chenillard sur 4, 6 et 8 groupes, vitesse ×0,5 / ×2 visible (contrat §8.2)

**Validation** : HUMAIN — **Issues** : #223

**Objectif** : conformité à #223 et mesure de la régularité sur le pont réel. À consigner dans le rapport de QUALIF.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | 4 groupes, chrono 30 s, **Chenillard**, vitesse ×1 | Un seul groupe à la couleur d'urgence, le fond au thème, **un groupe par seconde**, départ au groupe 1. **Relever** la durée réelle d'un pas | | |
| 2 | 6 groupes, même question | Idem sur 6 groupes. **Relever** la durée d'un pas et la régularité à l'œil | | |
| 3 | 8 groupes, même question | Idem sur 8 groupes (départ g1, tour complet). **Relever** | | |
| 4 | 5 groupes ou plus, **Chenillard avec traînée** | Tête + un groupe de part et d'autre à 50 % (avec rebouclage aux extrémités) | | |
| 5 | Vitesse **×2** | Le point va **deux fois plus vite** (≈ 500 ms par groupe) ; **relever** la durée réelle | | |
| 6 | Vitesse **×0,5** | Le point va **deux fois moins vite** (≈ 2 s par groupe) ; **relever** | | |
| 7 | Changer la vitesse **pendant** la question | Prise en compte au tick suivant, sans que le chenillard reparte du groupe 1 | | |
| 8 | Passer sous 10 s puis 5 s restantes | Tête orange puis rouge (couleur d'urgence vivante) | | |
| 9 | Groupes de 3 ampoules et plus vs groupes de 2 | Noter si les ampoules d'un groupe de 2 sont visiblement décalées (seuil `SubGroupMinLights`) | | |
| 10 | Fin du chrono, entracte, nouvelle question | Retour aux scènes ordinaires sans résidu de l'effet ; l'effet repart du groupe 1 | | |

Relevés : pas mesuré à 4 groupes = ___ s ; 6 = ___ s ; 8 = ___ s ; ×2 = ___ s ; ×0,5 = ___ s ; régularité : ___ ; décalage groupes de 2 : ___.

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 8 — Pulsation au nouveau rendu, dégradé et couleur de l'équipe

**Validation** : HUMAIN — **Issues** : #223, #211

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Chronomètre sur « Pulsation » (défaut), question chronométrée | Respiration de toute la salle : phase haute au **thème à 100 %**, phase basse **blanche à 50 %** (au-delà de 10 s), 500 ms chacune | | |
| 2 | Sous 10 s, puis 5 s | Phase basse **orange à 50 %** puis **rouge à 50 %**, cycle toujours 500 / 500 ms (plus de battement 250 / 750) | | |
| 3 | Sans aucun groupe configuré | La pulsation joue sur toute la salle (applicable sans groupe) | | |
| 4 | Phases calmes sur « Dégradé » (2 groupes ou plus) | Scène fixe : une couleur par groupe (cycle de 4 couleurs) ; une salle plus petite en montre le début | | |
| 5 | « Équipe qui marque » sur « Couleur de l'équipe », attribuer des points aux Rouges | Toute la salle prend la couleur des Rouges le temps de l'attribution puis revient à la scène | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 9 — Précédence : ON / OFF / Flash priment sur l'effet

**Validation** : HUMAIN — **Issues** : #223

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Chenillard en cours, popover Ambiance : OFF | Salle éteinte, effet interrompu | | |
| 2 | AUTO | L'effet reprend (état vivant) | | |
| 3 | ON puis Flash | Salle blanche / clignotante, pas de chenillard | | |
| 4 | Ampoule d'équipe pendant l'effet | Garde la couleur de son équipe, jamais modifiée par l'effet | | |
| 5 | Ampoule générale **hors groupe** pendant un chenillard | Suit `g0` (le fond), jamais la tête | | |
| 6 | Interrupteur général désactivé en plein effet, puis arrêt du serveur | Tout s'éteint, aucune ampoule ne se rallume toute seule | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 10 — Groupes animateur : un effet et une vitesse par moment et par groupe

**Validation** : HUMAIN — **Issues** : #223 (besoin issu de #211)

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Bloc « Groupe animateur 1 », liste du chronomètre | « Blanc fixe » par défaut ; Pulsation et Couleur de l'équipe proposées, les effets de répartition grisés | | |
| 2 | Groupe animateur 1 : « Quand une équipe marque » = « Couleur de l'équipe » ; groupe animateur 2 : « Blanc fixe » ; enregistrer | — | | |
| 3 | Attribuer des points à « Rouges » | **Seule** l'ampoule du groupe animateur 1 prend la couleur des Rouges, puis redevient blanche ; l'autre reste blanche fixe ; la salle suit son propre effet | | |
| 4 | Groupe animateur 2 : « Pulsation » pendant le chrono, **×2** ; groupe animateur 1 : « Pulsation » ×0,5 | Les deux pulsent à des vitesses différentes, indépendamment de la salle | | |
| 5 | Réordonner les deux groupes animateur | Les réglages (effets et vitesses) **suivent le groupe** (identifiant stable) | | |
| 6 | Supprimer le groupe animateur 2, enregistrer, en recréer un | Les réglages de l'ancien groupe ont disparu (le nouveau est en « Blanc fixe », ×1) ; ceux de l'autre restent | | |
| 7 | OFF / ON / Flash dans le popover | Aucun groupe animateur n'est modifié | | |
| 8 | Même réglage par manche (mode surcharge) | Valeur de manche prioritaire par groupe, par moment et par réglage (effet / vitesse), « Hérité » sinon | | |
| 9 | Config / rounds.json issus de la QUALIF 11.7.0.x antérieure (ancienne forme de `host_effects`, groupes avec `name` / `near_teams`) | Le serveur démarre, les anciennes formes sont ignorées sans erreur ; les groupes animateur existants sont en « Blanc fixe » | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 11 — Sauvegarde et restauration des fichiers d'effets

**Validation** : AUTO — **Issues** : #223

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `cd server-go && go test ./internal/server -run 'TW223.*Backup|TW223.*Restore' -count=1` | Verts : `files/effects/` (hors `natifs/`) dans la catégorie `ambiance`, restauration vers le même dossier | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Criteres de Validation

- [ ] Écrire un effet avec une variable par copie, le casser (message fichier + ligne), le restaurer : sans redémarrage
- [ ] Chenillard conforme à #223 à 4, 6 et 8 groupes ; relevés §8.2 consignés
- [ ] Vitesse ×0,5 / ×2 visible, « plafonné » sous 250 ms, changement pris au tick suivant
- [ ] Précédence ON / OFF / Flash respectée, aucune écriture fantôme à l'arrêt
- [ ] Groupes animateur : blanc fixe par défaut, effet et vitesse par moment **et par groupe**, insensibles à la salle et entre eux
- [ ] Pulsation au nouveau rendu (§5.5), dégradé et couleur de l'équipe

## Notes QA

[Espace pour observations — noter les relevés du scénario 7.]
