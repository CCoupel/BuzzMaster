# Procédure de Test — Surcharge d'ambiance par manche (#290)

**Version** : 11.6.0 (LOT-4)
**Date** : 2026-11-09
**Testeur** : QA / utilisateur (poste Windows : enceinte + pont Hue réel)

> Contrats : `contracts/ambiance-settings.md` (global ← manche ← geste direct, D1′, D7, D8), `contracts/lighting.md` §12, `contracts/game-state.md` (`AMBIANCE_EFFECTIVE`).
> Maquettes : `docs/mockup/v11.6.0/ui/ambiance_settings_panel__round_override.html`, `admin_questions_list__round_ambiance_override.html`, `admin_navbar__ambiance_popover_round_source.html`, `ambiance_settings_panel__global_per_type_tabs.html` §02.
> Équivalents automatisés : `internal/ambiancecfg/resolve_290_test.go`, `cmd/server/round_ambiance_290*_test.go`, `web/src/components/{AmbianceSettingsPanel,RoundAmbianceDialog,RoundGroup,Navbar}.*290.test.jsx`, `web/src/pages/{QuestionsPage.rounds,AmbiancePage.lightmode}.*290.test.jsx`.

## Prérequis

- [ ] Environnement : QUALIF (binaire Windows), enceinte reliée au serveur, `sound.enabled = true`, **pont Hue associé** avec au moins une ampoule en rôle « général »
- [ ] Un quiz de test avec **trois manches** : « Cinéma » (aucune surcharge), « Blind-test » (QCM + SPEEDY, chronomètres de réponse), « Finale » ; plus une question hors manche
- [ ] Un son personnalisé reconnaissable sur le bruitage « Départ » et sur « Indice révélé » (facultatif)
- [ ] Un moyen de lancer des questions (page Jeu ou animateur)

## Scénarios

### Scénario 1 — Fenêtre « Ambiance de la manche » : tout hérité par défaut

**Validation** : HUMAIN — **Issues** : #290

**Objectif** : une manche hérite de tout ; la fenêtre affiche les valeurs globales.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Éditeur de questions : repérer le bouton 🎛 dans l'en-tête de « Cinéma », « Blind-test », « Finale » ; chercher-le sur « Sans manche » | 🎛 sur chaque manche ; **absent** sur « Sans manche » ; aucun badge | | |
| 2 | Cliquer 🎛 sur « Blind-test » | Fenêtre « Ambiance de la manche « Blind-test » », « Hérite tout du réglage global » ; Son, Décompte, Bruitages (5 onglets), Lumière en « Hérité » avec la valeur globale en italique | | |
| 3 | Comparer à la maquette | Même disposition ; pas de colonne fichier / Remplacer / Tester | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 2 — Surcharger une manche et l'enregistrer

**Validation** : HUMAIN — **Issues** : #290

**Objectif** : la surcharge se compose champ par champ et se résume par un badge.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Dans « Blind-test » : Décompte → « Inactif » ; onglet QCM → « Indice révélé » → « Inactif » ; Lumière → « ON » | Le compteur passe à « 3 réglages surchargés » ; l'onglet QCM porte un point « • » | | |
| 2 | Enregistrer | La fenêtre se ferme ; badge « 🎛 Ambiance : 3 réglages » sur la manche ; l'info-bulle liste les réglages | | |
| 3 | Rouvrir 🎛 puis « Tout remettre à « Hérité » » puis Annuler | Rien n'est changé (le badge reste à 3) | | |
| 4 | Recharger la page | La surcharge est conservée (elle vit dans la manche) | | |
| 5 | Renommer « Blind-test », la déplacer avec ▲ ▼ | La surcharge suit la manche | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 3 — Entrée dans la manche : son, décompte et lumière réels

**Validation** : HUMAIN — **Issues** : #290

