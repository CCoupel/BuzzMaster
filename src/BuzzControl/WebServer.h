#pragma once
#include "Common/CustomLogger.h"
#include "Common/led.h"
#include "fsManager.h"
#include "messages_to_send.h"
#include "common/configManager.h"

#include <ESPAsyncWebServer.h>

static const char* WEB_TAG = "WEBSERVER";

// Stockage temporaire des fichiers uploadés en attente d'un ID
struct TempUpload {
    std::vector<uint8_t> data;
    uint32_t timestamp;
};

static std::map<String, TempUpload> pendingUploads;
static SemaphoreHandle_t uploadMutex = NULL;

// Fonction utilitaire pour générer un ID unique de requête
String generateRequestId(AsyncWebServerRequest *request) {
    String clientIP = request->client()->remoteIP().toString();
    uint32_t timestamp = millis();
    return clientIP + "_" + String(timestamp);
}

// Nettoie les uploads temporaires trop vieux (plus de 5 minutes)
void cleanupOldUploads() {
    uint32_t currentTime = millis();
    for (auto it = pendingUploads.begin(); it != pendingUploads.end();) {
        if (currentTime - it->second.timestamp > 300000) {
            ESP_LOGW(WEB_TAG, "Suppression d'un upload temporaire non réclamé: %s", it->first.c_str());
            it = pendingUploads.erase(it);
        } else {
            ++it;
        }
    }
}

void w_handleRoot(AsyncWebServerRequest *request) {
  ESP_LOGI(WEB_TAG, "Handling root request");
  request->send(200, "text/plain", "hello from BuzzControl!");
  digitalWrite(ledPin, LOW);
}

void handleWindowsConnectTest(AsyncWebServerRequest *request) {
    ESP_LOGI(WEB_TAG, "Windows url test request");
    AsyncWebServerResponse *response = request->beginResponse(200, "text/plain", "Microsoft NCSI");
    response->addHeader("Connection","close");
    response->addHeader("Cache-Control","no-cache, no-store");
    response->addHeader("Pragma","no-cache");

    request->send(response);
}

void w_handleNotFound(AsyncWebServerRequest *request) {
  String host=request->host();
  
  ESP_LOGW(WEB_TAG, "Handling 404 for: %s://%s", host.c_str(),request->url().c_str());
  if (host.equals("www.msftncsi.com") || host.equals("www.msftconnecttest.com")) {
    ESP_LOGI(WEB_TAG, "Windows host %s test request", host.c_str());
    handleWindowsConnectTest(request);
  }
  else {
    String message = "File Not Found\n\n";
    message += "URI: " + request->url() + "\n";
    message += "Method: " + String((request->method() == HTTP_GET) ? "GET" : "POST") + "\n";
    message += "Arguments: " + String(request->args()) + "\n";
    
    for (uint8_t i = 0; i < request->args(); i++) {
        message += " " + request->argName(i) + ": " + request->arg(i) + "\n";
    }
    
    AsyncWebServerResponse *response = request->beginResponse(404, "text/plain", message);

    request->send(response);
  }
}


//####### TOOLING ######
void w_handleRedirect(AsyncWebServerRequest *request) {
    ESP_LOGI(WEB_TAG, "Handling redirect request:%s",(request->url()).c_str());
    request->redirect("http://buzzcontrol.local/html/testSPA.html#config");
}

void w_handleReboot(AsyncWebServerRequest *request) {
    ESP_LOGI(WEB_TAG, "Handling reboot request");
    w_handleRedirect(request);
    rebootServer();
}

void w_handleReset(AsyncWebServerRequest *request) {
    ESP_LOGI(WEB_TAG, "Handling reset request");
    w_handleRedirect(request);
    resetServer();
}

void w_handleListFiles(AsyncWebServerRequest *request) {
    String result="";
    result+=listLittleFSFiles();
    result+=printLittleFSInfo();
    request->send(200, "text/plain", result);
}

