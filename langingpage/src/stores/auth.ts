// Authentication State Store
// Reference: ai-docs/02-islands-architecture.md - Shared state with nanostores

import { atom } from 'nanostores';
import type { User } from 'firebase/auth';

// User state atom (null when not authenticated)
export const userAtom = atom<User | null>(null);

// Loading state (true during auth operations)
export const loadingAtom = atom<boolean>(true);

// Error state (null when no error)
export const errorAtom = atom<string | null>(null);

// Actions to update state
export function setUser(user: User | null): void {
  userAtom.set(user);
}

export function setLoading(loading: boolean): void {
  loadingAtom.set(loading);
}

export function setError(error: string | null): void {
  errorAtom.set(error);
}

export function clearError(): void {
  errorAtom.set(null);
}

// Re-export Firebase auth functions for convenience
export { signInWithGitHub, signInWithGoogle, signOut } from '../lib/firebase';
