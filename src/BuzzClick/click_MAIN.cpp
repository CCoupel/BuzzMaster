//#include <gpio_viewer.h>
//GPIOViewer gpio_viewer;

#include "click_includes.h"
#include "click_WifiManager.h"

#include "Common/CustomLogger.h"
#include "Common/led.h"

#include "esp_task_wdt.h"
#include "click_serverConnection.h"
//#include "esp_sntp.h"

static const char* MAIN_TAG = "MAIN";
const uint16_t logPort = 8889;  // Port UDP pour les logs


void printPinInfo() {
  #if defined(ESP32)
    ESP_LOGI(MAIN_TAG, "RGB pin: %d", RGB_BUILTIN);
  #endif
  ESP_LOGI(MAIN_TAG, "LED pin: %d", LED_BUILTIN);
  #if defined(ESP32)
    ESP_LOGI(MAIN_TAG, "NEO pin: %d", PIN_NEOPIXEL);
  #endif
}

void watchdogTask(void *pvParameters) {
    // Configurer le watchdog avec un délai plus long si nécessaire
    esp_task_wdt_init(30, true); // 10 secondes de délai
    esp_task_wdt_add(NULL);      // S'enregistrer auprès du watchdog
    
    for (;;) {
        // Réinitialiser le watchdog
        esp_task_wdt_reset();
        
        // Surveillance du système
        ESP_LOGD("WATCHDOG", "Memory: %u bytes free", ESP.getFreeHeap());
        
        vTaskDelay(pdMS_TO_TICKS(1000));
    }
}

void setup() 
{
  setenv("TZ", "UTC-1", 1);
  tzset();
  struct timeval tv = { .tv_sec = 0 }; // Une date quelconque
  settimeofday(&tv, NULL);

  Serial.begin(921600);
  Serial.println("STARTING!!");

  esp_log_level_set("*", ESP_LOG_INFO);
  ESP_LOGI(MAIN_TAG, "Starting up...");
  
  // Créer une tâche spécifique pour surveiller le watchdog
    xTaskCreate(
        watchdogTask,
        "WatchdogTask",
        2048,
        NULL,
        configMAX_PRIORITIES - 1, // Haute priorité
        NULL
    );
 
  initLED();
  ESP_LOGI(MAIN_TAG, "STARTING:");
  printPinInfo();
  for (int led=0; led<NUMPIXELS/4; led++) {
    setPixelColor(led+1, 255, 0, 0);
  }
  showPixels();

  setupWifi();
  yield();
  
  CustomLogger::init(logPort);
  ESP_LOGI(MAIN_TAG, "BOOTING Version: %s", String(VERSION));

for (int led=0; led<NUMPIXELS/4; led++) {
    setPixelColor(NUMPIXELS/4+led+1, 0, 0, 255);
  }
  showPixels();
//  esp_task_wdt_init(15, true); 

//  esp_task_
  attachButtons();
for (int led=0; led<NUMPIXELS/4; led++) {
    setPixelColor(2*NUMPIXELS/2+led+1, 0, 255, 0);
  }
  showPixels();
}

void loop() {
//  checkWifiStatus();
  manageButtonMessages();
  
}
