package assets

import "embed"

//go:embed demo/*
var DemoAssets embed.FS

//go:embed firmware
var FirmwareAssets embed.FS

//go:embed default-question-image.svg
var DefaultQuestionImage []byte

// EffectsAssets holds the native light effects (v11.7.0, #223 — contracts/
// light-effects.md §3.2): read from the binary, never from disk.
//
//go:embed effects/*.yaml
var EffectsAssets embed.FS
