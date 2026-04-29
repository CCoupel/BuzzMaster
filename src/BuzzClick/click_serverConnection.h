#pragma once
#include "click_includes.h"
#include "Common/CustomLogger.h"
#include "Common/led.h"

#include <esp_timer.h>

#ifdef USE_WEBSOCKET
// Forward declarations for WebSocket wrapper functions
// Defined in click_websocket_espidf.h, included after this header via click_WifiManager.h
void ws_sendBuzz(const String& mac, const String& buttonName);
void ws_sendPong(const String& mac);
void ws_sendAck(const String& mac, const String& ackAction, const String& ackId);  // ACK v3.8.0 #54
bool ws_isConnected();
void ws_connect();

// Forward declaration for OTA manager
// Defined in click_otaManager.h, included after click_WifiManager.h in click_MAIN.cpp
void performOTA(const String& url, const String& expectedVersion);
#else
#include <AsyncUDP.h>
#endif

JsonDocument myCompleteConfig;  // Store the complete configuration
bool isConfigInitialized = false;

String myConfig="{ }";
static const char* SRV_TAG = "ServerConnection";
uint64_t lastNTPUpdate = 0;
int64_t ntpOffset = 0;

bool isGameStarted = false;
bool hasTeamAssignment = false;
bool bootComplete = false;

// Declared in click_otaManager.h; guarded here so LED_SET is ignored during OTA flash.
extern volatile bool otaInProgress;

// LED state driven by server LED_SET action (v3.4.0)
// The server sends LED_SET at each relevant game state change; the firmware applies it directly.
// No local LED state machine — the server is the single source of truth.
bool ledBlinking = false;
static unsigned long ledLastBlink = 0;
static bool ledBlinkOn = true;
bool ledSetReceived = false;  // true once the server has sent at least one LED_SET — skips boot LED overrides

// COMET animation state (v3.7.0) — triggered by LED_SET with EFFECT="COMET"
// Non-blocking state machine: called from loop() via manageLedComet().
// Visual: dense gold band (~14-15 LEDs) rotates around the 23-LED ring; background shows team colour.
static const uint8_t COMET_MAX_LAPS = 2;     // 2 full revolutions
static const unsigned long COMET_STEP_MS = 100; // 100 ms per step
static volatile bool cometActive    = false;
static uint8_t cometR        = 255;
static uint8_t cometG        = 215;
static uint8_t cometB        = 0;
static int16_t cometPos      = 0;   // current head position (0..NUMPIXELS-1)
static uint16_t cometLaps    = 0;   // completed full revolutions
static unsigned long cometLastTime = 0;

// SPINNER animation state (v3.7.0) — triggered by LED_SET with EFFECT="SPINNER"
// Non-blocking state machine: called from loop() via manageLedSpinner().
// Visual: single bright pixel orbiting the 23-LED ring at 100 ms/step (~2.3 s/turn).
// Delta-update identical to WS_RECONNECTING: only 2 pixels touched per tick.
// Runs until the next LED_SET cancels it (server-controlled duration).
#define SPINNER_STEP_MS 100
static volatile bool spinnerActive = false;
static uint8_t spinnerR      = 255;
static uint8_t spinnerG      = 215;
static uint8_t spinnerB      = 0;
static int16_t spinnerPos    = 0;   // current pixel position (0..NUMPIXELS-1)
static unsigned long spinnerLastTime = 0;

// Gray rotation animation state
static unsigned long lastRotationTime = 0;
static int currentLedIndex = 0;
const int ROTATION_INTERVAL_MS = 200;

#ifndef USE_WEBSOCKET
AsyncUDP udp;

String BcastJsonBuffer = "";
const size_t MAX_BUFFER_SIZE = 8192;
#endif


/* SOCKET */

#ifndef USE_WEBSOCKET
bool connectSRV()
{
  if (!client) {
    client = new AsyncClient();
  }

  if (client->connected()) {
    client->close(true);
  }

  client->onConnect(onConnect, NULL);
  client->onData(onData, NULL);
  client->onDisconnect(onDisconnect, NULL);

  ESP_LOGI(SRV_TAG, "Connecting from %s To SERVER %s:%i", WiFi.localIP().toString(), serverIP, CONTROLER_PORT);

  if (!client->connect(serverIP.c_str(), CONTROLER_PORT)) {
    ESP_LOGW(SRV_TAG, "Connection Failed...");
    return false;
    }

  unsigned long startTime = millis();
  while (!client->connected() && millis() - startTime < 5000) {
    Serial.print(".");
    delay(100);
  }
  
  if (!client->connected()) {
    ESP_LOGW(SRV_TAG, "Connection timeout");
    return false;
  }
  
  ESP_LOGW(SRV_TAG, "Connected to server");
  return true;
}
#endif // !USE_WEBSOCKET


