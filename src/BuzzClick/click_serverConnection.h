#pragma once
#include "click_includes.h"
#include "Common/CustomLogger.h"
#include "Common/led.h"

#include <esp_timer.h>

#ifdef USE_WEBSOCKET
// Forward declarations for WebSocket wrapper functions
// Defined in click_websocketClient.h, included after this header
void ws_sendBuzz(const String& mac, const String& buttonName);
void ws_sendPong(const String& mac);
bool ws_isConnected();
void ws_connect();
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

    // Clear all LEDs before setting team color
    for (int i = 0; i < NUMPIXELS; i++) {
        strip_sk98.setPixelColor(i, 0, 0, 0);
    }
    strip_sk98.show();

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

void manageLeds(JsonObject& buzzer, JsonObject& team, JsonArray& colorArray, JsonObject& message)
{
// Determine game phase with priority (buzzer status, then team status, then game phase)
  const char* game_phase = NULL;
  const char* team_status=team["STATUS"];
  const char* buzzer_Status=buzzer["STATUS"];
  if (buzzer_Status != NULL) {
    game_phase = buzzer_Status;
  } else if (team_status != NULL) {
    game_phase = team_status;
  } else if (message.containsKey("GAME") && message["GAME"].containsKey("PHASE")) {
    game_phase = message["GAME"]["PHASE"];
  } else {
    game_phase = "UNKNOWN"; // Default value if no valid status is found
  }
  
ESP_LOGI(SRV_TAG, "Game Phase=%s",game_phase);

  int current_time=message["GAME"]["CURRENT_TIME"];
  int delay=message["GAME"]["DELAY"];
  const char* button=buzzer["BUTTON"];
  if (strcmp(game_phase,"READY")==0)
  {
    setLedColor(colorArray[0], colorArray[1], colorArray[2],true);
  }
  else if (strcmp(game_phase,"STOP")==0)
  {
    setLedColor(colorArray[0], colorArray[1], colorArray[2]);
    setLedIntensity(255);
  }
  else if (strcmp(game_phase, "START")==0)
  {
    //setLedColor(colorArray[0], colorArray[1], colorArray[2],true);
    setLedIntensity(10);
    
    
    // Fix potential integer division issues with float calculation
    float progress = (float)current_time / delay;
    // Calculate LED position and color for progress indicator
    int led = current_time%(NUMPIXELS/4);
    int nR = 255 * (1.0 - progress);
    int nG = 255 * progress;
    int nB = 0;
    
    ESP_LOGD(SRV_TAG, "set LED %i R=%i G=%i B=%i", led, nR, nG, nB);
    for (int l=1; l<NUMPIXELS; l+=NUMPIXELS/4)
    {
      setPixelColor(l+led, nR, nG, nB);
    }
  }
  else if (strcmp(game_phase, "PAUSE")==0)
  {
    setLedIntensity(64);
    if ( button != NULL )
    {
      ESP_LOGD(SRV_TAG, "Button=%s", button);
      setLedIntensity(255);
      for (int led=0+current_time%2; led<NUMPIXELS; led+=2)
      {
        setPixelColor(led, 64, 64, 64);
      }
    }
  }
  else if (strcmp(game_phase, "PREPARE")==0)
  {
    setLedColor(colorArray[0], colorArray[1], colorArray[2]);
    setLedIntensity(255);
  }
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
    if (!isConfigInitialized) {
      setLedColorExclude(0, 255, 0, 4);
      ESP_LOGI(SRV_TAG, "Boot phase: GREEN 3/4 (HELLO acknowledged)");
      delay(500);
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

  // Update buzzer LED to team color (skip if gray rotation active)
  if (hasTeamAssignment) {
    #ifdef USE_WEBSOCKET
    if (ws_isConnected()) {
      stopGrayRotation();
      setLedColor(r, g, b, true);
    }
    #else
    stopGrayRotation();
    setLedColor(r, g, b, true);
    #endif
  }
  
  if (colorArray.size() == 3) {
    r = colorArray[0];
    g = colorArray[1];
    b = colorArray[2];
  }

  // Process status and intensity
  int intensity = 255;
  if (team.containsKey("STATUS")) {
    String status = team["STATUS"].as<String>();
    if (status == "PAUSE") {
      pauseGame();

      if (team.containsKey("BUMPER")) {
        String macBumper = team["BUMPER"].as<String>();
        if (macBumper == macAddress) {
          ESP_LOGI(SRV_TAG, "BUMP!");
          intensity = 255;
        } else {
          ESP_LOGI(SRV_TAG, "PAUSING");
          intensity = 2;
        }
      }
    }
  }
  
  // Always store the full message for reference
  myCompleteConfig["lastMessage"] = message;
  
  // Apply LED settings
  manageLeds(buzzer, team, colorArray, message);
}


void handleReadyAction(JsonDocument& doc) {
    ESP_LOGI(SRV_TAG, "Received READY - game state updated");

    String myMAC = WiFi.macAddress();
    JsonObject msg = doc["MSG"].as<JsonObject>();

    // Check if buzzer is in bumpers list
    if (msg.containsKey("bumpers") && msg["bumpers"].containsKey(myMAC)) {
        JsonObject myBumper = msg["bumpers"][myMAC].as<JsonObject>();

        if (myBumper.containsKey("TEAM")) {
            String teamName = myBumper["TEAM"].as<String>();

            if (teamName.length() > 0) {
                // Get team color from teams list
                if (msg.containsKey("teams") && msg["teams"].containsKey(teamName)) {
                    JsonObject team = msg["teams"][teamName].as<JsonObject>();

                    if (team.containsKey("COLOR")) {
                        JsonArray colorArray = team["COLOR"].as<JsonArray>();
                        int r = colorArray[0];
                        int g = colorArray[1];
                        int b = colorArray[2];

                        ESP_LOGI(SRV_TAG, "READY - Team %s, color RGB(%d,%d,%d)",
                                 teamName.c_str(), r, g, b);

                        #ifdef USE_WEBSOCKET
                        if (ws_isConnected()) {
                            stopGrayRotation();
                            setLedColor(r, g, b, true);
                        }
                        #else
                        stopGrayRotation();
                        setLedColor(r, g, b, true);
                        #endif
                    }
                }
            } else {
                // TEAM field exists but is empty - buzzer removed from team
                ESP_LOGI(SRV_TAG, "READY - Team field empty, starting gray rotation");
                startGrayRotation();
            }
        } else {
            // No TEAM field - buzzer not assigned to any team
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
      
    default:
      ESP_LOGW(SRV_TAG, "Unknown action: %s", action);
      break;
  }
}

int64_t getAbsoluteTimeMicros() {
  return micros() + ntpOffset;
}

void IRAM_ATTR buttonHandler(void *arg) {
  ButtonInfo* buttonInfo = static_cast<ButtonInfo*>(arg);
//  ESP_LOGI(SRV_TAG, "BUTTON Pressed ");
  if (isGameStarted) {
    buttonInfo->time = String(micros());
    buttonInfo->pressed = true;
  }
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

      buttonsInfo[id].pressed=false;
    }
  }
}
