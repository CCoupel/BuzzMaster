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

# Depuis la racine du projet — lancer le binaire QUALIF sur port 9090 (flag --port, v5.1.3+)
"$QUALIF_DIR/buzzcontrol-qualif-${FULL_VERSION}-windows-amd64.exe" --port 9090 &
QUALIF_PID=$!
sleep 2  # attendre démarrage

BASE="http://localhost:9090"

# Smoke tests
curl -sf $BASE/version               # version
curl -sf $BASE/                      # page principale
curl -sf $BASE/api/firmware/buzzclick/version  # firmware endpoint
curl -sf $BASE/questions             # questions
curl -sf $BASE/listGame              # liste des jeux
curl -sf $BASE/tv                    # affichage TV

# Arrêt propre
kill $QUALIF_PID

echo "Deploiement QUALIF termine - $FULL_VERSION"
```

## Echec

Pas de ssh/journalctl — un echec ici est un echec de lancement local ou de smoke test :

```
SendMessage({
  to: "main",
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
