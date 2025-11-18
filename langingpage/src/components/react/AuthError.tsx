// Authentication Error Display Component
// Reference: ai-docs/02-islands-architecture.md - Interactive island with client:load

import { useStore } from '@nanostores/react';
import { errorAtom, clearError } from '../../stores/auth';
import { useEffect, useRef } from 'react';

export default function AuthError() {
  const error = useStore(errorAtom);
  const alertRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (error && alertRef.current) {
      // Move focus to error for keyboard users and screen readers
      // Following WCAG 2.1 - Focus management for dynamic content
      alertRef.current.focus();

      // Auto-dismiss after 10 seconds
      // Gives users time to read but prevents stale errors
      const timer = setTimeout(() => {
        clearError();
      }, 10000);

      return () => clearTimeout(timer);
    }
  }, [error]);

  if (!error) {
    return null;
  }

  return (
    <div
      ref={alertRef}
      tabIndex={-1}
      role="alert"
      aria-live="assertive"
      aria-atomic="true"
      className="fixed bottom-4 left-1/2 transform -translate-x-1/2 z-50 max-w-md w-full focus:outline-none focus:ring-2 focus:ring-red-500 focus:ring-offset-2 rounded-lg"
    >
      <div className="bg-red-50 border border-red-200 rounded-lg p-4 shadow-lg">
        <div className="flex items-start">
          <div className="flex-shrink-0">
            {/* Error Icon */}
            <svg className="h-5 w-5 text-red-400" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
              <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clipRule="evenodd" />
            </svg>
          </div>
          <div className="ml-3 flex-1">
            <p className="text-sm font-medium text-red-800">
              {error}
            </p>
            <p className="text-xs text-red-600 mt-1">
              This message will auto-dismiss in 10 seconds.
            </p>
          </div>
          <div className="ml-auto pl-3">
            <button
              onClick={() => clearError()}
              className="inline-flex rounded-md text-red-400 hover:text-red-500 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-red-500"
              aria-label="Dismiss error"
            >
              <span className="sr-only">Dismiss</span>
              {/* Close Icon */}
              <svg className="h-5 w-5" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                <path fillRule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clipRule="evenodd" />
              </svg>
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
