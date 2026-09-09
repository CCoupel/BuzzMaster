# Procédure de Test — Pont Philips Hue : pilote et configuration (#206/#207, milestone v10.0.0)

**Version** : v10.0.0.x (QUALIF)
**Date** : 2026-09-04
**Testeur** : **Utilisateur** (voir note ci-dessous)
**Contrat** : `contracts/hue-bridge.md`
**Contrat associé** : `contracts/lighting.md` (#205, livré)
**Maquette** : `docs/mockups/lighting-hue-config-207.html` (révision 4, validée)

## ⚠️ Cette procédure est destinée à l'utilisateur, pas à `qa`

Toutes les scénarios ci-dessous exigent un **vrai pont Philips Hue** sur le réseau
et de **vraies ampoules**. Les sessions d'agents n'ont pas de navigateur fiable
ni de matériel Hue — `qa` ne peut valider que la suite automatisée (voir
`_work/reports/test-writer-issues206-207-<timestamp>.md` pour la liste des
commandes `go test`/`vitest`). Cette procédure est écrite pour quelqu'un qui a
le pont **devant lui**.

## Prérequis

- [ ] Un pont Philips Hue (Bridge v2, BSB002) allumé et sur le **même réseau**
      que le serveur BuzzMaster
- [ ] Au moins 2 ampoules Hue associées au pont, dont une accessible physiquement
      (pour le scénario « éteinte au mur »)
- [ ] Accès physique au bouton rond du pont
- [ ] Binaire QUALIF buildé depuis `milestone/v10.0.0`

## Scénarios

### Scénario 1 — Association par appui bouton (bout en bout, matériel réel)

**Objectif** : vérifier le parcours complet de découverte + association décrit
par la maquette, avec un vrai pont.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | Menu 🐝 → « Ambiance » | Page `/admin/ambiance`, badge « Non configuré » (ampoule grise, contour nu) | | |
| 2 | Cliquer « Rechercher un pont » | Le pont réel apparaît en **moins de quelques secondes** (mDNS ~0,2 s mesuré au spike), adresse + identifiant affichés | | |
| 3 | Cliquer « Associer ce pont » **sans appuyer sur le bouton du pont** | Attente en ligne (pas de modale), anneau + décompte depuis 45 s, message « Appuyez sur le bouton rond au centre du pont » | | |
| 4 | Appuyer sur le bouton rond du pont | Dans les 2 secondes suivantes : toast « Pont associé. », passage à l'étape 3 (liste des ampoules) | | |
| 5 | Observer le badge | « Pont connecté » (ampoule verte, pleine, rayons), compteur d'ampoules jointes | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 2 — Bouton « Tester » allume réellement l'ampoule

**Objectif** : vérifier l'effet physique du test, pas seulement la réponse HTTP.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | Étape 3, cliquer « Tester » sur une ampoule précise | L'ampoule **physique** flashe brièvement en blanc puis **revient exactement à son état d'avant** (couleur/luminosité/on-off inchangés) | | |
| 2 | Cliquer « Tester toutes les ampoules » | Toutes les ampoules sélectionnées flashent, chacune revient à son propre état antérieur | | |
| 3 | Lancer un test pendant qu'une partie tourne (buzz en cours) | La lumière de la salle (pilotée par le jeu) revient à la scène courante juste après le flash, sans rester bloquée sur blanc | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 3 — Pont débranché en cours de partie

**Objectif** : vérifier le comportement dégradé du contrat §5.5 sur un vrai pont.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | Lancer une partie normale avec l'éclairage configuré et actif | Buzzers ET salle réagissent aux événements (buzz, révélation, points) | | |
| 2 | **Débrancher physiquement le pont** en cours de partie | **Aucune latence perceptible** sur les transitions de jeu (buzz, révélation) — les buzzers continuent de réagir normalement | | |
| 3 | Consulter les logs serveur pendant les ~30 secondes suivantes | **Une seule** ligne mentionnant le pont Hue passé « injoignable » — jamais une ligne par tentative | | |
| 4 | Rouvrir `/admin/ambiance` | Badge « Pont injoignable », dernière sélection d'ampoules affichée et grisée, **aucune perte de configuration** | | |
| 5 | Rebrancher le pont | Dans un délai raisonnable (retrait exponentiel plafonné à 60 s), le badge repasse à « Pont connecté » et la salle recommence à réagir | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 4 — Une ampoule éteinte au mur n'affecte pas les autres

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | Éteindre une ampoule Hue à l'**interrupteur mural** (pas depuis l'app Hue) | Sur `/admin/ambiance`, la ligne de cette ampoule indique « éteinte au mur », bouton Tester inactif pour elle | | |
| 2 | Lancer un événement de jeu (buzz, révélation) | Les **autres** ampoules réagissent normalement ; aucun message d'erreur global, aucune latence | | |
| 3 | Rallumer l'ampoule au mur | Elle rejoint le pilotage normal au prochain événement (pas besoin de relancer le serveur) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 5 — Les trois couleurs de l'ampoule du menu, observées en vrai

