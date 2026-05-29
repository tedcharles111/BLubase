import React from 'react';
import Logo from '../icons/Logo';
import DatabaseIcon from '../icons/DatabaseIcon';
import AuthIcon from '../icons/AuthIcon';
import StorageIcon from '../icons/StorageIcon';
import EdgeIcon from '../icons/EdgeIcon';
import RealtimeIcon from '../icons/RealtimeIcon';
import LogsIcon from '../icons/LogsIcon';
import SettingsIcon from '../icons/SettingsIcon';
import AIIcon from '../icons/AIIcon';

interface SidebarProps {
  currentPath: string;
  onNavigate: (path: string) => void;
  activeRef: string | null;
}

export default function Sidebar({ currentPath, onNavigate, activeRef }: SidebarProps) {
  const links = [
    { name: 'Projects', path: '/projects', icon: DatabaseIcon },
    ...(activeRef ? [
      { name: 'Overview', path: `/projects/${activeRef}`, icon: Logo },
      { name: 'Database', path: `/projects/${activeRef}/database/tables`, icon: DatabaseIcon },
      { name: 'SQL', path: `/projects/${activeRef}/database/sql`, icon: DatabaseIcon },
      { name: 'Auth', path: `/projects/${activeRef}/auth`, icon: AuthIcon },
      { name: 'Storage', path: `/projects/${activeRef}/storage/files`, icon: StorageIcon },
      { name: 'Edge Functions', path: `/projects/${activeRef}/edge-functions`, icon: EdgeIcon },
      { name: 'Realtime', path: `/projects/${activeRef}/realtime`, icon: RealtimeIcon },
      { name: 'Logs', path: `/projects/${activeRef}/logs`, icon: LogsIcon },
      { name: 'Settings', path: `/projects/${activeRef}/settings`, icon: SettingsIcon },
      { name: 'AI Assistant', path: `/projects/${activeRef}/ai-assistant`, icon: AIIcon },
    ] : [])
  ];

  return (
    <aside className="w-56 bg-white dark:bg-zinc-950 border-r border-zinc-200 dark:border-zinc-800 p-4 flex flex-col gap-1">
      <div className="flex items-center gap-2 px-2 py-3 mb-4">
        <Logo className="h-6 w-6" />
        <span className="font-display font-bold text-lg tracking-tight">Blubase</span>
      </div>
      {links.map((link) => {
        const Icon = link.icon;
        const isActive = currentPath === link.path;
        return (
          <button
            key={link.path}
            onClick={() => onNavigate(link.path)}
            className={`flex items-center gap-3 px-3 py-2 text-sm rounded-lg transition-all duration-150 ${
              isActive
                ? 'bg-primary text-white shadow-sm'
                : 'text-zinc-600 dark:text-zinc-400 hover:bg-zinc-100 dark:hover:bg-zinc-900'
            }`}
          >
            <Icon className="h-4 w-4" />
            <span className="font-medium">{link.name}</span>
          </button>
        );
      })}
    </aside>
  );
}
