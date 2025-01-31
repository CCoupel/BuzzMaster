#pragma once
#include "Common/CustomLogger.h"
#include "Common/led.h"
#include "fsManager.h"

#include <ESPAsyncWebServer.h>

static const char* WEB_TAG = "WEBSERVER";

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
    request->redirect("http://buzzcontrol.local/config.html");
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
    String result;
    result=listLittleFSFiles();
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

void w_handleUploadComplete(AsyncWebServerRequest *request) {
    static File file;
    static size_t totalSize = 0;
    static String currentDir;
    static String questionText;
    static String reponseText;
    static String pointsText;
    static String tempsText;
    static bool hasFile = false;  // Pour suivre si un fichier est en cours d'upload
    static String baseDir="/files/questions/";
    String fullPath;
    static String jsonString="Upload Terminé";

    ESP_LOGI(WEB_TAG,"receive a question ?");
    if(request->hasParam("number", true)) {
        currentDir = request->getParam("number", true)->value();
        questionText = request->hasParam("question", true) ? request->getParam("question", true)->value() : "";
        reponseText = request->hasParam("answer", true) ? request->getParam("answer", true)->value() : "";
        pointsText = request->hasParam("points", true) ? request->getParam("points", true)->value() : "0";
        tempsText = request->hasParam("time", true) ? request->getParam("time", true)->value() : "0";
        
        // Créer le répertoire si nécessaire
        ensureDirectoryExists(fullPath);
        fullPath =  baseDir + currentDir;
        totalSize=0;
        ensureDirectoryExists(fullPath);
        // Créer le fichier JSON
        jsonString = "{\n";
            jsonString += "  \"ID\": \"" + currentDir + "\",\n";
            if (isFileExists(fullPath + "/media.jpg")) {
                jsonString += "  \"MEDIA\": \"/question/" + currentDir + "/media.jpg\",\n";
            }
            jsonString += "  \"QUESTION\": \"" + questionText + "\",\n";
            jsonString += "  \"ANSWER\": \"" + reponseText + "\",\n";
            jsonString += "  \"POINTS\": " + pointsText + ",\n";
            jsonString += "  \"TIME\": " + tempsText + "\n";
            jsonString += "}";

        ESP_LOGD(WEB_TAG, "Question received: %s", jsonString.c_str());
    
        File jsonFile = LittleFS.open(fullPath + "/question.json", "w");
        if(jsonFile) {
            if(jsonFile.print(jsonString)) {
                ESP_LOGI(WEB_TAG, "Fichier JSON créé avec succèsi dans %s", fullPath.c_str());
            } else {
                ESP_LOGE(WEB_TAG, "Erreur lors de l'écriture du JSON");
            }
            jsonFile.close();
        }
        putMsgToQueue("QUESTION",jsonString.c_str());
    }

    AsyncWebServerResponse *response = request->beginResponse(200, "text/json", jsonString);
    addCorsHeaders(response);
    request->send(response);
}

void w_handleUploadFile(AsyncWebServerRequest *request, String filename, size_t index, uint8_t *data, size_t len, bool final) {
    static File file;
    static size_t totalSize = 0;

    if(!index) { // Début de l'upload
        ESP_LOGI(WEB_TAG,"Début de l'upload du Background %i %s (%i)", index, filename.c_str(), len);
        file = LittleFS.open("/files/background.jpg", "w");
        totalSize=0;
        if(!file) {
            ESP_LOGI(WEB_TAG,"Échec de l'ouverture du fichier background en écriture");
            return;
        }
    }

    if(file && len) { // Écriture des données
        file.write(data, len);
        totalSize+=len;
    }

    if(final) { // Fin de l'upload
        if(file) {
            ESP_LOGD(WEB_TAG,"Upload background terminé: %i", totalSize);
            file.close();
        }
    }
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

void w_handleUploadQuestionFile(AsyncWebServerRequest *request, String filename, size_t index, uint8_t *data, size_t len, bool final) {
    static File file;
    static size_t totalSize = 0;
    static String currentDir;
    static String questionText;
    static String reponseText;
    static String pointsText;
    static String tempsText;
    static bool hasFile = false;  // Pour suivre si un fichier est en cours d'upload
    String fullPath;
    static String jsonString;
        
    hasFile = (filename != "");  // Vérifie si un fichier est présent

    if(!index) { // Début de l'upload
        if(request->hasParam("number", true)) {
            ensureDirectoryExists(fullPath);
            currentDir = request->getParam("number", true)->value();
            ESP_LOGI(WEB_TAG,"Début de l'upload de l'image question id : %s", currentDir);
            fullPath+="/"+currentDir;
            ensureDirectoryExists(fullPath);
            String filePath = fullPath + "/media.jpg";
            ESP_LOGI(WEB_TAG, "Début de l'upload de l'image vers: %s", filePath.c_str());
            
            totalSize = 0;
            file = LittleFS.open(filePath, "w");

            if(!file) {
                ESP_LOGI(WEB_TAG,"Échec de l'ouverture du fichier background en écriture");
                return;
            }
        }
    }    
    if(hasFile && file && len) { // Écriture des données
        file.write(data, len);
        totalSize+=len;
    }

    if(final) { // Fin de l'upload
        ESP_LOGD(WEB_TAG,"Upload media terminé: %i", totalSize);

        if(file) {
            file.close();
        }
    }
}

void startWebServer() {
    String ROOT="/";
    if (LittleFS.exists("/CURRENT/html/config.html")) {
        ESP_LOGD(FS_TAG, "Directory /CURRENT Exists");
        ROOT="/CURRENT";
    }
    server.onNotFound(w_handleNotFound);

    server.serveStatic("/", LittleFS, (ROOT+"/html").c_str());
    server.serveStatic("/js", LittleFS, (ROOT+"/js").c_str());
    server.serveStatic("/css", LittleFS, (ROOT+"/css").c_str());
    server.serveStatic("/html", LittleFS, (ROOT+"/html").c_str());

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

    server.on("/background", HTTP_POST, w_handleUploadComplete, w_handleUploadFile);

    server.on("/questions", HTTP_POST, w_handleUploadComplete, w_handleUploadQuestionFile);
    server.on("/questions", HTTP_GET, w_handleListQuestions);

    ws.onEvent(onWsEvent);
    server.addHandler(&ws);

    server.begin();
    ESP_LOGI(WEB_TAG, "HTTP server started on %s", ROOT);
}