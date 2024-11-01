#pragma once
#include "Common/CustomLogger.h"
#include "Common/led.h"

#include <ArduinoJson.h>
static const char* TEAMs_TAG = "Team And Bumper";
JsonDocument teamsAndBumpers;

JsonDocument& getTeamsAndBumpers() {
    return teamsAndBumpers;
}
// ### GAME ### */
void setGamePhase(String phase) {
    if (teamsAndBumpers["GAME"].isNull()) {
        teamsAndBumpers["GAME"] = JsonObject();
    }
    teamsAndBumpers["GAME"]["PHASE"] = phase;
}

void setGameTime() {
    if (teamsAndBumpers["GAME"].isNull()) {
        teamsAndBumpers["GAME"] = JsonObject();
    }
        teamsAndBumpers["GAME"]["TIME"] = micros();
}

void setGameCurrentTime(const int currentTime) {
    if (teamsAndBumpers["GAME"].isNull()) {
        teamsAndBumpers["GAME"] = JsonObject();
    }
    teamsAndBumpers["GAME"]["CURRENT_TIME"] = currentTime;

}

void setGameDelay(int delay=33) {
    if (teamsAndBumpers["GAME"].isNull()) {
        teamsAndBumpers["GAME"] = JsonObject();
    }
    teamsAndBumpers["GAME"]["DELAY"] = delay;
}

void setGamePage(const String remotePage) {
    if (teamsAndBumpers["GAME"].isNull()) {
        teamsAndBumpers["GAME"] = JsonObject();
    }
        teamsAndBumpers["GAME"]["REMOTE"] = remotePage;
}

