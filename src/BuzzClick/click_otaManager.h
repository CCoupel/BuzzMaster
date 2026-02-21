#pragma once

#ifdef USE_WEBSOCKET

#include <Update.h>
#include <HTTPClient.h>
#include <WiFi.h>
#include <ArduinoJson.h>
#include "Common/led.h"
#include "esp_log.h"

static const char* OTA_TAG = "OTA";

// Forward declaration - ws_sendRaw is defined in click_websocket_espidf.h
// which is included before click_otaManager.h in click_MAIN.cpp
void ws_sendRaw(const String& json);

// Send an OTA_PROGRESS message via WebSocket.
// status : "downloading", "flashing", "done" or "error"
// percent: 0-100
// error  : optional error message (only for status="error")
void sendOTAProgress(const String& status, int percent, const String& error = "") {
    String macAddr = WiFi.macAddress();

    JsonDocument doc;
    doc["ACTION"] = "OTA_PROGRESS";
    doc["ID"] = macAddr;

    JsonObject msg = doc["MSG"].to<JsonObject>();
    msg["STATUS"] = status;
    msg["PERCENT"] = percent;

    if (error.length() > 0) {
        msg["ERROR"] = error;
    }

    String json;
    serializeJson(doc, json);

    ESP_LOGI(OTA_TAG, "OTA_PROGRESS: %s", json.c_str());
    ws_sendRaw(json);
}

// Perform OTA firmware update via HTTP download and flash.
// url             : full HTTP URL to the firmware .bin file
// expectedVersion : version string for logging purposes
void performOTA(const String& url, const String& expectedVersion) {
    ESP_LOGI(OTA_TAG, "Starting OTA update to version %s from: %s",
             expectedVersion.c_str(), url.c_str());

    // Signal start of download phase
    sendOTAProgress("downloading", 0);

    // Blue blinking LED during download
    setLedColor(0, 0, 255, true);

    HTTPClient http;
    http.begin(url);

    // Follow redirects
    http.setFollowRedirects(HTTPC_STRICT_FOLLOW_REDIRECTS);

    int httpCode = http.GET();
    if (httpCode != HTTP_CODE_OK) {
        String errMsg = "HTTP error: " + String(httpCode);
        ESP_LOGE(OTA_TAG, "%s", errMsg.c_str());
        sendOTAProgress("error", 0, errMsg);
        // Red LED = error
        setLedColor(255, 0, 0, true);
        http.end();
        return;
    }

    int contentLength = http.getSize();
    if (contentLength <= 0) {
        String errMsg = "Invalid content length: " + String(contentLength);
        ESP_LOGE(OTA_TAG, "%s", errMsg.c_str());
        sendOTAProgress("error", 0, errMsg);
        setLedColor(255, 0, 0, true);
        http.end();
        return;
    }

    ESP_LOGI(OTA_TAG, "Firmware size: %d bytes", contentLength);

    // Initialize OTA update on the FLASH partition
    if (!Update.begin(contentLength, U_FLASH)) {
        String errMsg = String("Update.begin failed: ") + Update.errorString();
        ESP_LOGE(OTA_TAG, "%s", errMsg.c_str());
        sendOTAProgress("error", 0, errMsg);
        setLedColor(255, 0, 0, true);
        http.end();
        return;
    }

    // Get the HTTP stream for incremental reading
    WiFiClient* stream = http.getStreamPtr();

    // Write firmware in 1024-byte chunks
    const size_t CHUNK_SIZE = 1024;
    uint8_t buf[CHUNK_SIZE];
    int written = 0;
    int lastReportedPercent = -1;

    while (http.connected() && written < contentLength) {
        size_t available = stream->available();
        if (available > 0) {
            size_t toRead = (available > CHUNK_SIZE) ? CHUNK_SIZE : available;
            size_t read = stream->readBytes(buf, toRead);

            if (read > 0) {
                size_t w = Update.write(buf, read);
                if (w != read) {
                    String errMsg = String("Write mismatch: ") + Update.errorString();
                    ESP_LOGE(OTA_TAG, "%s", errMsg.c_str());
                    sendOTAProgress("error", 0, errMsg);
                    setLedColor(255, 0, 0, true);
                    http.end();
                    return;
                }
                written += read;

                // Report progress every 10%
                int percent = (written * 100) / contentLength;
                int reportBucket = percent / 10;
                int lastBucket = (lastReportedPercent < 0) ? -1 : (lastReportedPercent / 10);

                if (reportBucket > lastBucket) {
                    sendOTAProgress("downloading", percent);
                    lastReportedPercent = percent;

                    // Blink blue LED during download
                    static bool blinkState = false;
                    blinkState = !blinkState;
                    setLedColor(0, 0, blinkState ? 255 : 64, true);
                }
            }
        } else {
            // Wait a moment for more data to arrive
            delay(10);
        }
    }

    http.end();

    ESP_LOGI(OTA_TAG, "Download complete: %d bytes written", written);

    if (written != contentLength) {
        String errMsg = "Incomplete download: got " + String(written) + "/" + String(contentLength);
        ESP_LOGE(OTA_TAG, "%s", errMsg.c_str());
        sendOTAProgress("error", 0, errMsg);
        setLedColor(255, 0, 0, true);
        return;
    }

    // Signal flashing phase (finalizing write to flash)
    sendOTAProgress("flashing", 100);

    // Finalize flash write
    if (!Update.end(true)) {
        String errMsg = String("Update.end failed: ") + Update.errorString();
        ESP_LOGE(OTA_TAG, "%s", errMsg.c_str());
        sendOTAProgress("error", 0, errMsg);
        // Rollback is automatic on error - do NOT restart
        setLedColor(255, 0, 0, true);
        return;
    }

    if (Update.hasError()) {
        String errMsg = String("Update error: ") + Update.errorString();
        ESP_LOGE(OTA_TAG, "%s", errMsg.c_str());
        sendOTAProgress("error", 0, errMsg);
        // Rollback is automatic on error - do NOT restart
        setLedColor(255, 0, 0, true);
        return;
    }

    ESP_LOGI(OTA_TAG, "OTA flash complete! Restarting in 2 seconds...");

    // Signal success
    sendOTAProgress("done", 100);

    // Green LED = success
    setLedColor(0, 255, 0, true);

    delay(2000);
    ESP.restart();
}

#endif // USE_WEBSOCKET
