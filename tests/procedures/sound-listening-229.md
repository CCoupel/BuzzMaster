# Procédure de Test — Écoute des sons par défaut (#229, milestone v11.0)

**Version** : v11.0.0.x (QUALIF)
**Date** : 2026-09-21
**Testeur** : Utilisateur, à l'oreille, sur matériel réel
**Contrat** : `contracts/sound.md` §2/§3
**Plan** : `_work/reports/plan-dev-228-229-20260921-114500.md` Partie 2

## ⚠️ Préalable obligatoire — ne PAS exécuter avant que #228 ET #229 soient toutes les deux livrées

**Cette procédure ne produit un résultat valide que si le pilote (`#228`) ET le
générateur de sons (`#229`) sont TOUS LES DEUX présents dans le binaire testé.**

- `#228` seul (sans `#229`) : le pilote atteint le matériel, mais chaque cue envoie
  encore le paravent de `#227` (quelques octets de texte brut) — vous n'entendrez
  rien de reconnaissable. Voir `tests/procedures/sound-output-driver-228.md`.
- `#229` seul (sans `#228`) : les fichiers `.wav` existent sur disque
  (`data/files/sounds/`) et peuvent être écoutés **directement** (lecteur audio du
  système, hors BuzzControl) pour vérifier leur contenu, mais **le jeu ne les jouera
  pas** tant qu'aucun pilote réel n'est câblé.

**Si l'une des deux issues manque, arrêtez-vous ici** et signalez-le plutôt que de
consigner un résultat — un "aucun son entendu" serait alors un faux négatif, pas une
régression.

## Prérequis

- [ ] Binaire buildé depuis la branche `milestone/v11.0` (ou merge ultérieur),
      contenant **à la fois** `#228` et `#229`
- [ ] `config.json` : `sound.enabled = true`
- [ ] Un haut-parleur ou une enceinte fonctionnelle, volume raisonnable
- [ ] Serveur démarré **au moins une fois** avec ce binaire, pour que les 7 fichiers
      par défaut soient générés dans `data/files/sounds/` (génération automatique au
      premier lancement, silencieuse — voir les journaux si un doute existe)
- [ ] Idéalement, un environnement calme pour juger correctement de la présence ou
      non d'un clic audible en début/fin de son

## Scénario 1 — Écoute directe des 7 fichiers (hors jeu, la plus fiable)

**Objectif** : juger chaque son indépendamment du reste du pipeline (moteur, file,
pilote) — la mesure la plus directe de ce que `#229` a réellement produit.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | Ouvrir `data/files/sounds/` sur le serveur (ou copier les 7 fichiers `.wav` sur votre poste) | 7 fichiers : `depart.wav`, `temps-ecoule.wav`, `gagne.wav`, `perdu.wav`, `reveal.wav`, `entracte-debut.wav`, `entracte-fin.wav` | | |
| 2 | Lire chaque fichier avec un lecteur audio standard (VLC, lecteur système, etc.) | Chaque fichier se lit sans erreur, sans distorsion évidente | | |

Pour **chaque** son, noter le jugement ci-dessous :

| Son | Durée perçue < 1s ? | Clic audible au début ? | Clic audible à la fin ? | Caractère perçu (libre) |
|-----|---------------------|--------------------------|---------------------------|--------------------------|
| `depart` (appel ascendant bref suggéré) | | | | |
| `temps-ecoule` (deux notes descendantes suggérées) | | | | |
| `gagne` (arpège ascendant suggéré) | | | | |
| `perdu` (intervalle descendant sourd suggéré) | | | | |
| `reveal` (carillon suggéré) | | | | |
| `entracte-debut` (motif descendant suggéré) | | | | |
| `entracte-fin` (motif ascendant, symétrique suggéré) | | | | |

**Verdict** : [ ] PASS — aucun clic, durées correctes, caractères jugés satisfaisants
            [ ] PASS AVEC RÉSERVES — clics/durées/caractères à ajuster (les sons sont
                remplaçables par conception, #230 — pas un blocage de #229)
            [ ] FAIL — un ou plusieurs sons ne se lisent pas du tout, ou sont
                manifestement corrompus (silence total, bruit blanc, etc.)

---

## Scénario 2 — Écoute en jeu (bout en bout, pilote + générateur)

**Objectif** : confirmer que le pipeline COMPLET (moteur → pilote → matériel)
restitue bien ces mêmes sons pendant une vraie partie — pas seulement que les
fichiers sont corrects isolément (Scénario 1).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | Démarrer une question normale | Son `depart` entendu au démarrage (pas à la reprise après un buzz — voir la non-régression `#227`) | | |
| 2 | Faire crédit de points à une équipe | Son `gagne` entendu | | |
| 3 | Faire une paire ratée MEMORY (ou une invalidation RAFALE) | Son `perdu` entendu | | |
| 4 | Révéler la réponse | Son `reveal` entendu | | |
| 5 | Laisser le chrono global expirer sur une question courte | Son `temps-ecoule` entendu | | |
| 6 | Activer une question ENTRACTE (programmée ou manuelle) | Son `entracte-debut` entendu | | |
| 7 | Désactiver l'ENTRACTE | Son `entracte-fin` entendu | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Fichier personnalisé (aperçu de #230, vérification anticipée)

**Objectif** : confirmer qu'un fichier remplacé manuellement sur disque est bien
celui qui se joue — vérification anticipée du principe "le disque fait foi" avant
que `#230` ne livre l'upload par interface.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | Remplacer manuellement `data/files/sounds/gagne.wav` par un autre fichier `.wav` valide (même format canonique : PCM 16 bits, 44100 Hz, stéréo) | — | | |
| 2 | Faire crédit de points à une équipe, sans redémarrer le serveur | Le **nouveau** son se joue immédiatement, sans redémarrage ni action supplémentaire | | |
| 3 | Redémarrer le serveur | Le fichier personnalisé est toujours là (n'est jamais écrasé par la génération automatique) | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Critères de Validation

- [ ] Scénario 1 (écoute directe) : PASS ou PASS AVEC RÉSERVES
- [ ] Scénario 2 (écoute en jeu) : PASS
- [ ] Scénario 3 (fichier personnalisé) : PASS
- [ ] Aucun son ne dépasse ~1 seconde de façon flagrante
- [ ] Aucun clic net en début ou fin de son

## Notes QA

- Cette procédure nécessite une **oreille humaine** et du **matériel audio réel** —
  non exécutable par un agent (`qa`), règle projet.
- Le jugement sur le "caractère" de chaque son (Scénario 1) est **subjectif et
  ajustable** : #230 permettra de remplacer n'importe quel son, donc un caractère
  jugé imparfait n'est PAS un critère de blocage de #229 — seulement une réserve à
  noter.
- Si le Scénario 1 échoue (fichier corrompu/illisible), consigner **lequel** des 7
  sons pose problème et le message d'erreur du lecteur audio utilisé — c'est
  directement exploitable pour corriger le générateur sans deviner.
