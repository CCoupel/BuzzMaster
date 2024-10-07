#include <FS.h>
#include <LittleFS.h>
#include <WiFi.h>
#include <HTTPClient.h>
#include <esp_log.h>

static const char* FS_TAG = "FS_MANAGER";
#define VERSION_FILE "/config/version.txt"
#define TEMP_DIR "/temp_update"
#define BACKUP_DIR "/backup"

String baseURL="/config/base.url";
String baseFILE="/config/catalog.url";

void downloadFiles();
bool downloadFile(const String& url, const String& localPath);
bool createDirectories(const String& path);
void deleteDirectory(const char* dirPath);
bool moveDirectory(const String& sourceDir, const String& destDir);


bool compareVersions(const String& localVersion, const String& remoteVersion) {
    // Implémentez ici la logique de comparaison des versions
    // Retournez true si remoteVersion > localVersion
    return remoteVersion.compareTo(localVersion) > 0;
}

String readFile(const String& path, const String& defaultValue = "") {
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
void downloadFiles() {
    // Lire l'URL de base
    String baseUrl = readFile(baseURL);
    if (baseUrl.isEmpty()) {
        ESP_LOGE(FS_TAG, "Impossible de lire l'URL de base");
        return;
    }

    // Lire la version locale
    String localVersion = readFile(VERSION_FILE, "0");

    // Télécharger et lire la version distante
    String remoteVersionUrl = baseUrl + VERSION_FILE;
    String tempVersionPath = String(TEMP_DIR) + VERSION_FILE;
    if (!downloadFile(remoteVersionUrl, tempVersionPath)) {
        ESP_LOGE(FS_TAG, "Échec du téléchargement du fichier version distant");
        return;
    }

    String remoteVersion = readFile(tempVersionPath);
    if (remoteVersion.isEmpty()) {
        ESP_LOGE(FS_TAG, "Impossible de lire la version distante");
        return;
    }

    // Comparer les versions
    if (!compareVersions(localVersion, remoteVersion)) {
        ESP_LOGI(FS_TAG, "La version locale est à jour");
        LittleFS.remove(tempVersionPath);
        return;
    }
    
     // Télécharger le fichier catalogue distant
    String remoteCatalogUrl = baseUrl + baseFILE;
    String tempCatalogPath = String(TEMP_DIR) + baseFILE;
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
        int endPos = catalogContent.indexOf('\n', pos);
        if (endPos == -1) endPos = catalogContent.length();
        
        String filePath = catalogContent.substring(pos, endPos);
        filePath.trim();
        pos = endPos + 1;

        if (filePath.length() == 0) continue;

        String fileUrl = baseUrl + "/" + filePath;
        String tempFilePath = String(TEMP_DIR) + "/" + filePath;

        if (createDirectories(tempFilePath) && downloadFile(fileUrl, tempFilePath)) {
            ESP_LOGI(FS_TAG, "Téléchargé et sauvegardé %s", filePath.c_str());
        } else {
            ESP_LOGE(FS_TAG, "Échec du téléchargement ou de la création du répertoire pour %s", fileUrl.c_str());
            updateSuccess = false;
            break;
        }
    }

    if (updateSuccess) {
        // Créer un répertoire de sauvegarde
        if (!LittleFS.mkdir(BACKUP_DIR)) {
            ESP_LOGE(FS_TAG, "Échec de la création du répertoire de sauvegarde");
            updateSuccess = false;
        } else {
            // Déplacer les fichiers existants vers le répertoire de sauvegarde
            if (moveDirectory("/", BACKUP_DIR)) {
                // Déplacer les nouveaux fichiers du répertoire temporaire vers la racine
                if (moveDirectory(TEMP_DIR, "/")) {
                    // Mise à jour réussie, supprimer la sauvegarde
                    deleteDirectory(BACKUP_DIR);
                    ESP_LOGI(FS_TAG, "Mise à jour réussie vers la version %s", remoteVersion.c_str());
                } else {
                    // Échec du déplacement des nouveaux fichiers, restaurer la sauvegarde
                    moveDirectory(BACKUP_DIR, "/");
                    ESP_LOGE(FS_TAG, "Échec du déplacement des nouveaux fichiers, restauration de la sauvegarde");
                    updateSuccess = false;
                }
            } else {
                ESP_LOGE(FS_TAG, "Échec de la sauvegarde des fichiers existants");
                updateSuccess = false;
            }
        }
    }

    if (!updateSuccess) {
        // Nettoyer les fichiers téléchargés en cas d'échec
        deleteDirectory(TEMP_DIR);
        deleteDirectory(BACKUP_DIR);
        ESP_LOGE(FS_TAG, "Échec de la mise à jour, retour à la version locale");
    }
}
bool moveDirectory(const String& sourceDir, const String& destDir) {
    File source = LittleFS.open(sourceDir);
    if (!source || !source.isDirectory()) {
        return false;
    }

    if (!LittleFS.exists(destDir)) {
        if (!LittleFS.mkdir(destDir)) {
            return false;
        }
    }

    File file = source.openNextFile();
    while (file) {
        String sourceFilePath = String(sourceDir) + "/" + file.name();
        String destFilePath = String(destDir) + "/" + file.name();

        if (file.isDirectory()) {
            if (!moveDirectory(sourceFilePath, destFilePath)) {
                return false;
            }
        } else {
            if (!LittleFS.rename(sourceFilePath, destFilePath)) {
                return false;
            }
        }
        file = source.openNextFile();
    }

    return LittleFS.rmdir(sourceDir);
}

