#include "includes.h"
#include "WifiManager.h"

#include "Common/CustomLogger.h"
#include "Common/led.h"

#include "teamsAndBumpers.h"

#include "SocketManager.h"
#include "tcpManager.h"
#include "buttonManager.h"
#include "fsManager.h"
#include "messages.h"
#include "BumperServer.h"
#include "WebServer.h"
#include "sdManager.h"
//#include <esp_log.h>

static const char* MAIN_TAG = "BUZZCONTROL";
const uint16_t logPort = 8888;  // Port UDP pour les logs


void setup(void)
{
  setenv("TZ", "UTC-1", 1);
  tzset();
  struct timeval tv = { .tv_sec = 0 }; // Une date quelconque
  settimeofday(&tv, NULL);

  Serial.begin(921600);
  esp_log_level_set("*", ESP_LOG_INFO);
  ESP_LOGI(MAIN_TAG, "Starting up...");
  
  initLED();
  ESP_LOGI(MAIN_TAG, "STARTING:");

  setLedColor(255, 0, 0);
  setLedIntensity(255);

  wifiConnect();
  CustomLogger::init(logPort);

  setLedColor(255, 255, 0, true);
  ESP_LOGI(MAIN_TAG, "BOOTING Version: %s", String(VERSION));

  ESP_LOGI(MAIN_TAG, "RGB pin: %d", RGB_BUILTIN);
  ESP_LOGI(MAIN_TAG, "LED pin: %d", LED_BUILTIN);
  ESP_LOGI(MAIN_TAG, "NEO pin: %d", PIN_NEOPIXEL);

  setLedIntensity(128);
  setupAP();
  setupDNSServer();

  if (!LittleFS.begin()) {
    ESP_LOGE(MAIN_TAG, "Erreur de montage LittleFS");
    return;
  }
  
  yield();
  sleep(2);
  downloadFiles();

  listLittleFSFiles();
  printLittleFSInfo();

  setLedColor(128, 128, 0, true);

  attachButtons();
  loadJson(GameFile);

//  sdInit();

  startWebServer();
  startBumperServer();
  setLedColor(0, 128, 0, true);

  messageQueue = xQueueCreate(10, sizeof(messageQueue_t));

  if (messageQueue == NULL) {
    ESP_LOGE(MAIN_TAG, "Échec de la création de la file d'attente");
    return;
  }

  xTaskCreate(sendMessageTask, "Send Message Task", 14096, NULL, 1, NULL);
  sendHelloToAll();
  questionMutex = xSemaphoreCreateMutex();
  buttonMutex = xSemaphoreCreateMutex();

}

void loop(void)
{
  dnsServer.processNextRequest();
}
