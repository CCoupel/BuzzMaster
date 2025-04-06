#pragma once
#include "Common/CustomLogger.h"
#include "Common/led.h"

#include <FS.h>
#include <LittleFS.h>
#include <WiFi.h>
#include <HTTPClient.h>
//#include <esp_log.h>
#include "esp_littlefs.h"
#include <LittleFS.h>


static const char* FS_TAG = "FS_MANAGER";
String VERSION_FILE="/config/version.txt";
String TEMP_DIR="/temp_update";
String BACKUP_DIR="/backup";

String baseURL="/config/base.url";
String baseFILE="/config/catalog.url";

bool createDirectories(const String& path);

/***** FILES ********/
String readFile(const String& path, const String& defaultValue) {
    if (!LittleFS.begin()) {
        ESP_LOGE(FS_TAG, "Échec du montage de LittleFS");
        return defaultValue;
    }

    File file = LittleFS.open(path, "r");
    if (!file) {
        ESP_LOGE(FS_TAG, "Échec de l'ouverture du fichier %s", path.c_str());
        return defaultValue;
    }

    String content = file.readString();
    file.close();
    content.trim();
    return content;
}

bool deleteFile(const char* filePath) {
    if (!LittleFS.exists(filePath)) {
        ESP_LOGW(FS_TAG, "File does not exist: %s", filePath);
        return false;
    }
    
    if (LittleFS.remove(filePath)) {
        ESP_LOGI(FS_TAG, "File deleted: %s", filePath);
        return true;
    } else {
        ESP_LOGE(FS_TAG, "Failed to delete file: %s", filePath);
        return false;
    }
}

bool downloadFile(const String& fileUrl, const String& localPath) {
    HTTPClient http;
    http.begin(fileUrl);
    int httpCode = http.GET();

    createDirectories(localPath);

    if (httpCode == HTTP_CODE_OK) {
        WiFiClient * stream = http.getStreamPtr();

        File file = LittleFS.open(localPath, "w");
        if (!file) {
            ESP_LOGE(FS_TAG, "Failed to open file for writing %s", localPath.c_str());
            http.end();
            return false;
        }

        const size_t bufferSize = 512;
        uint8_t buffer[bufferSize];

        int bytesRead;
        while ((bytesRead = stream->read(buffer, bufferSize)) > 0) {
            file.write(buffer, bytesRead);
        }

        file.close();
        ESP_LOGI(FS_TAG, "File downloaded successfully %s to %s", fileUrl.c_str(), localPath.c_str());
    } else {
        ESP_LOGE(FS_TAG, "HTTP GET failed with code %d", httpCode);
        http.end();
        return false;
    }

    http.end();
    return true;
}

bool copyFile(const char* sourcePath, const char* destPath) {
    File sourceFile = LittleFS.open(sourcePath, "r");
    if (!sourceFile) {
        ESP_LOGE(FS_TAG, "Failed to open source file for reading %s", sourceFile);

        return false;
    }

    File destFile = LittleFS.open(destPath, "w");
    if (!destFile) {
        ESP_LOGE(FS_TAG, "Failed to open destination file for writing %s", destPath);
        sourceFile.close();
        return false;
    }

    static uint8_t buf[512];
    size_t len = 0;
    while ((len = sourceFile.read(buf, sizeof(buf))) > 0) {
        destFile.write(buf, len);
    }

    sourceFile.close();
    destFile.close();
    return true;
}

bool moveFile(const char* sourcePath, const char* destPath) {
    if (!LittleFS.exists(sourcePath)) {
        ESP_LOGE(FS_TAG, "Source file does not exist: %s", sourcePath);
        return false;
    }

    if (LittleFS.exists(destPath)) {
        ESP_LOGW(FS_TAG, "Destination file already exists, overwriting: %s", destPath);
        if (!deleteFile(destPath)) {
            ESP_LOGE(FS_TAG, "Failed to delete existing destination file: %s", destPath);
            return false;
        }
    }

    if (!copyFile(sourcePath, destPath)) {
        ESP_LOGE(FS_TAG, "Failed to copy file from %s to %s", sourcePath, destPath);
        return false;
    }

    if (!deleteFile(sourcePath)) {
        ESP_LOGE(FS_TAG, "Failed to delete source file after copy: %s", sourcePath);
        // Consider if you want to delete the destination file here in case of failure
        return false;
    }

    ESP_LOGI(FS_TAG, "File moved successfully from %s to %s", sourcePath, destPath);
    return true;
}

/***** DIR ********/

