# Firebase Authentication Setup Guide

This guide walks you through setting up Firebase Authentication with GitHub and Google OAuth providers for the Dingo landing page.

## Prerequisites

- A Google account (for Firebase Console access)
- A GitHub account (for GitHub OAuth app)
- Admin access to the repository (for GitHub secrets)

## Step 1: Create Firebase Project

1. Go to [Firebase Console](https://console.firebase.google.com/)
2. Click "Add Project" or "Create a project"
3. Enter project name: `dingo-landing-page` (or your preferred name)
4. Disable Google Analytics (optional for landing page)
5. Click "Create Project"

## Step 2: Enable Authentication Providers

### Enable Google Authentication

1. In Firebase Console, go to **Authentication** → **Sign-in method**
2. Click on **Google** provider
3. Toggle **Enable** switch
4. Set **Public-facing name**: "Dingo"
5. Choose **Project support email** from dropdown
6. Click **Save**

### Enable GitHub Authentication

1. In Firebase Console, go to **Authentication** → **Sign-in method**
2. Click on **GitHub** provider
3. Toggle **Enable** switch
4. Copy the **Authorization callback URL** (you'll need this in Step 3)
5. Leave this page open (you'll add GitHub OAuth credentials in Step 4)

## Step 3: Create GitHub OAuth App

1. Go to [GitHub Developer Settings](https://github.com/settings/developers)
2. Click **OAuth Apps** → **New OAuth App**
3. Fill in the form:
   - **Application name**: Dingo Landing Page
   - **Homepage URL**: `https://dingolang.com` (or your GitHub Pages URL)
   - **Authorization callback URL**: Paste the URL from Firebase (Step 2)
     - Format: `https://[your-project-id].firebaseapp.com/__/auth/handler`
4. Click **Register application**
5. Copy the **Client ID**
6. Click **Generate a new client secret** and copy the **Client Secret**

## Step 4: Add GitHub Credentials to Firebase

1. Return to Firebase Console (Step 2, GitHub provider)
2. Paste **Client ID** from GitHub
3. Paste **Client Secret** from GitHub
4. Click **Save**

## Step 5: Configure Authorized Domains

1. In Firebase Console, go to **Authentication** → **Settings** → **Authorized domains**
2. Add the following domains:
   - `localhost` (already added by default)
   - Your GitHub Pages domain (e.g., `username.github.io`)
   - Your custom domain if applicable (e.g., `dingolang.com`)
3. Click **Add domain** for each

## Step 6: Get Firebase Configuration

1. In Firebase Console, go to **Project Settings** (gear icon)
2. Scroll down to **Your apps** section
3. Click **Web app** icon (`</>`)
4. Register app name: "Dingo Landing Page"
5. Don't enable Firebase Hosting
6. Click **Register app**
7. Copy the `firebaseConfig` object values:

```javascript
const firebaseConfig = {
  apiKey: "AIza...",              // PUBLIC_FIREBASE_API_KEY
  authDomain: "project.firebaseapp.com",  // PUBLIC_FIREBASE_AUTH_DOMAIN
  projectId: "project-id",        // PUBLIC_FIREBASE_PROJECT_ID
  storageBucket: "project.appspot.com",   // PUBLIC_FIREBASE_STORAGE_BUCKET
  messagingSenderId: "123456789", // PUBLIC_FIREBASE_MESSAGING_SENDER_ID
  appId: "1:123456789:web:abc123" // PUBLIC_FIREBASE_APP_ID
};
```

## Step 7: Configure Local Environment

1. Copy `.env.example` to `.env`:
   ```bash
   cp .env.example .env
   ```

2. Edit `.env` and paste your Firebase config values:
   ```env
   PUBLIC_FIREBASE_API_KEY=AIza...
   PUBLIC_FIREBASE_AUTH_DOMAIN=project.firebaseapp.com
   PUBLIC_FIREBASE_PROJECT_ID=project-id
   PUBLIC_FIREBASE_STORAGE_BUCKET=project.appspot.com
   PUBLIC_FIREBASE_MESSAGING_SENDER_ID=123456789
   PUBLIC_FIREBASE_APP_ID=1:123456789:web:abc123
   ```

3. **IMPORTANT**: Never commit `.env` to git (it's in `.gitignore`)

## Step 8: Test Locally

1. Start the development server:
   ```bash
   pnpm dev
   ```

2. Open http://localhost:4321
3. Click "Sign in with Google" - verify popup and sign-in flow
4. Click "Sign in with GitHub" - verify popup and sign-in flow
5. Verify user menu appears with your name/avatar
6. Click "Sign out" - verify you're logged out
7. Refresh page - verify auth state persists (Firebase SDK uses localStorage)

## Step 9: Configure GitHub Secrets (for Deployment)

1. Go to your GitHub repository settings
2. Navigate to **Settings** → **Secrets and variables** → **Actions**
3. Click **New repository secret** for each:
   - Name: `PUBLIC_FIREBASE_API_KEY`, Value: (from Firebase config)
   - Name: `PUBLIC_FIREBASE_AUTH_DOMAIN`, Value: (from Firebase config)
   - Name: `PUBLIC_FIREBASE_PROJECT_ID`, Value: (from Firebase config)
   - Name: `PUBLIC_FIREBASE_STORAGE_BUCKET`, Value: (from Firebase config)
   - Name: `PUBLIC_FIREBASE_MESSAGING_SENDER_ID`, Value: (from Firebase config)
   - Name: `PUBLIC_FIREBASE_APP_ID`, Value: (from Firebase config)

## Step 10: Enable GitHub Pages

1. Go to repository **Settings** → **Pages**
2. Under **Source**, select **GitHub Actions**
3. The workflow in `.github/workflows/deploy.yml` will automatically deploy on push to main

## Step 11: Deploy

1. Commit your changes (but NOT `.env`):
   ```bash
   git add .
   git commit -m "Add Firebase Authentication"
   git push origin main
   ```

2. GitHub Actions will automatically build and deploy
3. Check **Actions** tab to monitor deployment
4. Once complete, visit your GitHub Pages URL

## Step 12: Update Firebase Authorized Domains (After First Deploy)

1. After your site is deployed, note the GitHub Pages URL
2. In Firebase Console, go to **Authentication** → **Settings** → **Authorized domains**
3. Verify your GitHub Pages domain is added
4. If using a custom domain, add it as well

## Testing Production Deployment

1. Visit your GitHub Pages URL
2. Test sign-in with Google
3. Test sign-in with GitHub
4. Verify auth state persists on page reload
5. Test sign-out

## Troubleshooting

### "auth/unauthorized-domain" Error

- **Cause**: Domain not authorized in Firebase
- **Fix**: Add the domain in Firebase Console → Authentication → Settings → Authorized domains

### GitHub OAuth Popup Closes Immediately

- **Cause**: Incorrect callback URL in GitHub OAuth app
- **Fix**: Verify callback URL matches Firebase's format: `https://[project-id].firebaseapp.com/__/auth/handler`

### "Firebase: Error (auth/configuration-not-found)"

- **Cause**: Firebase config not loaded (missing environment variables)
- **Fix**:
  - Local: Verify `.env` file exists with correct values
  - Production: Verify GitHub secrets are set correctly

### Auth State Not Persisting

- **Cause**: Browser blocking localStorage or cookies
- **Fix**: Check browser privacy settings, disable tracking protection for your domain

### CORS Errors

- **Cause**: Incorrect auth domain configuration
- **Fix**: Verify `PUBLIC_FIREBASE_AUTH_DOMAIN` matches your Firebase project

## Architecture Notes

### Why Firebase Auth Works with GitHub Pages

Firebase Authentication is a **client-side SDK** that works with any static hosting provider:

- All auth flows happen in the browser (OAuth popups/redirects)
- Firebase servers handle OAuth callback processing
- User tokens stored in browser localStorage
- No server-side code required

### Security Considerations

1. **API Key is Public**: Firebase API keys are safe to expose (they identify your project, not authenticate)
2. **Auth Rules**: Configure Firebase security rules to restrict data access
3. **OAuth Scopes**: GitHub/Google OAuth only requests basic profile info
4. **HTTPS Only**: GitHub Pages serves over HTTPS by default

## Next Steps

- Configure Firebase security rules (if using Firestore/Storage)
- Add user profile customization
- Implement protected content areas
- Add email/password authentication (optional)

## Resources

- [Firebase Authentication Docs](https://firebase.google.com/docs/auth)
- [Astro Environment Variables](https://docs.astro.build/en/guides/environment-variables/)
- [GitHub Pages Documentation](https://docs.github.com/en/pages)
- [GitHub OAuth Apps Guide](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps)
