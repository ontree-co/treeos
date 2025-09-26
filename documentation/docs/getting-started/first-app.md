---
sidebar_position: 3
---

# Your First App: Open WebUI

Let's deploy your first application with OnTree! We'll use Open WebUI - a powerful, self-hosted interface for running AI models locally. This tutorial will walk you through the entire process from start to finish.

## What is Open WebUI?

Open WebUI provides a ChatGPT-like interface for interacting with local LLMs (Large Language Models). It's perfect as a first app because it:
- Has a beautiful, intuitive interface
- Works immediately after deployment
- Demonstrates OnTree's power in simplifying complex deployments

## Step 1: Access the Templates Page

1. **Open OnTree** in your browser at `http://localhost:8080`
2. **Click "Create New App"** on the dashboard
3. You'll be taken to the templates page

## Step 2: Select Open WebUI Template

1. **Find "Open WebUI"** in the template list
2. **Click "Use Template"** to start the creation process

The template includes:
- Pre-configured docker-compose.yml
- Optimal resource settings
- Persistent volume for your data
- Automatic port allocation

## Step 3: Configure Your App

You'll see the app creation form with pre-filled values from the template.

### App Name
- **Default**: `openwebui`
- **Your choice**: Keep the default or choose something like `ai-chat` or `my-assistant`
- **Requirements**: Lowercase letters, numbers, and hyphens only

### Select an Emoji
OnTree lets you personalize your apps with emojis:

1. **Browse the emoji picker** showing 7 random emojis
2. **Click an emoji** to select it (it will highlight in blue)
3. **Click the shuffle button** 🔄 for different options
4. **Suggestion**: 🤖 or 🧠 work great for AI apps!

### Review Configuration
The template automatically configures:
- **Image**: `ghcr.io/open-webui/open-webui:main`
- **Port**: Automatically allocated (usually 3000-4000 range)
- **Volume**: Persistent storage at `/app/backend/data`

## Step 4: Create and Start

1. **Click "Create App"** at the bottom of the form
2. OnTree will:
   - Create the app directory
   - Generate the docker-compose.yml
   - Save your configuration

3. **On the app detail page**, click the green **"Create & Start"** button
4. Watch the operation logs as OnTree:
   - Pulls the Docker image
   - Creates the container
   - Starts Open WebUI

This may take 2-3 minutes on first run as the image downloads.

## Step 5: Access Open WebUI

Once the container is running:

1. **Look for the "Container Info" card**
2. **Find the mapped port** (e.g., `3000 → 8080`)
3. **Click the port number** or navigate to `http://localhost:3000`

### First Time Setup

When you first access Open WebUI:

1. **Create an account** - This is your local admin account
2. **Configure your name and preferences**
3. **You're ready to go!**

## Step 6: (Optional) Expose with a Domain

If you've [configured domains](/docs/getting-started/domain-setup), you can make Open WebUI accessible at a custom URL:

1. **In the "Domain & Access" section**, enter a subdomain:
   - Example: `chat` for `chat.yourdomain.com`
   - Or: `ai` for `ai.yourdomain.com`

2. **Click "Expose App"**

3. **Access your app** at `https://chat.yourdomain.com` with automatic HTTPS!

## Using Open WebUI

### Adding AI Models

Open WebUI supports multiple model backends:

1. **Ollama** (Recommended for beginners):
   ```bash
   # Install Ollama on your host
   curl -fsSL https://ollama.com/install.sh | sh
   
   # Pull a model
   ollama pull llama2
   ```

2. **Configure in Open WebUI**:
   - Go to Settings → Models
   - Add Ollama URL: `http://host.containers.internal:11434`
   - Your models will appear automatically

### Features to Explore

- **Chat Interface**: Similar to ChatGPT
- **Multiple Models**: Switch between different AI models
- **Conversation History**: All chats are saved locally
- **Custom Prompts**: Create reusable prompt templates
- **Document Upload**: Chat with your documents (PDFs, text files)

## Managing Your App

### Viewing Logs

1. Click **"View Logs"** on the app detail page
2. See real-time container output
3. Useful for troubleshooting

### Stopping and Starting

- **Stop**: Click the "Stop" button when the container is running
- **Start**: Click "Start" when the container is stopped
- OnTree preserves your data between restarts

### Editing Configuration

1. Click **"Edit"** in the Configuration card
2. Modify the docker-compose.yml
3. Click **"Save"** - OnTree will recreate the container if needed

## Troubleshooting

### Container Won't Start

Check the logs for errors:
- **Port conflict**: The assigned port might be in use
- **Resource limits**: Ensure you have enough RAM (2GB minimum)

### Can't Access the Web Interface

1. Verify the container is running (green status)
2. Check the mapped port in Container Info
3. Try accessing via `localhost` instead of `127.0.0.1`

### Models Not Loading

If using Ollama:
1. Ensure Ollama is running on the host
2. Use `http://host.containers.internal:11434` as the Ollama URL
3. Check that models are pulled: `ollama list`

## What's Next?

Congratulations! You've successfully deployed your first app with OnTree. Here's what to explore next:

### Try More Templates
- **Nginx**: Simple web server for testing
- **PostgreSQL**: Database server
- **Redis**: In-memory data store

### Learn OnTree Features
- [App Management](/docs/features/app-management) - Full container lifecycle
- [Monitoring](/docs/features/monitoring) - Track resource usage
- [Templates](/docs/features/templates) - Create custom templates

### Customize Open WebUI
- Add more AI models
- Create custom prompts
- Integrate with your workflow

## Tips for Success

1. **Start Simple**: Get comfortable with basic operations before advanced features
2. **Check Logs**: When something goes wrong, logs usually have the answer
3. **Use Emojis**: They make finding apps in your dashboard much easier
4. **Regular Backups**: OnTree preserves data, but backups are always wise

Welcome to the OnTree community! You're now ready to self-host any application with confidence. 🚀
