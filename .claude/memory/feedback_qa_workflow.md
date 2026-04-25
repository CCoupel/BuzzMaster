---
name: QA validée — continuer sans attendre
description: Après QA VALIDATED, lancer DOC + DEPLOY QUALIF directement sans demander validation utilisateur
type: feedback
---

Après un verdict QA VALIDATED (ou VALIDATED WITH RESERVATIONS sans bloquant), lancer immédiatement DOC + DEPLOY QUALIF sans attendre la validation de l'utilisateur.

**Why:** L'utilisateur n'a pas besoin d'approuver la transition QA → DOC → QUALIF. Ce sont des étapes automatiques. La seule validation requise est celle de la QUALIF (test visuel) et de la PROD.

**How to apply:**
- QA VALIDATED → lancer doc-updater directement
- doc-updater terminé → lancer deployer directement
- QUALIF déployée → demander validation visuelle à l'utilisateur
- Validation QUALIF OK → demander GO pour PROD (ou fermer issues si milestone)
