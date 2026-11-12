package config

import "encoding/json"

// Copy-on-write updates of the live config, for code that changes it after startup (tests mostly).
//
// The live *Config is read without locks from many goroutines (the ambiance writer, the effect
// reader, the sound engine, HTTP handlers), always through Get(). It must therefore never be
// written in place: a change is published as a NEW instance with SetInstance, exactly as
// POST /config.json does. These helpers do that.

// Mutate publishes a copy of the live config after applying f to it. f must ASSIGN fields (a new
// map, slice or value), never modify a map or slice of the copy in place — they are shared with the
// instance still being read.
func Mutate(f func(*Config)) {
	next := *Get()
	f(&next)
	SetInstance(&next)
}

// PatchJSON publishes a deep copy of the live config on which the JSON patch has been applied
// (json.Unmarshal merge semantics: only the keys present in the patch change).
func PatchJSON(patch []byte) error {
	cur, err := json.Marshal(Get())
	if err != nil {
		return err
	}
	var next Config
	if err := json.Unmarshal(cur, &next); err != nil {
		return err
	}
	if err := json.Unmarshal(patch, &next); err != nil {
		return err
	}
	SetInstance(&next)
	return nil
}
