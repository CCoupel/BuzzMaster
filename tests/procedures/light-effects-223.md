# Procédure de Test — Effets lumière en fichiers, séquenceur et chenillard (#223, milestone v11.7.0, LOT-2)

**Version** : v11.7.0.x (QUALIF)
**Date** : 2026-11-11
**Testeur** : QA (scénarios AUTO) / Utilisateur (scénarios HUMAIN)
**Contrat** : `contracts/light-effects.md` §3 à §7, §9 ; `contracts/ambiance-settings.md` §10
**Maquette** : `docs/mockup/v11.7.0/ui/ambiance_settings_panel__light_effects.html`
**Plan** : `.claude/workflow/plan-11.7.0.md` (LOT-2, tâches 8 à 17)

## Prerequis

- [ ] Environnement : QUALIF (binaire Windows, pont Hue réel pour le volet « ampoules réelles »)
- [ ] Données : au moins 8 ampoules générales, dont 4 à 8 rangées en groupes de salle (cf. `light-groups-225.md`),
      un groupe animateur (1 ampoule), équipes « Rouges » et « Bleus », un quiz avec une question chronométrée de 30 s
- [ ] Accès : page Ambiance › Lumière › Effets ; accès au dossier `data/files/effects/` de la QUALIF (explorateur de fichiers)
- [ ] Un éditeur de texte ordinaire (Bloc-notes)

## Scenarios

### Scenario 1 — Tests automatisés du lot

**Validation** : AUTO — **Issues** : #223

**Objectif** : trames, erreurs de fichiers, rechargement, lecteur, réglages et API passent.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `cd server-go && go test -race ./internal/effects ./internal/ambiancecfg ./internal/ambiance -count=1` | Tous verts | | |
| 2 | `cd server-go && go test -race ./internal/server -run 'TW223' -count=1` | Tous verts (réglages, `/api/lighting/effects`, sauvegarde) | | |
| 3 | `cd server-go/web && npx vitest run src/components/AmbianceSettingsPanel.223.test.jsx` | Tous verts | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 2 — Créer un effet en copiant un natif

**Validation** : HUMAIN — **Issues** : #223

**Objectif** : un effet se crée par copie d'un fichier, sans redémarrer ni recompiler (critère d'acceptation).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir `data/files/effects/natifs/` | Copies de référence des natifs (`chenillard.yaml`, `pulsation.yaml`, `couleur-equipe.yaml`…) avec l'en-tête « ne pas modifier ici » | | |
| 2 | Copier `chenillard.yaml` dans `data/files/effects/`, le renommer `mon-chenillard.yaml`, changer `id: mon-chenillard` et `name: Mon chenillard`, enregistrer | — | | |
| 3 | Dans la page Ambiance › Effets, attendre au plus 3 s (ou cliquer « Recharger les fichiers » si disponible) | « Mon chenillard (personnalisé) » apparaît dans la liste « Pendant le chronomètre » sans redémarrer | | |
| 4 | Dans le fichier, changer `step_ms: 1000` en `step_ms: 500`, enregistrer | La vitesse change à la prochaine lecture de l'effet, sans redémarrage | | |
| 5 | Choisir « Mon chenillard » pour le chronomètre, lancer une question chronométrée | Le point lumineux fait le tour de la salle, plus vite qu'avant | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 3 — Casser un fichier : message fichier + ligne, le reste continue

**Validation** : HUMAIN — **Issues** : #223