String getTeamsAndBumpersJSON() {
  String output;
  JsonDocument& tb=getTeamsAndBumpers();
  if (serializeJson(tb, output)) {
    return output;
  } else {
    ESP_LOGE(TEAMs_TAG, "Failed to serialize JSON");
  }
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

String  ensureBumperExists(const char* bumperID) {
    if (bumperID == nullptr || strlen(bumperID) == 0) {
        ESP_LOGE(TEAMs_TAG, "Invalid bumperID provided: %s", bumperID);
    }
    String bID = String(bumperID);
    if (teamsAndBumpers["bumpers"].isNull()) {
        teamsAndBumpers["bumpers"] = JsonObject();
    }

    if (teamsAndBumpers["bumpers"][bID].isNull()) {
        teamsAndBumpers["bumpers"][bID]=JsonObject();
    }
    return bID;
}

void setBumperIP(const char* bumperID, const char* IP) {
/*
    if (bumperID == nullptr || strlen(bumperID) == 0) {
        ESP_LOGE("Teams&Bumpers", "Invalid bumperID provided: %s", bumperID);
    }
*/
    if (IP == nullptr || strlen(IP) == 0) {
        ESP_LOGE("Teams&Bumpers", "Invalid IP address provided: %s", IP);
    }

    ESP_LOGD("Teams&Bumpers","set IP: %s => %s", bumperID, IP);

    String bID = ensureBumperExists(bumperID);
/*
    String bID = String(bumperID);
    if (teamsAndBumpers["bumpers"].isNull()) {
        teamsAndBumpers["bumpers"] = JsonObject();
    }

    if (teamsAndBumpers["bumpers"][bID].isNull()) {
        teamsAndBumpers["bumpers"][bID]=JsonObject();
    }
*/
    String ipCopy = String(IP);
    teamsAndBumpers["bumpers"][bID]["IP"] = ipCopy;

}

void setBumperNAME(const char* bumperID, const char* NAME) {
/*
    if (bumperID == nullptr || strlen(bumperID) == 0) {
        ESP_LOGE(TEAMs_TAG, "Invalid bumperID provided: %s", bumperID);
    }
*/
    if (NAME == nullptr || strlen(NAME) == 0) {
        ESP_LOGE(TEAMs_TAG, "Invalid NAME provided: %s", NAME);
    }
    ESP_LOGD(TEAMs_TAG,"set NAME: %s:%s", bumperID, NAME);

    String bID = ensureBumperExists(bumperID);
/*    String bID = String(bumperID);

    if (teamsAndBumpers["bumpers"].isNull()) {
        teamsAndBumpers["bumpers"] = JsonObject();
    }
    
    if (teamsAndBumpers["bumpers"][bID].isNull()) {
        teamsAndBumpers["bumpers"][bID]=JsonObject();
    }
*/
    String NAMECopy = String(NAME);
    teamsAndBumpers["bumpers"][bID]["NAME"] = NAMECopy;
    ESP_LOGI(TEAMs_TAG, "Bumper NAME %s => %s", bID, NAMECopy);
}

void setBumperVERSION(const char* bumperID, const char* version) {
/*
    if (bumperID == nullptr || strlen(bumperID) == 0) {
        ESP_LOGE("Teams&Bumpers", "Invalid bumperID provided: %s", bumperID);
    }
*/
    if (version == nullptr || strlen(version) == 0) {
        ESP_LOGE("Teams&Bumpers", "Invalid VERSION address provided: %s", version);
    }

    ESP_LOGD("Teams&Bumpers","set VERSION: %s => %s", bumperID, version);

    String bID = ensureBumperExists(bumperID);
/*
    String bID = String(bumperID);
    if (teamsAndBumpers["bumpers"].isNull()) {
        teamsAndBumpers["bumpers"] = JsonObject();
    }

    if (teamsAndBumpers["bumpers"][bID].isNull()) {
        teamsAndBumpers["bumpers"][bID]=JsonObject();
    }
*/
    String ipCopy = String(version);
    teamsAndBumpers["bumpers"][bID]["VERSION"] = ipCopy;

}

void setBumperButton(const char* bumperID, int button) {
/*
    if (bumperID == nullptr || strlen(bumperID) == 0) {
        ESP_LOGE(TEAMs_TAG, "Invalid bumperID provided: %s", bumperID);
    }
*/
    String bID = ensureBumperExists(bumperID);
/*
    if (teamsAndBumpers["bumpers"].isNull()) {
        teamsAndBumpers["bumpers"] = JsonObject();
    }
    String bID = String(bumperID);
    if (teamsAndBumpers["bumpers"][bumperID].isNull()) {
        teamsAndBumpers["bumpers"][bumperID]=JsonObject();
    }
*/
    int copy = int(button);
    teamsAndBumpers["bumpers"][bID]["BUTTON"] = copy;
    ESP_LOGI(TEAMs_TAG, "Bumper Button %s %i", bID, copy);
}

void setBumperStatus(const char* bumperID, String status) {
/*
    if (bumperID == nullptr || strlen(bumperID) == 0) {
        ESP_LOGE(TEAMs_TAG, "Invalid bumperID provided: %s", bumperID);
    }
*/
    String bID = ensureBumperExists(bumperID);
/*
    if (teamsAndBumpers["bumpers"].isNull()) {
        teamsAndBumpers["bumpers"] = JsonObject();
    }
    String bID = String(bumperID);
    if (teamsAndBumpers["bumpers"][bumperID].isNull()) {
        teamsAndBumpers["bumpers"][bumperID]=JsonObject();
    }
*/
    String copy = String(status);
    teamsAndBumpers["bumpers"][bID]["STATUS"] = copy;
    ESP_LOGI(TEAMs_TAG, "Bumper Status %s %s", bID, copy);
}

void setBumperScore(const char* bumperID, const int new_score) {
/*
    if (bumperID == nullptr || strlen(bumperID) == 0) {
        ESP_LOGE(TEAMs_TAG, "Invalid bumperID provided: %s", bumperID);
    }
*/
    String bID = ensureBumperExists(bumperID);
/*
    if (teamsAndBumpers["bumpers"].isNull()) {
        teamsAndBumpers["bumpers"] = JsonObject();
    }
*/
    int copy = int(new_score);
    teamsAndBumpers["bumpers"][bID]["SCORE"]=copy;
}

const int64_t getBumperTime(const char* bumperID) {
    return int64_t(teamsAndBumpers["bumpers"][bumperID]["TIMESTAMP"]);
}

void setBumperTime(const char* bumperID, const int64_t new_delay) {
/*
    if (bumperID == nullptr || strlen(bumperID) == 0) {
        ESP_LOGE(TEAMs_TAG, "Invalid bumperID provided: %s", bumperID);
    }
*/
    String bID = ensureBumperExists(bumperID);
/*
    if (teamsAndBumpers["bumpers"].isNull()) {
        teamsAndBumpers["bumpers"] = JsonObject();
    }
*/
    int64_t copy = int64_t(new_delay);
    teamsAndBumpers["bumpers"][bID]["TIMESTAMP"]=copy;
    ESP_LOGI(TEAMs_TAG, "BumperID Delay %s %i", bID, copy);
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
      if (!new_bumper["NAME"].isNull()) { setBumperNAME(bumperID, new_bumper["NAME"]); };
      if (!new_bumper["VERSION"].isNull()) { setBumperVERSION(bumperID, new_bumper["VERSION"]); };
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

String  ensureTeamExists(const char* teamID) {
    if (teamID == nullptr || strlen(teamID) == 0) {
        ESP_LOGE(TEAMs_TAG, "Invalid teamID provided: %s", teamID);
    }
    if (teamsAndBumpers["teams"].isNull()) {
        teamsAndBumpers["teams"] = JsonObject();
    }
    String bID = String(teamID);
    if (teamsAndBumpers["teams"][bID].isNull()) {
        teamsAndBumpers["teams"][bID]=JsonObject();
    }
    return bID;
}

void setTeamStatus(const char* teamID, String status) {
/*
    if (teamID == nullptr || strlen(teamID) == 0) {
        ESP_LOGE(TEAMs_TAG, "Invalid teamID provided: %s", teamID);
    }
    if (teamsAndBumpers["teams"].isNull()) {
        teamsAndBumpers["teams"] = JsonObject();
    }
    String bID = String(teamID);
    if (teamsAndBumpers["teams"][teamID].isNull()) {
        teamsAndBumpers["teams"][teamID]=JsonObject();
    }
*/
    String bID = ensureTeamExists(teamID);
    String copy = String(status);
    teamsAndBumpers["teams"][bID]["STATUS"] = copy;
    ESP_LOGI(TEAMs_TAG, "Team Status %s %s", bID, copy);
}

const int64_t getTeamTime(const char* teamID) {
    return int64_t(teamsAndBumpers["teams"][teamID]["TIMESTAMP"]);
}

void setTeamTime(const char* teamID, const int64_t new_delay) {
/*
    if (teamID == nullptr || strlen(teamID) == 0) {
        ESP_LOGE(TEAMs_TAG, "Invalid teamID provided: %s", teamID);
    }
    if (teamsAndBumpers["teams"].isNull()) {
        teamsAndBumpers["teams"] = JsonObject();
    }
    
    if (teamsAndBumpers["teams"][teamID].isNull()) {
        teamsAndBumpers["teams"][teamID]=JsonObject();
    }
*/
    String bID = ensureTeamExists(teamID);
    int64_t copy = int64_t(new_delay);
    teamsAndBumpers["teams"][bID]["TIMESTAMP"]=copy;
    ESP_LOGI(TEAMs_TAG, "Team Delay %s %i", bID, copy);
}

void setTeamBumper(const char* teamID, const char* bumperID) {
/*
    if (teamID == nullptr || strlen(teamID) == 0) {
        ESP_LOGE(TEAMs_TAG, "Invalid bumperID provided: %s", teamID);
    }
    if (teamsAndBumpers["teams"].isNull()) {
        teamsAndBumpers["teams"] = JsonObject();
    }
    String bID = String(teamID);
    if (teamsAndBumpers["teams"][teamID].isNull()) {
        teamsAndBumpers["teams"][teamID]=JsonObject();
    }
*/
    String bID = ensureTeamExists(teamID);
    String copy = String(bumperID);
    teamsAndBumpers["teams"][bID]["BUMPER"] = copy;
//    setTeamTime(teamID, micros());
    ESP_LOGI(TEAMs_TAG, "Team Bumper %s %s", teamID, copy.c_str());
}

void setTeamScore(const char* teamID, const int new_score) {
/*
    if (teamID == nullptr || strlen(teamID) == 0) {
        ESP_LOGE(TEAMs_TAG, "Invalid bumperID provided: %s", teamID);
    }
    if (teamsAndBumpers["teams"].isNull()) {
        teamsAndBumpers["teams"] = JsonObject();
    }
    
    if (teamsAndBumpers["teams"][teamID].isNull()) {
        teamsAndBumpers["teams"][teamID]=JsonObject();
    }

 */
    String bID = ensureTeamExists(teamID);
    int copy = int(new_score);
    teamsAndBumpers["teams"][bID]["SCORE"] = copy;
    ESP_LOGI(TEAMs_TAG, "Team Score %s %i", bID, copy);
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
