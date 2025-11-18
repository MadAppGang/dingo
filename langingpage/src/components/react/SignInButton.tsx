import { useState } from "react";
import { useStore } from "@nanostores/react";
import { Button } from "../ui/button";
import {
  Avatar,
  AvatarFallback,
  AvatarImage,
} from "../ui/avatar";
import { LogOut, User as UserIcon } from "lucide-react";
import { userAtom } from "../../stores/auth";
import { signInWithGitHub as firebaseSignInGitHub, signInWithGoogle as firebaseSignInGoogle, signOut as firebaseSignOut } from "../../lib/firebase";
import type { User as FirebaseUser } from "firebase/auth";

interface User {
  name: string;
  email: string;
  avatar: string;
  provider: "github" | "google";
}

interface SignInButtonProps {
  className?: string;
}

export function SignInButton({ className = "" }: SignInButtonProps) {
  const firebaseUser = useStore(userAtom) as FirebaseUser | null;
  const [isOpen, setIsOpen] = useState(false);

  // Convert Firebase user to component User format
  const user: User | null = firebaseUser
    ? {
        name: firebaseUser.displayName || firebaseUser.email?.split("@")[0] || "User",
        email: firebaseUser.email || "",
        avatar: firebaseUser.photoURL || `https://api.dicebear.com/7.x/avataaars/svg?seed=${firebaseUser.uid}`,
        provider: "github", // Default
      }
    : null;

  const handleGitHubLogin = async () => {
    try {
      await firebaseSignInGitHub();
      setIsOpen(false);
    } catch (error) {
      console.error("GitHub login failed:", error);
    }
  };

  const handleGoogleLogin = async () => {
    try {
      await firebaseSignInGoogle();
      setIsOpen(false);
    } catch (error) {
      console.error("Google login failed:", error);
    }
  };

  const handleLogout = async () => {
    try {
      await firebaseSignOut();
      setIsOpen(false);
    } catch (error) {
      console.error("Logout failed:", error);
    }
  };

  if (!user) {
    return (
      <div className={`fixed top-6 right-6 z-50 ${className}`}>
        <Button
          onClick={() => setIsOpen(!isOpen)}
          variant="outline"
          size="sm"
          className="bg-white shadow-lg hover:shadow-xl transition-shadow border-gray-200 gap-2"
        >
          <UserIcon className="w-4 h-4" />
          Sign In
        </Button>

        {isOpen && (
          <>
            <div
              className="fixed inset-0 z-40"
              onClick={() => setIsOpen(false)}
            />
            <div className="absolute top-full right-0 mt-2 w-64 bg-white rounded-lg shadow-xl border border-gray-200 p-4 z-50">
              <div className="space-y-3">
                <div className="pb-3 border-b">
                  <h4 className="font-medium text-sm">
                    Sign in to continue
                  </h4>
                  <p className="text-xs text-gray-500 mt-1">
                    Signing in to participate in discussions. And provide your feedback and value to the community.
                  </p>
                </div>

                <div className="space-y-2">
                  <Button
                    onClick={handleGitHubLogin}
                    variant="outline"
                    className="w-full gap-2 justify-start"
                  >
                    <svg
                      className="w-4 h-4"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      strokeWidth="2"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                    >
                      <path d="M15 22v-4a4.8 4.8 0 0 0-1-3.5c3 0 6-2 6-5.5A5.4 5.4 0 0 0 19 6a5.7 5.7 0 0 0 .1-3.5s-1 0-3 1.5a11.7 11.7 0 0 0-8 0C6.2 2 5.2 2 5.2 2A5.7 5.7 0 0 0 5.3 5.5 5.4 5.4 0 0 0 4 9c0 3.5 3 5.5 6 5.5-.4.5-.7 1-.9 1.7-.2.6-.2 1.3-.2 1.8v4"/>
                      <path d="M9 18c-4.51 2-5-2-7-2"/>
                    </svg>
                    Continue with GitHub
                  </Button>
                  <Button
                    onClick={handleGoogleLogin}
                    variant="outline"
                    className="w-full gap-2 justify-start"
                  >
                    <svg
                      className="w-4 h-4"
                      viewBox="0 0 24 24"
                    >
                      <path
                        fill="currentColor"
                        d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"
                      />
                      <path
                        fill="currentColor"
                        d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"
                      />
                      <path
                        fill="currentColor"
                        d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"
                      />
                      <path
                        fill="currentColor"
                        d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"
                      />
                    </svg>
                    Continue with Google
                  </Button>
                </div>
              </div>
            </div>
          </>
        )}
      </div>
    );
  }

  return (
    <div className={`fixed top-6 right-6 z-50 ${className}`}>
      <Button
        onClick={() => setIsOpen(!isOpen)}
        variant="outline"
        size="sm"
        className="bg-white shadow-lg hover:shadow-xl transition-shadow border-gray-200 gap-2"
      >
        <Avatar className="w-6 h-6">
          <AvatarImage src={user.avatar} alt={user.name} />
          <AvatarFallback>{user.name.charAt(0)}</AvatarFallback>
        </Avatar>
        <span className="text-sm">{user.name}</span>
      </Button>

      {isOpen && (
        <>
          <div
            className="fixed inset-0 z-40"
            onClick={() => setIsOpen(false)}
          />
          <div className="absolute top-full right-0 mt-2 w-64 bg-white rounded-lg shadow-xl border border-gray-200 p-4 z-50">
            <div className="space-y-3">
              <div className="flex items-center gap-3 pb-3 border-b">
                <Avatar className="w-10 h-10">
                  <AvatarImage
                    src={user.avatar}
                    alt={user.name}
                  />
                  <AvatarFallback>
                    {user.name.charAt(0)}
                  </AvatarFallback>
                </Avatar>
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium truncate">
                    {user.name}
                  </p>
                  <p className="text-xs text-gray-500 truncate">
                    {user.email}
                  </p>
                </div>
              </div>

              <div className="space-y-2">
                <h4 className="text-xs font-medium text-gray-900">
                  Your Stats
                </h4>
                <div className="grid gap-1.5 text-xs">
                  <div className="flex justify-between">
                    <span className="text-gray-600">
                      Examples Viewed
                    </span>
                    <span className="font-medium">24</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-gray-600">
                      Total Time
                    </span>
                    <span className="font-medium">45 min</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-gray-600">
                      Favorites
                    </span>
                    <span className="font-medium">7</span>
                  </div>
                </div>
              </div>

              <Button
                onClick={handleLogout}
                variant="outline"
                size="sm"
                className="w-full gap-2"
              >
                <LogOut className="w-4 h-4" />
                Logout
              </Button>
            </div>
          </div>
        </>
      )}
    </div>
  );
}

// Export the User type for external use
export type { User };