bool ensureDirectoryExists(const String& path) {
    if (LittleFS.exists(path)) {
        return true;  // Le répertoire existe déjà
    }
    ESP_LOGD(FS_TAG, "Creating Directory %s", path.c_str());
    return LittleFS.mkdir(path);
}

bool createDirectories(const String& path) {
    // Extraire le chemin du répertoire
    int lastSlash = path.lastIndexOf('/');
    if (lastSlash == -1) return true; // Pas de répertoire à créer

    String directoryPath = path.substring(0, lastSlash);
    ensureDirectoryExists(directoryPath);
    return true;
}

bool deleteDirectory(const char* dirPath) {
    std::vector<String> dirStack;
    dirStack.push_back(String(dirPath));

    while (!dirStack.empty()) {
        // Reset watchdog timer périodiquement
        esp_task_wdt_reset();
        
        // Ajouter un delay pour céder du temps CPU
        vTaskDelay(pdMS_TO_TICKS(1));
        String currentPath = dirStack.back();
        
        File dir = LittleFS.open(currentPath.c_str());
        if (!dir || !dir.isDirectory()) {
            ESP_LOGE(FS_TAG, "Failed to open directory: %s", currentPath.c_str());
            dirStack.pop_back();
            continue;
        }

        File file = dir.openNextFile();
        if (file) {
            char filePath[64];
            snprintf(filePath, sizeof(filePath), "%s/%s", currentPath.c_str(), file.name());

            if (file.isDirectory()) {
                dirStack.push_back(String(filePath));
            } else {
                file.close(); // Close the file before attempting to delete it
                if (LittleFS.remove(filePath)) {
                    ESP_LOGI(FS_TAG, "Deleted file: %s", filePath);
                } else {
                    ESP_LOGE(FS_TAG, "Failed to delete file: %s", filePath);
                    // Additional diagnostics
                    File testFile = LittleFS.open(filePath, "r");
                    if (testFile) {
                        ESP_LOGI(FS_TAG, "File can be opened for reading. Size: %d bytes", testFile.size());
                        testFile.close();
                    } else {
                        ESP_LOGE(FS_TAG, "File cannot be opened for reading");
                    }
                }
            }
        } else {
            dir.close();
            if (LittleFS.rmdir(currentPath.c_str())) {
                ESP_LOGI(FS_TAG, "Removed empty directory: %s", currentPath.c_str());
            } else {
                ESP_LOGE(FS_TAG, "Failed to remove directory: %s", currentPath.c_str());
            }
            dirStack.pop_back();
        }

        dir.close();
    }

    bool success = !LittleFS.exists(dirPath);
    ESP_LOGI(FS_TAG, "Directory deletion %s: %s", success ? "succeeded" : "failed", dirPath);
    return success;
}

bool moveDirectory(const char* sourceDir, const char* destDir) {
    std::vector<std::pair<String, String>> dirStack;
    dirStack.push_back({String(sourceDir), String(destDir)});

    while (!dirStack.empty()) {
        auto [currentSource, currentDest] = dirStack.back();
        dirStack.pop_back();

        if (!LittleFS.exists(currentSource.c_str())) {
            ESP_LOGE(FS_TAG, "Source directory does not exist: %s", currentSource.c_str());
            return false;
        }

        if (!ensureDirectoryExists(currentDest.c_str())) {
            ESP_LOGE(FS_TAG, "Failed to create destination directory: %s", currentDest.c_str());
            return false;
        }

        File source = LittleFS.open(currentSource.c_str());
        if (!source || !source.isDirectory()) {
            ESP_LOGE(FS_TAG, "Failed to open source directory: %s", currentSource.c_str());
            return false;
        }

        File file = source.openNextFile();
        while (file) {
            String sourceFilePath = currentSource + "/" + file.name();
            String destFilePath = currentDest + "/" + file.name();

            if (file.isDirectory()) {
                dirStack.push_back({sourceFilePath, destFilePath});
            } else {
                if (!copyFile(sourceFilePath.c_str(), destFilePath.c_str())) {
                    ESP_LOGE(FS_TAG, "Failed to move file: %s to %s", sourceFilePath.c_str(), destFilePath.c_str());
                    source.close();
                    return false;
                }
            }
            file = source.openNextFile();
        }

        source.close();
    }

    // Delete the original source directory
    if (!deleteDirectory(sourceDir)) {
        ESP_LOGE(FS_TAG, "Failed to remove source directory after move: %s", sourceDir);
        return false;
    }

    ESP_LOGI(FS_TAG, "Directory moved successfully from %s to %s", sourceDir, destDir);
    return true;
}

