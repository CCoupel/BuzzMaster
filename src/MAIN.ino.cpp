# 1 "C:\\Users\\cyril\\AppData\\Local\\Temp\\tmpys3udys2"
#include <Arduino.h>
# 1 "C:/Users/cyril/OneDrive/Documents/VScode/BuzzMaster/buzzcontrol/src/MAIN.ino"
#if defined(ESP32)
  #include <WiFi.h>
  #include <AsyncTCP.h>
  #include <ESPmDNS.h>
  #include <esp_task_wdt.h>
  #include <Arduino.h>

#elif defined(ESP8266)
  #include <ESP8266WiFi.h>
  #include <ESPAsyncTCP.h>
  #include <ESPAsyncUDP.h>
  #include <ESP8266mDNS.h>
  #include <ESPAsyncWebServer.h>
#endif

#include <ESPAsyncWebServer.h>

#include <NTPClient.h>
#include <WiFiClient.h>

#include <LittleFS.h>

#include <ArduinoJson.h>
#include <map>
const char* ssid = "CC-Home";
const char* password = "GenericPassword";

#if defined(ESP32)
  int ledPin = PIN_NEOPIXEL;
  int rgbPin = RGB_BUILTIN;
  hw_timer_t * timer = NULL;
  portMUX_TYPE timerMux = portMUX_INITIALIZER_UNLOCKED;

#elif defined(ESP8266)
  int ledPin = LED_BUILTIN;
#endif

int currentRed = 0;
int currentGreen = 0;
int currentBlue = 0;
int currentIntensity = 255;

struct ButtonInfo {
  int pin;
  String name;
};

ButtonInfo buttonsInfo[] = {
  {0, "toggle"}
};

volatile bool GameStarted=false;
int timeRef=0;
const int nbTeam=10;
std::map<std::string, int> timeRefTeam;

unsigned int localUdpPort = 1234;
unsigned int localWWWpPort = 80;

AsyncWebServer server(localWWWpPort);
AsyncWebSocket ws("/ws");

WiFiUDP ntpUDP;
NTPClient timeClient(ntpUDP, "pool.ntp.org");



JsonDocument teamsAndBumpers;
void applyLedColor();
void setLedIntensity(int intensity);
void wifiConnect();
void listLittleFSFiles();
void resetBumpersTime();
void attachButtons();
void setup(void);
void onTimerISR();
void loop(void);
void startGame();
void stopGame();
void pauseGame(AsyncClient* client);
void b_handleData(void* arg, AsyncClient* c, void *data, size_t len);
static void b_onClientDisconnect(void* arg, AsyncClient* client);
static void b_onCLientConnect(void* arg, AsyncClient* client);
void startBumperServer();
void sendMessageToClient(const String& action, const String& msg, AsyncClient* client);
void sendMessageToAllClients(const String& action, const String& msg );
void checkPingForAllClients();
void w_handleRoot(AsyncWebServerRequest *request);
void w_handle_bumpers(AsyncWebServerRequest *request);
void w_handle_teams(AsyncWebServerRequest *request);
void w_inline(AsyncWebServerRequest *request);
void w_handleNotFound(AsyncWebServerRequest *request);
void startWebServer();
void onWsEvent(AsyncWebSocket *server, AsyncWebSocketClient *client, AwsEventType type, void *arg, uint8_t *data, size_t len);
void parseDataFromSocket(const char* action, JsonObject message);
void mergeJson(JsonObject& destObj, const JsonObject& srcObj);
void update(String action, JsonObject& obj);
void notifyAll();
void loadJson(String path);
void saveJson();
#line 71 "C:/Users/cyril/OneDrive/Documents/VScode/BuzzMaster/buzzcontrol/src/MAIN.ino"
void applyLedColor() {
  int adjustedRed = (currentRed * currentIntensity) / 255;
  int adjustedGreen = (currentGreen * currentIntensity) / 255;
  int adjustedBlue = (currentBlue * currentIntensity) / 255;

#if defined(ESP32)
  neopixelWrite(rgbPin,adjustedRed,adjustedGreen,adjustedBlue);
  int pwmValue = 255 - currentIntensity;


#elif defined(ESP8266)
  int pwmValue = 255 - currentIntensity;
  analogWrite(LED_BUILTIN, pwmValue);
#endif
}

