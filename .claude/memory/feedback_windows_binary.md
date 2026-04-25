---
name: Fournir binaire Windows avant validation utilisateur
description: Toujours builder et fournir le binaire Windows AVANT de demander à l'utilisateur de tester en QUALIF
type: feedback
---

L'utilisateur teste TOUJOURS depuis Windows. Avant de demander une validation visuelle, il faut obligatoirement :

1. Builder le binaire Windows : `GOOS=windows GOARCH=amd64 /usr/local/go/bin/go build -ldflags="-s -w" -o buzzcontrol-vX.Y.Z-windows-amd64.exe ./cmd/server/`
2. Fournir le chemin du fichier dans `server-go/`
3. Mentionner la version du firmware embarqué (ou préciser qu'il n'a pas changé depuis vX.Y.Z)

**Why:** L'utilisateur ne peut pas tester le serveur Linux — il utilise le .exe Windows. Sans ce binaire, il ne peut pas valider la QUALIF.

**How to apply:** Systématiquement en fin de DEPLOY QUALIF, avant tout message "vérifie sur :8080". Séquence : build Linux → build Windows → redémarrer QUALIF Linux → fournir .exe Windows → demander validation.
