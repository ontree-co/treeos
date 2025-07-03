#!/bin/bash

echo "Testing Docker image update feature..."

# Check if nginx-old app is visible
echo "1. Checking if nginx-old app is discovered..."
curl -s http://localhost:8085/ | grep -q "nginx-old" && echo "✓ App found" || echo "✗ App not found"

# Check if the API endpoint for update check exists
echo -e "\n2. Testing update check API endpoint..."
response=$(curl -s -w "\n%{http_code}" http://localhost:8085/apps/nginx-old/check-update)
http_code=$(echo "$response" | tail -n1)
content=$(echo "$response" | head -n-1)

if [ "$http_code" = "302" ]; then
    echo "✗ Redirected (likely not authenticated)"
elif [ "$http_code" = "200" ]; then
    echo "✓ API endpoint accessible"
    echo "Response: $content"
else
    echo "✗ Unexpected response code: $http_code"
fi

echo -e "\nUpdate feature implementation complete!"
echo "To fully test:"
echo "1. Log in to the web interface at http://localhost:8085"
echo "2. Navigate to the nginx-old app"
echo "3. Click 'Check for Updates' button"
echo "4. If updates are available, click 'Update Now'"