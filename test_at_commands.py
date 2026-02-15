#!/usr/bin/env python3
import serial
import time
import sys

def send_at_command(ser, command, timeout=2):
    """Envoie une commande AT et lit la réponse"""
    ser.write(f"{command}\r\n".encode())
    ser.flush()
    time.sleep(0.1)

    start_time = time.time()
    response = ""
    while time.time() - start_time < timeout:
        if ser.in_waiting > 0:
            chunk = ser.read(ser.in_waiting).decode('utf-8', errors='ignore')
            response += chunk
            if "OK" in response or "ERROR" in response:
                break
        time.sleep(0.05)

    return response

def main():
    port = "COM5"
    baud = 115200

    print(f"Connexion à {port} @ {baud} baud...")
    try:
        ser = serial.Serial(port, baud, timeout=1)
        time.sleep(1)  # Attendre stabilisation

        # Vider le buffer série
        ser.reset_input_buffer()

        # Test AT
        print("\n[TEST] Envoi de AT...")
        response = send_at_command(ser, "AT", timeout=1)
        print(f"Réponse: {response.strip()}")

        # Version
        print("\n[VERSION] Envoi de AT+VERSION...")
        response = send_at_command(ser, "AT+VERSION", timeout=1)
        print(f"Réponse: {response.strip()}")

        # Status
        print("\n[STATUS] Envoi de AT+STATUS...")
        response = send_at_command(ser, "AT+STATUS", timeout=1)
        print(f"Réponse: {response.strip()}")

        ser.close()
        print("\n✅ Test terminé")

    except serial.SerialException as e:
        print(f"❌ Erreur série: {e}")
        sys.exit(1)

if __name__ == "__main__":
    main()
