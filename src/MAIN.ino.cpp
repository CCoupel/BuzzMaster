# 1 "C:\\Users\\cyril\\AppData\\Local\\Temp\\tmpitk5o3lg"
#include <Arduino.h>
# 1 "C:/Users/cyril/OneDrive/Documents/VScode/BuzzMaster/buzzcontrol/src/MAIN.ino"
#include <ESP8266WiFi.h>
#include <NTPClient.h>

#include <WiFiClient.h>

#include <ESP8266mDNS.h>
#include <ESPAsyncUDP.h>
#include <ESPAsyncTCP.h>
#include <ESPAsyncWebServer.h>
#include <LittleFS.h>

#include <ArduinoJson.h>

const char* ssid = "CC-Home";
const char* password = "GenericPassword";

int ledPin = LED_BUILTIN;
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
int timeRefTeam[nbTeam];

unsigned int localUdpPort = 1234;
unsigned int localWWWpPort = 80;

AsyncWebServer server(localWWWpPort);
AsyncWebSocket ws("/ws");

WiFiUDP ntpUDP;
NTPClient timeClient(ntpUDP, "pool.ntp.org");

AsyncUDP Udp;

JsonDocument teamsAndBumpers;
JsonObject bumpers=teamsAndBumpers["bumpers"].to<JsonObject>();
JsonObject teams=teamsAndBumpers["teams"].to<JsonObject>();
;
void wifiConnect();
void listLittleFSFiles();
void resetBumpersTime();
void attachButtons();
void setup(void);
void loop(void);
void startGame();
void stopGame();
void stopGameClient(AsyncClient* client);
void b_handleData(void* arg, AsyncClient* c, void *data, size_t len);
static void b_onClientDisconnect(void* arg, AsyncClient* client);
static void b_onCLientConnect(void* arg, AsyncClient* client);
void startBumperServer();
void sendMessageToClient(const String& message, AsyncClient* client);
void sendMessageToAllClients(const String& message);
void w_handleRoot(AsyncWebServerRequest *request);
void w_handle_bumpers(AsyncWebServerRequest *request);
void w_handle_teams(AsyncWebServerRequest *request);
void w_inline(AsyncWebServerRequest *request);
void w_handleNotFound(AsyncWebServerRequest *request);
void startWebServer();
void onWsEvent(AsyncWebSocket *server, AsyncWebSocketClient *client, AwsEventType type, void *arg, uint8_t *data, size_t len);
void mergeJson(JsonObject& destObj, const JsonObject& srcObj);
void update(String action, JsonObject& obj);
void notifyAll();
#line 48 "C:/Users/cyril/OneDrive/Documents/VScode/BuzzMaster/buzzcontrol/src/MAIN.ino"
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
    Dir dir = LittleFS.openDir("/");
    while (dir.next()) {
        String fileName = dir.fileName();
        size_t fileSize = dir.fileSize();
        Serial.printf("FILE: %s, SIZE: %d bytes\n", fileName.c_str(), fileSize);
    }
}

