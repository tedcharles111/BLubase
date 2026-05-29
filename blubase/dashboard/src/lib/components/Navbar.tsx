import React from 'react';
import Logo from '../icons/Logo';

export default function Navbar() {
  return (
    <nav className="h-12 border-b border-zinc-200 dark:border-zinc-800 flex items-center px-4 gap-3 bg-white dark:bg-zinc-950">
      <Logo className="h-5 w-5" />
      <span className="font-display font-bold text-sm tracking-tight">Blubase</span>
    </nav>
  );
}