void w_handleUpdate(AsyncWebServerRequest *request) {
    String result="";
    ESP_LOGI(WEB_TAG,"Updating Web pages from GIT");
    downloadFiles();
    result+=listLittleFSFiles();
    request->send(200, "text/plain", result);
}

void w_handleListGame(AsyncWebServerRequest *request) {
    String result;
    result=getTeamsAndBumpersJSON();
    request->send(200, "text/plain", result);
}

size_t saveFile(AsyncWebServerRequest *request, String destFile, String filename, size_t index, uint8_t *data, size_t len, bool final) {
    static File file;
    static size_t totalSize = 0;
    ESP_LOGI(WEB_TAG,"upload du fichier %s %i %s (%i) [%i]", destFile.c_str(), index, filename.c_str(), len,final);

    if(!index) { // Début de l'upload
        ESP_LOGI(WEB_TAG,"Début de l'upload du %s %i %s (%i) [%i]", destFile.c_str(),index, filename.c_str(), len,final);
        file = LittleFS.open(destFile, "w");
        totalSize=0;
        if(!file) {
            ESP_LOGI(WEB_TAG,"Échec de l'ouverture du fichier %s en écriture",destFile.c_str());
            return totalSize;
        }
    }

    if(file && len) { // Écriture des données
        file.write(data, len);
        totalSize+=len;
    }

    if(final) { // Fin de l'upload
        if(file) {
            ESP_LOGD(WEB_TAG,"Upload %s terminé: %i", destFile.c_str(),totalSize);
            file.close();
        }
    }
    return totalSize;
}

void w_handleUploadBackgroundComplete(AsyncWebServerRequest *request) {
    // Ne sauvegarde que s'il n'y a pas de fichier dans le formulaire
    ESP_LOGI(WEB_TAG,"upload du fichier Backgroud Complete");
    request->send(200, "text/json", "Background Saved");
}


void w_handleUploadBackgroundFile(AsyncWebServerRequest *request, String filename, size_t index, uint8_t *data, size_t len, bool final) {
    static String filePath;
    static String fileName;
    if(!index) { // Début de l'upload
        
        fileName="background_"+String(random(1000, 9999))+".jpg";
        filePath = "/files/" + fileName;
    }     
    saveFile(request, filePath,  filename,  index,  data,  len,  final);
    if(final) { // Fin de l'upload
        ESP_LOGI(WEB_TAG, "Upload du fichier Config terminé");
        setBackgroundFile(filePath);
        enqueueOutgoingMessage("UPDATE", getGameJSON().c_str(), false, nullptr);
    }
}

void w_handleUploadConfigComplete(AsyncWebServerRequest *request) {
    // Ne sauvegarde que s'il n'y a pas de fichier dans le formulaire
    ESP_LOGI(WEB_TAG,"upload Config Complete");
    request->send(200, "text/json", "Config Saved");
}

void w_handleUploadConfigBody(AsyncWebServerRequest *request, uint8_t *data, size_t len, size_t index, size_t total) {    
        ESP_LOGI(WEB_TAG, "Upload du fichier Config terminé: %i:%s", len, (char*)data);
        saveFile(request, "/files/config.json.current",  "",  index,  data,  len,  true);
        configManager.load();
}

void w_handleConfigBody(AsyncWebServerRequest *request) {
    String result;
    result=configManager.getConfigJSON() ;
    request->send(200, "text/plain", result);
}