**Objectif** : un fichier invalide est écarté avec un message lisible ; serveur et autres effets continuent (critère d'acceptation).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Dans `mon-chenillard.yaml`, remplacer `tete` par `teet` dans `cells:` (apparence inexistante), enregistrer | — | | |
| 2 | Attendre au plus 3 s, regarder la page Effets | Bandeau d'erreur : « mon-chenillard.yaml, ligne N : l'apparence « teet » n'est pas définie dans looks » (nom du fichier, bon numéro de ligne, texte en français) | | |
| 3 | Vérifier le chenillard natif et la pulsation | Toujours disponibles et fonctionnels ; le serveur n'a pas planté | | |
| 4 | Lire le journal du serveur | UNE seule ligne pour ce fichier (pas une par cycle de 2 s) | | |
| 5 | Introduire une clé inconnue (`bidule: 3`) puis un `id` avec majuscule | Messages distincts, chacun avec fichier et ligne | | |
| 6 | Pendant une partie, casser le fichier de l'effet en cours de lecture | La dernière version valide continue de jouer (pas de coupure) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 4 — Surcharge d'un natif et restauration

**Validation** : HUMAIN — **Issues** : #223

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Copier `chenillard.yaml` en gardant **`id: chenillard`**, modifier une couleur, enregistrer | Le catalogue indique « natif modifié » (surcharge) ; le chenillard joue la version modifiée | | |
| 2 | Supprimer (ou renommer en `.off`) le fichier copié | Retour à l'effet d'origine en moins de 3 s, sans redémarrage | | |
| 3 | Modifier la copie de référence dans `natifs/` | Aucun effet sur le rendu (les natifs viennent du binaire) ; réécrite au redémarrage | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 5 — Effet par moment, global et par manche

**Validation** : HUMAIN — **Issues** : #223

**Objectif** : trois sélecteurs (maquette §01-§03), valeurs par défaut identiques à v11.6.0.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Page Ambiance › Effets (réglages globaux) | Chronomètre = « Pulsation (actuel) », Phases calmes = « Aucun effet », Équipe qui marque = « Aucun effet » | | |
| 2 | Ouvrir la liste « Pendant le chronomètre » avec seulement 2 groupes configurés | « Chenillard » visible mais grisé, avec « demande 3 groupes, 2 configurés » | | |
| 3 | Choisir Chenillard (4 groupes), enregistrer, F5 | Choix conservé | | |
| 4 | Ouvrir « Ambiance de la manche » d'une manche : sélecteurs en « Hérité (Chenillard) » | La valeur globale est affichée entre parenthèses | | |
| 5 | Surcharger le chronomètre de la manche par « Pulsation » | Badge « manche » ; la question de cette manche pulse, les autres jouent le chenillard | | |
| 6 | Repasser en « Hérité » | La surcharge disparaît | | |
| 7 | Supprimer le fichier d'un effet choisi | Avertissement « absent ou invalide », valeur conservée « (introuvable) », aucun effet joué | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 6 — Volet « ampoules réelles » : chenillard à 4, 6 et 8 groupes (relevés contrat §8.2)

**Validation** : HUMAIN — **Issues** : #223

**Objectif** : conformité à #223 et mesure de la régularité sur le pont réel. À consigner dans le rapport de QUALIF.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | 4 groupes, chrono 30 s avec Chenillard : observer | Sans traînée sous 5 groupes ; un seul groupe allumé à la couleur d'urgence, un groupe par seconde. **Relever** la durée réelle d'un pas (chronomètre) | | |
| 2 | 6 groupes, même question | Tête + traînée à 50 % de chaque côté, fond à la couleur du thème. **Relever** la durée d'un pas et la régularité à l'œil (saccade ?) | | |
| 3 | 8 groupes, même question | Idem ; **relever** durée d'un pas, régularité | | |
| 4 | Passer sous 10 s puis 5 s restantes | Tête orange puis rouge | | |
| 5 | Groupes de 3 ampoules et plus vs groupes de 2 | Noter si les ampoules d'un groupe de 2 sont visiblement décalées (seuil `SubGroupMinLights`) | | |
| 6 | Fin du chrono, entracte, nouvelle question | Retour aux scènes ordinaires sans résidu de l'effet | | |

Relevés : pas mesuré à 4 groupes = ___ s ; 6 = ___ s ; 8 = ___ s ; régularité : ___ ; décalage groupes de 2 : ___.

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 7 — Précédence : ON / OFF / Flash priment sur l'effet

**Validation** : HUMAIN — **Issues** : #223

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Chenillard en cours, popover Ambiance : OFF | Salle éteinte, effet interrompu | | |
| 2 | AUTO | L'effet reprend (état vivant) | | |
| 3 | ON puis Flash | Salle blanche / clignotante, pas de chenillard | | |
| 4 | Ampoule d'équipe pendant l'effet | Garde la couleur de son équipe, jamais modifiée par l'effet | | |
| 5 | Interrupteur général désactivé en plein effet, puis arrêt du serveur | Tout s'éteint, aucune ampoule ne se rallume toute seule (pas d'écriture fantôme) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 8 — Groupe animateur : effet par moment et réaction au score

**Validation** : HUMAIN — **Issues** : #223 (besoin issu de #211)

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Page Effets, bloc « Groupe animateur » : ouvrir la liste du chronomètre | « Blanc fixe » par défaut ; le chenillard est grisé (« plusieurs groupes »), la pulsation et « Couleur de l'équipe » sont proposées | | |
| 2 | Choisir « Couleur de l'équipe » pour « Quand une équipe marque », enregistrer | — | | |
| 3 | Attribuer des points à « Rouges » | L'ampoule de l'animateur prend la couleur des Rouges le temps de l'attribution, puis redevient blanche ; la salle suit sa propre scène SCORE | | |
| 4 | Choisir « Pulsation » pour son chronomètre | L'animateur pulse pendant la question, la salle suit son propre effet | | |
| 5 | OFF / Flash dans le popover | L'animateur n'est pas modifié | | |
| 6 | Essai d'un effet depuis le catalogue (LOT-3, si livré) | Ne joue jamais sur l'animateur | | |
| 7 | Même réglage par manche (mode surcharge, bloc « Groupe animateur ») | Valeur de manche prioritaire, « Hérité » sinon ; pas de bloc si aucun groupe animateur (lien) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 9 — Non-régression : réglages par défaut = v11.6.0

**Validation** : HUMAIN — **Issues** : #223

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Réglages d'effets à leur valeur par défaut, sans groupe, partie complète | Pulsation de fin de chrono identique à v11.6.0 (cycle 1 s, orange puis rouge) ; scènes, SCORE, Flash, ON/OFF, extinction inchangés | | |
| 2 | Avec des groupes mais effets par défaut | Idem : la pulsation joue sur toute la salle | | |

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

- [ ] Créer un effet par copie, le casser (message fichier + ligne), le restaurer : sans redémarrage
- [ ] Chenillard conforme à #223 à 4, 6 et 8 groupes ; relevés §8.2 consignés
- [ ] Précédence ON / OFF / Flash respectée, aucune écriture fantôme à l'arrêt
- [ ] Groupe animateur : blanc fixe par défaut, effet par moment, insensible à la salle
- [ ] Aucune régression avec les réglages par défaut

## Notes QA

[Espace pour observations — noter les relevés du scénario 6.]