bool getServerIP()
{
  if (!MDNS.begin("buzzclick")) {
    ESP_LOGE(SRV_TAG, "Error setting up MDNS responder!");
    return false;
  }
  ESP_LOGI(SRV_TAG, "Waiting for Server...");
  unsigned long startTime = millis();
  while (serverIP.isEmpty() && millis() - startTime < 30000) { // 30 seconds timeout
    String hostName = "buzzcontrol";

    int n = MDNS.queryService("sock", "tcp");
    for (int i = 0; i < n; i++) {
      String hostname = MDNS.hostname(i);
      if (hostname == hostName || hostname == hostName + ".local") {
        serverIP = MDNS.IP(i).toString();
        localUdpPort = MDNS.port(i);
        ESP_LOGI(SRV_TAG, "Server found: %s:%d", serverIP.c_str(), localUdpPort);
        return true;
      }
    }
    delay(1000);
  }
  
  ESP_LOGW(SRV_TAG, "Server not found within timeout.");
  return false;
}

#ifndef USE_WEBSOCKET
void onConnect(void* arg, AsyncClient* c) {
  ESP_LOGI(SRV_TAG, "Connected to server: %s:%d", c->remoteIP().toString().c_str(), c->remotePort());
  hello_bumper();
}

void onData(void* arg, AsyncClient* c, void* data, size_t len) {
  String s_data = String((char*)data).substring(0, len);
  ESP_LOGE(SRV_TAG, "direct DATA received: %s", s_data.c_str());
}

void onDisconnect(void* arg, AsyncClient* c) {
  ESP_LOGI(SRV_TAG, "Disconnected from server");
  lastCheckTime=0;
  connectSRV();
}

void send_to_server(String msg)
{
  ESP_LOGI(SRV_TAG, "Send MSG:%s", msg.c_str());
  client->write((msg + "\n").c_str(), msg.length() + 1);
}

void sendMSG(String msgType, String message)
{
  String msg = "\"ID\": \"" + WiFi.macAddress() + "\"";
  msg += ", \"VERSION\": \"" + String(VERSION) +"\"";
  msg += ", \"ACTION\": \"" + msgType + "\"";
  msg += ", \"MSG\": " + message ;
  send_to_server("{" + msg +"}");
}

/* BROADCAST */
void resetBcastBuffer() {
  if (BcastJsonBuffer.length() > MAX_BUFFER_SIZE || BcastJsonBuffer.indexOf('{') == -1) {
    ESP_LOGW(SRV_TAG, "Buffer reset: size=%d", BcastJsonBuffer.length());
    BcastJsonBuffer = "";
  }
}

void onDataBroadcast(AsyncUDPPacket packet) {
  if (packet.length() <= 0) {
    ESP_LOGW(SRV_TAG, "Empty broadcast packet received");
    return;
  }

    String s_data = String((char*)packet.data(), packet.length());
    ESP_LOGI(SRV_TAG, "Broadcast data received from %s: %i => %s", packet.remoteIP().toString().c_str(), packet.length(), s_data.c_str());

    BcastJsonBuffer += s_data;
  if (BcastJsonBuffer.length() > MAX_BUFFER_SIZE) {
    ESP_LOGW(SRV_TAG, "Buffer overflow, truncating: %d bytes", BcastJsonBuffer.length());
    BcastJsonBuffer = BcastJsonBuffer.substring(BcastJsonBuffer.length() - MAX_BUFFER_SIZE);
  }

  int startPos = 0;
  int endOfJson;
  while ((endOfJson = BcastJsonBuffer.indexOf('\0', startPos)) > 0) {
    String jsonPart = BcastJsonBuffer.substring(startPos, endOfJson);

    if (jsonPart.indexOf('{') >= 0 && jsonPart.indexOf('}') >= 0) {
      ESP_LOGI(SRV_TAG, "Processing complete broadcast message: %s", jsonPart.c_str());
      parseJSON(jsonPart, nullptr);
    } else {
      ESP_LOGW(SRV_TAG, "Invalid JSON fragment received: %s", jsonPart.c_str());
    }

    startPos = endOfJson + 1;
  }

  if (startPos > 0) {
    BcastJsonBuffer = BcastJsonBuffer.substring(startPos);
  }

  resetBcastBuffer();
}

