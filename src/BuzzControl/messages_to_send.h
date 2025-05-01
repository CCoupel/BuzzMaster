#pragma once
#include "Common/CustomLogger.h"
#include "Common/led.h"

#include <ArduinoJson.h>
#include <freertos/FreeRTOS.h>
#include <freertos/queue.h>

// Configuration
const char* SEND_TAG = "MSG_SEND";

// Structure de message pour les envois
typedef struct {
    String action;
    String* message;  
    bool notifyAll;
    AsyncClient* client;
    String* msgTime;
} OutgoingMessage_t;

// Queue pour les messages sortants
QueueHandle_t outgoingQueue;

// Initialisation de la queue de messages sortants
void initOutgoingQueue() {
    outgoingQueue = xQueueCreate(20, sizeof(OutgoingMessage_t*));
    if (outgoingQueue == NULL) {
        ESP_LOGE(SEND_TAG, "Failed to create outgoing message queue");
    }
}

void notifyAll() {
    String output;
    JsonDocument& tb = getTeamsAndBumpers();

    if (serializeJson(tb, output)) {
        saveJson();
        ESP_LOGI(SEND_TAG, "Sending update to all clients: %s", output.c_str());
        sendMessageToAllClients("UPDATE", output.c_str());
    } else {
        ESP_LOGE(SEND_TAG, "Failed to serialize JSON");
    }
}

void enqueueOutgoingMessage(const char* action, const char* msg, bool notify, AsyncClient* client) {
    OutgoingMessage_t* message = new OutgoingMessage_t;
    message->action = action;
    message->message = new String(msg);
    message->msgTime = new String(micros());
    message->notifyAll = notify;
    message->client = client;

    if (xQueueSend(outgoingQueue, &message, pdMS_TO_TICKS(100)) != pdPASS) {
        ESP_LOGE(SEND_TAG, "Failed to send message to outgoing queue");
        delete message->message;
        delete message->msgTime;
        delete message;
    }
}

String makeJsonMessage(const String& action, const String& msg) {
    String message = "{";
    message += "\"ACTION\": \"" + action + "\"";
    message += ", \"VERSION\": \"" + String(VERSION) + "\"";
    message += ", \"MSG\":" + msg + "";
    message += ", \"TIME_EVENT\":" + String(micros()) + "";
    message += ", \"FSINFO\": " + printLittleFSInfo(true) + "";
    message += "} \n\0";

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
        String message = makeJsonMessage(action, msg);
        client->write(message.c_str(), message.length());
        ESP_LOGI(SEND_TAG, "Sent to %s: %s", client->remoteIP().toString().c_str(), message.c_str());
    } else {
        ESP_LOGW(SEND_TAG, "Client not connected or null");
    }
}

void sendMessageToAllClients(const String& action, const String& msg) {
    ESP_LOGI(SEND_TAG, "Sending broadcast message");
    String message = makeJsonMessage(action, msg);
    ESP_LOGD(SEND_TAG, "Broadcasting message: %s", message.c_str());

    // Envoyer le message en broadcast à tous les clients WebSocket
    ws.textAll(message.c_str());
    
    WiFiUDP udp;
    
    // Broadcast sur le réseau STA si connecté
    if (WiFi.status() == WL_CONNECTED) {
        // Calculer l'adresse de broadcast du réseau STA
        IPAddress staBroadcast = calculateBroadcast(WiFi.localIP(), WiFi.subnetMask());
        
        if (udp.beginPacket(staBroadcast, CONTROLER_PORT)) {
            udp.write((const uint8_t*)message.c_str(), message.length());
            udp.endPacket();
            ESP_LOGI(SEND_TAG, "UDP broadcast sent on STA network to %s", staBroadcast.toString().c_str());
        } else {
            ESP_LOGE(SEND_TAG, "Failed to send UDP broadcast on STA network");
        }
    }

    // Broadcast sur le réseau AP si actif
    if (WiFi.softAPgetStationNum() > 0) {
        // L'adresse de broadcast pour AP est généralement 192.168.4.255
        // (si votre AP est configuré sur 192.168.4.1)
        IPAddress apBroadcast = calculateBroadcast(WiFi.softAPIP(), IPAddress(255, 255, 255, 0));
        
        if (udp.beginPacket(apBroadcast, CONTROLER_PORT)) {
            udp.write((const uint8_t*)message.c_str(), message.length());
            udp.endPacket();
            ESP_LOGI(SEND_TAG, "UDP broadcast sent on AP network to %s", apBroadcast.toString().c_str());
        } else {
            ESP_LOGE(SEND_TAG, "Failed to send UDP broadcast on AP network");
        }
    }
}

void sendMessageTask(void *parameter) {
    OutgoingMessage_t* receivedMessage;
    while (1) {
        ESP_LOGD(SEND_TAG, "Low stack space in Send Message Task: %i", uxTaskGetStackHighWaterMark(NULL));
        if (xQueueReceive(outgoingQueue, &receivedMessage, portMAX_DELAY)) {
            ESP_LOGI(SEND_TAG, "New message in queue: %s (%s)", receivedMessage->action.c_str(), receivedMessage->msgTime->c_str());
            
            switch (hash(receivedMessage->action.c_str())) {
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
                ESP_LOGD(SEND_TAG, "client is not null");
                sendMessageToClient(receivedMessage->action, *(receivedMessage->message), receivedMessage->client);
            } else {
                ESP_LOGD(SEND_TAG, "client is null");
                sendMessageToAllClients(receivedMessage->action, *(receivedMessage->message));
            }

            if (receivedMessage->notifyAll) {
                ESP_LOGD(SEND_TAG, "notify all");
                notifyAll();
            }
            
            // Nettoyage
            delete receivedMessage->message;
            delete receivedMessage->msgTime;
            delete receivedMessage;
            ESP_LOGD(SEND_TAG, "queue finished");
        }
    }
}