OnTree Multi-Service Architecture Specification
1. Executive Summary

This specification outlines a robust and secure architecture for enhancing OnTree to orchestrate multi-service applications using Docker Compose. The current model, which manages single containers, will be replaced by a model that treats each "App" as a complete, isolated Docker Compose project.

This transition enables OnTree to manage complex applications with multiple interconnected services as a single, cohesive unit. The architecture prioritizes security, data integrity, and operational clarity by leveraging a file-system-based source of truth, enforcing strict security policies, and adhering to Docker Compose conventions for resource naming and networking.
2. Core Principles

An "App" in OnTree is a self-contained Docker Compose Project. Each project is defined by a docker-compose.yml file and an optional .env file, which reside in a dedicated directory on the host system. The OnTree application orchestrates these projects as isolated units, managing their entire lifecycle from creation to deletion.
3. Resource Naming Convention

To guarantee isolation and prevent conflicts with other Docker resources on the host system, OnTree will leverage Docker Compose's standard project-based naming scheme.

    Project Name: The application will assign a unique project name for each app, structured as ontree-{appName} (e.g., ontree-mywebapp). This project name is the primary identifier passed to the Docker Compose Go SDK.

    Automatic Resource Prefixing: By setting the projectName, Docker Compose automatically prefixes all resources it creates for that project. This provides comprehensive conflict avoidance.

        Container Names: ontree-{appName}-{serviceName}-{index} (e.g., ontree-mywebapp-api-1)

        Network Names: ontree-{appName}_default (e.g., ontree-mywebapp_default)

        Named Volume Names: ontree-{appName}_{volumeName} (e.g., ontree-mywebapp_db-data)

This approach is robust, predictable, and consistent with Docker's own tooling, ensuring that all resources related to an app are clearly identifiable and namespaced.
4. Network Architecture

The network architecture is designed for complete isolation between apps. By using the unique projectName for each app, Docker Compose automatically creates a dedicated, isolated Docker network (e.g., ontree-mywebapp_default).

This strategy provides two key benefits:

    Complete Isolation: Services within one app (e.g., ontree-mywebapp) cannot see or interact with services in another app (e.g., ontree-anotherapp) on the network level, unless explicitly configured to do so using advanced Docker networking features like external networks.

    Effortless Service Discovery: Within a single app's network, services can communicate with each other using their service names as hostnames (e.g., a web service can connect to a db service at db:5432). This is handled automatically by Docker's embedded DNS.

5. Data Persistence Architecture

Data integrity is paramount. The architecture makes a clear distinction between temporarily stopping an application and permanently deleting its data. This applies to named volumes managed by Docker.

    Stop Operation (docker-compose down): This is the default action for stopping an app. It will stop and remove the app's containers and network. However, it will not remove any named volumes associated with the app. This is the safe, non-destructive default that ensures user data is preserved between restarts.

    Delete Operation (docker-compose down -v): A separate and explicit "Delete App" or "Wipe Data" function will be available in the UI. This triggers a destructive command that removes everything, including the named volumes. This action must be protected by a strong confirmation dialog (e.g., "This will permanently delete all data for 'mywebapp'. This action cannot be undone.") to prevent accidental data loss.

6. Configuration, Storage, and Security Architecture

This architecture uses the host file system as the definitive source of truth for all app configurations, coupled with a stringent, runtime security validation model.
6.1. File System as the Source of Truth

App configurations will not be stored in the OnTree database. Instead, they will reside in a well-defined, permanent directory structure on the host.

    Base Directory: All OnTree app data and configurations will live under /opt/onTree/apps/.

    App-Specific Directory Structure: Each app will have its own subdirectory, which will contain its configuration and serve as the context for Docker Compose.

        /opt/onTree/apps/{appName}/docker-compose.yml

        /opt/onTree/apps/{appName}/.env (optional)

    Operational Flow:

        When a user creates or updates an app via the UI, the OnTree backend writes the docker-compose.yml and .env files to the corresponding directory.

        When any operation (start, stop, etc.) is triggered, the OnTree backend uses this directory as the working context for the Docker Compose Go SDK.

        This approach allows for easy inspection, backup, and even manual modification of configurations by system administrators.

6.2. The "Always Check" Security Model

Because the configuration files on disk are the source of truth, they could potentially be modified outside of the OnTree application. To mitigate this risk, security validation must be performed every time a start operation is initiated.

The StartApp Workflow:

    Read from Disk: The OnTree backend reads the contents of /opt/onTree/apps/{appName}/docker-compose.yml.

    Parse and Validate: The file content is immediately passed to a security sanitization function. This function acts as a hard gatekeeper. If any rule is violated, the operation is aborted before any Docker command is executed.

    Proceed or Fail: If validation passes, the backend proceeds to call the Docker Compose SDK. If it fails, the API returns a descriptive error to the user.

6.3. Security Validation Rules

The sanitization function will enforce the following rules:

    Strict Bind Mount Control:

        Allowed Path: Host-path bind mounts are strictly limited to subdirectories within /opt/onTree/apps/mount/. No other host path is permitted.

        Required Naming Scheme: To ensure clarity and prevent conflicts, the host path for a service's volume must follow the pattern: /opt/onTree/apps/mount/{appName}/{serviceName}/. The application should ensure these directories are created with appropriate permissions.

        Example: For an app named my-blog with a database service, a valid docker-compose.yml entry would be:
        Generated yaml

              
        services:
          database:
            image: postgres:15
            volumes:
              - /opt/onTree/apps/mount/my-blog/database:/var/lib/postgresql/data

            

        IGNORE_WHEN_COPYING_START

    Use code with caution. Yaml
    IGNORE_WHEN_COPYING_END

    Validation Logic: The backend will parse the YAML, extract all host paths from bind mount definitions, and verify that each path string starts with /opt/onTree/apps/mount/{appName}/ and adheres to the naming scheme. Any deviation will result in rejection.

Privileged Mode Disallowed: The configuration is scanned for privileged: true. If found, the operation is rejected unless an explicit, admin-level override exists for that specific app.

Dangerous Capabilities Disallowed: The configuration is scanned for high-risk capabilities in cap_add lists (e.g., SYS_ADMIN, NET_ADMIN). These will be rejected based on a configurable deny-list.