import type { User } from '../types';

export const AUTH_TOKEN_KEY = 'token';
export const AUTH_USER_KEY = 'user';

export const getToken = (): string | null => {
  return localStorage.getItem(AUTH_TOKEN_KEY);
};

export const setToken = (token: string): void => {
  localStorage.setItem(AUTH_TOKEN_KEY, token);
};

export const removeToken = (): void => {
  localStorage.removeItem(AUTH_TOKEN_KEY);
};

export const getUser = (): User | null => {
  const userStr = localStorage.getItem(AUTH_USER_KEY);
  if (userStr) {
    try {
      return JSON.parse(userStr);
    } catch (error) {
      console.error('Error parsing user data:', error);
      removeUser();
    }
  }
  return null;
};

export const setUser = (user: User): void => {
  localStorage.setItem(AUTH_USER_KEY, JSON.stringify(user));
};

export const removeUser = (): void => {
  localStorage.removeItem(AUTH_USER_KEY);
};

export const isAuthenticated = (): boolean => {
  return !!getToken();
};

export const logout = (): void => {
  removeToken();
  removeUser();
  window.location.href = '/login';
};