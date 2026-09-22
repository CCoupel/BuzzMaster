//go:build linux || windows

// Backend oto du MediaPlayer (v11.1, #219, contracts/sound.md §10.2/§10.3),
// partagé par Linux et Windows pour la même raison que output_oto.go :
// l'API Go d'`oto` ne diffère pas entre ces deux plateformes.
package audio

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/ebitengine/oto/v3"
)

// mediaMonitorPoll cadence le sondage de player.IsPlaying() par la
// goroutine de surveillance (monitor, plus bas) — même technique que
// otoOutput.Play (output_oto.go), mais ici l'appelant n'attend jamais :
// seule cette goroutine patiente. Les deux chemins ne s'appellent jamais
// l'un l'autre (contract §10.5).
const mediaMonitorPoll = 5 * time.Millisecond

// otoMediaPlayer est le MediaPlayer réel partagé par Linux et Windows.
type otoMediaPlayer struct {
	ctx          *oto.Context
	onNaturalEnd func()

	mu         sync.Mutex
	file       *os.File
	player     *oto.Player // référence FORTE — voir le commentaire de monitor
	state      MediaState
	generation uint64 // invalide une goroutine monitor périmée après Stop/replace/Close
}

// newPlatformMediaPlayer construit le backend oto, ou dégrade vers
// neutralMediaPlayer si le contexte `oto` partagé (§10.3) n'est pas
// disponible — exactement la dégradation du contract §5.5 déjà appliquée à
// newOtoOutput (output_oto.go), sur le MÊME contexte partagé
// (sharedOtoContext) : quel que soit celui des deux chemins qui construit
// le premier, il paie le coût (borné) de construction, l'autre réutilise
// le résultat mis en cache.
func newPlatformMediaPlayer(cfg OutputConfig, onNaturalEnd func()) MediaPlayer {
	ctx, err := sharedOtoContext()
	if err != nil {
		log.Printf("audio: oto context unavailable — question sound media disabled (silent degradation, contracts/sound.md §10.2/§5.5): %v", err)
		return neutralMediaPlayer{}
	}
	return &otoMediaPlayer{ctx: ctx, onNaturalEnd: onNaturalEnd, state: MediaIdle}
}

// Play implémente MediaPlayer (contract §10.2) — VOIX UNIQUE : tout extrait
// en cours de lecture ou en pause est remplacé (arrêt net), jamais mixé.
// Diffusé depuis le disque via un io.ReadSeeker construit sur *os.File — la
// section [offset, offset+size) du chunk "data", localisée en ne lisant
// QUE ses en-têtes de chunks, jamais le fichier entier (contract §10.2 :
// « jamais un os.ReadFile »).
func (p *otoMediaPlayer) Play(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("question sound: cannot open %s: %w", path, err)
	}

	offset, size, err := wavDataSection(f)
	if err != nil {
		_ = f.Close()
		return fmt.Errorf("question sound: %s does not conform to the canonical format (contracts/sound.md §3/§10.4): %w", path, err)
	}
	section := io.NewSectionReader(f, offset, size)

	p.mu.Lock()
	p.stopLocked() // remplace, ne superpose jamais (§10.2 voix unique) ; bump generation, pas de onNaturalEnd
	gen := p.generation

	player := p.ctx.NewPlayer(section)
	player.Play()

	p.file = f
	p.player = player
	p.state = MediaPlaying
	p.mu.Unlock()

	go p.monitor(gen, player)
	return nil
}

// monitor est la « goroutine de surveillance signalant la fin naturelle
// par rappel » exigée par le plan (§7 tâche 3) : elle sonde IsPlaying() —
// la même technique qu'otoOutput.Play utilise pour bloquer (output_oto.go),
// mais ici l'appelant n'est jamais bloqué, seule cette goroutine patiente.
// gen épingle cette goroutine à l'appel Play() qui l'a lancée : si
// Stop/replace/Close a tourné entre-temps, generation ne correspond plus et
// cette goroutine se termine silencieusement SANS appeler onNaturalEnd
// (contract §10.7 : seule une vraie fin naturelle l'appelle, jamais un
// Stop que l'appelant connaît déjà).
//
// La référence FORTE à player, conservée en paramètre local ET dans
// p.player tant qu'elle n'est pas remplacée, est ce qui empêche le
// finalizer GC d'oto de le fermer pendant qu'il joue encore (contract
// §10.3, « piège de durée de vie » : oto documente « a player is closed
// when it becomes unreachable »). Cette goroutine conserve cette même
// référence pendant toute sa durée de vie.
func (p *otoMediaPlayer) monitor(gen uint64, player *oto.Player) {
	for {
		time.Sleep(mediaMonitorPoll)

		p.mu.Lock()
		if p.generation != gen {
			p.mu.Unlock()
			return // remplacé par Stop/Play/Close — pas une fin naturelle
		}
		paused := p.state == MediaPaused
		p.mu.Unlock()

		if paused {
			// Un lecteur en pause rapporte aussi IsPlaying()==false —
			// attendre Resume() ou Stop() sans le prendre pour une fin.
			continue
		}
		if player.IsPlaying() {
			continue
		}

		p.mu.Lock()
		if p.generation != gen {
			p.mu.Unlock()
			return
		}
		p.state = MediaIdle
		if p.player == player {
			_ = p.player.Close() // no-op documenté depuis oto v3.4 — la fermeture réelle passe par le finalizer GC une fois inatteignable (§10.3)
			p.player = nil
		}
		if p.file != nil {
			_ = p.file.Close()
			p.file = nil
		}
		p.mu.Unlock()

		if p.onNaturalEnd != nil {
			p.onNaturalEnd()
		}
		return
	}
}

