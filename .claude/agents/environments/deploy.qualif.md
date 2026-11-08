---
name: deploy.qualif
description: "Adaptations projet BuzzControl pour DEPLOY QUALIF. Base generique : deploy.qualif.template.md."
---

# Deploy QUALIF — Adaptations BuzzControl

> **Base** : Voir `deploy.qualif.template.md` pour la structure generique (mecanisme `vps` —
> non applicable tel quel, il suppose un hote distant via ssh/scp). **BuzzControl n'a pas de
> serveur QUALIF distant** : le binaire Windows publie tourne en local, sur le meme poste que
> l'agent, pas de ssh, pas de systemctl. Ce fichier remplace entierement les etapes [2. Install]
> et [3. Verification post-deploy] generiques.

Aucun build ni publication ici — le binaire deja produit par PUBLISH QUALIF
(`$QUALIF_DIR/buzzcontrol-qualif-*.exe`) est celui qu'on lance et teste tel quel.

> **Conteneur Claude-Launcher (v11.5.0, #292)** : `deployer` tourne sous Linux, sans `powershell.exe`
> ni `/mnt/c` — il **ne lance plus** l'exe Windows (`… .exe &` est impossible). L'exe est lance par
> **l'utilisateur, depuis Windows**, via le partage (`build/qualif_v<X.Y.Z>/` du depot). `deployer`
> verifie la presence de l'exe, transmet au teamleader son chemin et la commande de lancement
> (`--port 9090`) ; les smoke tests ci-dessous sont executes par l'utilisateur (ou par `deployer`
> seulement si le port 9090 du poste Windows est joignable depuis le conteneur — sinon, ne pas
> pretendre les avoir passes). `access.verified` de QUALIF = `failed` sur `local-shell` tant que
> ce mode dure.

## Variables attendues

Aucune — pas de ssh, pas de registre, pas de credentials.

### Smoke tests QUALIF BuzzControl

Le serveur QUALIF est lancé sur le **port 9090** pour éviter toute interférence avec un serveur
de production tournant sur le port 80.

```bash
REPO_ROOT=$(git rev-parse --show-toplevel)
MILESTONE_VERSION=$(grep '"version"' server-go/config.json | sed 's/.*"\([0-9]*\.[0-9]*\.[0-9]*\)\..*/\1/')
FULL_VERSION=$(grep '"version"' server-go/config.json | sed 's/.*"\([0-9.]*\)".*/\1/')
QUALIF_DIR="$REPO_ROOT/build/qualif_v${MILESTONE_VERSION}"

test -f "$QUALIF_DIR/buzzcontrol-qualif-${FULL_VERSION}-windows-amd64.exe" || {
  echo "Aucune publication QUALIF trouvee — executer /publish qualif d'abord"; exit 1;
}

sha256sum "$QUALIF_DIR/buzzcontrol-qualif-${FULL_VERSION}-windows-amd64.exe"

# Lancement : PAR L'UTILISATEUR, depuis Windows (PowerShell, dossier build\qualif_v<X.Y.Z>\ du depot) :
#   .\buzzcontrol-qualif-<FULL_VERSION>-windows-amd64.exe --port 9090
# deployer ne lance rien sous Linux. Smoke tests (utilisateur, navigateur ou curl Windows ;
# deployer uniquement si BASE est joignable depuis le conteneur) :
BASE="${QUALIF_BASE_URL:-http://localhost:9090}"
curl -sf $BASE/version               # version (= $FULL_VERSION, serveur ET firmware)
curl -sf $BASE/                      # page principale
curl -sf $BASE/api/firmware/buzzclick/version  # firmware endpoint
curl -sf $BASE/questions             # questions
curl -sf $BASE/listGame              # liste des jeux
curl -sf $BASE/tv                    # affichage TV

# Arret : par l'utilisateur (Ctrl+C dans la console Windows, ou curl $BASE/shutdown)

echo "QUALIF prete - $FULL_VERSION (lancement et smoke tests cote utilisateur)"
```

## Echec

Pas de ssh/journalctl — un echec ici est un echec de lancement local ou de smoke test :

```
SendMessage({
  to: "team-lead",
  content: "DEPLOY FAILED
Environnement : QUALIF
Version  : v[X.Y.Z.a]
Etape    : [Lancement | Smoke test <endpoint>]"
})
```

## Rollback

Rien a rollback infra (pas de service systemd, pas de registre) — relancer l'ancien binaire
QUALIF si besoin (conserve dans `build/qualif_v<version precedente>/`), ou corriger et refaire
`/publish qualif` + `/deploy qualif`.