void setLedColor(int red, int green, int blue, bool isApplyLedColor=false) {

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

void wifiConnect()
{
  Serial.println();
  Serial.print("Connexion à ");
  Serial.println(ssid);

  WiFi.begin(ssid, password);

  while (WiFi.status() != WL_CONNECTED)
  {
    delay(500);
    Serial.print(".");
  }

  Serial.println("");
  Serial.println("WiFi connecté.");
  Serial.print("Adresse IP: ");
  Serial.println(WiFi.localIP());
  Serial.print("Adresse MAC: ");
  Serial.println(WiFi.macAddress());
}

void listLittleFSFiles() {
    Serial.println("Listing files in LittleFS:");
  #if defined(ESP8266)
    Dir dir = LittleFS.openDir("/");
    while (dir.next()) {
        String fileName = dir.fileName();
        size_t fileSize = dir.fileSize();
        Serial.printf("FILE: %s, SIZE: %d bytes\n", fileName.c_str(), fileSize);
    }
  #elif defined(ESP32)
    File root = LittleFS.open("/");
    if (!root) {
        Serial.println("- failed to open directory");
        return;
    }
    if (!root.isDirectory()) {
        Serial.println(" - not a directory");
        return;
    }

    File file = root.openNextFile();
    while (file) {
        if (file.isDirectory()) {
            Serial.print("DIR : ");
            Serial.println(file.name());
        } else {
            Serial.print("FILE: ");
            Serial.print(file.name());
            Serial.print("\tSIZE: ");
            Serial.println(file.size());
        }
        file = root.openNextFile();
    }
#endif
}

void resetBumpersTime() {
  JsonObject bumpers=teamsAndBumpers["bumpers"];
  JsonObject teams=teamsAndBumpers["teams"];
  for (JsonPair kvp : bumpers) {
    if (kvp.value().is<JsonObject>()) {
        JsonObject bumper = kvp.value().as<JsonObject>();
        bumper["TIME"] = 0;
        bumper["BUTTON"] = 0;
        bumper["DELAY"] = 0;
        bumper["DELAY_TEAM"] = 0;

        } else {
            Serial.println("Error: Bumper entry is not a JsonObject");
        }
  }
  for (JsonPair team : teams) {
    JsonObject teamData = team.value().as<JsonObject>();

    if (teamData.containsKey("STATUS")) {
        teamData.remove("STATUS");
    }


    if (teamData.containsKey("BUMPER")) {
        teamData.remove("BUMPER");
    }
  }
  timeRef=0;
  for (const auto& pair : timeRefTeam) {
    timeRefTeam[pair.first]=0;
  }
}

static void IRAM_ATTR buttonHandler(void *arg)
{
    ButtonInfo* buttonInfo = static_cast<ButtonInfo*>(arg);

    digitalWrite(ledPin, LOW);
    switch(buttonInfo->pin) {
      case 0:
        if (GameStarted) {
          stopGame();
        }
        else {
          startGame();
        }
        break;
    };
}

void attachButtons()
{
  for (size_t id = 0; id < sizeof(buttonsInfo) / sizeof(ButtonInfo); id++)
  {
    pinMode(buttonsInfo[id].pin, INPUT_PULLUP);
    attachInterruptArg(digitalPinToInterrupt(buttonsInfo[id].pin),reinterpret_cast<void (*)(void*)>(buttonHandler), &buttonsInfo[id],FALLING);
    Serial.println("Button " + String(id) + " (" + buttonsInfo[id].name + ") attached to pin " + String(buttonsInfo[id].pin));
  }
}

void setup(void)
{
  Serial.begin(115200);

#if defined(ESP32)

#elif defined(ESP8266)

  pinMode(LED_BUILTIN, OUTPUT);
#endif

  #if defined(ESP32)
    Serial.print("RGB pin:");
    Serial.println(RGB_BUILTIN);
  #endif
  Serial.print("LED pin:");
  Serial.println(LED_BUILTIN);
  #if defined(ESP32)
    Serial.print("NEO pin:");
    Serial.println(PIN_NEOPIXEL);
  #endif

  setLedColor(255, 0, 0);
  setLedIntensity(128);

  delay(10);
  wifiConnect();
  timeClient.update();




  if (MDNS.begin("buzzcontrol")) {
    Serial.println("MDNS responder started");
  }
  MDNS.addService("buzzcontrol", "tcp", localWWWpPort);
  MDNS.addService("http", "tcp", localWWWpPort);
  MDNS.addService("sock", "tcp", localUdpPort);

  if (!LittleFS.begin()) {
        Serial.println("Erreur de montage LittleFS");
        return;
    }

  listLittleFSFiles();

  attachButtons();
  loadJson("/game.json");

  startWebServer();
  startBumperServer();
  setLedColor(0, 255, 0);
  setLedIntensity(64);
# 288 "C:/Users/cyril/OneDrive/Documents/VScode/BuzzMaster/buzzcontrol/src/MAIN.ino"
}

void onTimerISR() {
  sendMessageToAllClients("PING", "'Are you alive?'");
  checkPingForAllClients();
}


void loop(void)
{
#if defined(ESP8266)

  MDNS.update();
#endif
}
# 1 "C:/Users/cyril/OneDrive/Documents/VScode/BuzzMaster/buzzcontrol/src/BumperServer.ino"







AsyncServer* bumperServer;
std::vector<AsyncClient*> bumperClients;



void startGame(){

  resetBumpersTime();
  GameStarted=true;
  digitalWrite(ledPin, LOW);
  sendMessageToAllClients("START","''");
  notifyAll();
}

void stopGame(){
  GameStarted=false;
  digitalWrite(ledPin, HIGH);
  sendMessageToAllClients("STOP","''");
  notifyAll();
}

void pauseGame(AsyncClient* client) {
    sendMessageToClient("PAUSE", "''", client);
}


void b_handleData(void* arg, AsyncClient* c, void *data, size_t len) {
  JsonDocument receivedData;

  String s_data=String((char*)data).substring(0, len);
  String action;
  String bumperID;
  JsonObject MSG;
  JsonObject bumpers=teamsAndBumpers["bumpers"];
  JsonObject teams=teamsAndBumpers["teams"];

  Serial.print("BUZZER: Data received: ");
  Serial.println(s_data);
  deserializeJson(receivedData, data);

        JsonObject jsonObjData = receivedData.as<JsonObject>();

  bumperID=jsonObjData["ID"].as<String>();
  action=jsonObjData["ACTION"].as<String>();
  MSG=jsonObjData["MSG"];

  Serial.println("bumperID="+bumperID+" ACTION="+action);



  if (action == "HELLO") {




      if ( !bumpers[bumperID].containsKey("NAME") ) {
        bumpers[bumperID]["NAME"]=MSG["NAME"];
      };
      if ( !bumpers[bumperID].containsKey("TEAM") ) {
        bumpers[bumperID]["TEAM"]="1";
      };
      bumpers[bumperID]["IP"]=MSG["IP"];


      notifyAll();
  };
  if (action == "BUTTON") {
    if ( bumpers.containsKey(bumperID) ) {

      if ( bumpers[bumperID].containsKey("IP") && bumpers[bumperID]["IP"] == c->remoteIP().toString()) {
        const char * b_team=bumpers[bumperID]["TEAM"];
        int b_time=MSG["time"];
        int b_button=MSG["button"];
        if ( b_team != nullptr ) {

          if (timeRef==0) {
            timeRef=b_time;
          }
          int b_delay=b_time-timeRef;

         if (timeRefTeam[b_team]==0) {
            timeRefTeam[b_team]=b_time;
          }

          int b_delayTeam=b_time-timeRefTeam[b_team];
          bumpers[bumperID]["TIME"]=b_time;
          bumpers[bumperID]["BUTTON"]=b_button;
          bumpers[bumperID]["DELAY"]=b_delay;
          bumpers[bumperID]["DELAY_TEAM"]=b_delayTeam;

          teams[b_team]["TIME"]=b_time;
          teams[b_team]["DELAY"]=b_delay;
          teams[b_team]["STATUS"]="PAUSE";
          teams[b_team]["BUMPER"]=bumperID;


          notifyAll();

        };
      };
    };
  };
  if (action == "PING") {
    bumpers[bumperID]["lastPingTime"] = millis();

    if (bumpers[bumperID]["STATUS"] != "online") {
      Serial.println("BUMPER: Bumper is going online");
      bumpers[bumperID]["STATUS"] = "online";
      notifyAll();
    }
  }

};

static void b_onClientDisconnect(void* arg, AsyncClient* client) {
  JsonObject bumpers = teamsAndBumpers["bumpers"];

  Serial.printf("Client disconnected\n");
  for (JsonPair bumperPair : bumpers) {
    JsonObject bumper = bumperPair.value().as<JsonObject>();
    if (bumper["IP"].as<String>() == client->remoteIP().toString()) {
      bumper["STATUS"] = "offline";
      notifyAll();
      break;
    }
  }

  bumperClients.erase(std::remove(bumperClients.begin(), bumperClients.end(), client), bumperClients.end());
  delete client;

}


static void b_onCLientConnect(void* arg, AsyncClient* client) {
  Serial.println("Buzzer: New client connected");
  client->onData(&b_handleData, NULL);
  client->onDisconnect(&b_onClientDisconnect, NULL);
  bumperClients.push_back(client);

}

void startBumperServer()
{
  bumperServer = new AsyncServer(1234);
  bumperServer->onClient(&b_onCLientConnect, bumperServer);

  bumperServer->begin();
    Serial.print("BUMPER server started on port ");
  Serial.println(1234);

}

void sendMessageToClient(const String& action, const String& msg, AsyncClient* client) {
Serial.println("BUMPER: send to "+client->remoteIP().toString());
Serial.println("BUMPER: action "+action);
Serial.println("BUMPER: message "+msg);
String message="{ \"ACTION\": \"" + action + "\", \"MSG\": " + msg + "}\n";
    if (client && client->connected()) {
      client->write(message.c_str(), message.length());
    }
}

void sendMessageToAllClients(const String& action, const String& msg ) {

  Serial.println("BUMPER: send to all");
  for (AsyncClient* client : bumperClients) {
    sendMessageToClient(action, msg, client);
  }
}

void checkPingForAllClients() {
  unsigned long currentTime = millis();


  JsonObject bumpers = teamsAndBumpers["bumpers"];
  for (JsonPair bumperPair : bumpers) {
    JsonObject bumper = bumperPair.value().as<JsonObject>();
    if (currentTime - bumper["lastPingTime"].as<unsigned long>() > 3000) {
      if (bumper["STATUS"] != "offline") {
        Serial.print("BUMPER: Bumper ");
        Serial.print(bumper["IP"].as<String>());
        Serial.println(" is going offline");
        bumper["STATUS"] = "offline";
        notifyAll();
      }
    }
  }
}
# 1 "C:/Users/cyril/OneDrive/Documents/VScode/BuzzMaster/buzzcontrol/src/WaitBumper.ino"
# 1 "C:/Users/cyril/OneDrive/Documents/VScode/BuzzMaster/buzzcontrol/src/WebServer.ino"
# 9 "C:/Users/cyril/OneDrive/Documents/VScode/BuzzMaster/buzzcontrol/src/WebServer.ino"
void w_handleRoot(AsyncWebServerRequest *request) {
  Serial.println("WWW: ROOT");
  digitalWrite(ledPin, HIGH);
  request->send(200, "text/plain", "hello from esp8266!");
  digitalWrite(ledPin, LOW);
}


void w_handle_bumpers(AsyncWebServerRequest *request) {
  JsonObject bumpers=teamsAndBumpers["bumpers"];
  JsonObject teams=teamsAndBumpers["teams"];
  String output;
    serializeJson(bumpers, output);

  request->send(200, "text/json", output);
}

void w_handle_teams(AsyncWebServerRequest *request) {
  JsonObject bumpers=teamsAndBumpers["bumpers"];
  JsonObject teams=teamsAndBumpers["teams"];
  String output;
    serializeJson(teams, output);

  request->send(200, "text/json", output);
}

void w_inline(AsyncWebServerRequest *request) {
  Serial.println("WWW: inline");
  request->send(200, "text/plain", "this works as well");
}


void w_handleNotFound(AsyncWebServerRequest *request) {
  Serial.println("WWW: not found");
  digitalWrite(ledPin, HIGH);
  String message = "File Not Found\n\n";
  message += "URI: ";
  message += request->url();
  message += "\nMethod: ";
  message += (request->method() == HTTP_GET)?"GET":"POST";
  message += "\nArguments: ";
  message += request->args();
  message += "\n";
  for (uint8_t i=0; i<request->args(); i++) {
    message += " " + request->argName(i) + ": " + request->arg(i) + "\n";
  }
  request->send(404, "text/plain", message);
  digitalWrite(ledPin, LOW);
}

void startWebServer() {
  server.serveStatic("/", LittleFS, "/").setDefaultFile("index.html");

  server.on("/bumpers", HTTP_GET, w_handle_bumpers);

  server.on("/teams", HTTP_GET, w_handle_teams);


  server.onNotFound(w_handleNotFound);

  ws.onEvent(onWsEvent);
  server.addHandler(&ws);

  server.begin();
  Serial.println("HTTP server started");
}
# 1 "C:/Users/cyril/OneDrive/Documents/VScode/BuzzMaster/buzzcontrol/src/WebSocket.ino"
void onWsEvent(AsyncWebSocket *server, AsyncWebSocketClient *client, AwsEventType type, void *arg, uint8_t *data, size_t len) {
    String output;
    JsonDocument receivedData;
    if (type == WS_EVT_CONNECT) {

        Serial.printf("SOCK: Client %u connecté\n", client->id());

    } else if (type == WS_EVT_DISCONNECT) {

        Serial.printf("SOCK: Client %u déconnecté\n", client->id());
    } else if (type == WS_EVT_DATA) {


        Serial.printf("SOCK: Message reçu de client %u : %.*s\n", client->id(), len, data);
        DeserializationError error = deserializeJson(receivedData, data);
        if (error) {
          Serial.print("Failed to parse JSON: ");
          Serial.println(error.f_str());
          return;
      }


        const char* action = receivedData["ACTION"];
        JsonObject message = receivedData["MSG"];
        parseDataFromSocket(action, message);
    }
}

void parseDataFromSocket(const char* action, JsonObject message) {

        if (strcmp(action, "HELLO") == 0) {
                notifyAll();
        }
        if (strcmp(action, "FULL") == 0) {
          JsonDocument& doc = teamsAndBumpers;
          doc=message;
        }
        if (strcmp(action, "UPDATE") == 0) {
          update("UPDATE", message);
        }
        if (strcmp(action, "DELETE") == 0) {
          update("DELETE", message);
        }
        if (strcmp(action, "RESET") == 0) {
          Serial.printf("SOCK: Rebooting....");
          ESP.restart();
        }
}


void mergeJson(JsonObject& destObj, const JsonObject& srcObj) {


  String msg="Merging :";
  for (JsonPair kvp : srcObj) {
      if (kvp.value().is<JsonObject>()) {
        JsonObject nestedDestObj;
        if (destObj.containsKey(kvp.key()) && destObj[kvp.key()].is<JsonObject>()) {
                nestedDestObj = destObj[kvp.key()].as<JsonObject>();
            } else {

                nestedDestObj = destObj[kvp.key()].to<JsonObject>();
            }


        JsonObject nestedSrcObj = kvp.value().as<JsonObject>();
        mergeJson(nestedDestObj, nestedSrcObj );
      }
      else {
        destObj[kvp.key()] = kvp.value();
      }
    }

}

void update(String action, JsonObject& obj) {
    JsonObject jsonObj = teamsAndBumpers.as<JsonObject>();
    String output;
      serializeJson(obj, output);



    Serial.printf("Update: received %s\n", output.c_str());

    mergeJson(jsonObj, obj);
    serializeJson(jsonObj, output);

    Serial.printf("Update: complete %s\n", output.c_str());

    notifyAll();
}

void notifyAll() {
  String output;
  serializeJson(teamsAndBumpers, output);
  saveJson();
  Serial.printf("SOCK: send to all %s\n", output.c_str());
  ws.textAll(output.c_str());
  sendMessageToAllClients("UPDATE", output.c_str() );
}

void loadJson(String path) {
  String output;

  File file = LittleFS.open(path, "r");
  if (!file) {
    Serial.println("Failed to open file for reading. Initializing with default values.");

    File file = LittleFS.open(path+".save", "r");
    if (!file) {
      Serial.println("Failed to open file for reading. Initializing with default values.");

      teamsAndBumpers["bumpers"] = JsonObject();
      teamsAndBumpers["teams"] = JsonObject();
      return;
    }
  }


  DeserializationError error = deserializeJson(teamsAndBumpers, file);
  if (error) {
    Serial.print("deserializeJson() failed: ");
    Serial.println(error.f_str());

    teamsAndBumpers["bumpers"] = JsonObject();
    teamsAndBumpers["teams"] = JsonObject();
  } else {
    Serial.println("JSON loaded successfully");
  }


  file.close();
  serializeJson(teamsAndBumpers, output);
  Serial.printf("JSON: loaded %s\n", output.c_str());
}



void saveJson() {

  File file = LittleFS.open("/game.json.save", "w");
  if (!file) {
    Serial.println("Failed to open file for writing");
    return;
  }


  if (serializeJson(teamsAndBumpers, file) == 0) {
    Serial.println("Failed to write to file");
  }


  file.close();

  Serial.println("JSON saved successfully");
}