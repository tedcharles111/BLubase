import React from 'react';
import logo from '/logo.svg';

export default function Navbar({ currentPath, onNavigate, activeRef }: { currentPath: string; onNavigate: (p: string) => void; activeRef: string | null }) {
  return (
    <nav className="h-12 border-b border-zinc-200 dark:border-zinc-800 flex items-center px-4 gap-3 bg-white dark:bg-zinc-950">
      <img src={logo} alt="Blubase" className="h-6 w-6" />
      <span className="font-bold text-sm">Blubase</span>
    </nav>
  );
}
