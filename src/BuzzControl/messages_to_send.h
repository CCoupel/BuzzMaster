#pragma once
#include "Common/CustomLogger.h"
#include "Common/led.h"

#include <ArduinoJson.h>
#include <freertos/FreeRTOS.h>
#include <freertos/queue.h>

// Configuration
const char* SEND_TAG = "MSG_SEND";
int sentMsgId=0;

// Structure de message pour les envois
typedef struct {
    String action;
    String* message;  
    bool notifyAll;
    AsyncClient* client;
    int msgID;
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
    String output= getTeamsAndBumpersJSON();;
    
        sendMessageToAllClients("UPDATE", output.c_str());
}

void enqueueOutgoingMessage(const char* action, const char* msg, bool notify, AsyncClient* client) {
    OutgoingMessage_t* message = new OutgoingMessage_t;
    message->action = action;
    message->message = new String(msg);
    message->msgTime = new String(micros());
    message->msgID = sentMsgId++;
    message->notifyAll = notify;
    message->client = client;

    if (xQueueSend(outgoingQueue, &message, pdMS_TO_TICKS(100)) != pdPASS) {
        ESP_LOGE(SEND_TAG, "Failed to send message to outgoing queue");
        delete message->message;
        delete message->msgTime;
        delete message;
    } else {
        UBaseType_t messagesWaiting = uxQueueMessagesWaiting(outgoingQueue);

        ESP_LOGD(SEND_TAG, "Message ID %i enqueued as %u %s : %s", message->msgID, messagesWaiting, message->action.c_str(), msg);
    }
}

String makeJsonMessage(const String& action, const String& msg) {
    String message = "{";
    message += "\"ACTION\": \"" + action + "\"";
    message += ", \"VERSION\": \"" + String(VERSION) + "\"";
    message += ", \"MSG\":" + msg + "";
    message += ", \"TIME_EVENT\":" + String(micros()) + "";
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

bool sendBroadcastUDP(const String& action, const String& msg) {
  ESP_LOGI(SEND_TAG, "Sending broadcast message: %s", action.c_str());
  String message = makeJsonMessage(action, msg);
  ESP_LOGD(SEND_TAG, "Broadcasting message: %s", message.c_str());

  bool success = false;
  WiFiUDP udp;
  const int maxRetries = 3;  // Nombre de tentatives maximum
  
  // Adresses de broadcast pour les réseaux STA et AP
  IPAddress staBroadcast;
  IPAddress apBroadcast;
  
  if (WiFi.status() == WL_CONNECTED) {
    staBroadcast = calculateBroadcast(WiFi.localIP(), WiFi.subnetMask());
  }
  
  if (WiFi.softAPgetStationNum() > 0) {
    apBroadcast = calculateBroadcast(WiFi.softAPIP(), IPAddress(255, 255, 255, 0));
  }
  
  // Tentatives d'envoi sur le réseau STA
  if (WiFi.status() == WL_CONNECTED) {
    for (int attempt = 0; attempt < maxRetries; attempt++) {
      if (udp.beginPacket(staBroadcast, configManager.getControllerPort())) {
        size_t bytesSent = udp.write((const uint8_t*)message.c_str(), message.length());
        if (bytesSent == message.length() && udp.endPacket()) {
          ESP_LOGI(SEND_TAG, "UDP broadcast sent successfully on STA network to %s (%d bytes)", 
                  staBroadcast.toString().c_str(), bytesSent);
          success = true;
          break;
        } else {
          ESP_LOGW(SEND_TAG, "UDP broadcast failed on STA network (attempt %d/%d): %d/%d bytes sent", 
                   attempt+1, maxRetries, bytesSent, message.length());
          delay(50); // Petit délai avant la prochaine tentative
        }
      } else {
        ESP_LOGE(SEND_TAG, "Failed to begin UDP packet on STA network (attempt %d/%d)", 
                 attempt+1, maxRetries);
      }
    }
  }
  
  // Tentatives d'envoi sur le réseau AP
  if (WiFi.softAPgetStationNum() > 0) {
    for (int attempt = 0; attempt < maxRetries; attempt++) {
      if (udp.beginPacket(apBroadcast, configManager.getControllerPort())) {
        size_t bytesSent = udp.write((const uint8_t*)message.c_str(), message.length());
        if (bytesSent == message.length() && udp.endPacket()) {
          ESP_LOGI(SEND_TAG, "UDP broadcast sent successfully on AP network to %s (%d bytes)", 
                  apBroadcast.toString().c_str(), bytesSent);
          success = true;
          break;
        } else {
          ESP_LOGW(SEND_TAG, "UDP broadcast failed on AP network (attempt %d/%d): %d/%d bytes sent", 
                   attempt+1, maxRetries, bytesSent, message.length());
          delay(50); // Petit délai avant la prochaine tentative
        }
      } else {
        ESP_LOGE(SEND_TAG, "Failed to begin UDP packet on AP network (attempt %d/%d)", 
                 attempt+1, maxRetries);
      }
    }
  }
  
  return success;
}

void sendMessageToAllClients(const String& action, const String& msg) {

    ESP_LOGI(SEND_TAG, "Sending broadcast message");
    String message = makeJsonMessage(action, msg);
    ESP_LOGD(SEND_TAG, "Broadcasting to all message: %s", message.c_str());

    // Envoyer le message en broadcast à tous les clients WebSocket
    ws.textAll(message.c_str());
    sendBroadcastUDP(action, msg);
}

void sendMessageTask(void *parameter) {
    OutgoingMessage_t* receivedMessage;
    while (1) {
        ESP_LOGD(SEND_TAG, "Low stack space in Send Message Task: %i", uxTaskGetStackHighWaterMark(NULL));
        UBaseType_t messagesWaitingBefore = uxQueueMessagesWaiting(outgoingQueue);
        ESP_LOGD(SEND_TAG, "Waiting for incoming messages (%u in queue)", messagesWaitingBefore);
        
        if (xQueueReceive(outgoingQueue, &receivedMessage, portMAX_DELAY)) {
            ESP_LOGI(SEND_TAG, "dequeue message %i : %s", receivedMessage->msgID, receivedMessage->action.c_str());
            
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