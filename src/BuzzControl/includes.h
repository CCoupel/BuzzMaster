#pragma once
#include "common/Constant.h"
#include "Common/CustomLogger.h"
#include "Common/led.h"

#include <WiFi.h>
#include <AsyncTCP.h>
#include <esp_task_wdt.h>  // Pour gérer le watchdog timer si nécessaire
#include <Arduino.h>

#include <ESPAsyncWebServer.h>

#include <WiFiClient.h>

#include <LittleFS.h>

#include <ArduinoJson.h>
#include <map>

#include <freertos/FreeRTOS.h>
#include <freertos/queue.h>
#include "freertos/semphr.h"

hw_timer_t * timer = NULL;
portMUX_TYPE timerMux = portMUX_INITIALIZER_UNLOCKED;

struct ButtonInfo {
  int pin;
  String name;
};

ButtonInfo buttonsInfo[] = {
  {0, "toggle"}
};

AsyncServer* bumperServer;
std::vector<AsyncClient*> bumperClients;

volatile bool GameStarted = false;
int64_t timeRef = 0;
const int nbTeam = 10;
std::map<std::string, int64_t> timeRefTeam;

unsigned int localWWWPort = 80;  // Port d'écoute local

AsyncWebServer server(localWWWPort);
AsyncWebSocket ws("/ws");
String jsonBuffer; // Tampon pour assembler les données JSON

WiFiUDP ntpUDP;
static const char* GameFile = "/config/game.json";
static const char* saveGameFile = "/config/game.json.save";
static const String questionsPath = "/files/questions";

SemaphoreHandle_t questionMutex = NULL;
SemaphoreHandle_t buttonMutex = NULL;

// Map pour stocker les buffers par client (identifiés par IP)
std::map<String, String> clientBuffers;

/* **** FUNCTIONS DEFINITIONS *** */

// teamsAndBumpers.h
//void setGamePhase(String phase);

// messages_to_send.h
void enqueueOutgoingMessage(const char* action, const char* msg, bool notify = false, AsyncClient* client = nullptr);
void sendMessageToClient(const String& action, const String& msg, AsyncClient* client);
void sendMessageToAllClients(const String& action, const String& msg);
void notifyAll();

// messages_received.h
void enqueueIncomingMessage(const char* source, const char* data, AsyncClient* client);
//void processDataFromSocket(const char* action, const JsonObject& message);
void processTCPMessage(const String& data, AsyncClient* client);
void processWebSocketMessage(const String& data);

// File system management
String readFile(const String& path, const String& defaultValue = "");
bool isFileExists(String path);
bool deleteFile(const char* filePath);
void loadJson(String path);
void saveJson();
void processClientBuffer(const String& clientID, AsyncClient* c);
bool ensureDirectoryExists(const String& path);

// WiFi and server management
void wifiConnect();
String listLittleFSFiles(String path = "/");
String printLittleFSInfo(bool isShort = false);

// Game management
void resetBumpersTime();
void updateTimer(const int Time, const int delta = 0);
void startGame(const int delay = 33);
void stopGame();
void pauseAllGame();
void pauseGame(AsyncClient* client);
void continueGame();
void revealGame();
void readyGame(const String question = "");
void deleteQuestion(const String ID);
void setRemotePage(const String remotePage);
void attachButtons();
void startBumperServer();
void checkPingForAllClients();
void resetServer();
void rebootServer();
void RAZscores();
String getQuestions();

/* **** INTERRUPTS *** */
static void IRAM_ATTR buttonHandler(void *arg);
void onTimerISR();
void b_handleData(void* arg, AsyncClient* c, void *data, size_t len);
static void b_onClientDisconnect(void* arg, AsyncClient* client);
static void b_onCLientConnect(void* arg, AsyncClient* client);
void onWsEvent(AsyncWebSocket *server, AsyncWebSocketClient *client, AwsEventType type, void *arg, uint8_t *data, size_t len);


