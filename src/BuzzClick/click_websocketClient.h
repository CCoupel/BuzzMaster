#pragma once

#ifdef USE_WEBSOCKET

#include <ArduinoWebsockets.h>
#include <WiFi.h>
#include <ArduinoJson.h>
#include "Common/CustomLogger.h"
#include "Common/led.h"
#include "esp_task_wdt.h"

using namespace websockets;

static const char* WS_TAG = "WebSocket";

WebsocketsClient wsClient;
bool wsConnected = false;
String wsServerIP = "";
uint16_t wsServerPort = 80;

// Forward declaration from click_serverConnection.h
void parseJSON(const String& data, AsyncClient* c);

// Callback for WebSocket messages
void onWsMessage(WebsocketsMessage message) {
    ESP_LOGI(WS_TAG, "Received: %s", message.data().c_str());

    // Parse JSON message
    JsonDocument doc;
    DeserializationError error = deserializeJson(doc, message.data());

    if (error) {
        ESP_LOGE(WS_TAG, "JSON parse error: %s", error.c_str());
        return;
    }

    // Handle different message types
    const char* action = doc["ACTION"];
    if (action) {
        ESP_LOGI(WS_TAG, "Action: %s", action);

        // Forward all messages to game engine (parseJSON from click_serverConnection.h)
        parseJSON(message.data(), nullptr);
    }
}

// Callback for WebSocket events
void onWsEvent(WebsocketsEvent event, String data) {
    if (event == WebsocketsEvent::ConnectionOpened) {
        ESP_LOGI(WS_TAG, "WebSocket Connected!");
        wsConnected = true;

        // Send HELLO message
        String macAddr = WiFi.macAddress();
        String ipAddr = WiFi.localIP().toString();

        JsonDocument helloDoc;
        helloDoc["ID"] = macAddr;
        helloDoc["VERSION"] = String(VERSION);
        helloDoc["ACTION"] = "HELLO";

        JsonObject msg = helloDoc["MSG"].to<JsonObject>();
        msg["IP"] = ipAddr;
        msg["VERSION"] = String(VERSION);
        msg["firmware_version"] = String(FIRMWARE_VERSION);

        String helloJson;
        serializeJson(helloDoc, helloJson);

        ESP_LOGI(WS_TAG, "Sending HELLO: %s", helloJson.c_str());
        wsClient.send(helloJson);

        // === BOOT PHASE 5: GREEN 2/4 - WebSocket connected ===
        setLedColor(0, 255, 0, true, 0, 2);
        ESP_LOGI(WS_TAG, "Boot phase: GREEN 2/4 (WebSocket connected)");
        delay(500);
    }
    else if (event == WebsocketsEvent::ConnectionClosed) {
        ESP_LOGW(WS_TAG, "WebSocket Disconnected!");
        wsConnected = false;

        // Red LED = disconnected
        setLedColor(255, 0, 0, true);
    }
    else if (event == WebsocketsEvent::GotPing) {
        ESP_LOGD(WS_TAG, "Got ping");
    }
    else if (event == WebsocketsEvent::GotPong) {
        ESP_LOGD(WS_TAG, "Got pong");
    }
}

// Connect to WebSocket server
bool connectWebSocket(const String& ip, uint16_t port) {
    wsServerIP = ip;
    wsServerPort = port;

    // Build WebSocket URL: ws://192.168.1.84:80/ws/buzzer
    String wsUrl = "ws://" + ip + ":" + String(port) + "/ws/buzzer";

    ESP_LOGI(WS_TAG, "Connecting to WebSocket: %s", wsUrl.c_str());

    // === BOOT PHASE 4: ORANGE 2/4 - WebSocket connecting ===
    setLedColor(255, 165, 0, true, 0, 2);
    ESP_LOGI(WS_TAG, "Boot phase: ORANGE 2/4 (WebSocket connecting)");
    delay(500);

    // Set callbacks
    wsClient.onMessage(onWsMessage);
    wsClient.onEvent(onWsEvent);

    // Connect with timeout
    bool connected = wsClient.connect(wsUrl);

    if (!connected) {
        ESP_LOGE(WS_TAG, "WebSocket connection failed!");
        setLedColor(255, 0, 0, true);
        return false;
    }

    // Wait for connection to be fully established
    int attempts = 0;
    while (!wsConnected && attempts < 50) {
        wsClient.poll();
        delay(100);
        attempts++;
        esp_task_wdt_reset();
    }

    if (!wsConnected) {
        ESP_LOGE(WS_TAG, "WebSocket connection timeout");
        setLedColor(255, 0, 0, true);
        return false;
    }

    ESP_LOGI(WS_TAG, "WebSocket connected successfully!");
    return true;
}