/***** QUESTIONS *******/
// Fonction pour sauvegarder une question et retourner son ID
String saveQuestion(AsyncWebServerRequest *request, String fileName = "") {
    String currentDir;
    String jsonString;

    ESP_LOGI(WEB_TAG, "saveQuestions with fileName=%s", fileName.c_str());
    // Détermine l'ID de la question
    if(request->hasParam("number", true)) {
        currentDir = request->getParam("number", true)->value();
        if (currentDir.isEmpty()) {
            currentDir = findFreeQuestion();
        }
    } else {
        currentDir = findFreeQuestion();
    }

    // Récupère les paramètres du formulaire
    String questionText = request->hasParam("question", true) ? request->getParam("question", true)->value() : "";
    String reponseText = request->hasParam("answer", true) ? request->getParam("answer", true)->value() : "";
    String pointsText = request->hasParam("points", true) ? request->getParam("points", true)->value() : "0";
    String tempsText = request->hasParam("time", true) ? request->getParam("time", true)->value() : "0";
//    ensureDirectoryExists(questionsPath);
    String fullPath = questionsPath + "/" + currentDir;
//    ensureDirectoryExists(fullPath);

    // Crée le JSON
    jsonString = "{\n";
    jsonString += "  \"ID\": \"" + currentDir + "\",\n";
    if (fileName.length()>0 && isFileExists(fullPath + "/" + fileName)) {
        ESP_LOGI(WEB_TAG, "saveQuestions adding filename");
        jsonString += "  \"MEDIA\": \"/question/" + currentDir + "/" + fileName +"\",\n";
    }
    jsonString += "  \"QUESTION\": \"" + questionText + "\",\n";
    jsonString += "  \"ANSWER\": \"" + reponseText + "\",\n";
    jsonString += "  \"POINTS\": " + pointsText + ",\n";
    jsonString += "  \"TIME\": " + tempsText + "\n";
    jsonString += "}";

    // Sauvegarde le fichier JSON
    /*
    File jsonFile = LittleFS.open(fullPath + "/question.json", "w");
    if(jsonFile) {
        if(jsonFile.print(jsonString)) {
            ESP_LOGI(WEB_TAG, "Fichier JSON créé avec succès dans %s", fullPath.c_str());
        } else {
            ESP_LOGE(WEB_TAG, "Erreur lors de l'écriture du JSON");
        }
        jsonFile.close();
    }
*/
    writeQuestion(currentDir,jsonString);
    return currentDir;
}

void w_handleListQuestions(AsyncWebServerRequest *request) {
    ESP_LOGI(WEB_TAG, "Handling question list  request");
    String jsonOutput=getQuestions();
    
    ESP_LOGI(WEB_TAG, "Questions: %s", jsonOutput.c_str());

    AsyncWebServerResponse *response = request->beginResponse(200, "text/json", jsonOutput);
    request->send(response);
}

void w_handleUploadQuestionComplete(AsyncWebServerRequest *request) {
    // Ne sauvegarde que s'il n'y a pas de fichier dans le formulaire
    ESP_LOGI(WEB_TAG,"upload Question Complete");

    if (!request->hasParam("file", true, true)) {  // Le dernier true indique qu'on cherche un fichier
        ESP_LOGI(WEB_TAG,"Saving Question %s", saveQuestion(request, "").c_str());
    }
    request->send(200, "text/json", "Question Saved");
    enqueueOutgoingMessage("QUESTIONS", getQuestions().c_str(), false, nullptr);
}

void w_handleUploadQuestionFile(AsyncWebServerRequest *request, String filename, size_t index, uint8_t *data, size_t len, bool final) {
    static File file;
    static size_t totalSize = 0;
    static String currentDir;
    static String fullPath;
    static String filePath;
    static String fileName;

    

    if(!index) { // Début de l'upload
        currentDir = saveQuestion(request, "" ); // Sauvegarde la question avec indicateur de fichier
        fullPath = questionsPath + "/" + currentDir;
        fileName="media_"+String(random(1000, 9999))+".jpg";
        filePath = fullPath + "/" + fileName;
    }        
    
    totalSize=saveFile(request, filePath,  filename,  index,  data,  len,  final);
    if(final) { // Fin de l'upload
        ESP_LOGI(WEB_TAG, "Upload du fichier Question terminé pour la question: %s (%i)", currentDir.c_str(), totalSize);
        currentDir = saveQuestion(request, fileName ); // Sauvegarde la question avec indicateur de fichier

    }
}

