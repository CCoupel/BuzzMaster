#pragma once
#include "Common/CustomLogger.h"
#include "Common/led.h"
#include "messages_to_send.h"

#include <ArduinoJson.h>
#include <freertos/FreeRTOS.h>
#include <freertos/queue.h>

// Configuration
const char* RECEIVE_TAG = "MSG_RECEIVE";

int receivedMsgId=0;
// Structure de message pour les messages entrants
typedef struct {
    String source;     // "TCP", "WebSocket", "Button", etc.
    String* data;      // Contenu du message
    AsyncClient* client; // Client source (si applicable)
    int64_t timestamp;  // Horodatage pour le traçage
    int msgID;
} IncomingMessage_t;

// Queue pour les messages entrants
QueueHandle_t incomingQueue;

// Initialisation de la queue de messages entrants
void initIncomingQueue() {
    incomingQueue = xQueueCreate(30, sizeof(IncomingMessage_t*));
    if (incomingQueue == NULL) {
        ESP_LOGE(RECEIVE_TAG, "Failed to create incoming message queue");
    }
}

void enqueueIncomingMessage(const char* source, const char* data, AsyncClient* client) {
    IncomingMessage_t* message = new IncomingMessage_t;
    message->source = source;
    message->data = new String(data);
    message->timestamp = micros();
    message->msgID = receivedMsgId++;
    message->client = client;

    if (xQueueSend(incomingQueue, &message, pdMS_TO_TICKS(100)) != pdPASS) {
        ESP_LOGE(RECEIVE_TAG, "Failed to send message to incoming queue");
        delete message->data;
    //    delete message->timestamp;
        delete message;
    } else {
        ESP_LOGD(RECEIVE_TAG, "Message ID %i enqueued from source: %s: %s at %i", message->msgID, source, message->data->c_str(), message->timestamp);
    }
}

void processDataFromSocket(const char* action, const JsonObject& message, int64_t timestamp) {
  ESP_LOGI(RECEIVE_TAG, "Processing action: %s", action);
  String output = "";
  serializeJson(message, output);

  if (output.length() > 0) {
    ESP_LOGI(RECEIVE_TAG, "Processing message: %i: %s", output.length(), output.c_str());
  } else {
    output = "!! NULL !!";
    ESP_LOGI(RECEIVE_TAG, "Processing null message: %s", output.c_str());
  }

  switch (hash(action)) {
    case hash("DELETE"):
      // Handle delete
      deleteQuestion(message["ID"]);
      break;
      
    case hash("HELLO"):
      notifyAll();
      enqueueOutgoingMessage("QUESTIONS", getQuestions().c_str(), false, nullptr);
      break;
      
    case hash("FULL"):
      setBumpers(message["bumpers"]);
      setTeams(message["teams"]);
      notifyAll();
      break;
      
    case hash("UPDATE"):
      updateTeams(message["teams"]);
      updateBumpers(message["bumpers"]);
      break;

    case hash("RESET"):
      resetServer();
      break;
      
    case hash("REBOOT"):
      rebootServer();
      break;
      
    case hash("REVEAL"):
      revealGame();
      break;
      
    case hash("READY"):
      readyGame(message["QUESTION"]);
      break;
      
    case hash("START"):
      startGame(message["DELAY"]);
      break;
      
    case hash("STOP"):
      stopGame();
      break;
      
    case hash("PAUSE"):
      pauseAllGame();
      break;
      
    case hash("CONTINUE"):
      continueGame();
      break;
      
    case hash("RAZ"):
      RAZscores();
      break;
      
    case hash("REMOTE"):
      setRemotePage(message["REMOTE"]);
      break;
      
    case hash("FSINFO"):
      enqueueOutgoingMessage("FSINFO", ("{\"FSINFO\": \"" + printLittleFSInfo() + "\"}").c_str(), false, nullptr);
      break;
      
    default:
      ESP_LOGW(RECEIVE_TAG, "Unrecognized action: %s", action);
      break;
  }
  saveJson();
}

// Déclaration externe de la fonction définie dans BumperServer.h
extern void processButtonPress(const String& bumperID, const char* b_team, int64_t b_time, String b_button);

