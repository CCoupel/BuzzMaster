void checkPingForAllClients() {
  unsigned long currentTime = millis();

  // Vérifier les bumpers non répondus
  JsonObject bumpers = teamsAndBumpers["bumpers"];
  for (JsonPair bumperPair : bumpers) {
    JsonObject bumper = bumperPair.value().as<JsonObject>();
    if (currentTime - bumper["lastPingTime"].as<unsigned long>() > 3000) {  // 2 secondes sans réponse
      if (bumper["STATUS"] != "offline") {
        Serial.print("BUMPER: Bumper ");
        Serial.print(bumper["IP"].as<String>());
        Serial.println(" is going offline");
        bumper["STATUS"] = "offline";
        notifyAll();
      }
    }
  }
}

void onTimerISR() {
  sendMessageToAllClients("PING", "'Are you alive?'");  // Appelée toutes les secondes
  checkPingForAllClients();
}