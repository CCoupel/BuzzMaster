//go:build !linux && !windows

package audio

import "log"

// newPlatformMediaPlayer sur tout GOOS autre que linux/windows renvoie un
// neutralMediaPlayer inoffensif — exactement le même partage de plateforme
// que newPlatformOutput (output_other.go), pour la même raison : aucun
// backend `oto` n'est construit ici, seule la bibliothèque standard.
func newPlatformMediaPlayer(cfg OutputConfig, onNaturalEnd func()) MediaPlayer {
	log.Printf("audio: no sound backend built for this platform — question sound media is a no-op here (contract sound.md §1.1: Windows + Linux/Raspberry Pi only)")
	return neutralMediaPlayer{}
}
