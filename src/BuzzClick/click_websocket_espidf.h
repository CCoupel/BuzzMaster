#pragma once

#ifdef USE_WEBSOCKET

#include <WiFi.h>
#include <ArduinoJson.h>
#include "Common/CustomLogger.h"
#include "Common/led.h"
#include "esp_task_wdt.h"
#include "esp_websocket_client.h"

static const char* WS_TAG = "WebSocket";

// volatile: ensures the event handler (which runs on the ESP-IDF internal
// FreeRTOS task) reads the current value instead of a cached register copy.
// Critical for the NULL guard below — without volatile the compiler may
// hoist the read outside the handler's entry and skip the guard entirely.
volatile esp_websocket_client_handle_t wsClient = NULL;
volatile bool wsConnected = false;
// Generation counter incremented on every destroy — event handler checks
// that the current generation matches the one at entry; stale events from
// a previously destroyed client are discarded even if a new client has
// been created in the meantime (wsClient would otherwise look "valid").
volatile uint32_t wsGeneration = 0;
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
// Guard against stale callbacks after esp_websocket_client_destroy():
//   1. wsClient NULL check — immediate early return
//   2. Generation counter — discards events from a prior destroyed client even
//      if a new one has been created (wsClient would otherwise look "valid")
// Both guards are read from volatile storage so the compiler cannot hoist or
// cache them. No logging inside the early-return path — logging may allocate
// and the goal is to minimize work in stale callbacks.
static void ws_event_handler(void *handler_args, esp_event_base_t base, int32_t event_id, void *event_data) {
    // Capture the generation this handler was registered at (stored in handler_args).
    uint32_t myGeneration = (uint32_t)(uintptr_t)handler_args;

    // Guard 1: client destroyed
    if (wsClient == NULL) {
        return;
    }

    // Guard 2: this event belongs to a destroyed generation
    if (myGeneration != wsGeneration) {
        return;
    }

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
                esp_websocket_client_send_text((esp_websocket_client_handle_t)wsClient, helloJson.c_str(), helloJson.length(), portMAX_DELAY);
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

// Safe teardown helper — always call this instead of destroy() directly.
//
// The v3.5.3 fix set wsClient=NULL before destroy to guard the event handler,
// but a stale event could still fire between destroy() of the old client and
// init() of the new client — at that moment wsClient points to the NEW handle
// so the NULL guard passes, yet the event carries stale data from the OLD
// destroyed client. Root cause of the Instruction access fault (MCAUSE=1,
// MEPC=0x00010000, T1=0x00010000): the ESP-IDF internal task dispatches a
// DISCONNECTED/ERROR event with a function pointer cached from the freed
// client structure. MEPC=0x00010000 is the app partition offset — classic
// signature of a dangling function pointer read from freed memory.
//
// Fix: generation counter invalidates stale events from any previous client,
// regardless of wsClient pointer state. Plus graceful close (esp_websocket_
// client_close) waits for the internal task to drain before stop()/destroy().
static void ws_safe_destroy() {
    if (wsClient == NULL) return;

    esp_websocket_client_handle_t handle = (esp_websocket_client_handle_t)wsClient;

    // 1. Invalidate the generation FIRST — any in-flight or future events
    //    from this client will now fail the generation check in the handler.
    //    This is atomic (single 32-bit write on RISC-V) and visible to other
    //    tasks immediately (volatile).
    wsGeneration++;

    // 2. Signal the event handler to ignore any further callbacks.
    wsConnected = false;
    wsClient = NULL;

    // 3. Graceful close — sends CLOSE frame and waits for the internal task
    //    to acknowledge. This drains pending events cleanly, unlike stop()
    //    which may leave events queued in the event loop.
    esp_websocket_client_close(handle, pdMS_TO_TICKS(500));

    // 4. Ensure the client task has fully exited.
    esp_websocket_client_stop(handle);

    // 5. Let the FreeRTOS event loop drain any residual dispatches before
    //    we free the handle memory. 200ms is conservative but cheap — this
    //    path runs only on connection failure, not on the hot path.
    delay(200);

    // 6. Destroy the handle.
    esp_websocket_client_destroy(handle);
    ESP_LOGD(WS_TAG, "ws_safe_destroy: client destroyed (gen=%u)", (unsigned)wsGeneration);
}

// Connect to WebSocket server
bool connectWebSocket(const String& ip, uint16_t port) {
    if (wsClient != NULL) {
        ws_safe_destroy();
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

    // Bump generation for the new client. The event handler receives the
    // current generation in handler_args (encoded as uintptr_t) so it can
    // detect events dispatched from a prior (destroyed) generation and
    // discard them — prevents Instruction access fault from stale callbacks.
    wsGeneration++;
    uintptr_t generationArg = (uintptr_t)wsGeneration;

    // Register event handler with generation as context
    esp_websocket_register_events((esp_websocket_client_handle_t)wsClient,
                                  WEBSOCKET_EVENT_ANY,
                                  ws_event_handler,
                                  (void *)generationArg);

    // === BOOT PHASE 4: ORANGE 2/4 - WebSocket connecting ===
    setLedColor(255, 165, 0, true, 0, 2);
    ESP_LOGI(WS_TAG, "Boot phase: ORANGE 2/4 (WebSocket connecting)");
    delay(500);

    // Start WebSocket client
    esp_err_t err = esp_websocket_client_start((esp_websocket_client_handle_t)wsClient);
    if (err != ESP_OK) {
        ESP_LOGE(WS_TAG, "WebSocket client start failed: %d", err);
        setLedColor(255, 0, 0, true);
        // Release the initialized (but unstarted) handle — otherwise wsClient
        // stays non-NULL and leaks on the ESP32-C3 heap across retries.
        ws_safe_destroy();
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
        ws_safe_destroy();  // safe: sets wsClient=NULL before destroy to drain stale events
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

            ws_safe_destroy();  // safe teardown before reconnect

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
    esp_websocket_client_send_text((esp_websocket_client_handle_t)wsClient, buzzJson.c_str(), buzzJson.length(), portMAX_DELAY);
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
    esp_websocket_client_send_text((esp_websocket_client_handle_t)wsClient, pongJson.c_str(), pongJson.length(), portMAX_DELAY);
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
    esp_websocket_client_send_text((esp_websocket_client_handle_t)wsClient, json.c_str(), json.length(), portMAX_DELAY);
}

// Close WebSocket connection
void closeWebSocket() {
    ws_safe_destroy();  // safe: NULL guard before destroy drains stale events
}

#endif // USE_WEBSOCKET
