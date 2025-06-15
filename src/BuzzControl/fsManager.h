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

#define DEST_FS_USES_LITTLEFS
#include <ESP32-targz.h>

static const char* FS_TAG = "FS_MANAGER";
String VERSION_FILE="/config/version.txt";
String TEMP_DIR="/temp_update";
//String BACKUP_DIR="/backup";

String baseURL="/config/base.url";
String baseFILE="/config/catalog.url";

bool createDirectories(const String& path);

// BACKUP //
// Classe Stream simple pour recevoir les données TAR d'ESP32-targz
class SimpleStreamBuffer : public Stream {
private:
    uint8_t* buffer;
    size_t bufferSize;
    size_t dataSize;
    size_t readPos;
    
public:
    SimpleStreamBuffer(size_t size = 16384) : bufferSize(size), dataSize(0), readPos(0) {
        buffer = (uint8_t*)malloc(size);
    }
    
    ~SimpleStreamBuffer() {
        if (buffer) free(buffer);
    }
    
    bool isValid() { return buffer != nullptr; }
    
    // Interface Stream pour ESP32-targz (écriture)
    size_t write(uint8_t data) override {
        if (dataSize < bufferSize) {
            buffer[dataSize++] = data;
            return 1;
        }
        return 0;
    }
    
    size_t write(const uint8_t *data, size_t len) override {
        size_t available = bufferSize - dataSize;
        size_t toWrite = min(len, available);
        if (toWrite > 0) {
            memcpy(buffer + dataSize, data, toWrite);
            dataSize += toWrite;
        }
        return toWrite;
    }
    
    // Interface Stream (pas utilisé ici)
    int available() override { return dataSize - readPos; }
    int read() override { return (readPos < dataSize) ? buffer[readPos++] : -1; }
    int peek() override { return (readPos < dataSize) ? buffer[readPos] : -1; }
    void flush() override {}
    
    // Méthodes pour récupérer les données
    size_t getDataSize() { return dataSize; }
    uint8_t* getData() { return buffer; }
    
    void reset() {
        dataSize = 0;
        readPos = 0;
    }
};

// Variables globales pour le backup
static SimpleStreamBuffer* tarBuffer = nullptr;
static bool tarReady = false;
static size_t tarSentBytes = 0;

// Génère le nom de fichier de backup
String generateBackupFilename() {
    time_t now = time(nullptr);
    struct tm* timeinfo = localtime(&now);
    
    char filename[64];
    strftime(filename, sizeof(filename), "buzzcontrol_backup_%Y%m%d_%H%M%S.tar", timeinfo);
    
    return String(filename);
}

// Initialise le backup TAR
bool initTarBackup() {
    // Nettoyer toute session précédente
    if (tarBuffer) {
        delete tarBuffer;
        tarBuffer = nullptr;
    }
    
    tarReady = false;
    tarSentBytes = 0;
    
    // Vérifier que le répertoire /files existe
    if (!LittleFS.exists("/files")) {
        ESP_LOGW(FS_TAG, "Répertoire /files non trouvé");
        return false;
    }
    
    // Reset watchdog avant opération longue
    yield();
    
    // Créer le buffer pour recevoir le TAR
    tarBuffer = new SimpleStreamBuffer(32768); // 32KB buffer
    if (!tarBuffer || !tarBuffer->isValid()) {
        ESP_LOGE(FS_TAG, "Échec allocation buffer TAR");
        if (tarBuffer) {
            delete tarBuffer;
            tarBuffer = nullptr;
        }
        return false;
    }
    
    // Reset watchdog avant collecte
    yield();
    
    // Collecter les entités du répertoire /files
    std::vector<TAR::dir_entity_t> entities;
    TarPacker::collectDirEntities(&entities, &LittleFS, "/files");
    
    if (entities.empty()) {
        ESP_LOGW(FS_TAG, "Aucun fichier trouvé dans /files");
        delete tarBuffer;
        tarBuffer = nullptr;
        return false;
    }
    
    ESP_LOGI(FS_TAG, "Collecté %d entités depuis /files", entities.size());
    
    // Reset watchdog avant génération TAR (opération la plus longue)
    yield();
    
    // Générer le TAR en une seule fois
    size_t tarSize = TarPacker::pack_files(&LittleFS, entities, tarBuffer);
    
    // Reset watchdog après génération
    yield();
    
    if (tarSize > 0) {
        ESP_LOGI(FS_TAG, "TAR généré avec succès: %zu bytes", tarSize);
        tarReady = true;
        return true;
    } else {
        ESP_LOGE(FS_TAG, "Erreur lors de la génération du TAR");
        delete tarBuffer;
        tarBuffer = nullptr;
        return false;
    }
}

