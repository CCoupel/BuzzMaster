# Procédure de Test — #248 lot 5b : retournement MEMORY / MEMOTION avec accusé (coupure WiFi)

**Version** : 11.2.1
**Date** : 2026-10-13
**Testeur** : utilisateur (QA n'exécute pas les procédures manuelles)
**Maquette** : `docs/mockup/v11.2.1/ui/vplayer_qcm_answer__ack.html` (flux MEMORY)

## Prérequis
- [ ] Environnement : QUALIF avec le serveur #248 lot 5b (FLIP_ID + FLIP_MEMORY_CARD_ACK)
- [ ] Une partie MEMORY multi-équipes et une manche MEMOTION avec une carte de type MEMORY ; une tablette par équipe ; possibilité de couper le WiFi d'une tablette

## Scénario 1 — Retournement nominal (équipe qui a la main)
| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | La tablette de l'équipe active touche une carte | Carte en attente, grille verrouillée, « ⏳ Retournement de la carte… » | | |
| 2 | Accusé du serveur | La carte se retourne normalement, plus de pastille, aucun « ✓ » | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 2 — Coupure WiFi pendant une PAIRE RATÉE (cas critique)
| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Retourner une 1re carte ; couper le WiFi ; toucher la 2e carte (paire ratée) | « ⚠ Connexion perdue — la carte sera retournée dès le retour du réseau » | | |
| 2 | Rétablir le WiFi | « ⏳ Renvoi du retournement… » puis la carte suit l'état du serveur | | |
| 3 | Vérifier la grille et la main | La carte n'est JAMAIS retournée deux fois (ni une paire faussée) ; la main passe une seule fois | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 3 — Main passée pendant l'attente
| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Tablette coupée, geste fait ; pendant ce temps la main passe à l'autre équipe ; WiFi rétabli | « ✕ Carte non retournée — ce n'est plus votre tour » ; carte qui tremble puis revient face cachée ; aucune invitation à réessayer | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 4 — Hors tour (dérogation conservée)
| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Une tablette qui n'a PAS la main touche une carte | Silence total : aucune pastille, aucun message | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 5 — Pause et fin de manche
| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Couper le WiFi, toucher une carte, mettre le jeu en PAUSE, rétablir le WiFi | « ⏸ Jeu en pause · ⏳ retournement en attente… », rien n'est envoyé | | |
| 2 | Reprendre le jeu | Renvoi puis accusé | | |
| 3 | Terminer la question avant le retour du réseau | « ✕ Carte non retournée » ; plus de renvoi | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Notes QA

