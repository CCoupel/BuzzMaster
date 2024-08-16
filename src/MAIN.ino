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

//AsyncUDP Udp;

JsonDocument teamsAndBumpers;
//JsonObject bumpers=teamsAndBumpers["bumpers"].to<JsonObject>();
//JsonObject teams=teamsAndBumpers["teams"].to<JsonObject>();
void applyLedColor() {
  int adjustedRed = (currentRed * currentIntensity) / 255;
  int adjustedGreen = (currentGreen * currentIntensity) / 255;
  int adjustedBlue = (currentBlue * currentIntensity) / 255;

#if defined(ESP32)
  neopixelWrite(rgbPin,adjustedRed,adjustedGreen,adjustedBlue);
  int pwmValue = 255 - currentIntensity;
  //analogWrite(LED_BUILTIN, pwmValue);
  //delay(500);
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
  std::vector<String> keysToRemove = {"STATUS", "BUTTON", "TIME", "DELAY", "DELAY_TEAM"};

  JsonObject bumpers=teamsAndBumpers["bumpers"];
  JsonObject teams=teamsAndBumpers["teams"];
  for (JsonPair kvp : bumpers) {
    if (kvp.value().is<JsonObject>()) {
      JsonObject bumper = kvp.value().as<JsonObject>();

      for (String key : keysToRemove) {
        if (bumper.containsKey(key)) {
          bumper.remove(key);
        }    
      }
    } else {
        Serial.println("Error: Bumper entry is not a JsonObject");
    }
  }
  for (JsonPair team : teams) {
    if (team.value().is<JsonObject>()) {
      JsonObject teamData = team.value().as<JsonObject>();

      for (String key : keysToRemove) {
        if (teamData.containsKey(key)) {
          teamData.remove(key);
        }    
      }
    } else {
        Serial.println("Error: Team entry is not a JsonObject");
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
//    Serial.println("Button " + String(buttonInfo->pin) + " pressed");
    //digitalWrite(ledPin, LOW);
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
  // Initialiser les broches comme sorties pour ESP8266
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
  //WiFi.mode(WIFI_STA);
  delay(10);
  wifiConnect();
  timeClient.update();

  //waitNewBumper();


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
  loadJson(GameFile);

  startWebServer();
  startBumperServer();
  setLedColor(0, 255, 0);
  setLedIntensity(64);
  messageQueue = xQueueCreate(10, sizeof(messageQueue_t));

    if (messageQueue == NULL) {
      Serial.println("Échec de la création de la file d'attente");
      return;
    }

  xTaskCreate(sendMessageTask, "Send Message Task", 4096, NULL, 1, NULL);

/*
#if defined(ESP8266)
  timer1_attachInterrupt(onTimerISR);
  timer1_enable(TIM_DIV256, TIM_EDGE, TIM_LOOP);
  timer1_write(312500);  // 1 seconde
    timer1_write(612500);  // 1 seconde
#endif
#if defined(ESP32)
    timer = timerBegin(0, 80, true);  // Timer 0, diviseur 80, comptage croissant
    timerAttachInterrupt(timer, &onTimerISR, true);  // Attache l'interrupt
    timerAlarmWrite(timer, 1000000, true);  // Déclenche l'interrupt toutes les 1 seconde (1 000 000 microsecondes)
    timerAlarmEnable(timer);  // Active l'alarme du timer

#endif
*/
}

void onTimerISR() {
  sendMessageToAllClients("PING", "'Are you alive?'");  // Appelée toutes les secondes
  checkPingForAllClients();
}


void loop(void)
{
#if defined(ESP8266)
//  server.handleClient(); // Gestion des requêtes clients
  MDNS.update();
#endif
}
