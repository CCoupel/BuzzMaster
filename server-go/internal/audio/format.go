package audio

// Canonical audio format — contracts/sound.md §3, normative: WAV PCM
// 16-bit, 44100 Hz, stereo. A direct consequence of `oto`'s "one audio
// context per process" constraint (spike #226 verdict §2): every sound in
// the application MUST share this exact sample rate and channel count.
// Declared ONCE here — every consumer (the platform Output backends in
// this package, and #229's WAV generator) reuses these constants rather
// than repeating the literals, per §3's own requirement.
const (
	SampleRate    = 44100
	ChannelCount  = 2
	BitsPerSample = 16
)

// BytesPerSample is one sample's size in bytes, for one channel — 16-bit
// PCM, so 2 bytes.
const BytesPerSample = BitsPerSample / 8

// FrameSize is one interleaved audio frame's size in bytes (all channels,
// one sample each) — used to size silence buffers and validate PCM
// lengths without repeating the arithmetic at each call site.
const FrameSize = BytesPerSample * ChannelCount