bool initBroadcastUDP()
{
  if (!udp.listen(CONTROLER_PORT)) {
    ESP_LOGW(SRV_TAG, "Failed to start UDP listener");
    return false;
  }

  ESP_LOGI(SRV_TAG, "UDP listening on port %d", CONTROLER_PORT);

  udp.onPacket([](AsyncUDPPacket packet) {
    onDataBroadcast(packet);
  });

  ESP_LOGI(SRV_TAG, "Broadcast UDP initialized on port %d", CONTROLER_PORT);
  return true;
}
#endif // !USE_WEBSOCKET


/* LED ANIMATION */
void startGrayRotation() {
    hasTeamAssignment = false;
    lastRotationTime = 0;
    currentLedIndex = 0;
}

void updateGrayRotation() {
    if (otaInProgress) return;
    if (millis() - lastRotationTime > ROTATION_INTERVAL_MS) {
        // Clear all LEDs
        for (int i = 0; i < NUMPIXELS; i++) {
            strip_sk98.setPixelColor(i, 0, 0, 0);
        }

        // Light up 1 LED out of 3 in gray
        for (int offset = 0; offset < NUMPIXELS; offset += 3) {
            int ledIndex = (currentLedIndex + offset) % NUMPIXELS;
            strip_sk98.setPixelColor(ledIndex, 64, 64, 64);
        }

        showPixels();

        currentLedIndex = (currentLedIndex + 1) % 3;
        lastRotationTime = millis();
    }
}

void stopGrayRotation() {
    hasTeamAssignment = true;
    // LED color is driven by server LED_SET — do not clear LEDs here.
    ESP_LOGD(SRV_TAG, "Gray rotation stopped");
}

/* CORE */
void startGame() {
  isGameStarted=true;
//  setLedIntensity(10);
}

void stopGame() {
  isGameStarted=false;
//  setLedIntensity(255);

}

void pauseGame() {
  isGameStarted=false;
//  setLedIntensity(128);
}

// manageLedBlink — called from loop() to animate the BLINK effect (LED_SET with EFFECT="BLINK").
// Toggles between 100% and 25% intensity at 400ms interval.
void manageLedBlink() {
  if (otaInProgress) return;
  if (!ledBlinking) return;
  const unsigned long BLINK_INTERVAL_MS = 400;
  if (millis() - ledLastBlink >= BLINK_INTERVAL_MS) {
    ledBlinkOn = !ledBlinkOn;
    setLedIntensity(ledBlinkOn ? 255 : 64);
    ledLastBlink = millis();
  }
}

// manageLedComet — called from loop() to animate the COMET band effect.
// Visual: a dense gold band (14/23 LEDs) rotates around the ring at 100ms/step.
// Background (9 LEDs) shows team colour. Auto-stops after COMET_MAX_LAPS revolutions.
void manageLedComet() {
  if (otaInProgress) return;
  if (!cometActive)  return;

  const unsigned long now = millis();
  if (now - cometLastTime < COMET_STEP_MS) return;
  cometLastTime = now;

  // 23-element pattern: 1 = gold, 0 = team colour
  // Dense gold centre band with sparse team-colour gaps at edges
  static const uint8_t kPattern[NUMPIXELS] = {
    0,1,0,0,1,0,1,0,1,1,1,1,1,1,1,1,1,1,1,0,1,0,0
  };

  // Compute team background colour at current intensity
  const uint8_t bg_r = (uint8_t)((currentRed   * currentIntensity) / 255);
  const uint8_t bg_g = (uint8_t)((currentGreen * currentIntensity) / 255);
  const uint8_t bg_b = (uint8_t)((currentBlue  * currentIntensity) / 255);

  // Band colour from COMET_COLOR (set by server for contrast, default gold)
  const uint8_t band_r = (uint8_t)((cometR * currentIntensity) / 255);
  const uint8_t band_g = (uint8_t)((cometG * currentIntensity) / 255);
  const uint8_t band_b = (uint8_t)((cometB * currentIntensity) / 255);

  // Paint all 23 LEDs based on rotated pattern
  for (int i = 0; i < NUMPIXELS; i++) {
    const uint8_t patIdx = (uint8_t)((i + (int)cometPos) % NUMPIXELS);
    if (kPattern[patIdx]) {
      strip_sk98.setPixelColor((int16_t)i, band_r, band_g, band_b);
    } else {
      strip_sk98.setPixelColor((int16_t)i, bg_r, bg_g, bg_b);
    }
  }
  showPixels();

  // Advance position
  cometPos = (uint8_t)((cometPos + 1) % NUMPIXELS);

  // Count completed laps
  if (cometPos == 0) {
    cometLaps++;
    if (cometLaps >= COMET_MAX_LAPS) {
      cometActive = false;
      applyLedColor();
      ESP_LOGI(SRV_TAG, "COMET animation complete (%d laps)", COMET_MAX_LAPS);
    }
  }
}

