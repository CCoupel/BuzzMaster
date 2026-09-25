package audio

// MediaPlayer joue un média sonore LONG attaché à une question (v11.1,
// #219), sur un chemin entièrement distinct de celui des cues (§5). Il ne
// partage avec le moteur de bruitages QUE le contexte audio du processus
// (§10.3, sharedOtoContext — output_oto.go).
//
// Contrairement à Output.Play (§4), AUCUNE méthode ici ne bloque : la
// lecture est asynchrone, et l'état est lisible à tout instant via State().
// C'est cette asymétrie qui rend possibles les contrôles de conduite
// (rejouer, pause, stop) que l'interface Output n'offre pas et n'a jamais
// eu à offrir.
//
// Interface normative — contracts/sound.md §10.2. Ne pas l'étendre : une
// notification de fin de lecture se règle au niveau du constructeur
// (NewMediaPlayer's onNaturalEnd), jamais par une méthode supplémentaire
// ici — voir le commentaire de NewMediaPlayer.
type MediaPlayer interface {
	// Play démarre la lecture du fichier désigné. VOIX UNIQUE : un Play
	// pendant une lecture en cours REMPLACE celle-ci (arrêt net), il ne
	// superpose jamais deux extraits.
	Play(path string) error
	Pause()
	Resume()
	Stop()
	State() MediaState
	Close() error
}

// MediaState est l'état de lecture diffusé par le serveur
// (GAME.QUESTION_SOUND_STATE, contracts/game-state.md — Batch 1). Ne porte
// jamais de position de lecture (contract §10.6 : "le chronomètre est le
// seul flux à cadence du projet").
type MediaState string

const (
	MediaIdle    MediaState = "IDLE"
	MediaPlaying MediaState = "PLAYING"
	MediaPaused  MediaState = "PAUSED"
)

// neutralMediaPlayer est la dégradation silencieuse de MediaPlayer —
// symétrique de noopOutput (output.go) : renvoyée quand la plateforme
// courante n'est pas ciblée (media_other.go) ou quand le contexte `oto`
// partagé n'a pas pu être construit (media_oto.go). Chaque méthode est un
// no-op réel : aucune goroutine, aucune erreur, rien à fermer — contract
// §10.2 "dégradation silencieuse identique à §5.5".
type neutralMediaPlayer struct{}

func (neutralMediaPlayer) Play(path string) error { return nil }
func (neutralMediaPlayer) Pause()                 {}
func (neutralMediaPlayer) Resume()                {}
func (neutralMediaPlayer) Stop()                  {}
func (neutralMediaPlayer) State() MediaState      { return MediaIdle }
func (neutralMediaPlayer) Close() error           { return nil }

// NewMediaPlayer construit le MediaPlayer réel de la plateforme (contract
// §10.2), adossé au contexte `oto` partagé (§10.3), ou un neutralMediaPlayer
// inoffensif — symétriquement à NewOutput (output.go) — sur toute
// plateforme non ciblée par ce lot, ou si le contexte `oto` partagé n'a pas
// pu être construit. Ne renvoie JAMAIS nil, jamais d'erreur dure : la
// dégradation silencieuse du contract §5.5 s'applique ici à l'identique
// (§10.2 le dit explicitement).
//
// onNaturalEnd, si non nil, est appelé depuis une goroutine de surveillance
// interne au plus une fois par appel à Play() — SI ET SEULEMENT SI la
// lecture atteint sa fin TOUTE SEULE (le fichier a été lu jusqu'au bout et
// le périphérique a fini de restituer). Il n'est JAMAIS appelé pour Pause,
// Stop, Close, ou un Play() ultérieur qui remplace la lecture en cours
// (contract §10.7 : « l'arrêt manuel » est la responsabilité de
// l'appelant — qui invoque Stop() sait déjà qu'il l'a fait). Appelé sans
// détenir aucun verrou interne au MediaPlayer.
func NewMediaPlayer(cfg OutputConfig, onNaturalEnd func()) MediaPlayer {
	return newPlatformMediaPlayer(cfg, onNaturalEnd)
}

// IsNeutralMedia rapporte si p est la dégradation silencieuse de
// MediaPlayer — symétrique de IsNeutral (output.go, contract §4 amendement
// #230) — ou nil.
func IsNeutralMedia(p MediaPlayer) bool {
	if p == nil {
		return true
	}
	_, ok := p.(neutralMediaPlayer)
	return ok
}
