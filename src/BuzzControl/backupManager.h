#pragma once
#include "Common/CustomLogger.h"
#include "Common/led.h"

#include <FS.h>
#include <LittleFS.h>
#include <WiFi.h>
#include <HTTPClient.h>
#include "esp_littlefs.h"
#include <LittleFS.h>

#define DEST_FS_USES_LITTLEFS
#include <ESP32-targz.h>
#include <vector>
#include <ArduinoJson.h>

static const char* BACKUP_TAG = "BACKUP_MANAGER";

// Constantes pour les chemins (redéfinies localement pour éviter les problèmes de liaison)
static const char* BACKUP_GAME_FILE = "/files/game.json.save";
static const String BACKUP_QUESTIONS_PATH = "/files/questions";

// Structure pour stocker les informations des fichiers à sauvegarder
struct BackupFileInfo {
    String originalPath;
    String archivePath;
    size_t size;
    bool isDirectory;
};

// SAUVEGARDE SELECTIVE DE JEU //
class GameBackupBuffer : public Stream {
private:
    std::vector<BackupFileInfo> gameFiles;
    size_t currentFileIndex;
    size_t currentFileOffset;
    File currentFile;
    bool headerSent;
    size_t totalBytesGenerated;
    size_t pendingPadding;
    size_t endBlocksSent;
    
    // Buffer temporaire pour les headers TAR
    uint8_t headerBuffer[512];
    size_t headerPos;
    
public:
    GameBackupBuffer() : currentFileIndex(0), currentFileOffset(0), 
                         headerSent(false), totalBytesGenerated(0), headerPos(0),
                         pendingPadding(0), endBlocksSent(0) {}
    
    ~GameBackupBuffer() {
        if (currentFile) currentFile.close();
    }
    
    bool initialize() {
        gameFiles.clear();
        currentFileIndex = 0;
        currentFileOffset = 0;
        headerSent = false;
        totalBytesGenerated = 0;
        headerPos = 0;
        pendingPadding = 0;
        endBlocksSent = 0;
        
        if (currentFile) {
            currentFile.close();
        }
        
        yield(); // Reset watchdog
        
        // Collecter les fichiers de jeu à sauvegarder
        if (!collectGameFiles()) {
            ESP_LOGW(BACKUP_TAG, "Aucun fichier de jeu trouvé pour la sauvegarde");
            return false;
        }
        
        ESP_LOGI(BACKUP_TAG, "Collecté %zu fichiers de jeu pour la sauvegarde", gameFiles.size());
        for (const auto& file : gameFiles) {
            ESP_LOGD(BACKUP_TAG, "  - %s -> %s (%s)", 
                    file.originalPath.c_str(), 
                    file.archivePath.c_str(), 
                    file.isDirectory ? "DIR" : "FILE");
        }
        return true;
    }
    
    // Méthodes Stream non utilisées mais nécessaires
    size_t write(uint8_t data) override { return 0; }
    size_t write(const uint8_t *data, size_t len) override { return 0; }
    int peek() override { return -1; }
    void flush() override {}
    
    int available() override {
        return (currentFileIndex < gameFiles.size() || pendingPadding > 0 || endBlocksSent < 1024) ? 1 : 0;
    }
    
    int read() override {
        uint8_t byte;
        return (readBytes(&byte, 1) == 1) ? byte : -1;
    }
    
