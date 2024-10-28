#pragma once

#include <Arduino.h>
#include <WiFiUdp.h>
#include <esp_log.h>
#include <time.h>

class CustomLogger {
private:
    static WiFiUDP udp;
    static uint16_t logPort;
    static bool initialized;
    static IPAddress broadcastIP;

    static const char* getLevelColor(esp_log_level_t level) {
        switch (level) {
            case ESP_LOG_ERROR:   return "\033[1;31m"; // Rouge gras
            case ESP_LOG_WARN:    return "\033[1;33m"; // Jaune gras
            case ESP_LOG_INFO:    return "\033[1;32m"; // Vert gras
            case ESP_LOG_DEBUG:   return "\033[1;36m"; // Cyan gras
            case ESP_LOG_VERBOSE: return "\033[1;35m"; // Magenta gras
            default:              return "\033[0m";    // Réinitialisation
        }
    }

    static const char* getLevelString(esp_log_level_t level) {
        switch (level) {
            case ESP_LOG_ERROR:   return "E";
            case ESP_LOG_WARN:    return "W";
            case ESP_LOG_INFO:    return "I";
            case ESP_LOG_DEBUG:   return "D";
            case ESP_LOG_VERBOSE: return "V";
            default:              return "U";
        }
    }

    static void getTimestamp(char* buffer, size_t bufferSize) {
        struct tm timeinfo;
        if(!getLocalTime(&timeinfo)){
            snprintf(buffer, bufferSize, "Time not set");
            return;
        }
        strftime(buffer, bufferSize, "%Y-%m-%d %H:%M:%S", &timeinfo);
    }

public:
    static void init(uint16_t port) {
        logPort = port;
        initialized = true;
        broadcastIP = IPAddress(255, 255, 255, 255);  // Adresse de broadcast
        configTime(0, 0, "pool.ntp.org");  // Configurez le fuseau horaire et le serveur NTP si nécessaire
//        esp_log_set_vprintf(customLogFunction);
    }

    static int customLogFunction(esp_log_level_t level, const char* tag, const char* format, va_list args) {
        enum { BUFFER_SIZE = 2048 };
        char* logBuffer = (char*)malloc(BUFFER_SIZE); // Allocation dynamique plus sûre
        if (!logBuffer) return -1;
        
        int totalLen = 0;
        int written = 0;
        size_t remainingSpace = BUFFER_SIZE;

        // Préparer les composants du message
        const char* levelColor = getLevelColor(level);
        const char* levelStr = getLevelString(level);
        unsigned long timestamp = millis();
        
        // Construire l'information IP de manière sécurisée
        String ipInfo;
        if (WiFi.getMode() & WIFI_AP) {
            ipInfo = WiFi.softAPIP().toString();
        }
        if (WiFi.getMode() & WIFI_STA && WiFi.status() == WL_CONNECTED) {
            if (ipInfo.length() > 0) {
                ipInfo += "/";
            }
            ipInfo += WiFi.localIP().toString();
        }

        // Écrire le préfixe
        written = snprintf(logBuffer, remainingSpace,
                        "%s%s \033[1;37m%lu\033[0m %.*s %s(\033[1m%s%s)\033[0m ",
                        levelColor, levelStr,
                        timestamp,
                        std::min((int)ipInfo.length(), 45), // Limiter la longueur de l'IP
                        ipInfo.c_str(),
                        levelColor, tag, levelColor);

        if (written < 0 || written >= remainingSpace) {
            free(logBuffer);
            return -1;
        }

        totalLen = written;
        remainingSpace -= written;

        // Écrire le message principal
        va_list args_copy;
        va_copy(args_copy, args);
        written = vsnprintf(logBuffer + totalLen, remainingSpace, format, args_copy);
        va_end(args_copy);

        if (written < 0 || written >= remainingSpace) {
            written = remainingSpace - 1;
        }

        totalLen += written;
        remainingSpace -= written;

        // Ajouter la séquence de fin
        const char* suffix = "\033[0m\n";
        size_t suffixLen = strlen(suffix);
        
        if (remainingSpace > suffixLen) {
            memcpy(logBuffer + totalLen, suffix, suffixLen);
            totalLen += suffixLen;
        }

        // Écrire vers la sortie
        if (totalLen > 0) {
            Serial.write(logBuffer, totalLen);
            
            if (initialized) {
                udp.beginPacket(broadcastIP, logPort);
                udp.write((const uint8_t*)logBuffer, totalLen);
                udp.endPacket();
            }
        }

        free(logBuffer);
        return totalLen;
    }

    static void log(esp_log_level_t level, const char* tag, const char* format, ...) {
        va_list args;
        va_start(args, format);
        customLogFunction(level, tag, format, args);
        va_end(args);
    }
};

WiFiUDP CustomLogger::udp;
uint16_t CustomLogger::logPort;
bool CustomLogger::initialized = false;
IPAddress CustomLogger::broadcastIP;

#define CUSTOM_LOGE(tag, format, ...) CustomLogger::log(ESP_LOG_ERROR, tag, format, ##__VA_ARGS__)
#define CUSTOM_LOGW(tag, format, ...) CustomLogger::log(ESP_LOG_WARN, tag, format, ##__VA_ARGS__)
#define CUSTOM_LOGI(tag, format, ...) CustomLogger::log(ESP_LOG_INFO, tag, format, ##__VA_ARGS__)
#define CUSTOM_LOGD(tag, format, ...) CustomLogger::log(ESP_LOG_DEBUG, tag, format, ##__VA_ARGS__)
#define CUSTOM_LOGV(tag, format, ...) CustomLogger::log(ESP_LOG_VERBOSE, tag, format, ##__VA_ARGS__)

// Surcharge des macros ESP_LOGx
#undef ESP_LOGE
#undef ESP_LOGW
#undef ESP_LOGI
#undef ESP_LOGD
#undef ESP_LOGV

#define ESP_LOGE(tag, format, ...) CUSTOM_LOGE(tag, format, ##__VA_ARGS__)
#define ESP_LOGW(tag, format, ...) CUSTOM_LOGW(tag, format, ##__VA_ARGS__)
#define ESP_LOGI(tag, format, ...) CUSTOM_LOGI(tag, format, ##__VA_ARGS__)
#define ESP_LOGD(tag, format, ...) CUSTOM_LOGD(tag, format, ##__VA_ARGS__)
#define ESP_LOGV(tag, format, ...) CUSTOM_LOGV(tag, format, ##__VA_ARGS__)