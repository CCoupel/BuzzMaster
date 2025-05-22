#pragma once
#include "Common/CustomLogger.h"
#include "Common/led.h"
#include <WiFi.h>

// Si vous voulez utiliser certaines fonctions esp_wifi
#include <esp_wifi.h>
#include <esp_private/wifi.h>

static const char* WIFI_TAG = "WIFI";
IPAddress apIP(192, 168, 4, 254);
IPAddress gateway(192, 168, 4, 254);
IPAddress subnet(255, 255, 255, 0);
TaskHandle_t wifiConnectTaskHandle = NULL;

// Fonction pour scanner et trouver le canal le moins encombré
int findBestChannel() {
    ESP_LOGI(WIFI_TAG, "Scanning for networks...");
    int networkCount = WiFi.scanNetworks();
    
    if (networkCount == 0) {
        ESP_LOGI(WIFI_TAG, "No networks found, using default channel 6");
        return 6;
    }
    
    // Analyser l'occupation des canaux
    int channelOccupancy[14] = {0};
    
    for (int i = 0; i < networkCount; i++) {
        int channel = WiFi.channel(i);
        int rssi = WiFi.RSSI(i);
        
        // Plus le signal est fort, plus l'impact est important
        int weight = map(abs(rssi), 100, 40, 1, 10);
        if (weight < 1) weight = 1;
        if (weight > 10) weight = 10;
        
        channelOccupancy[channel] += weight;
        
        ESP_LOGD(WIFI_TAG, "Found network: %s, channel: %d, RSSI: %d",
                 WiFi.SSID(i).c_str(), channel, rssi);
    }
    
    // Trouver le canal le moins encombré, privilégiant 1, 6, 11
    int standardChannels[3] = {1, 6, 11};
    int minOccupancy = 999;
    int bestChannel = 6;  // Canal par défaut
    
    // Vérifier d'abord les canaux standards
    for (int i = 0; i < 3; i++) {
        int ch = standardChannels[i];
        if (channelOccupancy[ch] < minOccupancy) {
            minOccupancy = channelOccupancy[ch];
            bestChannel = ch;
        }
    }
    
    // Si les canaux standards sont très occupés, chercher parmi tous
    if (minOccupancy > 20) {
        minOccupancy = 999;
        for (int ch = 1; ch <= 11; ch++) {
            if (channelOccupancy[ch] < minOccupancy) {
                minOccupancy = channelOccupancy[ch];
                bestChannel = ch;
            }
        }
    }
    
    ESP_LOGI(WIFI_TAG, "Best channel found: %d (occupancy: %d)",
             bestChannel, channelOccupancy[bestChannel]);
    
    // Libérer la mémoire du scan
    WiFi.scanDelete();
    
    return bestChannel;
}

void wifiConnectTask(void * parameter) {
    // Load configuration
    const int maxTries = 10;
    int tries = 0;
    String ssid = configManager.getWifiSSID();
    String password = configManager.getWifiPassword();
    
    while (tries++ < maxTries) {
        esp_task_wdt_reset();
        yield();
        ESP_LOGD(WIFI_TAG, "Try to connect to Wifi %s (%i/%i)", ssid.c_str(), tries, maxTries);
        
        if (WiFi.status() != WL_CONNECTED) {
            ESP_LOGI(WIFI_TAG, "Attempting to connect to %s", ssid.c_str());
            WiFi.begin(ssid, password);
            
            int attempts = 0;
            while (WiFi.status() != WL_CONNECTED && attempts < maxTries) {
                esp_task_wdt_reset();
                yield();
                vTaskDelay(500 / portTICK_PERIOD_MS);
                attempts++;
            }
            
            if (WiFi.status() == WL_CONNECTED) {
                ESP_LOGI(WIFI_TAG, "WiFi connected. IP address: %s", WiFi.localIP().toString().c_str());
                ESP_LOGI(WIFI_TAG, "MAC address: %s", WiFi.macAddress().c_str());
                tries = 0;
                vTaskDelete(NULL);
                return;
            } else {
                ESP_LOGW(WIFI_TAG, "Failed to connect to WiFi. Will retry...");
            }
        }
        
        WiFi.disconnect(true);
        vTaskDelay(1000 / portTICK_PERIOD_MS);
    }
    
    ESP_LOGW(WIFI_TAG, "No more try to connect to Wifi %s", ssid.c_str());
    vTaskDelete(NULL);
}

