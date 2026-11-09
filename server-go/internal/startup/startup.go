// Package startup holds what the server does at boot, before and around the
// HTTP server: port resolution (--port flag / config.json), the IP shown to
// buzzers, the list of URLs, the pages opened in the browser and the Bonjour
// check. Extracted from cmd/server (#291, Phase B).
package startup

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/server"
)

// ResolvePort returns the effective HTTP port.
// If flagPort > 0 it takes precedence over configPort (--port CLI flag).
func ResolvePort(configPort, flagPort int) int {
	if flagPort > 0 {
		return flagPort
	}
	return configPort
}

// ResolvePortSource returns a short human-readable description of where the
// effective HTTP port came from — surfaced in HTTPServer's bind-wait log
// messages (#220 acceptance criterion: "le message d'attente nomme le port
// résolu et sa provenance"). Reads config.json's raw bytes independently of
// config.Load/ApplyDefaults purely to tell an explicit http_port in the file
// apart from the code default (both of which config.Load already merges into
// one int by the time main() sees it) — best-effort only, this is diagnostic
// text, never used for any decision.
func ResolvePortSource(configPath string, flagPort, effectivePort int) string {
	if flagPort > 0 {
		return fmt.Sprintf("--port flag (%d)", flagPort)
	}
	if data, err := os.ReadFile(configPath); err == nil {
		var probe struct {
			Server struct {
				HTTPPort int `json:"http_port"`
			} `json:"server"`
		}
		if json.Unmarshal(data, &probe) == nil && probe.Server.HTTPPort != 0 {
			return fmt.Sprintf("config.json (%d)", probe.Server.HTTPPort)
		}
	}
	return fmt.Sprintf("code default (%d)", effectivePort)
}

// FormatIPList renders an address list for the journal: "[a,b]" ("[]" when empty).
func FormatIPList(ips []string) string {
	return "[" + strings.Join(ips, ",") + "]"
}

// ResolveServerIP returns the actual server IP to send to buzzers.
// It uses the first IP from GetServerIPs() (same source as UDP broadcast heartbeats)
// so that buzzers always receive the current server IP regardless of what is stored in config.json.
// Falls back to cfg.ServerIP if no active interface is found.
func ResolveServerIP(cfgServerIP string) string {
	ips := server.GetServerIPs()
	if len(ips) > 0 {
		server.LogInfo(game.LogComponentApp, "resolveServerIP: using detected IP %s (GetServerIPs=%v)", ips[0], ips)
		return ips[0]
	}
	server.LogInfo(game.LogComponentApp, "resolveServerIP: no detected IP, falling back to config value %q", cfgServerIP)
	return cfgServerIP
}

// startupPage describes one browser tab opened automatically at server startup.
type startupPage struct {
	path string
	name string
}

// startupPages returns the ordered list of pages to open automatically at
// server startup. Pure function (no side effect, no process spawned) so it
// can be unit-tested independently of displayAndOpenURLs, which is the one
// that actually launches browser processes via openBrowser.
//
// Order: /admin (régie), /anim (animateur), /tv (affichage TV),
// / (accueil joueurs), then /logs (logs (debug)) if debug is enabled.
func startupPages(debug bool) []startupPage {
	pages := []startupPage{
		{"/admin", "régie"},
		{"/anim", "animateur"},
		{"/tv", "affichage TV"},
		{"/", "accueil joueurs"},
	}

	if debug {
		pages = append(pages, startupPage{"/logs", "logs (debug)"})
	}

	return pages
}

