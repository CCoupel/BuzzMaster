#pragma once
#include <LittleFS.h>
#include <ArduinoJson.h>
#include "Common/CustomLogger.h"

static const char* CONFIG_TAG = "CONFIG_MANAGER";
static const char* CONFIG_FILE = "/config/config.json";

class ConfigManager {
private:
    JsonDocument config;
    bool loaded = false;
    
    // Default configuration values
    void setDefaults() {
        config["wifi"]["ssid"] = "CC-Home";
        config["wifi"]["password"] = "GenericPassword";
        config["ap"]["ssid"] = "buzzmaster";
        config["ap"]["password"] = "BuzzMaster";
        config["network"]["controler_port"] = 1234;
        config["network"]["log_port"] = 8888;
        config["update"]["base_url"] = "https://bitbucket.org/ccoupel/buzzcontrol/raw/main/data";
        config["update"]["version_file"] = "/config/version.txt";
    }
    
public:
    ConfigManager() {
        setDefaults();
    }
    
    bool load() {
        if (!LittleFS.begin()) {
            ESP_LOGE(CONFIG_TAG, "Failed to mount LittleFS");
            return false;
        }
        
        // Create directory if it doesn't exist
        if (!LittleFS.exists("/config")) {
            LittleFS.mkdir("/config");
        }
        
        if (!LittleFS.exists(CONFIG_FILE)) {
            ESP_LOGW(CONFIG_TAG, "Config file not found, creating default");
            return save(); // Save default configuration
        }
        
        File file = LittleFS.open(CONFIG_FILE, "r");
        if (!file) {
            ESP_LOGE(CONFIG_TAG, "Failed to open config file");
            return false;
        }
        
        DeserializationError error = deserializeJson(config, file);
        file.close();
        
        if (error) {
            ESP_LOGE(CONFIG_TAG, "Failed to parse config file: %s", error.c_str());
            setDefaults(); // Revert to defaults on error
            return false;
        }
        
        loaded = true;
        ESP_LOGI(CONFIG_TAG, "Configuration loaded successfully");
        return true;
    }
    
    bool save() {
        File file = LittleFS.open(CONFIG_FILE, "w");
        if (!file) {
            ESP_LOGE(CONFIG_TAG, "Failed to open config file for writing");
            return false;
        }
        
        if (serializeJsonPretty(config, file) == 0) {
            ESP_LOGE(CONFIG_TAG, "Failed to write config file");
            file.close();
            return false;
        }
        
        file.close();
        ESP_LOGI(CONFIG_TAG, "Configuration saved successfully");
        return true;
    }
    
    // Getters for WiFi configuration
    String getWifiSSID() {
        return config["wifi"]["ssid"].as<String>();
    }
    
    String getWifiPassword() {
        return config["wifi"]["password"].as<String>();
    }
    
    String getAPSSID() {
        return config["ap"]["ssid"].as<String>();
    }
    
    String getAPPassword() {
        return config["ap"]["password"].as<String>();
    }
    
    // Getters for network configuration
    int getControllerPort() {
        return config["network"]["controler_port"].as<int>();
    }
    
    int getLogPort() {
        return config["network"]["log_port"].as<int>();
    }
    
    // Getters for update configuration
    String getUpdateBaseURL() {
        return config["update"]["base_url"].as<String>();
    }
    
    String getVersionFile() {
        return config["update"]["version_file"].as<String>();
    }
    
    // Setters
    void setWifiCredentials(const String& ssid, const String& password) {
        config["wifi"]["ssid"] = ssid;
        config["wifi"]["password"] = password;
    }
    
    void setAPCredentials(const String& ssid, const String& password) {
        config["ap"]["ssid"] = ssid;
        config["ap"]["password"] = password;
    }
    
    void setNetworkPorts(int controllerPort, int logPort) {
        config["network"]["controler_port"] = controllerPort;
        config["network"]["log_port"] = logPort;
    }
    
    void setUpdateConfig(const String& baseUrl, const String& versionFile) {
        config["update"]["base_url"] = baseUrl;
        config["update"]["version_file"] = versionFile;
    }
    
    // Update individual values
    bool updateValue(const String& path, const JsonVariant& value) {
        JsonObject current = config.as<JsonObject>();
        String pathCopy = path;
        
        // Split path by dots
        int lastDot = -1;
        int nextDot = pathCopy.indexOf('.');
        
        while (nextDot != -1) {
            String segment = pathCopy.substring(lastDot + 1, nextDot);
            if (!current.containsKey(segment)) {
                current[segment] = JsonObject();
            }
            current = current[segment].as<JsonObject>();
            lastDot = nextDot;
            nextDot = pathCopy.indexOf('.', lastDot + 1);
        }
        
        // Set the final value
        String finalKey = pathCopy.substring(lastDot + 1);
        current[finalKey] = value;
        
        return save();
    }
    
    // Get the full configuration as JSON string
    String getConfigJSON() {
        String output;
        serializeJsonPretty(config, output);
        return output;
    }
    
    // Check if configuration is loaded
    bool isLoaded() {
        return loaded;
    }
    
    // Reset to defaults
    void resetToDefaults() {
        setDefaults();
        save();
    }
};

// Global instance
ConfigManager configManager;