let dark = true;
const listeners = new Set<() => void>();
const toggle = () => {
  dark = !dark;
  document.documentElement.classList.toggle('dark', dark);
  listeners.forEach(l => l());
};

export const themeStore = {
  subscribe: (cb: () => void) => { listeners.add(cb); return () => { listeners.delete(cb); } },
  get: () => dark
};
export const toggleTheme = toggle;
