#pragma once
#include "Common/CustomLogger.h"
#include "Common/led.h"

#include <ArduinoJson.h>

static const char* TEAMs_TAG = "Team And Bumper";
static const char* QUESTION_TAG = "Questions";

JsonDocument teamsAndBumpers;
JsonDocument& getTeamsAndBumpers() {
    return teamsAndBumpers;
}

JsonObject getKEYObj(String key) {
  JsonDocument& doc=getTeamsAndBumpers();
  if (doc[key].isNull()) {
    doc[key] = JsonObject();
  }
  return doc[key].as<JsonObject>();
}

JsonObject getTeamsObj() {
  return getKEYObj("teams");
}

JsonObject getBumpersObj() {
  return getKEYObj("bumpers");
}

JsonObject getGameObj() {
  return getKEYObj("GAME");
}

String getTeamsAndBumpersJSON() {
  String output;
  JsonDocument& tb=getTeamsAndBumpers();
  if (serializeJson(tb, output)) {
    ESP_LOGI(TEAMs_TAG, "TeamsAndGame: %s", output.c_str());
    return output;
  } else {
    ESP_LOGE(TEAMs_TAG, "Failed to serialize JSON");
    return "{}";
  }
}

// ### GAME ### */
void setBackgroundFile(String pathBackground) {
    String path;
  if (teamsAndBumpers["GAME"].isNull()) {
        teamsAndBumpers["GAME"] = JsonObject();
    }
  // Copy the GAME object to our temporary document
  
  if (!teamsAndBumpers["GAME"]["background"].isNull()) {
    path=teamsAndBumpers["GAME"]["background"].as<String>();
    deleteFile(path.c_str());
  }

  teamsAndBumpers["GAME"]["background"]=pathBackground;
}

String getGameJSON() {
  String output;
  JsonDocument doc;
  
  // Copy the GAME object to our temporary document
  doc["GAME"] = getGameObj();
  
  if (serializeJson(doc, output)) {
    return output;
  } else {
    ESP_LOGE(TEAMs_TAG, "Failed to serialize JSON");
    return "{}";
  }
}

void setGamePhase(String phase) {
    getGameObj()["PHASE"] = phase;
}

String getGamePhase() {
    if (teamsAndBumpers["GAME"].isNull()) {
        teamsAndBumpers["GAME"] = JsonObject();
    }
    if (teamsAndBumpers["GAME"]["PHASE"].isNull()) {
        teamsAndBumpers["GAME"]["PHASE"] = "";
    }
    return teamsAndBumpers["GAME"]["PHASE"];
}

bool isGameStarted() {
    if (teamsAndBumpers["GAME"].isNull()) {
        teamsAndBumpers["GAME"] = JsonObject();
    }
    if (teamsAndBumpers["GAME"]["PHASE"].isNull()) {
        teamsAndBumpers["GAME"]["PHASE"] = "";
    }
    if (teamsAndBumpers["GAME"]["PHASE"]=="START") {
        return true;
    }
    return false;
}

bool isGameStopped() {
    if (teamsAndBumpers["GAME"].isNull()) {
        teamsAndBumpers["GAME"] = JsonObject();
    }
    if (teamsAndBumpers["GAME"]["PHASE"].isNull()) {
        teamsAndBumpers["GAME"]["PHASE"] = "";
    }
    if (teamsAndBumpers["GAME"]["PHASE"]=="STOP") {
        return true;
    }
    return false;
}

bool isGamePrepare() {
    if (teamsAndBumpers["GAME"].isNull()) {
        teamsAndBumpers["GAME"] = JsonObject();
    }
    if (teamsAndBumpers["GAME"]["PHASE"].isNull()) {
        teamsAndBumpers["GAME"]["PHASE"] = "";
    }
    if (teamsAndBumpers["GAME"]["PHASE"]=="PREPARE") {
        return true;
    }
    return false;
}

