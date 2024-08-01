

/*void handleUDPPacket(AsyncUDPPacket incomingPacket)
{
    String output;

    String incomingMsg="Hello, server!";
    char* c_data=(char*)incomingPacket.data();
    String s_data=String((char*)incomingPacket.data()).substring(0, incomingPacket.length());
    Serial.print("Message UDP reçu:");
    
    Serial.println(incomingPacket.length());
    Serial.println(s_data);

    if (strncmp(c_data, incomingMsg.c_str(), incomingMsg.length() )== 0) 
    {
      String identifier = s_data.substring(incomingMsg.length()); // Extract the identifier
        identifier.trim(); // Remove any leading/trailing whitespace
        String replyMsg = "Hello " + identifier + ", welcome to the game@" + WiFi.localIP().toString() + ":" + localWWWpPort;

        incomingPacket.println(replyMsg.c_str());
        Serial.printf("Response: %s\n", replyMsg.c_str());

        Serial.printf("Bumpers: %s\n", output.c_str());
    }
}

void waitNewBumper()
{
  if(Udp.listen(localUdpPort)) 
  {
    Serial.print("UDP Écoute sur le port: ");
    Serial.println(localUdpPort);
    Udp.onPacket( handleUDPPacket);
  }
}
*/