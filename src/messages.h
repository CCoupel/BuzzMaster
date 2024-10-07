#include <ArduinoJson.h>
#include <freertos/FreeRTOS.h>
#include <freertos/queue.h>
#include <esp_log.h>

// Configuration
const char* TAG = "MESSAGES";
const int MAX_MESSAGE_LENGTH = 1024;

// Structure de message améliorée
typedef struct {
    const char* action;
    char message[MAX_MESSAGE_LENGTH];
    bool notifyAll;
    AsyncClient* client;
} messageQueue_t;

void notifyAll() {
    String output;
    JsonDocument& tb=getTeamsAndBumpers();
    if (serializeJson(tb, output)) {
        saveJson();
        ESP_LOGI(TAG, "Sending update to all clients: %s", output.c_str());
        ws.textAll(output.c_str());
        sendMessageToAllClients("UPDATE", output.c_str());
    } else {
        ESP_LOGE(TAG, "Failed to serialize JSON");
    }
}


void putMsgToQueue(const char* action, const char* msg, bool notify, AsyncClient* client) {
    messageQueue_t message;
    message.action = action;
    strncpy(message.message, (msg && *msg) ? msg : "\"\"", MAX_MESSAGE_LENGTH - 1);
    message.message[MAX_MESSAGE_LENGTH - 1] = '\0';  // Ensure null-termination
    message.notifyAll = notify;
    message.client = client;

    if (xQueueSend(messageQueue, &message, pdMS_TO_TICKS(100)) != pdPASS) {
        ESP_LOGE(TAG, "Failed to send message to queue");
    }
}

void sendMessageToClient(const String& action, const String& msg, AsyncClient* client) {
    if (client && client->connected()) {
        String message = "{\"ACTION\":\"" + action + "\",\"MSG\":" + msg + "}\n";
        client->write(message.c_str(), message.length());
        ESP_LOGI(TAG, "Sent to %s: %s", client->remoteIP().toString().c_str(), message.c_str());
    } else {
        ESP_LOGW(TAG, "Client not connected or null");
    }
}

void sendMessageToAllClients(const String& action, const String& msg) {
  ESP_LOGI(TAG, "Sending broadcast message");
  
  // Créer le message JSON
  String message = "{\"ACTION\":\"" + action + "\", \"MSG\":" + msg + "}\n";
  ESP_LOGD(TAG, "Broadcasting message: %s", message.c_str());

  // Envoyer le message en broadcast à tous les clients WebSocket
  ws.textAll(message.c_str());
  
  // Envoyer le message en broadcast à tous les clients TCP
  WiFiUDP udp;
  if (udp.beginPacket(IPAddress(255, 255, 255, 255), CONTROLER_PORT)) {
    udp.write((const uint8_t*)message.c_str(), message.length());
    udp.endPacket();
    ESP_LOGI(TAG, "UDP broadcast message sent successfully");
  } else {
    ESP_LOGE(TAG, "Failed to send UDP broadcast message");
  }

  ESP_LOGI(TAG, "Broadcast message sent: %s", message.c_str());
}

/*
void sendMessageToAllClients(const String& action, const String& msg ) {
  // Parcourez tous les clients connectés
  ESP_LOGI(TAG, "send to all");
  for (AsyncClient* client : bumperClients) {
    sendMessageToClient(action, msg, client);
  }
  String message = "{\"ACTION\":\"" + action + "\", \"MSG\":" + msg + "}\n";

   ws.textAll(message.c_str());
  ESP_LOGI(TAG, " sent to all: %s", message.c_str());
}
*/

void sendMessageTask(void *parameter) {
    messageQueue_t receivedMessage;
    while (1) {
        ESP_LOGD(TAG, "Low stack space in Send Message Task: %i", uxTaskGetStackHighWaterMark(NULL));
        if (xQueueReceive(messageQueue, &receivedMessage, portMAX_DELAY)) {
            ESP_LOGI(TAG, "New message in queue: %s", receivedMessage.action);
            
            switch (hash(receivedMessage.action)) {
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

            if (receivedMessage.client != nullptr) {
                ESP_LOGD(TAG, "client is not null");
                sendMessageToClient(receivedMessage.action, receivedMessage.message, receivedMessage.client);
            } else {
                ESP_LOGD(TAG, "client is null");
                sendMessageToAllClients(receivedMessage.action, receivedMessage.message);
            }

            if (receivedMessage.notifyAll) {
                ESP_LOGD(TAG, "notify all");
                notifyAll();
            }
            ESP_LOGD(TAG, "queue finished");
        }
    }
}
