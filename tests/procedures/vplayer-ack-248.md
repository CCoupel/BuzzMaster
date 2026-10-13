# Procédure de Test — #248 : réponse QCM et buzz SPEEDY avec accusé (coupure WiFi)

**Version** : 11.2.1
**Date** : 2026-10-12
**Testeur** : utilisateur (QA n'exécute pas les procédures manuelles)
**Maquette** : `docs/mockup/v11.2.1/ui/vplayer_qcm_answer__ack.html`

## Prérequis
- [ ] Environnement : QUALIF, serveur avec #247 (pas de PAUSE fantôme sur renvoi)
- [ ] Une tablette VJoueur (équipe A) + un coéquipier (tablette B, même équipe) + écran admin
- [ ] Une question QCM et une question SPEEDY prêtes ; possibilité de couper le WiFi de la tablette (mode avion)

## Scénario 1 — QCM nominal
| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer la QCM, la tablette A touche un choix | Choix mis en avant, grille verrouillée, « ⏳ Envoi de votre réponse… » | | |
| 2 | Attendre l'accusé du serveur | « ✓ Réponse reçue » apparaît seulement à ce moment ; overlay « ✓ » habituel | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 2 — QCM, coupure WiFi réelle
| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Couper le WiFi de la tablette A, puis toucher un choix | « ⚠ Connexion perdue — votre réponse sera envoyée dès le retour du réseau » ; aucun « ✓ » | | |
| 2 | Rétablir le WiFi avant la fin de la question | « ⏳ Renvoi de votre réponse… » puis « ✓ Réponse reçue » ; une seule réponse enregistrée côté admin | | |
| 3 | Idem, mais rétablir le WiFi APRÈS la fin de la question (REVEALED) | « ✕ Réponse non reçue — la question est terminée » ; aucune invitation à réessayer | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 3 — Pause avant l'accusé
| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Couper le WiFi, répondre, mettre le jeu en PAUSE (admin), rétablir le WiFi | « ⏸ Jeu en pause · ⏳ envoi de votre réponse… », aucun envoi pendant la pause | | |
| 2 | Reprendre le jeu | Renvoi puis « ✓ Réponse reçue » | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 4 — Équipe déjà servie
| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Tablette A coupée ; la tablette B (même équipe) répond ; A touche un choix puis le WiFi revient | « ℹ Votre équipe a déjà répondu » (gris), pas d'erreur, plus de renvoi | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 5 — SPEEDY (mêmes cas avec le buzz)
| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Buzz nominal | « ⏳ Envoi de votre buzz… » puis « ✓ Buzz reçu » (overlay « BUZZÉ ! » après accusé seulement) | | |
| 2 | Buzz pendant une coupure WiFi puis retour réseau | « ⚠ … votre buzz sera envoyé… » puis « Renvoi de votre buzz… » puis « ✓ Buzz reçu » ; pas de PAUSE en double | | |
| 3 | Question terminée sans accusé | « ✕ Buzz non reçu — la question est terminée » | | |
| 4 | Un coéquipier a buzzé avant | « ℹ Votre équipe a déjà buzzé » | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Notes QA

