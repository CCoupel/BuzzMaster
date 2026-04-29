#pragma once

#ifdef USE_WEBSOCKET

#include <WiFi.h>
#include <ArduinoJson.h>
#include "Common/CustomLogger.h"
#include "Common/led.h"
#include "click_ledErrorPatterns.h"
#include "click_broadcaster.h"
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
// Set true while connectWebSocket() is executing to prevent the main-loop
// checkWebSocketConnection() from triggering a concurrent reconnect.
// Race condition: loop() task and WiFi event task run concurrently on ESP32.
// Without this guard, checkWebSocketConnection() sees !wsConnected && wsClient!=NULL
// during init and calls esp_websocket_client_start() on an already-started handle.
volatile bool wsConnecting = false;
// Reconnect state machine for checkWebSocketConnection():
//   wsDisconnectedImmediate=true  → attempt reconnect on next loop() tick (no interval)
//   wsWaitingForBroadcast=true    → immediate reconnect failed; wait for UDP heartbeat
// Both flags are cleared when wsConnected goes true again.
volatile bool wsDisconnectedImmediate = false;
volatile bool wsWaitingForBroadcast = false;
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

            // Link restored — drop any WS_DISCONNECTED / WS_TIMEOUT error
            // pattern so the boot-phase GREEN set below is actually visible.
            clearLedError();

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
            wsDisconnectedImmediate = true;
            wsWaitingForBroadcast = false;

            // Spinner overlay on current game colour — immediate, non-intrusive.
            setLedError(LedErrorPattern::WS_RECONNECTING);
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
            wsDisconnectedImmediate = true;
            wsWaitingForBroadcast = false;
            setLedError(LedErrorPattern::WS_RECONNECTING);
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
// Flag set by ws_destroy_task when stop()+destroy() have completed.
static volatile bool wsDestroyComplete = false;

// Background FreeRTOS task: performs the blocking stop()+destroy() teardown so
// the calling task (loop()) can keep animating the spinner via manageLedError().
// Root cause: esp_websocket_client_stop() blocks until the internal ESP-IDF WS
// task exits its network poll loop — up to `network_timeout_ms` ms (default 10s,
// observed ~4s) when the server is unreachable. Running it on a separate task
// lets loop() continue executing while the TCP stack times out.
// Safety: generation counter + wsClient=NULL are set BEFORE spawning the task, so
// the event handler ignores any late callbacks. The destroy task only calls ESP-IDF
// WS/TCP functions — it never touches the SPI LED bus (no concurrency issue).
static void ws_destroy_task(void* pvParam) {
    esp_websocket_client_handle_t handle = (esp_websocket_client_handle_t)pvParam;
    esp_websocket_client_stop(handle);
    // Brief drain so FreeRTOS event loop flushes residual dispatches before free.
    vTaskDelay(pdMS_TO_TICKS(50));
    esp_websocket_client_destroy(handle);
    wsDestroyComplete = true;
    vTaskDelete(NULL);
}

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

    // 3. Graceful close — sends CLOSE frame. Timeout 0 = non-blocking fire-and-forget;
    //    we do NOT wait for the ACK since the peer may already be gone.
    esp_websocket_client_close(handle, pdMS_TO_TICKS(0));

    // 4. Offload blocking stop()+destroy() to a background task so loop() can keep
    //    animating the spinner while the TCP stack drains (avoids 4s freeze).
    wsDestroyComplete = false;
    xTaskCreate(ws_destroy_task, "ws_destroy", 4096, (void*)handle, 5, NULL);

    // 5. Animate spinner while background teardown runs (100ms ticks).
    while (!wsDestroyComplete) {
        manageLedError();
        delay(100);
        esp_task_wdt_reset();
    }

    ESP_LOGD(WS_TAG, "ws_safe_destroy: client destroyed (gen=%u)", (unsigned)wsGeneration);
}

