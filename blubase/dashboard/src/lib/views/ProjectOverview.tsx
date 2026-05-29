import React, { useEffect, useState } from 'react';
import { useStore } from '../stores/storeHelper';
import { projectStore } from '../stores/project';
import { toast } from '../components/Toast';
import GlassCard from '../components/GlassCard';
import { Clipboard, Eye, EyeOff, Server } from 'lucide-react';
import Button from '../components/Button';

export default function ProjectOverview({ project, onNavigate }: { project: any; onNavigate: (p: string) => void }) {
  const [showKeys, setShowKeys] = useState(false);
  const [projData, setProjData] = useState(project);

  // Fetch fresh project details from backend
  useEffect(() => {
    (async () => {
      try {
        const res = await fetch('/api/projects/projects');
        const projects = await res.json();
        const current = projects.find((p: any) => p.ref === project.ref);
        if (current) setProjData(current);
      } catch (e) {
        console.error('Failed to fetch project details');
      }
    })();
  }, [project.ref]);

  const copyToClipboard = (text: string, label: string) => {
    navigator.clipboard.writeText(text);
    toast.success(`${label} copied!`);
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 border-b border-zinc-200 dark:border-zinc-900 pb-5">
        <div>
          <h2 className="text-xl font-bold flex items-center gap-2">
            <Server className="h-5 w-5 text-primary" />
            {projData.name}
          </h2>
          <p className="text-xs text-zinc-500 font-mono">ref: {projData.ref} · region: {projData.region || 'us-east-1'}</p>
        </div>
        <button
          onClick={() => setShowKeys(!showKeys)}
          className="flex items-center gap-2 px-3 py-1.5 text-xs font-semibold rounded-lg bg-zinc-100 dark:bg-zinc-900 border border-zinc-300 dark:border-zinc-700"
        >
          {showKeys ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
          {showKeys ? 'Hide keys' : 'Reveal keys'}
        </button>
      </div>

      {/* Connection strings & keys */}
      <GlassCard className="p-5 space-y-4">
        <h3 className="text-xs font-semibold uppercase tracking-wider text-zinc-500">API Credentials</h3>
        <div className="space-y-3 text-xs">
          <div>
            <div className="flex justify-between items-center">
              <span className="font-semibold">Anon Public Key</span>
              <button onClick={() => copyToClipboard(projData.anon_key, 'Anon key')} className="text-primary hover:underline flex items-center gap-1">
                <Clipboard className="h-3.5 w-3.5" /> Copy
              </button>
            </div>
            <input
              type={showKeys ? 'text' : 'password'}
              readOnly
              value={projData.anon_key}
              className="w-full p-2 mt-1 font-mono bg-zinc-50 dark:bg-zinc-900 rounded border border-zinc-200 dark:border-zinc-800"
            />
          </div>
          <div>
            <div className="flex justify-between items-center">
              <span className="font-semibold text-amber-500">Service Role Key</span>
              <button onClick={() => copyToClipboard(projData.service_role_key, 'Service key')} className="text-amber-500 hover:underline flex items-center gap-1">
                <Clipboard className="h-3.5 w-3.5" /> Copy
              </button>
            </div>
            <input
              type={showKeys ? 'text' : 'password'}
              readOnly
              value={projData.service_role_key}
              className="w-full p-2 mt-1 font-mono bg-zinc-50 dark:bg-zinc-900 rounded border border-zinc-200 dark:border-zinc-800 text-amber-500"
            />
          </div>
        </div>
      </GlassCard>

      {/* Quick start code snippet */}
      <GlassCard className="p-5">
        <h3 className="text-xs font-semibold uppercase tracking-wider text-zinc-500 mb-2">Client SDK Setup</h3>
        <pre className="text-[10px] font-mono p-3 rounded-lg bg-zinc-950 text-emerald-400 overflow-x-auto">
{`import { createClient } from '@blubase/client';

const blubase = createClient({
  projectUrl: 'http://localhost',
  anonKey: '${projData.anon_key?.substring(0, 20)}...'
});`}
        </pre>
      </GlassCard>
    </div>
  );
}