    size_t readBytes(uint8_t* buffer, size_t length) {
        size_t bytesRead = 0;
        
        while (bytesRead < length) {
            yield(); // Reset watchdog périodiquement
            
            // Gérer le padding en attente d'abord
            if (pendingPadding > 0) {
                size_t paddingToAdd = min(pendingPadding, length - bytesRead);
                memset(buffer + bytesRead, 0, paddingToAdd);
                bytesRead += paddingToAdd;
                pendingPadding -= paddingToAdd;
                totalBytesGenerated += paddingToAdd;
                continue;
            }
            
            // Si on a fini tous les fichiers, envoyer les blocs de fin
            if (currentFileIndex >= gameFiles.size()) {
                if (endBlocksSent < 1024) {
                    size_t endBytesToSend = min((size_t)(1024 - endBlocksSent), length - bytesRead);
                    memset(buffer + bytesRead, 0, endBytesToSend);
                    bytesRead += endBytesToSend;
                    endBlocksSent += endBytesToSend;
                    totalBytesGenerated += endBytesToSend;
                    continue;
                } else {
                    break; // Vraiment fini
                }
            }
            
            // Si on n'a pas encore envoyé le header pour le fichier courant
            if (!headerSent) {
                size_t headerBytesAvailable = 512 - headerPos;
                if (headerBytesAvailable > 0) {
                    if (headerPos == 0) {
                        generateTarHeader(gameFiles[currentFileIndex]);
                    }
                    
                    size_t toCopy = min(headerBytesAvailable, length - bytesRead);
                    memcpy(buffer + bytesRead, headerBuffer + headerPos, toCopy);
                    headerPos += toCopy;
                    bytesRead += toCopy;
                    totalBytesGenerated += toCopy;
                    
                    if (headerPos >= 512) {
                        headerSent = true;
                        headerPos = 0;
                        
                        // Ouvrir le fichier s'il ne s'agit pas d'un répertoire
                        if (!gameFiles[currentFileIndex].isDirectory) {
                            currentFile = LittleFS.open(gameFiles[currentFileIndex].originalPath.c_str(), "r");
                            if (!currentFile) {
                                ESP_LOGE(BACKUP_TAG, "Impossible d'ouvrir le fichier: %s", 
                                        gameFiles[currentFileIndex].originalPath.c_str());
                                nextFile();
                                continue;
                            }
                        } else {
                            // Pour un répertoire, passer directement au suivant
                            nextFile();
                            continue;
                        }
                    }
                    continue;
                }
            }
            
            // Lire les données du fichier (seulement si ce n'est pas un répertoire)
            if (currentFile && currentFile.available()) {
                size_t maxRead = min((size_t)currentFile.available(), length - bytesRead);
                size_t actualRead = currentFile.read(buffer + bytesRead, maxRead);
                bytesRead += actualRead;
                currentFileOffset += actualRead;
                totalBytesGenerated += actualRead;
                
                // Vérifier si on a fini de lire le fichier
                if (!currentFile.available()) {
                    currentFile.close();
                    
                    // Calculer le padding nécessaire pour aligner sur 512 bytes
                    pendingPadding = (512 - (currentFileOffset % 512)) % 512;
                    
                    // Si on peut ajouter du padding maintenant, le faire
                    if (pendingPadding > 0 && bytesRead < length) {
                        size_t paddingToAdd = min(pendingPadding, length - bytesRead);
                        memset(buffer + bytesRead, 0, paddingToAdd);
                        bytesRead += paddingToAdd;
                        pendingPadding -= paddingToAdd;
                        totalBytesGenerated += paddingToAdd;
                    }
                    
                    // Passer au fichier suivant seulement si on a fini le padding
                    if (pendingPadding == 0) {
                        nextFile();
                    }
                }
            } else {
                // Pas de fichier ouvert ou plus de données - passer au suivant
                nextFile();
            }
        }
        
        return bytesRead;
    }
    
private:
    void nextFile() {
        currentFileIndex++;
        currentFileOffset = 0;
        headerSent = false;
        headerPos = 0;
        pendingPadding = 0;
        if (currentFile) {
            currentFile.close();
        }
    }
    
