void startBumperServer()
{
  bumperServer = new AsyncServer(1234);
  bumperServer->onClient(&b_onCLientConnect, bumperServer);
  
  bumperServer->begin();
    Serial.print("BUMPER server started on port ");
  Serial.println(1234);

}

void b_handleData(void* arg, AsyncClient* c, void *data, size_t len) {
//  JsonDocument obj;
  String s_data=String((char*)data).substring(0, len);
  String clientID = c->remoteIP().toString();
  
  Serial.print("BUZZER: Data received: "+s_data);

  // Ajouter les données au buffer spécifique du client
  clientBuffers[clientID] += s_data;
    // Traiter les messages complets dans le buffer
  processClientBuffer(clientID, c);
}

void processClientBuffer(const String& clientID, AsyncClient* c) {
  String& jsonBuffer = clientBuffers[clientID];

  int endOfJson = jsonBuffer.indexOf('\n'); // Ajuster si nécessaire
  while (endOfJson > 0) {
    // Extraire le JSON complet
    String jsonPart = jsonBuffer.substring(0, endOfJson);
    jsonBuffer = jsonBuffer.substring(endOfJson + 1); // Mettre à jour le tampon pour les données restantes
    
    // Appel à la fonction parseJSON pour gérer la logique de traitement du JSON
    parseJSON(jsonPart,c);
    
    endOfJson = jsonBuffer.indexOf('\n'); // Rechercher le prochain message JSON
  }

};

static void b_onClientDisconnect(void* arg, AsyncClient* client) {
  JsonObject bumpers = teamsAndBumpers["bumpers"];
  String clientID = client->remoteIP().toString();
  Serial.printf("Client disconnected\n");
  
  clientBuffers.erase(clientID);

  for (JsonPair bumperPair : bumpers) {
    JsonObject bumper = bumperPair.value().as<JsonObject>();
    if (bumper["IP"].as<String>() == clientID) {
      bumper["STATUS"] = "offline";
      notifyAll();
      break;
    }
  }

  bumperClients.erase(std::remove(bumperClients.begin(), bumperClients.end(), client), bumperClients.end()); // Retirer le client de la liste
  delete client; // Nettoyer la mémoire
  
}

static void b_onCLientConnect(void* arg, AsyncClient* client) {
  Serial.println("Buzzer: New client connected");
  client->onData(&b_handleData, NULL);
  client->onDisconnect(&b_onClientDisconnect, NULL);
  bumperClients.push_back(client);

}
