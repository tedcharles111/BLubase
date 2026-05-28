import React from 'react';
import { useNavigate } from './Navigate'; // we'll define a simple hook

export default function Sidebar({ currentPath, onNavigate, activeRef }: { currentPath: string; onNavigate: (p: string) => void; activeRef: string | null }) {
  const links = [
    { name: 'Projects', path: '/projects', icon: '📦' },
    ...(activeRef ? [
      { name: 'Overview', path: `/projects/${activeRef}`, icon: '📊' },
      { name: 'Database', path: `/projects/${activeRef}/database/tables`, icon: '🗃️' },
      { name: 'SQL', path: `/projects/${activeRef}/database/sql`, icon: '⚡' },
      { name: 'Auth', path: `/projects/${activeRef}/auth`, icon: '🔐' },
      { name: 'Storage', path: `/projects/${activeRef}/storage/files`, icon: '💾' },
      { name: 'Edge Functions', path: `/projects/${activeRef}/edge-functions`, icon: '⚙️' },
      { name: 'Realtime', path: `/projects/${activeRef}/realtime`, icon: '🔌' },
      { name: 'Logs', path: `/projects/${activeRef}/logs`, icon: '📄' },
      { name: 'Settings', path: `/projects/${activeRef}/settings`, icon: '⚙️' },
      { name: 'AI Assistant', path: `/projects/${activeRef}/ai-assistant`, icon: '🤖' },
    ] : [])
  ];
  return (
    <aside className="w-56 bg-zinc-100 dark:bg-zinc-900 border-r border-zinc-200 dark:border-zinc-800 p-3 flex flex-col gap-1">
      {links.map(l => (
        <button key={l.path} onClick={() => onNavigate(l.path)} className={`text-left px-3 py-2 rounded text-sm ${currentPath === l.path ? 'bg-primary text-white' : 'hover:bg-zinc-200 dark:hover:bg-zinc-800'}`}>
          {l.icon} {l.name}
        </button>
      ))}
    </aside>
  );
}
