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

const int led = 0;
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

void setup(void)
{
  Serial.begin(115200);

  pinMode(led, OUTPUT);
  digitalWrite(led, 0);
  //WiFi.mode(WIFI_STA);
  delay(10);
  wifiConnect();
  timeClient.update();

  waitNewBumper();


  if (MDNS.begin("esp8266")) {
    Serial.println("MDNS responder started");
  }

  if (!LittleFS.begin()) {
        Serial.println("Erreur de montage LittleFS");
        return;
    }

  listLittleFSFiles();


  startWebServer();
  startBumperServer();
}

void loop(void)
{
//  server.handleClient(); // Gestion des requêtes clients
}
