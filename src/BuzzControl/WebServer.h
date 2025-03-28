#pragma once
#include "Common/CustomLogger.h"
#include "Common/led.h"
#include "fsManager.h"

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

void addCorsHeaders(AsyncWebServerResponse* response) {
    response->addHeader("Access-Control-Allow-Origin", "*");
    response->addHeader("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS");
    response->addHeader("Access-Control-Allow-Headers", "Content-Type,Authorization");
    response->addHeader("Access-Control-Allow-Credentials", "true");
}

void w_handleRoot(AsyncWebServerRequest *request) {
  ESP_LOGI(WEB_TAG, "Handling root request");
  AsyncWebServerResponse *response = request->beginResponse(200, "text/html", "hello from BuzzControl!");
    addCorsHeaders(response);

  request->send(response);

  digitalWrite(ledPin, LOW);
}

void w_handleNotFound(AsyncWebServerRequest *request) {
  ESP_LOGW(WEB_TAG, "Handling 404 for: %s", request->url().c_str());
  String message = "File Not Found\n\n";
  message += "URI: " + request->url() + "\n";
  message += "Method: " + String((request->method() == HTTP_GET) ? "GET" : "POST") + "\n";
  message += "Arguments: " + String(request->args()) + "\n";
  
  for (uint8_t i = 0; i < request->args(); i++) {
      message += " " + request->argName(i) + ": " + request->arg(i) + "\n";
  }
  
  AsyncWebServerResponse *response = request->beginResponse(404, "text/plain", message);
    addCorsHeaders(response);

  request->send(response);
}

//####### TOOLING ######
void w_handleRedirect(AsyncWebServerRequest *request) {
    ESP_LOGD(WEB_TAG, "REdirecting for: %s", request->url().c_str());
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
    rebootServer();
}

void w_handleListFiles(AsyncWebServerRequest *request) {
    String result="";
    result+=listLittleFSFiles();
    result+=printLittleFSInfo();
    
    AsyncWebServerResponse *response = request->beginResponse(200, "text/text", result);
    addCorsHeaders(response);
    request->send(response);
}

void w_handleListGame(AsyncWebServerRequest *request) {
    String result;
    result=getTeamsAndBumpersJSON();
    
    AsyncWebServerResponse *response = request->beginResponse(200, "text/text", result);
    addCorsHeaders(response);
    request->send(response);
}

// Fonction pour sauvegarder une question et retourner son ID
String saveQuestion(AsyncWebServerRequest *request, bool hasFile = false) {
    String currentDir;
    String jsonString;

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
    ensureDirectoryExists(questionsPath);
    String fullPath = questionsPath + "/" + currentDir;
    ensureDirectoryExists(fullPath);

    // Crée le JSON
    jsonString = "{\n";
    jsonString += "  \"ID\": \"" + currentDir + "\",\n";
    if (hasFile || isFileExists(fullPath + "/media.jpg")) {
        jsonString += "  \"MEDIA\": \"/question/" + currentDir + "/media.jpg\",\n";
    }
    jsonString += "  \"QUESTION\": \"" + questionText + "\",\n";
    jsonString += "  \"ANSWER\": \"" + reponseText + "\",\n";
    jsonString += "  \"POINTS\": " + pointsText + ",\n";
    jsonString += "  \"TIME\": " + tempsText + "\n";
    jsonString += "}";

    // Sauvegarde le fichier JSON
    File jsonFile = LittleFS.open(fullPath + "/question.json", "w");
    if(jsonFile) {
        if(jsonFile.print(jsonString)) {
            ESP_LOGI(WEB_TAG, "Fichier JSON créé avec succès dans %s", fullPath.c_str());
        } else {
            ESP_LOGE(WEB_TAG, "Erreur lors de l'écriture du JSON");
        }
        jsonFile.close();
    }
    
 //   putMsgToQueue("QUESTIONS",getQuestions().c_str());

    // Envoie la réponse au client
//    AsyncWebServerResponse *response = request->beginResponse(200, "text/json", jsonString);
//    addCorsHeaders(response);
//    request->send(response);

    return currentDir;
}