bool isGameReady() {
    if (teamsAndBumpers["GAME"].isNull()) {
        teamsAndBumpers["GAME"] = JsonObject();
    }
    if (teamsAndBumpers["GAME"]["PHASE"].isNull()) {
        teamsAndBumpers["GAME"]["PHASE"] = "";
    }
    if (teamsAndBumpers["GAME"]["PHASE"]=="READY") {
        return true;
    }
    return false;
}

bool isGamePaused() {
    if (teamsAndBumpers["GAME"].isNull()) {
        teamsAndBumpers["GAME"] = JsonObject();
    }
    if (teamsAndBumpers["GAME"]["PHASE"].isNull()) {
        teamsAndBumpers["GAME"]["PHASE"] = "";
    }
    if (teamsAndBumpers["GAME"]["PHASE"]=="PAUSE") {
        return true;
    }
    return false;
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

int getGameCurrentTime() {
    if (teamsAndBumpers["GAME"].isNull()) {
        teamsAndBumpers["GAME"] = JsonObject();
    }
    if (teamsAndBumpers["GAME"]["CURRENT_TIME"].isNull()) {
        teamsAndBumpers["GAME"]["CURRENT_TIME"] = 0;
    }
    return teamsAndBumpers["GAME"]["CURRENT_TIME"];

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
//##### QUESTION ######
int findFreeQuestion() {
    try {
        if (!LittleFS.begin(true)) {
            ESP_LOGE(QUESTION_TAG, "Failed to mount LittleFS");
            xSemaphoreGive(questionMutex);
            return 0;
        }

        // Test chaque ID de 1 à 100
        for (int id = 1; id <= 100; id++) {
            vTaskDelay(pdMS_TO_TICKS(1));  // Évite le watchdog timer
            
            String dirPath = questionsPath + "/" + String(id);
            if (!LittleFS.exists(dirPath)) {
                ESP_LOGI(QUESTION_TAG, "Found free question ID: %d", id);
                return id;
            }
        }
        
        ESP_LOGW(QUESTION_TAG, "No free ID found between 1 and 100");
        return 1;  // Si aucun ID libre n'est trouvé, retourne 1

    } catch (...) {
        ESP_LOGE(QUESTION_TAG, "Exception in findFreeQuestion");
        return 0;
    }
}

String getQuestions() {
    struct DirectoryContent {
        String path;
        String questionJson;
    };

    String jsonOutput = "{";
    
    if(!LittleFS.begin(true)) {
        ESP_LOGE(QUESTION_TAG,"{\"error\": \"Failed to mount LittleFS\"}");
    }

    File root = LittleFS.open(questionsPath);
    if(!root || !root.isDirectory()) {
        ESP_LOGE(QUESTION_TAG, "{\"error\": \"Failed to open %s directory\"}",questionsPath.c_str());
    }

        // Première passe pour compter les répertoires
    int dirCount = 0;
    File countFile = root.openNextFile();
    while(countFile) {
        if(countFile.isDirectory()) {
            dirCount++;
        }
        countFile = root.openNextFile();
    }
    root.close();

    // Allouer le tableau avec la taille exacte
    DirectoryContent* directories = new DirectoryContent[dirCount];
    int currentIndex = 0;

    // Seconde passe pour remplir le tableau
    root = LittleFS.open(questionsPath);
    File file = root.openNextFile();
    while(file && currentIndex < dirCount) {
        if(file.isDirectory()) {
            directories[currentIndex].path = file.path();
            String questionPath = directories[currentIndex].path + "/question.json";
            File questionFile = LittleFS.open(questionPath, "r");
            
            if(questionFile) {
                directories[currentIndex].questionJson = questionFile.readString();
                questionFile.close();
                currentIndex++;
            }
        }
        file = root.openNextFile();
    }

    // Construire le JSON
    for(int i = 0; i < dirCount; i++) {
        if(i > 0) jsonOutput += ",";
        jsonOutput += "\"" + directories[i].path + "\":";
        jsonOutput += directories[i].questionJson;
    }
    jsonOutput += "}, \"FSINFO\": " + printLittleFSInfo(true) + "";
    
    // Libérer la mémoire
    delete[] directories;
    return jsonOutput;
}

void setCurrentQuestion(const String qID) {
    String question="";
    String qPath=questionsPath+"/"+qID+"/question.json";

    question=readFile(qPath);

    if (teamsAndBumpers["GAME"].isNull()) {
        teamsAndBumpers["GAME"] = JsonObject();
    }
    
    ESP_LOGD(QUESTION_TAG, "Set Question %s: %s", qID, question.c_str());
    
    DeserializationError error = deserializeJson(teamsAndBumpers["GAME"]["QUESTION"], question);
    if (error) {
        ESP_LOGE(QUESTION_TAG, "deserializeJson() failed: %s", error.c_str());
        teamsAndBumpers["GAME"].remove("QUESTION");
    } 
}

JsonObject getCurrentQuestion() {
    if (teamsAndBumpers["GAME"].isNull()) {
        teamsAndBumpers["GAME"] = JsonObject();
    }
    if (teamsAndBumpers["GAME"]["QUESTION"].isNull()) {
        teamsAndBumpers["GAME"]["QUESTION"] = JsonObject();
    }
    return teamsAndBumpers["GAME"]["QUESTION"];
}

String getQuestionElement(String Element) {
    return getCurrentQuestion()[Element];
}

String getQuestionElementJson() {
    String output="";

    JsonObject tb=getCurrentQuestion();
    if (serializeJson(tb, output)) {
        ESP_LOGI(QUESTION_TAG, "Question: %s", output.c_str());
        return output;
    } else {
        ESP_LOGE(QUESTION_TAG, "Failed to serialize JSON");
        return "";
    }
}
void writeQuestion(String id, String question) {

    ensureDirectoryExists(questionsPath);
    String fullPath = questionsPath + "/" + id;
    ensureDirectoryExists(fullPath);

    File jsonFile = LittleFS.open(fullPath + "/question.json", "w");
    if(jsonFile) {
        if(jsonFile.print(question)) {
            ESP_LOGI(QUESTION_TAG, "Fichier JSON créé avec succès dans %s", fullPath.c_str());
        } else {
            ESP_LOGE(QUESTION_TAG, "Erreur lors de l'écriture du JSON");
        }
        jsonFile.close();
    }
}

void setQuestionStatus(String status) {
    JsonObject tb=getCurrentQuestion();
    String output;
    tb["STATUS"]=status;
    if (serializeJson(tb, output)) {
        ESP_LOGI(QUESTION_TAG, "Question: %s", output.c_str());
        writeQuestion(tb["ID"],output);
    } else {
        ESP_LOGE(QUESTION_TAG, "Failed to serialize JSON");
    }
}

String getQuestionTime() {
    return getQuestionElement("TIME");
}

String getQuestionPoints() {
    return getQuestionElement("POINTS");
}

String getQuestionQuestion() {
    return getQuestionElement("QUESTION");
}

String getQuestionResponse() {
    return getQuestionElement("ANSWER");
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
    if (IP == nullptr || strlen(IP) == 0) {
        ESP_LOGE("Teams&Bumpers", "Invalid IP address provided: %s", IP);
    }

    ESP_LOGD("Teams&Bumpers","set IP: %s => %s", bumperID, IP);

    String bID = ensureBumperExists(bumperID);

    String ipCopy = String(IP);
    teamsAndBumpers["bumpers"][bID]["IP"] = ipCopy;
}

void setBumperNAME(const char* bumperID, const char* NAME) {
    if (NAME == nullptr || strlen(NAME) == 0) {
        ESP_LOGE(TEAMs_TAG, "Invalid NAME provided: %s", NAME);
    }
    ESP_LOGD(TEAMs_TAG,"set NAME: %s:%s", bumperID, NAME);

    String bID = ensureBumperExists(bumperID);

    String NAMECopy = String(NAME);
    teamsAndBumpers["bumpers"][bID]["NAME"] = NAMECopy;
    ESP_LOGI(TEAMs_TAG, "Bumper NAME %s => %s", bID.c_str(), NAMECopy.c_str());
}

void setBumperVERSION(const char* bumperID, const char* version) {
    if (version == nullptr || strlen(version) == 0) {
        ESP_LOGE("Teams&Bumpers", "Invalid VERSION address provided: %s", version);
    }

    ESP_LOGD("Teams&Bumpers","set VERSION: %s => %s", bumperID, version);

    String bID = ensureBumperExists(bumperID);

    String ipCopy = String(version);
    teamsAndBumpers["bumpers"][bID]["VERSION"] = ipCopy;
}

void setBumperButton(const char* bumperID, String button) {
    String bID = ensureBumperExists(bumperID);

    String copy = String (button);
    teamsAndBumpers["bumpers"][bID]["BUTTON"] = copy;
    ESP_LOGI(TEAMs_TAG, "Bumper Button %s %s", bID.c_str(), copy.c_str());
}

void setBumperStatus(const char* bumperID, String status) {
    String bID = ensureBumperExists(bumperID);

    String copy = String(status);
    teamsAndBumpers["bumpers"][bID]["STATUS"] = copy;
    ESP_LOGI(TEAMs_TAG, "Bumper Status %s %s", bID.c_str(), copy.c_str());
}

void setBumperScore(const char* bumperID, const int new_score) {
    String bID = ensureBumperExists(bumperID);

    int copy = int(new_score);
    teamsAndBumpers["bumpers"][bID]["SCORE"]=copy;
}

int  updateBumperScore(const char* bumperID, const int points) {
    String bID = ensureBumperExists(bumperID);
    int score=teamsAndBumpers["bumpers"][bID]["SCORE"];
    int newscore=score+points;
    ESP_LOGI(TEAMs_TAG, "Bumper update old Score %s %i+%i=%i", bID.c_str(), score, points, newscore);
    teamsAndBumpers["bumpers"][bID]["SCORE"]=newscore;
    return newscore;
}

const String getBumperTeam(const char* bumperID) {
    String bID = ensureBumperExists(bumperID);
    String tId=teamsAndBumpers["bumpers"][bID]["TEAM"];
    return tId;
}

const int64_t getBumperTime(const char* bumperID) {
    String bID = ensureBumperExists(bumperID);
    return int64_t(teamsAndBumpers["bumpers"][bID]["TIMESTAMP"]);
}

void setBumperTime(const char* bumperID, const int64_t new_delay) {
    String bID = ensureBumperExists(bumperID);

    int64_t copy = int64_t(new_delay);
    teamsAndBumpers["bumpers"][bID]["TIMESTAMP"]=copy;
    ESP_LOGI(TEAMs_TAG, "BumperID Delay %s %lld", bID.c_str(), copy);
}

void resetBumpersReady() {
  JsonObject bumpers = getBumpers();
  if (!bumpers.isNull()) {
    for (JsonPair bumperPair : bumpers) {
      JsonObject bumper = bumperPair.value().as<JsonObject>();
      bumper["READY"]="FALSE";
    }
  } else {
    ESP_LOGW(TEAMs_TAG, "Bumpers object is null or invalid");
  }  
  ESP_LOGI(TEAMs_TAG, "All bumpers marked as not ready");
}

void setBumperReady(const char* bumperID) {
  String bID = ensureBumperExists(bumperID);
  teamsAndBumpers["bumpers"][bID]["READY"]="TRUE";
  ESP_LOGI(TEAMs_TAG, "Bumper %s marked as ready", bID.c_str());
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
    String bID = ensureTeamExists(teamID);
    String copy = String(status);
    teamsAndBumpers["teams"][bID]["STATUS"] = copy;
    ESP_LOGI(TEAMs_TAG, "Team Status %s %s", bID, copy);
}

const int64_t getTeamTime(const char* teamID) {
    return int64_t(teamsAndBumpers["teams"][teamID]["TIMESTAMP"]);
}

void setTeamTime(const char* teamID, const int64_t new_delay) {
    String bID = ensureTeamExists(teamID);
    int64_t copy = int64_t(new_delay);
    teamsAndBumpers["teams"][bID]["TIMESTAMP"]=copy;
    ESP_LOGI(TEAMs_TAG, "Team Delay %s %i", bID, copy);
}

void setTeamBumper(const char* teamID, const char* bumperID) {
    String bID = ensureTeamExists(teamID);
    String copy = String(bumperID);
    teamsAndBumpers["teams"][bID]["BUMPER"] = copy;
    ESP_LOGI(TEAMs_TAG, "Team Bumper %s %s", teamID, copy.c_str());
}

void setTeamScore(const char* teamID, const int new_score) {
    String bID = ensureTeamExists(teamID);
    int copy = int(new_score);
    teamsAndBumpers["teams"][bID]["SCORE"] = copy;
    ESP_LOGI(TEAMs_TAG, "Team Score %s %i", bID, copy);
}






/*
void updateBumperScore(const char* bumperID, const int points) {
    String bID = ensureBumperExists(bumperID);
    int score=teamsAndBumpers["bumpers"][bID]["SCORE"];
    int newscore=score+points;
    ESP_LOGI(TEAMs_TAG, "Bumper update old Score %s %i+%i=%i", bID.c_str(), score, points, newscore);
    teamsAndBumpers["bumpers"][bID]["SCORE"]=newscore;
}

*/





int  updateBumperTeamScore(const char* bumperID, const int points) {
    ESP_LOGI(TEAMs_TAG, "team Bumper update old Score %s %i", bumperID, points);

    String bID=(teamsAndBumpers["bumpers"][String(bumperID)]["TEAM"]);
    int score=teamsAndBumpers["teams"][bID]["SCORE"];
    int newscore=score+points;
    ESP_LOGI(TEAMs_TAG, "Bumper Team update old Score %s %s %i+%i=%i",bumperID, bID.c_str(), score, points, newscore);

    teamsAndBumpers["teams"][bID]["SCORE"] = int(teamsAndBumpers["teams"][bID]["SCORE"])+points;
    return newscore;
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

void resetTeamsReady() {
JsonObject teams = getTeams();
  if (!teams.isNull()) {
    for (JsonPair teamPair : teams) {
      JsonObject team = teamPair.value().as<JsonObject>();
      team["READY"]="true";
    }
    ESP_LOGI(TEAMs_TAG, "All teams marked as initially ready");
  } else {
    ESP_LOGW(TEAMs_TAG, "Teams object is null or invalid");
  }
}

void updateTeamsReady() {
JsonObject team;
    resetTeamsReady() ;
    ESP_LOGD(TEAMs_TAG, "Updating team readiness status");

    JsonObject bumpers = getBumpers();
    if (!bumpers.isNull()) {
        for (JsonPair bumperPair : bumpers) {
            JsonObject bumper = bumperPair.value().as<JsonObject>();
            const char* teamID = bumper["TEAM"];
            if (teamID != nullptr && bumper["READY"] == "FALSE") {
                JsonObject  team=getTeam(bumper["TEAM"]);
                if (!team.isNull()) {
                    team["READY"]="FALSE";
                    ESP_LOGD(TEAMs_TAG, "Team %s marked as not ready due to bumper status", teamID);
                }
            }
        }
    } else {
        ESP_LOGW(TEAMs_TAG, "Bumpers object is null or invalid when updating team readiness");
    }
}

bool areAllTeamsReady() {
    JsonObject teams = getTeams();
    if (!teams.isNull()) {
        for (JsonPair teamPair : teams) {
            JsonObject team = teamPair.value().as<JsonObject>();
            if (team["READY"] == "FALSE") {
                return false;
            }
        }
        return true;
    }
    return false;
}
