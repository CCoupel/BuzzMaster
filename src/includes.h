#pragma once

#include <WiFi.h>
#include <AsyncTCP.h>
#include <esp_task_wdt.h>  // Pour gérer le watchdog timer si nécessaire
#include <Arduino.h>

#include <ESPAsyncWebServer.h>

#include <NTPClient.h>
#include <WiFiClient.h>

#include <LittleFS.h>

#include <ArduinoJson.h>
#include <map>

#include <freertos/FreeRTOS.h>
#include <freertos/queue.h>

#define _VERSION 0



int ledPin = PIN_NEOPIXEL; // Vérifiez la documentation pour la broche LED intégrée sur votre carte ESP32-S3
int rgbPin = RGB_BUILTIN;
hw_timer_t * timer = NULL;
portMUX_TYPE timerMux = portMUX_INITIALIZER_UNLOCKED;

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

AsyncServer* bumperServer;
std::vector<AsyncClient*> bumperClients;

volatile bool GameStarted=false;
int64_t  timeRef=0;
const int nbTeam=10;
std::map<std::string, int64_t > timeRefTeam;

unsigned int localWWWpPort = 80;  // Port d'écoute local

AsyncWebServer  server(localWWWpPort);
AsyncWebSocket ws("/ws");
String jsonBuffer; // Tampon pour assembler les données JSON

WiFiUDP ntpUDP;
NTPClient timeClient(ntpUDP, "pool.ntp.org");
const char* GameFile="/game.json";
const char* saveGameFile="/game.json.save";

/*
typedef struct {
    const char*   action;  // Type d'action
    const char* message;  // Contenu du message
    bool notifyAll;
    AsyncClient* client;
} messageQueue_t;
*/

QueueHandle_t messageQueue; // File d'attente pour les messages


// Map pour stocker les buffers par client (identifiés par IP)
std::map<String, String> clientBuffers;


/* teamsAndBumpers.h */
void setGamePhase(String phase);

/* **** TOOLS *** */

void putMsgToQueue(const char* action=nullptr, const char* msg="", bool notify=false, AsyncClient* client=nullptr );
void setLedColor(int red, int green, int blue, bool isApplyLedColor=false);
void setLedIntensity(int intensity);
void applyLedColor();
void sendMessageToClient(const String& action, const String& msg, AsyncClient* client);
void sendMessageToAllClients(const String& action, const String& msg );
void notifyAll();
void loadJson(String path);
void saveJson();
void processClientBuffer(const String& clientID, AsyncClient* c);
void parseJSON(const String& data, AsyncClient* c);

void wifiConnect();
void listLittleFSFiles();
void resetBumpersTime();
void startGame();
void stopGame();
void pauseAllGame();
void pauseGame(AsyncClient* client);
void continueGame();
void attachButtons();
void startBumperServer();
void checkPingForAllClients();
//void parseDataFromSocket(const char* action, JsonObject& message);
//void mergeJson(JsonObject& destObj, const JsonObject& srcObj);
void update(String action, JsonObject& obj);
void resetServer();
void rebootServer();
void RAZscores();

/* **** INTERUPTIONS *** */
static void IRAM_ATTR buttonHandler(void *arg);
void onTimerISR();
void b_handleData(void* arg, AsyncClient* c, void *data, size_t len);
static void b_onClientDisconnect(void* arg, AsyncClient* client);
static void b_onCLientConnect(void* arg, AsyncClient* client);
void onWsEvent(AsyncWebSocket *server, AsyncWebSocketClient *client, AwsEventType type, void *arg, uint8_t *data, size_t len);
void sendMessageTask(void *parameter);


constexpr unsigned int hash(const char* str, int h = 0) {
    return !str[h] ? 5381 : (hash(str, h+1) * 33) ^ str[h];
}
