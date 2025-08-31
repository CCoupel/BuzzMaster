#pragma once
//#include <gpio_viewer.h>
//GPIOViewer gpio_viewer;

#include "Common/CustomLogger.h"
#include "Common/led.h"
#include "click_serverConnection.h"
#include "esp_task_wdt.h"

#include <WiFi.h>

//const char* WIFI_SSID = "votre_ssid";
//const char* WIFI_PASSWORD = "votre_mot_de_passe";
static const char* WIFI_TAG = "WIFI";

const int WIFI_TIMEOUT_MS = 5000; // 5 secondes
const int WIFI_RECOVER_TIME_MS = 10000; // 10 secondes
bool connectToWifi();

void WiFiStationConnected(WiFiEvent_t event, WiFiEventInfo_t info){
  ESP_LOGI(WIFI_TAG,"Connecté au WiFi");

}

void WiFiGotIP(WiFiEvent_t event, WiFiEventInfo_t info){
  ESP_LOGI(WIFI_TAG,"Adresse IP obtenue %s", WiFi.localIP().toString());
//    gpio_viewer.begin();
    if (!getServerIP()) {
        ESP_LOGE(WIFI_TAG, "Failed to get server IP. Restarting...");
        ESP.restart();
    }
    setLedColor(128,128,0,true);
    resetGame();
    yield();
    if (!connectSRV()) {
        ESP_LOGE(WIFI_TAG, "Failed to connect to server. Restarting...");
        ESP.restart();
    }
    setLedColor(64,128,0,true);
    yield();
    if (!initBroadcastUDP()) {
        ESP_LOGE(WIFI_TAG, "Failed to listen to server. Restarting...");
        ESP.restart();
    }
    ESP_LOGI(WIFI_TAG, "READY");
    setLedColor(0,0,0);
    setLedIntensity(255);
}

void WiFiStationDisconnected(WiFiEvent_t event, WiFiEventInfo_t info){
  ESP_LOGI(WIFI_TAG,"Déconnecté du WiFi");
  WiFi.disconnect();
//  WiFi.begin(WIFI_SSID, WIFI_PASSWORD);
  connectToWifi();
}

bool connectToWifi() {
  ESP_LOGI(WIFI_TAG,"Tentative de connexion WiFi...");
  WiFi.mode(WIFI_STA);
  //WiFi.begin(ssid, password);
  WiFi.begin(WIFI_SSID, WIFI_PASSWORD);
  unsigned long startAttemptTime = millis();

  while (WiFi.status() != WL_CONNECTED && millis() - startAttemptTime < WIFI_TIMEOUT_MS) {
    esp_task_wdt_reset();

    delay(100);
    Serial.print(".");
  }

  if (WiFi.status() != WL_CONNECTED) {
    ESP_LOGE(WIFI_TAG,"Échec de la connexion WiFi");
    return false;
  }

  ESP_LOGI(WIFI_TAG,"WiFi connecté %s", WiFi.localIP().toString());
  return true;
}

void setupWifi() {

  WiFi.onEvent(WiFiStationConnected, WiFiEvent_t::ARDUINO_EVENT_WIFI_STA_CONNECTED);
  WiFi.onEvent(WiFiGotIP, WiFiEvent_t::ARDUINO_EVENT_WIFI_STA_GOT_IP);
  WiFi.onEvent(WiFiStationDisconnected, WiFiEvent_t::ARDUINO_EVENT_WIFI_STA_DISCONNECTED);

  connectToWifi();
}

  static unsigned long lastAttemptTime = 0;

void checkWifiStatus() {
  if (WiFi.status() != WL_CONNECTED) {
    if (millis() - lastAttemptTime > WIFI_RECOVER_TIME_MS) {
      ESP_LOGI(WIFI_TAG,"Tentative de reconnexion WiFi...");
      if (!connectToWifi()) {
        ESP_LOGE(WIFI_TAG, "Failed to connect to WIFI. Restarting...");
        ESP.restart();
      }
      lastAttemptTime = millis();
    }
  }
}