void resetBumpersTime() {
  for (JsonPair kvp : bumpers) {
    if (kvp.value().is<JsonObject>()) {
            JsonObject bumper = kvp.value().as<JsonObject>();
            bumper["TIME"] = 0;
            bumper["BUTTON"] = 0;
        } else {
            Serial.println("Error: Bumper entry is not a JsonObject");
        }
  }
  timeRef=0;
  for (int i=0; i<nbTeam; i++) {
    timeRefTeam[i]=0;
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

  pinMode(ledPin, OUTPUT);
  digitalWrite(ledPin, LOW);

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

  startWebServer();
  startBumperServer();
  digitalWrite(ledPin, HIGH);
}

void loop(void)
{

  MDNS.update();
}
# 1 "C:/Users/cyril/OneDrive/Documents/VScode/BuzzMaster/buzzcontrol/src/BumperServer.ino"







AsyncServer* bumperServer;
std::vector<AsyncClient*> bumperClients;



void startGame(){
  resetBumpersTime();
  GameStarted=true;
  digitalWrite(ledPin, LOW);
  sendMessageToAllClients("START");
  notifyAll();
}

void stopGame(){
  GameStarted=false;
  digitalWrite(ledPin, HIGH);
  sendMessageToAllClients("STOP");
  notifyAll();
}

void stopGameClient(AsyncClient* client) {
    sendMessageToClient("STOP", client);
}


void b_handleData(void* arg, AsyncClient* c, void *data, size_t len) {
  JsonDocument receivedData;
  JsonDocument obj;
  String s_data=String((char*)data).substring(0, len);
  String action;
  String bumperID;
  JsonObject MSG;

  Serial.print("BUZZER: Data received: ");
  Serial.println(s_data);
  deserializeJson(receivedData, data);

        JsonObject jsonObjData = receivedData.as<JsonObject>();

  bumperID=jsonObjData["ID"].as<String>();
  action=jsonObjData["ACTION"].as<String>();
  MSG=jsonObjData["MSG"];

  Serial.println("bumperID="+bumperID+" ACTION="+action);

  JsonObject subObj=obj["root"].to<JsonObject>();

  if (action == "HELLO") {
      subObj["bumpers"][bumperID]=MSG;



      if ( !bumpers[bumperID].containsKey("NAME") ) {
        bumpers[bumperID]["NAME"]="";
      };
      if ( !bumpers[bumperID].containsKey("TEAM") ) {
        bumpers[bumperID]["TEAM"]=1;
      };
      bumpers[bumperID]["IP"]=MSG["IP"];


      notifyAll();
  };
  if (action == "BUTTON") {
    if ( !bumpers[bumperID].containsKey(bumperID) ) {
      if ( bumpers[bumperID]["IP"] == c->remoteIP().toString()) {
        int b_team=bumpers[bumperID]["TEAM"];
        int b_time=MSG["time"];
        int b_button=MSG["button"];
        if ( b_team != 0 ) {

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

          notifyAll();
          stopGameClient(c);
        };
      };
    };
  };
};

static void b_onClientDisconnect(void* arg, AsyncClient* client) {
  Serial.printf("Client disconnected\n");
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

void sendMessageToClient(const String& message, AsyncClient* client) {
Serial.println("BUMPER: send to "+client->remoteIP().toString());
Serial.println("BUMPER: message "+message);

    if (client && client->connected()) {
      client->write(message.c_str(), message.length());
    }
}

void sendMessageToAllClients(const String& message) {

  Serial.println("BUMPER: send to all");
  for (AsyncClient* client : bumperClients) {
    sendMessageToClient(message, client);
  }
}
# 1 "C:/Users/cyril/OneDrive/Documents/VScode/BuzzMaster/buzzcontrol/src/WaitBumper.ino"
# 1 "C:/Users/cyril/OneDrive/Documents/VScode/BuzzMaster/buzzcontrol/src/WebServer.ino"





void w_handleRoot(AsyncWebServerRequest *request) {
  Serial.println("WWW: ROOT");
  digitalWrite(ledPin, HIGH);
  request->send(200, "text/plain", "hello from esp8266!");
  digitalWrite(ledPin, LOW);
}


void w_handle_bumpers(AsyncWebServerRequest *request) {
  String output;
    serializeJson(bumpers, output);

  request->send(200, "text/json", output);
}

void w_handle_teams(AsyncWebServerRequest *request) {
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
        notifyAll();

    } else if (type == WS_EVT_DISCONNECT) {

        Serial.printf("SOCK: Client %u déconnecté\n", client->id());
    } else if (type == WS_EVT_DATA) {


        Serial.printf("SOCK: Message reçu de client %u : %.*s\n", client->id(), len, data);

        deserializeJson(receivedData, data);

        JsonObject jsonObjData = receivedData.as<JsonObject>();

        update("ADD", jsonObjData);

       notifyAll();
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
  Serial.printf("SOCK: send to all %s\n", output.c_str());
  ws.textAll(output.c_str());
}