# spike-hue-bridge — spike de confirmation de la voie Hue Bridge (v10.0.0)

> Programme **jetable**, hors serveur (module Go séparé, aucune intégration à `server-go`).
> Il tourne contre le **bridge Hue du domicile de l'utilisateur** : voir la section Sécurité — elle
> est appliquée **dans le code** (`guard.go`), pas seulement dans la doc.

## Sécurité — ce que le programme peut faire, et rien d'autre

| Opération | Chemin | Usage |
|---|---|---|
| `POST /api` | enregistrement applicatif standard (appui sur le bouton du bridge) | `register` |
| `GET /api/<clé>/lights`, `GET /api/<clé>/lights/<id>` | retrouver l'ampoule **`BuzzHue1`** par son nom exact | toutes les commandes |
| `PUT /api/<clé>/lights/<id>/state` | on/off, luminosité, couleur de **cette seule ampoule** | `on`, `off`, `bri`, `colour`, `demo`, `bench` |

Garde-fous codés (`guard.go`, `hue.go`) — testés dans `guard_test.go` :
- **toute autre requête est refusée avant d'être émise** : aucun groupe/zone (jamais l'id `0` =
  toutes les lumières, ni `BuzzMaster1`), aucune scène, règle, planification, capteur,
  `resourcelinks`, `config`, whitelist, firmware, aucun `DELETE`, aucun renommage ;
- l'ampoule cible est résolue par **nom exact** ; `0` ou `>1` correspondance ⇒ arrêt, jamais de
  repli sur une autre ampoule ;
- **avant chaque écriture**, l'ampoule est relue et son nom re-vérifié ; si elle a été renommée,
  l'écriture est refusée ;
- l'id doit être un entier strictement positif.

La clé API est stockée dans `.hue-username` (gitignoré, droits 0600) et jamais affichée.

## Exécution (dans l'ordre)

```bash
cd spike/hue-bridge && go build -o spike-hue-bridge .        # ou GOOS=windows … -o spike-hue-bridge.exe

spike-hue-bridge discover                                   # mDNS _hue._tcp puis SSDP ; aucun appel au bridge
spike-hue-bridge register                                   # "Press the LINK BUTTON…" → appuyer sur le bouton
spike-hue-bridge lights                                     # liste ids/noms (lecture), flèche sur BuzzHue1
spike-hue-bridge state                                      # état de BuzzHue1 (lecture)
spike-hue-bridge -out demo.json demo                        # on → 254 → rouge → vert → bleu → blanc → 60 → off
spike-hue-bridge -iterations 12 -interval 300ms -out bench.json bench   # fiabilité + p50/p95
spike-hue-bridge colour red ; spike-hue-bridge off          # commandes unitaires
```

Si la découverte échoue : `-bridge-ip 192.168.x.y` (IP visible dans l'app Hue → Paramètres → Mon
système Hue → bridge → ⓘ, ou dans le routeur). `-https` pour passer en TLS (certificat auto-signé
accepté). `-target` change le nom cible (défaut `BuzzHue1`) — ne le faire que sur une ampoule dédiée.

## Ce que le spike mesure

- **Découverte** : mDNS et SSDP fonctionnent-ils sur ce réseau, ou faut-il une IP fixe ?
- **Latence** d'une commande (aller-retour HTTP complet, incluant la relecture de sécurité avant
  l'écriture — comptée à part dans le JSON via `latency` = écriture seule).
- **Fiabilité** : `bench` enchaîne N changements (couleur / luminosité / on-off) toutes les
  `-interval` et donne le taux de succès et p50/p95 — le critère pour un flash « bonne réponse »
  synchronisé avec les LED des buzzers.
- **Dépendances** : stdlib + `grandcat/zeroconf` (déjà dans `server-go/go.mod`). Rien d'autre.

`-transition 0` (défaut) demande un changement instantané ; la valeur par défaut du bridge est
4 (400 ms), à mesurer aussi si le rendu instantané paraît dur.
