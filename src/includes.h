#if defined(ESP32)
  #include <WiFi.h>
  #include <AsyncTCP.h>
  #include <ESPmDNS.h>
  #include <esp_task_wdt.h>  // Pour gérer le watchdog timer si nécessaire
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

#include <freertos/FreeRTOS.h>
#include <freertos/queue.h>


const char* ssid     = "CC-Home";
const char* password = "GenericPassword";

#if defined(ESP32)
  int ledPin = PIN_NEOPIXEL; // Vérifiez la documentation pour la broche LED intégrée sur votre carte ESP32-S3
  int rgbPin = RGB_BUILTIN;
  hw_timer_t * timer = NULL;
  portMUX_TYPE timerMux = portMUX_INITIALIZER_UNLOCKED;

#elif defined(ESP8266)
  int ledPin = LED_BUILTIN; // Broche LED intégrée sur NodeMCU (ESP8266)
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

AsyncServer* bumperServer;
std::vector<AsyncClient*> bumperClients;

volatile bool GameStarted=false;
int timeRef=0;
const int nbTeam=10;
std::map<std::string, int> timeRefTeam;

unsigned int localUdpPort = 1234;  // Port d'écoute local
unsigned int localWWWpPort = 80;  // Port d'écoute local

AsyncWebServer  server(localWWWpPort);
AsyncWebSocket ws("/ws");

WiFiUDP ntpUDP;
NTPClient timeClient(ntpUDP, "pool.ntp.org");
const char* GameFile="/game.json";
const char* saveGameFile="/game.json.save";

typedef struct {
    const char*   action;  // Type d'action
    const char* message;  // Contenu du message
    bool notifyAll;
    AsyncClient* client;
} messageQueue_t;

QueueHandle_t messageQueue; // File d'attente pour les messages

JsonDocument teamsAndBumpers;



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

void wifiConnect();
void listLittleFSFiles();
void resetBumpersTime();
void startGame();
void stopGame();
void pauseGame(AsyncClient* client);
void attachButtons();
void startBumperServer();
void checkPingForAllClients();
void parseDataFromSocket(const char* action, JsonObject message);
void mergeJson(JsonObject& destObj, const JsonObject& srcObj);
void update(String action, JsonObject& obj);

/* **** INTERUPTIONS *** */
static void IRAM_ATTR buttonHandler(void *arg);
void onTimerISR();
void b_handleData(void* arg, AsyncClient* c, void *data, size_t len);
static void b_onClientDisconnect(void* arg, AsyncClient* client);
static void b_onCLientConnect(void* arg, AsyncClient* client);
void onWsEvent(AsyncWebSocket *server, AsyncWebSocketClient *client, AwsEventType type, void *arg, uint8_t *data, size_t len);
void sendMessageTask(void *parameter);


