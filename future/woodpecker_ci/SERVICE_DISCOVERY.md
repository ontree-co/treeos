Of course. Here is a comprehensive software specification for the Caddy-based automated reverse proxy and service discovery system.

---

### **Software Specification: Automated Reverse Proxy and Service Discovery System**

**Version:** 1.0
**Date:** October 26, 2023
**Author:** System Architect
**Status:** Final

---

### **1. Introduction**

#### **1.1 Purpose**

This document specifies the design and implementation of an automated reverse proxy and service discovery system for a single-host server environment running Docker. The primary goal of this system is to eliminate the need for manual port management and configuration of the reverse proxy when deploying, updating, or removing containerized web services. The system will leverage Docker labels for configuration, enabling a "declarative" approach to service routing.

#### **1.2 Scope**

This specification covers the deployment of a proxy stack and its interaction with other service containers on a single Docker host. The system is responsible for:

- Routing HTTP/HTTPS traffic based on hostname to the appropriate backend service container.
- Automating the generation and renewal of TLS certificates for secure connections.
- Dynamically reconfiguring itself without downtime in response to changes in the container ecosystem (start, stop, removal of services).

This specification does not cover multi-host orchestration, load balancing across multiple physical nodes, or non-Dockerized workloads.

#### **1.3 Definitions, Acronyms, and Abbreviations**

- **Host System:** The physical or virtual machine running the Docker Engine.
- **Proxy Stack:** The set of containers responsible for the reverse proxy functionality, specifically `caddy` and `caddy-docker-proxy`.
- **Service Container:** Any Docker container that provides a service (e.g., a web application) and needs to be exposed via the proxy.
- **Docker Label:** Metadata attached to a Docker container used to declare configuration for the proxy.
- **Caddy:** A powerful, open-source web server with automatic HTTPS.
- **`caddy-docker-proxy`:** A helper application that monitors Docker events and generates a Caddyfile based on container labels.
- **Caddyfile:** The native configuration file for the Caddy web server.
- **Shared Docker Network:** A dedicated Docker bridge network (`caddy_net`) that allows the Proxy Stack to communicate with Service Containers.

---

### **2. System Architecture**

#### **2.1 Core Components**

The system is composed of three primary software components operating on the Host System:

1.  **Docker Engine:** Manages the lifecycle of all containers. Its API socket (`/var/run/docker.sock`) provides the event stream for service discovery.
2.  **Caddy (`caddy`) Container:** The core reverse proxy. It listens for inbound web traffic on ports 80 and 443 and routes it according to its Caddyfile configuration. It is responsible for all TLS termination and certificate management.
3.  **Caddy Docker Proxy (`caddy-docker-proxy`) Container:** The service discovery agent. It monitors the Docker API for container events, inspects containers for specific labels, and generates a valid Caddyfile reflecting the desired routing state.

#### **2.2 Architectural Diagram & Data Flow**

**A. Configuration Flow (Dynamic Update):**

```
+---------------------+      +----------------+      +-----------------------+      +-------------+
| 1. Admin Deploys    |----->| 2. Docker      |----->| 3. caddy-docker-proxy |----->| 4. Caddyfile|
|    Service Container|      |    API Event   |      |   (Sees Event & Labels) |      | (is updated)  |
|    (with Labels)    |      +----------------+      +-----------------------+      +-------------+
+---------------------+                                                                    |
                                                                                             V
                                                                                      +-------------+
                                                                                      | 5. Caddy    |
                                                                                      | (Auto-Reloads)|
                                                                                      +-------------+
```

**B. Request Flow (User Traffic):**

```
+-------------+      +-----------------+      +-----------------+      +--------------------+      +------------------+
| 1. User     |----->| 2. DNS Resolves |----->| 3. Host System  |----->| 4. Caddy Container |----->| 5. Service       |
|   (Browser) |      |   to Host IP    |      | (Ports 80/443)  |      |   (Routes by Host) |      |    Container     |
+-------------+      +-----------------+      +-----------------+      +--------------------+      +------------------+
                                                                               ^
                                                                               |
                                                                         (on caddy_net)
```

#### **2.3 Network Design**

A dedicated, user-defined Docker bridge network, named `caddy_net`, is a mandatory component.

- **Purpose:** It provides a stable network environment where containers can resolve each other by name. The Caddy container will route traffic to a Service Container using its container name (e.g., `http://whoami-app:80`).
- **Implementation:** This network must be created manually before deploying any stacks.
- **Attachment:** Both the `caddy` container and all `Service Containers` that need to be proxied **must** be attached to this network.

---

### **3. Functional Requirements**

