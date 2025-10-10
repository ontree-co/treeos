# Nzyme Docker Setup - Knowledge Base

## Summary
This document captures the investigation and implementation attempt for creating a Docker-based nzyme deployment template for TreeOS. The investigation revealed significant challenges with containerizing nzyme, ultimately leading to a decision to postpone this integration.

## Key Findings

### 1. No Official Docker Support
- Nzyme project does not provide official Docker images
- Documentation focuses on bare-metal installation (Ubuntu, Debian)
- Designed for direct hardware access rather than containerized environments

### 2. Community Docker Images
- **r4ulcl/nzyme**: Only available image, but severely outdated (v1.2.2 from 2021, current is v2.0+)
- Image has broken entrypoint script making it unusable
- No maintained community alternatives exist

### 3. Technical Challenges

#### Network Permissions
- Nzyme requires `NET_ADMIN` capability for packet capture
- Needs direct access to network interfaces for WiFi monitoring
- TreeOS security validator blocks dangerous capabilities for security reasons
- These restrictions are by design to protect multi-tenant environments

#### Distribution Challenges
- Nzyme distributed as .deb/.rpm packages, not container-friendly
- No simple JAR files or binaries available
- Official packages may be behind authentication/registration
- Building from source is complex and time-consuming

### 4. Architecture Considerations

#### Why Containers Are Problematic for Nzyme
1. **Hardware Access**: Needs direct access to WiFi adapters in monitor mode
2. **Security Model**: Requires root-like permissions that containers shouldn't have
3. **Network Isolation**: Container networking adds complexity for packet capture
4. **State Management**: Not designed as a stateless, cloud-native application

## Implementation Attempts

### Attempt 1: Using r4ulcl/nzyme Image
- Failed due to broken entrypoint script expecting `nzyme.conf.tmp`
- Version too old (1.2.2) with incompatible configuration format
- Container crashes immediately after starting

### Attempt 2: Custom Startup Script
Created custom initialization script to work around image issues:
- Still failed due to fundamental incompatibilities
- Old nzyme version doesn't support modern configuration format
- JAR file exits immediately even with proper configuration

### Final Solution: Documentation-Only Approach
Created a PostgreSQL + Instructions setup:
- Provides PostgreSQL database ready for nzyme
- Serves HTML page with detailed setup instructions
- Guides users to install nzyme on host system
- Honest about limitations while providing value

## Effort Analysis for Creating Custom Docker Image

### Option 1: Build from Source
**Effort: 20-30 hours + ongoing maintenance**
- Clone repository
- Set up build environment (Java, Maven, Node.js)
- Build backend and frontend
- Package into Docker image
- Create configuration management
- Test extensively

### Option 2: Extract from Packages
**Effort: 10-15 hours**
- Find or build .deb packages
- Extract files and dependencies
- Reverse-engineer file structure
- Create startup orchestration

### Option 3: Wrapper Around APT Install
**Effort: 5-10 hours**
- Add nzyme repository to container
- Install via apt
- Handle repository authentication
- Configure services

### Why Nobody Has Done It Properly
1. Nzyme team focuses on enterprise VM/bare-metal deployments
2. Small user base doesn't justify maintenance effort
3. Network permission issues make containers less attractive
4. Users needing nzyme can handle manual installation

## TreeOS Philosophy Discussion

### The Security Model Debate

**Power User Perspective:**
- Docker is just a distribution format
- Users should decide what permissions to grant
- Containers can have any capability if needed
- Trust the deployer to make informed decisions

**TreeOS Perspective:**
- Protect users by default
- One compromised app shouldn't access all network traffic
- Make dangerous permissions require explicit action
- Similar to iOS/Android app sandboxing

### Why TreeOS Blocks NET_ADMIN
1. **Multi-tenant safety**: Multiple apps run together
2. **User protection**: Most users don't understand risks
3. **Attack prevention**: Malicious apps could sniff passwords, redirect traffic
4. **Explicit security**: Dangerous operations require manual installation

## Recommendations for Future

### Short Term
- Keep PostgreSQL + Instructions approach
- Document as "requires manual setup"
- Wait for official Docker support from nzyme project

### Long Term Options
1. **Create Official TreeOS Nzyme Image**
   - Build from source
   - Maintain with each nzyme release
   - Requires significant ongoing effort

2. **Selective Permission Override**
   - Add TreeOS feature for "trusted apps"
   - Show warnings but allow NET_ADMIN for specific containers
   - User explicitly accepts risks

3. **Hybrid Approach**
   - Run nzyme-node in container (management interface)
   - Run nzyme-tap on host or dedicated hardware
   - Best of both worlds

### Prerequisites for Reconsideration
- Official nzyme Docker images
- Better container-friendly distribution
- TreeOS trusted app framework
- Clear user demand

## Conclusion
While technically possible to containerize nzyme, the effort-to-benefit ratio doesn't justify it currently. The combination of:
- Lack of official support
- Security model conflicts
- Limited user base
- High maintenance burden

Makes this a poor candidate for TreeOS integration at this time. The PostgreSQL + Instructions approach provides value while being honest about limitations.

## Files and Templates Created

The following files were created during this investigation and are preserved here for future reference:

### Docker Compose Template (nzyme.yml)
See: `future/nzyme.yml`

### HTML Instructions Page
See: `future/nzyme-instructions.html`

### Template Metadata
See: `future/nzyme-templates-entry.json`

### Configuration Attempts
See: `future/nzyme-config-attempts/`