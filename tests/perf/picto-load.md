# Test de Performance — Relais de dessin PICTO (#193, v13.0.0)

Contrat : `contracts/picto.md` §8.3, §8.5. Script : `server-go/cmd/server/picto_load_193_test.go` (vrai hub, vraies
WebSockets). Seuils : `testing.perf.picto_relay` de `project-config.json`. Arbitrage :
`_work/reports/plan-arbitrage-perf-picto-20261123.md`.

## Seuils cibles

| Scénario | Débit × points × durée | Points au total | Seuils |
|----------|------------------------|-----------------|--------|
| Nominal (`TestPictoLoad193_Nominal_…`) | 16 trames/s × 8 points × 280 s | 35 840 | 0 déconnexion, 0 trame perdue, p95 ≤ 100 ms |
| Trame maximale (`TestPictoLoad193_MaxFrame_…`) | 16 trames/s × 200 points × 15 s | 48 000 | 0 déconnexion, 0 trame perdue, p95 ≤ 100 ms, trame ≤ 4 096 o |
| 3 × le débit (`TestPictoLoad193_TripleRate_…`) | 48 trames/s × 200 points × 60 s | > 60 000 | 0 déconnexion ; pertes, `RATE_LIMITED` et `CANVAS_FULL` tolérés (comptés et rapportés) |

La manche de la fixture dure 300 s, maximum du contrat (§4) : le nominal dessine 280 s (et non 300 s) pour ne pas être
interrompu par le chronomètre. Une garde fait échouer explicitement un scénario dont « durée + 15 s de marge » dépasse
la manche, et la phase `STARTED` est vérifiée à la fin du dessin.

La borne de 60 000 points par époque (contrat §8.3) n'est jamais dépassée par le nominal ni par la trame maximale :
une garde de cohérence (`fps × points × durée ≤ 60 000`) fait échouer explicitement un scénario qui la franchirait
(par exemple `PICTO_LOAD_SECONDS` trop grand).

## Scénario de charge

- 1 TV + 1 admin + 1 anim + 12 VJoueurs (équipe TeamA), dessinateur = Alice, manche en `STARTED`.
- Le dessinateur envoie des trames au débit voulu ; trois clients de mesure horodatent chaque `SEQ`.
- Sortie `PERF …` : débit, points par trame, durée, trames envoyées, `CANVAS_FULL` / `RATE_LIMITED` reçus par le
  dessinateur, puis par destinataire : trames reçues, p50/p95/p99, trame maximale.

## Exécution

```bash
cd server-go
# mesure complète des trois scénarios (≈ 6 min) — scope perf, hors boucle rapide
bash ../tests/perf/picto-load.sh
# durées raccourcies (la garde de borne reste appliquée)
PICTO_LOAD_SECONDS=30 bash ../tests/perf/picto-load.sh
# smoke de 5 s par scénario, mêmes seuils (sans PICTO_LOAD)
go test ./cmd/server -run TestPictoLoad193 -v
```

Un scénario à 0 perte avec une trame perdue, un p95 > 100 ms, une trame > 4 096 o ou une déconnexion est un KO.
Le rapport est la sortie `PERF …`, à recopier dans le rapport de QA.