func (p *otoMediaPlayer) Pause() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.player == nil || p.state != MediaPlaying {
		return
	}
	p.player.Pause()
	p.state = MediaPaused
}

func (p *otoMediaPlayer) Resume() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.player == nil || p.state != MediaPaused {
		return
	}
	p.player.Play()
	p.state = MediaPlaying
}

// Stop implémente MediaPlayer — no-op au repos. N'appelle jamais
// onNaturalEnd : l'appelant qui invoque Stop() sait déjà qu'il l'a fait
// (contract §10.7).
func (p *otoMediaPlayer) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopLocked()
}

// stopLocked libère le lecteur/fichier courant, s'il y en a un, SANS
// appeler onNaturalEnd et SANS prendre p.mu (l'appelant le détient déjà).
// Incrémente generation pour qu'une goroutine monitor en vol se termine
// silencieusement plutôt que de déclencher un rappel de fin naturelle
// erroné.
func (p *otoMediaPlayer) stopLocked() {
	p.generation++
	if p.player != nil {
		_ = p.player.Close() // no-op documenté depuis oto v3.4 — la fermeture réelle passe par le finalizer GC une fois inatteignable (§10.3)
		p.player = nil
	}
	if p.file != nil {
		_ = p.file.Close()
		p.file = nil
	}
	p.state = MediaIdle
}

func (p *otoMediaPlayer) State() MediaState {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state
}

// Close libère les ressources — idempotent, appelable même si Play n'a
// jamais réussi (même discipline qu'Output.Close, output_oto.go).
func (p *otoMediaPlayer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopLocked()
	return nil
}

// wavDataSection localise le chunk "data" d'un fichier RIFF/WAVE ouvert
// dans f, en validant son chunk "fmt " contre le format canonique
// (contract §3 — miroir exact de la validation d'extractCanonicalPCM,
// bank.go), SANS lire la charge utile du chunk data elle-même (jusqu'à
// 6 Mio) — contract §10.2 : « jamais un os.ReadFile ». Renvoie l'offset
// absolu et la taille du chunk data dans f, prêts pour
// io.NewSectionReader(f, offset, size).
//
// Duplique volontairement la marche des chunks d'extractCanonicalPCM
// plutôt que de la réutiliser : celle-ci opère sur un []byte déjà chargé
// en mémoire (l'upload, où c'est acceptable — task 4/ValidateQuestionSound
// la réutilise telle quelle), alors que la lecture doit rester un flux
// disque. Les deux parseurs partagent les mêmes constantes de format
// (format.go) et ne peuvent donc pas diverger sur ce qui est canonique.
func wavDataSection(f *os.File) (offset int64, size int64, err error) {
	var header [12]byte
	if _, err := io.ReadFull(f, header[:]); err != nil {
		return 0, 0, fmt.Errorf("cannot read RIFF header: %w", err)
	}
	if string(header[0:4]) != "RIFF" || string(header[8:12]) != "WAVE" {
		return 0, 0, fmt.Errorf("not a RIFF/WAVE file")
	}

	var (
		haveFmt                 bool
		channels, bitsPerSample uint16
		sampleRate              uint32
	)

	pos := int64(12)
	for {
		if _, err := f.Seek(pos, io.SeekStart); err != nil {
			return 0, 0, fmt.Errorf("cannot seek to chunk at offset %d: %w", pos, err)
		}
		var chunkHeader [8]byte
		if _, err := io.ReadFull(f, chunkHeader[:]); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break // plus de chunk — sortie normale de boucle, traitée comme "no data chunk" plus bas
			}
			return 0, 0, fmt.Errorf("cannot read chunk header at offset %d: %w", pos, err)
		}
		id := string(chunkHeader[0:4])
		chunkSize := int64(binary.LittleEndian.Uint32(chunkHeader[4:8]))
		body := pos + 8

		switch id {
		case "fmt ":
			if chunkSize < 16 {
				return 0, 0, fmt.Errorf("fmt chunk too short (%d bytes)", chunkSize)
			}
			var fmtBody [16]byte
			if _, err := io.ReadFull(f, fmtBody[:]); err != nil {
				return 0, 0, fmt.Errorf("cannot read fmt chunk: %w", err)
			}
			format := binary.LittleEndian.Uint16(fmtBody[0:2])
			if format != 1 {
				return 0, 0, fmt.Errorf("unsupported WAV format tag %d (only PCM=1)", format)
			}
			channels = binary.LittleEndian.Uint16(fmtBody[2:4])
			sampleRate = binary.LittleEndian.Uint32(fmtBody[4:8])
			bitsPerSample = binary.LittleEndian.Uint16(fmtBody[14:16])
			haveFmt = true
		case "data":
			if !haveFmt {
				return 0, 0, fmt.Errorf("data chunk before fmt chunk")
			}
			if int(channels) != ChannelCount || sampleRate != uint32(SampleRate) || int(bitsPerSample) != BitsPerSample {
				return 0, 0, fmt.Errorf("non-canonical format: %d ch, %d Hz, %d-bit (expected %d ch, %d Hz, %d-bit)",
					channels, sampleRate, bitsPerSample, ChannelCount, SampleRate, BitsPerSample)
			}
			return body, chunkSize, nil
		}

		pos = body + chunkSize
		if chunkSize%2 == 1 {
			pos++
		}
	}

	return 0, 0, fmt.Errorf("no data chunk")
}
