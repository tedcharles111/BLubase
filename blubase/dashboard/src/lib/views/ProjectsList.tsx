import React, { useState } from 'react';
import { useStore } from '../stores/storeHelper';
import { projectStore, projectActions } from '../stores/project';
import { toast } from '../components/Toast';
import GlassCard from '../components/GlassCard';
import Button from '../components/Button';
import { Plus, Trash2 } from 'lucide-react';

export default function ProjectsList({ onNavigate }: { onNavigate: (path: string) => void }) {
  const store = useStore(projectStore);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [newProjName, setNewProjName] = useState('');

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newProjName) return;
    try {
      const proj = await projectActions.addProject(newProjName, 'us-east-1');
      toast.success(`Project "${proj.name}" created!`);
      onNavigate(`/projects/${proj.ref}`);
    } catch (err) {
      toast.error('Failed to create project');
    } finally {
      setShowCreateModal(false);
      setNewProjName('');
    }
  };

  return (
    <div>
      <div className="flex justify-between items-center mb-4">
        <h2 className="text-xl font-bold">Projects</h2>
        <Button variant="primary" size="sm" onClick={() => setShowCreateModal(true)}><Plus className="h-4 w-4"/> New Project</Button>
      </div>
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {store.projects.map(p => (
          <GlassCard key={p.ref} hoverable onClick={() => onNavigate(`/projects/${p.ref}`)} className="p-4">
            <div className="flex justify-between">
              <div>
                <h3 className="font-bold text-sm">{p.name}</h3>
                <p className="text-xs text-zinc-500">ref: {p.ref}</p>
              </div>
              <span className={`text-xs px-2 py-0.5 rounded ${p.status === 'active' ? 'bg-emerald-500/10 text-emerald-400' : 'bg-amber-500/10 text-amber-400'}`}>{p.status}</span>
            </div>
          </GlassCard>
        ))}
      </div>
      {showCreateModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <GlassCard className="p-6 w-full max-w-sm">
            <h3 className="font-bold mb-4">New Project</h3>
            <form onSubmit={handleCreate}>
              <input value={newProjName} onChange={e => setNewProjName(e.target.value)} className="w-full p-2 border rounded mb-4" placeholder="Project name" required />
              <div className="flex justify-end gap-2">
                <Button variant="outline" size="sm" type="button" onClick={() => setShowCreateModal(false)}>Cancel</Button>
                <Button variant="success" size="sm" type="submit">Create</Button>
              </div>
            </form>
          </GlassCard>
        </div>
      )}
    </div>
  );
}
