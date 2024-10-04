#include <ArduinoJson.h>

JsonDocument teamsAndBumpers;

JsonDocument& getTeamsAndBumpers() {
    return teamsAndBumpers;
}

//### BUMPERS ###
JsonObject getBumpers() {
    return teamsAndBumpers["bumpers"].as<JsonObject>();
}

void setBumpers(const JsonObject& bumpers) {
  teamsAndBumpers["bumpers"] = bumpers;
}

JsonObject getBumper(const char* bumperID) {
    return teamsAndBumpers["bumpers"][String(bumperID)].as<JsonObject>();
}

void setBumper(const char* bumperID, const JsonObject& bumper){
  teamsAndBumpers["bumpers"][String(bumperID)] = bumper;
}

void setBumperIP(const char* bumperID, const char* IP) {
    if (bumperID == nullptr || strlen(bumperID) == 0) {
        ESP_LOGE("Teams&Bumpers", "Invalid bumperID provided: %s", bumperID);
    }
    if (IP == nullptr || strlen(IP) == 0) {
        ESP_LOGE("Teams&Bumpers", "Invalid IP address provided: %s", IP);
    }

    ESP_LOGD("Teams&Bumpers","set IP: %s:%s", bumperID, IP);

    String bID = String(bumperID);
    if (teamsAndBumpers["bumpers"].isNull()) {
        teamsAndBumpers["bumpers"] = JsonObject();
    }

    if (teamsAndBumpers["bumpers"][bID].isNull()) {
        teamsAndBumpers["bumpers"][bID]=JsonObject();
    }
    String ipCopy = String(IP);
    teamsAndBumpers["bumpers"][bID]["IP"] = ipCopy;

}

void setBumperNAME(const char* bumperID, const char* NAME) {
    if (bumperID == nullptr || strlen(bumperID) == 0) {
        ESP_LOGE("Teams&Bumpers", "Invalid bumperID provided: %s", bumperID);
    }
    if (NAME == nullptr || strlen(NAME) == 0) {
        ESP_LOGE("Teams&Bumpers", "Invalid NAME provided: %s", NAME);
    }
    ESP_LOGD("Teams&Bumpers","set NAME: %s:%s", bumperID, NAME);

    if (teamsAndBumpers["bumpers"].isNull()) {
        teamsAndBumpers["bumpers"] = JsonObject();
    }
    String bID = String(bumperID);
    if (teamsAndBumpers["bumpers"][bID].isNull()) {
        teamsAndBumpers["bumpers"][bID]=JsonObject();
    }
    String NAMECopy = String(NAME);
    teamsAndBumpers["bumpers"][bID]["NAME"] = NAMECopy;
}

void  mergeJson(JsonObject& destObj, const JsonObject& srcObj) {
  for (JsonPair kvp : srcObj) {
      ESP_LOGD("Teams&Bumpers","Merging: %s", kvp.key());

      destObj[kvp.key()] = kvp.value();
    }
  
}

void updateBumper(const char* bumperID, const JsonObject& new_bumper) {
    String output;
    serializeJson(new_bumper, output);
  ESP_LOGD("Teams&Bumpers","Updating: bumper %s : %s", bumperID, output.c_str());
  JsonObject bumper = getBumper(bumperID);
  if (bumper.isNull()) {
    setBumper(bumperID,new_bumper);

  } else {
      if (!new_bumper["IP"].isNull()) { setBumperIP(bumperID, new_bumper["IP"]); };
      if (!new_bumper["NAME"].isNull()) { setBumperIP(bumperID, new_bumper["NAME"]); };
  }
  bumper = getBumper(bumperID);
  serializeJson(bumper, output);
  ESP_LOGD("Teams&Bumpers","Updating: merged bumper %s : %s", bumperID, output.c_str());
  
}

void updateBumpers(const JsonObject& bumpers) {
    String output;
    serializeJson(bumpers, output);
  ESP_LOGD("Teams&Bumpers","Updating: bumpers %s", output.c_str());
  for (JsonPair obj : bumpers) {
        updateBumper(obj.key().c_str(), obj.value().as<JsonObject>());
      }
}


String getBumperIDByIP(const char* clientIP) {
  JsonObject bumpers = getBumpers();
  for (JsonPair bumperPair : bumpers) {
    JsonObject bumper = bumperPair.value().as<JsonObject>();
    if (bumper["IP"].as<String>() == clientIP) {
      return String(bumperPair.key().c_str());
    }
  }
  return String(); // Retourne une chaîne vide si aucune correspondance n'est trouvée
}

//#### TEAMS ###
JsonObject getTeams() {
    return teamsAndBumpers["teams"].as<JsonObject>();
}

JsonObject getTeam(const char* teamID) {
    return teamsAndBumpers["teams"][teamID].as<JsonObject>();
}

void setTeams(const JsonObject& teams) {
  teamsAndBumpers["teams"] = teams;
}

void setTeam(const char* teamID, const JsonObject& team){
  teamsAndBumpers["teams"][teamID] = team;
}

void updateTeam(const char* teamID, const JsonObject& new_team) {
  JsonObject team = getTeam(teamID);
  mergeJson(team, new_team);
  setTeam(teamID, team);
}

void updateTeams(const JsonObject& teams) {
  for (JsonPair obj : teams) {
        updateTeam(obj.key().c_str(), obj.value().as<JsonObject>());
      }
}