    void generateTarHeader(const BackupFileInfo& file) {
        memset(headerBuffer, 0, 512);
        
        ESP_LOGD(BACKUP_TAG, "Génération header TAR: %s -> %s", 
                file.originalPath.c_str(), file.archivePath.c_str());
        
        strncpy((char*)headerBuffer, file.archivePath.c_str(), 100);
        
        // Mode (permissions)
        sprintf((char*)headerBuffer + 100, "%07o", file.isDirectory ? 0755 : 0644);
        
        // UID/GID
        sprintf((char*)headerBuffer + 108, "%07o", 0);
        sprintf((char*)headerBuffer + 116, "%07o", 0);
        
        // Taille
        if (!file.isDirectory) {
            sprintf((char*)headerBuffer + 124, "%011lo", (unsigned long)file.size);
        } else {
            sprintf((char*)headerBuffer + 124, "%011o", 0);
        }
        
        // Timestamp
        sprintf((char*)headerBuffer + 136, "%011lo", (unsigned long)time(nullptr));
        
        // Type de fichier
        headerBuffer[156] = file.isDirectory ? '5' : '0';
        
        // Magic number
        strcpy((char*)headerBuffer + 257, "ustar");
        headerBuffer[263] = '0';
        headerBuffer[264] = '0';
        
        // Calculer et définir le checksum
        unsigned int checksum = 0;
        for (int i = 0; i < 512; i++) {
            if (i >= 148 && i < 156) {
                checksum += ' ';
            } else {
                checksum += headerBuffer[i];
            }
        }
        sprintf((char*)headerBuffer + 148, "%06o", checksum);
        headerBuffer[154] = 0;
        headerBuffer[155] = ' ';
    }
    
    // Collecter les fichiers spécifiques au jeu à sauvegarder
    bool collectGameFiles() {
        // 1. Lire la configuration du jeu pour obtenir le fichier background
        String backgroundFile = getGameBackgroundFile();
        
        // 2. Ajouter le fichier background s'il existe
        if (!backgroundFile.isEmpty() && LittleFS.exists(backgroundFile)) {
            addFileToBackup(backgroundFile, extractFilename(backgroundFile));
            ESP_LOGI(BACKUP_TAG, "Background ajouté: %s", backgroundFile.c_str());
        }
        
        // 3. Parcourir toutes les questions pour collecter les médias
        collectQuestionMediaFiles();
        
        // 4. Ajouter le fichier de configuration du jeu
        if (LittleFS.exists(BACKUP_GAME_FILE)) {
            addFileToBackup(BACKUP_GAME_FILE, "game.json.save");
            ESP_LOGI(BACKUP_TAG, "Configuration de jeu ajoutée: %s", BACKUP_GAME_FILE);
        }
        
        return !gameFiles.empty();
    }
    
    // Obtenir le fichier background depuis la configuration
    String getGameBackgroundFile() {
        if (!LittleFS.exists(BACKUP_GAME_FILE)) {
            ESP_LOGW(BACKUP_TAG, "Fichier de configuration de jeu non trouvé: %s", BACKUP_GAME_FILE);
            return "";
        }
        
        File file = LittleFS.open(BACKUP_GAME_FILE, "r");
        if (!file) {
            ESP_LOGE(BACKUP_TAG, "Impossible d'ouvrir le fichier de configuration: %s", BACKUP_GAME_FILE);
            return "";
        }
        
        JsonDocument doc;
        DeserializationError error = deserializeJson(doc, file);
        file.close();
        
        if (error) {
            ESP_LOGE(BACKUP_TAG, "Erreur de désérialisation JSON: %s", error.c_str());
            return "";
        }
        
        if (doc["GAME"]["background"].isNull()) {
            ESP_LOGD(BACKUP_TAG, "Aucun fichier background spécifié dans la configuration");
            return "";
        }
        
        return doc["GAME"]["background"].as<String>();
    }
    
