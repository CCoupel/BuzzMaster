# Procédure de Test — Popover « Ambiance », son coupé, interrupteur éclairage (#284, #286)

**Version** : v11.4.0 (LOT-1)
**Date** : 2026-11-05
**Testeur** : QA (scénarios AUTO) / utilisateur (scénarios HUMAIN)
**Contrats** : `contracts/sound.md` §11, `contracts/http-endpoints.md` §Sound
**Maquette** : `docs/mockup/v11.4.0/ui/admin_navbar__ambiance_popover.html`

## Prérequis

- [ ] Environnement : QUALIF ou LOCAL, page admin `/admin` (page Jeu)
- [ ] Données : pont Hue configuré (ou fictif) et sortie audio disponible ; sinon seule la section présente s'affiche
- [ ] Au moins une question avec média audio (pour le scénario 5)
- [ ] Navigateur redimensionnable (1280 px et 1024 px de large)

## Scénarios

### Scénario 1 — Popover entièrement visible à 1280 px, en AUTO / ON / OFF (#284)

**Validation** : HUMAIN — **Issues** : #284

**Objectif** : le popover « Ambiance » ne déborde pas à gauche du viewport, quel que soit le mode.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Fenêtre à 1280 px, ouvrir le bouton « Ambiance » | Popover entièrement dans la fenêtre (bord gauche visible), titre « Ambiance — conduite en direct » | | |
| 2 | Cliquer ON | Avertissement « Mode ON engagé… » renvoyé à la ligne ; popover toujours entièrement visible | | |
| 3 | Cliquer OFF | Idem, avertissement lisible sans coupure à gauche | | |
| 4 | Cliquer AUTO | Avertissement disparu ; popover inchangé en position | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 2 — Popover entièrement visible à 1024 px (< 1094 px) (#284)

**Validation** : HUMAIN — **Issues** : #284

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Fenêtre à 1024 px : le libellé du bouton est masqué (glyphes seuls) | Bouton « Ambiance » avec 💡 (et 🔊/🔇) sans texte | | |
| 2 | Ouvrir le popover, enchaîner ON puis OFF | Aucun débordement horizontal ; largeur ≤ viewport − 2 rem ; pas de barre de défilement horizontale | | |
| 3 | Réduire à 800 px et ré-ouvrir | Popover toujours entièrement visible (texte renvoyé à la ligne) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 3 — Couper puis rétablir le son depuis l'en-tête (#286)

**Validation** : HUMAIN — **Issues** : #286

**Objectif** : le réglage est visible en permanence et son effet est immédiat.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Popover ouvert, section « Son » : cliquer OFF | Avertissement « Son coupé… conservé après redémarrage » ; bouton d'en-tête : libellé « Son coupé », glyphe 🔇, badge orange | | |
| 2 | Fermer le popover | L'indicateur « Son coupé » reste visible dans l'en-tête | | |
| 3 | Déclencher une cue (démarrer une question) | Aucun bruitage | | |
| 4 | Rouvrir, cliquer ON | Avertissement disparu ; glyphe 🔊 ; libellé « Ambiance » (ou « Mode … engagé » seul) | | |
| 5 | Déclencher une cue | Le bruitage est de nouveau audible, sans redémarrage | | |
| 6 | Section « Son » : bouton de test d'une cue (page Sons) pendant que le son est coupé | La cue de test est jouée (geste explicite) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 4 — Son coupé conservé après redémarrage (#286)

**Validation** : AUTO — **Issues** : #286
Équivalent automatisé : `TestSoundMuted286_Muted_SurvivesRestart_LoadFromFile`, `TestSoundMuted286_PostConfig_Muted_PersistedToDisk` (`internal/server/sound_muted_286_test.go`).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `POST /config.json {"sound":{"muted":true}}` | 200 ; `config.json` contient `"muted": true` | | |
| 2 | `curl -s http://localhost/shutdown`, relancer le serveur | Serveur démarré | | |
| 3 | `GET /api/sound/status` | `{"active":…,"muted":true}` | | |
| 4 | `POST /config.json {"sound":{"muted":false}}` puis relire `config.json` | Clé `muted` absente du fichier (valeur zéro) ; status `muted:false` | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 5 — Couper le son pendant un média de question (#286)

**Validation** : HUMAIN — **Issues** : #286

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer une question à média audio (chrono différé si disponible) | Le média joue | | |
| 2 | Cliquer OFF (Son) dans le popover | Le média s'arrête net ; le chronomètre différé démarre (non bloqué) | | |
| 3 | Cliquer ON | Le média n'est PAS repris automatiquement ; « Rejouer » manuel possible | | |
| 4 | Son coupé, préparer puis démarrer une autre question à média | La question démarre normalement (pas de message « média indisponible »), sans son ; le chronomètre ne reste pas bloqué | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 6 — Visibilité des sections du popover (#286)

**Validation** : HUMAIN — **Issues** : #286

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Pont Hue configuré + sortie audio disponible | Bouton « Ambiance » ; sections « Éclairage général » ET « Son » | | |
| 2 | Pas de pont Hue, sortie audio disponible | Bouton visible ; seule la section « Son » | | |
| 3 | Pont configuré, sortie audio indisponible | Bouton visible ; seule la section « Éclairage général » | | |
| 4 | Ni pont ni son | Bouton « Ambiance » absent | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 7 — Interrupteur général « Éclairage d'ambiance activé » (page Ambiance › Lumière) (#286)

**Validation** : HUMAIN — **Issues** : #286
Équivalents automatisés : `TestLightingMasterSwitch286_*` (`cmd/server`, `internal/server`).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir Ambiance › Lumière | Bloc « Éclairage d'ambiance activé » en tête, activé | | |
| 2 | Désactiver | Étapes de configuration grisées, bandeau d'état « désactivé » ; les ampoules de la zone générale et d'équipe s'éteignent | | |
| 3 | Ouvrir le popover de l'en-tête | Section Éclairage grisée avec renvoi vers « Ambiance › Lumière » ; ON/AUTO/OFF sans effet | | |
| 4 | Jouer une question (changement de couleur de thème) | Aucune ampoule ne change | | |
| 5 | Réactiver | Pilote reconstruit ; la scène courante est ré-appliquée aux ampoules | | |
| 6 | Redémarrer le serveur éclairage désactivé | Réglage conservé ; ampoules inertes | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 8 — Extinction Hue au basculement (matériel) (#286)

**Validation** : HUMAIN — **Issues** : #286

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Mode ON, ampoules allumées (général + équipes) | Ampoules allumées | | |
| 2 | Désactiver l'interrupteur général | Toutes les ampoules s'éteignent en quelques secondes | | |
| 3 | Pont injoignable pendant la désactivation | Aucune erreur bloquante ; réponse HTTP rapide | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Criteres de Validation

- [ ] Popover dans le viewport à 1280 / 1024 / 800 px en AUTO, ON, OFF
- [ ] Son coupé : cues, média de question et test hors-jeu conformes au contrat
- [ ] Indicateur « Son coupé » visible en permanence, persistant au redémarrage
- [ ] Aucune écriture Hue quand l'éclairage est désactivé

## Notes QA

