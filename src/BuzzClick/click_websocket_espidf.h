#pragma once

#ifdef USE_WEBSOCKET

#include <WiFi.h>
#include <ArduinoJson.h>
#include "Common/CustomLogger.h"
#include "Common/led.h"
#include "esp_task_wdt.h"
#include "esp_websocket_client.h"

static const char* WS_TAG = "WebSocket";

esp_websocket_client_handle_t wsClient = NULL;
bool wsConnected = false;
String wsServerIP = "";
uint16_t wsServerPort = 80;

// Buffer for fragmented messages
String fragmentBuffer = "";

// Max buffer size: 64KB
const size_t MAX_FRAGMENT_BUFFER_SIZE = 65536;

// Forward declaration from click_serverConnection.h
void parseJSON(const String& data, AsyncClient* c);

// Helper function to detect complete JSON by counting braces
bool isCompleteJSON(const String& buffer) {
    if (buffer.length() == 0) return false;

    int braceCount = 0;
    bool inString = false;
    bool escape = false;

    for (size_t i = 0; i < buffer.length(); i++) {
        char c = buffer.charAt(i);

        // Handle escape sequences in strings
        if (escape) {
            escape = false;
            continue;
        }

        if (c == '\\') {
            escape = true;
            continue;
        }

        // Track if we're inside a string (ignore braces in strings)
        if (c == '"') {
            inString = !inString;
            continue;
        }

        // Count braces only outside of strings
        if (!inString) {
            if (c == '{') {
                braceCount++;
            } else if (c == '}') {
                braceCount--;

                // Complete JSON when brace count returns to 0
                if (braceCount == 0) {
                    return true;
                }
            }
        }
    }

    return false;
}

// Callback for WebSocket events
static void ws_event_handler(void *handler_args, esp_event_base_t base, int32_t event_id, void *event_data) {
    esp_websocket_event_data_t *data = (esp_websocket_event_data_t *)event_data;

    switch (event_id) {
        case WEBSOCKET_EVENT_CONNECTED:
            ESP_LOGI(WS_TAG, "WebSocket CONNECTED!");
            wsConnected = true;

            // Send HELLO message immediately
            {
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
                esp_websocket_client_send_text(wsClient, helloJson.c_str(), helloJson.length(), portMAX_DELAY);
            }

            // === BOOT PHASE 5: GREEN 2/4 - WebSocket connected ===
            setLedColor(0, 255, 0, true, 0, 2);
            ESP_LOGI(WS_TAG, "Boot phase: GREEN 2/4 (WebSocket connected)");
            delay(500);
            break;

        case WEBSOCKET_EVENT_DISCONNECTED:
            ESP_LOGW(WS_TAG, "WebSocket DISCONNECTED!");
            wsConnected = false;

            // Red LED = disconnected
            setLedColor(255, 0, 0, true);
            break;

        case WEBSOCKET_EVENT_DATA:
            // Handle fragmented messages
            if (data->op_code == 0x01) {  // Text frame
                String fragment = String((char*)data->data_ptr, data->data_len);

                // Check buffer overflow
                if (fragmentBuffer.length() + fragment.length() > MAX_FRAGMENT_BUFFER_SIZE) {
                    ESP_LOGE(WS_TAG, "Fragment buffer overflow (%d bytes), discarding",
                             fragmentBuffer.length() + fragment.length());
                    fragmentBuffer = "";
                    break;
                }

                // Accumulate fragment
                fragmentBuffer += fragment;

                // Check if we have a complete JSON message by counting braces
                if (isCompleteJSON(fragmentBuffer)) {
                    ESP_LOGD(WS_TAG, "Received complete message: %d bytes", fragmentBuffer.length());
                    parseJSON(fragmentBuffer, nullptr);

                    // Reset buffer for next message
                    fragmentBuffer = "";
                } else {
                    // Incomplete message - wait for more fragments
                    ESP_LOGD(WS_TAG, "Buffering fragment (%d bytes), total: %d bytes",
                             data->data_len, fragmentBuffer.length());
                }
            }
            break;

        case WEBSOCKET_EVENT_ERROR:
            ESP_LOGE(WS_TAG, "WebSocket ERROR!");
            wsConnected = false;
            setLedColor(255, 0, 0, true);
            break;

        default:
            ESP_LOGD(WS_TAG, "WebSocket event: %d", event_id);
            break;
    }
}

