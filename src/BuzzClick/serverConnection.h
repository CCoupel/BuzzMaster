#pragma once
#include "includes.h"
#include "Common/CustomLogger.h"
#include "Common/led.h"

#include <esp_timer.h>
#include <AsyncUDP.h>

String myConfig="{ }";
static const char* SRV_TAG = "ServerConnection";
uint64_t lastNTPUpdate = 0;
int64_t ntpOffset = 0;

bool isGameStarted = false;

//const int CONTROLER_PORT = 1234;

AsyncUDP udp;

String BcastJsonBuffer = "";


/* SOCKET */

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

void onConnect(void* arg, AsyncClient* c) {
  ESP_LOGI(SRV_TAG, "Connected to server: %s:%d", c->remoteIP().toString().c_str(), c->remotePort());
  hello_bumper();
}

void onData(void* arg, AsyncClient* c, void* data, size_t len) {
/*  String s_data = String((char*)data).substring(0, len);
  ESP_LOGI(SRV_TAG, "DATA received: %s", s_data.c_str());
  
  jsonBuffer += s_data;
  int endOfJson;
  while ((endOfJson = jsonBuffer.indexOf('\n')) > 0) {
    String jsonPart = jsonBuffer.substring(0, endOfJson);
    jsonBuffer = jsonBuffer.substring(endOfJson + 1);
    parseJSON(jsonPart, c);
  }
  */
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

void onDataBroadcast(AsyncUDPPacket packet) {
  if (packet.length() > 0) {
    String s_data = String((char*)packet.data(), packet.length());
    ESP_LOGI(SRV_TAG, "Broadcast data received from %s: %i => %s", packet.remoteIP().toString().c_str(), packet.length(), s_data.c_str());
    
    BcastJsonBuffer += s_data;
    int endOfJson;
    while ((endOfJson = BcastJsonBuffer.indexOf('\0')) > 0) {
      String jsonPart = BcastJsonBuffer.substring(0, endOfJson);
      BcastJsonBuffer = BcastJsonBuffer.substring(endOfJson + 2);
      ESP_LOGI(SRV_TAG,"Broadcast MSG complete at %i : %s", endOfJson, jsonPart.c_str()); 

      parseJSON(jsonPart, nullptr);  // Passer nullptr car nous n'avons pas de client AsyncClient ici
    }
  }
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
  

  int current_time=message["GAME"]["CURRENT_TIME"];
  int delay=message["GAME"]["DELAY"];
  const char* button=buzzer["BUTTON"];
  if (strcmp(game_phase,"READY")==0)
  {
    setLedColor(colorArray[0], colorArray[1], colorArray[2],true);
  }
  else if (strcmp(game_phase,"STOP")==0)
  {
    ESP_LOGI(SRV_TAG, "Game Phase=STOP");
    setLedIntensity(255);
  }
  else if (strcmp(game_phase, "START")==0)
  {
    ESP_LOGI(SRV_TAG, "Game Phase=START");
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
    ESP_LOGI(SRV_TAG, "Game Phase=PAUSE");
    setLedIntensity(64);
    if ( button != NULL )
    {
      ESP_LOGD(SRV_TAG, "Button=%s", button);
      for (int led=0+current_time%2; led<NUMPIXELS; led+=2)
      {
        setPixelColor(led, 64, 64, 64);
      }
    }
  }
  else if (strcmp(game_phase, "PREPARE")==0)
  {
    ESP_LOGI(SRV_TAG, "Game Phase=PREPARE");
    setLedColor(0, 0, 0,true);
  }
}

void handleUpdateAction(JsonObject& message, const String& macAddress) {
  JsonObject buzzer;
    JsonObject team;
    String output;
    JsonDocument JsonDoc;
    JsonArray colorArray=JsonDoc.to<JsonArray>();
    colorArray.add(0);
    colorArray.add(0);
    colorArray.add(0);

  if ( message.containsKey("bumpers") ) {
    if ( message["bumpers"].containsKey(macAddress) ) {
      buzzer = message["bumpers"][macAddress].as<JsonObject>();
      serializeJson(buzzer, output);
      ESP_LOGI(SRV_TAG, "My Config=%s",output.c_str());
      myConfig=output;
    }
  }

  const char * t_name="";
  if (buzzer.containsKey("TEAM") ) {
    t_name=buzzer["TEAM"];
    ESP_LOGI(SRV_TAG, "My team=%s",t_name);
    if ( message.containsKey("teams") ) {
      if (message["teams"].containsKey(t_name) ) {
        team = message["teams"][t_name];
        serializeJson(team, output);
        ESP_LOGI(SRV_TAG, "    =>%s",output.c_str());
      }
    }
  }

  int r = 0;
  int g = 0;
  int b = 0;

  if (team.containsKey("COLOR")) {
    colorArray = team["COLOR"].as<JsonArray>();
  }
  if (team.containsKey("color")) {
    colorArray = team["color"].as<JsonArray>();
  }
  // S'assurer que le tableau contient bien 3 éléments (R, G, B)
  if (colorArray.size() == 3) 
  {
      r = colorArray[0];
      g = colorArray[1];
      b = colorArray[2];

  }
  // Change la couleur de la LED
//  setLedColor(r, g, b,true);

  int intensity=255;
  if (team.containsKey("STATUS")) {
    String status=team["STATUS"].as<String>();
    if (status == "PAUSE") {
      pauseGame();

      if (team.containsKey("BUMPER")) {
        String macBumper=team["BUMPER"].as<String>();
        if ( macBumper == macAddress) {
          ESP_LOGI(SRV_TAG, "BUMP!");
          intensity=255;
        }
        else {
          ESP_LOGI(SRV_TAG, "PAUSING");
          intensity=2;
        }
      }
    }
  }
//  setLedIntensity(intensity);
  manageLeds(buzzer, team, colorArray, message);
}

void hello_bumper()
{
  sendMSG("HELLO", myConfig);
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
  if (strcmp(action, "START") == 0) {
    ESP_LOGI(SRV_TAG, "STARTING");
    startGame();
  } else if (strcmp(action, "STOP") == 0) {
    ESP_LOGI(SRV_TAG, "STOPPING");
    stopGame();
  } else if (strcmp(action, "PAUSE") == 0) {
    ESP_LOGI(SRV_TAG, "PAUSING");
    pauseGame();
  } else if (strcmp(action, "CONTINUE") == 0) {
    ESP_LOGI(SRV_TAG, "STARTING");
    startGame();
  } else if (strcmp(action, "PING") == 0) {
    ESP_LOGI(SRV_TAG, "Replying PONG");
    sendMSG("PONG", "'" + WiFi.localIP().toString() + "'");
  } else if (strcmp(action, "UPDATE") == 0) {
    ESP_LOGI(SRV_TAG, "Updating My Config: %s", WiFi.macAddress().c_str());
    handleUpdateAction(message, WiFi.macAddress());
  } else if (strcmp(action, "HELLO") == 0) {
    ESP_LOGI(SRV_TAG, "Send HELLO to COntroler");
    connectSRV();
  }else if (strcmp(action, "RESET") == 0) {
    ESP_LOGI(SRV_TAG, "REseting Datas");
    resetGame();
  }else if (strcmp(action, "UPDATE_TIMER") == 0) {
    ESP_LOGI(SRV_TAG, "UPDATING TIMER");
    handleUpdateAction(message, WiFi.macAddress());
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
    pinMode(buttonsInfo[id].pin+1, OUTPUT); 
    digitalWrite(buttonsInfo[id].pin+1, LOW); 

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
      JsonDocument doc;  // Adjust size as needed
      doc["button"] = buttonsInfo[id].name;

      String msg;
      if (serializeJson(doc, msg) == 0) {
        ESP_LOGE(SRV_TAG, "Failed to serialize JSON for button %zu", id);
        continue;  // Skip to next iteration if serialization fails
      }

      ESP_LOGI(SRV_TAG, "Button pressed: %s", msg.c_str()); 

      sendMSG("BUTTON", msg);

      buttonsInfo[id].pressed=false;
    }
  }
}