**Objectif** : confirmer visuellement (pas seulement via les tests automatisés
`Navbar.ambiance.test.jsx`) que les trois glyphes/couleurs sont bien
**distinguables à l'œil** dans le menu réel, à taille réelle.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | Pont non configuré, ouvrir le menu 🐝 | Ampoule **grise**, contour nu, sans pastille | | |
| 2 | Pont connecté et fonctionnel | Ampoule **verte**, pleine, avec rayons | | |
| 3 | Débrancher le pont (ou révoquer la clé depuis l'app Hue officielle) | Ampoule **orange**, contour + pastille d'alerte | | |
| 4 | Survoler l'entrée « Ambiance » (ou lecteur d'écran) | Info-bulle/`title` dit l'état en toutes lettres (« Éclairage : pont connecté », etc.) | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Critères de Validation

- [ ] Scénario 1 (association bout en bout) : PASS
- [ ] Scénario 2 (test réel) : PASS
- [ ] Scénario 3 (débranchement en partie) : PASS
- [ ] Scénario 4 (ampoule éteinte au mur) : PASS
- [ ] Scénario 5 (trois couleurs du menu) : PASS
- [ ] Aucune régression sur une partie QCM/MEMORY/MEMOTION/RAFALE sans éclairage configuré

## Mesures chiffrées du contrat §8 — où les consigner

`dev-backend` publie latence p95, étalement (N=2/4/6 ampoules) et tenue en
rafale via `internal/lighting/hue/bench_real_test.go` (`TestRealBridgeLatencyAndSpread`,
variables d'environnement `HUE_BRIDGE_IP`/`HUE_API_KEY`/`HUE_LIGHTS`/`HUE_BENCH_OUT`)
contre un **vrai pont**. Ce test :

- échoue lui-même (`t.Errorf`) si p95 dépasse 150 ms par ampoule — c'est le
  verdict écrit automatique du contrat §8 ;
- écrit un JSON (`HUE_BENCH_OUT`) à archiver dans le rapport QUALIF (ex:
  `_work/reports/qa-hue-bench-<date>.json`) — **cette étape manuelle reste à
  faire par `deployer`/QA** : lancer la commande ci-dessous une fois par
  QUALIF avec le pont réel branché, et joindre le fichier produit au rapport.

```bash
cd server-go
HUE_BRIDGE_IP=192.168.1.101 HUE_API_KEY=<clé> HUE_LIGHTS=BuzzHue1,BuzzHue2 \
  HUE_BENCH_OUT=/tmp/hue-bench.json \
  go test ./internal/lighting/hue -run TestRealBridgeLatencyAndSpread -v
```

**Seuil de rafale (§8)** : « 100 % de succès et ≤ 10 écritures/s » n'est pas
mesuré par ce test-là (il mesure latence/étalement, pas la rafale RAFALE) —
si `dev-backend` n'a pas encore ajouté ce second scénario chiffré, le signaler
au CDP avant de clore #206.

## Notes QA/CDP

- Cette procédure ne remplace pas la suite automatisée (`go test ./internal/lighting/...`,
  `go test ./internal/server/... -run LightingConfig`, `npx vitest run src/pages/AmbiancePage.test.jsx
  src/components/Navbar.ambiance.test.jsx src/components/LightingBulbIcon.test.jsx
  src/hooks/useLightingStatus.test.js src/utils/lightingState.test.js`) — elle
  couvre uniquement ce qu'un vrai pont peut seul prouver.
- Règle projet : fermeture d'issue = validation manuelle explicite de
  l'utilisateur sur cette procédure, jamais automatique au passage QUALIF.
