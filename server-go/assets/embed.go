package assets

import "embed"

//go:embed demo/*
var DemoAssets embed.FS

//go:embed firmware
var FirmwareAssets embed.FS
