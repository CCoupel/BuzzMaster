#include "includes.h"
#include "WifiManager.h"

#include "Common/CustomLogger.h"
#include "Common/led.h"

//#include "jsonManager.h"
#include "teamsAndBumpers.h"

#include "SocketManager.h"
#include "tcpManager.h"
#include "buttonManager.h"
#include "fsManager.h"
#include "messages_received.h"
#include "messages_to_send.h"
#include "BumperServer.h"
#include "WebServer.h"
#include "sdManager.h"

static const char* MAIN_TAG = "BUZZCONTROL";
const uint16_t logPort = 8888;  // Port UDP pour les logs

// Nouvelle tâche de surveillance du watchdog
void watchdogTask(void *pvParameters) {
    // Configurer le watchdog avec un délai plus long si nécessaire
    esp_task_wdt_init(10, true); // 10 secondes de délai
    esp_task_wdt_add(NULL);      // S'enregistrer auprès du watchdog
    
    for (;;) {
        // Réinitialiser le watchdog
        esp_task_wdt_reset();
        
        // Surveillance du système
        ESP_LOGD("WATCHDOG", "Memory: %u bytes free", ESP.getFreeHeap());
        
        vTaskDelay(pdMS_TO_TICKS(1000));
    }
}

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

  startWebServer();
  startBumperServer();
  setLedColor(0, 128, 0, true);

  // Initialisation des files d'attente de messages
  initIncomingQueue();
  initOutgoingQueue();

  // Création des tâches pour traiter les messages
  xTaskCreate(receiveMessageTask, "Receive Message Task", 20480, NULL, 2, NULL);
  xTaskCreate(sendMessageTask, "Send Message Task", 20480, NULL, 2, NULL);

  // Création de la tâche pour surveiller le watchdog
  xTaskCreate(
      watchdogTask,
      "WatchdogTask",
      2048,
      NULL,
      configMAX_PRIORITIES - 1,
      NULL
  );
  
  // Initialisation des mutex
  questionMutex = xSemaphoreCreateMutex();
  buttonMutex = xSemaphoreCreateMutex();
  
  // Envoyer un message de bienvenue à tous les clients
  enqueueOutgoingMessage("HELLO", "{}", false, nullptr);
}

void loop(void)
{
  dnsServer.processNextRequest();
}