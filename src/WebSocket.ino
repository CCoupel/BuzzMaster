void onWsEvent(AsyncWebSocket *server, AsyncWebSocketClient *client, AwsEventType type, void *arg, uint8_t *data, size_t len) {
    String output;
    JsonDocument receivedData;
    if (type == WS_EVT_CONNECT) {
        // Quand un client se connecte, envoyer un message
        Serial.printf("SOCK: Client %u connecté\n", client->id());
        //notifyAll();
        //client->text("Bienvenue sur le serveur WebSocket !");
    } else if (type == WS_EVT_DISCONNECT) {
        // Quand un client se déconnecte
        Serial.printf("SOCK: Client %u déconnecté\n", client->id());
    } else if (type == WS_EVT_DATA) {
        
        // Quand un message est reçu, réagir
        Serial.printf("SOCK: Message reçu de client %u : %.*s\n", client->id(), len, data);
        
        deserializeJson(receivedData, data);
        //JsonObject jsonObj = teamsAndBumpers.as<JsonObject>();
        JsonObject jsonObjData = receivedData.as<JsonObject>();
        // Fusionne le JSON reçu avec 'teams'
        update("ADD", jsonObjData);
    }
}

// Fonction pour fusionner deux documents JSON
void mergeJson(JsonObject& destObj, const JsonObject& srcObj) {
    //JsonObject destObj = dest.as<JsonObject>();
    //JsonObject srcObj = src.as<JsonObject>();
  String msg="Merging :";
  for (JsonPair kvp : srcObj) {
      if (kvp.value().is<JsonObject>()) {
        JsonObject nestedDestObj;
        if (destObj.containsKey(kvp.key()) && destObj[kvp.key()].is<JsonObject>()) {
                nestedDestObj = destObj[kvp.key()].as<JsonObject>();
            } else {
                //nestedDestObj = destObj.createNestedObject(kvp.key());
                nestedDestObj = destObj[kvp.key()].to<JsonObject>();
            }

        //JsonObject dst = destObj[kvp.key()].as<JsonObject>();
        JsonObject nestedSrcObj  = kvp.value().as<JsonObject>();
        mergeJson(nestedDestObj, nestedSrcObj );
      }
      else {
        destObj[kvp.key()] = kvp.value();
      }
    }

}

void update(String action, JsonObject& obj) {
    JsonObject jsonObj = teamsAndBumpers.as<JsonObject>();
    String output;
      serializeJson(obj, output);
        // Fusionne le JSON reçu avec 'teams'
    //Serial.printf("Update: %s", obj);

    Serial.printf("Update: received %s\n", output.c_str());

    mergeJson(jsonObj, obj);
    serializeJson(jsonObj, output);
        // Fusionne le JSON reçu avec 'teams'
    Serial.printf("Update: complete %s\n", output.c_str());
    
    notifyAll();
}

void notifyAll() {
  String output;
  serializeJson(teamsAndBumpers, output);
  Serial.printf("SOCK: send to all %s\n", output.c_str());
  ws.textAll(output.c_str());
}
