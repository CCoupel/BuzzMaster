#pragma once

#include "Common/CustomLogger.h"
#include "Common/led.h"
#include "click_nvsConfig.h"
#include "click_serverConnection.h"
#include "click_broadcaster.h"

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

// Try connecting to a server IP via WebSocket or TCP.
// Returns true on success, sets serverIP and localUdpPort globals.
bool tryConnectToServer(const String& ip, uint16_t port) {
    ESP_LOGI(WIFI_TAG, "Trying server %s:%d", ip.c_str(), port);
    serverIP = ip;
    localUdpPort = port;

#ifdef USE_WEBSOCKET
    if (connectWebSocket(ip, port)) {
        ESP_LOGI(WIFI_TAG, "WebSocket connected to %s:%d", ip.c_str(), port);
        return true;
    }
#else
    if (connectSRV()) {
        ESP_LOGI(WIFI_TAG, "TCP connected to %s:%d", ip.c_str(), port);
        return true;
    }
#endif
    ESP_LOGW(WIFI_TAG, "Failed to connect to %s:%d", ip.c_str(), port);
    return false;
}

void WiFiGotIP(WiFiEvent_t event, WiFiEventInfo_t info){
    ESP_LOGI(WIFI_TAG, "Adresse IP obtenue %s", WiFi.localIP().toString());

    resetGame();
    yield();

    // === BOOT PHASE 3: ORANGE 1/4 - WiFi connected ===
    setLedColor(255, 165, 0, true, 0, 4);
    ESP_LOGI(WIFI_TAG, "Boot phase: ORANGE 1/4 (WiFi connected)");
    delay(500);

    // Start UDP broadcast listener for server discovery
    resetBroadcastDiscovery();
    if (!startBroadcastListener(CONTROLER_PORT)) {
        ESP_LOGE(WIFI_TAG, "Failed to start broadcast listener");
    }

    // === BOOT PHASE 4: YELLOW PULSING - Waiting for broadcast ===
    ESP_LOGI(WIFI_TAG, "Boot phase: YELLOW pulsing (waiting for broadcast)");
    bool connected = false;
    const unsigned long BROADCAST_TIMEOUT_MS = 30000; // 30s max wait
    unsigned long waitStart = millis();

    while (!connected) {
        // Yellow pulsing while waiting for broadcast
        bool pulseState = false;
        while (!hasBroadcastDiscovery()) {
            esp_task_wdt_reset();

            // Yellow pulsing at 2Hz
            pulseState = !pulseState;
            if (pulseState) {
                setLedColor(255, 200, 0, true);
            } else {
                setLedColor(64, 50, 0, true);
            }
            delay(250);

            // Timeout: fall back to NVS or mDNS
            if (millis() - waitStart > BROADCAST_TIMEOUT_MS) {
                ESP_LOGW(WIFI_TAG, "Broadcast timeout after %d ms, trying fallback", BROADCAST_TIMEOUT_MS);
                break;
            }
        }

        if (hasBroadcastDiscovery()) {
            // === BOOT PHASE 5: BLUE RAPID - Trying all IPs ===
            ESP_LOGI(WIFI_TAG, "Boot phase: BLUE rapid (trying %d IPs)", getBroadcastDiscovery().ipCount);
            const BroadcastDiscovery& disc = getBroadcastDiscovery();

            for (int i = 0; i < disc.ipCount; i++) {
                esp_task_wdt_reset();

                // Blue rapid blink for each attempt
                setLedColor(0, 0, 255, true);
                delay(100);
                setLedColor(0, 0, 64, true);
                delay(100);

                if (tryConnectToServer(disc.ips[i], disc.serverPort)) {
                    connected = true;
                    break;
                }
            }

            if (!connected) {
                ESP_LOGW(WIFI_TAG, "All broadcast IPs failed, waiting for next heartbeat...");
                resetBroadcastDiscovery();
                waitStart = millis(); // reset timeout for next round
                continue;
            }
        } else {
            // Broadcast timeout: try NVS server_ip or mDNS as fallback
            BuzzClickConfig& cfg = nvsGetConfig();

            if (cfg.server_ip.length() > 0) {
                ESP_LOGI(WIFI_TAG, "Fallback: using NVS server %s:%d", cfg.server_ip.c_str(), cfg.server_tcp_port);
                setLedColor(0, 0, 255, true);
                delay(200);

                if (tryConnectToServer(cfg.server_ip, cfg.server_tcp_port)) {
                    connected = true;
                } else {
                    ESP_LOGW(WIFI_TAG, "NVS server failed, retrying broadcast...");
                    resetBroadcastDiscovery();
                    waitStart = millis();
                    continue;
                }
            } else {
                // Last resort: mDNS discovery
                ESP_LOGI(WIFI_TAG, "Fallback: trying mDNS discovery");
                if (getServerIP()) {
                    BuzzClickConfig& cfg2 = nvsGetConfig();
                    if (tryConnectToServer(serverIP, cfg2.server_tcp_port > 0 ? cfg2.server_tcp_port : 80)) {
                        connected = true;
                    }
                }

                if (!connected) {
                    ESP_LOGW(WIFI_TAG, "mDNS fallback failed, retrying broadcast...");
                    resetBroadcastDiscovery();
                    waitStart = millis();
                    continue;
                }
            }
        }
    }

    // Stop broadcast listener once connected (no longer needed during active session)
    stopBroadcastListener();

#ifndef USE_WEBSOCKET
    // TCP legacy mode: also set up game broadcast UDP listener
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
    ESP_LOGW(WIFI_TAG, "WiFi primary failed (SSID=%s)", ssid);

    // Try fallback WiFi if configured
    if (cfg.wifi_ssid2.length() > 0) {
      ESP_LOGI(WIFI_TAG, "Trying fallback WiFi SSID2=%s", cfg.wifi_ssid2.c_str());
      setLedColor(255, 165, 0, true, 0, 4);  // orange = trying fallback

      WiFi.disconnect();
      WiFi.begin(cfg.wifi_ssid2.c_str(), cfg.wifi_pass2.c_str());
      startAttemptTime = millis();

      while (WiFi.status() != WL_CONNECTED && millis() - startAttemptTime < WIFI_TIMEOUT_MS) {
        esp_task_wdt_reset();
        delay(100);
        Serial.print(".");
      }

      if (WiFi.status() != WL_CONNECTED) {
        ESP_LOGE(WIFI_TAG, "WiFi fallback also failed (SSID2=%s)", cfg.wifi_ssid2.c_str());
        setLedColor(255, 0, 0, true);
        return false;
      }

      ESP_LOGI(WIFI_TAG, "Connected via fallback WiFi (SSID2=%s) IP=%s",
               cfg.wifi_ssid2.c_str(), WiFi.localIP().toString().c_str());
    } else {
      ESP_LOGE(WIFI_TAG, "WiFi connection failed and no fallback configured");
      setLedColor(255, 0, 0, true);
      return false;
    }
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
