#pragma once
#include "click_nvsConfig.h"
#include "esp_log.h"

static const char* USB_TAG = "USB_CONFIG";
static const unsigned long USB_CONFIG_BAUD = 115200;

static String usbInputBuffer = "";

// Timestamp of last AT command received (for LED fast blink feedback)
unsigned long lastAtCommandTime = 0;

// Staging area for AT commands before AT+SAVE
static String staged_ssid = "";
static String staged_pass = "";
static String staged_server_ip = "";
static int staged_server_port = -1;  // -1 = not staged

void usbSendResponse(const String& response) {
    Serial.println(response);
}

void usbShowConfig() {
    BuzzClickConfig& cfg = nvsGetConfig();
    usbSendResponse("+SSID:" + cfg.wifi_ssid);
    usbSendResponse("+PASS:" + cfg.wifi_password);
    usbSendResponse("+SERVERIP:" + cfg.server_ip);
    usbSendResponse("+SERVERPORT:" + String(cfg.server_tcp_port));
    usbSendResponse("+VALID:" + String(cfg.valid ? "YES" : "NO"));
    usbSendResponse("OK");
}

void usbShowStaged() {
    usbSendResponse("+STAGED_SSID:" + staged_ssid);
    usbSendResponse("+STAGED_PASS:" + staged_pass);
    usbSendResponse("+STAGED_SERVERIP:" + staged_server_ip);
    usbSendResponse("+STAGED_SERVERPORT:" + String(staged_server_port));
    usbSendResponse("OK");
}

void usbProcessCommand(const String& line) {
    String cmd = line;
    cmd.trim();

    if (cmd.length() == 0) return;

    ESP_LOGI(USB_TAG, "AT command: %s", cmd.c_str());

    // AT - basic test
    if (cmd == "AT") {
        usbSendResponse("OK");
        return;
    }

    // AT+SHOW - display current NVS config
    if (cmd == "AT+SHOW") {
        usbShowConfig();
        return;
    }

    // AT+STAGED - display staged values
    if (cmd == "AT+STAGED") {
        usbShowStaged();
        return;
    }

    // AT+SSID=value
    if (cmd.startsWith("AT+SSID=")) {
        staged_ssid = cmd.substring(8);
        if (staged_ssid.length() == 0) {
            usbSendResponse("ERROR:SSID cannot be empty");
            return;
        }
        if (staged_ssid.length() > 32) {
            usbSendResponse("ERROR:SSID too long (max 32)");
            staged_ssid = "";
            return;
        }
        usbSendResponse("+SSID:" + staged_ssid);
        usbSendResponse("OK");
        return;
    }

    // AT+PASS=value
    if (cmd.startsWith("AT+PASS=")) {
        staged_pass = cmd.substring(8);
        if (staged_pass.length() > 0 && staged_pass.length() < 8) {
            usbSendResponse("ERROR:Password too short (min 8)");
            staged_pass = "";
            return;
        }
        if (staged_pass.length() > 63) {
            usbSendResponse("ERROR:Password too long (max 63)");
            staged_pass = "";
            return;
        }
        usbSendResponse("+PASS:" + staged_pass);
        usbSendResponse("OK");
        return;
    }

    // AT+SERVERIP=value
    if (cmd.startsWith("AT+SERVERIP=")) {
        staged_server_ip = cmd.substring(12);
        // Basic IP format validation
        if (staged_server_ip.length() > 0) {
            IPAddress testIP;
            if (!testIP.fromString(staged_server_ip)) {
                usbSendResponse("ERROR:Invalid IP format");
                staged_server_ip = "";
                return;
            }
        }
        usbSendResponse("+SERVERIP:" + staged_server_ip);
        usbSendResponse("OK");
        return;
    }

    // AT+SERVERPORT=value
    if (cmd.startsWith("AT+SERVERPORT=")) {
        String portStr = cmd.substring(14);
        int port = portStr.toInt();
        if (port < 1 || port > 65535) {
            usbSendResponse("ERROR:Port must be 1-65535");
            return;
        }
        staged_server_port = port;
        usbSendResponse("+SERVERPORT:" + String(staged_server_port));
        usbSendResponse("OK");
        return;
    }

    // AT+SAVE - persist staged values to NVS and reboot
    if (cmd == "AT+SAVE") {
        BuzzClickConfig& cfg = nvsGetConfig();

        // Apply staged values (only if they were set)
        if (staged_ssid.length() > 0) cfg.wifi_ssid = staged_ssid;
        if (staged_pass.length() > 0) cfg.wifi_password = staged_pass;
        if (staged_server_ip.length() > 0) cfg.server_ip = staged_server_ip;
        if (staged_server_port > 0) cfg.server_tcp_port = (uint16_t)staged_server_port;

        if (cfg.wifi_ssid.length() == 0) {
            usbSendResponse("ERROR:SSID is required. Use AT+SSID=<value> first");
            return;
        }

        if (nvsSaveConfig()) {
            usbSendResponse("+SAVED:SSID=" + cfg.wifi_ssid + ",IP=" + cfg.server_ip + ",PORT=" + String(cfg.server_tcp_port));
            usbSendResponse("OK");
            usbSendResponse("+REBOOTING");
            Serial.flush();
            delay(500);
            ESP.restart();
        } else {
            usbSendResponse("ERROR:Failed to save to NVS");
        }
        return;
    }

    // AT+FACTORY - clear NVS and reboot
    if (cmd == "AT+FACTORY") {
        nvsClearConfig();
        usbSendResponse("+FACTORY:Config cleared");
        usbSendResponse("OK");
        usbSendResponse("+REBOOTING");
        Serial.flush();
        delay(500);
        ESP.restart();
        return;
    }

    // AT+VERSION
    if (cmd == "AT+VERSION") {
        usbSendResponse("+VERSION:" + String(VERSION));
        usbSendResponse("OK");
        return;
    }

    // AT+MAC
    if (cmd == "AT+MAC") {
        usbSendResponse("+MAC:" + WiFi.macAddress());
        usbSendResponse("OK");
        return;
    }

    // AT+HELP
    if (cmd == "AT+HELP") {
        usbSendResponse("BuzzClick AT Commands:");
        usbSendResponse("  AT              - Test connection");
        usbSendResponse("  AT+SSID=<val>   - Set WiFi SSID");
        usbSendResponse("  AT+PASS=<val>   - Set WiFi password");
        usbSendResponse("  AT+SERVERIP=<val> - Set server IP");
        usbSendResponse("  AT+SERVERPORT=<val> - Set server port (1-65535)");
        usbSendResponse("  AT+SAVE         - Save config and reboot");
        usbSendResponse("  AT+SHOW         - Show current NVS config");
        usbSendResponse("  AT+STAGED       - Show staged values");
        usbSendResponse("  AT+FACTORY      - Factory reset and reboot");
        usbSendResponse("  AT+VERSION      - Show firmware version");
        usbSendResponse("  AT+MAC          - Show MAC address");
        usbSendResponse("  AT+HELP         - Show this help");
        usbSendResponse("OK");
        return;
    }

    usbSendResponse("ERROR:Unknown command. Use AT+HELP");
}

void usbConfigProcess() {
    while (Serial.available()) {
        char c = Serial.read();
        if (c == '\n' || c == '\r') {
            if (usbInputBuffer.length() > 0) {
                lastAtCommandTime = millis();
                usbProcessCommand(usbInputBuffer);
                usbInputBuffer = "";
            }
        } else {
            if (usbInputBuffer.length() < 256) {
                usbInputBuffer += c;
            }
        }
    }
}
