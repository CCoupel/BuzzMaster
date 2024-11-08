#pragma once
#include "common/Constant.h"

  #include <WiFi.h>
  #include <AsyncTCP.h>
  #include <ESPmDNS.h>
#include "esp_log.h"

#include <WiFiUDP.h>
//#include <NTPClient.h>
#include <ArduinoJson.h>

// Définir le numéro de broche pour la LED intégrée
//  int ledPin = PIN_NEOPIXEL; // Vérifiez la documentation pour la broche LED intégrée sur votre carte ESP32-S3
//  int rgbPin = RGB_BUILTIN;

// Définir le port UDP
unsigned int localUdpPort;  // Port d'écoute local
unsigned int localWWWpPort;  // Port d'écoute local

String serverIP="";
String serverPort="";

AsyncClient* client = new AsyncClient();
/*
const char* WIFI_SSID     = "buzzmaster";
const char* WIFI_PASSWORD = "BuzzMaster";
*/

// Créer une instance de la classe WiFiUDP
  WiFiUDP bcast;

JsonDocument buzzer;
String jsonBuffer; // Tampon pour assembler les données JSON

unsigned long lastCheckTime = 0;
const unsigned long checkInterval = 2000; // Intervalle de 5 secondes

struct ButtonInfo {
  int pin;
  String name;
  bool pressed;
  String time;
};

ButtonInfo buttonsInfo[] = {
  {0, "test", false, ""}  // Initialisation complète
};


/*
int currentRed = 0;
int currentGreen = 0;
int currentBlue = 0;
int currentIntensity = 255;
*/

//void applyLedColor();
//void setLedColor(int red, int green, int blue, bool isApplyLedColor=false);
//void setLedIntensity(int intensity);
bool getServerIP();
void sendPing();
bool checkConnection();
bool connectSRV();
void sendMSG(String msgType, String message);
void hello_bumper();
void attachButtons();
void detachButtons();
void manageButtonMessages();
void handleUpdateAction(JsonObject& message, const String& macAddress);
bool wifiConnect();
void parseJSON(const String& data, AsyncClient* c);

/* *** INTERUPTION *** */
void IRAM_ATTR buttonHandler(void *arg);
void onConnect(void* arg, AsyncClient* c);
void onData(void* arg, AsyncClient* c, void* data, size_t len);
void onDisconnect(void* arg, AsyncClient* c);
