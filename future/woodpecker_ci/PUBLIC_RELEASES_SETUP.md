# Public Releases Setup for TreeOS

This document describes how to set up automatic publishing of releases to the public repository.

## Prerequisites

1. **Public Repository Created**: The public repository at https://github.com/ontree-co/treeos should exist
2. **Personal Access Token**: You need to create a GitHub Personal Access Token with appropriate permissions

## Step-by-Step Setup

### 1. Create Personal Access Token (PAT)

1. Go to GitHub Settings → Developer settings → Personal access tokens → Tokens (classic)
2. Click "Generate new token" → "Generate new token (classic)"
3. Configure the token:
   - **Note**: `TreeOS Public Releases`
   - **Expiration**: Choose an appropriate expiration (recommend 1 year)
   - **Scopes**: Select only `public_repo` scope
4. Click "Generate token"
5. **IMPORTANT**: Copy the token immediately (you won't see it again)

### 2. Add Token as Repository Secret

1. Go to your private repository: https://github.com/stefanmunz/treeos
2. Navigate to Settings → Secrets and variables → Actions
3. Click "New repository secret"
4. Add the secret:
   - **Name**: `PUBLIC_RELEASES_TOKEN`
   - **Secret**: Paste the token you copied
5. Click "Add secret"

### 3. Ensure Public Repository Exists

The public repository should be created at https://github.com/ontree-co/treeos with:
- Public visibility
- A basic README.md explaining it's for releases only
- No source code

## How It Works

When you push a tag to your private repository:

```bash
git tag v1.0.0
git push origin v1.0.0
```

The workflow will:
1. Run tests in the private repo
2. Build and release binaries to the private repo using GoReleaser
3. Download those release assets
4. Create a matching release in the public repository with:
   - The same version tag
   - All binary assets and checksums
   - Public-friendly release notes
   - Installation instructions

## Testing

To test the setup:
1. Create a test tag: `git tag v0.0.1-test`
2. Push it: `git push origin v0.0.1-test`
3. Monitor the Actions tab in your private repo
4. Check the public repo for the new release

## Troubleshooting

### Token Permission Issues
- Ensure the token has `public_repo` scope
- Verify the token hasn't expired
- Check the secret name is exactly `PUBLIC_RELEASES_TOKEN`

### Release Already Exists
- The workflow will handle existing releases by updating them
- Manual cleanup may be needed for failed partial releases

### Missing Assets
- Check the GoReleaser job completed successfully
- Verify the asset names match the expected patterns

## Maintenance

- **Token Expiration**: Set a calendar reminder to renew the PAT before it expires
- **Release Notes**: Update the template in `.github/workflows/release.yml` as needed
- **Asset Names**: If binary naming changes in GoReleaser, update the download instructions