// manageLedSpinner — called from loop() to animate the SPINNER effect (LED_SET with EFFECT="SPINNER").
// Non-blocking state machine: each call advances the animation by at most one step.
// Visual: a single bright pixel orbits the 23-LED ring; background pixels are restored via delta-update.
// Identical mechanism to WS_RECONNECTING in click_ledErrorPatterns.h but server-color driven.
// Runs until the next LED_SET from the server sets spinnerActive = false.
void manageLedSpinner() {
  if (otaInProgress) return;
  if (!spinnerActive) return;

  const unsigned long now = millis();
  if (now - spinnerLastTime < SPINNER_STEP_MS) return;

  // Restore previous pixel to current background colour (adjusted for intensity)
  const uint8_t adj_r = (uint8_t)((currentRed   * currentIntensity) / 255);
  const uint8_t adj_g = (uint8_t)((currentGreen * currentIntensity) / 255);
  const uint8_t adj_b = (uint8_t)((currentBlue  * currentIntensity) / 255);
  strip_sk98.setPixelColor((int16_t)spinnerPos, adj_r, adj_g, adj_b);

  // Advance and light the next pixel with the spinner colour at full intensity
  spinnerPos = (spinnerPos + 1) % NUMPIXELS;
  strip_sk98.setPixelColor((int16_t)spinnerPos, spinnerR, spinnerG, spinnerB);

  showPixels();
  spinnerLastTime = now;
}

