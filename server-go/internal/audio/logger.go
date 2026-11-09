package audio

import (
	"log"
	"sync/atomic"
)

// Logger est la signature du journal injecté dans ce paquet (#283 P1,
// contracts/http-endpoints.md « Journal de démarrage ») : le serveur y branche
// son logger (/ws/logs + fichier). Le niveau est porté par le premier argument.
type Logger func(level LogLevel, format string, args ...any)

// LogLevel est le niveau d'un message du paquet audio.
type LogLevel int

const (
	LogInfo LogLevel = iota
	LogWarn
	LogError
)

var logger atomic.Pointer[Logger]

// SetLogger remplace le journal du paquet (nil ⇒ retour au défaut log.Printf).
// Sûr en accès concurrent.
func SetLogger(l Logger) {
	if l == nil {
		logger.Store(nil)
		return
	}
	logger.Store(&l)
}

// logf émet un message via le logger injecté, ou log.Printf par défaut.
func logf(level LogLevel, format string, args ...any) {
	if p := logger.Load(); p != nil {
		(*p)(level, format, args...)
		return
	}
	log.Printf(format, args...)
}

var outputReason atomic.Pointer[string]

func setOutputReason(r string) { outputReason.Store(&r) }

// OutputReason renvoie la cause lisible de la dégradation de la sortie
// (construction de NewOutput ayant produit un Output neutre), "" si aucune
// (#283 : même texte pour le journal de démarrage et GET /api/sound/status).
func OutputReason() string {
	if p := outputReason.Load(); p != nil {
		return *p
	}
	return ""
}
