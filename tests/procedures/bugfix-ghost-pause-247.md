# Procédure de Test — Bugfix #247 : PAUSE fantôme sur appui refusé

**Version** : 11.2.1
**Date** : 2026-10-12
**Testeur** : utilisateur (QA n'exécute pas les procédures manuelles)

## Prérequis
- [ ] Environnement : LOCAL ou QUALIF, au moins un buzzer physique et une tablette VJoueur
- [ ] Admin, TV et tablette ouverts ; une question ARDOISE et une question SPEEDY prêtes

## Scénario 1 — Appui physique pendant une ARDOISE
| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer une question ARDOISE avec des textes saisis sur les tablettes | Zone ARDOISE visible sur l'admin | | |
| 2 | Appuyer sur un buzzer physique | Rien ne change : zone ARDOISE toujours visible, partie toujours en cours (pas de pause), LED inchangées | | |
| 3 | Continuer à saisir sur une tablette | Clavier actif, texte mis à jour (pas figé), pas de « ✓ Réponse envoyée » parasite | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 2 — Appui accepté (non-régression)
| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Question SPEEDY en cours, appuyer sur un buzzer | Pause normale, LED de pause allumées, ambiance passée en pause | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 3 — Appuis refusés
| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Question MEMORY/MEMOTION/RAFALE : appuyer sur un buzzer | Aucune pause, aucun changement | | |
| 2 | SPEEDY : un coéquipier appuie après le premier | Aucune nouvelle diffusion de pause | | |
| 3 | QCM : buzzer physique d'une équipe qui a un VJoueur | Aucune pause | | |
| 4 | QCM : coupure/reprise réseau d'une tablette qui renvoie sa réponse | Aucune pause supplémentaire | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Notes QA

