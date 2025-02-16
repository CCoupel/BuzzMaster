#pragma once
#include "Common/CustomLogger.h"
#include "Common/led.h"

#include <ArduinoJson.h>
#include <freertos/FreeRTOS.h>
#include <freertos/queue.h>
//#include <esp_log.h>

// Configuration
const char* TAG = "MESSAGES";
//const int MAX_MESSAGE_LENGTH = 4096;

// Structure de message améliorée
typedef struct {
    const char* action;
    String* message;  
    bool notifyAll;
    AsyncClient* client;
} messageQueue_t;

void notifyAll() {
    String output;
    JsonDocument& tb=getTeamsAndBumpers();

    if (serializeJson(tb, output)) {
        saveJson();
        ESP_LOGI(TAG, "Sending update to all clients: %s", output.c_str());
        sendMessageToAllClients("UPDATE", output.c_str());
    } else {
        ESP_LOGE(TAG, "Failed to serialize JSON");
    }
}


void putMsgToQueue(const char* action, const char* msg, bool notify, AsyncClient* client) {
    messageQueue_t* message=new messageQueue_t;
    message->action = action;
    message->message = new String(msg);  // Création dynamique de la String
    message->notifyAll = notify;
    message->client = client;


    if (xQueueSend(messageQueue, &message, pdMS_TO_TICKS(100)) != pdPASS) {
        ESP_LOGE(TAG, "Failed to send message to queue");
        delete message->message;  // Nettoyage en cas d'échec
        delete message;

    }
}

String makeJsonMessage(const String& action, const String& msg) {
    String message="{";
    message += "\"ACTION\": \"" + action + "\"";
    message += ", \"VERSION\": \"" + String(VERSION) + "\"";
    message += ", \"MSG\":" + msg + "";
    message += ", \"TIME_EVENT\":" + String(micros()) + "";
    message += ", \"FSINFO\": "+printLittleFSInfo(true)+"";
    message += "} \n";

    return message;
}

// Fonction générique pour calculer l'adresse de broadcast
IPAddress calculateBroadcast(const IPAddress& ip, const IPAddress& subnet) {
    IPAddress broadcast;
    for (int i = 0; i < 4; i++) {
        broadcast[i] = ip[i] | ~subnet[i];
    }
    return broadcast;
}

void sendMessageToClient(const String& action, const String& msg, AsyncClient* client) {
    if (client && client->connected()) {
        String message=makeJsonMessage(action, msg);
        client->write(message.c_str(), message.length());
        ESP_LOGI(TAG, "Sent to %s: %s", client->remoteIP().toString().c_str(), message.c_str());
    } else {
        ESP_LOGW(TAG, "Client not connected or null");
    }
}

void sendMessageToAllClients(const String& action, const String& msg) {
    ESP_LOGI(TAG, "Sending broadcast message");
    String message = makeJsonMessage(action, msg);
    ESP_LOGD(TAG, "Broadcasting message: %s", message.c_str());

    // Envoyer le message en broadcast à tous les clients WebSocket
    ws.textAll(message.c_str());
    
    WiFiUDP udp;
    
    // Broadcast sur le réseau STA si connecté
    if(WiFi.status() == WL_CONNECTED) {
        // Calculer l'adresse de broadcast du réseau STA
        IPAddress staBroadcast = calculateBroadcast(WiFi.localIP(), WiFi.subnetMask());
        
        if (udp.beginPacket(staBroadcast, CONTROLER_PORT)) {
            udp.write((const uint8_t*)message.c_str(), message.length());
            udp.endPacket();
            ESP_LOGI(TAG, "UDP broadcast sent on STA network to %s", staBroadcast.toString().c_str());
        } else {
            ESP_LOGE(TAG, "Failed to send UDP broadcast on STA network");
        }
    }

    // Broadcast sur le réseau AP si actif
    if(WiFi.softAPgetStationNum() > 0) {
        // L'adresse de broadcast pour AP est généralement 192.168.4.255
        // (si votre AP est configuré sur 192.168.4.1)
        IPAddress apBroadcast = calculateBroadcast(WiFi.softAPIP(), IPAddress(255, 255, 255, 0));
        
        if (udp.beginPacket(apBroadcast, CONTROLER_PORT)) {
            udp.write((const uint8_t*)message.c_str(), message.length());
            udp.endPacket();
            ESP_LOGI(TAG, "UDP broadcast sent on AP network to %s", apBroadcast.toString().c_str());
        } else {
            ESP_LOGE(TAG, "Failed to send UDP broadcast on AP network");
        }
    }

    ESP_LOGI(TAG, "Broadcast messages sent: %s", message.c_str());
}

void sendMessageTask(void *parameter) {
    messageQueue_t* receivedMessage;
    while (1) {
        ESP_LOGD(TAG, "Low stack space in Send Message Task: %i", uxTaskGetStackHighWaterMark(NULL));
        if (xQueueReceive(messageQueue, &receivedMessage, portMAX_DELAY)) {
            ESP_LOGI(TAG, "New message in queue: %s", receivedMessage->action);
            
            switch (hash(receivedMessage->action)) {
                case hash("HELLO"):
                    sendMessageToAllClients("HELLO", "{  }");
                    break;
                case hash("START"):
                    setLedColor(255, 0, 0);
                    setLedIntensity(255);
                    break;
                case hash("STOP"):
                    setLedColor(0, 255, 0);
                    setLedIntensity(255);
                    break;
                case hash("PAUSE"):
                    setLedColor(255, 255, 0);
                    setLedIntensity(64);
                    break;
            }

            if (receivedMessage->client != nullptr) {
                ESP_LOGD(TAG, "client is not null");
                sendMessageToClient(receivedMessage->action, *(receivedMessage->message), receivedMessage->client);
            } else {
                ESP_LOGD(TAG, "client is null");
                sendMessageToAllClients(receivedMessage->action, *(receivedMessage->message));
            }

//            if (receivedMessage.notifyAll) {
                ESP_LOGD(TAG, "notify all");
                notifyAll();
//            }
// Nettoyage
            delete receivedMessage->message;
            delete receivedMessage;
            ESP_LOGD(TAG, "queue finished");
        }
    }
}
