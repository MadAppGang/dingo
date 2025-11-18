// Authentication State Listener Component
// Reference: ai-docs/02-islands-architecture.md - Critical island with client:load
// MUST use client:load to initialize auth state immediately on page load

import { useEffect } from 'react';
import { onAuthStateChanged } from 'firebase/auth';
import { auth } from '../../lib/firebase';
import { setUser, setLoading } from '../../stores/auth';

export default function AuthStateListener() {
  useEffect(() => {
    // Subscribe to Firebase auth state changes
    const unsubscribe = onAuthStateChanged(
      auth,
      (user) => {
        // Update nanostores with current user
        setUser(user);
        setLoading(false);
      },
      (error) => {
        console.error('Auth state error:', error);
        setUser(null);
        setLoading(false);
      }
    );

    // Cleanup subscription on unmount
    return () => unsubscribe();
  }, []);

  // This component has no UI - it only manages state
  return null;
}
