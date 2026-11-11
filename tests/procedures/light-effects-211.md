# Procédure de Test — Effets natifs sur ampoules réelles : chenillard, vitesse, « Tester », zone Public, dégradé (#211, #223, milestone v11.7.0, LOT-2)

**Version** : v11.7.0.x (QUALIF)
**Date** : 2026-11-11 (révision 4 : effets natifs joués par **zone**, vitesse réglable, bouton « Tester » ×2, zone Public)
**Testeur** : QA (scénario AUTO) / Utilisateur (scénarios HUMAIN)
**Contrat** : `contracts/light-effects.md` §4.5, §5 (natifs), §6 (vitesses), §7.4 (essai), §8.2 (relevés sur matériel), §9
**Maquette** : `docs/mockup/v11.7.0/ui/ambiance_settings_panel__light_effects_zones.html`
**Plan** : `.claude/workflow/plan-11.7.0.md` (LOT-2, #223 et #211 — l'effet `degrade` est la fin de #211)
**Procédure voisine** : `light-effects-223.md` (format de fichier, réglages, précédence, sauvegarde)

## Prerequis

- [ ] Environnement : QUALIF (binaire Windows, pont Hue réel)
- [ ] Données : au moins 8 ampoules générales, rangées en **8 groupes dans la zone Ambiance** (puis réductibles à 6 et 4) ;
      **un groupe animateur** (1 ampoule) ; **un groupe public** (1 ampoule réglée à la main avant chaque scénario) ;
      équipes « Rouges » et « Bleus » ; un quiz avec une question chronométrée de 30 s
- [ ] Accès : page Ambiance › Lumière › Effets
- [ ] Un chronomètre ou la vidéo du téléphone pour les relevés

## Scenarios

### Scenario 1 — Tests automatisés des effets natifs

**Validation** : AUTO — **Issues** : #223, #211

**Objectif** : les natifs rendent ce que le chronogramme du contrat prévoit et le lecteur les joue à la bonne vitesse.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `cd server-go && go test -race ./internal/effects -run 'Native|Chronogram|Speed|Render' -count=1` | Verts : chenillard 4 et 8 groupes, traînée, pulsation (§5.5), couleur de l'équipe, dégradé, plancher 250 ms | | |
| 2 | `cd server-go && go test -race ./internal/ambiance -run 'Reader|Trial223' -count=1` | Verts : un effet par zone à sa vitesse, essai d'effet par zone, retour à l'état vivant | | |
| 3 | `cd server-go/web && npx vitest run src/components/AmbianceSettingsPanel.zones225.test.jsx src/components/AmbianceSettingsPanel.223.test.jsx` | Verts : bouton « Tester », vitesse, « plafonné » | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 2 — Chenillard sur 4, 6 et 8 groupes (zone Ambiance), relevés §8.2

**Validation** : HUMAIN — **Issues** : #223

**Objectif** : conformité du chenillard et mesure de la régularité sur le pont réel. À consigner dans le rapport de QUALIF.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | 4 groupes dans la zone Ambiance, chrono 30 s, zone Ambiance › Chronomètre = **Chenillard**, vitesse ×1 | Un seul groupe à la couleur d'urgence, le fond au thème, **un groupe par seconde**, départ au groupe 1. **Relever** la durée réelle d'un pas | | |
| 2 | 6 groupes, même question | Idem sur 6 groupes. **Relever** la durée d'un pas et la régularité à l'œil | | |
| 3 | 8 groupes, même question | Idem sur 8 groupes (départ g1, tour complet). **Relever** | | |
| 4 | 5 groupes ou plus, **Chenillard avec traînée** | Tête + un groupe de part et d'autre à 50 % (avec rebouclage aux extrémités) | | |
| 5 | Passer sous 10 s puis 5 s restantes | Tête orange puis rouge (couleur d'urgence vivante) | | |
| 6 | Groupes de 3 ampoules et plus vs groupes de 2 | Noter si les ampoules d'un groupe de 2 sont visiblement décalées (seuil `SubGroupMinLights`) | | |
| 7 | Un groupe vide au milieu (groupe 3 sans ampoule) | Le groupe vide garde son numéro mais n'est jamais allumé ; le chenillard passe de g2 à g4 | | |
| 8 | Fin du chrono, entracte, nouvelle question | Retour aux scènes ordinaires sans résidu de l'effet ; l'effet repart du groupe 1 | | |

Relevés : pas mesuré à 4 groupes = ___ s ; 6 = ___ s ; 8 = ___ s ; régularité : ___ ; décalage groupes de 2 : ___.

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 3 — Vitesse ×0,5 / ×2 visible, changement en cours de question

**Validation** : HUMAIN — **Issues** : #223

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Chenillard sur 6 groupes, vitesse **×2** | Le point va **deux fois plus vite** (≈ 500 ms par groupe) ; **relever** la durée réelle | | |
| 2 | Vitesse **×0,5** | Le point va **deux fois moins vite** (≈ 2 s par groupe) ; **relever** | | |
| 3 | Changer la vitesse **pendant** la question | Prise en compte au tick suivant, sans que le chenillard reparte du groupe 1 | | |
| 4 | Un effet à tick court (400 ms) en ×2 | Étiquette « plafonné » : jamais sous 250 ms par tick ; relever la vitesse réelle | | |

Relevés : ×2 = ___ s par groupe ; ×0,5 = ___ s par groupe ; plafond : ___.

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 4 — « Tester » ×2 : le bouton de l'onglet Effets et « Essayer » du catalogue

**Validation** : HUMAIN — **Issues** : #223

**Objectif** : deux façons d'essayer un effet (contrat §7.4) : « ▶ Tester » (zone et vitesse **réglées**) et « Essayer » du catalogue (zone Ambiance, vitesse ×1).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Zone Ambiance, chronomètre = Chenillard ×2, « ▶ Tester » | L'effet joue ≈ 10 s **à ×2** sur les ampoules de la zone Ambiance seulement ; « ■ Arrêter » | | |
| 2 | Même effet, catalogue : « Essayer » | L'effet joue ≈ 10 s à **×1** sur la zone Ambiance (quelle que soit la vitesse réglée) | | |
| 3 | « ▶ Tester » sur un effet de la zone Animateur | Seule la zone Animateur joue ; la salle et le public ne bougent pas ; retour au blanc fixe à la fin | | |
| 4 | Pendant un test, lancer l'autre bouton | Un **seul** essai à la fois : le second remplace le premier | | |
| 5 | Couleur d'urgence pendant un essai de 10 s | Orange puis rouge (chronomètre simulé qui descend de 15 s) ; thème `#4F46E5`, couleur d'équipe `#E53935` pour « Couleur de l'équipe » | | |
| 6 | Essayer pendant une question | Refusé (« Une question est en cours ») | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 5 — Zone Public : l'effet n'est joué que s'il est choisi, état tenu à la fin

**Validation** : HUMAIN — **Issues** : #223

**Objectif** : la zone Public ne reçoit rien sans effet, joue son effet quand il est choisi et tient l'état final (POST), seule zone où il est visible (contrat §2.5, §4.5).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Zone Public sans effet, lampe réglée à la main : dérouler une question | La lampe ne bouge jamais | | |
| 2 | Zone Public › Équipe qui marque = « Couleur de l'équipe » ; attribuer des points aux Rouges | La lampe du public prend la couleur des Rouges ; à la fin du moment **l'état posé par l'effet est tenu** | | |
| 3 | Remettre « Aucun effet » | Plus rien n'est envoyé : la lampe reste dans son dernier état, sans retour à un blanc fixe | | |
| 4 | Zone Public › Chronomètre = Pulsation ×1 : question chronométrée | La lampe pulse sur le thème, indépendamment de la salle ; ON / OFF / Flash du popover ne la touchent pas | | |
| 5 | « ▶ Tester » en zone Public (lampe réglée à la main avant) | À la fin du test, la lampe retrouve **son réglage d'avant** (lu sur le pont puis réécrit) ; éteinte si le pont n'était pas lisible | | |
| 6 | Interrupteur général désactivé, puis arrêt du serveur | La lampe du public s'éteint | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 6 — Pulsation au nouveau rendu, couleur de l'équipe et dégradé (#211)

**Validation** : HUMAIN — **Issues** : #223, #211

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Chronomètre de la zone Ambiance sur « Pulsation » (défaut), question chronométrée | Respiration de toute la salle : phase haute au **thème à 100 %**, phase basse **blanche à 50 %** (au-delà de 10 s), 500 ms chacune | | |
| 2 | Sous 10 s, puis 5 s | Phase basse **orange à 50 %** puis **rouge à 50 %**, cycle toujours 500 / 500 ms | | |
| 3 | Sans aucun groupe configuré | La pulsation joue sur toute la salle (applicable sans groupe, zone Ambiance) | | |
| 4 | Phases calmes sur « Dégradé » (2 groupes ou plus dans la zone) | Scène fixe : une couleur par groupe (cycle de 4 couleurs) ; une zone plus petite en montre le début ; **tick de 60 s**, pas de clignotement | | |
| 5 | « Dégradé » sur la zone Animateur (2 groupes) puis sur 1 seul groupe | Appliqué avec 2 groupes ; grisé « demande 2 groupes, 1 dans cette zone » avec 1 seul | | |
| 6 | « Équipe qui marque » sur « Couleur de l'équipe », attribuer des points aux Rouges | Toute la zone prend la couleur des Rouges le temps de l'attribution puis revient à la scène | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Criteres de Validation

- [ ] Chenillard conforme à #223 à 4, 6 et 8 groupes ; relevés §8.2 consignés
- [ ] Vitesse ×0,5 / ×2 visible, « plafonné » sous 250 ms, changement pris au tick suivant
- [ ] « Tester » (zone et vitesse réglées) et « Essayer » (×1, ambiance) : un seul essai à la fois, refusé pendant une question
- [ ] Zone Public : jamais écrite sans effet ; effet joué quand il est choisi ; état rétabli après un essai
- [ ] Dégradé (#211), pulsation au nouveau rendu (§5.5) et couleur de l'équipe conformes

## Notes QA

[Espace pour observations — noter les relevés des scénarios 2 et 3.]
