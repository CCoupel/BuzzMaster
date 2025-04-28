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
    ESP_LOGI(WIFI_TAG, "Connecting to %s", ssid);
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
    WiFi.softAPConfig(apIP, gateway, subnet);
    WiFi.softAP(apSSID, apPASSWORD);
    
    // Attendez un peu que l'AP soit complètement configuré
    delay(100);
    
    IPAddress IP = WiFi.softAPIP();
    ESP_LOGI(WIFI_TAG, "WiFi AP: %s started with IP: %s", apSSID, IP.toString().c_str());
}


void setupDNSServer() {
    dnsServer.start(53, "*", apIP);
    ESP_LOGI(WIFI_TAG, "Serveur DNS démarré");

    // Configuration du mDNS
    if(MDNS.begin("buzzcontrol")) {
        ESP_LOGI(WIFI_TAG, "MDNS responder started");
        
        // Enregistrer les services
        MDNS.addService("buzzcontrol", "tcp", localWWWPort);
        MDNS.addService("http", "tcp", localWWWPort);
        MDNS.addService("sock", "tcp", CONTROLER_PORT);
        
        // Afficher les IPs pour debug
        ESP_LOGI(WIFI_TAG, "AP IP: %s", apIP.toString().c_str());
        ESP_LOGI(WIFI_TAG, "STA IP: %s", WiFi.localIP().toString().c_str());
    }
}
