#pragma once
#include "common/Constant.h"
#include "Common/CustomLogger.h"
#include "Common/led.h"

#include <WiFi.h>
#include <AsyncTCP.h>
#include <esp_task_wdt.h>  // Pour gérer le watchdog timer si nécessaire
#include <Arduino.h>

#include <ESPAsyncWebServer.h>

//#include <NTPClient.h>
#include <WiFiClient.h>

#include <LittleFS.h>

#include <ArduinoJson.h>
#include <map>

#include <freertos/FreeRTOS.h>
#include <freertos/queue.h>

hw_timer_t * timer = NULL;
portMUX_TYPE timerMux = portMUX_INITIALIZER_UNLOCKED;
/*
int currentRed = 0;
int currentGreen = 0;
int currentBlue = 0;
int currentIntensity = 255;
*/
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

unsigned int localWWWPort = 80;  // Port d'écoute local

AsyncWebServer  server(localWWWPort);
AsyncWebSocket ws("/ws");
String jsonBuffer; // Tampon pour assembler les données JSON

WiFiUDP ntpUDP;
//NTPClient timeClient(ntpUDP, "pool.ntp.org");
const char* GameFile="/game.json";
const char* saveGameFile="/game.json.save";

QueueHandle_t messageQueue; // File d'attente pour les messages


// Map pour stocker les buffers par client (identifiés par IP)
std::map<String, String> clientBuffers;


/* teamsAndBumpers.h */
void setGamePhase(String phase);

/* **** TOOLS *** */

void putMsgToQueue(const char* action=nullptr, const char* msg="", bool notify=false, AsyncClient* client=nullptr );
//void setLedColor(int red, int green, int blue, bool isApplyLedColor=false);
//void setLedIntensity(int intensity);
//void applyLedColor();
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
void startGame(const int delay=33);
void stopGame();
void pauseAllGame(const int currentTime);
void pauseGame(AsyncClient* client);
void continueGame();
void setRemotePage(const String remotePage);
void attachButtons();
void startBumperServer();
void checkPingForAllClients();
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
