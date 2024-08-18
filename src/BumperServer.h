
void resetBumpersTime() {
  std::vector<String> keysToRemove = {"STATUS", "BUTTON", "TIME", "DELAY", "DELAY_TEAM"};

  JsonObject bumpers=teamsAndBumpers["bumpers"];
  JsonObject teams=teamsAndBumpers["teams"];
  for (JsonPair kvp : bumpers) {
    if (kvp.value().is<JsonObject>()) {
      JsonObject bumper = kvp.value().as<JsonObject>();

      for (String key : keysToRemove) {
        if (bumper.containsKey(key)) {
          bumper.remove(key);
        }    
      }
    } else {
        Serial.println("Error: Bumper entry is not a JsonObject");
    }
  }
  for (JsonPair team : teams) {
    if (team.value().is<JsonObject>()) {
      JsonObject teamData = team.value().as<JsonObject>();

      for (String key : keysToRemove) {
        if (teamData.containsKey(key)) {
          teamData.remove(key);
        }    
      }
    } else {
        Serial.println("Error: Team entry is not a JsonObject");
    }
  }
  timeRef=0;
  for (const auto& pair : timeRefTeam) {
    timeRefTeam[pair.first]=0;
  }
}

void startGame(){
  resetBumpersTime();
  GameStarted=true;
  putMsgToQueue("START","",true);
  
}

void stopGame(){
  GameStarted=false;
  putMsgToQueue("STOP","",true);

}

void pauseGame(AsyncClient* client) {
  putMsgToQueue("PAUSE","",true, client);
}

// Fonction pour fusionner deux documents JSON
void mergeJson(JsonObject& destObj, const JsonObject& srcObj) {
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

    Serial.printf("Update: received %s\n", output.c_str());

    mergeJson(jsonObj, obj);
    serializeJson(jsonObj, output);
        // Fusionne le JSON reçu avec 'teams'
    Serial.printf("Update: complete %s\n", output.c_str());
    
    notifyAll();
}
