//#include <gpio_viewer.h>
//GPIOViewer gpio_viewer;

#include "includes.h"
#include "WifiManager.h"

#include "Common/CustomLogger.h"
#include "Common/led.h"

#include "esp_task_wdt.h"
#include "serverConnection.h"
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
  
  
  initLED();
  ESP_LOGI(MAIN_TAG, "STARTING:");
  printPinInfo();
  setLedColor(255,0,0);
  setLedIntensity(255);

  setupWifi();
  yield();

  CustomLogger::init(logPort);
  ESP_LOGI(MAIN_TAG, "BOOTING Version: %s", String(VERSION));

  setLedColor(255,255,0,true);

  esp_task_wdt_init(15, true); 
  setLedColor(0,64,0,true);

  attachButtons();

  }

void loop() {
  checkWifiStatus();
  manageButtonMessages();
  
}