// DisplayAndOpenURLs shows all accessible URLs and opens the browser
func DisplayAndOpenURLs(httpPort int, autoOpen bool, debug bool) {
	log.Println("")
	log.Println("â•”â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•—")
	log.Println("â•‘                    WEB INTERFACE URLs                      â•‘")
	log.Println("â• â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•£")

	var primaryURL string

	// Get all local IPs
	interfaces, err := getNetworkInterfaces()
	if err == nil {
		for _, iface := range interfaces {
			url := fmt.Sprintf("http://%s:%d", iface.IP, httpPort)
			if httpPort == 80 {
				url = fmt.Sprintf("http://%s", iface.IP)
			}
			log.Printf("â•‘  %-56s  â•‘", fmt.Sprintf("%s (%s)", url, iface.Name))

			// Use first non-virtual interface as primary
			// (same virtual-interface list as the network alert — server.IsVirtualInterfaceName, #267)
			if primaryURL == "" && !server.IsVirtualInterfaceName(iface.Name) {
				primaryURL = url
			}
		}
	}

	// Localhost
	localhostURL := fmt.Sprintf("http://localhost:%d", httpPort)
	if httpPort == 80 {
		localhostURL = "http://localhost"
	}
	log.Printf("â•‘  %-56s  â•‘", localhostURL+" (localhost)")

	log.Println("â•šâ•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•")
	log.Println("")

	// Use localhost if no primary found
	if primaryURL == "" {
		primaryURL = localhostURL
	}

	// Only auto-open if enabled in config
	if !autoOpen {
		log.Println("Auto-open browsers disabled in config")
		return
	}

	// Open browsers to /admin, /anim, /tv and / pages with small delays
	pagesToOpen := startupPages(debug)

	for i, page := range pagesToOpen {
		url := primaryURL + page.path
		log.Printf("Opening browser: %s (%s)", url, page.name)
		openBrowser(url)
		// Small delay between browser opens to avoid resource issues
		if i < len(pagesToOpen)-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}
}

// NetworkInterface represents a network interface with IP
type NetworkInterface struct {
	Name string
	IP   string
}

// getNetworkInterfaces returns all active network interfaces with IPv4 addresses
func getNetworkInterfaces() ([]NetworkInterface, error) {
	var result []NetworkInterface

	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range interfaces {
		// Skip down and loopback interfaces
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			// Only IPv4
			if ip == nil || ip.To4() == nil {
				continue
			}

			result = append(result, NetworkInterface{
				Name: iface.Name,
				IP:   ip.String(),
			})
		}
	}

	return result, nil
}

// openBrowser opens the default browser with the given URL
func openBrowser(url string) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default: // Linux
		cmd = exec.Command("xdg-open", url)
	}

	if err := cmd.Start(); err != nil {
		server.LogWarn(game.LogComponentApp, "Failed to open browser: %v", err)
	}
}

// CheckBonjourSupport checks if Bonjour/mDNS is available on the system
func CheckBonjourSupport() {
	if runtime.GOOS != "windows" {
		// On Linux/macOS, mDNS is usually available via avahi or built-in
		log.Println("[mDNS] Running on", runtime.GOOS, "- mDNS should be available")
		return
	}

	// On Windows, check if Bonjour service is running
	log.Println("[mDNS] Checking Bonjour service status...")

	// Query the Bonjour service using sc command
	cmd := exec.Command("sc", "query", "Bonjour Service")
	output, err := cmd.Output()
	if err != nil {
		// Service not found or error
		log.Println("[mDNS] âš  Bonjour is NOT installed")
		log.Println("[mDNS]   â†’ mDNS hostname resolution (buzzcontrol.local) will NOT work")
		log.Println("[mDNS]   â†’ BuzzClick service discovery will still work")
		log.Println("[mDNS]   â†’ To enable hostname resolution, install Bonjour:")
		log.Println("[mDNS]     https://support.apple.com/kb/DL999")
		return
	}

	// Check if service is running
	outputStr := string(output)
	if strings.Contains(outputStr, "RUNNING") {
		log.Println("[mDNS] âœ“ Bonjour service is running")
		log.Println("[mDNS]   â†’ mDNS hostname resolution (buzzcontrol.local) should work")
	} else if strings.Contains(outputStr, "STOPPED") {
		log.Println("[mDNS] âš  Bonjour service is installed but STOPPED")
		log.Println("[mDNS]   â†’ Start the service: sc start \"Bonjour Service\"")
	} else {
		log.Println("[mDNS] âš  Bonjour service status unknown")
		log.Printf("[mDNS]   â†’ Output: %s", strings.TrimSpace(outputStr))
	}
}
