#pragma once
#include "Common/CustomLogger.h"
#include "Common/led.h"
#include "messages_received.h"

#include <ArduinoJson.h>

const char* SOCKET_TAG = "SOCKET";

void handleWebSocketData(AsyncWebSocketClient *client, uint8_t *data, size_t len) {
  IPAddress clientIP = client->remoteIP();
  String ipStr = clientIP.toString();

  ESP_LOGI(SOCKET_TAG, "Received WebSocket data from client %u (IP: %s)", client->id(), ipStr.c_str());

  if (len > 0) {
    // Limiter la taille des données loguées pour éviter de surcharger les logs
    const size_t maxLogLength = 100;
    size_t logLength = len < maxLogLength ? len : maxLogLength;

    // Créer une copie temporaire des données pour s'assurer qu'elles sont null-terminées
    char logBuffer[maxLogLength + 1];
    memcpy(logBuffer, data, logLength);
    logBuffer[logLength] = '\0';  // Ajouter un caractère nul à la fin

    // Logger les données en tant que chaîne de caractères
    ESP_LOGD(SOCKET_TAG, "Received WebSocket data from %s (first %u bytes): %s%s", ipStr.c_str(),
             logLength, logBuffer, len > maxLogLength ? "..." : "");
  } else {
    ESP_LOGD(SOCKET_TAG, "Received empty WebSocket data");
    return;
  }

  // Utiliser le même système de buffer que pour TCP
  static std::map<String, String> wsClientBuffers;
  String clientID = ipStr + "_" + String(client->id());
  String partial_data = String((char*)data, len);
  
  // Ajouter les données au buffer du client
  wsClientBuffers[clientID] += partial_data;
  
  // Traiter le buffer du client, à la recherche de messages JSON complets
  String& jsonBuffer = wsClientBuffers[clientID];
  int endOfJson;
  
  while ((endOfJson = jsonBuffer.indexOf('\0')) > 0) {
    String jsonPart = jsonBuffer.substring(0, endOfJson);
    jsonBuffer = jsonBuffer.substring(endOfJson + 1);
    
    // Envoyer le message JSON complet à la file d'attente
    enqueueIncomingMessage("WebSocket", jsonPart.c_str(), nullptr);
  }
}

void onWsEvent(AsyncWebSocket *server, AsyncWebSocketClient *client, AwsEventType type, void *arg, uint8_t *data, size_t len) {
  IPAddress clientIP = client->remoteIP();
  String ipStr = clientIP.toString();
  
  switch(type) {
    case WS_EVT_CONNECT:
      // Quand un client se connecte, envoyer un message
      ESP_LOGI(SOCKET_TAG, "WebSocket client %u IP: %s connected", client->id(), ipStr.c_str());
      break;
      
    case WS_EVT_DISCONNECT:
      // Quand un client se déconnecte
      ESP_LOGI(SOCKET_TAG, "WebSocket client %u disconnected", client->id());
      break;
      
    case WS_EVT_DATA:
      handleWebSocketData(client, data, len);
      break;
      
    default:
      break;
  }
}