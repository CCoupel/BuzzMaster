#include <ESP8266WiFi.h>
#include <NTPClient.h>

#include <WiFiClient.h>

#include <ESP8266mDNS.h>
#include <ESPAsyncUDP.h>
#include <ESPAsyncTCP.h>
#include <ESPAsyncWebServer.h>
#include <LittleFS.h>

#include <ArduinoJson.h>

const char* ssid     = "CC-Home";
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
int timeRefTeam[10];

unsigned int localUdpPort = 1234;  // Port d'écoute local
unsigned int localWWWpPort = 80;  // Port d'écoute local

AsyncWebServer  server(localWWWpPort);
AsyncWebSocket ws("/ws");

WiFiUDP ntpUDP;
NTPClient timeClient(ntpUDP, "pool.ntp.org");

AsyncUDP Udp;

JsonDocument teamsAndBumpers;
JsonObject bumpers=teamsAndBumpers["bumpers"].to<JsonObject>();
JsonObject teams=teamsAndBumpers["teams"].to<JsonObject>();
;

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
}

static void IRAM_ATTR buttonHandler(void *arg)
{
    ButtonInfo* buttonInfo = static_cast<ButtonInfo*>(arg);
//    Serial.println("Button " + String(buttonInfo->pin) + " pressed");
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
  //WiFi.mode(WIFI_STA);
  delay(10);
  wifiConnect();
  timeClient.update();

  waitNewBumper();


  if (MDNS.begin("buzzcontrol.local")) {
    Serial.println("MDNS responder started");
  }

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
//  server.handleClient(); // Gestion des requêtes clients
}