void handleUpdateAction(JsonObject& message, const String& macAddress) {
  JsonObject buzzer;
  JsonObject team;
  String output;
  JsonDocument JsonDoc;
  JsonArray colorArray = JsonDoc.to<JsonArray>();
  colorArray.add(0);
  colorArray.add(0);
  colorArray.add(0);

  // Check if we're receiving a bumper update for our device
  if (message.containsKey("bumpers") && message["bumpers"].containsKey(macAddress)) {
    // Extract the bumper config for our device
    buzzer = message["bumpers"][macAddress].as<JsonObject>();

    // === BOOT PHASE 6: GREEN 3/4 - Server acknowledged HELLO ===
    // Skip if LED_SET was already received — server color takes priority over boot animation.
    if (!isConfigInitialized) {
      if (!ledSetReceived) {
        setLedColorExclude(0, 255, 0, 4);
        ESP_LOGI(SRV_TAG, "Boot phase: GREEN 3/4 (HELLO acknowledged)");
        delay(500);
      } else {
        ESP_LOGI(SRV_TAG, "Boot phase: GREEN 3/4 skipped (LED_SET already received)");
      }
      bootComplete = true;
    }

    myCompleteConfig["buzzer"] = buzzer;
    isConfigInitialized = true;
  }

  buzzer = myCompleteConfig["buzzer"].as<JsonObject>();
    // Log current buzzer config
  serializeJson(myCompleteConfig["buzzer"], output);
  ESP_LOGI(SRV_TAG, "My Config=%s", output.c_str());
  myConfig = output;


  // Process team information
  const char* t_name = "";
  if (buzzer.containsKey("TEAM")) {
    t_name = buzzer["TEAM"];
    String teamName = String(t_name);

    if (teamName.length() > 0) {
      hasTeamAssignment = true;
      ESP_LOGI(SRV_TAG, "My team=%s", t_name);

      if (message.containsKey("teams") && message["teams"].containsKey(t_name)) {
        team = message["teams"][t_name];

        // Store/update the team information in our complete config
        myCompleteConfig["team"] = team;

        serializeJson(team, output);
        ESP_LOGI(SRV_TAG, "    =>%s", output.c_str());
      }
    } else {
      // TEAM field exists but is empty - buzzer removed from team
      ESP_LOGI(SRV_TAG, "UPDATE - Team field empty, starting gray rotation");
      startGrayRotation();
    }
  } else {
    // No TEAM field at all - buzzer not assigned to any team
    ESP_LOGI(SRV_TAG, "UPDATE - No team assigned, starting gray rotation");
    startGrayRotation();
  }

  team = myCompleteConfig["team"].as<JsonObject>();

  // Extract color information and update LED
  int r = 128, g = 128, b = 128;  // Default: gray (no team)

  if (team.containsKey("COLOR")) {
    colorArray = team["COLOR"].as<JsonArray>();
    if (colorArray.size() >= 3) {
      r = colorArray[0];
      g = colorArray[1];
      b = colorArray[2];
      ESP_LOGI(SRV_TAG, "Team color: RGB(%d, %d, %d)", r, g, b);
    }
  } else if (team.containsKey("color")) {
    colorArray = team["color"].as<JsonArray>();
    if (colorArray.size() >= 3) {
      r = colorArray[0];
      g = colorArray[1];
      b = colorArray[2];
      ESP_LOGI(SRV_TAG, "Team color: RGB(%d, %d, %d)", r, g, b);
    }
  } else {
    // No team assigned - start gray rotation
    ESP_LOGI(SRV_TAG, "No team assigned - starting gray rotation");
    startGrayRotation();
  }

  // Stop gray rotation if team assigned — LED will be driven by next LED_SET from server
  if (hasTeamAssignment) {
    #ifdef USE_WEBSOCKET
    if (ws_isConnected()) {
      stopGrayRotation();
    }
    #else
    stopGrayRotation();
    #endif
  }

  // Process team status (game state tracking only — no LED changes here)
  if (team.containsKey("STATUS")) {
    String status = team["STATUS"].as<String>();
    if (status == "PAUSE") {
      pauseGame();
    }
  }

  // Always store the full message for reference
  myCompleteConfig["lastMessage"] = message;
}


void handleReadyAction(JsonDocument& doc) {
    // READY: server will send LED_SET immediately after.
    // We only update team assignment state here — no LED changes.
    ESP_LOGI(SRV_TAG, "Received READY - awaiting LED_SET from server");

    String myMAC = WiFi.macAddress();
    JsonObject msg = doc["MSG"].as<JsonObject>();

    if (msg.containsKey("bumpers") && msg["bumpers"].containsKey(myMAC)) {
        JsonObject myBumper = msg["bumpers"][myMAC].as<JsonObject>();

        if (myBumper.containsKey("TEAM")) {
            String teamName = myBumper["TEAM"].as<String>();
            if (teamName.length() == 0) {
                ESP_LOGI(SRV_TAG, "READY - Team field empty, starting gray rotation");
                startGrayRotation();
            } else {
                hasTeamAssignment = true;
                stopGrayRotation();
            }
        } else {
            ESP_LOGI(SRV_TAG, "READY - No team assigned, starting gray rotation");
            startGrayRotation();
        }
    }
}

void hello_bumper()
{
#ifdef USE_WEBSOCKET
  // In WS mode, HELLO/enroll is sent by wsClient on connect
  ESP_LOGI(SRV_TAG, "hello_bumper: WS mode, enroll handled by wsClient");
#else
  sendMSG("HELLO", myConfig);
#endif
}

void resetGame() {
  myConfig = "{ 'IP': '" + WiFi.localIP().toString() + "'";
        myConfig += ", 'VERSION': '" + String(VERSION) +"'";
        myConfig += "}";
}

