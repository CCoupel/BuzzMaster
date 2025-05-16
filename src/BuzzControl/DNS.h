#pragma once

#include <DNSServer.h>
#include <ESPmDNS.h>

DNSServer dnsServer;

void setupDNSServer() {
    ESP_LOGI(WIFI_TAG, "Serveur DNS démarré");

    // Configuration du mDNS
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

    dnsServer.setErrorReplyCode(DNSReplyCode::NoError);
    dnsServer.start(53, "dns.msftncsi.com", IPAddress(131,107,255,255));

}
