# Procédure de Test — Administration des sons, `/admin/ambiance` onglet Son (#230, milestone v11.0)

**Version** : v11.0.0.x (QUALIF)
**Date** : 2026-09-21
**Testeur** : Utilisateur — cette procédure exige des oreilles et du matériel réel
**Contrat** : `contracts/http-endpoints.md` §Sound (SHA `1ce78d73`), `contracts/sound.md` §6.3
**Maquette** : `docs/mockups/sound-config-230.html` (révision 4)
**Plan** : `_work/reports/plan-dev-230-20260921-144000.md`, delta modale : `_work/reports/plan-delta-230-lotC-20260921-160000.md`

## Important — pourquoi un agent ne peut pas exécuter cette procédure

Aucun agent (`qa` compris) n'a d'oreilles. C'est précisément la raison d'être du verdict
manuel de la modale de test (section 03 de la maquette) : **le serveur ne peut jamais
savoir si un son est réellement sorti** — `result: "played"` signifie « confié au
moteur », jamais « entendu ». Chaque scénario ci-dessous doit être exécuté par
l'utilisateur, sur du matériel audio réel.

## Prérequis

- [ ] Binaire buildé depuis `milestone/v11.0` (ou merge ultérieur), avec `#227`/`#228`/`#229`/`#230`
- [ ] `config.json` : `sound.enabled = true`
- [ ] Une enceinte ou un haut-parleur fonctionnel relié au serveur
- [ ] Accès à `/admin/ambiance`, onglet **Son**
- [ ] Au moins un fichier `.wav` de test **conforme** (44 100 Hz, 16 bits, stéréo, < 5 s) et
      plusieurs fichiers **non conformes** préparés à l'avance : un MP3 renommé en `.wav`, un
      WAV en 22 050 Hz, un WAV mono, un WAV de ~10 s, un WAV de ~3 s (accepté avec avertissement)

## Scénarios

### Scénario 1 — Remplacer un son, écouter, tester, constater « personnalisé »

**Objectif** : le parcours complet de remplacement, sur une cue par défaut.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | Onglet Son, choisir une ligne marquée « Défaut » (ex. « Départ »), cliquer « ⬆ Remplacer », choisir un fichier `.wav` conforme | La ligne passe à « Personnalisé », la durée se met à jour, le bouton « ↺ Restaurer » apparaît | | |
| 2 | Cliquer « ▶ Tester » sur cette ligne | La modale s'ouvre — **rien ne se joue** encore | | |
| 3 | Cliquer « ▶ Jouer dans ce navigateur » | Le fichier remplacé s'entend depuis les haut-parleurs de VOTRE appareil | | |
| 4 | Poser le verdict « Ok » ou « Ko » sur la section 1 | Le sélecteur reflète le choix | | |
| 5 | Cliquer « 🔊 Jouer sur l'enceinte » | Le son s'entend depuis l'enceinte reliée au SERVEUR (pas votre appareil, sauf si c'est la même machine) ; un message ("Son envoyé à l'enceinte") s'affiche dans la section 2 | | |
| 6 | Poser le verdict de la section 2 | Le sélecteur reflète le choix, indépendamment de celui de la section 1 | | |
| 7 | Fermer la modale | Les deux pastilles de la colonne « Test » du tableau reflètent les verdicts posés | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 2 — Restaurer ce son, constater le retour à « défaut » et l'effacement des verdicts

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | Sur la ligne personnalisée du Scénario 1, cliquer « ↺ Restaurer » | La ligne repasse à « Défaut », le bouton « ↺ Restaurer » disparaît, la durée redevient celle d'origine | | |
| 2 | Observer la colonne « Test » | Les deux pastilles sont revenues à l'état « pas testé » (grises) — les verdicts portaient sur un fichier qui n'existe plus | | |
| 3 | Écouter à nouveau (« ▶ Tester » → « ▶ Jouer dans ce navigateur ») | Le son d'origine (généré) s'entend, distinct du fichier remplacé au Scénario 1 | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 3 — Fichiers non conformes : un message clair à chaque fois

**Objectif** : chaque refus doit porter une raison lisible, jamais un code nu (contrat,
section "Sécurité"/validation).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | Remplacer un son par un MP3 renommé en `.wav` | Refus, message du type « Ce fichier n'est pas un WAV » | | |
| 2 | Remplacer un son par un WAV 22 050 Hz | Refus, message du type « Format incompatible — 22 050 Hz, mono » (ou équivalent nommant la cause) | | |
| 3 | Remplacer un son par un WAV mono | Refus, message explicite sur le nombre de canaux | | |
| 4 | Remplacer un son par un WAV de ~10 s | Refus, message du type « Ce son dure X s — la limite est de 5 s » | | |
| 5 | Remplacer un son par un WAV de ~3 s | **Accepté**, mais un avertissement s'affiche (« Ce son dure X s — il sera accepté, mais c'est long pour un bruitage ») — la ligne passe bien à « Personnalisé » | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 4 — Éteindre l'interrupteur d'une seule cue, jouer une partie

**Objectif** : l'interrupteur par ligne coupe UN SEUL moment, les six autres continuent.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | Onglet Son, éteindre l'interrupteur de la ligne « Erreur » (paire ratée MEMORY / réponse invalidée RAFALE) | La ligne s'estompe visuellement | | |
| 2 | Lancer une partie réelle, déclencher les sept moments (départ, temps écoulé, points, erreur, révélation, entracte début/fin) | **Aucun son** au moment « Erreur » ; les **six autres** sonnent normalement | | |
| 3 | Sur la ligne « Erreur » (toujours éteinte), cliquer « ▶ Tester » puis les deux écoutes | Le son **sort quand même** — tester est un geste explicite, indépendant de l'interrupteur de ligne | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 5 — Éteindre l'interrupteur général

