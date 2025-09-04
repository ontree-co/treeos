# Deploying Docusaurus to Netlify

## Step-by-Step Instructions

### 1. Login to Netlify
Go to https://app.netlify.com and login to your account.

### 2. Create New Site
Click "Add new site" → "Import an existing project"

### 3. Connect to Git
1. Choose "Deploy with GitHub"
2. Authenticate if needed
3. Select the `ontree-node` repository

### 4. Configure Build Settings
Netlify should auto-detect these from `netlify.toml`, but verify:

- **Base directory**: `documentation`
- **Build command**: `npm run build`
- **Publish directory**: `documentation/build`
- **Node version**: Will use v20 (from netlify.toml)

### 5. Deploy Site
Click "Deploy site" - The initial deploy will start automatically.

### 6. Change Site Name (Important!)
1. Go to "Site configuration" → "Site details"
2. Click "Change site name"
3. Change it to something private like: `ontree-docs-private`
4. This creates the URL: `ontree-docs-private.netlify.app`

### 7. Update the Proxy Configuration
Once deployed, update the proxy in `ontree-landingpages/netlify.toml`:

```toml
[[redirects]]
  from = "/docs/*"
  to = "https://ontree-docs-private.netlify.app/docs/:splat"
  status = 200
  force = true
```

Replace `ontree-docs-private` with your actual site name.

## What's Already Configured

✅ **Build Configuration** - Set in `netlify.toml`
✅ **Node Version** - Set to v20
✅ **Base URL** - Set to `/docs/` in Docusaurus config
✅ **SEO Protection** - robots.txt blocks crawlers on private URL
✅ **Broken Links** - Set to warn (not fail build)

## Testing After Deployment

1. **Test Private URL**: Visit `https://[your-site-name].netlify.app/docs/`
   - Should load documentation
   - Check browser console for errors
   - Navigate between pages

2. **Deploy Landing Pages**: After updating proxy URL
   - Push changes to ontree-landingpages
   - Test at `https://ontree.co/docs`

## Troubleshooting

**Build Fails**: 
- Check Node version (needs 18.20.8+)
- Check build logs in Netlify dashboard

**404 Errors**:
- Verify baseUrl is `/docs/` in docusaurus.config.ts
- Check publish directory is `documentation/build`

**Assets Not Loading**:
- Check browser console for specific errors
- Verify baseUrl matches proxy configuration

## Next Steps

After successful deployment:
1. Update proxy URL in landing pages
2. Deploy landing pages
3. Test full integration at ontree.co/docs