    // Collecter les médias associés aux questions
    void collectQuestionMediaFiles() {
        if (!LittleFS.exists(BACKUP_QUESTIONS_PATH)) {
            ESP_LOGD(BACKUP_TAG, "Dossier des questions non trouvé: %s", BACKUP_QUESTIONS_PATH.c_str());
            return;
        }
        
        File questionsDir = LittleFS.open(BACKUP_QUESTIONS_PATH);
        if (!questionsDir || !questionsDir.isDirectory()) {
            ESP_LOGE(BACKUP_TAG, "Impossible d'ouvrir le dossier des questions");
            return;
        }
        
        // Parcourir chaque dossier de question
        File questionDir = questionsDir.openNextFile();
        while (questionDir) {
            yield(); // Reset watchdog
            
            if (questionDir.isDirectory()) {
                String questionId = questionDir.name();
                String questionPath = BACKUP_QUESTIONS_PATH + "/" + questionId;
                
                ESP_LOGD(BACKUP_TAG, "Traitement question: %s", questionId.c_str());
                
                // Toujours inclure le fichier question.json
                String questionJsonPath = questionPath + "/question.json";
                if (LittleFS.exists(questionJsonPath)) {
                    addFileToBackup(questionJsonPath, "questions/" + questionId + "/question.json");
                }
                
                // Lire le fichier question.json pour obtenir les médias
                collectMediaFromQuestion(questionPath, questionId);
            }
            
            questionDir = questionsDir.openNextFile();
        }
        questionsDir.close();
    }
    
    // Collecter les médias d'une question spécifique
    void collectMediaFromQuestion(const String& questionPath, const String& questionId) {
        String questionJsonPath = questionPath + "/question.json";
        if (!LittleFS.exists(questionJsonPath)) {
            return;
        }
        
        File file = LittleFS.open(questionJsonPath, "r");
        if (!file) {
            ESP_LOGE(BACKUP_TAG, "Impossible d'ouvrir: %s", questionJsonPath.c_str());
            return;
        }
        
        JsonDocument doc;
        DeserializationError error = deserializeJson(doc, file);
        file.close();
        
        if (error) {
            ESP_LOGE(BACKUP_TAG, "Erreur JSON dans %s: %s", questionJsonPath.c_str(), error.c_str());
            return;
        }
        
        // Vérifier si le champ "MEDIA" existe et n'est pas nul
        if (!doc["MEDIA"].isNull()) {
            String mediaPath = doc["MEDIA"].as<String>();
            
            if (!mediaPath.isEmpty()) {
                // Si le chemin média est relatif, le construire depuis le dossier de la question
                String archivePath = "/questions/" + questionId + "/" + extractFilename(mediaPath);
                if (LittleFS.exists("/files"+archivePath)) {
                    
                    addFileToBackup("/files"+archivePath, archivePath);
                    ESP_LOGI(BACKUP_TAG, "Média ajouté pour question %s: %s ", 
                            questionId.c_str(),  archivePath.c_str());
                } else {
                    ESP_LOGW(BACKUP_TAG, "Fichier média non trouvé pour question %s: %s ", 
                            questionId.c_str(),  archivePath.c_str());
                }
            }
        }
    }
    
    // Ajouter un fichier à la liste de sauvegarde
    void addFileToBackup(const String& originalPath, const String& archivePath) {
        if (!LittleFS.exists(originalPath)) {
            ESP_LOGW(BACKUP_TAG, "Fichier non trouvé: %s", originalPath.c_str());
            return;
        }
        
        File file = LittleFS.open(originalPath);
        if (!file) {
            ESP_LOGE(BACKUP_TAG, "Impossible d'ouvrir: %s", originalPath.c_str());
            return;
        }
        
        BackupFileInfo backupFile;
        backupFile.originalPath = originalPath;
        backupFile.archivePath = archivePath;
        backupFile.size = file.size();
        backupFile.isDirectory = file.isDirectory();
        
        file.close();
        
        gameFiles.push_back(backupFile);
        ESP_LOGD(BACKUP_TAG, "Fichier ajouté: %s (%zu bytes)", originalPath.c_str(), backupFile.size);
    }
    
    // Extraire le nom de fichier d'un chemin
    String extractFilename(const String& path) {
        int lastSlash = path.lastIndexOf('/');
        if (lastSlash >= 0) {
            return path.substring(lastSlash + 1);
        }
        return path;
    }
    
public:
    size_t getTotalBytesGenerated() { return totalBytesGenerated; }
    bool isFinished() { 
        return currentFileIndex >= gameFiles.size() && 
               pendingPadding == 0 && 
               endBlocksSent >= 1024; 
    }
    
