# Procédure de Test — Son fiable : diagnostic de la sortie audio (#283) et conversion des WAV (#232)

**Version** : 11.6.0 (LOT-1)
**Date** : 2026-11-09
**Testeur** : QA / utilisateur (poste Windows avec enceinte)

> Contrats : `contracts/http-endpoints.md` (« GET /api/sound/status — diagnostic », « Uploads sonores — conversion automatique »), `contracts/sound.md` §3 (amendement #232).
> Maquette : `docs/mockup/v11.6.0/ui/admin_sounds_manager__output_diagnostic.html`.
> Équivalents automatisés : `internal/audio/resample_232_test.go`, `internal/server/sound_upload_232_test.go`, `internal/server/sound_status_283_test.go`, `cmd/server/sound_diagnostic_283_test.go`, `web/src/pages/AmbiancePage.diagnostic.283.test.jsx`.

## Prérequis

- [ ] Environnement : QUALIF (binaire Windows `server.exe` + `config.json` du répertoire de lancement), navigateur sur `/admin/ambiance` onglet **Son**
- [ ] Une enceinte (filaire ou Bluetooth) reliée au poste serveur ; volume système audible
- [ ] Fichiers de référence (à préparer une fois, ex. avec Audacity ou `ffmpeg`) :
  - `ref-48k-24b-mono.wav` : sinus 440 Hz puis balayage 1 → 20 kHz, **48 000 Hz, 24 bits, mono**, ~3 s
  - `ref-float32.wav` : même contenu en **float 32 bits** stéréo
  - `ref-canonique.wav` : même contenu en **44 100 Hz, 16 bits, stéréo**
  - `faux.mp3` : un MP3 quelconque (renommé aussi `faux.wav` pour le cas « MP3 déguisé »)
- [ ] `config.json` : `sound.enabled = true` au départ

## Scénarios

### Scénario 1 — Écoute d'un bruitage converti (48 kHz / 24 bits / mono)

**Validation** : HUMAIN — **Issues** : #232

**Objectif** : un WAV qui était refusé (« format non canonique ») est maintenant accepté et se joue proprement sur l'enceinte.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Onglet Son, ligne « Départ » → **Remplacer** → `ref-48k-24b-mono.wav` | Toast de succès contenant « converti depuis 48000Hz/1ch/24bit » (aucun refus) | | |
| 2 | Cliquer **▶ Tester** puis **Tester sur l'enceinte** | Le son sort sur l'enceinte, à la bonne hauteur (440 Hz stable), sans craquement ni « sifflement » parasite | | |
| 3 | Réécouter le balayage aigu (partie 10 → 20 kHz) | Pas de tonalité fantôme qui redescend en fréquence (repliement) ; le son s'éteint simplement vers l'aigu | | |
| 4 | Comparer le volume au son par défaut « Départ » restauré (**Restaurer**) | Niveau du même ordre (pas d'écrêtage, pas d'affaiblissement net) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 2 — Même conversion sur les deux autres chemins (décompte et son de question)

**Validation** : HUMAIN — **Issues** : #232

**Objectif** : la conversion s'applique aussi au décompte du chronomètre et au son d'une question.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ligne « Décompte du chronomètre » → **Remplacer** → `ref-float32.wav` | Toast « converti depuis 48000Hz/2ch/32bit » (ou la fréquence réelle du fichier) | | |
| 2 | **Tester sur l'enceinte** (décompte) | Écoute propre, durée conforme au fichier | | |
| 3 | Éditeur de questions : créer une question SPEEDY avec `ref-48k-24b-mono.wav` comme son | Enregistrement accepté, message « Son converti depuis 48000Hz/1ch/24bit » | | |
| 4 | Lancer la question en jeu | Le son de la question est audible, mono rendu sur les deux voies, sans artefact | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 3 — Fichier déjà canonique : rien ne change

**Validation** : AUTO — **Issues** : #232

**Objectif** : un WAV déjà canonique n'est pas retraité (non-régression).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Remplacer « Départ » par `ref-canonique.wav` | Toast « Son remplacé. » **sans** mention « converti » | | |
| 2 | Comparer `data/files/sounds/depart.wav` au fichier envoyé (somme SHA-256) | Fichiers identiques octet pour octet | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 4 — Refus des formats compressés

**Validation** : AUTO — **Issues** : #232

**Objectif** : MP3 et WAV compressé restent refusés avec un message qui nomme la cause.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Remplacer « Départ » par `faux.mp3` | Refus (extension non WAV), son existant inchangé | | |
| 2 | Remplacer « Départ » par `faux.wav` (MP3 déguisé) | Refus nommant la cause (format compressé / non PCM), son existant inchangé | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 5 — Diagnostic, sortie OK

**Validation** : HUMAIN — **Issues** : #283

**Objectif** : l'état nominal est lisible et ne montre aucune alerte.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Démarrer le serveur avec l'enceinte connectée, ouvrir l'onglet Son | Pastille « Son actif » ; aucune bannière « Redémarrage requis » ni « Enceinte indisponible » | | |
| 2 | Comparer à la maquette (état « Sortie OK ») | Même disposition (interrupteur, pastille, ligne de cause) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 6 — Diagnostic, enceinte indisponible

**Validation** : HUMAIN — **Issues** : #283

**Objectif** : une enceinte absente au démarrage est expliquée, pas seulement signalée par une pastille grise.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Arrêter le serveur, **déconnecter** l'enceinte Bluetooth / débrancher la sortie, relancer le serveur | Serveur démarré normalement | | |
| 2 | Ouvrir l'onglet Son | Bloc « Enceinte indisponible » avec la cause renvoyée par le système (ex. « oto: no output device ») | | |
| 3 | Cliquer **Tester sur l'enceinte** | Le bouton reste proposé ; échec explicite avec le même motif (toast) | | |
| 4 | Reconnecter l'enceinte, redémarrer le serveur | L'état repasse à « Son actif », bloc disparu | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 7 — Interrupteur changé sans redémarrage

**Validation** : HUMAIN — **Issues** : #283

**Objectif** : modifier « Bruitages activés » après le démarrage affiche la bannière « Redémarrage requis » (la sortie n'est ouverte qu'au démarrage).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Serveur lancé avec `sound.enabled = false` ; ouvrir l'onglet Son | Note « Bruitages désactivés » (config.json) | | |
| 2 | Activer l'interrupteur « Bruitages activés » | Bannière « **Redémarrage requis** » ; pastille inchangée (« Son inactif ») ; aucun son joué | | |
| 3 | Désactiver à nouveau l'interrupteur | Bannière disparue (valeur = celle du démarrage) | | |
| 4 | Activer, **redémarrer** le serveur | « Son actif », bannière absente, « Tester sur l'enceinte » joue | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 8 — Journal de démarrage

**Validation** : AUTO — **Issues** : #283

**Objectif** : une seule ligne `AUDIO` décrit la sortie au démarrage, avec le même texte que l'interface.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Démarrer le serveur ; ouvrir l'onglet Logs (ou le fichier journal) | Exactement une ligne `sound: enabled=<b> output=<ok\|unavailable\|disabled> reason="…" device=… format=44100Hz/2ch/16bit (config: <chemin>)` | | |
| 2 | Cas enceinte absente (scénario 6) | Ligne en niveau WARN/ERROR, `reason` identique à la cause affichée dans l'onglet Son | | |
| 3 | Cas `sound.enabled=false` | `output=disabled reason="sound.enabled=false"`, niveau INFO | | |
| 4 | Le chemin `config:` | C'est bien le `config.json` du répertoire de lancement (chemin absolu) | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Criteres de Validation

- [ ] Les scénarios 1, 2, 5, 6, 7 (écoute / lecture à l'écran) sont validés par l'utilisateur sur le poste Windows
- [ ] Les scénarios AUTO (3, 4, 8) sont couverts par les tests automatisés listés en tête
- [ ] Aucune régression sur les 7 bruitages par défaut et sur le son de question existant

## Notes QA

[Espace pour observations]
