import React, { useState, useEffect } from 'react';
import { toast } from '../components/Toast';
import { Users, ShieldCheck, Plus, Search, UserCheck } from 'lucide-react';
import GlassCard from '../components/GlassCard';
import Button from '../components/Button';
import AuthProviderForm from '../components/AuthProviderForm';

export default function AuthSettingsView() {
  const [activeTab, setActiveTab] = useState<'users' | 'providers'>('users');
  const [users, setUsers] = useState<any[]>([]);
  const [search, setSearch] = useState('');
  const [showAddUser, setShowAddUser] = useState(false);
  const [newEmail, setNewEmail] = useState('');
  const [newPassword, setNewPassword] = useState('');

  // Fetch users from auth backend (mock for now – will need a proper endpoint; for demo we show a placeholder)
  useEffect(() => {
    // This would be a real call to auth-server to list users (not implemented yet). We'll show a message.
    setUsers([]);
  }, []);

  const handleCreateUser = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newEmail || !newPassword) return;
    try {
      const res = await fetch('/api/auth/signup', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: newEmail, password: newPassword })
      });
      if (res.ok) {
        toast.success(`User ${newEmail} registered!`);
        setUsers([...users, { id: 'usr_' + Date.now(), email: newEmail, provider: 'email', created_at: new Date().toISOString(), mfa: 'inactive' }]);
        setNewEmail('');
        setNewPassword('');
        setShowAddUser(false);
      } else {
        toast.error('Failed to create user');
      }
    } catch (err) {
      toast.error('Network error');
    }
  };

  const handleDeleteUser = (id: string) => {
    setUsers(users.filter(u => u.id !== id));
    toast.error('User removed (local only – server still has account)');
  };

  const filteredUsers = users.filter(u => u.email.toLowerCase().includes(search.toLowerCase()));

  return (
    <div className="space-y-6">
      <div className="border-b border-zinc-200 dark:border-zinc-900 flex gap-4 text-xs font-semibold uppercase tracking-wider">
        <button onClick={() => setActiveTab('users')} className={`pb-3 border-b-2 ${activeTab === 'users' ? 'border-primary text-primary' : 'border-transparent text-zinc-500'}`}>
          <Users className="inline h-4 w-4 mr-1" /> Users
        </button>
        <button onClick={() => setActiveTab('providers')} className={`pb-3 border-b-2 ${activeTab === 'providers' ? 'border-primary text-primary' : 'border-transparent text-zinc-500'}`}>
          <ShieldCheck className="inline h-4 w-4 mr-1" /> Providers
        </button>
      </div>

      {activeTab === 'users' && (
        <div className="space-y-4">
          <div className="flex gap-3">
            <div className="relative flex-1 max-w-xs">
              <Search className="absolute left-2 top-1/2 -translate-y-1/2 h-4 w-4 text-zinc-400" />
              <input
                type="text"
                placeholder="Search users..."
                value={search}
                onChange={e => setSearch(e.target.value)}
                className="w-full pl-8 pr-4 py-2 text-sm rounded-lg border border-zinc-300 dark:border-zinc-700 bg-white dark:bg-zinc-900"
              />
            </div>
            <Button variant="primary" size="sm" onClick={() => setShowAddUser(true)}><Plus className="h-4 w-4" /> Add User</Button>
          </div>

          {showAddUser && (
            <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
              <GlassCard className="p-6 w-80">
                <h3 className="font-bold mb-4">Register User</h3>
                <form onSubmit={handleCreateUser} className="space-y-4">
                  <input value={newEmail} onChange={e => setNewEmail(e.target.value)} placeholder="Email" className="w-full p-2 border rounded" required />
                  <input type="password" value={newPassword} onChange={e => setNewPassword(e.target.value)} placeholder="Password" className="w-full p-2 border rounded" required />
                  <div className="flex justify-end gap-2">
                    <button type="button" onClick={() => setShowAddUser(false)} className="text-sm">Cancel</button>
                    <button type="submit" className="text-sm bg-primary text-white px-3 py-1 rounded">Create</button>
                  </div>
                </form>
              </GlassCard>
            </div>
          )}

          <GlassCard className="overflow-hidden">
            <table className="w-full text-xs">
              <thead className="bg-zinc-50 dark:bg-zinc-900">
                <tr className="text-zinc-500">
                  <th className="px-4 py-2 text-left">ID</th>
                  <th className="px-4 py-2 text-left">Email</th>
                  <th className="px-4 py-2 text-left">Provider</th>
                  <th className="px-4 py-2 text-left">MFA</th>
                  <th className="px-4 py-2 text-left">Created</th>
                  <th className="px-4 py-2"></th>
                </tr>
              </thead>
              <tbody>
                {filteredUsers.map(u => (
                  <tr key={u.id} className="border-t border-zinc-200 dark:border-zinc-800">
                    <td className="px-4 py-2">{u.id}</td>
                    <td className="px-4 py-2">{u.email}</td>
                    <td className="px-4 py-2">{u.provider}</td>
                    <td className="px-4 py-2">{u.mfa}</td>
                    <td className="px-4 py-2">{new Date(u.created_at).toLocaleDateString()}</td>
                    <td className="px-4 py-2">
                      <button onClick={() => handleDeleteUser(u.id)} className="text-red-500 hover:underline">Delete</button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </GlassCard>
        </div>
      )}

      {activeTab === 'providers' && <AuthProviderForm />}
    </div>
  );
}