// Connect to WebSocket server.
// isReconnect=true: skip boot-phase LEDs, use WS_RECONNECTING spinner instead.
bool connectWebSocket(const String& ip, uint16_t port, bool isReconnect = false) {
    wsConnecting = true;
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
    ws_cfg.task_stack = 8192;  // default 4096 overflows when ota_task sends concurrently

    wsClient = esp_websocket_client_init(&ws_cfg);
    if (!wsClient) {
        ESP_LOGE(WS_TAG, "Failed to initialize WebSocket client!");
        setLedError(isReconnect ? LedErrorPattern::WS_RECONNECTING : LedErrorPattern::WS_DISCONNECTED);
        wsConnecting = false;
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

    if (!isReconnect) {
        // === BOOT PHASE 4: ORANGE 2/4 - WebSocket connecting ===
        setLedColor(255, 165, 0, true, 0, 2);
        ESP_LOGI(WS_TAG, "Boot phase: ORANGE 2/4 (WebSocket connecting)");
        delay(500);
    }

    // Start WebSocket client
    esp_err_t err = esp_websocket_client_start((esp_websocket_client_handle_t)wsClient);
    if (err != ESP_OK) {
        ESP_LOGE(WS_TAG, "WebSocket client start failed: %d", err);
        setLedError(isReconnect ? LedErrorPattern::WS_RECONNECTING : LedErrorPattern::WS_DISCONNECTED);
        ws_safe_destroy();
        wsConnecting = false;
        return false;
    }

    // Wait for connection (up to 10 seconds).
    // manageLedError() is called each iteration so the WS_RECONNECTING spinner
    // keeps animating while loop() is blocked here.
    int attempts = 0;
    while (!wsConnected && attempts < 100) {
        delay(100);
        manageLedError();
        attempts++;
        esp_task_wdt_reset();
    }

    if (!wsConnected) {
        ESP_LOGE(WS_TAG, "WebSocket connection timeout!");
        // Reconnect attempts keep the spinner — checkWebSocketConnection() will
        // retry indefinitely. Boot-time timeout uses the slow pulse (WS_TIMEOUT)
        // which is distinct but will also clear once the reconnect loop kicks in.
        setLedError(isReconnect ? LedErrorPattern::WS_RECONNECTING : LedErrorPattern::WS_TIMEOUT);
        ws_safe_destroy();
        wsConnecting = false;
        return false;
    }

    ESP_LOGI(WS_TAG, "WebSocket connected successfully!");
    wsConnecting = false;
    return true;
}

// Poll WebSocket (not needed with ESP-IDF client - it has its own task)
void pollWebSocket() {
    // Nothing to do - ESP-IDF client handles this internally
}

// Check WebSocket connection and reconnect if needed.
//
// Reconnect state machine (3 states):
//   1. IMMEDIATE (wsDisconnectedImmediate=true):
//      Fires on the loop() tick after a DISCONNECTED/ERROR event.
//      Attempts one reconnect to the last known server immediately.
//      → success: clear flags, done.
//      → failure: enter WAIT_UDP (start UDP broadcast listener).
//
//   2. WAIT_UDP (wsWaitingForBroadcast=true):
//      Waits for a UDP heartbeat from the server broadcaster.
//      No periodic retry — the spinner keeps running via manageLedError().
//      → heartbeat arrives: try each IP from heartbeat in order.
//        success: clear flags, done.
//        all fail: reset discovery, keep waiting for next heartbeat.
//
//   3. IDLE (both flags false, wsConnected=true): nothing to do.
//
// WiFiGotIP (reconnect path) resets both flags and sets wsDisconnectedImmediate=true
// so that a WiFi drop+reconnect also re-enters state 1.
void checkWebSocketConnection() {
    if (wsConnecting) return;

    if (wsConnected) {
        wsDisconnectedImmediate = false;
        wsWaitingForBroadcast = false;
        return;
    }

    if (wsServerIP.isEmpty()) return;

    if (wsDisconnectedImmediate) {
        wsDisconnectedImmediate = false;
        ESP_LOGW(WS_TAG, "WS disconnected — immediate reconnect attempt to %s:%d",
                 wsServerIP.c_str(), (int)wsServerPort);
        setLedError(LedErrorPattern::WS_RECONNECTING);
        if (wsClient != NULL) ws_safe_destroy();
        if (connectWebSocket(wsServerIP, wsServerPort, true)) {
            return;  // reconnected — flags cleared on next call via wsConnected=true
        }
        // Immediate attempt failed — fall back to UDP broadcast discovery.
        ESP_LOGW(WS_TAG, "Immediate reconnect failed — waiting for UDP broadcast");
        wsWaitingForBroadcast = true;
        resetBroadcastDiscovery();
        stopBroadcastListener();
        startBroadcastListener(CONTROLER_PORT);
        return;
    }

    if (wsWaitingForBroadcast) {
        if (!hasBroadcastDiscovery()) return;  // keep spinner, wait for heartbeat
        // New server IPs received — try each one.
        stopBroadcastListener();
        const BroadcastDiscovery& disc = getBroadcastDiscovery();
        for (int i = 0; i < disc.ipCount; i++) {
            ESP_LOGI(WS_TAG, "Broadcast reconnect: trying %s:%d",
                     disc.ips[i].c_str(), disc.serverPort);
            if (connectWebSocket(disc.ips[i], disc.serverPort, true)) {
                wsWaitingForBroadcast = false;
                return;
            }
        }
        // All IPs failed — reset discovery and wait for next heartbeat.
        ESP_LOGW(WS_TAG, "All broadcast IPs failed — waiting for next heartbeat");
        resetBroadcastDiscovery();
        startBroadcastListener(CONTROLER_PORT);
        return;
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

// Send ACK response for a received message with MSG_ID (v3.8.0 — #54).
// Called from parseJSON() which executes inside the WebSocket event handler task.
// MUST be non-blocking: portMAX_DELAY would deadlock (event task waiting for itself).
// Timeout 0 = non-blocking fire-and-forget; drops silently if send queue is full.
// The server handles non-acknowledgement via AckManager retry/expiry.
void ws_sendAck(const String& mac, const String& ackAction, const String& ackId) {
    if (!wsConnected || !wsClient) {
        ESP_LOGD(WS_TAG, "ws_sendAck: not connected, dropping ACK for %s", ackId.c_str());
        return;
    }

    JsonDocument ackDoc;
    ackDoc["ID"] = mac;
    ackDoc["ACTION"] = "ACK";
    JsonObject ackMsg = ackDoc["MSG"].to<JsonObject>();
    ackMsg["ack_action"] = ackAction;
    ackMsg["ack_id"] = ackId;

    String ackJson;
    serializeJson(ackDoc, ackJson);

    // timeout=0: non-blocking — drops if the internal WS send ring-buffer is full.
    // Safe to call from the WS event handler task (parseJSON context).
    esp_err_t err = esp_websocket_client_send_text(
        (esp_websocket_client_handle_t)wsClient,
        ackJson.c_str(),
        ackJson.length(),
        0
    );
    if (err == ESP_OK) {
        ESP_LOGI(WS_TAG, "ACK sent: action=%s id=%s", ackAction.c_str(), ackId.c_str());
    } else {
        ESP_LOGW(WS_TAG, "ACK send failed (err=%d) action=%s id=%s", err, ackAction.c_str(), ackId.c_str());
    }
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
