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
/*
// LED 
void applyLedColor() {
  int adjustedRed = (currentRed * currentIntensity) / 255;
  int adjustedGreen = (currentGreen * currentIntensity) / 255;
  int adjustedBlue = (currentBlue * currentIntensity) / 255;

  neopixelWrite(rgbPin,adjustedRed,adjustedGreen,adjustedBlue);
  int pwmValue = 255 - currentIntensity;

}

void setLedColor(int red, int green, int blue, bool isApplyLedColor) {

  currentRed = red;
  currentGreen = green;
  currentBlue = blue;
  if (isApplyLedColor) {
    applyLedColor();
  }
}

void setLedIntensity(int intensity) {
  currentIntensity = intensity;
  applyLedColor();
}
*/

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

/*
void sendPing() {
  if (client->connected()) {
    sendMSG("PING", "''");
  } else {
    ESP_LOGW(SRV_TAG, "Connection lost. Attempting to reconnect...");
    connectSRV();  // Tenter de se reconnecter
  }
}


bool checkConnection() {
  if (!client->connected()) {
    ESP_LOGW(SRV_TAG, "Connection lost. Attempting to reconnect...");
    return connectSRV();
  } else {
    sendPing();
    return true;
  }
}
*/

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
  String s_data = String((char*)data).substring(0, len);
  ESP_LOGI(SRV_TAG, "DATA received: %s", s_data.c_str());
  
  jsonBuffer += s_data;
  int endOfJson;
  while ((endOfJson = jsonBuffer.indexOf('\n')) > 0) {
    String jsonPart = jsonBuffer.substring(0, endOfJson);
    jsonBuffer = jsonBuffer.substring(endOfJson + 1);
    parseJSON(jsonPart, c);
  }
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
    while ((endOfJson = BcastJsonBuffer.indexOf('\n')) > 0) {
      String jsonPart = BcastJsonBuffer.substring(0, endOfJson);
      BcastJsonBuffer = BcastJsonBuffer.substring(endOfJson + 1);
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
  setLedIntensity(10);
}

void stopGame() {
  isGameStarted=false;
  setLedIntensity(255);

}

void pauseGame() {
  isGameStarted=false;
  setLedIntensity(128);
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
  setLedColor(r, g, b,true);

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
  setLedIntensity(intensity);
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
  }

}

int64_t getAbsoluteTimeMicros() {
  return micros() + ntpOffset;
}

void IRAM_ATTR buttonHandler(void *arg) {
  ButtonInfo* buttonInfo = static_cast<ButtonInfo*>(arg);

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
