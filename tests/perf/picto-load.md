# Test de Performance — Relais de dessin PICTO (#193, v13.0.0)

Contrat : `contracts/picto.md` §8.5. Script : `server-go/cmd/server/picto_load_193_test.go` (vrai hub, vraies WebSockets).

## Seuils cibles (à reporter dans `testing.perf`)

| Mesure | Seuil |
|--------|-------|
| Relais serveur, entrée → sortie (TV, admin, anim) | p95 ≤ 100 ms |
| Taille d'une trame `PICTO_STROKES` relayée | ≤ 4 Ko |
| Débit nominal (16 trames/s × 200 points), 5 minutes | 0 déconnexion, 0 trame perdue |
| 3 × le débit nominal | 0 déconnexion (pertes tolérées, rattrapées par `PICTO_CANVAS_SYNC`) |

## Scénario de charge

- 1 TV + 1 admin + 1 anim + 12 VJoueurs (équipe TeamA), dessinateur = Alice, manche en `STARTED`.
- Le dessinateur envoie des trames de 200 points au débit voulu ; trois clients de mesure horodatent chaque `SEQ`.

## Exécution

```bash
cd server-go
# mesure complète (5 min nominal + 60 s à 3 ×) — scope perf, hors boucle rapide
bash ../tests/perf/picto-load.sh
# durée raccourcie
PICTO_LOAD_SECONDS=30 bash ../tests/perf/picto-load.sh
# smoke de 5 s, mêmes seuils (sans PICTO_LOAD)
go test ./cmd/server -run TestPictoLoad193 -v
```

Le rapport est la sortie `PERF …` du test (trames reçues/envoyées, p50/p95/p99, trame max par destinataire), à
recopier dans le rapport de QA. Une trame perdue au débit nominal ou un p95 > 100 ms est un KO.