void parseJSON(const String& data, AsyncClient* c) {
  JsonDocument receivedData;
  ESP_LOGD(SRV_TAG, " parse JSON: %s", data.c_str());

  DeserializationError error = deserializeJson(receivedData, data);
  if (error) {
    ESP_LOGE(SRV_TAG, "Failed to parse JSON: %s", error.c_str());
    return;
  }

  const char* action = receivedData["ACTION"];
  JsonObject message = receivedData["MSG"];
  ESP_LOGD(SRV_TAG, "Parsing ACTION=%s", action);

#ifdef USE_WEBSOCKET
  // ACK protocol (v3.8.0 — #54): send ACK immediately before applying action.
  // MSG_ID is optional (omitempty on server) — old firmware ignores it, new firmware ACKs it.
  // ws_sendAck() is non-blocking (timeout=0) — safe to call from event handler context.
  if (receivedData.containsKey("MSG_ID")) {
    const char* msgId = receivedData["MSG_ID"] | "";
    if (strlen(msgId) > 0) {
      ws_sendAck(WiFi.macAddress(), String(action), String(msgId));
    }
  }
#endif

  // Utiliser un switch case avec hash pour un traitement plus rapide et plus propre
  switch (hash(action)) {
    case hash("START"):
    case hash("CONTINUE"):
      ESP_LOGI(SRV_TAG, "STARTING");
      startGame();
      break;
      
    case hash("STOP"):
      ESP_LOGI(SRV_TAG, "STOPPING");
      stopGame();
      break;
      
    case hash("PAUSE"):
      ESP_LOGI(SRV_TAG, "PAUSING");
      pauseGame();
      break;
      
    case hash("PING"):
      ESP_LOGI(SRV_TAG, "Replying PONG");
      resetGame();
#ifdef USE_WEBSOCKET
      ws_sendPong(WiFi.macAddress());
#else
      sendMSG("PONG", "'" + WiFi.localIP().toString() + "'");
#endif
      break;
      
    case hash("UPDATE"):
    case hash("UPDATE_TIMER"):
      ESP_LOGI(SRV_TAG, action[0] == 'U' ? "Updating My Config: %s" : "UPDATING TIMER", 
               WiFi.macAddress().c_str());
      handleUpdateAction(message, WiFi.macAddress());
      break;
      
    case hash("HELLO"):
      ESP_LOGI(SRV_TAG, "Send HELLO to Controller");
#ifdef USE_WEBSOCKET
      if (!ws_isConnected()) {
        ws_connect();
      }
#else
      connectSRV();
#endif
      break;
      
    case hash("READY"):
      ESP_LOGI(SRV_TAG, "Received READY state");
      handleReadyAction(receivedData);
      break;

    case hash("RESET"):
      ESP_LOGI(SRV_TAG, "Resetting Data");
      resetGame();
      break;

    case hash("LED_SET"):
      {
        // Server-driven LED control (v3.4.0).
        // MSG: { "COLOR": [R, G, B], "INTENSITY": 0-255, "EFFECT": "SOLID"|"BLINK"|"DIM" }
        if (otaInProgress) break;  // OTA controls LEDs directly; ignore server overrides.
        JsonArray colorArr = message["COLOR"].as<JsonArray>();
        int intensity = message["INTENSITY"] | 255;
        const char* effect = message["EFFECT"] | "SOLID";
        if (colorArr.size() == 3) {
          int r = colorArr[0]; int g = colorArr[1]; int b = colorArr[2];
          if (strcmp(effect, "COMET") == 0) {
            // COMET: COLOR = background (team color), COMET_COLOR = band color (gold or white).
            // COMET_COLOR is chosen by the server for contrast; defaults to gold if absent.
            setLedColor(r, g, b);
            setLedIntensity(intensity);
            JsonArray cometColorArr = message["COMET_COLOR"].as<JsonArray>();
            if (cometColorArr.size() == 3) {
              cometR = (uint8_t)(int)cometColorArr[0];
              cometG = (uint8_t)(int)cometColorArr[1];
              cometB = (uint8_t)(int)cometColorArr[2];
            } else {
              cometR = 255; cometG = 215; cometB = 0;  // default gold
            }
            cometPos      = 0;
            cometLaps     = 0;
            cometLastTime = millis();
            cometActive   = true;
            ledBlinking   = false;
            spinnerActive = false;  // cancel any running spinner
            ledSetReceived = true;
            ESP_LOGI(SRV_TAG, "LED_SET: COMET bg=RGB(%d,%d,%d) band=RGB(%d,%d,%d)", r, g, b, cometR, cometG, cometB);
          } else if (strcmp(effect, "SPINNER") == 0) {
            // SPINNER: single bright pixel orbiting the ring at 100 ms/step.
            // Background colour is preserved (delta-update) — the server controls duration
            // by sending the next LED_SET to stop it.
            spinnerR = (uint8_t)r;
            spinnerG = (uint8_t)g;
            spinnerB = (uint8_t)b;
            spinnerPos      = 0;
            spinnerLastTime = millis();
            spinnerActive   = true;
            ledBlinking     = false;  // cancel any running blink
            cometActive     = false;  // cancel any running comet
            ledSetReceived  = true;
            ESP_LOGI(SRV_TAG, "LED_SET: SPINNER RGB(%d,%d,%d)", r, g, b);
          } else {
            setLedColor(r, g, b);
            setLedIntensity(intensity);
            ledBlinking = (strcmp(effect, "BLINK") == 0);
            if (ledBlinking) { ledBlinkOn = true; ledLastBlink = millis(); }
            cometActive   = false;  // cancel any running comet
            spinnerActive = false;  // cancel any running spinner
            ledSetReceived = true;
            ESP_LOGI(SRV_TAG, "LED_SET: RGB(%d,%d,%d) intensity=%d effect=%s", r, g, b, intensity, effect);
          }
        } else {
          ESP_LOGW(SRV_TAG, "LED_SET: invalid COLOR array (size=%d)", colorArr.size());
        }
      }
      break;

#ifdef USE_WEBSOCKET
    case hash("OTA_UPDATE"):
      {
        const char* version = message["VERSION"] | "";
        // Build firmware URL from the server we are actually connected to.
        // serverIP + localUdpPort are set by tryConnectToServer() and reflect the
        // real connection (UDP broadcast heartbeat, NVS fallback, or mDNS).
        // The broadcasted PORT must be honored — servers may run on non-default
        // ports (e.g. 8080) where hardcoding port 80 would make OTA fail.
        String otaUrl;
        if (serverIP.length() > 0 && localUdpPort > 0) {
            otaUrl = "http://" + serverIP + ":" + String(localUdpPort) + "/api/firmware/buzzclick/latest.bin";
        } else if (currentConfig.server_ip.length() > 0) {
            uint16_t port = currentConfig.server_tcp_port > 0 ? currentConfig.server_tcp_port : 80;
            otaUrl = "http://" + currentConfig.server_ip + ":" + String(port) + "/api/firmware/buzzclick/latest.bin";
            ESP_LOGW(SRV_TAG, "Using NVS fallback for OTA URL: %s", otaUrl.c_str());
        } else {
            otaUrl = String(message["URL"] | "");
            ESP_LOGW(SRV_TAG, "No connected server known, using URL from message: %s", otaUrl.c_str());
        }
        ESP_LOGI(SRV_TAG, "Received OTA_UPDATE: version=%s url=%s", version, otaUrl.c_str());
        if (strlen(version) == 0) {
            ESP_LOGW(SRV_TAG, "OTA_UPDATE ignored: missing version");
        } else if (otaInProgress) {
            ESP_LOGW(SRV_TAG, "OTA_UPDATE ignored: OTA already in progress");
        } else {
            // Launch OTA in a separate FreeRTOS task so the WebSocket event loop
            // is not blocked — otherwise the server's ReadDeadline (5 s) fires
            // during download (no pong response possible) and closes the socket,
            // causing ws_sendRaw(portMAX_DELAY) inside performOTA to deadlock and
            // the HTTP download to time out (~20 % failure).
            String* params = new String[2];
            params[0] = otaUrl;
            params[1] = String(version);
            xTaskCreate([](void* p) {
                String* args = static_cast<String*>(p);
                performOTA(args[0], args[1]);
                delete[] args;
                vTaskDelete(NULL);
            }, "ota_task", 16384, params, 5, nullptr);
        }
      }
      break;

    case hash("WIFI_CONFIG"):
      {
        const char* ssid  = message["SSID"]       | "";
        const char* pass  = message["PASS"]       | "";
        const char* ip    = message["SERVER_IP"]  | "";
        int port          = message["SERVER_PORT"] | 0;
        const char* ssid2 = message["SSID2"]      | "";
        const char* pass2 = message["PASS2"]      | "";

        ESP_LOGI(SRV_TAG, "WIFI_CONFIG received: SSID=%s IP=%s PORT=%d SSID2=%s",
                 ssid, ip, port, ssid2);

        if (strlen(ssid) == 0 || strlen(ip) == 0) {
            ESP_LOGW(SRV_TAG, "WIFI_CONFIG ignored: empty SSID or IP");
            break;
        }

        BuzzClickConfig& cfg = nvsGetConfig();
        bool changed = (cfg.wifi_ssid != ssid)
                    || (cfg.wifi_password != pass)
                    || (cfg.server_ip != ip)
                    || (port > 0 && cfg.server_tcp_port != (uint16_t)port)
                    || (cfg.wifi_ssid2 != ssid2)
                    || (cfg.wifi_pass2 != pass2);

        cfg.wifi_ssid = ssid;
        cfg.wifi_password = pass;
        cfg.server_ip = ip;
        if (port > 0) cfg.server_tcp_port = (uint16_t)port;
        cfg.wifi_ssid2 = ssid2;
        cfg.wifi_pass2 = pass2;
        nvsSaveConfig();

        if (changed) {
            ESP_LOGI(SRV_TAG, "WIFI_CONFIG: config changed, rebooting in 3s...");
            delay(3000);
            ESP.restart();
        } else {
            ESP_LOGI(SRV_TAG, "WIFI_CONFIG: config unchanged, no reboot needed");
        }
      }
      break;
#endif

    default:
      ESP_LOGW(SRV_TAG, "Unknown action: %s", action);
      break;
  }
}

