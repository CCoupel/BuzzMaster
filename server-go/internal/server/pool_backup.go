package server

import (
	"archive/tar"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Pool list for selective backup / reset / restore (v13.1.0, #324,
// contracts/picto-pool.md §7). The four sites (handleBackupSelect,
// handleResetSelect, detectTARContents, handleRestore) walk this list instead
// of a hard-coded `rafale` flag. Key `rafale` (files/rafale,
// config/rafale_used.json) is unchanged; `picto` is new (files/picto,
// config/picto_used.json). An archive without files/picto restores exactly as
// before and creates no Pool Picto file.
type poolBackup struct {
	key       string // query-string / detected / result key
	dir       string // folder under files/ ; archive path "files/<dir>"
	usedFile  string // file under config/ ; archive path "config/<usedFile>"
	clearPool func(h *HTTPServer)
	clearUsed func(h *HTTPServer)
	reload    func(h *HTTPServer) (poolErr, usedErr error)
}

var poolBackups = []poolBackup{
	{
		key: "rafale", dir: "rafale", usedFile: "rafale_used.json",
		clearPool: func(h *HTTPServer) { h.engine.ClearRafaleReservoir() },
		clearUsed: func(h *HTTPServer) { h.engine.ClearRafaleUsed() },
		reload:    func(h *HTTPServer) (error, error) { return h.engine.LoadRafale(), h.engine.LoadRafaleUsed() },
	},
	{
		key: "picto", dir: "picto", usedFile: "picto_used.json",
		clearPool: func(h *HTTPServer) { h.engine.ClearPictoReservoir() },
		clearUsed: func(h *HTTPServer) { h.engine.ClearPictoUsed() },
		reload:    func(h *HTTPServer) (error, error) { return h.engine.LoadPicto(), h.engine.LoadPictoUsed() },
	},
}

// poolFlags reads one boolean query flag per pool (?rafale=true&picto=true).
func poolFlags(r *http.Request) map[string]bool {
	out := make(map[string]bool, len(poolBackups))
	for _, p := range poolBackups {
		out[p.key] = r.URL.Query().Get(p.key) == "true"
	}
	return out
}

func anyPoolFlag(flags map[string]bool) bool {
	for _, v := range flags {
		if v {
			return true
		}
	}
	return false
}

func setAllPoolFlags(flags map[string]bool) {
	for _, p := range poolBackups {
		flags[p.key] = true
	}
}

// poolFlagsLog renders the flags for the existing log lines ("rafale=…, picto=…").
func poolFlagsLog(flags map[string]bool) string {
	parts := make([]string, 0, len(poolBackups))
	for _, p := range poolBackups {
		parts = append(parts, p.key+"="+boolStr(flags[p.key]))
	}
	return strings.Join(parts, ", ")
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// backupPools adds the selected pools to the TAR: the question bank
// (files/<dir>/) and the "already used" flags (config/<usedFile>).
func (h *HTTPServer) backupPools(tw *tar.Writer, flags map[string]bool, filesDir, configDir string) {
	for _, p := range poolBackups {
		if !flags[p.key] {
			continue
		}
		dir := filepath.Join(filesDir, p.dir)
		if _, err := os.Stat(dir); err == nil {
			h.addDirToTAR(tw, dir, "files/"+p.dir)
		}
		used := filepath.Join(configDir, p.usedFile)
		if _, err := os.Stat(used); err == nil {
			h.addFileToTAR(tw, used, "config/"+p.usedFile)
		}
	}
}

// resetPools purges the selected pools: bank + used flags, files and memory.
func (h *HTTPServer) resetPools(flags map[string]bool, filesDir, configDir string, result map[string]bool) {
	for _, p := range poolBackups {
		if !flags[p.key] {
			continue
		}
		dir := filepath.Join(filesDir, p.dir)
		if err := os.RemoveAll(dir); err == nil {
			os.MkdirAll(dir, 0755)
			p.clearPool(h)
			result[p.key] = true
			log.Printf("[HTTP] Reset: %s reservoir cleared", p.key)
		}
		os.Remove(filepath.Join(configDir, p.usedFile))
		p.clearUsed(h)
		log.Printf("[HTTP] Reset: %s used-flags cleared", p.key)
	}
}

// restoreTarget maps an archive path to its restore target when it belongs to
// a detected pool.
func (h *HTTPServer) poolRestoreTarget(tarPath string, detected map[string]bool, configDir string) (target string, handled, allowed bool) {
	for _, p := range poolBackups {
		switch {
		case strings.HasPrefix(tarPath, "files/"+p.dir+"/"):
			if detected[p.key] {
				return filepath.Join(h.dataDir, tarPath), true, true
			}
			return "", true, false
		case tarPath == "config/"+p.usedFile:
			if detected[p.key] {
				return filepath.Join(configDir, p.usedFile), true, true
			}
			return "", true, false
		}
	}
	return "", false, false
}

// detectPoolPath flags the pool an archive path belongs to, if any.
func detectPoolPath(tarPath string, detected map[string]bool) bool {
	for _, p := range poolBackups {
		if strings.HasPrefix(tarPath, "files/"+p.dir+"/") || tarPath == "config/"+p.usedFile {
			detected[p.key] = true
			return true
		}
	}
	return false
}

// reloadPools reloads every detected pool into the engine after extraction.
func (h *HTTPServer) reloadPools(detected map[string]bool, restoredMap map[string]bool) {
	for _, p := range poolBackups {
		if !detected[p.key] {
			continue
		}
		poolErr, usedErr := p.reload(h)
		if poolErr == nil {
			log.Printf("[HTTP] Restore: %s reservoir loaded into engine", p.key)
		} else {
			log.Printf("[HTTP] Restore: %s reservoir extracted but could not be reloaded: %v", p.key, poolErr)
		}
		if usedErr == nil {
			log.Printf("[HTTP] Restore: %s used-flags loaded into engine", p.key)
		} else {
			log.Printf("[HTTP] Restore: %s used-flags extracted but could not be reloaded: %v", p.key, usedErr)
		}
		restoredMap[p.key] = true
	}
}

// isPoolArchivePath reports whether tarPath belongs to a pool of the list.
func isPoolArchivePath(tarPath string) bool {
	for _, p := range poolBackups {
		if strings.HasPrefix(tarPath, "files/"+p.dir+"/") || tarPath == "config/"+p.usedFile {
			return true
		}
	}
	return false
}
