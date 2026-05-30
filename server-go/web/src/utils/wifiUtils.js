/**
 * wifiUtils — helpers pour la génération de QR codes WiFi (ZXing WiFi spec)
 *
 * Caractères à échapper dans les champs SSID et password : \ ; , "
 * Le backslash est traité en premier (regex single-pass, pas de risque de double-escape).
 */

export function escapeWifiString(str) {
  return str.replace(/[\\;,"]/g, c => '\\' + c)
}
