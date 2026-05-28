import React from 'react';

interface GlassCardProps extends React.HTMLAttributes<HTMLDivElement> {
  hoverable?: boolean;
}

export default function GlassCard({ children, className = '', hoverable, ...props }: GlassCardProps) {
  return (
    <div
      className={`rounded-lg border border-zinc-200 dark:border-zinc-800 bg-white dark:bg-zinc-950 shadow-sm transition-all duration-200 ${
        hoverable ? 'hover:shadow-md hover:scale-[1.01] cursor-pointer' : ''
      } ${className}`}
      {...props}
    >
      {children}
    </div>
  );
}