// Poll WebSocket (call in loop)
void pollWebSocket() {
    if (wsClient.available()) {
        wsClient.poll();
    }
}

// Check WebSocket connection and reconnect if needed
void checkWebSocketConnection() {
    static unsigned long lastReconnectAttempt = 0;
    const unsigned long RECONNECT_INTERVAL = 10000; // 10 seconds

    if (!wsConnected) {
        unsigned long now = millis();
        if (now - lastReconnectAttempt > RECONNECT_INTERVAL) {
            ESP_LOGW(WS_TAG, "WebSocket disconnected, attempting reconnect...");
            lastReconnectAttempt = now;

            if (!wsServerIP.isEmpty()) {
                connectWebSocket(wsServerIP, wsServerPort);
            }
        }
    }
}

// Send button press via WebSocket
void sendBuzzWebSocket(int playerNum, unsigned long timestamp) {
    if (!wsConnected) {
        ESP_LOGW(WS_TAG, "Cannot send BUZZ - not connected");
        return;
    }

    JsonDocument buzzDoc;
    buzzDoc["ID"] = WiFi.macAddress();
    buzzDoc["ACTION"] = "BUZZ";
    buzzDoc["PLAYER"] = playerNum;
    buzzDoc["TIME"] = timestamp;

    String buzzJson;
    serializeJson(buzzDoc, buzzJson);

    ESP_LOGI(WS_TAG, "Sending BUZZ: %s", buzzJson.c_str());
    wsClient.send(buzzJson);
}

// Close WebSocket connection
void closeWebSocket() {
    if (wsConnected) {
        wsClient.close();
        wsConnected = false;
    }
}

// Wrapper function for button press (called from manageButtonMessages)
void ws_sendBuzz(const String& mac, const String& buttonName) {
    if (!wsConnected) {
        ESP_LOGW(WS_TAG, "Cannot send BUTTON - not connected");
        return;
    }

    JsonDocument buzzDoc;
    buzzDoc["ID"] = mac;
    buzzDoc["ACTION"] = "BUTTON";
    buzzDoc["MSG"]["button"] = buttonName;
    buzzDoc["VERSION"] = String(VERSION);

    String buzzJson;
    serializeJson(buzzDoc, buzzJson);

    ESP_LOGI(WS_TAG, "Sending BUTTON: %s", buzzJson.c_str());
    wsClient.send(buzzJson);
}

// Wrapper function for PONG (called from click_serverConnection.h)
void ws_sendPong(const String& mac) {
    if (!wsConnected) {
        ESP_LOGW(WS_TAG, "Cannot send PONG - not connected");
        return;
    }

    JsonDocument pongDoc;
    pongDoc["ID"] = mac;
    pongDoc["ACTION"] = "PONG";
    pongDoc["VERSION"] = String(VERSION);
    pongDoc["MSG"] = WiFi.localIP().toString();

    String pongJson;
    serializeJson(pongDoc, pongJson);

    ESP_LOGI(WS_TAG, "Sending PONG: %s", pongJson.c_str());
    wsClient.send(pongJson);
}

// Check if WebSocket is connected
bool ws_isConnected() {
    return wsConnected;
}

// Connect WebSocket (wrapper)
void ws_connect() {
    if (!wsServerIP.isEmpty()) {
        connectWebSocket(wsServerIP, wsServerPort);
    }
}

#endif // USE_WEBSOCKET
