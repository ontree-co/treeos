#!/bin/bash
# Fix MongoDB URI format for Unifi Network Application
# This script removes empty credentials from MongoDB connection strings

CONFIG_FILE="/config/data/system.properties"

# Wait for the config file to be created
while [ ! -f "$CONFIG_FILE" ]; do
    echo "Waiting for $CONFIG_FILE to be created..."
    sleep 2
done

# Fix the MongoDB URIs by removing empty credentials and escapes
echo "Fixing MongoDB connection strings..."
sed -i 's|mongodb\\://\\:@|mongodb://|g' "$CONFIG_FILE"
sed -i 's|?tls\\=false|?tls=false|g' "$CONFIG_FILE"

# Verify the changes
echo "Updated MongoDB URIs:"
grep "mongo.uri" "$CONFIG_FILE"

echo "MongoDB URI fix completed"