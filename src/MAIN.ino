#include "includes.h"
#include "CustomLogger.h"
#include "teamsAndBumpers.h"

#include "wifiManager.h"
#include "SocketManager.h"
#include "tcpManager.h"
#include "buttonManager.h"
#include "led.h"
#include "fsManager.h"
#include "messages.h"
#include "BumperServer.h"
#include "WebServer.h"
//#include "myNTPserver.h"
#include <esp_log.h>

static const char* MAIN_TAG = "BUZZCONTROL";
const uint16_t logPort = 8888;  // Port UDP pour les logs

void setup(void)
{
  Serial.begin(921600);
  esp_log_level_set("*", ESP_LOG_INFO);

  setLedColor(255, 0, 0);
  setLedIntensity(255);

  wifiConnect();
  CustomLogger::init(logPort);
  setLedColor(255, 255, 0, true);
  ESP_LOGI(MAIN_TAG, "RGB pin: %d", RGB_BUILTIN);
  ESP_LOGI(MAIN_TAG, "LED pin: %d",LED_BUILTIN);
  ESP_LOGI(MAIN_TAG, "NEO pin: %d",PIN_NEOPIXEL);

//  timeClient.update();
  setLedIntensity(128);
  setupAP();
  setupDNSServer();

//  setupNTPserver();

  

  if (!LittleFS.begin()) {
    ESP_LOGE(MAIN_TAG, "Erreur de montage LittleFS");
    return;
  }

  downloadFiles();

  listLittleFSFiles();
  setLedColor(128, 128, 0, true);

  attachButtons();
  loadJson(GameFile);

  

  startWebServer();
  startBumperServer();
  setLedColor(0, 128, 0, true);

  messageQueue = xQueueCreate(10, sizeof(messageQueue_t));

    if (messageQueue == NULL) {
      ESP_LOGE(MAIN_TAG, "Échec de la création de la file d'attente");
      return;
    }

  xTaskCreate(sendMessageTask, "Send Message Task", 14096, NULL, 1, NULL);
  sendMessageToAllClients("HELLO", "{  }");
}

void loop(void)
{
//  handleNTPserver();
  dnsServer.processNextRequest();

}