// Récupère le prochain chunk du TAR
size_t getTarChunk(uint8_t* output, size_t maxLen) {
    if (!tarBuffer || !tarReady) {
        return 0;
    }
    
    // Reset watchdog pendant l'envoi (opération potentiellement longue)
    yield();
    
    size_t totalSize = tarBuffer->getDataSize();
    size_t remaining = totalSize - tarSentBytes;
    
    if (remaining == 0) {
        // Terminé
        return 0;
    }
    
    size_t toSend = min(remaining, maxLen);
    memcpy(output, tarBuffer->getData() + tarSentBytes, toSend);
    tarSentBytes += toSend;
    
    if (tarSentBytes >= totalSize) {
        ESP_LOGI(FS_TAG, "Backup TAR envoyé complètement: %zu bytes", tarSentBytes);
    }
    
    return toSend;
}

// Nettoie le backup TAR
void cleanupTarBackup() {
    if (tarBuffer) {
        delete tarBuffer;
        tarBuffer = nullptr;
    }
    tarReady = false;
    tarSentBytes = 0;
    ESP_LOGI(FS_TAG, "Backup TAR nettoyé");
}

// RESTAURE //

// ========== PARTIE RESTORE TAR ==========
// À ajouter dans fsManager.h après les fonctions de backup

// Classe pour traiter les données TAR en streaming lors du restore
// Classe Stream pour recevoir les données TAR depuis HTTP en streaming
class HttpTarStream : public Stream {
private:
    uint8_t* buffer;
    size_t bufferSize;
    size_t writePos;
    size_t readPos;
    size_t totalReceived; // Nouvelle variable pour tracer la taille totale
    bool streamComplete;
    
public:
    HttpTarStream(size_t size = 8192) : 
        bufferSize(size), writePos(0), readPos(0), totalReceived(0), streamComplete(false) {
        buffer = (uint8_t*)malloc(size);
    }
    
    ~HttpTarStream() {
        if (buffer) free(buffer);
    }
    
    bool isValid() { return buffer != nullptr; }
    
    // Méthodes pour recevoir les données HTTP
    size_t receiveData(const uint8_t* data, size_t len) {
        if (!buffer || streamComplete) return 0;
        
        size_t space = bufferSize - writePos;
        size_t toWrite = min(len, space);
        
        if (toWrite > 0) {
            memcpy(buffer + writePos, data, toWrite);
            writePos += toWrite;
            totalReceived += toWrite; // Mettre à jour la taille totale
        }
        
        yield(); // Céder la main pendant la réception
        return toWrite;
    }
    
    void markComplete() {
        streamComplete = true;
    }
    
    bool isComplete() {
        return streamComplete && (readPos >= writePos);
    }
    
    // Nouvelle méthode pour obtenir la taille totale reçue
    size_t getTotalSize() {
        return totalReceived;
    }
    
    // Interface Stream pour ESP32-targz (lecture)
    int available() override {
        return writePos - readPos;
    }
    
    int read() override {
        if (readPos < writePos) {
            return buffer[readPos++];
        }
        return -1;
    }
    
    int peek() override {
        if (readPos < writePos) {
            return buffer[readPos];
        }
        return -1;
    }
    
    void flush() override {
        // Compacter le buffer : déplacer les données non lues au début
        if (readPos > 0 && readPos < writePos) {
            memmove(buffer, buffer + readPos, writePos - readPos);
            writePos -= readPos;
            readPos = 0;
        }
    }
    
