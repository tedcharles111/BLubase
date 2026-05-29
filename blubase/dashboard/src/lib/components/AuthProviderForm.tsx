import React, { useState } from 'react';
import { toast } from './Toast';
import { ShieldCheck, Plus, Key, Globe } from 'lucide-react';
import GlassCard from './GlassCard';
import Button from './Button';

interface AuthProvider {
  id: string;
  name: string;
  enabled: boolean;
  client_id: string;
  client_secret: string;
  redirect_url: string;
}

const initialProviders: AuthProvider[] = [
  { id: 'github', name: 'GitHub', enabled: false, client_id: '', client_secret: '', redirect_url: 'http://localhost:3001/auth/github/callback' },
  { id: 'google', name: 'Google', enabled: false, client_id: '', client_secret: '', redirect_url: 'http://localhost:3001/auth/google/callback' },
  { id: 'gitlab', name: 'GitLab', enabled: false, client_id: '', client_secret: '', redirect_url: 'http://localhost:3001/auth/gitlab/callback' },
  { id: 'figma', name: 'Figma', enabled: false, client_id: '', client_secret: '', redirect_url: 'http://localhost:3001/auth/figma/callback' },
  { id: 'facebook', name: 'Facebook', enabled: false, client_id: '', client_secret: '', redirect_url: 'http://localhost:3001/auth/facebook/callback' },
];

export default function AuthProviderForm() {
  const [providers, setProviders] = useState<AuthProvider[]>(initialProviders);
  const [editId, setEditId] = useState<string | null>(null);
  const [form, setForm] = useState({ client_id: '', client_secret: '' });

  const handleToggle = (id: string) => {
    setProviders(prev =>
      prev.map(p => (p.id === id ? { ...p, enabled: !p.enabled } : p))
    );
    toast.success(`Provider ${id} toggled`);
  };

  const handleEdit = (id: string) => {
    const provider = providers.find(p => p.id === id);
    if (!provider) return;
    setEditId(id);
    setForm({ client_id: provider.client_id, client_secret: provider.client_secret });
  };

  const handleSave = () => {
    if (!editId) return;
    setProviders(prev =>
      prev.map(p => (p.id === editId ? { ...p, client_id: form.client_id, client_secret: form.client_secret } : p))
    );
    setEditId(null);
    toast.success('Provider configuration saved');
  };

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
      {providers.map(provider => (
        <GlassCard key={provider.id} className="p-4">
          <div className="flex justify-between items-center mb-3">
            <div className="flex items-center gap-2">
              <ShieldCheck className="h-4 w-4 text-primary" />
              <span className="font-semibold text-sm">{provider.name}</span>
            </div>
            <label className="relative inline-flex items-center cursor-pointer">
              <input
                type="checkbox"
                checked={provider.enabled}
                onChange={() => handleToggle(provider.id)}
                className="sr-only peer"
              />
              <div className="w-9 h-5 bg-zinc-300 peer-checked:bg-primary rounded-full peer peer-checked:after:translate-x-full after:content-[''] after:absolute after:top-0.5 after:left-[2px] after:bg-white after:rounded-full after:h-4 after:w-4 after:transition-all" />
            </label>
          </div>

          {provider.enabled && (
            <div className="space-y-2 text-xs">
              <div>
                <label className="text-zinc-500 font-semibold">Client ID</label>
                <input
                  type="text"
                  value={editId === provider.id ? form.client_id : provider.client_id}
                  onChange={e => setForm({ ...form, client_id: e.target.value })}
                  className="w-full p-1.5 rounded border border-zinc-300 dark:border-zinc-700 bg-zinc-50 dark:bg-zinc-900"
                />
              </div>
              <div>
                <label className="text-zinc-500 font-semibold">Client Secret</label>
                <input
                  type="password"
                  value={editId === provider.id ? form.client_secret : provider.client_secret}
                  onChange={e => setForm({ ...form, client_secret: e.target.value })}
                  className="w-full p-1.5 rounded border border-zinc-300 dark:border-zinc-700 bg-zinc-50 dark:bg-zinc-900"
                />
              </div>
              <div className="flex gap-2 items-center">
                <Globe className="h-3.5 w-3.5 text-zinc-500" />
                <span className="text-zinc-500 truncate">{provider.redirect_url}</span>
              </div>
              {editId === provider.id ? (
                <div className="flex gap-2 mt-2">
                  <button onClick={handleSave} className="px-3 py-1 text-[10px] bg-primary text-white rounded">Save</button>
                  <button onClick={() => setEditId(null)} className="px-3 py-1 text-[10px] bg-zinc-200 dark:bg-zinc-700 rounded">Cancel</button>
                </div>
              ) : (
                <button onClick={() => handleEdit(provider.id)} className="mt-2 px-3 py-1 text-[10px] bg-zinc-100 dark:bg-zinc-800 rounded hover:bg-zinc-200">
                  Configure
                </button>
              )}
            </div>
          )}
        </GlassCard>
      ))}
    </div>
  );
}
