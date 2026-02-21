#pragma once
#include <Preferences.h>
#include "esp_log.h"

static const char* NVS_TAG = "NVS_CONFIG";
static const char* NVS_NAMESPACE = "buzzclick";

// NVS keys
static const char* NVS_KEY_SSID = "wifi_ssid";
static const char* NVS_KEY_PASS = "wifi_password";
static const char* NVS_KEY_SERVER_IP = "server_ip";
static const char* NVS_KEY_SERVER_PORT = "server_port";
static const char* NVS_KEY_SSID2 = "wifi_ssid2";
static const char* NVS_KEY_PASS2 = "wifi_pass2";

struct BuzzClickConfig {
    String wifi_ssid;
    String wifi_password;
    String wifi_ssid2;    // fallback WiFi SSID (optional)
    String wifi_pass2;    // fallback WiFi password (optional)
    String server_ip;
    uint16_t server_tcp_port;
    bool valid;  // true if at least SSID is configured
};

static Preferences nvsPrefs;
static BuzzClickConfig currentConfig;

bool nvsLoadConfig() {
    if (!nvsPrefs.begin(NVS_NAMESPACE, true)) {  // read-only
        ESP_LOGE(NVS_TAG, "Failed to open NVS namespace");
        return false;
    }

    currentConfig.wifi_ssid = nvsPrefs.getString(NVS_KEY_SSID, "");
    currentConfig.wifi_password = nvsPrefs.getString(NVS_KEY_PASS, "");
    currentConfig.wifi_ssid2 = nvsPrefs.getString(NVS_KEY_SSID2, "");
    currentConfig.wifi_pass2 = nvsPrefs.getString(NVS_KEY_PASS2, "");
    currentConfig.server_ip = nvsPrefs.getString(NVS_KEY_SERVER_IP, "");
    currentConfig.server_tcp_port = nvsPrefs.getUShort(NVS_KEY_SERVER_PORT, 1234);
    currentConfig.valid = (currentConfig.wifi_ssid.length() > 0);

    nvsPrefs.end();

    ESP_LOGI(NVS_TAG, "Config loaded: SSID=%s, SSID2=%s, ServerIP=%s, Port=%d, Valid=%s",
             currentConfig.wifi_ssid.c_str(),
             currentConfig.wifi_ssid2.c_str(),
             currentConfig.server_ip.c_str(),
             currentConfig.server_tcp_port,
             currentConfig.valid ? "YES" : "NO");

    return currentConfig.valid;
}

bool nvsSaveConfig() {
    if (!nvsPrefs.begin(NVS_NAMESPACE, false)) {  // read-write
        ESP_LOGE(NVS_TAG, "Failed to open NVS namespace for writing");
        return false;
    }

    nvsPrefs.putString(NVS_KEY_SSID, currentConfig.wifi_ssid);
    nvsPrefs.putString(NVS_KEY_PASS, currentConfig.wifi_password);
    nvsPrefs.putString(NVS_KEY_SSID2, currentConfig.wifi_ssid2);
    nvsPrefs.putString(NVS_KEY_PASS2, currentConfig.wifi_pass2);
    nvsPrefs.putString(NVS_KEY_SERVER_IP, currentConfig.server_ip);
    nvsPrefs.putUShort(NVS_KEY_SERVER_PORT, currentConfig.server_tcp_port);
    currentConfig.valid = (currentConfig.wifi_ssid.length() > 0);

    nvsPrefs.end();

    ESP_LOGI(NVS_TAG, "Config saved: SSID=%s, SSID2=%s, ServerIP=%s, Port=%d",
             currentConfig.wifi_ssid.c_str(),
             currentConfig.wifi_ssid2.c_str(),
             currentConfig.server_ip.c_str(),
             currentConfig.server_tcp_port);

    return true;
}

void nvsClearConfig() {
    if (!nvsPrefs.begin(NVS_NAMESPACE, false)) {
        ESP_LOGE(NVS_TAG, "Failed to open NVS namespace for clearing");
        return;
    }

    nvsPrefs.clear();
    nvsPrefs.end();

    currentConfig.wifi_ssid = "";
    currentConfig.wifi_password = "";
    currentConfig.wifi_ssid2 = "";
    currentConfig.wifi_pass2 = "";
    currentConfig.server_ip = "";
    currentConfig.server_tcp_port = 1234;
    currentConfig.valid = false;

    ESP_LOGI(NVS_TAG, "Config cleared (factory reset)");
}

BuzzClickConfig& nvsGetConfig() {
    return currentConfig;
}
