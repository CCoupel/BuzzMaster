# Procédure de Test — Bugfix #189 : zone réponse /anim en REVEAL MEMOTION

**Version** : 11.2.1
**Date** : 2026-10-12
**Testeur** : utilisateur (QA n'exécute pas les procédures manuelles)

## Prérequis
- [ ] Environnement : LOCAL, tablette ou navigateur sur `/anim`
- [ ] Une question MEMOTION avec au moins une carte

## Scénario 1 — Carte MEMOTION révélée
| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer la question MEMOTION, sélectionner une carte, passer en sous-phase QUESTION | Zone « Réponse » masquée (floue), révélable en maintenant le doigt | | |
| 2 | Révéler la carte (sous-phase REVEAL) | Zone « Réponse » visible en permanence, sans appui maintenu | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 2 — Non-régression question classique
| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Question classique en STARTED | Zone « Réponse » masquée, révélable par appui | | |
| 2 | Révéler la réponse (REVEALED) | Zone visible en permanence | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Notes QA

