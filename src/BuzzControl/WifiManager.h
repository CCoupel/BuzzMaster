#pragma once
#include "Common/CustomLogger.h"
#include "Common/led.h"

#include <WiFi.h>
//#include <esp_log.h>
#include <DNSServer.h>
#include <ESPmDNS.h>

static const char* WIFI_TAG = "WIFI";


IPAddress apIP(192,168,100,254);
IPAddress gateway(192,168,100,254);
IPAddress subnet(255, 255, 255, 0);


DNSServer dnsServer;
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


void setupDNSServer() {
    dnsServer.start(53, "*", apIP);
    ESP_LOGI(WIFI_TAG, "Serveur DNS démarré");

    // Configuration du mDNS
    if(MDNS.begin("buzzcontrol")) {
        ESP_LOGI(WIFI_TAG, "MDNS responder started");
        
        // Enregistrer les services
        MDNS.addService("buzzcontrol", "tcp", configManager.getControllerPort());
        MDNS.addService("http", "tcp", 80);  // HTTP is always on port 80
        MDNS.addService("sock", "tcp", configManager.getControllerPort());
        
        // Show IPs for debug
        ESP_LOGI(WIFI_TAG, "AP IP: %s", apIP.toString().c_str());
        ESP_LOGI(WIFI_TAG, "STA IP: %s", WiFi.localIP().toString().c_str());

    }
}