void wifiConnect() {
    ESP_LOGI(WIFI_TAG, "Starting/Restarting WiFi connection");
    
    // Vérifier si une tâche WiFi existe déjà et la supprimer si c'est le cas
    if (wifiConnectTaskHandle != NULL) {
        // Vérifier d'abord si la tâche existe encore dans le système
        eTaskState taskState = eTaskGetState(wifiConnectTaskHandle);
        if (taskState != eDeleted) {
            ESP_LOGI(WIFI_TAG, "Stopping existing WiFi connection task");
            vTaskDelete(wifiConnectTaskHandle);
        } else {
            ESP_LOGI(WIFI_TAG, "WiFi task handle exists but task already deleted");
        }
        wifiConnectTaskHandle = NULL;
        
        // Petit délai pour s'assurer que la tâche est correctement terminée
        vTaskDelay(100 / portTICK_PERIOD_MS);
    }
    
    // Déconnecter du réseau WiFi si déjà connecté
    if (WiFi.status() == WL_CONNECTED) {
        ESP_LOGI(WIFI_TAG, "Disconnecting from current WiFi network");
        WiFi.disconnect(true);
        
        // Court délai pour s'assurer que la déconnexion est traitée
        vTaskDelay(500 / portTICK_PERIOD_MS);
    }
    
    // S'assurer que nous sommes en mode AP+STA
    WiFi.mode(WIFI_AP_STA);
    
    // Créer une nouvelle tâche pour gérer la connexion
    ESP_LOGI(WIFI_TAG, "Creating new WiFi connection task");
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
    
    // Trouver le meilleur canal
    int bestChannel = findBestChannel();
    
    // Configurer l'AP
    WiFi.softAPConfig(apIP, gateway, subnet);
    
    // Démarrer l'AP avec le canal optimal
    // Le dernier paramètre est le nombre maximal de connexions (8 est souvent la limite dans Arduino ESP32)
    if (WiFi.softAP(apSSID.c_str(), apPassword.c_str(), bestChannel, 0, 16)) {
        ESP_LOGI(WIFI_TAG, "AP started with max 8 connections");
    } else {
        ESP_LOGW(WIFI_TAG, "AP started with default max connections");
        WiFi.softAP(apSSID.c_str(), apPassword.c_str(), bestChannel);
    }
    
    // Attendez un peu que l'AP soit complètement configuré
    delay(100);
    
    IPAddress IP = WiFi.softAPIP();
    ESP_LOGI(WIFI_TAG, "WiFi AP: %s started with IP: %s on channel %d", 
             apSSID.c_str(), IP.toString().c_str(), bestChannel);
    
    // Optimisations supplémentaires via méthodes spécifiques à Arduino
    WiFi.setSleep(WIFI_PS_NONE);  // Désactiver le mode d'économie d'énergie
    
    // Optionnellement, tenter d'utiliser des méthodes de bas niveau si disponibles
    #ifdef ESP_IDF_VERSION_MAJOR
    // Ces fonctions ne sont pas toujours disponibles ou peuvent changer
    // entre les versions d'ESP-IDF, donc nous les enveloppons dans un try/catch
    try {
        #if defined(WIFI_PS_NONE)
        esp_wifi_set_ps(WIFI_PS_NONE);
        #endif
        #if defined(esp_wifi_set_max_tx_power)
        esp_wifi_set_max_tx_power(84);  // 84 = 20dBm
        #endif
    } catch (...) {
        ESP_LOGW(WIFI_TAG, "Some low-level WiFi optimizations not available");
    }
    #endif
}

// Fonction pour redémarrer périodiquement l'AP (peut améliorer la stabilité)
void restartAP() {
    static unsigned long lastRestartTime = 0;
    const unsigned long restartInterval = 3600000; // 1 heure en ms
    
    unsigned long currentTime = millis();
    if (currentTime - lastRestartTime >= restartInterval) {
        lastRestartTime = currentTime;
        
        ESP_LOGI(WIFI_TAG, "Performing periodic AP restart for stability");
        
        // Stocker les paramètres actuels
        String apSSID = configManager.getAPSSID();
        String apPassword = configManager.getAPPassword();
        int channel = WiFi.channel();
        
        // Redémarrer l'AP
        WiFi.softAPdisconnect(false);
        delay(500);
        
        WiFi.softAPConfig(apIP, gateway, subnet);
        WiFi.softAP(apSSID.c_str(), apPassword.c_str(), channel, 0, 8);
        
        ESP_LOGI(WIFI_TAG, "AP restarted");
    }
}

// Tâche pour surveiller et maintenir la stabilité du WiFi
void wifiMonitorTask(void * parameter) {
    const portTickType xDelay = 10000 / portTICK_PERIOD_MS; // 10 secondes
    
    while (true) {
        // Vérifier l'état de l'AP
        int clientCount = WiFi.softAPgetStationNum();
        ESP_LOGD(WIFI_TAG, "AP client count: %d", clientCount);
        
        // Appliquer les optimisations périodiquement
        WiFi.setSleep(WIFI_PS_NONE);  // Réappliquer le mode sans économie d'énergie
        
        // Vérifier s'il faut redémarrer l'AP
        restartAP();
        
        vTaskDelay(xDelay);
    }
}

// Fonction principale à appeler dans setup()
void setupWiFi() {
    // Initialiser le WiFi en mode AP+STA
    WiFi.mode(WIFI_AP_STA);
    
    // Configurer le point d'accès
    setupAP();
    
    // Créer une tâche pour surveiller le WiFi
    TaskHandle_t wifiMonitorTaskHandle = NULL;
    xTaskCreate(
        wifiMonitorTask,
        "WiFiMonitor",
        4096,
        NULL,
        1,
        &wifiMonitorTaskHandle
    );
    
    // Optionnel: Se connecter à un réseau existant
    // wifiConnect();
}