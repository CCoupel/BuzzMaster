#pragma once

#include <DNSServer.h>
#include <ESPmDNS.h>


DNSServer dnsServer;
void setupDNS() {
    dnsServer.setErrorReplyCode(DNSReplyCode::NoError);
    dnsServer.start(53, "dns.msftncsi.com", IPAddress(131,107,255,255));
    dnsServer.start(53, "*", apIP);
}

void setupmDNS() {
    if(MDNS.begin("buzzcontrol")) {
        ESP_LOGI(WIFI_TAG, "MDNS responder started");
        
        // Enregistrer les services
        MDNS.addService("buzzcontrol", "tcp", configManager.getControllerPort());
        MDNS.addService("http", "tcp", 80);  // HTTP is always on port 80
        MDNS.addService("sock", "tcp", configManager.getControllerPort());
        
        // Show IPs for debug
        ESP_LOGI(WIFI_TAG, "AP IP: %s", apIP.toString().c_str());
        ESP_LOGI(WIFI_TAG, "STA IP: %s", WiFi.localIP().toString().c_str());
    }
}


void setupDNSServer() {
    ESP_LOGI(WIFI_TAG, "Serveur DNS démarré");

    // Configuration du mDNS
        setupmDNS();
        setupDNS();

}