// Connect to WebSocket server
bool connectWebSocket(const String& ip, uint16_t port) {
    if (wsClient != NULL) {
        esp_websocket_client_stop(wsClient);
        esp_websocket_client_destroy(wsClient);
        wsClient = NULL;
        wsConnected = false;
    }

    wsServerIP = ip;
    wsServerPort = port;

    // Build WebSocket URL: ws://192.168.1.84:80/ws/buzzer
    String wsUrl = "ws://" + ip + ":" + String(port) + "/ws/buzzer";

    ESP_LOGI(WS_TAG, "Connecting to WebSocket: %s", wsUrl.c_str());

    // Configure WebSocket client
    esp_websocket_client_config_t ws_cfg = {};
    ws_cfg.uri = wsUrl.c_str();

    wsClient = esp_websocket_client_init(&ws_cfg);
    if (!wsClient) {
        ESP_LOGE(WS_TAG, "Failed to initialize WebSocket client!");
        setLedColor(255, 0, 0, true);
        return false;
    }

    // Register event handler
    esp_websocket_register_events(wsClient, WEBSOCKET_EVENT_ANY, ws_event_handler, NULL);

    // === BOOT PHASE 4: ORANGE 2/4 - WebSocket connecting ===
    setLedColor(255, 165, 0, true, 0, 2);
    ESP_LOGI(WS_TAG, "Boot phase: ORANGE 2/4 (WebSocket connecting)");
    delay(500);

    // Start WebSocket client
    esp_err_t err = esp_websocket_client_start(wsClient);
    if (err != ESP_OK) {
        ESP_LOGE(WS_TAG, "WebSocket client start failed: %d", err);
        setLedColor(255, 0, 0, true);
        return false;
    }

    // Wait for connection (up to 10 seconds)
    int attempts = 0;
    while (!wsConnected && attempts < 100) {
        delay(100);
        attempts++;
        esp_task_wdt_reset();
    }

    if (!wsConnected) {
        ESP_LOGE(WS_TAG, "WebSocket connection timeout!");
        setLedColor(255, 0, 0, true);
        esp_websocket_client_stop(wsClient);
        esp_websocket_client_destroy(wsClient);
        wsClient = NULL;
        return false;
    }

    ESP_LOGI(WS_TAG, "WebSocket connected successfully!");
    return true;
}

// Poll WebSocket (not needed with ESP-IDF client - it has its own task)
void pollWebSocket() {
    // Nothing to do - ESP-IDF client handles this internally
}

// Check WebSocket connection and reconnect if needed
void checkWebSocketConnection() {
    static unsigned long lastReconnectAttempt = 0;
    const unsigned long RECONNECT_INTERVAL = 10000; // 10 seconds

    if (!wsConnected && wsClient != NULL) {
        unsigned long now = millis();
        if (now - lastReconnectAttempt > RECONNECT_INTERVAL) {
            ESP_LOGW(WS_TAG, "WebSocket disconnected, attempting reconnect...");
            lastReconnectAttempt = now;

            esp_websocket_client_stop(wsClient);
            esp_websocket_client_destroy(wsClient);
            wsClient = NULL;

            if (!wsServerIP.isEmpty()) {
                connectWebSocket(wsServerIP, wsServerPort);
            }
        }
    }
}

// Send button press via WebSocket
void ws_sendBuzz(const String& mac, const String& buttonName) {
    if (!wsConnected || !wsClient) {
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
    esp_websocket_client_send_text(wsClient, buzzJson.c_str(), buzzJson.length(), portMAX_DELAY);
}

// Send PONG
void ws_sendPong(const String& mac) {
    if (!wsConnected || !wsClient) {
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
    esp_websocket_client_send_text(wsClient, pongJson.c_str(), pongJson.length(), portMAX_DELAY);
}

// Check if WebSocket is connected
bool ws_isConnected() {
    return wsConnected && wsClient != NULL;
}

// Connect WebSocket (wrapper)
void ws_connect() {
    if (!wsServerIP.isEmpty()) {
        connectWebSocket(wsServerIP, wsServerPort);
    }
}

// Send raw JSON string via WebSocket (used by OTA manager and other modules)
void ws_sendRaw(const String& json) {
    if (!wsConnected || !wsClient) {
        ESP_LOGW(WS_TAG, "Cannot send message - not connected");
        return;
    }
    esp_websocket_client_send_text(wsClient, json.c_str(), json.length(), portMAX_DELAY);
}

// Close WebSocket connection
void closeWebSocket() {
    if (wsClient) {
        esp_websocket_client_stop(wsClient);
        esp_websocket_client_destroy(wsClient);
        wsClient = NULL;
        wsConnected = false;
    }
}

#endif // USE_WEBSOCKET
