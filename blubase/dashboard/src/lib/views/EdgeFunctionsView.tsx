import React, { useState, useEffect } from 'react';
import { toast } from '../components/Toast';
import GlassCard from '../components/GlassCard';
import Button from '../components/Button';
import CodeEditor from '../components/CodeEditor';
import { Plus, Play, Trash2, Cpu } from 'lucide-react';

export default function EdgeFunctionsView() {
  const [functions, setFunctions] = useState<{ name: string; code: string; logs: string[] }[]>([]);
  const [activeFn, setActiveFn] = useState<string | null>(null);
  const [code, setCode] = useState('');
  const [newFnName, setNewFnName] = useState('');
  const [showCreate, setShowCreate] = useState(false);

  useEffect(() => {
    // fetch functions from edge manager (mock for now)
  }, []);

  const handleDeploy = async () => {
    if (!activeFn) return;
    try {
      const res = await fetch(`/api/edge/invoke/${activeFn}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ code })
      });
      if (res.ok) {
        toast.success(`Function ${activeFn} deployed`);
      } else {
        toast.error('Deploy failed');
      }
    } catch (err) {
      toast.error('Network error');
    }
  };

  const handleCreate = () => {
    if (!newFnName) return;
    setFunctions([...functions, { name: newFnName, code: `export default async function(req) { return new Response("Hello"); }`, logs: [] }]);
    setActiveFn(newFnName);
    setCode(`export default async function(req) { return new Response("Hello"); }`);
    setNewFnName('');
    setShowCreate(false);
    toast.success('Function created (locally)');
  };

  return (
    <div className="space-y-4">
      <div className="flex gap-2 items-center">
        <Button variant="primary" size="sm" onClick={() => setShowCreate(true)}><Plus className="h-4 w-4" /> New Function</Button>
      </div>

      {showCreate && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <GlassCard className="p-6 w-80">
            <h3 className="font-bold mb-4">Create Edge Function</h3>
            <input value={newFnName} onChange={e => setNewFnName(e.target.value)} className="w-full p-2 border rounded mb-4" placeholder="function-name" />
            <div className="flex justify-end gap-2">
              <button onClick={() => setShowCreate(false)} className="text-sm">Cancel</button>
              <button onClick={handleCreate} className="text-sm bg-primary text-white px-3 py-1 rounded">Create</button>
            </div>
          </GlassCard>
        </div>
      )}

      {functions.length > 0 && (
        <div className="flex gap-4">
          <div className="w-48 space-y-1">
            {functions.map(f => (
              <div
                key={f.name}
                onClick={() => { setActiveFn(f.name); setCode(f.code); }}
                className={`p-2 rounded cursor-pointer text-sm ${activeFn === f.name ? 'bg-primary text-white' : 'hover:bg-zinc-100 dark:hover:bg-zinc-800'}`}
              >
                {f.name}
              </div>
            ))}
          </div>
          <div className="flex-1">
            <CodeEditor value={code} onChange={setCode} language="typescript" onRun={handleDeploy} />
          </div>
        </div>
      )}
    </div>
  );
}