/******* TOOLS *******/

String printLittleFSInfo(bool isShort) {
    String result="";
    String line="";
    String s_result="{";

    if (!LittleFS.begin(true)) {
        ESP_LOGE(FS_TAG, "Failed to mount LittleFS");
        return result;
    }

    unsigned long totalBytes = LittleFS.totalBytes();
    unsigned long usedBytes = LittleFS.usedBytes();
    unsigned long freeBytes = totalBytes - usedBytes;

    float usedPercentage = (float)usedBytes / totalBytes * 100;
    float freePercentage = (float)freeBytes / totalBytes * 100;

    line="LittleFS Usage Information:";
    result += line+"\n";
    ESP_LOGI(FS_TAG, line.c_str());

    line="Total size: "+String(totalBytes/1024)+" Kbytes";
    
    result += line+"\n";
    ESP_LOGI(FS_TAG, line.c_str());
    //ESP_LOGI(FS_TAG, "Total size: %lu bytes", totalBytes);

    line="Used space: "+String(usedBytes/1024)+" Kbytes ("+usedPercentage+"%)";
    result += line+"\n";
    
    ESP_LOGI(FS_TAG, line.c_str());
    //ESP_LOGI(FS_TAG, "Used space: %lu bytes (%.2f%%)", usedBytes, usedPercentage);

    line="Free space: "+String(freeBytes/1024)+" Kbytes ("+String(freePercentage)+"%)";
    result += line+"\n";

    ESP_LOGI(FS_TAG, line.c_str());
    //ESP_LOGI(FS_TAG, "Free space: %lu bytes (%.2f%%)", freeBytes, freePercentage);

    s_result+="\"TOTAL\": "+String(totalBytes/1024)+", ";
    s_result+="\"USED\": "+String(usedBytes/1024)+", ";
    s_result+="\"FREE\": "+String(freeBytes/1024)+", ";
    s_result+="\"P_USED\": "+String(usedPercentage)+", ";
    s_result+="\"P_FREE\": "+String(freePercentage)+"}";

    if (isShort) {return s_result;}
    else {return result;}

}

String listLittleFSFilesRecursive(File &dir, const String &indent = "") {
    String result="";
    String line="";
    File file = dir.openNextFile();
    while (file) {
        // Reset watchdog timer périodiquement
        esp_task_wdt_reset();
        
        // Ajouter un delay pour céder du temps CPU
        vTaskDelay(pdMS_TO_TICKS(1));
        if (file.isDirectory()) {
            line=String(indent)+"L__"+String(file.name())+"/";
            //ESP_LOGI(FS_TAG, "%s├── %s/", indent.c_str(), file.name());
//            ESP_LOGI(FS_TAG, line.c_str());
            line+=listLittleFSFilesRecursive(file, indent + "    ");
        } else {
            line=String(indent)+"L__"+String(file.name())+" ("+String(file.size())+")";
//            ESP_LOGI(FS_TAG, line.c_str());
            //ESP_LOGI(FS_TAG, "%s├── %s (%d bytes)", indent.c_str(), file.name(), file.size());
        }
        
        result+="\n"+line;
        file = dir.openNextFile();
    }
    return result;
}

String listLittleFSFiles(String path) {
    String result="";
    ESP_LOGI(FS_TAG, "Listing files in LittleFS:");
    if (!LittleFS.begin()) {
        ESP_LOGE(FS_TAG, "Failed to mount LittleFS");
        return result;
    }

    File root = LittleFS.open(path);
    if (!root) {
        ESP_LOGE(FS_TAG, "Failed to open root directory");
        return result;
    }
    if (!root.isDirectory()) {
        ESP_LOGE(FS_TAG, "Root is not a directory");
        return result;
    }

    ESP_LOGI(FS_TAG, path.c_str());
    result=listLittleFSFilesRecursive(root);

//    result+=printLittleFSInfo();
    return result;
}

void loadJson(String path) {
    File file;
    String output;
    ESP_LOGI(FS_TAG, "Loading game file");
    
    if (LittleFS.exists(saveGameFile)) {
        file = LittleFS.open(saveGameFile, "r");
        ESP_LOGI(FS_TAG, "Loading from save file: %s", saveGameFile);
    } else if (LittleFS.exists(GameFile)) {
        file = LittleFS.open(GameFile, "r");
        ESP_LOGI(FS_TAG, "Loading from game file: %s", GameFile);
    }

    if (!file) {
        ESP_LOGE(FS_TAG, "Failed to open file for reading. Initializing with default values.");
        setBumpers(JsonObject());
        setTeams(JsonObject());
        return;
    }

    DeserializationError error = deserializeJson(teamsAndBumpers, file);
    if (error) {
        ESP_LOGE(FS_TAG, "deserializeJson() failed: %s", error.c_str());
        setBumpers( JsonObject());
        setTeams( JsonObject());
    } else {
        ESP_LOGI(FS_TAG, "JSON loaded successfully");
    }

    file.close();
    serializeJson(teamsAndBumpers, output);
    ESP_LOGI(FS_TAG, "JSON loaded: %s", output.c_str());
}

