# Procédure de Test — Bugfix #242 : a11y de la Navbar

**Version** : 11.2.1
**Date** : 2026-10-12
**Testeur** : utilisateur (QA n'exécute pas les procédures manuelles)

## Prérequis
- [ ] Environnement : LOCAL, page admin, lecteur d'écran ou outil d'inspection d'accessibilité du navigateur (arbre d'accessibilité)

## Scénario 1 — Nom accessible du bouton d'entracte
| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Activer l'entracte (bouton « REPRISE » visible) puis inspecter le nom accessible du bouton | Le nom contient « REPRISE » (ex. « Reprise — fin d'entracte ») | | |
| 2 | Entracte inactif : inspecter le bouton « ENTRACTE » | Le nom contient « ENTRACTE » | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 2 — Menus déroulants
| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir « Préparation » (fenêtre < 1685 px) puis « Interface » (≥ 1685 px) et inspecter le conteneur de liste | Conteneur exposé comme groupe nommé (rôle `group`), pas comme `div` générique nommé | | |
| 2 | Navigation clavier et ouverture/fermeture des menus | Comportement inchangé | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Notes QA

