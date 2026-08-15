"use client";

import React, { createContext, useContext, useState, useEffect } from "react";

export interface User {
  id: string;
  username: string;
  role: string;
  roles?: string[];
  legal_entity_id?: string;
}

export function hasRole(user: User | null | undefined, role: string): boolean {
  return user?.roles?.includes(role) || user?.role === role;
}

interface AuthContextType {
  user: User | null;
  token: string | null;
  login: (token: string, user: User, refreshToken?: string) => void;
  logout: () => void;
  isLoading: boolean;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [token, setToken] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    const storedToken = localStorage.getItem("token");
    const storedUser = localStorage.getItem("user");
    if (storedToken && storedUser) {
      try {
        setUser(JSON.parse(storedUser));
        setToken(storedToken);
      } catch {
        // FIX-022: corrupted localStorage user must not leave the app stuck
        // on "loading…" — parse failure is treated as logged out (clear both
        // keys) instead of being swallowed silently, so the user always
        // lands on the login page with a recoverable state.
        localStorage.removeItem("token");
        localStorage.removeItem("refresh_token");
        localStorage.removeItem("user");
      }
    }
    const handleTokenRefresh = (event: Event) => {
      const refreshedToken = (event as CustomEvent<string>).detail;
      if (refreshedToken) setToken(refreshedToken);
    };
    window.addEventListener("auth-token-refreshed", handleTokenRefresh);
    const handleSessionExpired = () => {
      localStorage.removeItem("token");
      localStorage.removeItem("refresh_token");
      localStorage.removeItem("user");
      setToken(null);
      setUser(null);
    };
    window.addEventListener("auth-session-expired", handleSessionExpired);
    setIsLoading(false);
    return () => {
      window.removeEventListener("auth-token-refreshed", handleTokenRefresh);
      window.removeEventListener("auth-session-expired", handleSessionExpired);
    };
  }, []);

  const login = (newToken: string, newUser: User, newRefreshToken?: string) => {
    localStorage.setItem("token", newToken);
    if (newRefreshToken) localStorage.setItem("refresh_token", newRefreshToken);
    localStorage.setItem("user", JSON.stringify(newUser));
    setToken(newToken);
    setUser(newUser);
  };

  const logout = () => {
    localStorage.removeItem("token");
    localStorage.removeItem("refresh_token");
    localStorage.removeItem("user");
    setToken(null);
    setUser(null);
  };

  return (
    <AuthContext.Provider value={{ user, token, login, logout, isLoading }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return context;
}
