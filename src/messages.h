void notifyAll() {
  String output;
  serializeJson(teamsAndBumpers, output);
  saveJson();
  Serial.printf("SOCK: send to all %s\n", output.c_str());
  ws.textAll(output.c_str());
  sendMessageToAllClients("UPDATE", output.c_str() );
}

void putMsgToQueue(const char* action, const char* msg, bool notify, AsyncClient* client )
{
  messageQueue_t message;

    message.action = action;
    message.message = msg;
    if (msg == "") { message.message="''"; }
    
    message.notifyAll = notify;
    message.client = client;

    xQueueSend(messageQueue, &message, portMAX_DELAY); // Utilisation en dehors de l'ISR
}

void sendMessageToClient(const String& action, const String& msg, AsyncClient* client) {
Serial.println("BUMPER: send to "+client->remoteIP().toString());
Serial.println("BUMPER: action "+action);
Serial.println("BUMPER: message "+msg);
String message="{ \"ACTION\": \"" + action + "\", \"MSG\": " + msg + "}\n";
    if (client && client->connected()) {
      client->write(message.c_str(), message.length()); // Envoie le message au client connecté
    }
}

void sendMessageToAllClients(const String& action, const String& msg ) {
  // Parcourez tous les clients connectés
  Serial.println("BUMPER: send to all");
  for (AsyncClient* client : bumperClients) {
    sendMessageToClient(action, msg, client);
  }
  Serial.println("BUMPER: all is sent");
}

void sendMessageTask(void *parameter) {
    messageQueue_t receivedMessage;

    while (1) {
        // Attendre qu'un message soit disponible dans la file d'attente
        if (xQueueReceive(messageQueue, &receivedMessage, portMAX_DELAY)) {
          Serial.print("BUZZCONTROL: new message in queue: "); 
          if (receivedMessage.action != nullptr) {
            Serial.println(receivedMessage.action);
            if (strcmp(receivedMessage.action, "START")) {
              setLedColor(255, 0, 0);
              setLedIntensity(255);
            }
            if (strcmp(receivedMessage.action , "STOP")) {
              setLedColor(0, 255, 0);
              setLedIntensity(255);
            }
            if (strcmp(receivedMessage.action , "PAUSE")) {
              setLedColor(255, 255, 0);
              setLedIntensity(64);
            }
            // Envoyer le message à travers la socket TCP
            // Vous pouvez remplacer cette partie par votre code de socket
            if (receivedMessage.client != nullptr) {
              sendMessageToClient(receivedMessage.action, receivedMessage.message, receivedMessage.client);
            }
            else {
              sendMessageToAllClients(receivedMessage.action, receivedMessage.message);
            }
          }
          if (receivedMessage.notifyAll == true) {
            notifyAll();
          }
        }
    }
}

