# Application Management (Complex)

This document describes how to manage a complex, multi-service application using the onTree Node application.

## Creating a New Application

To create a new application, you can use one of the pre-defined templates. For this example, we will use the "OpenWebUI with Ollama" template.

1.  Navigate to the "Create App" page.
2.  Select the "OpenWebUI with Ollama" template.
3.  Leave all the default settings.
4.  Click the "Create" button.

This will create a new OpenWebUI application with two services: `openwebui` and `ollama`.

## Verifying the Application is Running

Once the application is created, you can verify that it is running by:

1.  Checking the application status in the UI. Both the `openwebui` and `ollama` services should show as "running".
2.  Accessing the OpenWebUI interface in your browser at the assigned port. For example, if OpenWebUI is on port 8080, you can `curl` it:
    ```bash
    curl http://localhost:8080
    ```
    You should receive the HTML content of the Web UI.
3.  Using the `docker ps` command to see the running containers. Assuming the app was named `my-webui`, the command would be:
    ```bash
    docker ps --filter "name=ontree-my-webui"
    ```
    You should see two running containers, one for `openwebui` and one for `ollama`.

## Stopping and Starting the Application

You can stop and start the application from the UI:

*   **Stop:** Click the "Stop" button to stop the application. This will stop both the `openwebui` and `ollama` services. You can verify they are stopped using `docker ps`.
*   **Start:** Click the "Start" button to start the application again.

## Deleting the Application

To delete the application:

1.  Stop the application.
2.  Click the "Delete" button.
3.  Confirm the deletion.

This will delete the application containers and the application directory from the filesystem.