    // Interface Stream pour écriture (pas utilisé)
    size_t write(uint8_t) override { return 0; }
    size_t write(const uint8_t*, size_t) override { return 0; }
    
    // Méthodes utilitaires
    bool hasSpace() {
        return writePos < bufferSize;
    }
    
    size_t getFreeSpace() {
        return bufferSize - writePos;
    }
    
    void reset() {
        writePos = 0;
        readPos = 0;
        totalReceived = 0;
        streamComplete = false;
    }
};

// Variables globales pour le restore
static HttpTarStream* tarStream = nullptr;
static TarUnpacker* tarUnpacker = nullptr;
static bool restoreInProgress = false;
static bool restoreSuccess = false;
static bool tarGzFSInitialized = false;

// Déclaration anticipée
void cleanupTarRestore();

// Initialise tarGzFS (requis par ESP32-targz)
bool initTarGzFS() {
    if (tarGzFSInitialized) {
        return true;
    }
    
    // tarGzFS est un alias automatique créé par ESP32-targz quand DEST_FS_USES_LITTLEFS est défini
    // Il pointe vers LittleFS
    if (!tarGzFS.begin()) {
        ESP_LOGE(FS_TAG, "Échec initialisation tarGzFS");
        return false;
    }
    
    tarGzFSInitialized = true;
    ESP_LOGI(FS_TAG, "tarGzFS initialisé");
    return true;
}

// Initialise le restore TAR streaming
bool initTarRestore() {
    if (restoreInProgress) {
        ESP_LOGW(FS_TAG, "Un restore est déjà en cours");
        return false;
    }
    
    // Nettoyer toute session précédente
    cleanupTarRestore();
    
    yield();
    
    // Initialiser tarGzFS (OBLIGATOIRE)
    if (!initTarGzFS()) {
        ESP_LOGE(FS_TAG, "Impossible d'initialiser tarGzFS");
        return false;
    }
    
    // Créer le stream pour recevoir les données HTTP
    tarStream = new HttpTarStream();
    if (!tarStream || !tarStream->isValid()) {
        ESP_LOGE(FS_TAG, "Échec allocation HttpTarStream");
        cleanupTarRestore();
        return false;
    }
    
    // Créer le TarUnpacker d'ESP32-targz
    tarUnpacker = new TarUnpacker();
    if (!tarUnpacker) {
        ESP_LOGE(FS_TAG, "Échec création TarUnpacker");
        cleanupTarRestore();
        return false;
    }
    
    // Configuration du unpacker (basée sur l'exemple)
    tarUnpacker->haltOnError(false); // Continuer en cas d'erreur mineure
    tarUnpacker->setTarVerify(false); // Pas de vérification pour économiser la RAM
    
    // Setup callbacks comme dans l'exemple
    tarUnpacker->setupFSCallbacks(targzTotalBytesFn, targzFreeBytesFn);
    
    // Callbacks de progression
    tarUnpacker->setTarProgressCallback([](uint8_t progress) {
        static uint8_t lastProgress = 255;
        if (progress != lastProgress && progress % 20 == 0) {
            ESP_LOGI(FS_TAG, "Restore progress: %d%%", progress);
            lastProgress = progress;
        }
    });
    
    tarUnpacker->setTarStatusProgressCallback([](const char* name, size_t size, size_t total_unpacked) {
        ESP_LOGI(FS_TAG, "Extrait: %s (%zu bytes)", name, size);
    });
    
    tarUnpacker->setTarMessageCallback(BaseUnpacker::targzPrintLoggerCallback);
    
    // S'assurer que le répertoire de destination existe
    ensureDirectoryExists("/files");
    
    restoreInProgress = true;
    restoreSuccess = false;
    
    ESP_LOGI(FS_TAG, "Restore TAR streaming initialisé");
    return true;
}

