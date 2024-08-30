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
    if (serializeJson(teamsAndBumpers, output)) {
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
    strncpy(message.message, (msg && *msg) ? msg : "''", MAX_MESSAGE_LENGTH - 1);
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

void sendMessageToAllClients(const String& action, const String& msg ) {
  // Parcourez tous les clients connectés
  ESP_LOGI(TAG, "send to all");
  for (AsyncClient* client : bumperClients) {
    sendMessageToClient(action, msg, client);
  }
  ESP_LOGI(TAG, " all is sent");
}

void sendMessageTask(void *parameter) {
    messageQueue_t receivedMessage;
    while (1) {
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
                sendMessageToClient(receivedMessage.action, receivedMessage.message, receivedMessage.client);
            } else {
                sendMessageToAllClients(receivedMessage.action, receivedMessage.message);
            }

            if (receivedMessage.notifyAll) {
                notifyAll();
            }
        }
    }
}
