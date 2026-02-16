#pragma once

#include "Common/CustomLogger.h"
#include "Common/led.h"
#include "click_nvsConfig.h"
#include "click_serverConnection.h"

#ifdef USE_WEBSOCKET
#include "click_websocket_espidf.h"
#endif

#include "esp_task_wdt.h"

#include <WiFi.h>

static const char* WIFI_TAG = "WIFI";

const int WIFI_TIMEOUT_MS = 5000; // 5 secondes
const int WIFI_RECOVER_TIME_MS = 10000; // 10 secondes
bool connectToWifi();

void WiFiStationConnected(WiFiEvent_t event, WiFiEventInfo_t info){
  ESP_LOGI(WIFI_TAG,"Connecté au WiFi");

}

void WiFiGotIP(WiFiEvent_t event, WiFiEventInfo_t info){
  ESP_LOGI(WIFI_TAG,"Adresse IP obtenue %s", WiFi.localIP().toString());

    BuzzClickConfig& cfg = nvsGetConfig();

    // If server_ip is configured in NVS, use it directly
    if (cfg.server_ip.length() > 0) {
        serverIP = cfg.server_ip;
        localUdpPort = cfg.server_tcp_port;
        ESP_LOGI(WIFI_TAG, "Using NVS server: %s:%d", serverIP.c_str(), localUdpPort);
    } else {
        // Fallback: discover server via mDNS
        if (!getServerIP()) {
            ESP_LOGE(WIFI_TAG, "Failed to get server IP. Restarting...");
            ESP.restart();
        }
    }

    resetGame();
    yield();

    // === BOOT PHASE 3: ORANGE 1/4 - WiFi connected ===
    setLedColor(255, 165, 0, true, 0, 4);
    ESP_LOGI(WIFI_TAG, "Boot phase: ORANGE 1/4 (WiFi connected)");
    delay(500);

#ifdef USE_WEBSOCKET
    // WebSocket protocol
    if (!connectWebSocket(serverIP, 80)) {
        ESP_LOGE(WIFI_TAG, "Failed to connect WebSocket. Restarting...");
        ESP.restart();
    }
#else
    // TCP/UDP legacy protocol
    if (!connectSRV()) {
        ESP_LOGE(WIFI_TAG, "Failed to connect to server. Restarting...");
        ESP.restart();
    }
    // === BOOT PHASE 4: GREEN 2/4 - Server connected (TCP mode) ===
    setLedColor(0, 255, 0, true, 0, 2);
    ESP_LOGI(WIFI_TAG, "Boot phase: GREEN 2/4 (server connected)");
    yield();
    if (!initBroadcastUDP()) {
        ESP_LOGE(WIFI_TAG, "Failed to listen to server. Restarting...");
        ESP.restart();
    }
#endif

    ESP_LOGI(WIFI_TAG, "READY");
}

void WiFiStationDisconnected(WiFiEvent_t event, WiFiEventInfo_t info){
  ESP_LOGI(WIFI_TAG,"Déconnecté du WiFi");
  // Red = disconnected/error
  setLedColor(255, 0, 0, true);
  WiFi.disconnect();
  connectToWifi();
}

bool connectToWifi() {
  BuzzClickConfig& cfg = nvsGetConfig();

  // If NVS is empty (after factory reset), stay in USB-only mode (orange LED)
  if (!cfg.valid) {
    ESP_LOGI(WIFI_TAG, "No WiFi config in NVS, skipping WiFi connection (USB mode)");
    // Orange = USB-only mode
    setLedColor(255, 128, 0, true);
    return false;
  }

  // Use NVS config for WiFi connection
  const char* ssid = cfg.wifi_ssid.c_str();
  const char* password = cfg.wifi_password.c_str();

  ESP_LOGI(WIFI_TAG, "Connecting to WiFi SSID=%s (source=NVS)", ssid);

  // === BOOT PHASE 2: RED 1/4 - WiFi connecting ===
  setLedColor(255, 0, 0, true, 0, 4);
  ESP_LOGI(WIFI_TAG, "Boot phase: RED 1/4 (WiFi connecting)");

  WiFi.mode(WIFI_STA);
  WiFi.begin(ssid, password);
  unsigned long startAttemptTime = millis();

  while (WiFi.status() != WL_CONNECTED && millis() - startAttemptTime < WIFI_TIMEOUT_MS) {
    esp_task_wdt_reset();
    delay(100);
    Serial.print(".");
  }

  if (WiFi.status() != WL_CONNECTED) {
    ESP_LOGE(WIFI_TAG, "WiFi connection failed to SSID=%s", ssid);
    // Red = error
    setLedColor(255, 0, 0, true);
    return false;
  }

  ESP_LOGI(WIFI_TAG, "WiFi connecté %s", WiFi.localIP().toString());
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
  BuzzClickConfig& cfg = nvsGetConfig();

  // If NVS is empty (USB-only mode), don't attempt WiFi reconnection
  if (!cfg.valid) {
    return;
  }

  if (WiFi.status() != WL_CONNECTED) {
    if (millis() - lastAttemptTime > WIFI_RECOVER_TIME_MS) {
      ESP_LOGI(WIFI_TAG, "WiFi disconnected, attempting reconnection...");
      if (!connectToWifi()) {
        ESP_LOGE(WIFI_TAG, "Failed to reconnect to WiFi. Restarting...");
        ESP.restart();
      }
      lastAttemptTime = millis();
    }
  }
}