void processTCPMessage(const String& data, AsyncClient* client, int64_t timestamp) {
    JsonDocument receivedData;
    DeserializationError error = deserializeJson(receivedData, data);
    if (error) {
        ESP_LOGE(RECEIVE_TAG, "Failed to parse JSON from TCP: %s", error.c_str());
        return;
    }

    String bumperID = receivedData["ID"].as<String>();
    String versionBuzzer = receivedData["VERSION"].as<String>();
    String action = receivedData["ACTION"].as<String>();
    JsonObject MSG = receivedData["MSG"];

    ESP_LOGD(RECEIVE_TAG, "TCP message: bumperID=%s version=%s ACTION=%s", 
             bumperID.c_str(), versionBuzzer.c_str(), action.c_str());
    
    // Utiliser if-else au lieu de switch pour éviter les problèmes de sauts
    if (action == "HELLO") {
        // Handle hello action
        updateBumper(bumperID.c_str(), MSG);
        notifyAll();
    }
    else if (action == "BUTTON") {
        // Handle button action
        ESP_LOGE(RECEIVE_TAG, "Button pressed: %s", bumperID.c_str());
        JsonObject bumper = getBumper(bumperID.c_str());
        const char* teamID = bumper["TEAM"];
        String b_button = MSG["button"];
        if (teamID != nullptr) {
            processButtonPress(bumperID, teamID, timestamp, b_button);
//            pauseGame(client);

        }
    }
    else if (action == "PONG") {
        // Handle ping response
        ESP_LOGI(RECEIVE_TAG, "Bumper PONG received from: %s", bumperID.c_str());
        if (isGamePrepare()) {
            setBumperReady(bumperID.c_str());
            updateTeamsReady();
            notifyAll();
            
            // Check if all teams are ready to potentially transition to READY state
            if (areAllTeamsReady()) {
                ESP_LOGI(RECEIVE_TAG, "All teams are ready to start");
                setGamePhase("READY");
                enqueueOutgoingMessage("READY", getTeamsAndBumpersJSON().c_str(), true, nullptr);
            }
        }
    }
    else {
        ESP_LOGW(RECEIVE_TAG, "Unknown TCP action: %s", action.c_str());
    }
    saveJson();
}

void processWebSocketMessage(const String& data, int64_t timestamp) {
    JsonDocument receivedData;
    DeserializationError error = deserializeJson(receivedData, data);
    if (error) {
        ESP_LOGE(RECEIVE_TAG, "Failed to parse JSON from WebSocket: %s", error.c_str());
        return;
    }

    if (!receivedData.containsKey("ACTION") || !receivedData.containsKey("MSG")) {
        ESP_LOGE(RECEIVE_TAG, "Invalid WebSocket JSON structure");
        return;
    }

    const char* action = receivedData["ACTION"];
    JsonObject message = receivedData["MSG"].as<JsonObject>();

    // Le message est déjà validé, le traiter directement
    processDataFromSocket(action, message, timestamp);
}

void receiveMessageTask(void *parameter) {
    IncomingMessage_t* receivedMessage;
    while (1) {
        ESP_LOGD(RECEIVE_TAG, "Waiting for incoming messages");
        if (xQueueReceive(incomingQueue, &receivedMessage, portMAX_DELAY)) {
            ESP_LOGI(RECEIVE_TAG, "dequeue message %i from %s : %s", receivedMessage->msgID,
                     receivedMessage->source.c_str(), receivedMessage->data->c_str());
            
            // Traiter le message selon sa source
            if (receivedMessage->source == "TCP") {
                processTCPMessage(*(receivedMessage->data), receivedMessage->client, receivedMessage->timestamp);
            } 
            else if (receivedMessage->source == "WebSocket") {
                processWebSocketMessage(*(receivedMessage->data), receivedMessage->timestamp);
            }
            else {
                ESP_LOGW(RECEIVE_TAG, "Unknown message source: %s", receivedMessage->source.c_str());
            }
            
            // Nettoyage
            delete receivedMessage->data;
        //    delete receivedMessage->timestamp;
            delete receivedMessage;
        }
    }
}