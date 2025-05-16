#pragma once
#include "Common/CustomLogger.h"
#include "Common/led.h"

#include <WiFi.h>
//#include <esp_log.h>

static const char* WIFI_TAG = "WIFI";


IPAddress apIP(192,168,100,254);
IPAddress gateway(192,168,100,254);
IPAddress subnet(255, 255, 255, 0);

TaskHandle_t wifiConnectTaskHandle = NULL;

void wifiConnectTask(void * parameter) {
        // Load configuration
    String ssid = configManager.getWifiSSID();
    String password = configManager.getWifiPassword();
    for (;;) {
        if (WiFi.status() != WL_CONNECTED) {
            ESP_LOGI(WIFI_TAG, "Attempting to connect to %s", ssid);
            WiFi.begin(ssid, password);
            
            int attempts = 0;
            while (WiFi.status() != WL_CONNECTED && attempts < 20) {
                vTaskDelay(500 / portTICK_PERIOD_MS);
                attempts++;
                //ESP_LOGI(WIFI_TAG, ".");
            }
            
            if (WiFi.status() == WL_CONNECTED) {
                ESP_LOGI(WIFI_TAG, "WiFi connected. IP address: %s", WiFi.localIP().toString().c_str());
                ESP_LOGI(WIFI_TAG, "MAC address: %s", WiFi.macAddress().c_str());
            } else {
                ESP_LOGW(WIFI_TAG, "Failed to connect to WiFi. Will retry...");
            }
        }
        vTaskDelay(1000 / portTICK_PERIOD_MS);  // Wait for 30 seconds before checking again
    }
}

void wifiConnect() {
    ESP_LOGI(WIFI_TAG, "Starting WiFi connection");
    WiFi.mode(WIFI_AP_STA);
    xTaskCreate(
        wifiConnectTask,
        "WiFiConnectTask",
        4096,
        NULL,
        1,
        &wifiConnectTaskHandle
    );
}

void setupAP() {
    String apSSID = configManager.getAPSSID();
    String apPassword = configManager.getAPPassword();

    WiFi.softAPConfig(apIP, gateway, subnet);
    WiFi.softAP(apSSID.c_str(), apPassword.c_str());
    
    // Attendez un peu que l'AP soit complètement configuré
    delay(100);
    
    IPAddress IP = WiFi.softAPIP();
    ESP_LOGI(WIFI_TAG, "WiFi AP: %s started with IP: %s", apSSID.c_str(), IP.toString().c_str());

}


