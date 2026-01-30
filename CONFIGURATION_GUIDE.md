# XOBITO Configuration Guide

## ✅ Current Configuration Status

### 1. **Email Service** - ❌ NOT CONFIGURED
The application does NOT have email sending functionality built-in. Email fields are only used for:
- User authentication (login)
- User identification
- Display purposes

**No SMTP/Email service is required.**

### 2. **Storage Service** - ✅ CONFIGURED (Local Storage)

**Current Setup:**
- **Type**: Local file storage
- **Path**: `./uploads` (inside Docker container)
- **Volume**: `whatomate_uploads` (persistent Docker volume)

**What it's used for:**
- WhatsApp media files (images, videos, documents, audio)
- Downloaded from Meta's WhatsApp API
- Served to users through the application

**S3 Configuration (Optional):**
If you want to use AWS S3 instead of local storage, you can configure it:

```bash
# Add to docker/.env.production
WHATOMATE_STORAGE_TYPE=s3
WHATOMATE_STORAGE_S3_BUCKET=your-bucket-name
WHATOMATE_STORAGE_S3_REGION=us-east-1
WHATOMATE_STORAGE_S3_KEY=your-aws-access-key
WHATOMATE_STORAGE_S3_SECRET=your-aws-secret-key
```

Then update `docker/docker-compose.production.yml`:
```yaml
whatomate-api:
  environment:
    - WHATOMATE_STORAGE_TYPE=${STORAGE_TYPE:-local}
    - WHATOMATE_STORAGE_S3_BUCKET=${S3_BUCKET:-}
    - WHATOMATE_STORAGE_S3_REGION=${S3_REGION:-}
    - WHATOMATE_STORAGE_S3_KEY=${S3_KEY:-}
    - WHATOMATE_STORAGE_S3_SECRET=${S3_SECRET:-}
```

**Note:** S3 support is built into the code but NOT currently active. Local storage is sufficient for most use cases.

### 3. **Webhook URL** - ✅ CONFIGURED

**Current URL:** `https://wp-api.xobito.com/api/webhook`

This URL is now hardcoded in the frontend and will display correctly in:
- Settings → Accounts page
- WhatsApp account configuration

### 4. **API URLs** - ✅ CONFIGURED

**Frontend Configuration:**
- API URL: `https://wp-api.xobito.com/api`
- WebSocket URL: `wss://wp-api.xobito.com/ws`

**Backend Configuration:**
- Server: `0.0.0.0:8080` (internal)
- Exposed via nginx: `https://wp-api.xobito.com`

## 📋 Services Summary

| Service | Status | Configuration Required |
|---------|--------|------------------------|
| **Email/SMTP** | ❌ Not Used | None - Not implemented |
| **Storage (Local)** | ✅ Active | Already configured |
| **Storage (S3)** | ⚪ Optional | See above if needed |
| **WhatsApp API** | ✅ Active | Configured in Settings |
| **Database** | ✅ Active | PostgreSQL on port 5703 |
| **Redis** | ✅ Active | Redis on port 5704 |
| **WebSocket** | ✅ Active | wss://wp-api.xobito.com/ws |

## 🔧 Apply Frontend Changes

To apply the webhook URL update and branding changes:

```bash
# On your server
cd /home/production/app/whatomate/docker

# Rebuild frontend
docker compose -f docker-compose.production.yml build whatomate-frontend

# Restart frontend
docker compose -f docker-compose.production.yml up -d whatomate-frontend

# Verify
docker compose -f docker-compose.production.yml ps
```

## 🎯 What's Changed

1. ✅ **Branding**: "Whatomate" → "XOBITO"
2. ✅ **Logo**: Updated to your XOBITO logo
3. ✅ **Webhook URL**: Now shows `https://wp-api.xobito.com/api/webhook`
4. ✅ **API URLs**: Using custom domain instead of IP

## 📝 No Additional Configuration Needed

You do NOT need to configure:
- ❌ Email/SMTP service (not used)
- ❌ S3 storage (local storage is working)
- ❌ Additional environment variables

Everything is ready to use!

## 🔍 Optional: Enable S3 Storage

Only if you want to use S3 instead of local storage:

1. Create an S3 bucket in AWS
2. Create IAM user with S3 access
3. Add credentials to `.env.production`
4. Update docker-compose environment variables
5. Rebuild and restart containers

**Benefits of S3:**
- Scalable storage
- Better for multiple server instances
- Automatic backups
- CDN integration possible

**Benefits of Local Storage (current):**
- Simpler setup
- No AWS costs
- Faster access
- Sufficient for single-server deployments

## ✅ Current Status: Production Ready

Your XOBITO deployment is fully configured and ready to use with:
- Custom domain (wp.xobito.com, wp-api.xobito.com)
- SSL/HTTPS enabled
- Local file storage
- All services running
- Webhook configured

No additional configuration is required unless you want to enable S3 storage.
