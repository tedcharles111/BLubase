import React, { useState, useEffect, useCallback } from 'react';

let toastId = 0;
const listeners = new Set<() => void>();
let toasts: { id: number; message: string; type: string }[] = [];

const notify = () => listeners.forEach(l => l());
export const toast = {
  success: (msg: string) => { toasts.push({ id: ++toastId, message: msg, type: 'success' }); notify(); },
  error: (msg: string) => { toasts.push({ id: ++toastId, message: msg, type: 'error' }); notify(); },
  info: (msg: string) => { toasts.push({ id: ++toastId, message: msg, type: 'info' }); notify(); },
  warning: (msg: string) => { toasts.push({ id: ++toastId, message: msg, type: 'warning' }); notify(); }
};

export default function ToastContainer() {
  const [items, setItems] = useState<typeof toasts>([]);

  useEffect(() => {
    const handler = () => setItems([...toasts]);
    listeners.add(handler);
    return () => { listeners.delete(handler); };
  }, []);

  const remove = (id: number) => {
    toasts = toasts.filter(t => t.id !== id);
    setItems([...toasts]);
  };

  return (
    <div className="fixed bottom-4 right-4 z-50 space-y-2">
      {items.map(t => (
        <div key={t.id} className={`px-4 py-2 rounded-lg text-white text-xs font-medium shadow-lg animate-slide-up ${
          t.type === 'success' ? 'bg-emerald-500' : t.type === 'error' ? 'bg-red-500' : t.type === 'info' ? 'bg-blue-500' : 'bg-amber-500'
        }`}>
          {t.message}
          <button className="ml-2 opacity-70 hover:opacity-100" onClick={() => remove(t.id)}>✕</button>
        </div>
      ))}
    </div>
  );
}
