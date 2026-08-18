# Procédure de Test — Communication Animateur (#167 + #168 + #175 + #176)

**Version** : v6.4.x (branche `feature/anim-communication`)
**Date** : 2026-08-18 (révisée pour #176)
**Testeur** : QA / Utilisateur
**Issues** : #167 (messagerie régie → tablettes animateur) + #168 (note d'explication par question) +
#175 (menu « Quitter » — arrêt du serveur) + #176 (correctifs UX de #167 : champ régie permanent,
double-tap remplace le bouton « Vu »)
**Référence** : Plan `_work/reports/plan-20260818-121500.md` (#167/#168), `_work/reports/plan-20260818-140953.md`
(#175), `_work/reports/plan-20260818-141638.md` (#176), maquette `docs/mockups/anim-communication-167-168.html`
(**republiée pour #176** — vérifier qu'elle montre bien le champ permanent et le double-tap, pas
l'ancien bouton « Vu »/« Nouveau message »), `contracts/websocket-actions.md` §"Messagerie régie",
`contracts/models.md` §EXPLANATION, `contracts/http-endpoints.md` §`/shutdown`

> ⚠️ **#176 change l'interface décrite dans les scénarios 1/3/4/5/6 ci-dessous** (rédigés pour #167,
> révisés ici) : le bouton « Vu » côté `/anim` est remplacé par un **double-tap sur toute la zone du
> message** ; côté régie, le champ de saisie est désormais **toujours visible et pré-rempli**, et le
> bouton « Nouveau message » a disparu au profit d'un indicateur fugace. Les scénarios ont été mis à
> jour en conséquence — ne pas chercher ces boutons disparus lors de la recette.

---

## Prérequis

- [ ] Environnement : QUALIF ou LOCAL
- [ ] Trois postes/onglets ouverts au minimum : **deux** `/admin/*` (deux sessions régie distinctes,
      ex. deux navigateurs ou une fenêtre privée), **une** `/anim` (tablette animateur) — idéalement
      une **seconde** `/anim` pour le Scénario 1 (acquittement croisé)
- [ ] Un quiz contenant au moins une question avec une note d'explication renseignée et une question
      sans note
- [ ] Suite automatisée exécutée et verte (voir section Non-Régression en fin de document) avant de
      démarrer la validation manuelle

---

## Scénario 1 — Deux tablettes simultanées, acquittement croisé (AC2, AC3, D3)

**Objectif** : Vérifier qu'un message envoyé par la régie atteint toutes les tablettes connectées, et
qu'un acquittement depuis N'IMPORTE LAQUELLE efface le message partout — un message unique, un
acquittement unique, jamais de comptage par tablette.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir deux tablettes `/anim` (A et B) côte à côte | Les deux affichent « Aucun message de la régie » | | |
| 2 | Depuis `/admin`, taper une consigne courte et attendre l'envoi automatique (voir Scénario 3 pour le détail des déclencheurs) | La consigne apparaît sur A **et** B, avec un indice « Double-tap pour marquer comme vu » sur chacune (#176 — **plus de bouton « Vu »**) | | |
| 3 | **Double-tapper** (deux appuis rapprochés) sur la zone du message depuis la tablette A **uniquement** | Le message disparaît immédiatement sur A **et** sur B (pas seulement A) | | |
| 4 | Observer la régie | Un indicateur fugace « Vu par l'animateur » apparaît **à côté** du champ de saisie, qui reste visible et vide (#176 — plus d'état bloquant) | | |
| 5 | Répéter l'envoi, puis double-tapper depuis B cette fois | Même résultat croisé : disparition sur A et B simultanément | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Mise en veille puis reconnexion d'une tablette (AC6, D6)

**Objectif** : Vérifier qu'une tablette qui se reconnecte alors qu'un message est actif le reçoit —
livraison différée, jamais perdue.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur une tablette `/anim`, couper le Wi-Fi (ou fermer l'onglet) | Déconnexion observée (indicateur de statut) | | |
| 2 | Depuis `/admin`, envoyer une consigne pendant que la tablette est hors ligne | Aucune erreur côté régie ; le bandeau régie affiche la consigne comme active | | |
| 3 | Reconnecter la tablette (Wi-Fi rétabli ou onglet rouvert sur `/anim`) | La consigne apparaît **immédiatement** à la reconnexion, sans action supplémentaire — pas d'attente d'un prochain événement de jeu | | |
| 4 | Double-tapper depuis cette même tablette | Le message s'efface partout comme au Scénario 1 | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Envoi automatique, 140 caractères accentués (AC1c, AC9, AC13)

**Objectif** : Vérifier les trois déclencheurs d'envoi (aucun bouton « Envoyer »), la troncature
serveur à 140 caractères sur du texte accentué, et l'affichage intégral côté tablette.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur `/admin`, observer le bandeau du bas de l'écran | Un champ de saisie et un compteur (140), **aucun bouton « Envoyer »** nulle part sur la page | | |
| 2 | Taper une courte consigne puis appuyer sur **Entrée** | Envoi immédiat, la tablette affiche la consigne sans délai perceptible | | |
| 3 | Effacer, taper une nouvelle consigne puis cliquer **ailleurs sur la page** (perte de focus) | Envoi déclenché par le blur, même résultat | | |
| 4 | Effacer, taper une nouvelle consigne et **ne rien faire** pendant ~2 secondes | Envoi automatique après la pause de frappe, sans Entrée ni clic | | |
| 5 | Composer un texte de plus de 140 caractères, **entièrement accentué** (é/è/à/ç, ex. copier-coller un paragraphe français riche en accents) et l'envoyer | Le texte reçu sur la tablette est tronqué à 140 caractères **exactement**, lisible, **sans caractère coupé/corrompu en fin de texte** (pas de `�` ni de glyphe cassé) | | |
| 6 | Observer la bande `/anim` avec ce message de 140 caractères | Le texte tient **en entier** dans la bande (elle s'agrandit verticalement si besoin), rien n'est coupé visuellement, la zone reste double-tapable sur toute sa surface (#176), et le bloc « à suivre » n'est pas repoussé hors écran | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Résurrection après acquittement (AC1d — cas limite critique)

**Objectif** : Reproduire précisément le risque documenté au plan : une pause de frappe suivie d'un
blur sur un texte **identique**, après acquittement, ne doit **jamais** faire réapparaître le message.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur `/admin`, taper une consigne et attendre la pause de frappe (2s) pour déclencher l'envoi | La consigne est active, visible sur `/anim`, le champ régie affiche le même texte (pré-rempli, #176) | | |
| 2 | Depuis `/anim`, **double-tapper** sur la zone du message | Le champ régie se vide **automatiquement** (sans clic), un indicateur fugace « Vu par l'animateur » apparaît brièvement à côté | | |
| 3 | Sans rien retaper, **cliquer dans le champ régie désormais vide puis ailleurs sur la page** (déclenche un blur sur un champ vide) | **Aucun message ne réapparaît** sur les tablettes ni en régie (un texte vide n'envoie rien) | | |
| 4 | Vérifier que le champ régie est bien vide, sans résidu de l'ancien texte | Le champ a bien été vidé automatiquement à l'acquittement | | |
| 5 | Taper le texte **identique** à l'ancienne consigne (celle déjà acquittée) et l'envoyer | Le message réapparaît normalement — ce n'est plus la même "instance" de message pour le serveur (le champ a été vidé, ce n'est pas un blur résiduel sur l'ancien texte) ; **c'est le scénario 4bis ci-dessous qui teste la vraie garde de résurrection, plus difficile à déclencher accidentellement depuis #176** | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4bis — Résurrection, variante #176 : re-préremplissage puis blur immédiat

**Objectif** : #176 rend la résurrection accidentelle plus difficile (le champ se vide tout seul), mais
la garde serveur doit rester active pour le cas où le champ est encore synchronisé au moment de
l'acquittement (ex. re-render juste avant le clear, ou un deuxième poste régie qui n'a pas eu le temps
de voir le champ se vider).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir **deux** sessions `/admin`. Depuis le poste 1, envoyer une consigne | Les deux postes affichent le champ pré-rempli avec la consigne | | |
| 2 | Depuis `/anim`, double-tapper immédiatement (avant que le poste 2 n'ait eu le temps de réagir à l'écran) | Le message est acquitté | | |
| 3 | Observer le poste 2 juste après | Son champ se vide également (dès que son propre état WebSocket reçoit l'effacement) — **si un blur survient sur ce poste entre l'acquittement et le vidage affiché**, aucune résurrection ne doit se produire malgré la fenêtre de course | | |

**Verdict** : [ ] PASS  [ ] FAIL

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Retrait régie (AC4, AC5, D4)

**Objectif** : Vérifier que la régie peut retirer son propre message avant tout acquittement animateur,
avec un statut distinct de l'acquittement.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Depuis `/admin`, envoyer une consigne (erreur de frappe, par exemple) | Message actif sur `/admin` et `/anim` | | |
| 2 | Depuis `/admin` (PAS `/anim`), cliquer sur « Effacer » | Le message disparaît immédiatement sur `/admin` **et** `/anim` | | |
| 3 | Observer l'état régie après ce retrait | Le champ (toujours visible, #176) repasse à vide, **AUCUN** indicateur « Vu par l'animateur » n'apparaît — distinction `CLEARED_BY` REGIE vs ANIM préservée malgré le changement d'interface | | |
| 4 | Observer `/anim` après ce retrait | La bande affiche « Aucun message de la régie », aucune trace de l'ancien texte | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 6 — Synchronisation multi-régie (AC1e, F3b)

**Objectif** : Vérifier que deux sessions `/admin` ouvertes simultanément restent synchronisées sur
l'état du message, sans état local optimiste divergent.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir deux sessions `/admin` (poste 1 et poste 2), les placer côte à côte | Les deux affichent le champ de saisie vide | | |
| 2 | Depuis le poste 1, taper et envoyer une consigne | Le poste 2 affiche **immédiatement** son champ **pré-rempli** avec la consigne (#176, AC2/AC3) et le bouton « Effacer », sans avoir rien tapé lui-même | | |
| 3 | Depuis le poste 2, cliquer sur « Effacer » | Le message disparaît sur les deux postes, les deux champs se vident | | |
| 4 | Renvoyer une consigne depuis le poste 1, puis double-tapper depuis une tablette `/anim` | Les deux postes régie affichent simultanément l'indicateur fugace « Vu par l'animateur », et leurs champs se vident automatiquement en même temps | | |
| 5 | Sur le poste 1, **pendant que le poste 2 a encore l'indicateur fugace affiché**, commencer à taper une nouvelle consigne | Le poste 1 peut taper librement sans être perturbé par l'état d'affichage du poste 2 — les deux postes évoluent indépendamment une fois le champ vidé | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 7 — Enchaînement de questions avec message actif (AC12, D5)

**Objectif** : Vérifier qu'aucune transition de jeu n'efface automatiquement un message régie non
acquitté — canal orthogonal au déroulé de la partie.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Envoyer une consigne régie, la laisser **non acquittée** | Message actif sur `/anim` | | |
| 2 | Enchaîner normalement : LANCER → STOP → RÉVÉLER → question suivante (READY) | À chaque étape, le message régie reste affiché sur `/anim`, sans interruption ni clignotement | | |
| 3 | Effectuer un RAZ (si accessible en environnement de test) | Le message régie reste actif malgré le RAZ — comportement voulu, pas une régression | | |
| 4 | Acquitter enfin le message | Efface normalement, comme dans les scénarios précédents | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 8 — Emplacement du bandeau régie (AC1, AC1a, AC1b)

**Objectif** : Vérifier la portée du bandeau (présent sur tout `/admin/*`, absent ailleurs) et son
absence de recouvrement sur les pages longues.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Naviguer sur plusieurs pages `/admin/*` (jeu, équipes, questions, logs, scores) | Le bandeau régie est présent **en bas de l'écran, pleine largeur**, sur **toutes** ces pages | | |
| 2 | Naviguer sur `/anim` | Le bandeau régie (bas d'écran, saisie) est **absent** — seule la bande de réception (Scénario 1-4) est présente, à un autre endroit de la page | | |
| 3 | Naviguer sur `/tv` | Aucune trace du bandeau régie ni de la bande de réception | | |
| 4 | Naviguer sur `/player` (VJoueur) | Aucune trace non plus | | |
| 5 | Sur `/admin/quiz` (page longue, liste de questions) avec de nombreuses questions listées | Le bandeau du bas ne recouvre **aucun** contenu — le bas de la liste reste consultable en défilant | | |
| 6 | Sur `/admin/logs` (page longue) | Même vérification — aucun recouvrement | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 9 — Note d'explication : édition, persistance, réédition (#168, AC14, AC15, AC20)

**Objectif** : Vérifier que la note survit à une réédition de la question (piège `handleUploadQuestion`)
et que la vider l'efface correctement.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir `/admin/quiz`, créer ou éditer une question, renseigner « Note d'explication (animateur seul) » | Le champ accepte un texte long, sans limite visible | | |
| 2 | Enregistrer | Question sauvegardée sans erreur | | |
| 3 | Rouvrir la MÊME question en édition | La note est toujours présente, identique | | |
| 4 | Modifier **uniquement** le texte de la question (pas la note), enregistrer | Rouvrir à nouveau : la note **est toujours là**, inchangée (c'est le piège du plan — une régression ferait disparaître la note à cette étape précisément) | | |
| 5 | Vider complètement le champ note, enregistrer, rouvrir | La note est bien effacée (champ vide, pas de résidu) | | |
| 6 | Ouvrir une question qui n'a **jamais** eu de note | Le champ note est vide, aucune erreur | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 10 — Note d'explication sur `/anim` (#168, AC16, AC17, AC18, AC19)

**Objectif** : Vérifier l'affichage floutée/révélée par pression, l'emplacement au repos, et
l'absence totale ailleurs.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Charger sur `/anim` une question **avec** note, avant révélation de la réponse | La zone note (sous la zone réponse) est floutée, libellé « Note — maintenir pour lire » | | |
| 2 | Maintenir un doigt/clic sur la zone note | Le texte devient lisible tant que la pression est maintenue | | |
| 3 | Relâcher | Zone à nouveau floutée | | |
| 4 | Amener la question en RÉVÉLÉ (RÉPONSE) | La note est visible **en permanence**, sans avoir besoin de maintenir quoi que ce soit | | |
| 5 | Charger une question **sans** note | L'emplacement affiche « Aucune note pour cette question » — jamais un blanc | | |
| 6 | Charger une question avec une note **très longue** (plusieurs paragraphes) | Le bloc central défile pour l'afficher, **sans** déplacer le reste de la mise en page (méta, chrono, bouton « à suivre ») | | |
| 7 | Observer `/tv`, `/player` et `/admin` pendant qu'une question avec note est active | La note n'apparaît **nulle part** sur ces trois surfaces | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 11 — Menu « Quitter », annulation sans effet (#175, AC1-AC4)

**Objectif** : Vérifier la présence, la position et le caractère non destructif d'une annulation.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur `/admin`, ouvrir le menu déroulant (logo 🐝) | Une entrée « Quitter » apparaît **en dernière position**, après « Logs », visuellement distincte (séparateur/teinte d'avertissement) des quatre entrées de navigation | | |
| 2 | Survoler l'entrée « Quitter » | Aucune barre d'état de navigateur n'affiche une URL de destination (ce n'est pas un lien) | | |
| 3 | Cliquer sur « Quitter » | Une confirmation apparaît, mentionnant explicitement que TV, joueurs et animateur seront déconnectés | | |
| 4 | **Annuler** la confirmation | Le menu se referme, **aucune déconnexion** ne se produit, la partie en cours continue normalement — vérifier notamment que `/tv` reste connecté | | |
| 5 | Rouvrir le menu, cliquer à nouveau sur « Quitter », annuler à nouveau | Comportement identique, reproductible | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 12 — Menu « Quitter », confirmation avec postes multiples connectés (#175, AC5, AC8, B1)

**Objectif** : Vérifier l'arrêt effectif pour tous les participants ET la libération immédiate du
port réseau (le vrai test de la correction B1 — c'est le symptôme rencontré à la dernière QUALIF
v6.2.0.35, où le port ne se libérait pas et une machine entière avait dû être redémarrée).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Connecter simultanément : un `/tv`, un `/player` (VJoueur), une tablette `/anim`, et deux sessions `/admin` | Les cinq postes affichent un statut connecté | | |
| 2 | Depuis une session `/admin`, ouvrir le menu, cliquer « Quitter », **confirmer** | Le menu se referme immédiatement (AC7) | | |
| 3 | Observer `/tv` | Perd la connexion ; n'entre pas dans une boucle de reconnexion silencieuse — un état explicite doit apparaître (AC8, si F3 retenue) plutôt qu'un badge "Déconnecté" figé sans explication | | |
| 4 | Observer `/player` | Perd la connexion, même constat | | |
| 5 | Observer la tablette `/anim` | Perd la connexion, même constat | | |
| 6 | Observer la **seconde** session `/admin` (celle qui n'a pas cliqué) | Perd elle aussi la connexion — l'arrêt est global, pas propre à la session qui a cliqué | | |
| 7 | Observer la session `/admin` qui a cliqué | Affiche un message clair indiquant que le serveur est arrêté (AC8, si F3 retenue), pas une page figée sans explication | | |
| 8 | **Vérification critique B1** — relancer immédiatement le serveur sur le même port (`./buzzcontrol.exe` ou équivalent, sans changer de port) | Le serveur redémarre **sans erreur "port already in use"**, sans délai d'attente ni redémarrage de la machine — c'est la preuve que `OnShutdown`/`a.stop()` a bien fermé le port proprement (contrat `http-endpoints.md` : arrêt "**proprement**") | | |
| 9 | Se reconnecter sur les cinq postes après le redémarrage | Tous se reconnectent normalement, comme après un redémarrage propre habituel | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 13 — Double-tap réel sur tablette tactile (#176, AC12-AC16)

**Objectif** : Le double-tap ne peut être validé qu'à la main, sur un vrai appareil tactile — c'est le
seul scénario du lot où l'automatisé (jsdom) ne peut rien prouver sur le comportement réel du
navigateur (zoom natif, latence tactile).

**Prérequis** : une tablette ou un smartphone réel connecté sur `/anim` (pas un émulateur desktop).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Envoyer une consigne depuis `/admin` | Le message apparaît sur la tablette avec l'indice « Double-tap pour marquer comme vu » | | |
| 2 | Faire un **tap unique** (un seul appui bref) sur la zone du message | **Rien ne se passe** — le message reste actif, aucun acquittement accidentel (AC13) | | |
| 3 | Faire un **double-tap** franc (deux appuis rapprochés, comme pour zoomer une photo) sur la zone du message | Le message s'efface (acquitté) — **ET** le navigateur **ne zoome PAS** sur la zone (AC16, `touch-action: manipulation`) | | |
| 4 | Répéter l'envoi, puis faire un appui **suivi d'un glissement du doigt** (comme pour faire défiler) sur la zone | Rien ne se passe — le geste n'est pas compté comme un tap (AC15), la page peut défiler normalement si applicable | | |
| 5 | Répéter l'envoi, faire un tap, **attendre plus d'une seconde**, puis faire un second tap | Rien ne se passe — les deux taps sont hors fenêtre, pas comptés comme un double-tap (AC14) | | |
| 6 | Vérifier l'accès clavier (tablette avec clavier externe ou test sur desktop) : **Tab** jusqu'à la zone du message, puis **Entrée** | Le message s'efface en **un seul appui** — le double-tap protège le doigt, pas le clavier (AC18) | | |
| 7 | Même vérification avec la touche **Espace** | Même résultat | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 14 — Brouillon régie préservé pendant un acquittement concurrent (#176, AC4, AC6)

**Objectif** : Le scénario exact de la décision ② du plan #176 — la régie compose une nouvelle
consigne PENDANT qu'un message précédent (le sien ou celui d'un collègue) est en train d'être
acquitté par l'animateur.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Envoyer une première consigne « A » depuis `/admin` | Le champ régie affiche « A » (pré-rempli) | | |
| 2 | **Sans envoyer**, cliquer dans le champ et remplacer le contenu par « B » (ne pas attendre les 2s de pause de frappe, ou annuler avant l'envoi) | Le champ affiche « B », en cours de frappe | | |
| 3 | PENDANT que « B » est en cours de composition, double-tapper le message « A » depuis `/anim` | « A » est acquitté sur les tablettes | | |
| 4 | Observer le champ régie immédiatement après | Le champ affiche **toujours « B »** — le brouillon n'a **PAS** été écrasé ni vidé par l'acquittement de « A » (AC4 course écho/saisie + AC6 brouillon divergent) | | |
| 5 | Terminer la frappe de « B » et l'envoyer (Entrée) | « B » devient le nouveau message actif normalement | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation Globale

- [ ] Acquittement croisé entre deux tablettes fonctionne, message unique (Scénario 1)
- [ ] Livraison différée à la reconnexion d'une tablette (Scénario 2)
- [ ] Trois déclencheurs d'envoi actifs, aucun bouton « Envoyer », troncature 140 runes propre sur
      texte accentué, affichage intégral sur la bande `/anim` (Scénario 3)
- [ ] **Aucune résurrection** d'un message acquitté sur un renvoi automatique du même texte (Scénario 4 —
      critère de non-régression le plus sensible du lot)
- [ ] Retrait régie distinct de l'acquittement animateur (`CLEARED_BY`) (Scénario 5)
- [ ] Deux sessions régie restent synchronisées sur l'état serveur, jamais sur un état local optimiste
      (Scénario 6)
- [ ] Aucune transition de jeu n'efface un message régie non acquitté (Scénario 7)
- [ ] Bandeau régie présent sur tout `/admin/*`, absent de `/anim`/`/tv`/`/player`, aucun recouvrement
      de contenu (Scénario 8)
- [ ] Note d'explication : survit à une réédition de la question, effacée si vidée explicitement
      (Scénario 9)
- [ ] Note d'explication : geste identique à la réponse, jamais rendue hors `/anim` (Scénario 10)
- [ ] Menu « Quitter » : annulation sans effet, entrée non-lien en dernière position (Scénario 11)
- [ ] Menu « Quitter » : confirmation déconnecte TOUS les postes (pas seulement le poste cliqueur),
      et surtout **le port est immédiatement réutilisable au redémarrage** (Scénario 12, B1)
- [ ] Double-tap réel sur tablette : tap unique sans effet, double-tap déclenche SANS zoom navigateur,
      glissement et fenêtre expirée ignorés, accès clavier intact (Scénario 13)
- [ ] Un brouillon régie en cours de composition survit à l'acquittement d'un AUTRE message
      (Scénario 14)

---

## Non-Régression (suite automatisée, à exécuter avant validation manuelle)

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `cd server-go && go build ./... && go test ./...` | Build OK, tous les tests PASS, y compris `regie_message_test.go` (T2-T4b, NOUVEAU), `inbound_allowlist_anim_test.go` (T1), `broadcast_anim_test.go`/`send_state_to_client_anim_test.go` (T5/T6), `messages_anim_test.go` (T7), `http_test.go` (T8), `main_test.go` (T5 #175, NOUVEAU — vérifie l'assignation d'`OnShutdown`) | | |
| 2 | `cd server-go/web && npm test` (suite Vitest complète) | Tous les tests PASS, y compris `RegieMessageBar.test.jsx`, `AnimExplanationNote.test.jsx` (NOUVEAUX #167/#168), `QuestionsPage.explanation.test.jsx`, `PlayerDisplay.explanation.test.jsx` (NOUVEAUX), `AnimPage.test.jsx` et `AnimConductPanel.test.jsx` (blocs réécrits), `Navbar.test.jsx` (bloc #175 additif), `useDoubleTap.test.js` (NOUVEAU #176), `RegieMessageBar.test.jsx` (blocs état réécrits pour #176 — champ permanent, course écho/saisie, indicateur fugace) | | |
| 3 | `AnimAnswerZone.test.jsx` passe **sans la moindre modification** | Preuve que l'extraction du geste de révélation en `useHoldToPeek` (F6) n'a rien changé au comportement #169 — `useDoubleTap` (#176) est un hook SÉPARÉ, ne le touche pas non plus | | |
| 4 | Manche QCM/SPEEDY/ARDOISE/MEMORY/MEMOTION sur `/anim` (hors #167/#168) | Aucune régression : conduite, crédit, colonne équipes inchangés | | |
| 5 | Les quatre entrées de navigation existantes du menu (Config/Backup/Mises à jour/Logs) | Comportement strictement inchangé (mêmes routes, même rendu) — #175 n'affecte que l'ajout de « Quitter » | | |
| 6 | Section "Envoi automatique" de `RegieMessageBar.test.jsx` (Entrée/blur/pause 2s) | PASSE **sans la moindre modification** — #176 ne touche pas ce mécanisme (garde-fou explicite du plan #176) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Notes QA

[Espace pour observations]