void saveJson() {
    File file = LittleFS.open(saveGameFile, "w");
    if (!file) {
        ESP_LOGE(FS_TAG, "Failed to open file for writing");
        return;
    }

    if (serializeJson(teamsAndBumpers, file) == 0) {
        ESP_LOGE(FS_TAG, "Failed to write to file");
    }

    file.close();
    ESP_LOGI(FS_TAG, "JSON saved successfully");
}

void downloadFiles() {
    // Lire l'URL de base
    String baseUrl = readFile(baseURL,"https://bitbucket.org/ccoupel/buzzcontrol/raw/main/data");
    if (baseUrl.isEmpty()) {
        ESP_LOGE(FS_TAG, "Impossible de lire l'URL de base");
        return;
    }

    // Lire la version locale
    float localVersion =readFile("/CURRENT"+VERSION_FILE,"-1").toFloat();
    ESP_LOGI(FS_TAG, "CURRENT Version=%f", localVersion);
    if (localVersion<0) {
        localVersion = readFile(VERSION_FILE, "-1").toFloat();
        ESP_LOGW(FS_TAG, "Local Version=%f", localVersion);
    }
    

    // Télécharger et lire la version distante
    String remoteVersionUrl = baseUrl + VERSION_FILE;
    String tempVersionPath = "/remote_version.txt";
    deleteDirectory(TEMP_DIR.c_str());

    ensureDirectoryExists(TEMP_DIR);
    if (!downloadFile(remoteVersionUrl, tempVersionPath)) {
        ESP_LOGE(FS_TAG, "Échec du téléchargement du fichier version distant");
        return;
    }

    float remoteVersion = atof(readFile(tempVersionPath,"-1").c_str());
    if (remoteVersion<0) {
        ESP_LOGE(FS_TAG, "Impossible de lire la version distante");
        return;
    }

    // Comparer les versions
    if (localVersion >= remoteVersion) {
        ESP_LOGI(FS_TAG, "La version locale est à jour");
        LittleFS.remove(tempVersionPath);
        return;
    }

    ESP_LOGI(FS_TAG, "La version locale est a remplacer %f / %f", localVersion, remoteVersion);
    setLedColor(255, 128, 0, true);
    // Télécharger le fichier catalogue distant
    String remoteCatalogUrl = baseUrl + baseFILE;
    String tempCatalogPath = baseFILE+"_remote";
    if (!downloadFile(remoteCatalogUrl, tempCatalogPath)) {
        ESP_LOGE(FS_TAG, "Échec du téléchargement du fichier catalogue distant");
        return;
    }

    String catalogContent = readFile(tempCatalogPath);
    if (catalogContent.isEmpty()) {
        ESP_LOGE(FS_TAG, "Impossible de lire le fichier catalogue");
        return;
    }

    bool updateSuccess = true;
    int pos = 0;
    while (pos < catalogContent.length()) {
        setLedColor(255*(catalogContent.length()-pos)/catalogContent.length(),128*(catalogContent.length()-pos)/catalogContent.length(),0,true);
        int endPos = catalogContent.indexOf('\n', pos);
        if (endPos == -1) endPos = catalogContent.length();
        
        String filePath = catalogContent.substring(pos, endPos);
        filePath.trim();
        pos = endPos + 1;

        if (filePath.length() == 0) continue;

        String fileUrl = baseUrl + "/" + filePath;
        String tempFilePath = TEMP_DIR + "/" + filePath;

        if (!downloadFile(fileUrl, tempFilePath)) {
            ESP_LOGE(FS_TAG, "Échec du téléchargement ou de la création du répertoire pour %s", fileUrl.c_str());
            updateSuccess = false;
            break;
        }
    }

    if (updateSuccess) { 
        deleteDirectory("/CURRENT");
        moveDirectory(TEMP_DIR.c_str(),"/CURRENT" );
    }
}

bool isFileExists(String Path) {
    return LittleFS.exists(Path);
}