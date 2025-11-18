// Firebase Authentication Configuration
// Reference: ai-docs/02-islands-architecture.md - Client-side Firebase SDK

import { initializeApp } from 'firebase/app';
import {
  getAuth,
  signInWithPopup,
  signOut as firebaseSignOut,
  GoogleAuthProvider,
  GithubAuthProvider,
  type Auth,
  type UserCredential
} from 'firebase/auth';

// Firebase configuration from environment variables
// Uses PUBLIC_ prefix to expose to client-side (Astro requirement)
const firebaseConfig = {
  apiKey: import.meta.env.PUBLIC_FIREBASE_API_KEY,
  authDomain: import.meta.env.PUBLIC_FIREBASE_AUTH_DOMAIN,
  projectId: import.meta.env.PUBLIC_FIREBASE_PROJECT_ID,
  storageBucket: import.meta.env.PUBLIC_FIREBASE_STORAGE_BUCKET,
  messagingSenderId: import.meta.env.PUBLIC_FIREBASE_MESSAGING_SENDER_ID,
  appId: import.meta.env.PUBLIC_FIREBASE_APP_ID,
};

// Initialize Firebase
const app = initializeApp(firebaseConfig);

// Initialize Firebase Authentication
export const auth: Auth = getAuth(app);

// Auth Providers
const googleProvider = new GoogleAuthProvider();
const githubProvider = new GithubAuthProvider();

// Sign in with Google
export async function signInWithGoogle(): Promise<UserCredential> {
  return signInWithPopup(auth, googleProvider);
}

// Sign in with GitHub
export async function signInWithGitHub(): Promise<UserCredential> {
  return signInWithPopup(auth, githubProvider);
}

// Sign out
export async function signOut(): Promise<void> {
  return firebaseSignOut(auth);
}
