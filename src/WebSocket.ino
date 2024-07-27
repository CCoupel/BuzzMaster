void onWsEvent(AsyncWebSocket *server, AsyncWebSocketClient *client, AwsEventType type, void *arg, uint8_t *data, size_t len) {
    String output;
    JsonDocument receivedData;
    if (type == WS_EVT_CONNECT) {
        // Quand un client se connecte, envoyer un message
        Serial.printf("Client %u connecté\n", client->id());
        serializeJson(teamsAndBumpers, output);
        ws.textAll(output.c_str());
        //client->text("Bienvenue sur le serveur WebSocket !");
    } else if (type == WS_EVT_DISCONNECT) {
        // Quand un client se déconnecte
        Serial.printf("Client %u déconnecté\n", client->id());
    } else if (type == WS_EVT_DATA) {
        // Quand un message est reçu, réagir
        Serial.printf("Message reçu de client %u : %s\n", client->id(), (char*)data);
        
        deserializeJson(receivedData, data);
        JsonObject jsonObj = teamsAndBumpers.as<JsonObject>();
        JsonObject jsonObjData = receivedData.as<JsonObject>();
        // Fusionne le JSON reçu avec 'teams'
        mergeJson(jsonObj, jsonObjData);
        // Exemple : renvoyer le même message à tous les clients
        serializeJson(teamsAndBumpers, output);
        ws.textAll(output.c_str());
    }
}