**Objectif** : l'interrupteur général reste maître, quels que soient les interrupteurs de ligne.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | Onglet Son, éteindre « Bruitages activés » (interrupteur du haut) | La pastille passe à « Son inactif » | | |
| 2 | Jouer une partie, déclencher plusieurs moments | **Aucun son** ne sort, pour aucune cue — y compris celles individuellement allumées | | |
| 3 | Rallumer l'interrupteur général | La pastille repasse à « Son actif » (si une sortie réelle est disponible) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 6 — Tester un moment éteint : le son sort quand même

*(Combiné avec le Scénario 4, étape 3 — reformulé ici pour une vérification isolée si besoin.)*

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | Une ligne avec l'interrupteur éteint, cliquer « ▶ Tester » → « 🔊 Jouer sur l'enceinte » | Le son sort sur l'enceinte — tester est un geste explicite, jamais bridé par l'interrupteur de ligne | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 7 — Tester avec les bruitages désactivés : message explicite

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | Interrupteur général éteint (voir Scénario 5) | — | | |
| 2 | Onglet Son, « ▶ Tester » une ligne → « 🔊 Jouer sur l'enceinte » | **Aucun silence inexpliqué** : un message explicite apparaît (« Bruitages désactivés — rien n'a été joué »), jamais une absence de retour | | |
| 3 | Rallumer l'interrupteur général avant de continuer les scénarios suivants | — | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 8 — La configuration Hue fonctionne exactement comme avant

**Objectif** : non-régression de l'assistant Hue (onglet Lumière) — R9 du plan, aucune ligne de
l'assistant ne doit avoir été modifiée par #230.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | Ouvrir `/admin/ambiance` — vérifier que l'onglet **Lumière** est actif par défaut | Comportement inchangé depuis #207/#208/#213 | | |
| 2 | Dérouler l'assistant Hue (recherche du pont, association, sélection des ampoules) jusqu'au bout, comme avant #230 | Aucune différence de comportement, aucun nouvel élément inattendu | | |
| 3 | Lancer une partie et vérifier que l'éclairage réagit normalement au jeu | Comportement inchangé | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 9 — Enceinte éteinte en cours de partie : le serveur ne dégrade jamais, et la pastille reste au vert

**Objectif** : ⚠️ **le comportement attendu ici est contre-intuitif — lire avant d'exécuter.**
La pastille d'état ne surveille rien en continu : elle date du démarrage du serveur. Ce
scénario vérifie que c'est bien documenté et que le serveur reste robuste, **pas** que la
pastille détecte la coupure (elle ne le fera jamais, par conception).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | Partie en cours, `sound.enabled=true`, pastille « Son actif », enceinte Bluetooth connectée | — | | |
| 2 | Éteindre l'enceinte (ou couper le Bluetooth) **en cours de partie** | Le jeu continue normalement — aucun gel, aucune erreur visible | | |
| 3 | Observer la pastille de l'onglet Son, SANS recharger la page | ⚠️ **Elle reste au vert « Son actif »** — c'est le comportement ATTENDU, documenté dans l'interface (« État établi au démarrage du serveur ») | | |
| 4 | Cliquer « ▶ Tester » sur une ligne → « 🔊 Jouer sur l'enceinte » | Le message de la section 2 reflète la réalité (le serveur tente l'envoi ; `result` peut rester `played` même si rien n'est physiquement entendu — c'est tout le sens du verdict manuel) | | |
| 5 | Rallumer l'enceinte, tester à nouveau | Le son peut à nouveau être entendu si le pilote se ré-arme (voir `tests/procedures/sound-output-driver-228.md` scénario 3 pour la validation détaillée du ré-armement) | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Critères de Validation

- [ ] Scénario 1 (remplacer, écouter, tester) : PASS
- [ ] Scénario 2 (restaurer, verdicts effacés) : PASS
- [ ] Scénario 3 (fichiers non conformes) : PASS
- [ ] Scénario 4 (interrupteur de ligne) : PASS
- [ ] Scénario 5 (interrupteur général) : PASS
- [ ] Scénario 6 (tester un moment éteint) : PASS
- [ ] Scénario 7 (tester avec bruitages désactivés) : PASS
- [ ] Scénario 8 (non-régression Hue) : PASS
- [ ] Scénario 9 (enceinte éteinte, pastille reste au vert) : PASS **avec ce comportement
      spécifique confirmé comme attendu, pas comme un bug**

## Notes QA

- Cette procédure nécessite du matériel audio réel et une oreille humaine — non exécutable
  par un agent, règle projet.
- Le Scénario 9 est le plus susceptible d'être mal interprété : si la pastille reste au vert
  après extinction de l'enceinte, **ce n'est PAS un échec** — documenter le contraire serait
  une fausse alerte.
- Pour la suite automatisée complémentaire (non-auditive), voir `go test ./internal/server/...
  -run 'TestGETAPISounds|TestPOSTAPISounds'`, `go test ./cmd/server/... -run TestT3_`, et la
  suite frontend `npx vitest run src/pages/AmbiancePage.sound.test.jsx` (`server-go/web/`).