void setGlobalWebRoute(AsyncWebServer& server, const char* uri, const char* path, const char* fileType) {
        // Pour servir le même fichier pour toute URL commençant par /background
    server.on(uri, HTTP_GET, [path, fileType](AsyncWebServerRequest *request){
        AsyncWebServerResponse *response = request->beginResponse(LittleFS, path, fileType);
        request->send(response);
    });

    // Si vous voulez aussi capturer les sous-chemins (ex: /background/something)
    String wildcardUri = String(uri) + "/*";
    server.on(wildcardUri.c_str(), HTTP_GET, [path, fileType](AsyncWebServerRequest *request){
        AsyncWebServerResponse *response = request->beginResponse(LittleFS, path, fileType);
        request->send(response);
    });
}

void startWebServer() {
    String ROOT="/";
    if (LittleFS.exists("/CURRENT/html/testSPA.html")) {
        ESP_LOGD(FS_TAG, "Directory /CURRENT Exists");
        ROOT="/CURRENT";
    }
    else {
        ESP_LOGE(FS_TAG, "Directory /CURRENT do not contains /CURRENT/html/testSPA.html : fall back to local version");
    }

    DefaultHeaders::Instance().addHeader("Access-Control-Allow-Origin", "*");
    DefaultHeaders::Instance().addHeader("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS");
    DefaultHeaders::Instance().addHeader("Access-Control-Allow-Headers", "Content-Type,Authorization");
    DefaultHeaders::Instance().addHeader("Access-Control-Allow-Credentials", "true");

    server.onNotFound(w_handleNotFound);
    
    server.on("/*", HTTP_OPTIONS, [](AsyncWebServerRequest *request) {
        ESP_LOGD(FS_TAG, "Default response: %s", (request->url()).c_str());
      AsyncWebServerResponse *response = request->beginResponse(200);
      request->send(response);
    });

    server.serveStatic("/jspa", LittleFS, (ROOT+"/jspa").c_str());
    server.serveStatic("/js", LittleFS, (ROOT+"/js").c_str());
    server.serveStatic("/css", LittleFS, (ROOT+"/css").c_str());
    server.serveStatic("/html", LittleFS, (ROOT+"/html").c_str());
    server.serveStatic("/config", LittleFS,  (ROOT+"/config").c_str());

    // Servir les fichiers du répertoire files pour /files/*
    server.serveStatic("/files/", LittleFS, "/files/");
    
    // Servir les fichiers du répertoire files pour /files/*
    server.serveStatic("/question/", LittleFS, "/files/questions/");

    server.on("/connecttest.txt", HTTP_GET, handleWindowsConnectTest);
    server.on("/ncsi.txt", HTTP_GET, handleWindowsConnectTest);

    server.on("/", HTTP_GET, w_handleRedirect);
    server.on("/redirect", HTTP_GET, w_handleRedirect);
    server.on("/index.html", w_handleRedirect);

    server.on("/reset", HTTP_GET, w_handleReset);
    server.on("/reboot", HTTP_GET, w_handleReboot);
    server.on("/update", HTTP_GET, w_handleUpdate);
    server.on("/listFiles",HTTP_GET, w_handleListFiles);
    server.on("/listGame",HTTP_GET, w_handleListGame);

    server.on("/version", HTTP_GET, [](AsyncWebServerRequest *request){request->send(200,"text/plain",VERSION);});
    server.on("/background", HTTP_POST, w_handleUploadBackgroundComplete, w_handleUploadBackgroundFile);

    server.on("/config.json", HTTP_GET, w_handleConfigBody);
    server.on("/config.json", HTTP_POST, w_handleUploadConfigComplete, NULL, w_handleUploadConfigBody);

    server.on("/questions", HTTP_POST, w_handleUploadQuestionComplete, w_handleUploadQuestionFile);
    server.on("/questions", HTTP_GET, w_handleListQuestions);

    ws.onEvent(onWsEvent);
    server.addHandler(&ws);

    server.begin();
    ESP_LOGI(WEB_TAG, "HTTP server started on %s", ROOT);
}