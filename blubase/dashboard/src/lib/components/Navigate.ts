import { useCallback } from 'react';
export const useNavigate = () => {
  return useCallback((path: string) => window.history.pushState({}, '', path), []);
};