// Traite un chunk de données TAR reçues depuis HTTP
size_t processTarChunk(const uint8_t* data, size_t len) {
    if (!tarStream || !restoreInProgress) {
        ESP_LOGE(FS_TAG, "Restore non initialisé");
        return 0;
    }
    
    yield();
    
    // Recevoir les données dans le stream
    size_t received = tarStream->receiveData(data, len);
    
    if (received != len) {
        ESP_LOGW(FS_TAG, "Chunk partiellement reçu: %zu/%zu bytes", received, len);
    }
    
    return received;
}

// Finalise le restore en traitant toutes les données
bool finalizeTarRestore() {
    if (!tarStream || !tarUnpacker || !restoreInProgress) {
        ESP_LOGW(FS_TAG, "Restore non initialisé pour finalisation");
        return false;
    }
    
    yield();
    
    // Marquer le stream comme complet
    tarStream->markComplete();
    
    ESP_LOGI(FS_TAG, "Début extraction TAR streaming...");
    
    // Obtenir la taille totale du TAR reçu
    size_t streamSize = tarStream->getTotalSize();
    if (streamSize == 0) {
        ESP_LOGW(FS_TAG, "Aucune donnée TAR reçue");
        restoreInProgress = false;
        return false;
    }
    
    ESP_LOGI(FS_TAG, "Taille totale du TAR reçu: %zu bytes", streamSize);
    
    // Utiliser la signature correcte de l'exemple Unpack_tar_stream.ino
    // Format : tarStreamExpander(Stream* stream, size_t streamSize, fs::FS& destinationFS, const char* destinationPath)
    bool success = tarUnpacker->tarStreamExpander(tarStream, streamSize, tarGzFS, "/");
    
    if (success) {
        ESP_LOGI(FS_TAG, "Restore TAR terminé avec succès");
        restoreSuccess = true;
    } else {
        int errorCode = tarUnpacker->tarGzGetError();
        ESP_LOGE(FS_TAG, "Erreur restore TAR, code: %d", errorCode);
        restoreSuccess = false;
    }
    
    // Nettoyer
    restoreInProgress = false;
    
    return success;
}

// Nettoie le restore TAR
void cleanupTarRestore() {
    if (tarUnpacker) {
        delete tarUnpacker;
        tarUnpacker = nullptr;
    }
    
    if (tarStream) {
        delete tarStream;
        tarStream = nullptr;
    }
    
    restoreInProgress = false;
    ESP_LOGI(FS_TAG, "Restore TAR nettoyé");
}

// Vérifie si un restore est en cours
bool isRestoreInProgress() {
    return restoreInProgress;
}

// Obtient le status du restore
String getRestoreStatus() {
    if (!restoreInProgress) {
        if (restoreSuccess) {
            return "Restore terminé avec succès";
        } else {
            return "Aucun restore en cours";
        }
    }
    
    return "Restore en cours...";
}






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
        esp_task_wdt_reset();
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
            line+=listLittleFSFilesRecursive(file, indent + "    ");
        } else {
            line=String(indent)+"L__"+String(file.name())+" ("+String(file.size())+")";
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
        ESP_LOGI(FS_TAG, "La version locale est à jour %f / %f", localVersion, remoteVersion);
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
        esp_task_wdt_reset();
        setLedColor(0,128*(pos)/catalogContent.length(),255*(pos)/catalogContent.length(),true);
        int endPos = catalogContent.indexOf('\n', pos);
        if (endPos == -1) endPos = catalogContent.length();
        
        String filePath = catalogContent.substring(pos, endPos);
        filePath.trim();
        pos = endPos + 1;

        if (filePath.length() == 0) continue;

        String fileUrl = baseUrl + "/" + filePath;
        String tempFilePath = TEMP_DIR + "/" + filePath;
        setLedColor(0,0,0,true);

        if (!downloadFile(fileUrl, tempFilePath)) {
            ESP_LOGE(FS_TAG, "Échec du téléchargement ou de la création du répertoire pour %s", fileUrl.c_str());
            updateSuccess = false;
            break;
        }
    }
    setLedColor(128,255,0,true);

    if (updateSuccess) { 
        deleteDirectory("/CURRENT");
        moveDirectory(TEMP_DIR.c_str(),"/CURRENT" );
    }
}

bool isFileExists(String Path) {
    return LittleFS.exists(Path);
}
