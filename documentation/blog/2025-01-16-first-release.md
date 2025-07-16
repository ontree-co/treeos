---
slug: ontree-v0.1.0-release
title: OnTree v0.1.0 - First Release 🎉
authors: [ontree]
tags: [release, announcement]
---

We're excited to announce the first public release of OnTree - a self-hosted container management application that makes running containerized applications as simple as clicking a button.

<!--truncate-->

## What is OnTree?

OnTree is a web-based Docker container manager designed to make self-hosting applications accessible to everyone. Whether you're running a home lab, managing development environments, or deploying applications for your team, OnTree provides an intuitive interface for container management without the complexity.

## Key Features in v0.1.0

### 🚀 Easy App Deployment
- **One-Click Deployment**: Deploy applications from templates with a single click
- **Template Library**: Pre-configured templates for popular applications like Open WebUI, Nginx, and more
- **Custom Apps**: Create your own applications with guided setup

### 🎨 Modern User Interface
- **HTMX-Powered**: Fast, responsive interface without page reloads
- **Real-time Updates**: See container status changes instantly
- **Emoji Support**: Personalize your apps with custom emojis
- **Dark Mode Ready**: Clean, modern design with gradient buttons

### 📊 System Monitoring
- **Real-time Metrics**: Monitor CPU, memory, disk, and network usage
- **Historical Data**: View 24-hour trends with interactive sparkline charts
- **Detailed Views**: Click any metric for expanded time-range analysis (1h, 6h, 24h, 7d)
- **Performance Optimized**: Efficient caching and batch queries for smooth operation

### 🌐 Domain Management
- **Caddy Integration**: Automatic HTTPS with Let's Encrypt certificates
- **Subdomain Support**: Expose apps at `app.yourdomain.com`
- **Tailscale Ready**: Works with both public domains and Tailscale networks

### 🛠️ Developer Features
- **In-Browser YAML Editor**: Edit docker-compose.yml files directly
- **Operation Logging**: See detailed logs of all container operations
- **Background Workers**: Asynchronous operations for better performance
- **Migration Tools**: Easy migration from database to compose-file storage

### 🔒 Self-Contained Design
- **Single Binary**: Deploy OnTree as a single executable file
- **Embedded Assets**: All templates and static files included
- **SQLite Database**: No external database dependencies
- **Docker Integration**: Works with your existing Docker installation

## Getting Started

OnTree is designed to get you up and running quickly:

1. Download the OnTree binary for your platform
2. Run `./ontree-server` to start the application
3. Open http://localhost:8080 in your browser
4. Create your first app from our template library

Popular first apps include:
- **Open WebUI**: Run AI models locally with a ChatGPT-like interface
- **Nginx**: Quick web server for testing
- **Custom Apps**: Build your own with our guided setup

## Coming Next

We're actively developing OnTree and have exciting features planned:
- More application templates
- Enhanced monitoring capabilities
- Improved backup and restore options
- Multi-user support
- API for automation

## Get Involved

OnTree is open source and we welcome contributions!

- **GitHub**: [github.com/ontree/ontree-node](https://github.com/ontree/ontree-node)
- **Documentation**: Check out our [Getting Started guide](/docs/intro)
- **Issues**: Report bugs or request features on GitHub

## Thank You

This first release represents months of development and refinement. We're excited to share OnTree with the community and look forward to your feedback!

Happy self-hosting! 🌳