void deleteDirectory(const char* dirPath) {
    File dir = LittleFS.open(dirPath);
    if (!dir || !dir.isDirectory()) {
        return;
    }

    File file = dir.openNextFile();
    while (file) {
        if (file.isDirectory()) {
            deleteDirectory(file.path());
        } else {
            LittleFS.remove(file.path());
        }
        file = dir.openNextFile();
    }

    LittleFS.rmdir(dirPath);
}



void listLittleFSFiles() {
    ESP_LOGI(FS_TAG, "Listing files in LittleFS:");
    File root = LittleFS.open("/");
    if (!root) {
        ESP_LOGE(FS_TAG, "Failed to open directory");
        return;
    }
    if (!root.isDirectory()) {
        ESP_LOGE(FS_TAG, "Not a directory");
        return;
    }

    File file = root.openNextFile();
    while (file) {
        if (file.isDirectory()) {
            ESP_LOGI(FS_TAG, "DIR : %s", file.name());
        } else {
            ESP_LOGI(FS_TAG, "FILE: %s\tSIZE: %d", file.name(), file.size());
        }
        file = root.openNextFile();
    }
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
/*
void downloadFiles() {
    if (!LittleFS.begin()) {
        ESP_LOGE(FS_TAG, "Failed to mount LittleFS");
        return;
    }

    File baseFile = LittleFS.open(baseURL, "r");
    if (!baseFile) {
        ESP_LOGE(FS_TAG, "Failed to open base.url");
        return;
    }

    String baseUrl = baseFile.readString();
    baseUrl.trim();
    baseFile.close();

    File catalogFile = LittleFS.open(baseFILE, "r");
    if (!catalogFile) {
        ESP_LOGE(FS_TAG, "Failed to open catalog.url");
        return;
    }

    while (catalogFile.available()) {
        String filePath = catalogFile.readStringUntil('\n');
        filePath.trim();
        if (filePath.length() == 0) continue;

        String fileUrl = baseUrl + "/" + filePath;

        if (downloadFile(fileUrl, "/" + filePath)) {
            ESP_LOGI(FS_TAG, "Downloaded and saved %s", filePath.c_str());
        } else {
            ESP_LOGE(FS_TAG, "Failed to download %s", fileUrl.c_str());
        }
    }

    catalogFile.close();
}
*/
bool downloadFile(const String& fileUrl, const String& localPath) {
    HTTPClient http;
    http.begin(fileUrl);
    int httpCode = http.GET();

    if (httpCode == HTTP_CODE_OK) {
        WiFiClient * stream = http.getStreamPtr();

        File file = LittleFS.open(localPath, "w");
        if (!file) {
            ESP_LOGE(FS_TAG, "Failed to open file for writing");
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
        ESP_LOGI(FS_TAG, "File downloaded successfully");
    } else {
        ESP_LOGE(FS_TAG, "HTTP GET failed with code %d", httpCode);
        http.end();
        return false;
    }

    http.end();
    return true;
}

bool createDirectories(const String& path) {
    // Extraire le chemin du répertoire
    int lastSlash = path.lastIndexOf('/');
    if (lastSlash == -1) return true; // Pas de répertoire à créer

    String directoryPath = path.substring(0, lastSlash);
    
    // Si le répertoire existe déjà, rien à faire
    if (LittleFS.exists(directoryPath)) {
        return true;
    }

    String subPath = "";
    for (int i = 1; i < lastSlash; i++) {
        if (path[i] == '/') {
            if (!LittleFS.exists(subPath)) {
                if (!LittleFS.mkdir(subPath)) {
                    ESP_LOGE(FS_TAG, "HTTP GET failed with code %s", subPath);
                    return false;
                }
            }
        }
        subPath += path[i];
    }

    return true;
}

