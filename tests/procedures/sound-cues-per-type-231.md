# Procédure de Test — Bruitages fins par type de question (#231)

**Version** : 11.6.0 (LOT-3)
**Date** : 2026-11-09
**Testeur** : QA / utilisateur (poste Windows avec enceinte)

> Contrats : `contracts/sound.md` §2.1 (amendement #231 : 8 cues, une cue par site), `contracts/http-endpoints.md` (`GET /api/sounds` typé).
> Maquette : `docs/mockup/v11.6.0/ui/ambiance_settings_panel__global_per_type_tabs.html`.
> Équivalents automatisés : `cmd/server/sound_231_test.go`, `internal/audio/cues_231_test.go`, `internal/audio/synth/cues_231_test.go`, `internal/server/sound_cues_231_test.go`, `web/src/components/AmbianceSettingsPanel.231.test.jsx`, `web/src/components/SoundsManager.cues231.test.jsx`.

## Prérequis

- [ ] Environnement : QUALIF (binaire Windows), enceinte reliée au serveur, `sound.enabled = true`, son non coupé
- [ ] Un quiz de test avec : une QCM avec indices activés, une MEMORY (≥ 3 paires), une manche MEMOTION (≥ 2 cartes), une manche RAFALE (réservoir d'au moins 5 questions, lot de 3 questions)
- [ ] Une équipe, des buzzers ou la page animateur
- [ ] Un WAV de remplacement (≤ 2 s) reconnaissable

## Scénarios

### Scénario 1 — Page Ambiance › Son : cinq onglets et 15 bruitages

**Validation** : HUMAIN — **Issues** : #231

**Objectif** : l'écran présente les bruitages par type, conforme à la maquette.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir `/admin/ambiance` › Son | Onglets Global 7 · QCM 1 · MEMORY 2 · MEMOTION 2 · RAFALE 3 ; l'onglet Global liste les 7 bruitages historiques puis « Décompte du chronomètre » (sons longs) | | |
| 2 | Parcourir les 4 autres onglets | Chacun ne contient que ses événements : Indice révélé · Carte retournée / Paire trouvée · Carte choisie / Fin de carte · Bonne réponse / Mauvaise réponse / Fin de lot | | |
| 3 | Sur chaque ligne : « Tester » | Un son par défaut distinct sort de l'enceinte pour chacun des 8 nouveaux événements, court (≈ 0,3 à 1,1 s) | | |
| 4 | Comparer à la maquette | Même disposition (compteurs, colonnes Actif / Fichier / Durée / Actions) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 2 — Écoute en partie réelle : un seul son par événement

**Validation** : HUMAIN — **Issues** : #231

**Objectif** : chaque événement déclenche son bruitage, un seul, sans doublon avec les sons existants (D4).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | QCM : laisser un indice barrer une réponse | « Indice révélé » (une seule fois) | | |
| 2 | MEMORY : retourner une carte | « Carte retournée » | | |
| 3 | MEMORY : trouver une paire | « Paire trouvée » (sans « Erreur ») | | |
| 4 | MEMORY : rater une paire | « Erreur » (le son historique), pas « Paire trouvée » | | |
| 5 | MEMOTION : choisir une carte | « Carte choisie » | | |
| 6 | MEMOTION : fermer une carte **sans** gagnant | « Fin de carte » | | |
| 7 | MEMOTION : fermer une carte **avec** gagnant | « Points gagnés » seulement (pas « Fin de carte ») | | |
| 8 | RAFALE : valider une réponse | « Bonne réponse » | | |
| 9 | RAFALE : invalider une réponse | « Mauvaise réponse » à la place de « Erreur » | | |
| 10 | RAFALE : terminer le lot (dernière question ou temps du lot écoulé) | « Fin de lot » ; « Temps écoulé » reste pour le chronomètre global | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 3 — Désactiver, remplacer, restaurer une nouvelle cue

**Validation** : HUMAIN — **Issues** : #231

**Objectif** : les interrupteurs et fichiers des nouvelles cues se comportent comme ceux des 7 historiques.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Onglet RAFALE : désactiver « Bonne réponse » ; valider une réponse en partie | Aucun son ; « Tester » joue quand même | | |
| 2 | Onglet MEMORY : « Remplacer » « Paire trouvée » par le WAV de test | Message de succès ; « personnalisé » affiché ; la nouvelle paire joue votre son | | |
| 3 | « Restaurer » cette ligne | Retour au son par défaut | | |
| 4 | Couper le son (popover Ambiance) puis rejouer un événement | Aucun bruitage | | |
| 5 | « Restaurer les sons par défaut » (en bas) | Les 15 bruitages et le décompte reviennent au défaut | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 4 — Non-régression des 7 bruitages historiques

**Validation** : AUTO — **Issues** : #231

**Objectif** : les anciens bruitages gardent leurs sites et leurs fichiers.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Départ de question, fin du chrono, crédit de points, révélation, entracte (début / fin) | Mêmes sons qu'avant le lot | | |
| 2 | Un fichier remplacé avant la mise à jour | Toujours en place (non écrasé au démarrage) | | |
| 3 | Un indice QCM | Joue « Indice révélé » seulement, aucun bruitage global en plus | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Criteres de Validation

- [ ] Scénarios 1 à 3 (écoute réelle et lecture de l'écran) validés par l'utilisateur
- [ ] Scénario 4 (AUTO) couvert par les tests automatisés listés en tête
- [ ] Aucun événement ne joue deux bruitages à la fois

## Notes QA

[Espace pour observations]
