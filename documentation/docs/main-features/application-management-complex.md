# Application Management (Complex)

This document describes how to manage a complex, multi-service application using OnTree Node.

## Creating a New Application

To create a new application, you can use one of the pre-defined templates. For this example, we will use the "OpenWebUI with Ollama" template.

1. Navigate to the "Create App" page.
2. Select the "OpenWebUI with Ollama" template.
3. Leave all the default settings.
4. Click the "Create" button.

This will create a new OpenWebUI application with:
- Two services: `openwebui` and `ollama`
- Three configuration files: `docker-compose.yml`, `.env`, and `app.yaml`
- The `initial_setup_required` flag set to trigger automatic image pulling and version locking

## Application File Structure

After creation, your application directory will contain:

```
/opt/ontree/apps/openwebui-amd/
├── docker-compose.yml    # Docker services configuration
├── .env                   # Docker Compose project settings
└── app.yaml              # OnTree application metadata
```

The `.env` file will contain:
```bash
COMPOSE_PROJECT_NAME=ontree-openwebui-amd
COMPOSE_SEPARATOR=-
```

The `app.yaml` file will contain:
```yaml
id: openwebui-amd
name: OpenWebUI AMD
primary_service: openwebui
expected_services:
  - ontree-openwebui-amd-openwebui-1
  - ontree-openwebui-amd-ollama-1
initial_setup_required: true
```

## Initial Setup Process

When an application is created from a template with `initial_setup_required: true`, the OnTree agent automatically:

1. Detects the flag in `app.yaml`
2. Pulls the latest Docker images for all services
3. Updates the `docker-compose.yml` to lock specific version tags
4. Removes the `initial_setup_required` flag
5. Reports progress through the chat interface in the application detail page

You can monitor this process in the "Agent Chat" section of the application detail page.

## Verifying the Application is Running

Once the application is created and initial setup is complete, you can verify that it is running by:

1. **Checking the application status in the UI**: Both the `openwebui` and `ollama` services should show as "running".

2. **Accessing the OpenWebUI interface**: Navigate to the assigned port in your browser. For example, if OpenWebUI is on port 8080:
   ```bash
   curl http://localhost:8080
   ```
   You should receive the HTML content of the Web UI.

3. **Using Docker commands**: To see the running containers (assuming the app was named `openwebui-amd`):
   ```bash
   docker ps --filter "name=ontree-openwebui-amd"
   ```
   You should see two running containers:
   - `ontree-openwebui-amd-openwebui-1`
   - `ontree-openwebui-amd-ollama-1`

## Container Naming

Note that all containers follow the OnTree naming convention:
- Directory name: `OpenWebUI-AMD` (can be mixed case)
- Container names: `ontree-openwebui-amd-*` (always lowercase)

This is because OnTree automatically converts application identifiers to lowercase for Docker operations. See the [Naming Convention](./naming-convention.md) documentation for more details.

## Stopping and Starting the Application

You can control the application from the UI:

- **Stop**: Click the "Stop" button to stop all services. This executes `podman compose down` which stops and removes containers but preserves volumes.
- **Start**: Click the "Start" button to start all services again. This executes `podman compose up -d`.

The stop/start operations use the `COMPOSE_PROJECT_NAME` from the `.env` file to ensure the correct containers are managed.

## Viewing Logs

Access container logs through:
1. The "Logs" button in the UI for each service
2. Docker CLI: `docker logs ontree-openwebui-amd-openwebui-1`

## Editing Configuration

You can edit all three configuration files directly from the UI:
1. Navigate to the application detail page
2. Click on the respective "Edit" button for:
   - `.env` Configuration
   - `app.yaml` Configuration  
   - `docker-compose.yml` Configuration
3. Make your changes
4. Click "Save"

If containers are running when you save `docker-compose.yml`, OnTree will automatically recreate them with the new configuration.

## Deleting the Application

To delete the application:

1. Stop the application (if running).
2. Click the "Delete" button.
3. Confirm the deletion.

This will:
- Remove all containers with the `ontree-<app>-*` prefix
- Delete the application directory from `/opt/ontree/apps/`
- Remove associated volumes (unless explicitly preserved)