void w_handleUploadBackgroundComplete(AsyncWebServerRequest *request) {
    // Ne sauvegarde que s'il n'y a pas de fichier dans le formulaire
    ESP_LOGI(WEB_TAG,"upload du fichier w_handleUploadBackground Complete");
    AsyncWebServerResponse *response = request->beginResponse(200, "text/json", "Background Saved");
    addCorsHeaders(response);
    request->send(response);
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

void w_handleUploadFile(AsyncWebServerRequest *request, String filename, size_t index, uint8_t *data, size_t len, bool final) {
    saveFile(request, "/files/background.jpg",  filename,  index,  data,  len,  final);
}

/***** QUESTIONS *******/

void w_handleListQuestions(AsyncWebServerRequest *request) {
    ESP_LOGI(WEB_TAG, "Handling question list  request");
    String jsonOutput=getQuestions();
    
    ESP_LOGI(WEB_TAG, "Questions: %s", jsonOutput.c_str());

    AsyncWebServerResponse *response = request->beginResponse(200, "text/json", jsonOutput);
    addCorsHeaders(response);
    request->send(response);

}

void w_handleUploadQuestionComplete(AsyncWebServerRequest *request) {
    // Ne sauvegarde que s'il n'y a pas de fichier dans le formulaire
        ESP_LOGI(WEB_TAG,"upload du fichier w_handleUploadQuestion Complete");

    if (!request->hasParam("file", true, true)) {  // Le dernier true indique qu'on cherche un fichier
        ESP_LOGI(WEB_TAG,"Saving Question %s", saveQuestion(request, false).c_str());
    }
    AsyncWebServerResponse *response = request->beginResponse(200, "text/json", "Question Saved");
    addCorsHeaders(response);
    request->send(response);
    putMsgToQueue("QUESTIONS",getQuestions().c_str());
}

void w_handleUploadQuestionFile(AsyncWebServerRequest *request, String filename, size_t index, uint8_t *data, size_t len, bool final) {
    static File file;
    static size_t totalSize = 0;
    static String currentDir;
    static String fullPath;
    static String filePath;

    if(!index) { // Début de l'upload
       currentDir = saveQuestion(request, true); // Sauvegarde la question avec indicateur de fichier
        
        fullPath = questionsPath + "/" + currentDir;
        filePath = fullPath + "/media.jpg";
    }        

    totalSize=saveFile(request, filePath,  filename,  index,  data,  len,  final);
    if(final) { // Fin de l'upload
        ESP_LOGI(WEB_TAG, "Upload du fichier terminé pour la question: %s (%i)", currentDir.c_str(), totalSize);
        
    }
}

void _w_handleUploadQuestionFile(AsyncWebServerRequest *request, String filename, size_t index, uint8_t *data, size_t len, bool final) {
    static File file;
    static size_t totalSize = 0;
    static String currentDir;
    ESP_LOGI(WEB_TAG, "Chunk received %s: %i: %i Bytes (%i)", filename.c_str(), index, len,final);
    if(!index) { // Début de l'upload
    // Close any previously open file that wasn't properly closed
        if (file) {
            ESP_LOGW(WEB_TAG, "Closing previously unclosed file");
            file.close();
        }
        currentDir = saveQuestion(request, true); // Sauvegarde la question avec indicateur de fichier
        
        String fullPath = questionsPath + "/" + currentDir;
        String filePath = fullPath + "/media.jpg";
        
        
          file = LittleFS.open(filePath, "w"); // First chunk - create new file
          

        if(!file) {
            ESP_LOGE(WEB_TAG, "Échec de l'ouverture du fichier media en écriture");
            return;
        }
        ESP_LOGI(WEB_TAG, "Début de l'upload de l'image question id: %s", currentDir.c_str());
    }

        if(len>0) { // Écriture des données
            size_t bytesWritten=file.write(data, len);
//            size_t bytesWritten=len;
            totalSize+=bytesWritten;
            ESP_LOGI(WEB_TAG, "Writing image question id: %s (%i=>%i) %i bytes written %i", currentDir.c_str(),index, len,totalSize, bytesWritten);
            if(bytesWritten != len) {
                ESP_LOGW(WEB_TAG, "Partial write: %d of %d bytes written", bytesWritten, len);
            }
            
        }
        if(final) { // Fin de l'upload
            file.close();
            ESP_LOGI(WEB_TAG, "Upload du fichier terminé pour la question: %s (%i)", currentDir.c_str(), totalSize);
            putMsgToQueue("QUESTIONS",getQuestions().c_str());
        }
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
    server.onNotFound(w_handleNotFound);

    server.serveStatic("/jspa", LittleFS, (ROOT+"/jspa").c_str());
    server.serveStatic("/js", LittleFS, (ROOT+"/js").c_str());
    server.serveStatic("/css", LittleFS, (ROOT+"/css").c_str());
    server.serveStatic("/html", LittleFS, (ROOT+"/html").c_str());
    server.serveStatic("/config", LittleFS, (ROOT+"/config").c_str());

    // Servir les fichiers du répertoire files pour /files/*
    server.serveStatic("/files/", LittleFS, "/files/");
    
    // Servir les fichiers du répertoire files pour /files/*
    server.serveStatic("/question/", LittleFS, "/files/questions/");

    // Route spécifique pour le background
    server.serveStatic("/background", LittleFS, "/files/background.jpg");
    server.serveStatic("/version", LittleFS, "/config/version.txt");

    server.on("/", HTTP_GET, w_handleRedirect);
    server.on("/index.html", w_handleRedirect);

    server.on("/generate_204", w_handleRedirect);  // Android captive portal
    server.on("/gen_204", w_handleRedirect);       // Android captive portal
    server.on("/hotspot-detect.html", w_handleRedirect);  // iOS captive portal
    server.on("/canonical.html", w_handleRedirect);       // Windows captive portal
    server.on("/success.txt", w_handleRedirect);          // macOS captive portal
    server.on("/favicon.ico", w_handleRedirect);   // Browser icon

    server.on("/reset", HTTP_GET, w_handleReset);
    server.on("/reboot", HTTP_GET, w_handleReboot);
    server.on("/listFiles",HTTP_GET, w_handleListFiles);
    server.on("/listGame",HTTP_GET, w_handleListGame);

    server.on("/background", HTTP_POST, w_handleUploadBackgroundComplete, w_handleUploadFile);

    server.on("/questions", HTTP_POST, w_handleUploadQuestionComplete, w_handleUploadQuestionFile);
    server.on("/questions", HTTP_GET, w_handleListQuestions);

    ws.onEvent(onWsEvent);
    server.addHandler(&ws);

    server.begin();
    ESP_LOGI(WEB_TAG, "HTTP server started on %s", ROOT);
}