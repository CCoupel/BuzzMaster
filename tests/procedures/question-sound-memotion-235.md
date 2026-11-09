# Procédure de Test — Son sur MEMORY, ENTRACTE et cartes MEMOTION (#235)

**Version** : 11.6.0 (LOT-2)
**Date** : 2026-11-09
**Testeur** : QA / utilisateur (poste Windows avec enceinte)

> Contrats : `contracts/sound.md` §14, `contracts/http-endpoints.md` (« POST /questions — sons de cartes MEMOTION »), `contracts/game-state.md` (`QUESTION_SOUND_OWNER`).
> Maquette : `docs/mockup/v11.6.0/ui/admin_question_editor__sound_memory_memotion_entracte.html`.
> Équivalents automatisés : `internal/server/question_sound_cards_235_test.go`, `cmd/server/question_sound_235_test.go`, `web/src/pages/QuestionsPage.sound.235.test.jsx`, `web/src/utils/phaseRules.sound.235.test.js`.

## Prérequis

- [ ] Environnement : QUALIF (binaire Windows), enceinte reliée au serveur, `sound.enabled = true`, son non coupé
- [ ] Trois WAV d'écoute faciles à distinguer : `hote.wav` (musique de générique, ~25 s), `jingle-a.wav` (~8 s), `jingle-b.wav` (~8 s)
- [ ] Un quiz de test avec : une question MEMORY, une ENTRACTE, une manche MEMOTION de 3 cartes (A, B, C)
- [ ] Une tablette ou la page animateur (`/anim`) pour la conduite

## Scénarios

### Scénario 1 — MEMORY : son de manche

**Validation** : HUMAIN — **Issues** : #235

**Objectif** : le son attaché à une question MEMORY démarre au START et s'arrête avec la manche.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Éditeur : question MEMORY → bloc « Son de la question » → `hote.wav`, « En boucle » | Bloc visible avec « Chrono démarre » et « Lecture » ; enregistrement accepté | | |
| 2 | Lancer la manche | Le son démarre au START (pendant la phase de mémorisation) et boucle | | |
| 3 | STOP | Le son s'arrête immédiatement | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 2 — ENTRACTE : son d'entracte (boucle) et arrêt à la sortie

**Validation** : HUMAIN — **Issues** : #235

**Objectif** : le son d'entracte démarre juste après le bruitage de début et s'arrête quand l'entracte se termine.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Éditeur : question ENTRACTE → bloc « Son de l'entracte » → `hote.wav`, « En boucle » | Pas d'option « Chrono démarre » (aucun chronomètre) | | |
| 2 | Lancer l'entracte (manuellement, puis en entracte programmée) | Bruitage « Début d'entracte » puis le son d'entracte en boucle — une seule lecture, pas de doublon | | |
| 3 | Pendant l'entracte : gestes Pause / Reprendre / Stop du son | Ils s'appliquent au son d'entracte | | |
| 4 | Terminer l'entracte | Bruitage « Fin d'entracte », le son d'entracte s'arrête | | |
| 5 | Couper le son (popover Ambiance) puis relancer une entracte | Aucun son d'entracte | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 3 — MEMOTION : son d'hôte remplacé par le son de carte (D9)

**Validation** : HUMAIN — **Issues** : #235

**Objectif** : l'hôte sonne au START ; la carte sonore le remplace sans reprise ; une carte sans son laisse l'hôte continuer.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Éditeur : manche MEMOTION → « Son de la manche » = `hote.wav` ; carte A = `jingle-a.wav` ; carte B = `jingle-b.wav` (boucle) ; carte C sans son | Pas de « Chrono démarre » pour l'hôte ; chip ♪ sur A et B seulement | | |
| 2 | Lancer la manche | `hote.wav` démarre au START | | |
| 3 | Retourner la carte C (sans son) | L'hôte continue sans coupure | | |
| 4 | Fermer C, retourner la carte A | `hote.wav` s'arrête net, `jingle-a.wav` joue | | |
| 5 | Fermer A (point attribué ou non) | Le jingle s'arrête ; **l'hôte ne reprend pas** | | |
| 6 | Retourner B, puis révéler la réponse | `jingle-b.wav` boucle puis s'arrête à la révélation | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 4 — Chrono de la carte différé (« À la fin ») et conduite animateur

**Validation** : HUMAIN — **Issues** : #235

**Objectif** : le chrono de la carte attend la fin du jingle ; les gestes de l'animateur agissent sur le son courant.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Éditeur : carte A, « Chrono de la carte démarre » = « À la fin » | « En boucle » devient indisponible (exclusif) | | |
| 2 | Jouer la carte A | Le jingle joue ; le chrono de la carte ne décompte qu'à la fin du jingle | | |
| 3 | Rejouer la carte A ; au milieu du jingle, cliquer « Stop » côté animateur | Le son s'arrête et le chrono de la carte démarre | | |
| 4 | Sur `/anim` et sur l'admin pendant une carte sonore | La ligne de contrôle du son (Rejouer / Pause / Stop) est visible, même si l'hôte n'a pas de son | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 5 — Fichier de carte indisponible

**Validation** : HUMAIN — **Issues** : #235

**Objectif** : un fichier de carte manquant ne bloque ni la manche ni le retournement ; l'alerte est visible.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Supprimer à la main le fichier son de la carte A dans `data/files/questions/<id>/` | — | | |
| 2 | Lancer la manche MEMOTION | La manche démarre normalement (la préparation n'est pas bloquée par un fichier de carte) | | |
| 3 | Retourner la carte A | La carte s'ouvre, aucun son ; la pastille d'alerte du son s'affiche | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 6 — Ré-édition : les sons de cartes ne disparaissent pas

**Validation** : AUTO — **Issues** : #235

**Objectif** : modifier le titre ou une autre carte n'efface aucun son (« destruction silencieuse »).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Éditer la manche, changer seulement le titre, enregistrer | Réouvrir : tous les sons de cartes et leurs réglages (boucle, « À la fin ») sont conservés | | |
| 2 | Supprimer le son de la carte B (« Supprimer »), enregistrer | Seule B perd son son ; le fichier disparaît du disque ; A garde le sien | | |
| 3 | Supprimer la carte A de la manche, enregistrer | Le fichier son de A est supprimé du disque | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 7 — Non-régression (questions sans nouveau son)

**Validation** : AUTO — **Issues** : #235

**Objectif** : les types déjà sonores et les manches sans son se comportent comme avant.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Jouer une SPEEDY / QCM avec son (mode simultané puis « À la fin ») | Comportement v11.1/v11.4 inchangé | | |
| 2 | Jouer une manche MEMOTION et une ENTRACTE sans aucun son | Aucun son de média, aucune erreur dans les logs | | |
| 3 | Éditeur RAFALE | Aucun bloc son | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Criteres de Validation

- [ ] Scénarios 1 à 5 (écoute réelle sur enceinte) validés par l'utilisateur
- [ ] Scénarios 6 et 7 (AUTO) couverts par les tests automatisés listés en tête
- [ ] Aucun doublon de lecture entre bruitages (cues) et sons de média

## Notes QA

[Espace pour observations]