    size_t getFileCount() const { return gameFiles.size(); }
};

// Variables globales pour le backup de jeu streaming
static GameBackupBuffer* gameBackupBuffer = nullptr;
static bool gameBackupReady = false;

// Initialise le backup de jeu TAR streaming
bool initGameBackup() {
    // Nettoyer toute session précédente
    if (gameBackupBuffer) {
        delete gameBackupBuffer;
        gameBackupBuffer = nullptr;
    }
    
    gameBackupReady = false;
    
    yield(); // Reset watchdog
    
    // Créer le buffer streaming pour la sauvegarde de jeu
    gameBackupBuffer = new GameBackupBuffer();
    if (!gameBackupBuffer) {
        ESP_LOGE(BACKUP_TAG, "Échec allocation GameBackupBuffer");
        return false;
    }
    
    // Initialiser le streaming
    if (!gameBackupBuffer->initialize()) {
        ESP_LOGE(BACKUP_TAG, "Échec initialisation GameBackupBuffer");
        delete gameBackupBuffer;
        gameBackupBuffer = nullptr;
        return false;
    }
    
    gameBackupReady = true;
    ESP_LOGI(BACKUP_TAG, "Backup de jeu TAR streaming initialisé avec succès (%zu fichiers)", 
            gameBackupBuffer->getFileCount());
    return true;
}

// Récupère le prochain chunk du TAR de jeu (streaming)
size_t getGameBackupChunk(uint8_t* output, size_t maxLen) {
    if (!gameBackupBuffer || !gameBackupReady) {
        return 0;
    }
    
    yield(); // Reset watchdog
    
    // Logging pour debug (seulement périodiquement)
    static size_t lastLoggedBytes = 0;
    size_t currentBytes = gameBackupBuffer->getTotalBytesGenerated();
    if (currentBytes - lastLoggedBytes > 5120) { // Log tous les 5KB
        ESP_LOGI(BACKUP_TAG, "Backup de jeu progress: %zu bytes générés", currentBytes);
        lastLoggedBytes = currentBytes;
    }
    
    size_t bytesRead = gameBackupBuffer->readBytes(output, maxLen);
    
    if (bytesRead == 0 && gameBackupBuffer->isFinished()) {
        ESP_LOGI(BACKUP_TAG, "Backup de jeu TAR streaming terminé: %zu bytes total", 
                gameBackupBuffer->getTotalBytesGenerated());
        lastLoggedBytes = 0; // Reset pour le prochain backup
    }
    
    return bytesRead;
}

// Nettoie le backup de jeu TAR streaming
void cleanupGameBackup() {
    if (gameBackupBuffer) {
        delete gameBackupBuffer;
        gameBackupBuffer = nullptr;
    }
    gameBackupReady = false;
    ESP_LOGI(BACKUP_TAG, "Backup de jeu TAR streaming nettoyé");
}

// Fonction utilitaire pour obtenir la taille estimée du backup de jeu
size_t getEstimatedGameBackupSize() {
    if (!gameBackupBuffer) {
        return 0;
    }
    
    size_t estimatedSize = 0;
    size_t fileCount = gameBackupBuffer->getFileCount();
    
    // Estimation approximative : chaque fichier nécessite un header + données paddées
    estimatedSize = fileCount * 512; // Headers TAR
    // La taille exacte des fichiers sera calculée lors de l'initialisation
    estimatedSize += 1024; // Blocs de fin TAR
    
    return estimatedSize;
}

// Génère le nom de fichier de backup de jeu
String generateGameBackupFilename() {
    time_t now = time(nullptr);
    struct tm* timeinfo = localtime(&now);
    
    char filename[64];
    strftime(filename, sizeof(filename), "buzzcontrol_game_backup_%Y%m%d_%H%M%S.tar", timeinfo);
    
    return String(filename);
}