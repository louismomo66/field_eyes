#!/bin/bash

# Configuration
API_URL="http://localhost:9002/api"
TOKEN_FILE="/path/to/your/auth_token.txt"  # Create this file and put your JWT token in it

# Load the authentication token
if [ -f "$TOKEN_FILE" ]; then
    TOKEN=$(cat "$TOKEN_FILE")
else
    echo "Error: Token file not found at $TOKEN_FILE"
    exit 1
fi

# Generate notifications for all devices
echo "Generating notifications for all devices at $(date)"
curl -s -X POST "$API_URL/notifications/generate" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  > /dev/null

# Check the exit status
if [ $? -eq 0 ]; then
    echo "Notification generation completed successfully"
else
    echo "Error: Failed to generate notifications"
    exit 1
fi

# Optional: Check specific devices that need close monitoring
# Uncomment and add your device serial numbers
# CRITICAL_DEVICES=("350123451234560" "360123451234561")
# 
# for DEVICE in "${CRITICAL_DEVICES[@]}"; do
#     echo "Checking critical device: $DEVICE"
#     curl -s -X GET "$API_URL/devices/notifications?serial_number=$DEVICE" \
#       -H "Authorization: Bearer $TOKEN" \
#       -H "Content-Type: application/json" \
#       > /dev/null
# done

exit 0 