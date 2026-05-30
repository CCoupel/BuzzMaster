/**
 * Tests unitaires — escapeWifiString (non-régression #85)
 *
 * La fonction escapeWifiString() échappe les caractères spéciaux dans les
 * champs SSID et mot de passe du format WiFi QR code (ZXing WiFi spec).
 *
 * Caractères à échapper : \ ; , "
 * (ordre d'échappement : \ en premier pour éviter le double-échappement)
 *
 * Ces tests seront FAILING jusqu'à la création de src/utils/wifiUtils.js
 * par dev-frontend — comportement TDD attendu.
 */
import { describe, it, expect } from 'vitest'
import { escapeWifiString } from './wifiUtils'

describe('escapeWifiString — escaping champs WiFi QR', () => {

  describe('cas nominal — chaînes sans caractères spéciaux', () => {

    it('retourne la chaîne inchangée si aucun caractère spécial', () => {
      expect(escapeWifiString('SimpleWifi')).toBe('SimpleWifi')
    })

    it('retourne une chaîne vide inchangée', () => {
      expect(escapeWifiString('')).toBe('')
    })

    it('retourne une chaîne alphanumérique inchangée', () => {
      expect(escapeWifiString('BuzzControl2024')).toBe('BuzzControl2024')
    })

    it('conserve les tirets et underscores (non réservés)', () => {
      expect(escapeWifiString('Buzz-Control_2024')).toBe('Buzz-Control_2024')
    })

    it('conserve les espaces (non réservés dans le spec WiFi QR)', () => {
      expect(escapeWifiString('Mon Reseau')).toBe('Mon Reseau')
    })

  })

  describe("échappement du ';' (séparateur de champs WiFi QR)", () => {

    it("échappe un ';' isolé", () => {
      expect(escapeWifiString(';')).toBe('\\;')
    })

    it("échappe un SSID contenant ';'", () => {
      expect(escapeWifiString('My;Wifi')).toBe('My\\;Wifi')
    })

    it("échappe plusieurs ';' dans la chaîne", () => {
      expect(escapeWifiString('a;b;c')).toBe('a\\;b\\;c')
    })

    it("échappe un ';' en début de chaîne", () => {
      expect(escapeWifiString(';Wifi')).toBe('\\;Wifi')
    })

    it("échappe un ';' en fin de chaîne", () => {
      expect(escapeWifiString('Wifi;')).toBe('Wifi\\;')
    })

  })

  describe("échappement du ',' (virgule)", () => {

    it("échappe une ',' isolée", () => {
      expect(escapeWifiString(',')).toBe('\\,')
    })

    it("échappe un SSID contenant ','", () => {
      expect(escapeWifiString('My,Wifi')).toBe('My\\,Wifi')
    })

    it("échappe plusieurs ',' dans la chaîne", () => {
      expect(escapeWifiString('a,b,c')).toBe('a\\,b\\,c')
    })

  })

  describe('échappement du \'"\' (guillemet double)', () => {

    it('échappe un guillemet double isolé', () => {
      expect(escapeWifiString('"')).toBe('\\"')
    })

    it('échappe un SSID contenant des guillemets doubles', () => {
      expect(escapeWifiString('My"Wifi"')).toBe('My\\"Wifi\\"')
    })

  })

  describe("échappement du '\\' (backslash)", () => {

    it("échappe un backslash isolé", () => {
      expect(escapeWifiString('\\')).toBe('\\\\')
    })

    it("échappe un SSID contenant un backslash", () => {
      expect(escapeWifiString('My\\Wifi')).toBe('My\\\\Wifi')
    })

    it("n'échappe pas deux fois les backslashes déjà échappés (idempotence : NON)", () => {
      // escapeWifiString n'est pas idempotente : appliquer deux fois double l'échappement
      const once = escapeWifiString('\\')
      expect(once).toBe('\\\\')
      // Appliquer une seconde fois doit donner '\\\\\\\\' (4 backslashes)
      expect(escapeWifiString(once)).toBe('\\\\\\\\')
    })

  })

  describe('cas complexes — combinaisons de caractères spéciaux', () => {

    it("échappe '\\' avant ';' (ordre d'échappement correct)", () => {
      // '\;' → la backslash est échappée d'abord → '\\;'
      // puis ';' est échappée → '\\\\;' ? Non — si on échappe '\' en premier :
      // input: '\;'
      // après étape 1 (\ → \\): '\\;'
      // après étape 2 (; → \;): '\\\;'
      expect(escapeWifiString('\\;')).toBe('\\\\\\;')
    })

    it("échappe un SSID avec plusieurs types de caractères spéciaux", () => {
      // 'a;b,c"d' → 'a\;b\,c\"d'
      expect(escapeWifiString('a;b,c"d')).toBe('a\\;b\\,c\\"d')
    })

    it("échappe un mot de passe réaliste avec caractères spéciaux", () => {
      // 'P@ss;w0rd"!' → 'P@ss\;w0rd\"!'
      expect(escapeWifiString('P@ss;w0rd"!')).toBe('P@ss\\;w0rd\\"!')
    })

    it("SSID avec backslash et point-virgule", () => {
      // 'Net\\;work' → backslash d'abord → 'Net\\\\;work' → puis ; → 'Net\\\\\;work'
      expect(escapeWifiString('Net\\;work')).toBe('Net\\\\\\;work')
    })

  })

  describe('construction URL WiFi complète (intégration)', () => {

    it("construit une URL WiFi valide avec SSID et mot de passe simples", () => {
      const ssid = 'HomeWifi'
      const password = 'secret123'
      const url = `WIFI:T:WPA;S:${escapeWifiString(ssid)};P:${escapeWifiString(password)};;`
      expect(url).toBe('WIFI:T:WPA;S:HomeWifi;P:secret123;;')
    })

    it("construit une URL WiFi avec SSID contenant ';' (ne casse pas le format)", () => {
      const ssid = 'My;Network'
      const password = 'pass'
      const url = `WIFI:T:WPA;S:${escapeWifiString(ssid)};P:${escapeWifiString(password)};;`
      expect(url).toBe('WIFI:T:WPA;S:My\\;Network;P:pass;;')
    })

    it("construit une URL WiFi avec mot de passe contenant ','", () => {
      const ssid = 'SimpleWifi'
      const password = 'p,a,s,s'
      const url = `WIFI:T:WPA;S:${escapeWifiString(ssid)};P:${escapeWifiString(password)};;`
      expect(url).toBe('WIFI:T:WPA;S:SimpleWifi;P:p\\,a\\,s\\,s;;')
    })

    it("construit une URL WiFi sans mot de passe (nopass)", () => {
      const ssid = 'OpenNetwork'
      const url = `WIFI:T:nopass;S:${escapeWifiString(ssid)};P:;;`
      expect(url).toBe('WIFI:T:nopass;S:OpenNetwork;P:;;')
    })

  })

})