int64_t getAbsoluteTimeMicros() {
  return micros() + ntpOffset;
}

void IRAM_ATTR buttonHandler(void *arg) {
  // ISR: only set the flag, no heap allocation (String forbidden in ISR context).
  // The server decides whether to accept or reject the buzz based on game phase.
  ButtonInfo* buttonInfo = static_cast<ButtonInfo*>(arg);
  buttonInfo->pressed = true;
}

void attachButtons()
{
  static const size_t BUTTON_COUNT = sizeof(buttonsInfo) / sizeof(ButtonInfo);
  
  for (size_t id = 0; id < BUTTON_COUNT; id++)  
  {
    if (buttonsInfo[id].pin == -1) {
      ESP_LOGW(SRV_TAG, "Button %zu (%s) has no pin assigned, skipping", id, buttonsInfo[id].name.c_str());
      continue;
    }

    pinMode(buttonsInfo[id].pin, INPUT_PULLUP); 
    pinMode(pin_gnd, OUTPUT); 
    digitalWrite(pin_gnd, LOW); 

    int8_t interruptPin = digitalPinToInterrupt(buttonsInfo[id].pin);
    if (interruptPin == -1) {
      ESP_LOGE(SRV_TAG, "Failed to get interrupt for button %zu (%s) on pin %d", id, buttonsInfo[id].name.c_str(), buttonsInfo[id].pin);
      continue;
    }

    attachInterruptArg(interruptPin, buttonHandler, &buttonsInfo[id], FALLING);
    ESP_LOGI(SRV_TAG, "Button %zu (%s) attached to pin %d", id, buttonsInfo[id].name.c_str(), buttonsInfo[id].pin);
  }

  ESP_LOGI(SRV_TAG, "All buttons initialized. Total buttons: %zu", BUTTON_COUNT);
}

void manageButtonMessages() {
  static const size_t BUTTON_COUNT = sizeof(buttonsInfo) / sizeof(ButtonInfo);

  for (size_t id = 0; id < BUTTON_COUNT; id++)
  {
    if (buttonsInfo[id].pressed) {
      // Capture timestamp here (safe: main loop context, not ISR)
      buttonsInfo[id].time = String(micros());
      buttonsInfo[id].pressed = false;  // clear flag before sending to avoid double-send

      ESP_LOGI(SRV_TAG, "Button pressed: %s", buttonsInfo[id].name.c_str());

#ifdef USE_WEBSOCKET
      ws_sendBuzz(WiFi.macAddress(), buttonsInfo[id].name);
#else
      JsonDocument doc;
      doc["button"] = buttonsInfo[id].name;

      String msg;
      if (serializeJson(doc, msg) == 0) {
        ESP_LOGE(SRV_TAG, "Failed to serialize JSON for button %zu", id);
        continue;
      }

      sendMSG("BUTTON", msg);
#endif
    }
  }
}
