# Naming Convention

This document describes the naming convention used for Docker resources in the onTree Node application.

## Resource Naming

To ensure isolation and prevent conflicts between applications, onTree Node uses a consistent naming scheme for all Docker resources. The naming convention is as follows:

```
ontree-{appName}-{serviceName}-{index}
```

*   `ontree`: A static prefix to identify all resources managed by onTree Node.
*   `appName`: The name of the application, as defined by the user.
*   `serviceName`: The name of the service, as defined in the `docker-compose.yml` file.
*   `index`: A number to differentiate between multiple instances of the same service.

## Example

For an application named `my-web-app` with a `web` service and a `db` service, the Docker resources would be named:

*   **Containers:** `ontree-my-web-app-web-1`, `ontree-my-web-app-db-1`
*   **Network:** `ontree-my-web-app_default`
*   **Volumes:** `ontree-my-web-app_db-data`
