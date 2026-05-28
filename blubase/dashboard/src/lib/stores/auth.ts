type AuthState = { token: string | null; user: { email: string; id: string } | null };

let state: AuthState = { token: null, user: null };
const listeners = new Set<() => void>();

const update = (newState: Partial<AuthState>) => {
  state = { ...state, ...newState };
  listeners.forEach(l => l());
};

export const authStore = {
  subscribe: (cb: () => void) => { listeners.add(cb); return () => { listeners.delete(cb); } },
  get: () => state
};

export const authActions = {
  login: async (email: string, password: string) => {
    const res = await fetch('/api/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password })
    });
    const data = await res.json();
    if (res.ok) update({ token: data.token, user: { email, id: data.userId } });
    return data;
  },
  signup: async (email: string, password: string) => {
    const res = await fetch('/api/auth/signup', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password })
    });
    return res.json();
  },
  logout: () => update({ token: null, user: null })
};