**Objectif** : les réglages de la manche s'appliquent réellement, et seulement pendant la manche.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer une question hors manche | Lumière en AUTO ; décompte sonore actif au chronomètre ; « Indice révélé » audible sur une QCM | | |
| 2 | Lancer une question de « Blind-test » | Lumière **ON** (ampoule générale allumée) ; **pas de décompte sonore** ; un indice QCM est **silencieux** ; les autres bruitages (Départ, Points gagnés…) restent audibles | | |
| 3 | Popover « Ambiance » de l'en-tête | « Manche « Blind-test » : 3 réglages imposés » ; étiquettes « manche » sur l'éclairage et le décompte ; badge d'en-tête « Mode ON engagé » | | |
| 4 | Passer à « Cinéma » (ou une question hors manche) | Retour immédiat au réglage global (AUTO, décompte, indices audibles) ; plus de ligne « réglages imposés » | | |
| 5 | NEW_GAME | Retour au global | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 4 — Manche « son coupé » : bruitages, média de question, décompte, entracte

**Validation** : HUMAIN — **Issues** : #290

**Objectif** : « Son : Coupé » imposé par une manche rend tout muet, y compris les médias du LOT-2.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Surcharger « Finale » : Son → « Coupé » ; y mettre une question avec son attaché (et une entracte avec son) | — | | |
| 2 | Pendant une question de la manche précédente, un média joue ; passer à « Finale » | Le média en cours est coupé à l'entrée (effet « son coupé » déclenché par la manche) | | |
| 3 | Jouer les questions de « Finale » | Aucun bruitage, aucun son de question ou de carte, aucun décompte, aucun son d'entracte | | |
| 4 | « Tester sur l'enceinte » depuis Ambiance › Son | Joue quand même (geste explicite) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 5 — Geste direct : précédence (D1′)

**Validation** : HUMAIN — **Issues** : #290

**Objectif** : un geste de régie l'emporte sur la manche, et ne réécrit le réglage global que si la manche ne le surcharge pas.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Hors manche : popover → Son « Coupé » | Son coupé ; **persisté** (Ambiance › Son : réglage « Coupé » aussi, comportement v11.4.0) ; remettre « ON » | | |
| 2 | Dans « Blind-test » (lumière ON imposée) : popover → éclairage « OFF » | Lumière OFF jusqu'au prochain changement de manche ; étiquette « direct » ; **la page Ambiance › Lumière ne change pas** (global inchangé) | | |
| 3 | Dans « Blind-test » : refaire un geste sur un champ **surchargé** par la manche (éclairage, imposé à ON) | Vaut pour la manche en cours seulement ; le réglage global n'est pas modifié (vérifier la page Ambiance › Lumière) | | |
| 4 | Dans « Blind-test » : geste sur un champ **non** surchargé (ex. Son « Coupé ») | Le réglage global est bien réécrit (persisté) | | |
| 5 | Passer à une autre manche | Les gestes directs sont oubliés ; retour global ← manche | | |
| 6 | Question suivante de la **même** manche | Le geste direct est conservé | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 6 — Mode d'éclairage par défaut (Ambiance › Lumière)

**Validation** : HUMAIN — **Issues** : #290

**Objectif** : le mode d'éclairage devient un réglage persisté (D8).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ambiance › Lumière, bloc « Réglages d'ambiance — Lumière » | ON / AUTO / OFF, AUTO sélectionné (config sans `default_mode`) | | |
| 2 | Choisir « ON », redémarrer le serveur | Au démarrage l'éclairage est ON (plus AUTO implicite) | | |
| 3 | Remettre AUTO | Persisté | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 7 — Non-régression (aucune manche surchargée)

**Validation** : AUTO — **Issues** : #290

**Objectif** : sans surcharge, v11.5.0 à l'identique.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `rounds.json` v11.2 sans `AMBIANCE`, `config.json` sans `default_mode` | Le serveur démarre ; manches chargées ; AUTO au démarrage | | |
| 2 | Popover d'une partie sans surcharge | Aucune étiquette « manche », aucune ligne « réglages imposés » | | |
| 3 | Un geste « Son coupé » hors manche | Persisté comme avant (v11.4.0) | | |
| 4 | Journal serveur | Aucune erreur liée à l'ambiance | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Criteres de Validation

- [ ] Scénarios 1 à 6 (écoute, lumière réelle et lecture de l'écran) validés par l'utilisateur
- [ ] Scénario 7 (AUTO) couvert par les tests automatisés listés en tête
- [ ] Aucune manche ne modifie le global ; un geste direct ne le modifie que hors surcharge du champ

## Notes QA

[Espace pour observations]
