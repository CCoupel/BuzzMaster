# Handoff — DEPLOY QUALIF v3.8.0

**Feature** : QUALIF déploiement v3.8.0 WebSocket broadcast + ACK  
**SHA** : `2acbaec` (chore(v3.8.0): bump package.json version to 3.8.0)  
**Branche** : `feature/ws-broadcast-ack-v380`

## Ce qui a été fait

QUALIF v3.8.0 exécutée et validée sur branche `feature/ws-broadcast-ack-v380` :
- Correction `package.json` 3.7.0 → 3.8.0 et commit pushé
- Build React + 3 binaires Go (Windows 17MB, ARM64 16MB, natif 21MB)
- Serveur démarré sur port 8080, v3.8.0 confirmée dans les logs
- Smoke tests : 3 endpoints HTTP (200 OK) + 5 endpoints WS (101 Switching Protocols)
- Rapport QUALIF_REPORT_v3.8.0.md généré à la racine (non-commité)

## Décisions clés

1. **package.json oublié** : la version 3.7.0 était restée dans package.json (oubli phase doc). Corrigé et commité en QUALIF directement sur la branche feature
2. **Firmware PIO indisponible** : PIO non installé en WSL → firmware v3.7.0 conservé dans assets (compatible v3.8.0 serveur). La CI recompilera v3.8.0 au tag PROD
3. **HTTP 101** retourné par tous les endpoints WebSocket = succès réel (upgrade accepté, pas juste réponse 101 générique)
4. **Port 8080** utilisé (non-root, standard WSL) — le serveur de PROD tourne sur port 80

## Points d'attention

- Le binaire Windows `buzzcontrol-v3.8.0-windows-amd64.exe` est dans `server-go/` (non-commité, produit local)
- La CI GitHub Actions est le seul moyen d'avoir un firmware v3.8.0 officiel compilé
- Avant merge PROD, vérifier que `package.json` est bien à 3.8.0 (commit `2acbaec` est sur la branche)
- Le rapport QUALIF_REPORT_v3.8.0.md est non-commité (à la racine du repo, fichier local uniquement)

## Fichiers modifiés

- `server-go/web/package.json` — version 3.7.0 → 3.8.0 (commit `2acbaec`)
- `QUALIF_REPORT_v3.8.0.md` — rapport QUALIF (fichier local, non-commité)