| ID       | Requirement                         | Description                                                                                                                                                                                                                                 |
| :------- | :---------------------------------- | :------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **FR-1** | **Automated Routing Configuration** | The system shall automatically create and remove reverse proxy routes for Service Containers based on the presence and content of their Docker labels. No manual editing of the Caddyfile shall be required for standard operations.        |
| **FR-2** | **Zero-Downtime Reconfiguration**   | Caddy shall gracefully reload its configuration upon changes to the Caddyfile, ensuring that existing connections are not dropped and new routes become active without service interruption.                                                |
| **FR-3** | **Automatic HTTPS**                 | The system shall automatically provision and renew TLS certificates for any public-facing domain names. For internal-only hostnames (e.g., `.local`, `.lan`), it shall use its own internal certificate authority to provide trusted HTTPS. |
| **FR-4** | **Hostname-Based Routing**          | All routing decisions shall be based on the HTTP `Host` header of the incoming request. Port-based routing is explicitly avoided for user-facing access.                                                                                    |
| **FR-5** | **Label-Driven Configuration**      | All aspects of a service's proxy configuration (hostname, internal port, etc.) shall be defined via Docker labels on the Service Container itself.                                                                                          |
| **FR-6** | **Secure by Default**               | The system shall automatically redirect all HTTP requests to their HTTPS equivalent.                                                                                                                                                        |
| **FR-7** | **Service Isolation**               | Service Containers shall not expose any ports directly to the Host System's network interface. All access shall be brokered through the Caddy container.                                                                                    |

---

### **4. Non-Functional Requirements**

| ID        | Requirement         | Description                                                                                                                                                               |
| :-------- | :------------------ | :------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **NFR-1** | **Performance**     | The proxy shall introduce minimal latency (<10ms) to requests under typical load.                                                                                         |
| **NFR-2** | **Reliability**     | The Proxy Stack containers shall be configured to restart automatically on failure to ensure high availability of the routing layer.                                      |
| **NFR-3** | **Maintainability** | The system shall be maintainable through standard Docker and Docker Compose commands. The generated Caddyfile shall be human-readable for debugging purposes.             |
| **NFR-4** | **Security**        | The `caddy-docker-proxy` container's access to the Docker socket shall be read-only. The Proxy Stack shall run on its dedicated network to limit its direct connectivity. |

---

### **5. Implementation Details**

#### **5.1 Prerequisites**

- A Linux-based Host System.
- Docker Engine (Version 20.10.x or newer).
- Docker Compose (Version 2.x or newer).

#### **5.2 Network Configuration**

The shared Docker network must be created with the following command prior to any container deployment:

```bash
docker network create caddy_net
```

#### **5.3 Directory Structure**

A recommended directory structure on the Host System:

```
/opt/caddy/
├── docker-compose.yml   # For the Proxy Stack
└── caddy/
    ├── Caddyfile        # Will be auto-generated
    └── data/            # For Caddy's persistent data (certs, etc.)
```

#### **5.4 Proxy Stack Deployment (`/opt/caddy/docker-compose.yml`)**

```yaml
version: "3.8"

services:
  caddy:
    image: caddy:2
    container_name: caddy
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
      - "443:443/udp" # Required for HTTP/3
    volumes:
      - ./caddy/Caddyfile:/etc/caddy/Caddyfile
      - ./caddy/data:/data
    networks:
      - caddy_net

  caddy-docker-proxy:
    image: lucaslorentz/caddy-docker-proxy:2.7
    container_name: caddy-docker-proxy
    restart: unless-stopped
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./caddy/Caddyfile:/etc/caddy/Caddyfile
    networks:
      - caddy_net

networks:
  caddy_net:
    external: true
```

#### **5.5 Service Container Deployment (Example)**

This `docker-compose.yml` file would be located in a separate project directory.

```yaml
version: "3.8"

services:
  webapp:
    image: traefik/whoami # A simple service that displays request info
    container_name: my-webapp
    restart: unless-stopped
    # No ports are exposed to the host.
    networks:
      - caddy_net # Must connect to the shared network
    labels:
      # The primary label defining the hostname.
      # This will create a route for 'app.mydomain.com'.
      caddy: app.mydomain.com

      # Example of a more complex configuration for a service
      # running on a non-standard internal port (e.g., 3000).
      # caddy_1: dashboard.mydomain.com
      # caddy_1.reverse_proxy: "{{upstreams 3000}}"

networks:
  caddy_net:
    external: true
```

---

### **6. Operational Procedures**

#### **6.1 Deploying a New Service**

1.  Create a `docker-compose.yml` for the new service.
2.  Ensure the service is attached to the `caddy_net` network.
3.  Add the appropriate `caddy` labels to define its desired hostname.
4.  Run `docker-compose up -d` in the service's directory.
5.  The `caddy-docker-proxy` will detect the new container, update the Caddyfile, and Caddy will automatically apply the new route.

#### **6.2 Decommissioning a Service**

1.  Run `docker-compose down` in the service's directory.
2.  The `caddy-docker-proxy` will detect the container's removal and regenerate the Caddyfile without the service's entry. Caddy will reload and remove the route.

#### **6.3 Monitoring and Debugging**

- **Caddy Logs:** Check the Caddy container logs for information about certificate acquisition and traffic routing issues: `docker logs caddy`.
- **Generated Configuration:** To verify the system's state, inspect the auto-generated Caddyfile on the host: `cat /opt/caddy/caddy/Caddyfile`. This file is the ground truth for current routing rules.
- **Proxy Logs:** Check the `caddy-docker-proxy` logs for errors related to Docker events or label parsing: `docker logs caddy-docker-proxy`.
