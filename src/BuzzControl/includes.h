#pragma once
#include "common/Constant.h"
#include "Common/configManager.h"

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
struct LedColor {
    uint8_t red;
    uint8_t green;
    uint8_t blue;
    uint8_t intensity;
    
    LedColor(uint8_t r = 0, uint8_t g = 0, uint8_t b = 0, uint8_t i = 255) 
        : red(r), green(g), blue(b), intensity(i) {}
};

// Énumération des états possibles pour une meilleure gestion
enum class GameState {
    BOOT1, BOOT2, BOOT3, BOOT4,
    START,
    STOP,
    PAUSE,
    PREPARE,
    READY,
    REVEAL,
    ERROR,
    WAITING,
    CONNECTED,
    DISCONNECTED
};

// Tableau de définition des couleurs par état
const std::map<GameState, LedColor> stateColors = {
    {GameState::BOOT1,          LedColor(255,   0, 0,   64)},  // Rouge low
    {GameState::BOOT2,          LedColor(255,   0, 0,   200)},  // Rouge brillant
    {GameState::BOOT3,          LedColor(255,   255, 0,   200)},  // Orange low
    {GameState::BOOT4,          LedColor(255,   255, 0,   200)},  // Orange brillant

    {GameState::PREPARE,        LedColor(0,   0,   255, 64)},  // Bleu low
    {GameState::READY,          LedColor(0,   0,   255, 180)},  // Bleu
    {GameState::START,          LedColor(0,   255, 0,   200)},  // Vert brillant
    {GameState::STOP,           LedColor(0,   255, 0,   64)},  // Rouge brillant
    {GameState::PAUSE,          LedColor(0,   255, 0,   64)},  // Vert modéré

    {GameState::REVEAL,         LedColor(255, 255, 255, 128)},  // Blanc maximal
    {GameState::ERROR,          LedColor(255, 0,   0,   255)},  // Rouge maximal
    {GameState::WAITING,        LedColor(128, 128, 128, 100)},  // Gris faible
    {GameState::CONNECTED,      LedColor(0,   255, 255, 150)},  // Cyan
    {GameState::DISCONNECTED,   LedColor(128, 0,   128, 100)}   // Violet faible
};

void setLedByState(GameState state) {
    auto it = stateColors.find(state);
    if (it != stateColors.end()) {
        const LedColor& color = it->second;
        setLedColor(color.red, color.green, color.blue);
        setLedIntensity(color.intensity);
    }
}

void setLedByState(const String& stateName) {
    // Conversion string vers enum
    static const std::map<String, GameState> stringToState = {
      {"BOOT1",        GameState::BOOT1},
      {"BOOT2",        GameState::BOOT2},
      {"BOOT3",        GameState::BOOT3},
      {"BOOT4",        GameState::BOOT4},
      {"PREPARE",        GameState::PREPARE},
        {"START",        GameState::START},
        {"STOP",         GameState::STOP},
        {"PAUSE",        GameState::PAUSE},
        {"READY",        GameState::READY},
        {"REVEAL",       GameState::REVEAL},
        {"ERROR",        GameState::ERROR},
        {"WAITING",      GameState::WAITING},
        {"CONNECTED",    GameState::CONNECTED},
        {"DISCONNECTED", GameState::DISCONNECTED}
    };
    
    auto it = stringToState.find(stateName);
    if (it != stringToState.end()) {
        setLedByState(it->second);
    }
}

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
static const char* saveGameFile = "/files/game.json.save";
static const String questionsPath = "/files/questions";

SemaphoreHandle_t questionMutex = NULL;
SemaphoreHandle_t buttonMutex = NULL;
SemaphoreHandle_t updateMutex = NULL;

// Map pour stocker les buffers par client (identifiés par IP)
std::map<String, String> clientBuffers;

/* **** FUNCTIONS DEFINITIONS *** */

// teamsAndBumpers.h
//void setGamePhase(String phase);

// messages_to_send.h
void enqueueOutgoingMessage(const char* action, const char* msg, const char* update, bool notify = false, AsyncClient* client = nullptr);
void sendMessageToClient(const String& action, const String& msg, const String& update, AsyncClient* client);
void sendMessageToAllClients(const String& action, const String& msg, const String& update="");
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
void updateScore(const String bumperID, const int points);
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

void startWebServer();
/* **** INTERRUPTS *** */
static void IRAM_ATTR buttonHandler(void *arg);
void onTimerISR();
void b_handleData(void* arg, AsyncClient* c, void *data, size_t len);
static void b_onClientDisconnect(void* arg, AsyncClient* client);
static void b_onCLientConnect(void* arg, AsyncClient* client);
void onWsEvent(AsyncWebSocket *server, AsyncWebSocketClient *client, AwsEventType type, void *arg, uint8_t *data, size_t len);


