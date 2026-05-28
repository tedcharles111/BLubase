import { useSyncExternalStore } from 'react';

export function useStore<T>(store: { subscribe: (cb: () => void) => () => void; get: () => T }) {
  return useSyncExternalStore(store.subscribe, store.get);
}
